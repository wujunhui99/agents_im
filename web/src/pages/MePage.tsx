import { ChangeEvent, FormEvent, useEffect, useState } from 'react';
import { Settings } from 'lucide-react';
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
};

export function MePage({ profile, onUpdateProfile, onUploadAvatar, onAvatarUpdated }: MePageProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [draft, setDraft] = useState<ProfileDraft>(() => createDraft(profile));
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState('');
  const [avatarStatus, setAvatarStatus] = useState('');
  const [isUploadingAvatar, setIsUploadingAvatar] = useState(false);
  const [avatarURL, setAvatarURL] = useState(profile.avatar_url ?? '');
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
        <ActionRow label="朋友圈" helper="我的动态" accent="blue" />
        <ActionRow label="设置" helper="账号、安全、通知" accent="gray" trailingIcon={Settings} />
      </ListCard>
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
