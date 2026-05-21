import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { Activity, Bot, Database, MessageSquareText, Search, Users } from 'lucide-react';
import {
  createAdminApi,
  type AdminApi,
  type AdminConversation,
  type AdminConversationMessagesResponse,
  type AdminDashboard,
  type AdminLLMTrace,
  type AdminUser,
  type AdminUserConversationsResponse,
  type AdminUserDetailResponse,
  type AdminUserFriendsResponse,
  type AdminUserSearchResponse,
} from '../api/admin';

type AdminView = 'dashboard' | 'traces' | 'conversation' | 'users';

type AdminConsoleProps = {
  adminApi?: AdminApi;
};

const navItems: Array<{ key: AdminView; label: string }> = [
  { key: 'dashboard', label: 'Dashboard' },
  { key: 'traces', label: 'LLM Traces' },
  { key: 'conversation', label: 'Conversation' },
  { key: 'users', label: 'Users' },
];

export function AdminConsole({ adminApi }: AdminConsoleProps) {
  const api = useMemo(() => adminApi ?? createAdminApi(), [adminApi]);
  const [activeView, setActiveView] = useState<AdminView>('dashboard');
  const [dashboard, setDashboard] = useState<AdminDashboard | null>(null);
  const [dashboardLoading, setDashboardLoading] = useState(true);
  const [dashboardError, setDashboardError] = useState('');
  const [traceList, setTraceList] = useState<AdminLLMTrace[]>([]);
  const [traceLoading, setTraceLoading] = useState(false);
  const [traceError, setTraceError] = useState('');
  const [selectedTrace, setSelectedTrace] = useState<AdminLLMTrace | null>(null);
  const [conversationID, setConversationID] = useState('');
  const [conversation, setConversation] = useState<AdminConversationMessagesResponse | null>(null);
  const [conversationLoading, setConversationLoading] = useState(false);
  const [conversationError, setConversationError] = useState('');
  const [userQuery, setUserQuery] = useState('');
  const [userResults, setUserResults] = useState<AdminUserSearchResponse | null>(null);
  const [userDetail, setUserDetail] = useState<AdminUserDetailResponse | null>(null);
  const [userFriends, setUserFriends] = useState<AdminUserFriendsResponse | null>(null);
  const [userConversations, setUserConversations] = useState<AdminUserConversationsResponse | null>(null);
  const [userLoading, setUserLoading] = useState(false);
  const [userError, setUserError] = useState('');

  useEffect(() => {
    let active = true;
    setDashboardLoading(true);
    setDashboardError('');
    api
      .getDashboard()
      .then((data) => {
        if (!active) {
          return;
        }
        setDashboard(data);
      })
      .catch(() => {
        if (active) {
          setDashboardError('Could not load admin dashboard');
        }
      })
      .finally(() => {
        if (active) {
          setDashboardLoading(false);
        }
      });
    return () => {
      active = false;
    };
  }, [api]);

  async function loadTraces() {
    setTraceLoading(true);
    setTraceError('');
    try {
      const data = await api.listLLMTraces();
      setTraceList(data.traces);
    } catch {
      setTraceError('Could not load LLM traces');
    } finally {
      setTraceLoading(false);
    }
  }

  async function openTrace(trace: AdminLLMTrace) {
    setTraceError('');
    try {
      const detail = await api.getLLMTraceDetail(trace.traceId);
      setSelectedTrace(detail.trace);
    } catch {
      setTraceError('Could not load trace detail');
    }
  }

  async function loadConversation(nextConversationID = conversationID) {
    const trimmed = nextConversationID.trim();
    if (!trimmed) {
      setConversationError('Conversation ID is required');
      return;
    }
    setConversationID(trimmed);
    setConversationLoading(true);
    setConversationError('');
    setConversation(null);
    setActiveView('conversation');
    try {
      const data = await api.getConversationMessages(trimmed);
      setConversation(data);
    } catch {
      setConversationError('Could not load conversation');
    } finally {
      setConversationLoading(false);
    }
  }

  async function searchUsers(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await loadUsers(userQuery);
  }

  async function loadUsers(query: string) {
    setUserLoading(true);
    setUserError('');
    setUserDetail(null);
    setUserFriends(null);
    setUserConversations(null);
    try {
      const data = await api.searchUsers(query.trim());
      setUserResults(data);
    } catch {
      setUserError('Could not search users');
    } finally {
      setUserLoading(false);
    }
  }

  async function openUser(user: AdminUser) {
    setUserLoading(true);
    setUserError('');
    setUserResults({ users: [] });
    try {
      const [detail, friends, conversations] = await Promise.all([
        api.getUserDetail(user.userId),
        api.getUserFriends(user.userId),
        api.getUserConversations(user.userId),
      ]);
      setUserDetail(detail);
      setUserFriends(friends);
      setUserConversations(conversations);
    } catch {
      setUserError('Could not load user detail');
    } finally {
      setUserLoading(false);
    }
  }

  function switchView(view: AdminView) {
    setActiveView(view);
    if (view === 'traces' && traceList.length === 0 && !traceLoading) {
      void loadTraces();
    }
    if (view === 'users' && userResults === null && !userLoading) {
      void loadUsers('');
    }
  }

  return (
    <main className="admin-shell">
      <aside className="admin-sidebar" aria-label="Admin navigation">
        <div>
          <p className="admin-kicker">Agents IM</p>
          <h1>Admin Console</h1>
        </div>
        <nav className="admin-nav">
          {navItems.map((item) => (
            <button
              type="button"
              className={item.key === activeView ? 'admin-nav-button active' : 'admin-nav-button'}
              onClick={() => switchView(item.key)}
              key={item.key}
            >
              {item.label}
            </button>
          ))}
        </nav>
      </aside>

      <section className="admin-main">
        {activeView === 'dashboard' && renderDashboard(dashboard, dashboardLoading, dashboardError, loadConversation)}
        {activeView === 'traces' &&
          renderTraces({
            traces: traceList,
            loading: traceLoading,
            error: traceError,
            selectedTrace,
            onOpenTrace: openTrace,
            onOpenConversation: loadConversation,
          })}
        {activeView === 'conversation' &&
          renderConversation({
            conversationID,
            setConversationID,
            conversation,
            conversations: dashboard?.recentConversations ?? [],
            loading: conversationLoading,
            error: conversationError,
            listLoading: dashboardLoading,
            listError: dashboardError,
            onSubmit: (event) => {
              event.preventDefault();
              void loadConversation();
            },
            onOpenConversation: loadConversation,
          })}
        {activeView === 'users' &&
          renderUsers({
            userQuery,
            setUserQuery,
            userResults,
            userDetail,
            userFriends,
            userConversations,
            loading: userLoading,
            error: userError,
            onSearch: searchUsers,
            onOpenUser: openUser,
            onOpenConversation: loadConversation,
          })}
      </section>
    </main>
  );
}

