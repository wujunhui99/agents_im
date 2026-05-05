import { useEffect, useMemo, useState, type ComponentType, type FormEvent } from 'react';
import { Check, ChevronRight, Megaphone, Search, Tag, UserPlus, UsersRound } from 'lucide-react';
import type { ContactsApi, Friendship, FriendRequestDecisionData } from '../api/contacts';
import { createContactsApi } from '../api/contacts';
import type { Group, GroupsApi } from '../api/groups';
import { createGroupsApi } from '../api/groups';
import type { UserApi, UserProfile } from '../api/user';
import { createUserApi } from '../api/user';
import { Avatar } from './ui/Avatar';
import { Button } from './ui/Button';
import { Card } from './ui/Card';
import { ListItem } from './ui/ListItem';
import { TextField } from './ui/TextField';
import { accountTypeLabel, avatarText, firstNonEmpty, profileDisplayName, profileIdentifier } from '../utils/profileDisplay';

type Friend = {
  userId: string;
  name: string;
  identifier?: string;
  initial: string;
  avatar: string;
  accountType?: UserProfile['account_type'];
  profile?: UserProfile;
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
  groupsApi?: GroupsApi;
  onStartChat?: (profile: UserProfile) => void;
  onOpenGroup?: (group: Group) => void;
};

const contactEntries: ContactEntry[] = [
  { id: 'new', label: '新的朋友', helper: '查看好友申请并添加好友', accent: 'orange', icon: UserPlus, available: true },
  { id: 'groups', label: '群聊', helper: '创建或打开群聊', accent: 'green', icon: UsersRound, available: true },
  { id: 'tags', label: '标签', helper: '标签功能暂未开放', accent: 'blue', icon: Tag, available: false },
  { id: 'official', label: '公众号', helper: '系统通知与服务号暂未开放', accent: 'gray', icon: Megaphone, available: false },
];

type AddFriendResult = {
  friendship: Friendship;
  created: boolean;
};

type IdentifierSearchState = 'idle' | 'pending' | 'accepted';

