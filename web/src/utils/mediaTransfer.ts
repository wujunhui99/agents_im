// 整文件内容寻址上传/下载编排（EPIC #527 §3，**不分片**）。复用于头像与消息附件：
//   1. 整文件 SHA-256（hex 喂契约、base64 回放校验头）
//   2. CreateUploadIntent —— 秒传命中（alreadyComplete）直接拿 mediaId 返回，零字节传输
//   3. 否则带 x-amz-checksum-sha256 直传 OSS（失败整体重传），再 CompleteUpload 确认
// 下载侧：拿整文件 presigned GET URL，直拉 OSS 还原 Blob。

import type { MediaApi, MediaObject, MediaPurpose, MediaUploadProgress } from '../api/media';
import { uploadMediaBytes } from '../api/media';
import { digestSha256 } from './sha256';

export type MediaUploadPhase = 'hashing' | 'uploading' | 'finalizing' | 'instant';

export type MediaUploadProgressEvent = {
  phase: MediaUploadPhase;
  loaded: number;
  total: number;
};

export type UploadFileToMediaOptions = {
  file: Blob;
  purpose: MediaPurpose;
  mediaApi: MediaApi;
  /** 缺省取 File.name，再退回 'file'。 */
  filename?: string;
  /** 缺省取 Blob.type，再退回 application/octet-stream。 */
  contentType?: string;
  width?: number;
  height?: number;
  onProgress?: (event: MediaUploadProgressEvent) => void;
  /** 直传重试次数（整体重传，不分片）。默认 2 次重试（共 3 次尝试）。 */
  uploadRetries?: number;
  signal?: AbortSignal;
  /** 注入用于测试。 */
  fetchImpl?: typeof fetch;
};

export type UploadFileToMediaResult = {
  mediaId: string;
  /** true=文件级秒传命中，未传输字节。 */
  instant: boolean;
  /** 确认上传后的完整媒体对象；秒传路径不返回（未调用 complete）。 */
  media?: MediaObject;
};

export async function uploadFileToMedia(options: UploadFileToMediaOptions): Promise<UploadFileToMediaResult> {
  const { file, purpose, mediaApi } = options;
  const filename = options.filename ?? fileName(file) ?? 'file';
  const contentType = options.contentType ?? file.type ?? 'application/octet-stream';
  const total = file.size;

  options.onProgress?.({ phase: 'hashing', loaded: 0, total });
  const digest = await digestSha256(file);
  options.onProgress?.({ phase: 'hashing', loaded: total, total });

  const intent = await mediaApi.createUploadIntent({
    purpose,
    filename,
    contentType,
    sizeBytes: total,
    sha256: digest.hex,
    ...(options.width != null && options.height != null ? { width: options.width, height: options.height } : {}),
  });

  // 秒传：字节已在 OSS，CreateUploadIntent 已落 ready 行并返回 mediaId，无需 PUT/确认。
  if (intent.alreadyComplete) {
    options.onProgress?.({ phase: 'instant', loaded: total, total });
    return { mediaId: intent.mediaId, instant: true };
  }

  options.onProgress?.({ phase: 'uploading', loaded: 0, total });
  await putWithRetry(intent.uploadUrl, file, digest.base64, contentType, total, options);

  options.onProgress?.({ phase: 'finalizing', loaded: total, total });
  const completed = await mediaApi.completeUpload(intent.mediaId);
  return { mediaId: completed.media?.mediaId ?? intent.mediaId, instant: false, media: completed.media };
}

async function putWithRetry(
  uploadUrl: string,
  body: Blob,
  checksumSha256: string,
  contentType: string,
  total: number,
  options: UploadFileToMediaOptions,
): Promise<void> {
  const attempts = Math.max(1, (options.uploadRetries ?? 2) + 1);
  let lastError: unknown;
  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    try {
      await uploadMediaBytes(uploadUrl, body, {
        contentType,
        checksumSha256,
        signal: options.signal,
        fetchImpl: options.fetchImpl,
        // 仅在调用方要进度时透传，让无进度需求的上传走默认 fetch 路径。
        onProgress: options.onProgress
          ? (progress: MediaUploadProgress) =>
              options.onProgress?.({ phase: 'uploading', loaded: progress.loaded, total: progress.total })
          : undefined,
      });
      return;
    } catch (error) {
      // 取消不重试，整体重传无意义。
      if (error instanceof DOMException && error.name === 'AbortError') {
        throw error;
      }
      lastError = error;
      if (attempt < attempts) {
        options.onProgress?.({ phase: 'uploading', loaded: 0, total });
      }
    }
  }
  throw lastError instanceof Error ? lastError : new Error('上传文件失败');
}

export type DownloadMediaOptions = {
  signal?: AbortSignal;
  fetchImpl?: typeof fetch;
};

// downloadMediaBlob 拿整文件 presigned GET URL 后直拉 OSS 还原 Blob（不分片）。
export async function downloadMediaBlob(
  mediaId: string,
  mediaApi: MediaApi,
  options: DownloadMediaOptions = {},
): Promise<Blob> {
  const { downloadUrl } = await mediaApi.getDownloadURL(mediaId);
  const fetchImpl = options.fetchImpl ?? fetch;
  const response = await fetchImpl(downloadUrl, { signal: options.signal });
  if (!response.ok) {
    throw new Error(`下载文件失败 (${response.status})`);
  }
  return response.blob();
}

function fileName(file: Blob): string | undefined {
  const name = (file as File).name;
  return typeof name === 'string' && name.length > 0 ? name : undefined;
}