function renderDashboard(
  dashboard: AdminDashboard | null,
  loading: boolean,
  error: string,
  onOpenConversation: (conversationID: string) => void,
) {
  if (loading) {
    return <p className="admin-status">Loading admin dashboard</p>;
  }
  if (error) {
    return <p className="admin-error">{error}</p>;
  }
  if (!dashboard) {
    return <p className="admin-empty">No dashboard data</p>;
  }

  const cards = [
    { label: 'Total users', value: dashboard.totals.users, icon: Users },
    { label: 'Conversations', value: dashboard.totals.conversations, icon: MessageSquareText },
    { label: 'Messages', value: dashboard.totals.messages, icon: Database },
    { label: 'AI runs', value: dashboard.totals.aiRuns, icon: Bot },
    { label: 'Failed AI runs', value: dashboard.totals.failedAiRuns, icon: Activity },
  ];

  return (
    <div className="admin-page">
      <header className="admin-page-header">
        <h2>Dashboard</h2>
      </header>
      <section className="admin-stat-grid" aria-label="Overview cards">
        {cards.map((card) => {
          const Icon = card.icon;
          return (
            <article className="admin-stat-card" key={card.label}>
              <Icon aria-hidden="true" size={18} />
              <span>{card.label}</span>
              <strong>{card.value}</strong>
            </article>
          );
        })}
      </section>
      <section className="admin-grid-two">
        <section>
          <h3>Recent traces</h3>
          {dashboard.recentTraces.length === 0 ? (
            <p className="admin-empty">No traces found</p>
          ) : (
            <div className="admin-table" role="table" aria-label="Recent traces">
              {dashboard.recentTraces.map((trace) => (
                <button type="button" className="admin-row" key={trace.traceId} onClick={() => trace.conversationId && onOpenConversation(trace.conversationId)}>
                  <span>{trace.traceId}</span>
                  <span>{trace.status}</span>
                  <span>{trace.conversationId || 'No conversation'}</span>
                </button>
              ))}
            </div>
          )}
        </section>
        <section>
          <h3>Recent conversations</h3>
          {dashboard.recentConversations.length === 0 ? (
            <p className="admin-empty">No active conversations</p>
          ) : (
            <div className="admin-table" role="table" aria-label="Recent conversations">
              {dashboard.recentConversations.map((conversation) => (
                <button
                  type="button"
                  className="admin-row"
                  key={conversation.conversationId}
                  onClick={() => onOpenConversation(conversation.conversationId)}
                >
                  <span>{conversation.conversationId}</span>
                  <span>max seq {conversation.maxSeq}</span>
                </button>
              ))}
            </div>
          )}
        </section>
      </section>
    </div>
  );
}

