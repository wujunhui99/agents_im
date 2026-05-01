import { useMemo, useState, type FormEvent } from 'react';
import { Compass, Contact, MessageCircle, ShieldCheck, UserRound } from 'lucide-react';
import { AuthProvider, authErrorMessage, useAuth } from './auth/AuthContext';
import type { AuthUser } from './auth/session';
import { createApiClient } from './api/client';
import { createContactsApi, type ContactsApi } from './api/contacts';
import { createMessageApi, type MessageApi } from './api/messages';
import { createUserApi, type UserApi, type UserProfile, type UserProfilePatch } from './api/user';
import ContactsPage from './components/ContactsPage';
import { TabBar, type TabDefinition } from './components/ui/TabBar';
import { TopBar } from './components/ui/TopBar';
import { MessagesPage } from './features/messages/MessagesPage';
import { DiscoverPage } from './pages/DiscoverPage';
import { MePage } from './pages/MePage';

type TabKey = 'messages' | 'contacts' | 'discover' | 'me';

const tabs: TabDefinition<TabKey>[] = [
  { key: 'messages', label: '消息', icon: MessageCircle },
  { key: 'contacts', label: '联系人', icon: Contact },
  { key: 'discover', label: '发现', icon: Compass },
  { key: 'me', label: '我的', icon: UserRound },
];

type AppProps = {
  initialUser?: UserProfile;
  userApi?: UserApi;
};

function App(props: AppProps) {
  return (
    <AuthProvider>
      <AuthGate {...props} />
    </AuthProvider>
  );
}

function AuthGate(props: AppProps) {
  const { session } = useAuth();

  if (!session) {
    return <AuthPage />;
  }

  return <AuthenticatedApp {...props} authUser={session.user} />;
}

type AuthenticatedAppProps = AppProps & {
  authUser: AuthUser;
};

function AuthenticatedApp({ authUser, initialUser, userApi }: AuthenticatedAppProps) {
  const [activeTab, setActiveTab] = useState<TabKey>('messages');
  const [currentUser, setCurrentUser] = useState<UserProfile>(() => initialUser ?? userProfileFromAuth(authUser));
  const [startChatSignal, setStartChatSignal] = useState(0);
  const activeLabel = useMemo(() => tabs.find((tab) => tab.key === activeTab)?.label ?? '消息', [activeTab]);
  const { session, logout } = useAuth();
  const effectiveUserApi = useMemo(
    () =>
      userApi ??
      createUserApi(
        createApiClient({
          getToken: () => session?.token,
        }),
      ),
    [session?.token, userApi],
  );
  const authedApiClient = useMemo(
    () =>
      createApiClient({
        getToken: () => session?.token,
      }),
    [session?.token],
  );
  const messageApi = useMemo(() => createMessageApi(authedApiClient), [authedApiClient]);
  const contactsApi = useMemo(() => createContactsApi(authedApiClient), [authedApiClient]);

  async function updateProfile(patch: UserProfilePatch) {
    const updatedUser = await effectiveUserApi.patchCurrentUser(patch);
    setCurrentUser(updatedUser);
  }

  return (
    <main className="app-shell" aria-label="Agents IM 微信风格主框架">
      <section className="phone-frame">
        <TopBar
          title={activeLabel}
          onAdd={activeTab === 'messages' ? () => setStartChatSignal((current) => current + 1) : undefined}
        />

        <section className="content-area">
          {renderPage(activeTab, currentUser, updateProfile, logout, effectiveUserApi, contactsApi, messageApi, startChatSignal)}
        </section>

        <TabBar tabs={tabs} activeTab={activeTab} onChange={setActiveTab} />
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
              已有账号，去登录
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

function renderPage(
  tab: TabKey,
  currentUser: UserProfile,
  onUpdateProfile: (patch: UserProfilePatch) => Promise<void>,
  onLogout: () => void,
  userApi: UserApi,
  contactsApi: ContactsApi,
  messageApi: MessageApi,
  startChatSignal: number,
) {
  if (tab === 'messages') {
    return (
      <MessagesPage
        currentUserId={currentUser.user_id}
        userApi={userApi}
        messageApi={messageApi}
        startChatSignal={startChatSignal}
      />
    );
  }

  if (tab === 'contacts') {
    return <ContactsPage userApi={userApi} contactsApi={contactsApi} />;
  }

  if (tab === 'discover') {
    return <DiscoverPage />;
  }

  return (
    <>
      <MePage profile={currentUser} onUpdateProfile={onUpdateProfile} />
      <button type="button" className="logout-button" onClick={onLogout}>
        退出登录
      </button>
    </>
  );
}

function userProfileFromAuth(user: AuthUser): UserProfile {
  return {
    user_id: user.userId,
    identifier: user.identifier,
    display_name: user.displayName,
    name: user.displayName,
    gender: user.gender ?? '',
    age: user.age ?? 0,
    region: user.region ?? '',
  };
}

export default App;
