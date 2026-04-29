import { useMemo, useState, type ComponentType, type FormEvent } from 'react';
import {
  Bell,
  ChevronRight,
  Compass,
  Contact,
  LogOut,
  MessageCircle,
  MoreHorizontal,
  Plus,
  Search,
  Settings,
  ShieldCheck,
  UserRound,
  UsersRound,
} from 'lucide-react';
import { AuthProvider, authErrorMessage, useAuth } from './auth/AuthContext';
import type { AuthUser } from './auth/session';

type TabKey = 'messages' | 'contacts' | 'discover' | 'me';

type TabDefinition = {
  key: TabKey;
  label: string;
  icon: ComponentType<{ size?: number; strokeWidth?: number }>;
};

const tabs: TabDefinition[] = [
  { key: 'messages', label: '消息', icon: MessageCircle },
  { key: 'contacts', label: '联系人', icon: Contact },
  { key: 'discover', label: '发现', icon: Compass },
  { key: 'me', label: '我的', icon: UserRound },
];

const conversations = [
  {
    id: 'product-room',
    title: '产品讨论群',
    avatar: '产',
    preview: '后端 MVP 已发布，开始搭前端主框架。',
    time: '20:08',
    unread: 3,
    color: 'green',
  },
  {
    id: 'junhui',
    title: 'junhui',
    avatar: 'J',
    preview: '参考微信，先做四个主页面。',
    time: '19:46',
    unread: 1,
    color: 'blue',
  },
  {
    id: 'agent',
    title: 'Agent 助手',
    avatar: 'AI',
    preview: '我可以帮你整理联系人和群聊。',
    time: '昨天',
    unread: 0,
    color: 'purple',
  },
];

const contacts = [
  { id: 'new', label: '新的朋友', helper: '好友申请与推荐', accent: 'orange' },
  { id: 'groups', label: '群聊', helper: '产品讨论群、Agent 群', accent: 'green' },
  { id: 'tags', label: '标签', helper: '按角色整理联系人', accent: 'blue' },
  { id: 'official', label: '公众号', helper: '系统通知与服务号', accent: 'gray' },
];

const friends = [
  { id: 'alice', name: 'Alice Chen', identifier: 'alice_001', initial: 'A' },
  { id: 'bob', name: 'Bob Lin', identifier: 'bob_002', initial: 'B' },
  { id: 'agent', name: 'Agent 助手', identifier: 'agent_helper', initial: 'AI' },
];

function App() {
  return (
    <AuthProvider>
      <AuthGate />
    </AuthProvider>
  );
}

function AuthGate() {
  const { session } = useAuth();

  if (!session) {
    return <AuthPage />;
  }

  return <AuthenticatedApp user={session.user} />;
}

function AuthenticatedApp({ user }: { user: AuthUser }) {
  const [activeTab, setActiveTab] = useState<TabKey>('messages');
  const activeLabel = useMemo(() => tabs.find((tab) => tab.key === activeTab)?.label ?? '消息', [activeTab]);

  return (
    <main className="app-shell" aria-label="Agents IM 微信风格主框架">
      <section className="phone-frame">
        <header className="top-bar">
          <h1>{activeLabel}</h1>
          <div className="top-actions" aria-label="页面操作">
            <button type="button" aria-label="搜索">
              <Search size={20} />
            </button>
            <button type="button" aria-label="新增">
              <Plus size={21} />
            </button>
          </div>
        </header>

        <section className="content-area">{renderPage(activeTab, user)}</section>

        <nav className="tab-bar" role="tablist" aria-label="主导航">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            const selected = activeTab === tab.key;
            return (
              <button
                key={tab.key}
                type="button"
                role="tab"
                aria-selected={selected}
                className={selected ? 'tab-item active' : 'tab-item'}
                onClick={() => setActiveTab(tab.key)}
              >
                <Icon size={24} strokeWidth={selected ? 2.6 : 2.1} />
                <span>{tab.label}</span>
              </button>
            );
          })}
        </nav>
      </section>
    </main>
  );
}

function AuthPage() {
  const { login, register } = useAuth();
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [identifier, setIdentifier] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const isRegister = mode === 'register';

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');
    setSubmitting(true);

    try {
      if (isRegister) {
        await register({
          identifier: identifier.trim(),
          displayName: displayName.trim(),
          password,
        });
      } else {
        await login({
          identifier: identifier.trim(),
          password,
        });
      }
    } catch (caughtError) {
      setError(authErrorMessage(caughtError));
    } finally {
      setSubmitting(false);
    }
  }

  function switchMode(nextMode: 'login' | 'register') {
    setMode(nextMode);
    setError('');
  }

  return (
    <main className="app-shell auth-app-shell" aria-label="Agents IM 认证">
      <section className="phone-frame auth-frame">
        <div className="auth-hero">
          <div className="avatar avatar-green avatar-large">
            <ShieldCheck size={30} />
          </div>
          <div>
            <p className="auth-kicker">Agents IM</p>
            <h1>{isRegister ? '注册 Agents IM' : '登录 Agents IM'}</h1>
          </div>
        </div>

        <form className="auth-form" onSubmit={handleSubmit}>
          <label className="auth-field" htmlFor="auth-identifier">
            <span>账号</span>
            <input
              id="auth-identifier"
              value={identifier}
              onChange={(event) => setIdentifier(event.target.value)}
              autoComplete="username"
              required
            />
          </label>

          {isRegister ? (
            <label className="auth-field" htmlFor="auth-display-name">
              <span>昵称</span>
              <input
                id="auth-display-name"
                value={displayName}
                onChange={(event) => setDisplayName(event.target.value)}
                autoComplete="nickname"
                required
              />
            </label>
          ) : null}

          <label className="auth-field" htmlFor="auth-password">
            <span>密码</span>
            <input
              id="auth-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              type="password"
              autoComplete={isRegister ? 'new-password' : 'current-password'}
              required
            />
          </label>

          {error ? <p className="auth-error" role="alert">{error}</p> : null}

          <button className="auth-submit" type="submit" disabled={submitting}>
            {submitting ? '请稍候' : isRegister ? '注册并登录' : '登录'}
          </button>
        </form>

        <div className="auth-switch">
          {isRegister ? (
            <button type="button" onClick={() => switchMode('login')}>
              返回登录
            </button>
          ) : (
            <button type="button" onClick={() => switchMode('register')}>
              注册账号
            </button>
          )}
        </div>
      </section>
    </main>
  );
}