function renderTraces({
  traces,
  loading,
  error,
  selectedTrace,
  onOpenTrace,
  onOpenConversation,
}: {
  traces: AdminLLMTrace[];
  loading: boolean;
  error: string;
  selectedTrace: AdminLLMTrace | null;
  onOpenTrace: (trace: AdminLLMTrace) => void;
  onOpenConversation: (conversationID: string) => void;
}) {
  return (
    <div className="admin-page">
      <header className="admin-page-header">
        <h2>LLM Traces</h2>
      </header>
      {loading && <p className="admin-status">Loading LLM traces</p>}
      {error && <p className="admin-error">{error}</p>}
      {!loading && traces.length === 0 && <p className="admin-empty">No traces found</p>}
      <div className="admin-table" role="table" aria-label="LLM trace list">
        {traces.map((trace) => (
          <button type="button" className="admin-row" key={trace.traceId} onClick={() => onOpenTrace(trace)}>
            <span>{trace.traceId}</span>
            <span>{trace.status}</span>
            <span>{trace.model || trace.provider || 'model unknown'}</span>
            <span>{trace.conversationId || 'No conversation'}</span>
          </button>
        ))}
      </div>
      {selectedTrace && (
        <section className="admin-detail-panel">
          <h3>{selectedTrace.traceId}</h3>
          <p>{selectedTrace.runId}</p>
          <p>{selectedTrace.errorMessage || selectedTrace.status}</p>
          {selectedTrace.jaegerUrl && (
            <a className="admin-secondary-button" href={selectedTrace.jaegerUrl} target="_blank" rel="noreferrer">
              Open in Jaeger
            </a>
          )}
          {selectedTrace.conversationId && (
            <button type="button" className="admin-secondary-button" onClick={() => onOpenConversation(selectedTrace.conversationId!)}>
              Open conversation
            </button>
          )}
        </section>
      )}
    </div>
  );
}

function renderConversation({
  conversationID,
  setConversationID,
  conversation,
  conversations,
  loading,
  error,
  listLoading,
  listError,
  onSubmit,
  onOpenConversation,
}: {
  conversationID: string;
  setConversationID: (value: string) => void;
  conversation: AdminConversationMessagesResponse | null;
  conversations: AdminConversation[];
  loading: boolean;
  error: string;
  listLoading: boolean;
  listError: string;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onOpenConversation: (conversationID: string) => void;
}) {
  return (
    <div className="admin-page">
      <header className="admin-page-header">
        <h2>Conversation Inspector</h2>
      </header>
      <form className="admin-inline-form" onSubmit={onSubmit}>
        <label>
          <span>Conversation ID</span>
          <input value={conversationID} onChange={(event) => setConversationID(event.target.value)} />
        </label>
        <button type="submit" className="admin-primary-button">
          <Search aria-hidden="true" size={16} />
          Load conversation
        </button>
      </form>
      {loading && <p className="admin-status">Loading conversation</p>}
      {error && <p className="admin-error">{error}</p>}
      <section className="admin-detail-panel">
        <h3>Recent conversations</h3>
        {listLoading && <p className="admin-status">Loading conversation list</p>}
        {listError && <p className="admin-error">Could not load conversation list</p>}
        {!listLoading && !listError && conversations.length === 0 && (
          <p className="admin-empty">No active conversations</p>
        )}
        {!listLoading && !listError && conversations.length > 0 && (
          <div className="admin-table" role="table" aria-label="Conversation browse list">
            {conversations.map((item) => (
              <button
                type="button"
                className="admin-row admin-row-conversation"
                key={item.conversationId}
                onClick={() => onOpenConversation(item.conversationId)}
                aria-label={`Open conversation ${item.conversationId}`}
              >
                <span>{item.conversationId}</span>
                <span>max seq {item.maxSeq}</span>
                <span>{conversationSummary(item)}</span>
                <span>{formatAdminTimestamp(item.maxSeqTime || item.lastMessage?.createdAt || item.lastMessage?.sendTime)}</span>
              </button>
            ))}
          </div>
        )}
      </section>
      {conversation && conversation.messages.length === 0 && <p className="admin-empty">No messages found</p>}
      {conversation && conversation.messages.length > 0 && (
        <div className="admin-message-list" aria-label="Conversation messages">
          {conversation.messages.map((message) => (
            <article className="admin-message-row" key={message.serverMsgId}>
              <div>
                <strong>seq {message.seq}</strong>
                <span>{message.messageOrigin}</span>
                {message.agentRunId && <span>{message.agentRunId}</span>}
              </div>
              <p>{message.content}</p>
              <small>{message.senderId}</small>
            </article>
          ))}
        </div>
      )}
    </div>
  );
}

