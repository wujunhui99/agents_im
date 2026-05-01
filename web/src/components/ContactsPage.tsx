import { useMemo, useState, type ComponentType, type FormEvent } from 'react';
import { ChevronRight, Megaphone, Search, Tag, UserPlus, UsersRound } from 'lucide-react';
import type { ContactsApi, Friendship } from '../api/contacts';
import { createContactsApi } from '../api/contacts';
import type { UserApi, UserProfile } from '../api/user';
import { createUserApi } from '../api/user';
import { Avatar } from './ui/Avatar';
import { Button } from './ui/Button';
import { Card } from './ui/Card';
import { ListItem } from './ui/ListItem';
import { TextField } from './ui/TextField';

type Friend = {
  userId: string;
  name: string;
  identifier: string;
  initial: string;
  avatar: string;
};

type ContactEntry = {
  id: 'new' | 'groups' | 'tags' | 'official';
  label: string;
  helper: string;
  accent: string;
  icon: ComponentType<{ size?: number }>;
};

type ContactsPageProps = {
  userApi?: UserApi;
  contactsApi?: ContactsApi;
};

const contactEntries: ContactEntry[] = [
  { id: 'new', label: '新的朋友', helper: '通过账号搜索并添加好友', accent: 'orange', icon: UserPlus },
  { id: 'groups', label: '群聊', helper: '群聊能力走 groups API，后续在群聊页展开', accent: 'green', icon: UsersRound },
  { id: 'tags', label: '标签', helper: '标签功能暂未实现', accent: 'blue', icon: Tag },
  { id: 'official', label: '公众号', helper: '系统通知与服务号暂未实现', accent: 'gray', icon: Megaphone },
];

function ContactsPage({ userApi = createUserApi(), contactsApi = createContactsApi() }: ContactsPageProps) {
  const [friends, setFriends] = useState<Friend[]>([]);
  const [friendStatus, setFriendStatus] = useState('点击刷新好友列表');

  async function refreshFriends() {
    setFriendStatus('正在加载好友列表');
    const response = await contactsApi.listFriends();
    setFriends(response.friends.map(friendshipToFriend));
    setFriendStatus(response.friends.length > 0 ? `已加载 ${response.friends.length} 位好友` : '暂无好友');
  }

  async function addFriend(profile: UserProfile) {
    await contactsApi.addFriend(profile.user_id);
    setFriends((current) => upsertFriend(current, userProfileToFriend(profile)));
    setFriendStatus(`已添加好友：${profile.identifier}`);
  }

  return (
    <div className="page-stack contacts-page">
      <IdentifierSearch userApi={userApi} onAddFriend={addFriend} />
      <section className="list-card" aria-label="联系人快捷入口">
        {contactEntries.map((entry) => (
          <ContactEntryButton key={entry.id} entry={entry} />
        ))}
      </section>
      <section aria-label="好友列表">
        <div className="panel-heading">
          <h2>好友</h2>
          <Button className="text-command" variant="tonal" size="small" onClick={refreshFriends}>
            刷新好友
          </Button>
        </div>
        <p className="inline-status" role="status">
          {friendStatus}
        </p>
        <FriendDirectory friends={friends} />
      </section>
    </div>
  );
}

