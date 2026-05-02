import { useEffect, useMemo, useState, type ComponentType, type FormEvent } from 'react';
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
  available: boolean;
};

type ContactsPageProps = {
  userApi?: UserApi;
  contactsApi?: ContactsApi;
  onStartChat?: (profile: UserProfile) => void;
};

const contactEntries: ContactEntry[] = [
  { id: 'new', label: '新的朋友', helper: '通过账号搜索并添加好友', accent: 'orange', icon: UserPlus, available: true },
  { id: 'groups', label: '群聊', helper: '群聊入口暂未开放', accent: 'green', icon: UsersRound, available: false },
  { id: 'tags', label: '标签', helper: '标签功能暂未开放', accent: 'blue', icon: Tag, available: false },
  { id: 'official', label: '公众号', helper: '系统通知与服务号暂未开放', accent: 'gray', icon: Megaphone, available: false },
];

function ContactsPage({ userApi = createUserApi(), contactsApi = createContactsApi(), onStartChat }: ContactsPageProps) {
  const [friends, setFriends] = useState<Friend[]>([]);
  const [friendStatus, setFriendStatus] = useState('正在加载好友列表');
  const [openingFriendId, setOpeningFriendId] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadFriends() {
      setFriendStatus('正在加载好友列表');
      try {
        const response = await contactsApi.listFriends();
        if (cancelled) {
          return;
        }
        const nextFriends = response.friends.map(friendshipToFriend);
        setFriends(nextFriends);
        setFriendStatus(response.friends.length > 0 ? `已加载 ${response.friends.length} 位好友` : '暂无好友');
      } catch (error) {
        if (!cancelled) {
          setFriendStatus(error instanceof Error ? error.message : '加载好友列表失败');
        }
      }
    }

    void loadFriends();
    return () => {
      cancelled = true;
    };
  }, [contactsApi]);

  async function refreshFriends() {
    setFriendStatus('正在加载好友列表');
    try {
      const response = await contactsApi.listFriends();
      const nextFriends = response.friends.map(friendshipToFriend);
      setFriends(nextFriends);
      setFriendStatus(response.friends.length > 0 ? `已加载 ${response.friends.length} 位好友` : '暂无好友');
    } catch (error) {
      setFriendStatus(error instanceof Error ? error.message : '加载好友列表失败');
    }
  }

  async function addFriend(profile: UserProfile) {
    await contactsApi.addFriend(profile.user_id);
    setFriends((current) => {
      const nextFriends = upsertFriend(current, userProfileToFriend(profile));
      return nextFriends;
    });
    setFriendStatus(`已添加好友：${profile.identifier}`);
  }

  async function openFriendChat(friend: Friend) {
    if (!onStartChat || openingFriendId) {
      return;
    }

    setOpeningFriendId(friend.userId);
    setFriendStatus(`正在打开 ${friend.identifier} 的聊天`);
    try {
      const profile = await userApi.getPublicProfileByIdentifier(friend.identifier);
      onStartChat(profile);
      setFriendStatus(`已打开 ${profileDisplayName(profile)} 的聊天`);
    } catch (error) {
      setFriendStatus(error instanceof Error ? error.message : `打开 ${friend.identifier} 的聊天失败`);
    } finally {
      setOpeningFriendId(null);
    }
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
        <FriendDirectory friends={friends} openingFriendId={openingFriendId} onOpenFriendChat={openFriendChat} />
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
              aria-label={isAdded ? '已添加' : `添加好友 ${result.identifier}`}
              disabled={adding || isAdded}
              onClick={handleAddFriend}
            >
              {isAdded ? '已添加' : adding ? '添加中' : '添加好友'}
            </Button>
          }
        />
      ) : null}
    </section>
  );
}

function ContactEntryButton({ entry }: { entry: ContactEntry }) {
  const Icon = entry.icon;
  const disabled = !entry.available;

  return (
<ListItem
      className={`action-row${disabled ? ' action-row-disabled' : ''}`}
      ariaLabel={disabled ? `${entry.label} 暂未开放` : entry.label}
      ariaDisabled={disabled}
      leading={
        <div className={`action-icon action-${entry.accent}`}>
          <Icon size={19} />
        </div>
      }
      headline={entry.label}
      supportingText={entry.helper}
      trailing={disabled ? <span className="row-badge">暂未开放</span> : <ChevronRight size={18} />}
    />
  );
}

function FriendDirectory({
  friends,
  openingFriendId,
  onOpenFriendChat,
}: {
  friends: Friend[];
  openingFriendId: string | null;
  onOpenFriendChat: (friend: Friend) => void;
}) {
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
            {groupedFriends.map((friend) => {
              const isOpening = openingFriendId === friend.userId;
              return (
                <ListItem
                  className="friend-row"
                  key={friend.userId}
                  onClick={() => onOpenFriendChat(friend)}
                  ariaLabel={`和 ${friend.identifier} 聊天`}
                  ariaDisabled={isOpening}
                  leading={<Avatar label={friend.avatar} color="blue" />}
                  headline={friend.name}
                  supportingText={
                    <span className="friend-supporting-lines">
                      <span>{friend.identifier}</span>
                      <span>{friend.userId}</span>
                    </span>
                  }
                  trailing={isOpening ? <span className="row-badge">打开中</span> : <ChevronRight size={18} />}
                />
              );
            })}
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
  return userProfileToFriend(friendshipToUserProfile(friendship));
}

function friendshipToUserProfile(friendship: Friendship): UserProfile {
  const profile = friendship.friend ?? friendship.friend_profile ?? friendship.profile ?? {};
  const userId = profile.user_id || friendship.friend_id;
  const identifier = profile.identifier || friendship.friend_identifier || friendship.identifier || userId;
  const displayName =
    profile.display_name ||
    profile.name ||
    friendship.friend_display_name ||
    friendship.friend_name ||
    friendship.display_name ||
    friendship.name ||
    identifier;

  return {
    user_id: userId,
    identifier,
    display_name: displayName,
    name: profile.name || displayName,
    gender: profile.gender ?? '',
    age: profile.age ?? 0,
    region: profile.region ?? '',
    account_type: profile.account_type,
    created_at: profile.created_at,
    updated_at: profile.updated_at,
  };
}

function upsertFriend(friends: Friend[], friend: Friend) {
  if (friends.some((item) => item.userId === friend.userId)) {
    return friends.map((item) => (item.userId === friend.userId ? friend : item));
  }
  return [...friends, friend];
}

function profileDisplayName(profile: UserProfile) {
  return profile.display_name || profile.name || profile.identifier || profile.user_id;
}

function avatarText(value: string) {
  return value.trim().slice(0, 2).toUpperCase() || 'U';
}

export default ContactsPage;