function renderUsers({
  userQuery,
  setUserQuery,
  userResults,
  userDetail,
  userFriends,
  userConversations,
  loading,
  error,
  onSearch,
  onOpenUser,
  onOpenConversation,
}: {
  userQuery: string;
  setUserQuery: (value: string) => void;
  userResults: AdminUserSearchResponse | null;
  userDetail: AdminUserDetailResponse | null;
  userFriends: AdminUserFriendsResponse | null;
  userConversations: AdminUserConversationsResponse | null;
  loading: boolean;
  error: string;
  onSearch: (event: FormEvent<HTMLFormElement>) => void;
  onOpenUser: (user: AdminUser) => void;
  onOpenConversation: (conversationID: string) => void;
}) {
  return (
    <div className="admin-page">
      <header className="admin-page-header">
        <h2>User Inspector</h2>
      </header>
      <form className="admin-inline-form" onSubmit={onSearch}>
        <label>
          <span>User query</span>
          <input value={userQuery} onChange={(event) => setUserQuery(event.target.value)} />
        </label>
        <button type="submit" className="admin-primary-button">
          <Search aria-hidden="true" size={16} />
          Search users
        </button>
      </form>
      {loading && <p className="admin-status">Loading user data</p>}
      {error && <p className="admin-error">{error}</p>}
      {userResults && userResults.users.length === 0 && !userDetail && <p className="admin-empty">No users found</p>}
      {userResults && userResults.users.length > 0 && (
        <div className="admin-table" role="table" aria-label="User browse results">
          {userResults.users.map((user) => (
            <button
              type="button"
              className="admin-row admin-row-user"
              key={user.userId}
              onClick={() => onOpenUser(user)}
              aria-label={`Open ${user.identifier}`}
            >
              <span>{displayUserName(user)}</span>
              <span>{user.identifier}</span>
              <span>{user.accountType}</span>
              <span>{user.region || user.gender || 'No profile metadata'}</span>
              <span>{user.createdAt || user.updatedAt || 'No timestamp'}</span>
            </button>
          ))}
        </div>
      )}
      {userDetail && (
        <section className="admin-detail-panel">
          <h3>{userDetail.user.displayName || userDetail.user.identifier}</h3>
          <dl className="admin-definition-grid">
            <dt>Identifier</dt>
            <dd>{userDetail.user.identifier}</dd>
            <dt>Type</dt>
            <dd>{userDetail.user.accountType}</dd>
            <dt>Region</dt>
            <dd>{userDetail.user.region || 'Empty'}</dd>
          </dl>
        </section>
      )}
      {userFriends && (
        <section className="admin-detail-panel">
          <h3>Friends</h3>
          {userFriends.friends.length === 0 ? (
            <p className="admin-empty">No accepted friends</p>
          ) : (
            userFriends.friends.map((friend) => (
              <div className="admin-row-static" key={friend.friendId}>
                <span>{friend.friend?.identifier || friend.friendId}</span>
                <span>{friend.status}</span>
              </div>
            ))
          )}
        </section>
      )}
      {userConversations && (
        <section className="admin-detail-panel" role="region" aria-label="User conversations">
          <h3>Conversations</h3>
          {userConversations.conversations.length === 0 ? (
            <p className="admin-empty">No conversations found</p>
          ) : (
            userConversations.conversations.map((conversation) => (
              <button
                type="button"
                className="admin-row"
                key={conversation.conversationId}
                onClick={() => onOpenConversation(conversation.conversationId)}
                aria-label={`Open ${conversation.conversationId}`}
              >
                <span>{conversation.conversationId}</span>
                <span>max seq {conversation.maxSeq}</span>
              </button>
            ))
          )}
        </section>
      )}
    </div>
  );
}

function displayUserName(user: AdminUser) {
  return user.displayName || user.name || user.identifier;
}

function conversationSummary(conversation: AdminConversation) {
  const content = conversation.lastMessage?.content.trim();
  if (content) {
    return content;
  }
  if (conversation.lastMessage?.contentType) {
    return `${conversation.lastMessage.contentType} message`;
  }
  return 'No message summary';
}

function formatAdminTimestamp(value?: number) {
  if (!value || value <= 0) {
    return 'No timestamp';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return 'No timestamp';
  }
  return date.toISOString();
}

export default AdminConsole;
