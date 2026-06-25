import { Search } from 'lucide-react';
import { useState, type FormEvent } from 'react';
import type { UserApi, UserProfile } from '../../../api/user';
import { Avatar } from '../../../components/ui/Avatar';
import { Button } from '../../../components/ui/Button';
import { ListItem } from '../../../components/ui/ListItem';
import { TextField } from '../../../components/ui/TextField';
import { accountTypeLabel, avatarText, profileDisplayName, profileIdentifier } from '../../../utils/profileDisplay';

export function StartChatPanel({
  userApi,
  onStartChat,
  onClose,
}: {
  userApi: UserApi;
  onStartChat: (profile: UserProfile) => void;
  onClose: () => void;
}) {
  const [identifier, setIdentifier] = useState('');
  const [result, setResult] = useState<UserProfile | null>(null);
  const [status, setStatus] = useState('输入账号搜索用户');
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const query = identifier.trim();
    if (!query) {
      setResult(null);
      setStatus('请输入账号');
      return;
    }
    setSubmitting(true);
    setStatus('正在搜索用户');
    try {
      const profile = await userApi.getPublicProfileByIdentifier(query);
      setResult(profile);
      setStatus(`找到 ${profileDisplayName(profile)}`);
    } catch (error) {
      setResult(null);
      setStatus(error instanceof Error ? error.message : `未找到 ${query}`);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <section className="start-chat-card" aria-label="发起聊天">
      <div className="start-chat-heading">
        <h2>发起聊天</h2>
        <Button type="button" className="text-command" variant="text" onClick={onClose}>
          关闭
        </Button>
      </div>
      <form className="identifier-search-form" onSubmit={handleSubmit}>
        <TextField
          label="按账号搜索聊天对象"
          hideLabel
          placeholder="输入唯一账号"
          value={identifier}
          onChange={(event) => setIdentifier(event.target.value)}
          leadingIcon={<Search size={17} />}
          fieldClassName="search-box identifier-field"
        />
        <Button className="compact-command" type="submit" aria-label="搜索聊天对象" disabled={submitting}>
          <Search size={17} />
          <span>搜索</span>
        </Button>
      </form>
      <p className="inline-status" role="status">
        {status}
      </p>
      {result ? (
        <ListItem
          className="search-result"
          leading={
            <Avatar
              label={avatarText(profileDisplayName(result))}
              color="blue"
              src={result.avatar_url}
              alt={`${profileDisplayName(result)} 头像`}
            />
          }
          headline={profileDisplayName(result)}
          supportingText={
            <span className="friend-supporting-lines">
              <span>{profileIdentifier(result) ?? '资料未同步'}</span>
              <span>{accountTypeLabel(result.account_type)}</span>
            </span>
          }
          trailing={
            <Button
              className="text-command"
              variant="tonal"
              size="small"
              aria-label={`发起聊天 ${profileDisplayName(result)}`}
              onClick={() => onStartChat(result)}
            >
              发起聊天
            </Button>
          }
        />
      ) : null}
    </section>
  );
}
