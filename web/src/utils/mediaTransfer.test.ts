import { describe, expect, it, vi } from 'vitest';
import type {
  CreateMediaUploadRequest,
  CreateMediaUploadResponse,
  MediaApi,
  MediaObject,
} from '../api/media';
import { digestSha256 } from './sha256';
import { downloadMediaBlob, uploadFileToMedia, type MediaUploadProgressEvent } from './mediaTransfer';

function readyMedia(mediaId: string, overrides: Partial<MediaObject> = {}): MediaObject {
  return {
    mediaId,
    ownerUserId: '1001',
    bucket: 'agents_im',
    objectKey: `agents_im/${'a'.repeat(64)}`,
    sha256: 'a'.repeat(64),
    contentType: 'application/pdf',
    sizeBytes: 8,
    originalFilename: 'report.pdf',
    purpose: 'message_file',
    status: 'ready',
    createdAt: '2026-06-18T00:00:00Z',
    updatedAt: '2026-06-18T00:00:00Z',
    ...overrides,
  };
}

function mediaApiWith(intent: (request: CreateMediaUploadRequest) => CreateMediaUploadResponse): MediaApi {
  return {
    createUploadIntent: vi.fn(async (request) => intent(request)),
    completeUpload: vi.fn(async (mediaId) => ({ media: readyMedia(mediaId) })),
    getDownloadURL: vi.fn(async (mediaId) => ({
      mediaId,
      downloadUrl: `https://oss.test/get/${mediaId}`,
      expiresAt: 1_800_000_000_000,
    })),
  };
}

