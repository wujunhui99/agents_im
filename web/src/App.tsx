import { useMemo, useState } from 'react';
import {
  Bell,
  ChevronRight,
  Compass,
  Contact,
  MessageCircle,
  MoreHorizontal,
  Plus,
  Search,
  Settings,
  UserRound,
  UsersRound,
} from 'lucide-react';
import ContactsPage from './components/ContactsPage';

type TabKey = 'messages' | 'contacts' | 'discover' | 'me';

type TabDefinition = {
  key: TabKey;
  label: string;
  icon: React.ComponentType<{ size?: number; strokeWidth?: number }>;
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

function App() {
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

        <section className="content-area">{renderPage(activeTab)}</section>

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

function renderPage(tab: TabKey) {
  switch (tab) {
    case 'contacts':
      return <ContactsPage />;
    case 'discover':
      return <DiscoverPage />;
    case 'me':
      return <MePage />;
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

function MePage() {
  return (
    <div className="page-stack me-page">
      <section className="profile-card">
        <div className="avatar avatar-green avatar-large">J</div>
        <div className="profile-main">
          <strong>junhui</strong>
          <p>微信号：alice_001</p>
          <p>地区：Shanghai</p>
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
  trailingIcon?: React.ComponentType<{ size?: number }>;
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

export default App;