function ContactsPage({
  userApi = createUserApi(),
  contactsApi = createContactsApi(),
  groupsApi: groupsApiProp,
  onStartChat,
  onOpenGroup,
}: ContactsPageProps) {
  const groupsApi = useMemo(() => groupsApiProp ?? createGroupsApi(), [groupsApiProp]);
  const [friends, setFriends] = useState<Friend[]>([]);
  const [friendStatus, setFriendStatus] = useState('正在加载好友列表');
  const [openingFriendId, setOpeningFriendId] = useState<string | null>(null);
  const [incomingRequests, setIncomingRequests] = useState<Friendship[]>([]);
  const [outgoingRequests, setOutgoingRequests] = useState<Friendship[]>([]);
  const [requestStatus, setRequestStatus] = useState('正在加载好友申请');
  const [decidingRequestId, setDecidingRequestId] = useState<string | null>(null);
  const [showGroups, setShowGroups] = useState(false);
  const [groups, setGroups] = useState<Group[]>([]);
  const [groupStatus, setGroupStatus] = useState('正在加载群聊');
  const [selectedGroupMemberIds, setSelectedGroupMemberIds] = useState<Set<string>>(() => new Set());
  const [groupName, setGroupName] = useState('');
  const [creatingGroup, setCreatingGroup] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function loadInitialData() {
      await Promise.all([loadFriends({ cancelled: () => cancelled }), loadFriendRequests({ cancelled: () => cancelled })]);
    }

    void loadInitialData();
    return () => {
      cancelled = true;
    };
  }, [contactsApi]);

  async function loadFriends(options?: { cancelled?: () => boolean }) {
    setFriendStatus('正在加载好友列表');
    try {
      const response = await contactsApi.listFriends();
      if (options?.cancelled?.()) {
        return;
      }
      const acceptedFriendships = response.friends.filter(isAcceptedFriendship);
      const nextFriends = acceptedFriendships.map(friendshipToFriend);
      setFriends(nextFriends);
      setFriendStatus(acceptedFriendships.length > 0 ? `已加载 ${acceptedFriendships.length} 位好友` : '暂无好友');
    } catch (error) {
      if (!options?.cancelled?.()) {
        setFriendStatus(error instanceof Error ? error.message : '加载好友列表失败');
      }
    }
  }

  async function loadFriendRequests(options?: { cancelled?: () => boolean }) {
    setRequestStatus('正在加载好友申请');
    try {
      const response = await contactsApi.listFriendRequests();
      if (options?.cancelled?.()) {
        return;
      }
      setIncomingRequests(response.incoming ?? []);
      setOutgoingRequests(response.outgoing ?? []);
      const total = (response.incoming?.length ?? 0) + (response.outgoing?.length ?? 0);
      setRequestStatus(total > 0 ? `已加载 ${total} 条好友申请` : '暂无好友申请');
    } catch (error) {
      if (!options?.cancelled?.()) {
        setRequestStatus(error instanceof Error ? error.message : '加载好友申请失败');
      }
    }
  }

  async function loadGroups(options?: { cancelled?: () => boolean }) {
    setGroupStatus('正在加载群聊');
    try {
      const response = await groupsApi.listGroups();
      if (options?.cancelled?.()) {
        return;
      }
      const nextGroups = response.groups ?? [];
      setGroups(nextGroups);
      setGroupStatus(nextGroups.length > 0 ? `已加载 ${nextGroups.length} 个群聊` : '暂无群聊');
    } catch (error) {
      if (!options?.cancelled?.()) {
        setGroupStatus(error instanceof Error ? error.message : '加载群聊失败');
      }
    }
  }

  async function refreshFriends() {
    await Promise.all([loadFriends(), loadFriendRequests()]);
  }

  async function addFriend(profile: UserProfile): Promise<AddFriendResult> {
    const result = await contactsApi.addFriend(profile.user_id);
    if (isAcceptedFriendship(result.friendship)) {
      setFriends((current) => upsertFriend(current, userProfileToFriend(profile)));
      setFriendStatus(`已添加好友：${profile.identifier}`);
    } else {
      setOutgoingRequests((current) => upsertFriendship(current, result.friendship));
      setRequestStatus(`已发送好友申请：${profile.identifier}`);
      setFriendStatus(`已发送好友申请：${profile.identifier}`);
    }
    return result;
  }

  async function acceptFriendRequest(request: Friendship) {
    const requesterID = request.user_id;
    setDecidingRequestId(requesterID);
    setRequestStatus('正在同意好友申请');
    try {
      const result = await contactsApi.acceptFriendRequest(requesterID);
      applyAcceptedRequest(result, request);
      setRequestStatus(`已同意 ${profileDisplayName(friendshipToUserProfile(request))} 的好友申请`);
      setFriendStatus('好友列表已更新');
    } catch (error) {
      setRequestStatus(error instanceof Error ? error.message : '同意好友申请失败');
    } finally {
      setDecidingRequestId(null);
    }
  }

  async function rejectFriendRequest(request: Friendship) {
    const requesterID = request.user_id;
    setDecidingRequestId(requesterID);
    setRequestStatus('正在拒绝好友申请');
    try {
      await contactsApi.rejectFriendRequest(requesterID);
      setIncomingRequests((current) => current.filter((item) => item.user_id !== requesterID));
      setRequestStatus(`已拒绝 ${profileDisplayName(friendshipToUserProfile(request))} 的好友申请`);
    } catch (error) {
      setRequestStatus(error instanceof Error ? error.message : '拒绝好友申请失败');
    } finally {
      setDecidingRequestId(null);
    }
  }

  function applyAcceptedRequest(result: FriendRequestDecisionData, request: Friendship) {
    setIncomingRequests((current) => current.filter((item) => item.user_id !== request.user_id));
    const profile = friendshipToUserProfile({ ...request, ...result.friendship, friend: request.friend ?? request.friend_profile ?? request.profile });
    setFriends((current) => upsertFriend(current, userProfileToFriend(profile)));
  }

  async function openFriendChat(friend: Friend) {
    if (!onStartChat || openingFriendId) {
      return;
    }

    setOpeningFriendId(friend.userId);
    try {
      const profile = friend.profile ?? friendToUserProfile(friend);
      onStartChat(profile);
      setFriendStatus(`已打开 ${profileDisplayName(profile)} 的聊天`);
    } catch (error) {
      setFriendStatus(error instanceof Error ? error.message : `打开 ${friend.identifier ?? friend.name} 的聊天失败`);
    } finally {
      setOpeningFriendId(null);
    }
  }

  function toggleGroupPanel() {
    setShowGroups((current) => !current);
    void loadGroups();
  }

  function toggleSelectedGroupMember(userId: string) {
    setSelectedGroupMemberIds((current) => {
      const next = new Set(current);
      if (next.has(userId)) {
        next.delete(userId);
      } else {
        next.add(userId);
      }
      return next;
    });
  }

  function selectAllGroupMembers() {
    setSelectedGroupMemberIds(new Set(friends.map((friend) => friend.userId)));
  }

  async function createSelectedGroup() {
    const memberUserIDs = friends.map((friend) => friend.userId).filter((userId) => selectedGroupMemberIds.has(userId));
    if (memberUserIDs.length === 0) {
      setGroupStatus('请选择至少一位联系人');
      return;
    }
    if (memberUserIDs.length + 1 > 200) {
      setGroupStatus('群聊最多 200 人');
      return;
    }

    const name = groupName.trim() || defaultGroupName(friends.filter((friend) => selectedGroupMemberIds.has(friend.userId)));
    setCreatingGroup(true);
    setGroupStatus('正在创建群聊');
    try {
      const group = await groupsApi.createGroup({ name, member_user_ids: memberUserIDs });
      setGroups((current) => upsertGroup(current, group));
      setGroupStatus(`已创建群聊：${group.name}`);
      setGroupName('');
      setSelectedGroupMemberIds(new Set());
      onOpenGroup?.(group);
    } catch (error) {
      setGroupStatus(groupCreateErrorMessage(error));
    } finally {
      setCreatingGroup(false);
    }
  }

  function openGroup(group: Group) {
    onOpenGroup?.(group);
    setGroupStatus(`已打开群聊：${group.name}`);
  }

  return (
    <div className="page-stack contacts-page">
      <IdentifierSearch userApi={userApi} onAddFriend={addFriend} />
      <section className="list-card" aria-label="联系人快捷入口">
        {contactEntries.map((entry) => (
          <ContactEntryButton key={entry.id} entry={entry} onClick={entry.id === 'groups' ? toggleGroupPanel : undefined} />
        ))}
      </section>
      {showGroups ? (
        <GroupChatPanel
          friends={friends}
          groups={groups}
          groupName={groupName}
          selectedMemberIds={selectedGroupMemberIds}
          status={groupStatus}
          creating={creatingGroup}
          onGroupNameChange={setGroupName}
          onToggleMember={toggleSelectedGroupMember}
          onSelectAll={selectAllGroupMembers}
          onCreate={createSelectedGroup}
          onRefresh={loadGroups}
          onOpenGroup={openGroup}
        />
      ) : null}
      <FriendRequestsPanel
        incomingRequests={incomingRequests}
        outgoingRequests={outgoingRequests}
        status={requestStatus}
        decidingRequestId={decidingRequestId}
        onRefresh={loadFriendRequests}
        onAccept={acceptFriendRequest}
        onReject={rejectFriendRequest}
      />
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

function IdentifierSearch({ userApi, onAddFriend }: { userApi: UserApi; onAddFriend: (profile: UserProfile) => Promise<AddFriendResult> }) {
  const [identifier, setIdentifier] = useState('');
  const [result, setResult] = useState<UserProfile | null>(null);
  const [status, setStatus] = useState('按账号搜索用户');
  const [submitting, setSubmitting] = useState(false);
  const [adding, setAdding] = useState(false);
  const [requestStates, setRequestStates] = useState<Map<string, IdentifierSearchState>>(() => new Map());
  const requestState = result ? requestStates.get(result.user_id) ?? 'idle' : 'idle';
  const addButtonLabel = requestState === 'accepted' ? '已添加' : requestState === 'pending' ? '等待对方确认' : adding ? '添加中' : '添加好友';
  const addButtonAriaLabel = requestState === 'accepted' ? '已添加' : requestState === 'pending' ? '等待对方确认' : `添加好友 ${result?.identifier ?? ''}`;

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

  async function handleAddFriend() {
    if (!result) {
      return;
    }

    setAdding(true);
    setStatus('正在添加好友');
    try {
      const addResult = await onAddFriend(result);
      const nextState: IdentifierSearchState = isAcceptedFriendship(addResult.friendship) ? 'accepted' : 'pending';
      setRequestStates((current) => new Map(current).set(result.user_id, nextState));
      setStatus(nextState === 'accepted' ? `已添加好友：${result.identifier}` : '已发送好友申请，等待对方确认');
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
          label="按账号搜索用户"
          hideLabel
          placeholder="输入唯一账号"
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
          leading={<Avatar label={avatarText(profileDisplayName(result))} color="blue" />}
          headline={profileDisplayName(result)}
          supportingText={<ProfileSupportingLines identifier={profileIdentifier(result)} accountType={result.account_type} />}
          trailing={
            <Button
              className="text-command"
              variant="tonal"
              size="small"
              aria-label={addButtonAriaLabel}
              disabled={adding || requestState !== 'idle'}
              onClick={handleAddFriend}
            >
              {addButtonLabel}
            </Button>
          }
        />
      ) : null}
    </section>
  );
}