describe('uploadFileToMedia', () => {
  it('uploads whole-file: hashes, PUTs with the checksum header, then confirms', async () => {
    const file = new File([new Uint8Array([1, 2, 3, 4])], 'report.pdf', { type: 'application/pdf' });
    const expected = await digestSha256(file);
    const mediaApi = mediaApiWith(() => ({
      mediaId: 'med_1',
      objectKey: `agents_im/${expected.hex}`,
      uploadUrl: 'https://oss.test/put/tmp',
      expiresAt: 1_800_000_000_000,
    }));
    const uploadFetch = vi.fn<typeof fetch>(async () => new Response('', { status: 200 }));

    const result = await uploadFileToMedia({ file, purpose: 'message_file', mediaApi, fetchImpl: uploadFetch });

    expect(mediaApi.createUploadIntent).toHaveBeenCalledWith({
      purpose: 'message_file',
      filename: 'report.pdf',
      contentType: 'application/pdf',
      sizeBytes: 4,
      sha256: expected.hex,
    });
    expect(uploadFetch).toHaveBeenCalledWith(
      'https://oss.test/put/tmp',
      expect.objectContaining({
        method: 'PUT',
        body: file,
        headers: expect.objectContaining({
          'Content-Type': 'application/pdf',
          'x-amz-checksum-sha256': expected.base64,
        }),
      }),
    );
    expect(mediaApi.completeUpload).toHaveBeenCalledWith('med_1');
    expect(result).toMatchObject({ mediaId: 'med_1', instant: false });
  });

  it('takes the instant (秒传) path without uploading or confirming when alreadyComplete', async () => {
    const file = new File([new Uint8Array([9, 9, 9])], 'dup.png', { type: 'image/png' });
    const mediaApi = mediaApiWith(() => ({
      mediaId: 'med_dup',
      objectKey: `agents_im/${'b'.repeat(64)}`,
      uploadUrl: '',
      expiresAt: 0,
      alreadyComplete: true,
    }));
    const uploadFetch = vi.fn<typeof fetch>(async () => new Response('', { status: 200 }));

    const result = await uploadFileToMedia({ file, purpose: 'message_image', mediaApi, fetchImpl: uploadFetch });

    expect(result).toEqual({ mediaId: 'med_dup', instant: true });
    expect(uploadFetch).not.toHaveBeenCalled();
    expect(mediaApi.completeUpload).not.toHaveBeenCalled();
  });

  it('emits hashing → uploading → finalizing progress phases for a real upload', async () => {
    const file = new File([new Uint8Array(16)], 'big.bin', { type: 'application/octet-stream' });
    const mediaApi = mediaApiWith(() => ({
      mediaId: 'med_p',
      objectKey: 'agents_im/x',
      uploadUrl: 'https://oss.test/put/tmp',
      expiresAt: 0,
    }));
    const events: MediaUploadProgressEvent[] = [];

    await uploadFileToMedia({
      file,
      purpose: 'message_file',
      mediaApi,
      fetchImpl: vi.fn<typeof fetch>(async () => new Response('', { status: 200 })),
      onProgress: (event) => events.push(event),
    });

    const phases = events.map((event) => event.phase);
    expect(phases).toContain('hashing');
    expect(phases).toContain('uploading');
    expect(phases).toContain('finalizing');
    expect(events.every((event) => event.total === 16)).toBe(true);
  });

  it('retries the whole-file PUT on a transient failure (整体重传)', async () => {
    const file = new File([new Uint8Array([7])], 'r.bin', { type: 'application/octet-stream' });
    const mediaApi = mediaApiWith(() => ({
      mediaId: 'med_r',
      objectKey: 'agents_im/x',
      uploadUrl: 'https://oss.test/put/tmp',
      expiresAt: 0,
    }));
    const uploadFetch = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(new Response('', { status: 503 }))
      .mockResolvedValueOnce(new Response('', { status: 200 }));

    const result = await uploadFileToMedia({ file, purpose: 'message_file', mediaApi, fetchImpl: uploadFetch });

    expect(uploadFetch).toHaveBeenCalledTimes(2);
    expect(result.mediaId).toBe('med_r');
  });

  it('gives up after exhausting retries and surfaces the failure', async () => {
    const file = new File([new Uint8Array([7])], 'r.bin', { type: 'application/octet-stream' });
    const mediaApi = mediaApiWith(() => ({
      mediaId: 'med_r',
      objectKey: 'agents_im/x',
      uploadUrl: 'https://oss.test/put/tmp',
      expiresAt: 0,
    }));
    const uploadFetch = vi.fn<typeof fetch>(async () => new Response('', { status: 500 }));

    await expect(
      uploadFileToMedia({ file, purpose: 'message_file', mediaApi, fetchImpl: uploadFetch, uploadRetries: 1 }),
    ).rejects.toThrow(/上传文件失败/);
    expect(uploadFetch).toHaveBeenCalledTimes(2);
    expect(mediaApi.completeUpload).not.toHaveBeenCalled();
  });
});

describe('downloadMediaBlob', () => {
  it('resolves a presigned GET URL then pulls the object bytes from OSS', async () => {
    const mediaApi = mediaApiWith(() => ({ mediaId: 'm', objectKey: 'k', uploadUrl: '', expiresAt: 0 }));
    const payload = new Blob([new Uint8Array([1, 2, 3])]);
    // 用轻量假 Response 而非真 Response(Blob)：undici 在读 body 时要求 body.stream()，
    // 而跨 realm 的 jsdom Blob 在 CI 下没有该方法，会误伤断言。
    const downloadFetch = vi.fn<typeof fetch>(async () => ({ ok: true, blob: async () => payload }) as Response);

    const blob = await downloadMediaBlob('med_x', mediaApi, { fetchImpl: downloadFetch });

    expect(mediaApi.getDownloadURL).toHaveBeenCalledWith('med_x');
    expect(downloadFetch).toHaveBeenCalledWith('https://oss.test/get/med_x', expect.anything());
    expect(blob).toBe(payload);
  });

  it('throws when OSS rejects the presigned GET', async () => {
    const mediaApi = mediaApiWith(() => ({ mediaId: 'm', objectKey: 'k', uploadUrl: '', expiresAt: 0 }));
    const downloadFetch = vi.fn<typeof fetch>(async () => ({ ok: false, status: 403 }) as Response);

    await expect(downloadMediaBlob('med_x', mediaApi, { fetchImpl: downloadFetch })).rejects.toThrow(
      '下载文件失败 (403)',
    );
  });
});
