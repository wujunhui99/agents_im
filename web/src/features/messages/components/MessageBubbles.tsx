import { Download, FileText, Image as ImageIcon, RefreshCw, X } from 'lucide-react';
import { useEffect, useMemo, useState, type ReactNode } from 'react';
import type { MediaApi } from '../../../api/media';
import { Button } from '../../../components/ui/Button';
import type { ChatMessage } from '../../../models/messages';
import type { MediaDownloadHandler } from '../types';
import {
  fileDisplayLabel,
  fileMessageFilename,
  fileMessageMetadata,
  imageDisplayLabel,
  imageMessageFilename,
  parseFileMessagePayload,
  parseImageMessagePayload,
} from '../utils/mediaUtils';

export function FileMessageBubble({
  message,
  mediaApi,
  downloadMedia,
  onStatus,
  metadata,
}: {
  message: ChatMessage;
  mediaApi: MediaApi;
  downloadMedia: MediaDownloadHandler;
  onStatus: (status: string) => void;
  metadata: ReactNode;
}) {
  const payload = useMemo(() => parseFileMessagePayload(message.content), [message.content]);
  const mediaId = payload.mediaId;
  const filename = fileMessageFilename(payload);
  const label = fileDisplayLabel(payload);
  const fileMeta = fileMessageMetadata(payload);
  const [downloadError, setDownloadError] = useState('');
  const [downloading, setDownloading] = useState(false);

  async function handleDownload() {
    if (!mediaId) {
      const msg = '文件信息缺失，无法下载';
      setDownloadError(msg);
      onStatus(msg);
      return;
    }
    setDownloading(true);
    setDownloadError('');
    try {
      const result = await mediaApi.getDownloadURL(mediaId);
      downloadMedia(result.downloadUrl, filename);
      onStatus('已获取文件下载链接');
    } catch {
      const msg = '下载文件失败，请稍后重试';
      setDownloadError(msg);
      onStatus(msg);
    } finally {
      setDownloading(false);
    }
  }

  return (
    <div className={`file-message-card file-message-card-${message.direction}`}>
      <div className="file-message-content">
        <span className="file-message-icon" aria-hidden="true"><FileText size={22} /></span>
        <span className="file-message-main">
          <span className="file-message-title">{label}</span>
          {fileMeta ? <span className="file-message-metadata">{fileMeta}</span> : null}
        </span>
        <span className="file-message-actions">
          <Button
            variant="icon"
            size="small"
            className="file-download-button"
            type="button"
            aria-label={`下载${label}`}
            onClick={handleDownload}
            disabled={downloading}
          >
            <Download size={16} />
          </Button>
          <span className="file-message-status">{metadata}</span>
        </span>
      </div>
      {downloadError ? <p className="file-message-error" role="alert">{downloadError}</p> : null}
    </div>
  );
}

