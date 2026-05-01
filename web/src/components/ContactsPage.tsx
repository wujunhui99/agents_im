import { useMemo, useState, type ComponentType, type FormEvent } from 'react';
import { ChevronRight, Megaphone, Search, Tag, UserPlus, UsersRound } from 'lucide-react';
import type { ContactsApi, Friendship } from '../api/contacts';
import { createContactsApi } from '../api/contacts';
import type { UserApi, UserProfile } from '../api/user';
import { createUserApi } from '../api/user';

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
  available: boolean;
};

type ContactsPageProps = {
  userApi?: UserApi;
  contactsApi?: ContactsApi;
};

const contactEntries: ContactEntry[] = [
  { id: 'new', label: '新的朋友', helper: '通过账号搜索并添加好友', accent: 'orange', icon: UserPlus, available: true },
  { id: 'groups', label: '群聊', helper: '群聊入口暂未开放', accent: 'green', icon: UsersRound, available: false },
  { id: 'tags', label: '标签', helper: '标签功能暂未开放', accent: 'blue', icon: Tag, available: false },
  { id: 'official', label: '公众号', helper: '系统通知与服务号暂未开放', accent: 'gray', icon: Megaphone, available: false },
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
          <button className="text-command" type="button" onClick={refreshFriends}>
            刷新好友
          </button>
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
  const [addedUserIds, setAddedUserIds] = useState<Set<string>>(() => new Set());
  const isAdded = result ? addedUserIds.has(result.user_id) : false;

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
      setAddedUserIds((current) => new Set(current).add(result.user_id));
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
        <label className="search-box identifier-field">
          <Search size={17} />
          <input
            placeholder="输入唯一 identifier"
            aria-label="按 identifier 搜索用户"
            value={identifier}
            onChange={(event) => setIdentifier(event.target.value)}
          />
        </label>
        <button className="compact-command" type="submit" aria-label="搜索用户" disabled={submitting}>
          <Search size={17} />
          <span>搜索</span>
        </button>
      </form>
      <p className="inline-status" role="status">
        {status}
      </p>
      {result ? (
        <article className="search-result">
          <div className="avatar avatar-blue">{avatarText(result.display_name || result.name || result.identifier)}</div>
          <div className="row-main">
            <strong>{result.display_name || result.name || result.identifier}</strong>
            <p>{result.identifier}</p>
          </div>
          <button
            className="text-command"
            type="button"
            aria-label={isAdded ? '已添加' : `添加好友 ${result.identifier}`}
            disabled={adding || isAdded}
            onClick={handleAddFriend}
          >
            {isAdded ? '已添加' : adding ? '添加中' : '添加好友'}
          </button>
        </article>
      ) : null}
    </section>
  );
}

function ContactEntryButton({ entry }: { entry: ContactEntry }) {
  const Icon = entry.icon;
  const disabled = !entry.available;

  return (
    <div
      className={`action-row${disabled ? ' action-row-disabled' : ''}`}
      aria-label={disabled ? `${entry.label} 暂未开放` : entry.label}
      aria-disabled={disabled || undefined}
    >
      <div className={`action-icon action-${entry.accent}`}>
        <Icon size={19} />
      </div>
      <div className="row-main">
        <strong>{entry.label}</strong>
        <p>{entry.helper}</p>
      </div>
      {disabled ? (
        <div className="row-trailing">
          <span className="row-badge">暂未开放</span>
        </div>
      ) : (
        <ChevronRight size={18} />
      )}
    </div>
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
          <div className="list-card">
            {groupedFriends.map((friend) => (
              <article className="friend-row" key={friend.userId}>
                <div className="avatar avatar-blue">{friend.avatar}</div>
                <div>
                  <strong>{friend.name}</strong>
                  <p>{friend.identifier}</p>
                  <p>{friend.userId}</p>
                </div>
              </article>
            ))}
          </div>
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
