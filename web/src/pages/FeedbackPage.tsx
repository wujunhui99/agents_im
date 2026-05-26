import { useState, type ChangeEvent, type FormEvent } from 'react';
import { CircleHelp, ImagePlus, Lightbulb, MessageSquareWarning, Sparkles, type LucideIcon } from 'lucide-react';
import type { SubmitFeedbackRequest } from '../api/feedback';
import { createMediaApi, uploadMediaBytes, type MediaApi } from '../api/media';
import { Button } from '../components/ui/Button';
import { ListCard } from '../components/ui/ListCard';
import { TextField } from '../components/ui/TextField';

type FeedbackPageProps = {
  onSubmitFeedback?: (request: SubmitFeedbackRequest) => Promise<void>;
  mediaApi?: MediaApi;
  uploadFetch?: typeof fetch;
};

type FeedbackDraft = {
  category: string;
  title: string;
  content: string;
  contact: string;
};

type FeedbackCategoryOption = {
  value: string;
  label: string;
  helper: string;
  icon: LucideIcon;
};

type UploadedFeedbackAttachment = {
  mediaId: string;
  filename: string;
  sizeBytes: number;
  contentType: string;
};

const feedbackCategories: FeedbackCategoryOption[] = [
  {
    value: 'bug',
    label: '问题反馈',
    helper: '功能异常、无法使用、报错',
    icon: MessageSquareWarning,
  },
  {
    value: 'poor_experience',
    label: '体验建议',
    helper: '流程不顺、界面不清晰、操作不方便',
    icon: Sparkles,
  },
  {
    value: 'feature_request',
    label: '功能建议',
    helper: '希望增加新的能力或入口',
    icon: Lightbulb,
  },
  {
    value: 'other',
    label: '其他反馈',
    helper: '其他想告诉我们的内容',
    icon: CircleHelp,
  },
];

function defaultFeedbackDraft(): FeedbackDraft {
  return { category: 'bug', title: '', content: '', contact: '' };
}

