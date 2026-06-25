import { createApiClient, type ApiClient } from './client';

export type MediaPurpose = 'avatar' | 'message_image' | 'message_file';

export type MediaObject = {
  mediaId: string;
  ownerUserId: string;
  bucket: string;
  objectKey: string;
  sha256: string;
  contentType: string;
  sizeBytes: number;
  width?: number;
  height?: number;
  originalFilename: string;
  purpose: MediaPurpose | string;
  status: string;
  createdAt: string;
  updatedAt: string;
};

export type CreateMediaUploadRequest = {
  purpose: MediaPurpose;
  filename: string;
  contentType: string;
  sizeBytes: number;
  sha256?: string;
  width?: number;
  height?: number;
};

export type CreateMediaUploadResponse = {
  mediaId: string;
  objectKey: string;
  /** 秒传命中时为空 —— 字节已在 OSS，无需直传。 */
  uploadUrl: string;
  expiresAt: number;
  /** 文件级秒传命中（object_key 已有 ready 行）：直接完成，无需 PUT/确认。 */
  alreadyComplete?: boolean;
};

export type CompleteMediaUploadResponse = {
  media: MediaObject;
};

export type GetMediaDownloadURLResponse = {
  mediaId: string;
  downloadUrl: string;
  expiresAt: number;
};

export type GetMediaDownloadURLOptions = {
  msgId?: string;
};

export type MediaApi = {
  createUploadIntent: (request: CreateMediaUploadRequest) => Promise<CreateMediaUploadResponse>;
  completeUpload: (mediaId: string) => Promise<CompleteMediaUploadResponse>;
  getDownloadURL: (mediaId: string, options?: GetMediaDownloadURLOptions) => Promise<GetMediaDownloadURLResponse>;
};

export function createMediaApi(api: ApiClient = createApiClient()): MediaApi {
  return {
    createUploadIntent(request) {
      return api.post<CreateMediaUploadResponse>('/media/uploads', request);
    },
    completeUpload(mediaId) {
      return api.post<CompleteMediaUploadResponse>(`/media/uploads/${encodeURIComponent(mediaId)}/complete`);
    },
    getDownloadURL(mediaId, options) {
      const params = new URLSearchParams();
      const msgId = options?.msgId?.trim();
      if (msgId) {
        params.set('msg_id', msgId);
      }
      const query = params.toString();
      return api.get<GetMediaDownloadURLResponse>(`/media/${encodeURIComponent(mediaId)}/download-url${query ? `?${query}` : ''}`);
    },
  };
}

export type MediaUploadProgress = { loaded: number; total: number };

export type UploadMediaBytesOptions = {
  contentType: string;
  /** 整文件 SHA-256 的 base64，回放到 x-amz-checksum-sha256（presigned PUT 已把它烤进签名）。 */
  checksumSha256?: string;
  onProgress?: (progress: MediaUploadProgress) => void;
  signal?: AbortSignal;
  /** 注入用于测试；提供时走 fetch（无原生上传进度，仅在完成时回报 100%）。 */
  fetchImpl?: typeof fetch;
};

// uploadMediaBytes 整文件直传 OSS（不分片）。presigned PUT 把 Content-Type 与
// x-amz-checksum-sha256 烤进 SigV4 签名，客户端必须原样回放这两个头，否则 OSS 拒签/校验失败
// （EPIC #527 §3）。默认走 XHR 以拿到真实上传进度；注入 fetchImpl 或无 XHR 时退回 fetch。
export async function uploadMediaBytes(
  uploadUrl: string,
  body: Blob,
  options: UploadMediaBytesOptions,
): Promise<void> {
  const headers: Record<string, string> = { 'Content-Type': options.contentType };
  if (options.checksumSha256) {
    headers['x-amz-checksum-sha256'] = options.checksumSha256;
  }

  // fetch 无法回报上传进度：仅当调用方要进度、且未注入 fetchImpl 时才走 XHR 拿真实进度。
  const canUseXhr = !options.fetchImpl && !!options.onProgress && typeof XMLHttpRequest !== 'undefined';
  if (canUseXhr) {
    return uploadViaXhr(uploadUrl, body, headers, options);
  }

  const fetchImpl = options.fetchImpl ?? fetch;
  const response = await fetchImpl(uploadUrl, {
    method: 'PUT',
    body,
    headers,
    signal: options.signal,
  });
  if (!response.ok) {
    throw new Error(`上传文件失败 (${response.status})`);
  }
  options.onProgress?.({ loaded: body.size, total: body.size });
}

function uploadViaXhr(
  uploadUrl: string,
  body: Blob,
  headers: Record<string, string>,
  options: UploadMediaBytesOptions,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('PUT', uploadUrl);
    for (const [name, value] of Object.entries(headers)) {
      xhr.setRequestHeader(name, value);
    }
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) {
        options.onProgress?.({ loaded: event.loaded, total: event.total });
      }
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        options.onProgress?.({ loaded: body.size, total: body.size });
        resolve();
        return;
      }
      reject(new Error(`上传文件失败 (${xhr.status})`));
    };
    xhr.onerror = () => reject(new Error('上传文件失败（网络错误）'));
    xhr.onabort = () => reject(new DOMException('上传已取消', 'AbortError'));
    if (options.signal) {
      if (options.signal.aborted) {
        xhr.abort();
        return;
      }
      options.signal.addEventListener('abort', () => xhr.abort(), { once: true });
    }
    xhr.send(body);
  });
}
