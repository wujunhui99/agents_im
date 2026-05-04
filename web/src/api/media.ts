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
  uploadUrl: string;
  expiresAt: number;
};

export type CompleteMediaUploadResponse = {
  media: MediaObject;
};

export type MediaApi = {
  createUploadIntent: (request: CreateMediaUploadRequest) => Promise<CreateMediaUploadResponse>;
  completeUpload: (mediaId: string) => Promise<CompleteMediaUploadResponse>;
};

export function createMediaApi(api: ApiClient = createApiClient()): MediaApi {
  return {
    createUploadIntent(request) {
      return api.post<CreateMediaUploadResponse>('/media/uploads', request);
    },
    completeUpload(mediaId) {
      return api.post<CompleteMediaUploadResponse>(`/media/uploads/${encodeURIComponent(mediaId)}/complete`);
    },
  };
}

export async function uploadMediaBytes(uploadUrl: string, file: File, contentType: string, fetchImpl: typeof fetch = fetch) {
  const response = await fetchImpl(uploadUrl, {
    method: 'PUT',
    body: file,
    headers: {
      'Content-Type': contentType,
    },
  });

  if (!response.ok) {
    throw new Error(`上传文件失败 (${response.status})`);
  }
}
