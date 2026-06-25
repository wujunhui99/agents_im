import type { ImageMessagePayload, FileMessagePayload, ImageDimensions, AttachmentKind } from '../types';
import type { ChatMessage } from '../../../models/messages';
import { isRecord } from './serverMessageParser';

const allowedImageMimeTypes = new Set(['image/jpeg', 'image/png', 'image/webp', 'image/gif']);

function parseContentObject(content: string) {
  try {
    const parsed = JSON.parse(content) as unknown;
    return isRecord(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function stringField(value: Record<string, unknown>, ...keys: string[]) {
  for (const key of keys) {
    const field = value[key];
    if (typeof field === 'string') return field;
  }
  return undefined;
}

function numberField(value: Record<string, unknown>, ...keys: string[]) {
  for (const key of keys) {
    const field = value[key];
    if (typeof field === 'number' && Number.isFinite(field)) return field;
  }
  return undefined;
}

export function parseImageMessagePayload(content: string): ImageMessagePayload {
  const payload = parseContentObject(content);
  if (!payload) return {};
  return {
    mediaId: stringField(payload, 'mediaId'),
    filename: stringField(payload, 'filename'),
    width: numberField(payload, 'width'),
    height: numberField(payload, 'height'),
    sizeBytes: numberField(payload, 'sizeBytes'),
    contentType: stringField(payload, 'contentType'),
  };
}

export function parseFileMessagePayload(content: string): FileMessagePayload {
  const payload = parseContentObject(content);
  if (!payload) return {};
  return {
    mediaId: stringField(payload, 'mediaId'),
    filename: stringField(payload, 'filename'),
    sizeBytes: numberField(payload, 'sizeBytes'),
    contentType: stringField(payload, 'contentType'),
  };
}

export function imageMessageFilename(payload: ImageMessagePayload) {
  return payload.filename?.trim() || '图片消息';
}

export function imageDisplayLabel(payload: ImageMessagePayload) {
  const filename = payload.filename?.trim();
  return filename ? `图片 ${filename}` : '图片消息';
}

export function fileMessageFilename(payload: FileMessagePayload) {
  return payload.filename?.trim() || '文件消息';
}

export function fileDisplayLabel(payload: FileMessagePayload) {
  const filename = payload.filename?.trim();
  return filename ? `文件 ${filename}` : '文件消息';
}

export function fileMessageMetadata(payload: FileMessagePayload) {
  return [formatFileSize(payload.sizeBytes), payload.contentType?.trim()].filter(Boolean).join(' / ');
}

export function formatFileSize(sizeBytes: number | undefined) {
  if (sizeBytes === undefined || sizeBytes < 0) return undefined;
  if (sizeBytes < 1024) return `${sizeBytes} B`;
  if (sizeBytes < 1024 * 1024) return `${formatFileSizeNumber(sizeBytes / 1024)} KiB`;
  return `${formatFileSizeNumber(sizeBytes / (1024 * 1024))} MiB`;
}

function formatFileSizeNumber(value: number) {
  return Number.isInteger(value) || value >= 10 ? value.toFixed(0) : value.toFixed(1);
}

export function messageDisplayText(message: ChatMessage) {
  if (message.contentType === 'image') return imageDisplayLabel(parseImageMessagePayload(message.content));
  if (message.contentType === 'file') return fileDisplayLabel(parseFileMessagePayload(message.content));
  return message.content;
}

export function uploadFilename(file: File, kind: AttachmentKind) {
  return file.name.trim() || (kind === 'image' ? 'image' : 'file');
}

export function isAllowedMessageImageType(contentType: string) {
  return allowedImageMimeTypes.has(contentType.toLowerCase().trim());
}

export function defaultDownloadMedia(downloadUrl: string, filename: string) {
  const anchor = document.createElement('a');
  anchor.href = downloadUrl;
  anchor.download = filename;
  anchor.rel = 'noopener';
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
}

export async function readImageDimensions(file: File): Promise<ImageDimensions | undefined> {
  if (typeof createImageBitmap === 'function') {
    try {
      const bitmap = await createImageBitmap(file);
      const dimensions = bitmap.width > 0 && bitmap.height > 0 ? { width: bitmap.width, height: bitmap.height } : undefined;
      bitmap.close();
      return dimensions;
    } catch {
      return undefined;
    }
  }

  if (typeof URL === 'undefined' || typeof URL.createObjectURL !== 'function' || typeof Image === 'undefined') {
    return undefined;
  }

  return new Promise((resolve) => {
    const objectUrl = URL.createObjectURL(file);
    const image = new Image();
    const timeout = window.setTimeout(() => { cleanup(); resolve(undefined); }, 250);
    function cleanup() {
      window.clearTimeout(timeout);
      URL.revokeObjectURL(objectUrl);
      image.onload = null;
      image.onerror = null;
    }
    image.onload = () => {
      cleanup();
      const width = image.naturalWidth || image.width;
      const height = image.naturalHeight || image.height;
      resolve(width > 0 && height > 0 ? { width, height } : undefined);
    };
    image.onerror = () => { cleanup(); resolve(undefined); };
    image.src = objectUrl;
  });
}
