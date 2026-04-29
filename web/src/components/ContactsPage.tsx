import { useMemo, useState, type ComponentType, type FormEvent } from 'react';
import {
  Check,
  ChevronLeft,
  ChevronRight,
  Hash,
  Megaphone,
  Plus,
  Search,
  Tag,
  UserPlus,
  UserRound,
  UsersRound,
} from 'lucide-react';

type Friend = {
  id: string;
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

type GroupMemberView = {
  userId: string;
  name: string;
  identifier: string;
};

type GroupView = {
  id: string;
  name: string;
  description: string;
  members: GroupMemberView[];
};

const currentUser: GroupMemberView = {
  userId: 'usr_000001',
  name: 'Alice Chen',
  identifier: 'alice_001',
};

const friends: Friend[] = [
  { id: 'alice', userId: 'usr_000001', name: 'Alice Chen', identifier: 'alice_001', initial: 'A', avatar: 'A' },
  { id: 'agent', userId: 'usr_agent', name: 'Agent 助手', identifier: 'agent_helper', initial: 'A', avatar: 'AI' },
  { id: 'bob', userId: 'usr_000002', name: 'Bob Lin', identifier: 'bob_002', initial: 'B', avatar: 'B' },
];

const searchableUsers: Friend[] = [
  ...friends,
  { id: 'carol', userId: 'usr_000003', name: 'Carol Wu', identifier: 'carol_003', initial: 'C', avatar: 'C' },
];

const contactEntries: ContactEntry[] = [
  { id: 'new', label: '新的朋友', helper: '好友申请与推荐', accent: 'orange', icon: UserPlus },
  { id: 'groups', label: '群聊', helper: 'Frontend Demo、Agent 群', accent: 'green', icon: UsersRound },
  { id: 'tags', label: '标签', helper: '按角色整理联系人', accent: 'blue', icon: Tag },
  { id: 'official', label: '公众号', helper: '系统通知与服务号', accent: 'gray', icon: Megaphone },
];

const initialGroups: GroupView[] = [
  {
    id: 'grp_000001',
    name: 'Frontend Demo',
    description: 'MVP smoke room',
    members: [
      currentUser,
      { userId: 'usr_000002', name: 'Bob Lin', identifier: 'bob_002' },
    ],
  },
  {
    id: 'grp_agent',
    name: 'Agent 群',
    description: 'Agent 生命周期与群聊演示',
    members: [
      currentUser,
      { userId: 'usr_agent', name: 'Agent 助手', identifier: 'agent_helper' },
    ],
  },
];

function ContactsPage() {
  const [showGroups, setShowGroups] = useState(false);

  return (
    <div className="page-stack contacts-page">
      <IdentifierSearch />
      <section className="list-card" aria-label="联系人快捷入口">
        {contactEntries.map((entry) => (
          <ContactEntryButton key={entry.id} entry={entry} onClick={() => entry.id === 'groups' && setShowGroups(true)} />
        ))}
      </section>

      {showGroups ? <GroupsPanel onBack={() => setShowGroups(false)} /> : <FriendDirectory friends={friends} />}
    </div>
  );
}

function IdentifierSearch() {
  const [identifier, setIdentifier] = useState('');
  const [result, setResult] = useState<Friend | null>(null);
  const [status, setStatus] = useState('按唯一 identifier 搜索用户');
  const [addedIdentifier, setAddedIdentifier] = useState<string | null>(null);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const query = identifier.trim().toLowerCase();

    if (!query) {
      setResult(null);
      setAddedIdentifier(null);
      setStatus('请输入 identifier');
      return;
    }

    const found = searchableUsers.find((user) => user.identifier.toLowerCase() === query) ?? null;
    setResult(found);
    setAddedIdentifier(null);
    setStatus(found ? `找到 ${found.name}` : `未找到 ${identifier.trim()}`);
  }

  function handleAddFriend() {
    if (!result) {
      return;
    }

    setAddedIdentifier(result.identifier);
    setStatus(`已发送添加请求：${result.identifier}`);
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
        <button className="compact-command" type="submit" aria-label="搜索用户">
          <Search size={17} />
          <span>搜索</span>
        </button>
      </form>
      <p className="inline-status" role="status">
        {status}
      </p>
      {result ? (
        <article className="search-result">
          <div className="avatar avatar-blue">{result.avatar}</div>
          <div className="row-main">
            <strong>{result.name}</strong>
            <p>{result.identifier}</p>
          </div>
          <button
            className="text-command"
            type="button"
            aria-label={`添加好友 ${result.identifier}`}
            disabled={addedIdentifier === result.identifier}
            onClick={handleAddFriend}
          >
            {addedIdentifier === result.identifier ? '已添加' : '添加好友'}
          </button>
        </article>
      ) : null}
    </section>
  );
}

function ContactEntryButton({ entry, onClick }: { entry: ContactEntry; onClick: () => void }) {
  const Icon = entry.icon;

  return (
    <button className="action-row row-button" type="button" aria-label={entry.label} onClick={onClick}>
      <div className={`action-icon action-${entry.accent}`}>
        <Icon size={19} />
      </div>
      <div className="row-main">
        <strong>{entry.label}</strong>
        <p>{entry.helper}</p>
      </div>
      <ChevronRight size={18} />
    </button>
  );
}

