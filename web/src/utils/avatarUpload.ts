import type { MediaApi } from '../api/media';
import { uploadMediaBytes } from '../api/media';
import type { UserApi, UserProfile } from '../api/user';

export const AVATAR_MAX_BYTES = 5 * 1024 * 1024;
export const AVATAR_COMPRESS_THRESHOLD_BYTES = 3 * 1024 * 1024;
export const AVATAR_TARGET_BYTES = 256 * 1024;
export const AVATAR_MAX_DIMENSION = 512;

const allowedAvatarTypes = new Set(['image/jpeg', 'image/png', 'image/webp']);
const compressionQualities = [0.82, 0.72, 0.62, 0.52];

export type PreparedAvatar = {
  file: File;
  width?: number;
  height?: number;
  compressed: boolean;
};

export type UploadAvatarOptions = {
  file: File;
  mediaApi: MediaApi;
  userApi: Pick<UserApi, 'patchCurrentUserAvatar'>;
  fetchImpl?: typeof fetch;
};

type LoadedAvatarImage = {
  source: CanvasImageSource;
  width: number;
  height: number;
  close?: () => void;
};

export async function uploadAvatarForProfile({
  file,
  mediaApi,
  userApi,
  fetchImpl = fetch,
}: UploadAvatarOptions): Promise<UserProfile> {
  const prepared = await prepareAvatarFile(file);
  const uploadIntent = await mediaApi.createUploadIntent({
    purpose: 'avatar',
    filename: prepared.file.name || avatarFilename(file),
    contentType: prepared.file.type,
    sizeBytes: prepared.file.size,
    ...(prepared.width && prepared.height ? { width: prepared.width, height: prepared.height } : {}),
  });

  await uploadMediaBytes(uploadIntent.uploadUrl, prepared.file, prepared.file.type, fetchImpl);
  const completed = await mediaApi.completeUpload(uploadIntent.mediaId);
  const mediaId = completed.media?.mediaId ?? uploadIntent.mediaId;
  return userApi.patchCurrentUserAvatar(mediaId);
}

export async function prepareAvatarFile(file: File): Promise<PreparedAvatar> {
  validateAvatarFile(file);

  const loaded = await loadAvatarImage(file);
  if (!loaded) {
    return { file, compressed: false };
  }

  try {
    const target = fitWithinSquare(loaded.width, loaded.height, AVATAR_MAX_DIMENSION);
    const shouldCompress =
      file.size > AVATAR_COMPRESS_THRESHOLD_BYTES || loaded.width > AVATAR_MAX_DIMENSION || loaded.height > AVATAR_MAX_DIMENSION;
    if (!shouldCompress) {
      return { file, width: loaded.width, height: loaded.height, compressed: false };
    }

    const compressed = await compressAvatar(file, loaded.source, target.width, target.height);
    if (!compressed || compressed.size > AVATAR_MAX_BYTES || compressed.size >= file.size) {
      return { file, width: loaded.width, height: loaded.height, compressed: false };
    }

    return {
      file: new File([compressed], avatarFilename(file, 'jpg'), { type: compressed.type || 'image/jpeg' }),
      width: target.width,
      height: target.height,
      compressed: true,
    };
  } finally {
    loaded.close?.();
  }
}

function validateAvatarFile(file: File) {
  const contentType = file.type.toLowerCase();
  if (!allowedAvatarTypes.has(contentType)) {
    throw new Error('头像仅支持 JPG、PNG 或 WebP');
  }
  if (file.size > AVATAR_MAX_BYTES) {
    throw new Error('头像不能超过 5 MiB');
  }
}

async function loadAvatarImage(file: File): Promise<LoadedAvatarImage | null> {
  if (typeof createImageBitmap === 'function') {
    try {
      const bitmap = await createImageBitmap(file);
      if (bitmap.width > 0 && bitmap.height > 0) {
        return {
          source: bitmap,
          width: bitmap.width,
          height: bitmap.height,
          close: () => bitmap.close(),
        };
      }
      bitmap.close();
    } catch {
      return null;
    }
  }

  if (typeof document === 'undefined' || typeof Image === 'undefined' || typeof URL === 'undefined' || !URL.createObjectURL) {
    return null;
  }

  return new Promise((resolve) => {
    const objectUrl = URL.createObjectURL(file);
    const image = new Image();
    const timeout = window.setTimeout(() => {
      cleanup();
      resolve(null);
    }, 500);
    function cleanup() {
      window.clearTimeout(timeout);
      URL.revokeObjectURL(objectUrl);
      image.onload = null;
      image.onerror = null;
    }
    image.onload = () => {
      const width = image.naturalWidth || image.width;
      const height = image.naturalHeight || image.height;
      cleanup();
      resolve(width > 0 && height > 0 ? { source: image, width, height } : null);
    };
    image.onerror = () => {
      cleanup();
      resolve(null);
    };
    image.src = objectUrl;
  });
}

function fitWithinSquare(width: number, height: number, maxDimension: number) {
  const scale = Math.min(1, maxDimension / Math.max(width, height));
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
  };
}

async function compressAvatar(file: File, source: CanvasImageSource, width: number, height: number): Promise<Blob | null> {
  if (typeof document === 'undefined') {
    return null;
  }
  const canvas = document.createElement('canvas');
  const context = canvas.getContext('2d');
  if (!context || typeof canvas.toBlob !== 'function') {
    return null;
  }

  canvas.width = width;
  canvas.height = height;
  context.drawImage(source, 0, 0, width, height);

  const outputType = file.type === 'image/webp' ? 'image/webp' : 'image/jpeg';
  for (const quality of compressionQualities) {
    const blob = await canvasToBlob(canvas, outputType, quality);
    if (!blob) {
      continue;
    }
    if (blob.size <= AVATAR_TARGET_BYTES || quality === compressionQualities[compressionQualities.length - 1]) {
      return blob;
    }
  }
  return null;
}

function canvasToBlob(canvas: HTMLCanvasElement, type: string, quality: number) {
  return new Promise<Blob | null>((resolve) => {
    canvas.toBlob((blob) => resolve(blob), type, quality);
  });
}

function avatarFilename(file: File, extension?: string) {
  const trimmed = file.name.trim();
  const fallback = `avatar.${extension ?? extensionForContentType(file.type)}`;
  if (!trimmed) {
    return fallback;
  }
  if (!extension) {
    return trimmed;
  }
  return trimmed.replace(/\.[^.]+$/, '') + `.${extension}`;
}

function extensionForContentType(contentType: string) {
  switch (contentType.toLowerCase()) {
    case 'image/png':
      return 'png';
    case 'image/webp':
      return 'webp';
    default:
      return 'jpg';
  }
}