export function ImageMessageBubble({
  message,
  mediaApi,
  downloadMedia,
  onStatus,
  metadata,
}: {
  message: ChatMessage;
  mediaApi: MediaApi;
  downloadMedia: MediaDownloadHandler;
  onStatus: (status: string) => void;
  metadata: ReactNode;
}) {
  const payload = useMemo(() => parseImageMessagePayload(message.content), [message.content]);
  const mediaId = payload.mediaId;
  const filename = imageMessageFilename(payload);
  const label = imageDisplayLabel(payload);
  const [imageUrl, setImageUrl] = useState('');
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState('');
  const [downloadError, setDownloadError] = useState('');
  const [downloading, setDownloading] = useState(false);
  const [previewOpen, setPreviewOpen] = useState(false);

  useEffect(() => {
    let cancelled = false;
    if (!mediaId) {
      setImageUrl('');
      setLoadError('图片信息缺失，无法加载');
      return () => { cancelled = true; };
    }
    setLoading(true);
    setLoadError('');
    mediaApi
      .getDownloadURL(mediaId)
      .then((result) => { if (!cancelled) setImageUrl(result.downloadUrl); })
      .catch(() => {
        if (!cancelled) {
          const msg = '图片加载失败，请稍后重试';
          setLoadError(msg);
          onStatus(msg);
        }
      })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [mediaApi, mediaId, onStatus]);

  async function retryLoad() {
    if (!mediaId || loading) return;
    setLoading(true);
    setLoadError('');
    try {
      const result = await mediaApi.getDownloadURL(mediaId);
      setImageUrl(result.downloadUrl);
    } catch {
      const msg = '图片加载失败，请稍后重试';
      setLoadError(msg);
      onStatus(msg);
    } finally {
      setLoading(false);
    }
  }

  async function handleDownload() {
    if (!mediaId) {
      const msg = '图片信息缺失，无法下载';
      setDownloadError(msg);
      onStatus(msg);
      return;
    }
    setDownloading(true);
    setDownloadError('');
    try {
      const result = await mediaApi.getDownloadURL(mediaId);
      downloadMedia(result.downloadUrl, filename);
      onStatus('已获取图片下载链接');
    } catch {
      const msg = '下载图片失败，请稍后重试';
      setDownloadError(msg);
      onStatus(msg);
    } finally {
      setDownloading(false);
    }
  }

  return (
    <>
      <div className={`image-message-card image-message-card-${message.direction}`}>
        <div className="image-message-frame">
          {imageUrl ? (
            <button className="image-preview-button" type="button" aria-label={`预览${label}`} onClick={() => setPreviewOpen(true)}>
              <img src={imageUrl} alt={label} />
            </button>
          ) : (
            <button
              className="image-preview-button image-preview-placeholder"
              type="button"
              aria-label={loadError ? `重新加载${label}` : label}
              onClick={retryLoad}
              disabled={!mediaId || loading}
            >
              <ImageIcon size={22} />
              <span>{loading ? '正在加载图片' : loadError || '图片信息缺失，无法加载'}</span>
              {loadError ? <RefreshCw size={14} /> : null}
            </button>
          )}
        </div>
        <div className="image-message-actions">
          <span className="image-message-filename">{filename}</span>
          <Button
            variant="icon"
            size="small"
            className="image-download-button"
            type="button"
            aria-label={`下载${label}`}
            onClick={handleDownload}
            disabled={downloading}
          >
            <Download size={16} />
          </Button>
          <span className="image-message-status">{metadata}</span>
        </div>
        {downloadError ? <p className="image-message-error" role="alert">{downloadError}</p> : null}
      </div>
      {previewOpen ? (
        <ImagePreviewDialog
          imageUrl={imageUrl}
          label={label}
          filename={filename}
          onClose={() => setPreviewOpen(false)}
          onDownload={handleDownload}
          downloading={downloading}
          error={downloadError || loadError}
        />
      ) : null}
    </>
  );
}

function ImagePreviewDialog({
  imageUrl,
  label,
  filename,
  onClose,
  onDownload,
  downloading,
  error,
}: {
  imageUrl: string;
  label: string;
  filename: string;
  onClose: () => void;
  onDownload: () => void;
  downloading: boolean;
  error: string;
}) {
  return (
    <div className="image-preview-overlay" role="dialog" aria-modal="true" aria-label="图片预览">
      <div className="image-preview-toolbar">
        <span>{filename}</span>
        <div className="image-preview-actions">
          <Button variant="icon" type="button" aria-label={`下载${label}`} onClick={onDownload} disabled={downloading}>
            <Download size={18} />
          </Button>
          <Button variant="icon" type="button" aria-label="关闭预览" onClick={onClose}>
            <X size={20} />
          </Button>
        </div>
      </div>
      <div className="image-preview-content">
        {imageUrl ? <img src={imageUrl} alt={`预览${label}`} /> : <p>{error || '图片加载中'}</p>}
      </div>
      {error ? <p className="image-preview-error" role="alert">{error}</p> : null}
    </div>
  );
}