function FriendDirectory({ friends: directoryFriends }: { friends: Friend[] }) {
  const groups = useMemo(() => groupFriends(directoryFriends), [directoryFriends]);

  return (
    <section aria-label="好友列表">
      {groups.map(([initial, groupedFriends]) => (
        <section className="friend-group" aria-labelledby={`friend-group-${initial}`} key={initial}>
          <h2 className="section-label" id={`friend-group-${initial}`}>
            {initial}
          </h2>
          <div className="list-card">
            {groupedFriends.map((friend) => (
              <article className="friend-row" key={friend.id}>
                <div className="avatar avatar-blue">{friend.avatar}</div>
                <div>
                  <strong>{friend.name}</strong>
                  <p>{friend.identifier}</p>
                </div>
              </article>
            ))}
          </div>
        </section>
      ))}
    </section>
  );
}

function GroupsPanel({ onBack }: { onBack: () => void }) {
  const [groups, setGroups] = useState<GroupView[]>(initialGroups);
  const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null);
  const [groupName, setGroupName] = useState('');
  const [joinGroupId, setJoinGroupId] = useState('');
  const [notice, setNotice] = useState('可创建群或输入群 ID 加入群聊');
  const selectedGroup = groups.find((group) => group.id === selectedGroupId) ?? null;

  function handleCreateGroup(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const name = groupName.trim();

    if (!name) {
      setNotice('请输入群名称');
      return;
    }

    const group: GroupView = {
      id: `grp_mock_${groups.length + 1}`,
      name,
      description: '本地 mock 群聊',
      members: [currentUser],
    };
    setGroups((items) => [group, ...items]);
    setGroupName('');
    setSelectedGroupId(group.id);
    setNotice(`已创建 ${group.name}`);
  }

  function handleJoinGroup(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const groupId = joinGroupId.trim();

    if (!groupId) {
      setNotice('请输入群 ID');
      return;
    }

    const existingGroup = groups.find((group) => group.id === groupId);
    if (existingGroup) {
      setGroups((items) =>
        items.map((group) =>
          group.id === groupId && !group.members.some((member) => member.userId === currentUser.userId)
            ? { ...group, members: [currentUser, ...group.members] }
            : group,
        ),
      );
      setSelectedGroupId(existingGroup.id);
      setNotice(`已加入 ${existingGroup.name}`);
      setJoinGroupId('');
      return;
    }

    const joinedGroup: GroupView = {
      id: groupId,
      name: groupId,
      description: '通过群 ID 加入的本地 mock 群聊',
      members: [currentUser],
    };
    setGroups((items) => [joinedGroup, ...items]);
    setSelectedGroupId(joinedGroup.id);
    setJoinGroupId('');
    setNotice(`已加入 ${joinedGroup.name}`);
  }

  return (
    <section className="group-workspace" aria-label="群聊工作区">
      <div className="panel-heading">
        <button className="icon-command" type="button" aria-label="返回联系人列表" onClick={onBack}>
          <ChevronLeft size={19} />
        </button>
        <h2>群聊</h2>
      </div>

      <div className="group-actions">
        <form className="mini-form" aria-label="创建群" onSubmit={handleCreateGroup}>
          <label>
            <span>群名称</span>
            <input value={groupName} onChange={(event) => setGroupName(event.target.value)} />
          </label>
          <button className="compact-command" type="submit" aria-label="创建群">
            <Plus size={17} />
            <span>创建</span>
          </button>
        </form>
        <form className="mini-form" aria-label="加入群" onSubmit={handleJoinGroup}>
          <label>
            <span>群 ID</span>
            <input value={joinGroupId} onChange={(event) => setJoinGroupId(event.target.value)} />
          </label>
          <button className="compact-command" type="submit" aria-label="加入群">
            <Check size={17} />
            <span>加入</span>
          </button>
        </form>
        <p className="helper-line">{notice}</p>
      </div>

      <section className="list-card" aria-label="群聊列表">
        {groups.map((group) => (
          <button
            className="group-row row-button"
            type="button"
            key={group.id}
            aria-label={`查看 ${group.name}`}
            onClick={() => setSelectedGroupId(group.id)}
          >
            <div className="avatar avatar-green">
              <UsersRound size={19} />
            </div>
            <div className="row-main">
              <strong>{group.name}</strong>
              <p>
                {group.id} · {group.members.length} 位成员
              </p>
            </div>
            <ChevronRight size={18} />
          </button>
        ))}
      </section>

      {selectedGroup ? <GroupDetail group={selectedGroup} /> : null}
    </section>
  );
}

function GroupDetail({ group }: { group: GroupView }) {
  return (
    <section className="group-detail" aria-label="群详情">
      <div className="group-detail-head">
        <div className="avatar avatar-green avatar-large">
          <Hash size={25} />
        </div>
        <div className="row-main">
          <h2>{group.name}</h2>
          <p>{group.description}</p>
        </div>
      </div>
      <ul className="member-list" aria-label="群成员列表">
        {group.members.map((member) => (
          <li className="member-row" key={member.userId}>
            <div className="avatar avatar-blue">
              <UserRound size={18} />
            </div>
            <div>
              <strong>{member.name}</strong>
              <p>{member.identifier}</p>
            </div>
          </li>
        ))}
      </ul>
    </section>
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

export default ContactsPage;
