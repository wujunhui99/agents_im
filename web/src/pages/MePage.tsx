import { FormEvent, useEffect, useState } from 'react';
import { Settings } from 'lucide-react';
import type { UserProfile, UserProfilePatch } from '../api/user';
import { ActionRow } from '../components/ui/ActionRow';
import { Avatar } from '../components/ui/Avatar';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { ListCard } from '../components/ui/ListCard';
import { TextField } from '../components/ui/TextField';

type ProfileDraft = {
  display_name: string;
  gender: string;
  age: string;
  region: string;
};

type MePageProps = {
  profile: UserProfile;
  onUpdateProfile: (patch: UserProfilePatch) => Promise<void>;
};

export function MePage({ profile, onUpdateProfile }: MePageProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [draft, setDraft] = useState<ProfileDraft>(() => createDraft(profile));
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    setDraft(createDraft(profile));
  }, [profile]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');
    setIsSaving(true);

    try {
      await onUpdateProfile({
        display_name: draft.display_name.trim(),
        gender: draft.gender.trim(),
        age: Number(draft.age),
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
        <Avatar label={profileInitial(profile)} color="green" size="large" />
        <div className="profile-main">
          <strong>{profile.display_name}</strong>
          <p>账号：{profile.identifier}</p>
          <p>地区：{profile.region}</p>
        </div>
        <Button variant="tonal" size="small" className="profile-edit-button" aria-label="编辑个人资料" onClick={() => setIsEditing(true)}>
          <span>编辑资料</span>
        </Button>
      </Card>

      <ListCard ariaLabel="个人资料详情" className="profile-detail-card">
        <dl className="profile-detail-list">
          <div>
            <dt>user_id</dt>
            <dd>{profile.user_id}</dd>
          </div>
          <div>
            <dt>identifier</dt>
            <dd>{profile.identifier}</dd>
          </div>
          <div>
            <dt>display_name</dt>
            <dd>{profile.display_name}</dd>
          </div>
          <div>
            <dt>gender</dt>
            <dd>{profile.gender}</dd>
          </div>
          <div>
            <dt>age</dt>
            <dd>{profile.age}</dd>
          </div>
          <div>
            <dt>region</dt>
            <dd>{profile.region}</dd>
          </div>
        </dl>
      </ListCard>

      {isEditing ? (
        <ListCard ariaLabel="编辑个人资料" className="profile-edit-card">
          <form className="profile-edit-form" onSubmit={handleSubmit}>
            <TextField
              label="display_name"
              value={draft.display_name}
              onChange={(event) => setDraft((current) => ({ ...current, display_name: event.target.value }))}
              fieldClassName="profile-field"
            />
            <TextField
              label="gender"
              value={draft.gender}
              onChange={(event) => setDraft((current) => ({ ...current, gender: event.target.value }))}
              fieldClassName="profile-field"
            />
            <TextField
              label="age"
              type="number"
              min="0"
              value={draft.age}
              onChange={(event) => setDraft((current) => ({ ...current, age: event.target.value }))}
              fieldClassName="profile-field"
            />
            <TextField
              label="region"
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
    age: String(profile.age),
    region: profile.region,
  };
}

function profileInitial(profile: UserProfile) {
  const displayName = profile.display_name.trim();
  if (displayName) {
    return displayName.slice(0, 2).toUpperCase();
  }

  return profile.identifier.slice(0, 2).toUpperCase();
}
