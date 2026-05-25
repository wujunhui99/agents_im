import { ChangeEvent, FormEvent, useEffect, useState } from 'react';
import { Settings } from 'lucide-react';
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
      });
      setFeedbackDraft(defaultFeedbackDraft());
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
      <ListCard>
        <ActionRow label="收藏" helper="重要消息和 Agent 输出" accent="orange" />
        <ActionRow label="意见反馈" helper="Bug、体验问题和功能建议" accent="green" onClick={() => setFeedbackOpen((current) => !current)} />
        <ActionRow label="设置" helper="账号、安全、通知" accent="gray" trailingIcon={Settings} />
      </ListCard>

      {feedbackOpen ? (
        <ListCard ariaLabel="意见反馈" className="feedback-card">
          <form className="profile-edit-form" onSubmit={handleFeedbackSubmit}>
            <label className="profile-field">
              <span>反馈类型</span>
              <select
                value={feedbackDraft.category}
                onChange={(event) => setFeedbackDraft((current) => ({ ...current, category: event.target.value }))}
              >
                <option value="bug">Bug</option>
                <option value="poor_experience">体验不好</option>
                <option value="feature_request">功能建议</option>
                <option value="other">其他</option>
              </select>
            </label>
            <TextField
              label="标题"
              value={feedbackDraft.title}
              onChange={(event) => setFeedbackDraft((current) => ({ ...current, title: event.target.value }))}
              fieldClassName="profile-field"
              required
            />
            <label className="profile-field">
              <span>反馈内容</span>
              <textarea
                value={feedbackDraft.content}
                onChange={(event) => setFeedbackDraft((current) => ({ ...current, content: event.target.value }))}
                required
              />
            </label>
            <TextField
              label="联系方式（选填）"
              value={feedbackDraft.contact}
              onChange={(event) => setFeedbackDraft((current) => ({ ...current, contact: event.target.value }))}
              fieldClassName="profile-field"
            />
            {feedbackError ? <p className="form-error" role="alert">{feedbackError}</p> : null}
            {feedbackStatus ? <p className="profile-avatar-status" role="status">{feedbackStatus}</p> : null}
            <div className="profile-form-actions">
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