function renderPage(tab: TabKey, user: AuthUser) {
  switch (tab) {
    case 'contacts':
      return <ContactsPage />;
    case 'discover':
      return <DiscoverPage />;
    case 'me':
      return <MePage user={user} />;
    case 'messages':
    default:
      return <MessagesPage />;
  }
}

function MessagesPage() {
  return (
    <div className="page-stack">
      <SearchBox placeholder="搜索" />
      <section className="list-card conversation-list" aria-label="消息列表">
        {conversations.map((item) => (
          <article className="conversation-row" key={item.id}>
            <div className={`avatar avatar-${item.color}`}>{item.avatar}</div>
            <div className="row-main">
              <div className="row-title-line">
                <strong>{item.title}</strong>
                <time>{item.time}</time>
              </div>
              <p>{item.preview}</p>
            </div>
            {item.unread > 0 ? <span className="unread-badge">{item.unread}</span> : null}
          </article>
        ))}
      </section>
    </div>
  );
}

function ContactsPage() {
  return (
    <div className="page-stack">
      <SearchBox placeholder="搜索联系人、账号或群聊" />
      <section className="list-card" aria-label="联系人快捷入口">
        {contacts.map((item) => (
          <ActionRow key={item.id} label={item.label} helper={item.helper} accent={item.accent} />
        ))}
      </section>
      <p className="section-label">A</p>
      <section className="list-card" aria-label="好友列表">
        {friends.map((friend) => (
          <article className="friend-row" key={friend.id}>
            <div className="avatar avatar-blue">{friend.initial}</div>
            <div>
              <strong>{friend.name}</strong>
              <p>{friend.identifier}</p>
            </div>
          </article>
        ))}
      </section>
    </div>
  );
}

function DiscoverPage() {
  return (
    <div className="page-stack discover-page">
      <section className="list-card">
        <ActionRow label="朋友圈" helper="动态、图片和 Agent 工作流分享" accent="blue" />
      </section>
      <section className="list-card">
        <ActionRow label="扫一扫" helper="扫码登录、加好友和加入群聊预留" accent="green" />
        <ActionRow label="摇一摇" helper="后续探索式匹配入口" accent="purple" />
      </section>
      <section className="list-card">
        <ActionRow label="小程序" helper="Agent 插件和工具入口" accent="orange" />
      </section>
    </div>
  );
}

function MePage({ user }: { user: AuthUser }) {
  const { logout } = useAuth();

  return (
    <div className="page-stack me-page">
      <section className="profile-card">
        <div className="avatar avatar-green avatar-large">{avatarText(user.displayName)}</div>
        <div className="profile-main">
          <strong>{user.displayName}</strong>
          <p>微信号：{user.identifier}</p>
          <p>地区：{user.region || '未设置'}</p>
          <p>用户 ID：{user.userId}</p>
        </div>
        <ChevronRight size={20} />
      </section>
      <section className="list-card">
        <ActionRow label="服务" helper="钱包、收藏、卡包等能力预留" accent="green" />
      </section>
      <section className="list-card">
        <ActionRow label="收藏" helper="重要消息和 Agent 输出" accent="orange" />
        <ActionRow label="朋友圈" helper="我的动态" accent="blue" />
        <ActionRow label="设置" helper="账号、安全、通知" accent="gray" trailingIcon={Settings} />
      </section>
      <button className="logout-button" type="button" onClick={logout}>
        <LogOut size={18} />
        <span>退出登录</span>
      </button>
    </div>
  );
}

function SearchBox({ placeholder }: { placeholder: string }) {
  return (
    <label className="search-box">
      <Search size={17} />
      <input placeholder={placeholder} aria-label={placeholder} />
    </label>
  );
}

function ActionRow({
  label,
  helper,
  accent,
  trailingIcon: TrailingIcon = ChevronRight,
}: {
  label: string;
  helper: string;
  accent: string;
  trailingIcon?: ComponentType<{ size?: number }>;
}) {
  return (
    <article className="action-row">
      <div className={`action-icon action-${accent}`}>
        {accent === 'gray' ? <MoreHorizontal size={19} /> : accent === 'green' ? <UsersRound size={19} /> : <Bell size={19} />}
      </div>
      <div className="row-main">
        <strong>{label}</strong>
        <p>{helper}</p>
      </div>
      <TrailingIcon size={18} />
    </article>
  );
}

function avatarText(displayName: string) {
  const trimmed = displayName.trim();
  if (!trimmed) {
    return '我';
  }
  return trimmed.slice(0, 2).toUpperCase();
}

export default App;