export function FeedbackPage({ onSubmitFeedback, mediaApi = createMediaApi(), uploadFetch = fetch }: FeedbackPageProps) {
  const [feedbackDraft, setFeedbackDraft] = useState<FeedbackDraft>(() => defaultFeedbackDraft());
  const [feedbackStatus, setFeedbackStatus] = useState('');
  const [feedbackError, setFeedbackError] = useState('');
  const [feedbackSubmitting, setFeedbackSubmitting] = useState(false);
  const [feedbackAttachments, setFeedbackAttachments] = useState<File[]>([]);

  function handleFeedbackAttachmentChange(event: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.currentTarget.files ?? []).filter((file) => file.type.startsWith('image/'));
    event.currentTarget.value = '';
    setFeedbackAttachments(files.slice(0, 4));
  }

  async function uploadFeedbackAttachments(): Promise<UploadedFeedbackAttachment[]> {
    const uploaded: UploadedFeedbackAttachment[] = [];
    for (const file of feedbackAttachments) {
      const contentType = file.type || 'application/octet-stream';
      const intent = await mediaApi.createUploadIntent({
        purpose: 'message_image',
        filename: file.name,
        contentType,
        sizeBytes: file.size,
      });
      await uploadMediaBytes(intent.uploadUrl, file, contentType, uploadFetch);
      const completed = await mediaApi.completeUpload(intent.mediaId);
      uploaded.push({
        mediaId: completed.media?.mediaId ?? intent.mediaId,
        filename: file.name,
        sizeBytes: file.size,
        contentType,
      });
    }
    return uploaded;
  }

  async function handleFeedbackSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFeedbackStatus('');
    setFeedbackError('');
    if (!onSubmitFeedback) {
      setFeedbackError('反馈功能暂不可用');
      return;
    }
    setFeedbackSubmitting(true);
    try {
      const uploadedAttachments = await uploadFeedbackAttachments();
      await onSubmitFeedback({
        category: feedbackDraft.category,
        title: feedbackDraft.title.trim(),
        content: feedbackDraft.content.trim(),
        contact: feedbackDraft.contact.trim(),
        pageUrl: window.location.href,
        userAgent: navigator.userAgent,
        clientMeta: {
          attachmentFileNames: feedbackAttachments.map((file) => file.name),
          attachmentCount: feedbackAttachments.length,
          attachmentUploadStatus: uploadedAttachments.length === feedbackAttachments.length ? 'uploaded' : 'none',
          attachments: uploadedAttachments,
        },
      });
      setFeedbackDraft(defaultFeedbackDraft());
      setFeedbackAttachments([]);
      setFeedbackStatus('反馈已提交，感谢你的帮助');
    } catch {
      setFeedbackError('反馈提交失败，请稍后重试');
    } finally {
      setFeedbackSubmitting(false);
    }
  }

  return (
    <div className="page-stack feedback-page">
      <ListCard ariaLabel="意见反馈表单" className="feedback-card">
        <form className="feedback-form" onSubmit={handleFeedbackSubmit}>
          <div className="feedback-form-head">
            <span className="feedback-kicker">意见反馈</span>
            <h2>请选择反馈类型</h2>
            <p>我们会把页面信息和设备信息随反馈一并记录，便于定位问题。</p>
          </div>

          <div className="feedback-category-grid" role="radiogroup" aria-label="反馈类型">
            {feedbackCategories.map((category) => {
              const Icon = category.icon;
              const selected = feedbackDraft.category === category.value;
              return (
                <label className={`feedback-category-option${selected ? ' is-selected' : ''}`} key={category.value}>
                  <input
                    type="radio"
                    name="feedback-category"
                    value={category.value}
                    checked={selected}
                    onChange={() => setFeedbackDraft((current) => ({ ...current, category: category.value }))}
                  />
                  <span className="feedback-category-icon" aria-hidden="true">
                    <Icon size={18} />
                  </span>
                  <span className="feedback-category-copy">
                    <strong>{category.label}</strong>
                    <small>{category.helper}</small>
                  </span>
                </label>
              );
            })}
          </div>

          <TextField
            label="反馈标题"
            value={feedbackDraft.title}
            onChange={(event) => setFeedbackDraft((current) => ({ ...current, title: event.target.value }))}
            fieldClassName="feedback-field"
            placeholder="一句话描述你遇到的问题或建议"
            required
          />
          <label className="feedback-field feedback-textarea-field">
            <span>详细说明</span>
            <textarea
              aria-label="详细说明"
              value={feedbackDraft.content}
              onChange={(event) => setFeedbackDraft((current) => ({ ...current, content: event.target.value }))}
              placeholder="请补充发生场景、期望效果或复现步骤"
              rows={5}
              required
            />
            <small>建议包含操作路径、出现时间和你期望的结果。</small>
          </label>
          <TextField
            label="联系方式（选填）"
            value={feedbackDraft.contact}
            onChange={(event) => setFeedbackDraft((current) => ({ ...current, contact: event.target.value }))}
            fieldClassName="feedback-field"
            placeholder="邮箱、微信或其他便于联系的方式"
          />

          <label className="feedback-attachment-picker">
            <span className="feedback-attachment-icon" aria-hidden="true">
              <ImagePlus size={20} />
            </span>
            <span>
              <strong>添加截图或图片</strong>
              <small>最多 4 张，图片会随反馈上传并展示在后台</small>
            </span>
            <input
              className="sr-only"
              type="file"
              accept="image/jpeg,image/png,image/webp"
              multiple
              aria-label="添加截图或图片"
              onChange={handleFeedbackAttachmentChange}
            />
          </label>

          {feedbackAttachments.length > 0 ? (
            <ul className="feedback-attachment-list" aria-label="已选择图片">
              {feedbackAttachments.map((file) => (
                <li key={`${file.name}-${file.size}`}>{file.name}</li>
              ))}
            </ul>
          ) : null}

          {feedbackError ? (
            <p className="form-error" role="alert">
              {feedbackError}
            </p>
          ) : null}
          {feedbackStatus ? (
            <p className="profile-avatar-status" role="status">
              {feedbackStatus}
            </p>
          ) : null}
          <div className="profile-form-actions feedback-actions">
            <Button type="submit" className="primary-button" disabled={feedbackSubmitting}>
              {feedbackSubmitting ? '提交中' : '提交反馈'}
            </Button>
          </div>
        </form>
      </ListCard>
    </div>
  );
}