function IdentifierSearch({ userApi, onAddFriend }: { userApi: UserApi; onAddFriend: (profile: UserProfile) => Promise<void> }) {
  const [identifier, setIdentifier] = useState('');
  const [result, setResult] = useState<UserProfile | null>(null);
  const [status, setStatus] = useState('按唯一 identifier 搜索用户');
  const [submitting, setSubmitting] = useState(false);
  const [adding, setAdding] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const query = identifier.trim();

    if (!query) {
      setResult(null);
      setStatus('请输入 identifier');
      return;
    }

    setSubmitting(true);
    setStatus('正在搜索用户');
    try {
      const profile = await userApi.getPublicProfileByIdentifier(query);
      setResult(profile);
      setStatus(`找到 ${profile.display_name || profile.name || profile.identifier}`);
    } catch (error) {
      setResult(null);
      setStatus(error instanceof Error ? error.message : `未找到 ${query}`);
    } finally {
      setSubmitting(false);
    }
  }

  async function handleAddFriend() {
    if (!result) {
      return;
    }

    setAdding(true);
    setStatus('正在添加好友');
    try {
      await onAddFriend(result);
      setStatus(`已添加好友：${result.identifier}`);
    } catch (error) {
      setStatus(error instanceof Error ? error.message : '添加好友失败');
    } finally {
      setAdding(false);
    }
  }

  return (
    <section className="identifier-search-card" aria-label="账号搜索">
      <form className="identifier-search-form" onSubmit={handleSubmit}>
        <TextField
          label="按 identifier 搜索用户"
          hideLabel
          placeholder="输入唯一 identifier"
          value={identifier}
          onChange={(event) => setIdentifier(event.target.value)}
          leadingIcon={<Search size={17} />}
          fieldClassName="search-box identifier-field"
        />
        <Button className="compact-command" type="submit" aria-label="搜索用户" disabled={submitting}>
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
          leading={<Avatar label={avatarText(result.display_name || result.name || result.identifier)} color="blue" />}
          headline={result.display_name || result.name || result.identifier}
          supportingText={result.identifier}
          trailing={
            <Button
              className="text-command"
              variant="tonal"
              size="small"
              aria-label={`添加好友 ${result.identifier}`}
              disabled={adding}
              onClick={handleAddFriend}
            >
              {adding ? '添加中' : '添加好友'}
            </Button>
          }
        />
      ) : null}
    </section>
  );
}

function ContactEntryButton({ entry }: { entry: ContactEntry }) {
  const Icon = entry.icon;

  return (
    <ListItem
      className="action-row"
      ariaLabel={entry.label}
      leading={
        <div className={`action-icon action-${entry.accent}`}>
          <Icon size={19} />
        </div>
      }
      headline={entry.label}
      supportingText={entry.helper}
      trailing={<ChevronRight size={18} />}
    />
  );
}

function FriendDirectory({ friends }: { friends: Friend[] }) {
  const groups = useMemo(() => groupFriends(friends), [friends]);

  if (friends.length === 0) {
    return <p className="empty-state">暂无好友</p>;
  }

  return (
    <>
      {groups.map(([initial, groupedFriends]) => (
        <section className="friend-group" aria-labelledby={`friend-group-${initial}`} key={initial}>
          <h2 className="section-label" id={`friend-group-${initial}`}>
            {initial}
          </h2>
          <Card className="list-card">
            {groupedFriends.map((friend) => (
              <ListItem
                className="friend-row"
                key={friend.userId}
                leading={<Avatar label={friend.avatar} color="blue" />}
                headline={friend.name}
                supportingText={
                  <span className="friend-supporting-lines">
                    <span>{friend.identifier}</span>
                    <span>{friend.userId}</span>
                  </span>
                }
              />
            ))}
          </Card>
        </section>
      ))}
    </>
  );
}

function groupFriends(directoryFriends: Friend[]) {
  const grouped = directoryFriends.reduce<Map<string, Friend[]>>((accumulator, friend) => {
    const initial = friend.initial.slice(0, 1).toUpperCase() || '#';
    const items = accumulator.get(initial) ?? [];
    accumulator.set(initial, [...items, friend]);
    return accumulator;
  }, new Map<string, Friend[]>());

  return [...grouped.entries()].sort(([left], [right]) => left.localeCompare(right));
}

function userProfileToFriend(profile: UserProfile): Friend {
  const name = profile.display_name || profile.name || profile.identifier;
  return {
    userId: profile.user_id,
    name,
    identifier: profile.identifier,
    initial: avatarText(name).slice(0, 1),
    avatar: avatarText(name),
  };
}

function friendshipToFriend(friendship: Friendship): Friend {
  const name = friendship.friend_id;
  return {
    userId: friendship.friend_id,
    name,
    identifier: friendship.friend_id,
    initial: avatarText(name).slice(0, 1),
    avatar: avatarText(name),
  };
}

function upsertFriend(friends: Friend[], friend: Friend) {
  if (friends.some((item) => item.userId === friend.userId)) {
    return friends.map((item) => (item.userId === friend.userId ? friend : item));
  }
  return [...friends, friend];
}

function avatarText(value: string) {
  return value.trim().slice(0, 2).toUpperCase() || 'U';
}

export default ContactsPage;
