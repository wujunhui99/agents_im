import { ChangeEvent, FormEvent, useEffect, useState } from 'react';
import { CircleHelp, ImagePlus, Lightbulb, MessageSquareWarning, Settings, Sparkles, type LucideIcon } from 'lucide-react';
import type { SubmitFeedbackRequest } from '../api/feedback';
import type { UserProfile, UserProfilePatch } from '../api/user';
import { ActionRow } from '../components/ui/ActionRow';
import { Avatar } from '../components/ui/Avatar';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { ListCard } from '../components/ui/ListCard';
import { TextField } from '../components/ui/TextField';
import { accountTypeLabel, avatarText, firstNonEmpty, genderLabel, profileDisplayName } from '../utils/profileDisplay';

type ProfileDraft = {
  display_name: string;
  gender: string;
  birth_date: string;
  region: string;
};

type MePageProps = {
  profile: UserProfile;
  onUpdateProfile: (patch: UserProfilePatch) => Promise<void>;
  onUploadAvatar: (file: File) => Promise<UserProfile>;
  onAvatarUpdated?: (profile: UserProfile) => void;
  onSubmitFeedback?: (request: SubmitFeedbackRequest) => Promise<void>;
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

export function MePage({ profile, onUpdateProfile, onUploadAvatar, onAvatarUpdated, onSubmitFeedback }: MePageProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [draft, setDraft] = useState<ProfileDraft>(() => createDraft(profile));
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState('');
  const [avatarStatus, setAvatarStatus] = useState('');
  const [isUploadingAvatar, setIsUploadingAvatar] = useState(false);
  const [avatarURL, setAvatarURL] = useState(profile.avatar_url ?? '');
  const [feedbackOpen, setFeedbackOpen] = useState(false);
  const [feedbackDraft, setFeedbackDraft] = useState<FeedbackDraft>(() => defaultFeedbackDraft());
  const [feedbackStatus, setFeedbackStatus] = useState('');
  const [feedbackError, setFeedbackError] = useState('');
  const [feedbackSubmitting, setFeedbackSubmitting] = useState(false);
  const [feedbackAttachments, setFeedbackAttachments] = useState<File[]>([]);
  const visibleProfile = { ...profile, avatar_url: avatarURL };

  useEffect(() => {
    setDraft(createDraft(profile));
  }, [profile]);

  useEffect(() => {
    setAvatarURL(profile.avatar_url ?? '');
  }, [profile.avatar_url]);

  async function handleAvatarChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.currentTarget.files?.[0];
    event.currentTarget.value = '';
    if (!file || isUploadingAvatar) {
      return;
    }

    setAvatarStatus('正在上传头像');
    setIsUploadingAvatar(true);
    try {
      const updatedProfile = await onUploadAvatar(file);
      setAvatarURL(updatedProfile.avatar_url ?? '');
      onAvatarUpdated?.(updatedProfile);
      setAvatarStatus('头像已更新');
    } catch (caughtError) {
      const detail = caughtError instanceof Error ? caughtError.message : '';
      setAvatarStatus(detail ? `头像上传失败：${detail}` : '头像上传失败，请稍后重试');
    } finally {
      setIsUploadingAvatar(false);
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');
    setIsSaving(true);

    try {
      await onUpdateProfile({
        display_name: draft.display_name.trim(),
        gender: draft.gender.trim(),
        birth_date: draft.birth_date.trim(),
        region: draft.region.trim(),
      });
      setIsEditing(false);
    } catch {
      setError('保存失败，请稍后重试');
    } finally {
      setIsSaving(false);
    }
  }

  function cancelEdit() {
    setDraft(createDraft(profile));
    setError('');
    setIsEditing(false);
  }

  function handleFeedbackAttachmentChange(event: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.currentTarget.files ?? []).filter((file) => file.type.startsWith('image/'));
    event.currentTarget.value = '';
    setFeedbackAttachments(files.slice(0, 4));
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
          attachmentUploadStatus: 'local_selection_only',
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
    <div className="page-stack me-page">
      <Card className="profile-card" variant="elevated">
        <div className="profile-avatar-stack">
          <Avatar
            label={profileInitial(visibleProfile)}
            color="green"
            size="large"
            src={visibleProfile.avatar_url}
            alt={`${profileDisplayName(visibleProfile)} 头像`}
            onImageError={() => setAvatarStatus('头像加载失败，已显示默认头像')}
          />
          <label className={`avatar-upload-control${isUploadingAvatar ? ' is-disabled' : ''}`}>
            <span>{isUploadingAvatar ? '上传中' : '更换头像'}</span>
            <input
              className="sr-only"
              type="file"
              accept="image/jpeg,image/png,image/webp"
              aria-label="上传头像"
              disabled={isUploadingAvatar}
              onChange={handleAvatarChange}
            />
          </label>
        </div>
        <div className="profile-main">
          <strong>{profileDisplayName(visibleProfile)}</strong>
          <p>账号：{visibleProfile.identifier}</p>
          <p>账号类型：{accountTypeLabel(visibleProfile.account_type)}</p>
          <p>地区：{visibleProfile.region}</p>
          {avatarStatus ? (
            <p className="profile-avatar-status" role="status">
              {avatarStatus}
            </p>
          ) : null}
        </div>
        <Button variant="tonal" size="small" className="profile-edit-button" aria-label="编辑个人资料" onClick={() => setIsEditing(true)}>
          <span>编辑资料</span>
        </Button>
      </Card>

      <ListCard ariaLabel="个人资料详情" className="profile-detail-card">
        <dl className="profile-detail-list">
          <div>
            <dt>账号</dt>
            <dd>{profile.identifier}</dd>
          </div>
          <div>
            <dt>昵称</dt>
            <dd>{profileDisplayName(profile)}</dd>
          </div>
          <div>
            <dt>账号类型</dt>
            <dd>{accountTypeLabel(profile.account_type)}</dd>
          </div>
          <div>
            <dt>性别</dt>
            <dd>{genderLabel(profile.gender)}</dd>
          </div>
          <div>
            <dt>生日</dt>
            <dd>{profile.birth_date || '未设置'}</dd>
          </div>
          <div>
            <dt>地区</dt>
            <dd>{profile.region}</dd>
          </div>
        </dl>
      </ListCard>

      {isEditing ? (
        <ListCard ariaLabel="编辑个人资料" className="profile-edit-card">
          <form className="profile-edit-form" onSubmit={handleSubmit}>
            <TextField
              label="昵称"
              value={draft.display_name}
              onChange={(event) => setDraft((current) => ({ ...current, display_name: event.target.value }))}
              fieldClassName="profile-field"
            />
            <TextField
              label="性别"
              value={draft.gender}
              onChange={(event) => setDraft((current) => ({ ...current, gender: event.target.value }))}
              fieldClassName="profile-field"
            />
            <TextField
              label="生日"
              type="date"
              value={draft.birth_date}
              onChange={(event) => setDraft((current) => ({ ...current, birth_date: event.target.value }))}
              fieldClassName="profile-field"
            />
            <TextField
              label="地区"
              value={draft.region}
              onChange={(event) => setDraft((current) => ({ ...current, region: event.target.value }))}
              fieldClassName="profile-field"
            />
            {error ? <p className="form-error">{error}</p> : null}
            <div className="profile-form-actions">
              <Button variant="tonal" className="secondary-button" onClick={cancelEdit}>
                取消
              </Button>
              <Button type="submit" className="primary-button" disabled={isSaving}>
                {isSaving ? '保存中' : '保存'}
              </Button>
            </div>
          </form>
        </ListCard>
      ) : null}

      <ListCard>
        <ActionRow label="服务" helper="钱包、收藏、卡包等能力预留" accent="green" />
      </ListCard>
      <button
        className="md-card md-card-elevated feedback-hero-card"
        type="button"
        aria-label="帮助我们改进 Agents IM，提交问题反馈、体验建议和功能建议"
        onClick={() => setFeedbackOpen((current) => !current)}
      >
        <span className="feedback-hero-icon" aria-hidden="true">
          <MessageSquareWarning size={24} />
        </span>
        <span className="feedback-hero-copy">
          <strong>帮助我们改进 Agents IM</strong>
          <span>提交问题、体验建议或功能需求，可附截图说明。</span>
        </span>
        <span className="feedback-hero-action">{feedbackOpen ? '收起' : '反馈'}</span>
      </button>

      <ListCard>
        <ActionRow label="收藏" helper="重要消息和 Agent 输出" accent="orange" />
        <ActionRow label="设置" helper="账号、安全、通知" accent="gray" trailingIcon={Settings} />
      </ListCard>

      {feedbackOpen ? (
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
                <small>最多 4 张，当前版本先随反馈记录文件名</small>
                <small>图片将在后续版本上传到反馈工单</small>
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

            {feedbackError ? <p className="form-error" role="alert">{feedbackError}</p> : null}
            {feedbackStatus ? <p className="profile-avatar-status" role="status">{feedbackStatus}</p> : null}
            <div className="profile-form-actions feedback-actions">
              <Button type="submit" className="primary-button" disabled={feedbackSubmitting}>
                {feedbackSubmitting ? '提交中' : '提交反馈'}
              </Button>
            </div>
          </form>
        </ListCard>
      ) : null}
    </div>
  );
}

function createDraft(profile: UserProfile): ProfileDraft {
  return {
    display_name: profile.display_name,
    gender: profile.gender,
    birth_date: profile.birth_date ?? '',
    region: profile.region,
  };
}

function profileInitial(profile: UserProfile) {
  return avatarText(firstNonEmpty(profile.display_name, profile.name, profile.identifier) ?? '');
}
