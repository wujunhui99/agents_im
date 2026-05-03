import { FormEvent, useEffect, useState } from 'react';
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
        <Avatar label={profileInitial(profile)} color="green" size="large" />
        <div className="profile-main">
          <strong>{profileDisplayName(profile)}</strong>
          <p>账号：{profile.identifier}</p>
          <p>账号类型：{accountTypeLabel(profile.account_type)}</p>
          <p>地区：{profile.region}</p>
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