function ContactEntryButton({ entry, onClick }: { entry: ContactEntry; onClick?: () => void }) {
  const Icon = entry.icon;
  const disabled = !entry.available;

  return (
    <ListItem
      className={`action-row${disabled ? ' action-row-disabled' : ''}`}
      ariaLabel={disabled ? `${entry.label} 暂未开放` : entry.id === 'groups' ? '打开群聊' : entry.label}
      ariaDisabled={disabled}
      onClick={disabled ? undefined : onClick}
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

function GroupChatPanel({
  friends,
  groups,
  groupName,
  selectedMemberIds,
  status,
  creating,
  onGroupNameChange,
  onToggleMember,
  onSelectAll,
  onCreate,
  onRefresh,
  onOpenGroup,
}: {
  friends: Friend[];
  groups: Group[];
  groupName: string;
  selectedMemberIds: Set<string>;
  status: string;
  creating: boolean;
  onGroupNameChange: (value: string) => void;
  onToggleMember: (userId: string) => void;
  onSelectAll: () => void;
  onCreate: () => void;
  onRefresh: () => void;
  onOpenGroup: (group: Group) => void;
}) {
  return (
    <section className="group-workspace" aria-label="群聊">
      <div className="panel-heading">
        <h2>群聊</h2>
        <Button className="text-command" variant="tonal" size="small" onClick={onRefresh}>
          刷新群聊
        </Button>
      </div>
      <p className="inline-status" role="status">
        {status}
      </p>
      <section className="group-actions" aria-label="创建群聊">
        <div className="group-create-row">
          <TextField label="群名称" value={groupName} placeholder="群名称" onChange={(event) => onGroupNameChange(event.target.value)} />
          <Button className="compact-command" type="button" onClick={onCreate} disabled={creating}>
            <UsersRound size={17} />
            <span>{creating ? '创建中' : '创建群聊'}</span>
          </Button>
        </div>
        <div className="group-create-toolbar">
          <Button className="text-command" variant="tonal" size="small" type="button" onClick={onSelectAll} disabled={friends.length === 0}>
            <Check size={15} />
            <span>全选联系人</span>
          </Button>
          <span>{selectedMemberIds.size} 位联系人</span>
        </div>
        <div className="group-select-list" aria-label="选择群成员">
          {friends.length === 0 ? <p className="empty-state">暂无可选好友</p> : null}
          {friends.map((friend) => (
            <label className="group-member-option" key={friend.userId}>
              <input
                type="checkbox"
                aria-label={`选择 ${friend.name}`}
                checked={selectedMemberIds.has(friend.userId)}
                onChange={() => onToggleMember(friend.userId)}
              />
              <span>选择 {friend.name}</span>
              {friend.identifier ? <small>{friend.identifier}</small> : null}
            </label>
          ))}
        </div>
      </section>
      <Card className="list-card" aria-label="群聊列表">
        {groups.length === 0 ? <p className="empty-state">暂无群聊</p> : null}
        {groups.map((group) => (
          <ListItem
            className="group-row"
            key={group.group_id}
            onClick={() => onOpenGroup(group)}
            ariaLabel={`打开群聊 ${group.name}`}
            leading={<Avatar label={avatarText(group.name)} color="green" />}
            headline={group.name}
            supportingText={group.description || '群聊'}
            trailing={<ChevronRight size={18} />}
          />
        ))}
      </Card>
    </section>
  );
}

function FriendRequestsPanel({
  incomingRequests,
  outgoingRequests,
  status,
  decidingRequestId,
  onRefresh,
  onAccept,
  onReject,
}: {
  incomingRequests: Friendship[];
  outgoingRequests: Friendship[];
  status: string;
  decidingRequestId: string | null;
  onRefresh: () => void;
  onAccept: (request: Friendship) => void;
  onReject: (request: Friendship) => void;
}) {
  return (
    <section aria-label="好友申请">
      <div className="panel-heading">
        <h2>好友申请</h2>
        <Button className="text-command" variant="tonal" size="small" onClick={onRefresh}>
          刷新申请
        </Button>
      </div>
      <p className="inline-status" role="status">
        {status}
      </p>
      <Card className="list-card">
        {incomingRequests.length === 0 && outgoingRequests.length === 0 ? <p className="empty-state">暂无好友申请</p> : null}
        {incomingRequests.map((request) => (
          <FriendRequestRow
            key={`incoming-${request.user_id}`}
            request={request}
            direction="incoming"
            busy={decidingRequestId === request.user_id}
            onAccept={() => onAccept(request)}
            onReject={() => onReject(request)}
          />
        ))}
        {outgoingRequests.map((request) => (
          <FriendRequestRow key={`outgoing-${request.friend_id}`} request={request} direction="outgoing" busy={false} />
        ))}
      </Card>
    </section>
  );
}

function FriendRequestRow({
  request,
  direction,
  busy,
  onAccept,
  onReject,
}: {
  request: Friendship;
  direction: 'incoming' | 'outgoing';
  busy: boolean;
  onAccept?: () => void;
  onReject?: () => void;
}) {
  const profile = friendshipToUserProfile(request);
  const name = profileDisplayName(profile);
  const helper = direction === 'incoming' ? '请求添加你为好友' : '等待对方确认';
  return (
    <ListItem
      className="friend-request-row"
      leading={<Avatar label={avatarText(name)} color={direction === 'incoming' ? 'orange' : 'blue'} />}
      headline={name}
      supportingText={<ProfileSupportingLines identifier={profileIdentifier(profile)} accountType={profile.account_type} helper={helper} />}
      trailing={
        direction === 'incoming' ? (
          <span className="request-actions">
            <Button className="text-command" variant="tonal" size="small" aria-label={`同意 ${profileIdentifier(profile)}`} disabled={busy} onClick={onAccept}>
              同意
            </Button>
            <Button className="text-command" variant="tonal" size="small" aria-label={`拒绝 ${profileIdentifier(profile)}`} disabled={busy} onClick={onReject}>
              拒绝
            </Button>
          </span>
        ) : (
          <span className="row-badge">等待对方确认</span>
        )
      }
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
              const chatLabel = friend.identifier ?? friend.name;
              return (
                <ListItem
                  className="friend-row"
                  key={friend.userId}
                  onClick={() => onOpenFriendChat(friend)}
                  ariaLabel={`和 ${chatLabel} 聊天`}
                  ariaDisabled={isOpening}
                  leading={<Avatar label={friend.avatar} color="blue" />}
                  headline={friend.name}
                  supportingText={<ProfileSupportingLines identifier={friend.identifier} accountType={friend.accountType} />}
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

function isAcceptedFriendship(friendship: Friendship) {
  return friendship.status === 'accepted' || friendship.status === 'active' || friendship.is_friend;
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
  const name = profileDisplayName(profile);
  return {
    userId: profile.user_id,
    name,
    identifier: profileIdentifier(profile),
    initial: avatarText(name).slice(0, 1),
    avatar: avatarText(name),
    accountType: profile.account_type,
    profile,
  };
}

function friendshipToFriend(friendship: Friendship): Friend {
  return userProfileToFriend(friendshipToUserProfile(friendship));
}

function friendToUserProfile(friend: Friend): UserProfile {
  return {
    user_id: friend.userId,
    identifier: friend.identifier ?? '',
    display_name: friend.name,
    name: friend.name,
    gender: '',
    birth_date: '',
    region: '',
    account_type: friend.accountType,
  };
}

export function friendshipToUserProfile(friendship: Friendship): UserProfile {
  const profile = friendship.friend ?? friendship.friend_profile ?? friendship.profile ?? {};
  const userId = profile.user_id || friendship.friend_id;
  const identifier = firstNonEmpty(profile.identifier, friendship.friend_identifier, friendship.identifier) ?? '';
  const displayName =
    firstNonEmpty(
      profile.display_name,
      profile.name,
      friendship.friend_display_name,
      friendship.friend_name,
      friendship.display_name,
      friendship.name,
      identifier,
    ) ?? '未知联系人';

  return {
    user_id: userId,
    identifier,
    display_name: displayName,
    name: profile.name || displayName,
    gender: profile.gender ?? '',
    birth_date: profile.birth_date ?? '',
    region: profile.region ?? '',
    account_type: profile.account_type,
    avatar_media_id: profile.avatar_media_id,
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

function upsertFriendship(friendships: Friendship[], friendship: Friendship) {
  const key = `${friendship.user_id}:${friendship.friend_id}`;
  if (friendships.some((item) => `${item.user_id}:${item.friend_id}` === key)) {
    return friendships.map((item) => (`${item.user_id}:${item.friend_id}` === key ? friendship : item));
  }
  return [...friendships, friendship];
}

function upsertGroup(groups: Group[], group: Group) {
  if (groups.some((item) => item.group_id === group.group_id)) {
    return groups.map((item) => (item.group_id === group.group_id ? group : item));
  }
  return [group, ...groups];
}

function defaultGroupName(selectedFriends: Friend[]) {
  const names = selectedFriends.slice(0, 3).map((friend) => friend.name);
  return names.length > 0 ? `${names.join('、')} 的群聊` : '新群聊';
}

function groupCreateErrorMessage(error: unknown) {
  const message = error instanceof Error ? error.message : '';
  if (/200|limit|最多/.test(message)) {
    return '群聊最多 200 人';
  }
  return message || '创建群聊失败';
}

function ProfileSupportingLines({
  identifier,
  accountType,
  helper,
}: {
  identifier?: string;
  accountType?: UserProfile['account_type'];
  helper?: string;
}) {
  return (
    <span className="friend-supporting-lines">
      <span>{identifier ?? '资料未同步'}</span>
      <span>{accountTypeLabel(accountType)}</span>
      {helper ? <span>{helper}</span> : null}
    </span>
  );
}

export default ContactsPage;
