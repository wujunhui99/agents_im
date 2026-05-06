import { useMemo, useRef, useState, type ChangeEvent, type FormEvent } from 'react';
import { Compass, Contact, MessageCircle, ShieldCheck, UserRound } from 'lucide-react';
import { AuthProvider, authErrorMessage, useAuth } from './auth/AuthContext';
import type { AuthUser } from './auth/session';
import { createApiClient } from './api/client';
import { createContactsApi, type ContactsApi } from './api/contacts';
import { createGroupsApi, type Group, type GroupsApi } from './api/groups';
import { createMediaApi, type MediaApi } from './api/media';
import { createMessageApi, type MessageApi } from './api/messages';
import { createUserApi, type UserApi, type UserProfile, type UserProfilePatch } from './api/user';
import ContactsPage from './components/ContactsPage';
import { Avatar } from './components/ui/Avatar';
import { Button } from './components/ui/Button';
import { TabBar, type TabDefinition } from './components/ui/TabBar';
import { TextField } from './components/ui/TextField';
import { TopBar } from './components/ui/TopBar';
import { MessagesPage } from './features/messages/MessagesPage';
import { DiscoverPage } from './pages/DiscoverPage';
import { MePage } from './pages/MePage';
import { uploadAvatarForProfile } from './utils/avatarUpload';

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
  webSocketToken?: string;
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

function AuthenticatedApp({ authUser, initialUser, userApi, webSocketToken }: AuthenticatedAppProps) {
  const [activeTab, setActiveTab] = useState<TabKey>('messages');
  const [mountedTabs, setMountedTabs] = useState<Set<TabKey>>(() => new Set(['messages']));
  const [currentUser, setCurrentUser] = useState<UserProfile>(() => initialUser ?? userProfileFromAuth(authUser));
  const [startChatSignal, setStartChatSignal] = useState(0);
  const [pendingChatProfile, setPendingChatProfile] = useState<UserProfile | null>(null);
  const [pendingGroup, setPendingGroup] = useState<Group | null>(null);
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
  const mediaApi = useMemo(() => createMediaApi(authedApiClient), [authedApiClient]);
  const contactsApi = useMemo(() => createContactsApi(authedApiClient), [authedApiClient]);
  const groupsApi = useMemo(() => createGroupsApi(authedApiClient), [authedApiClient]);

  async function updateProfile(patch: UserProfilePatch) {
    const updatedUser = await effectiveUserApi.patchCurrentUser(patch);
    setCurrentUser(updatedUser);
  }

  async function uploadAvatar(file: File) {
    const updatedUser = await uploadAvatarForProfile({
      file,
      mediaApi,
      userApi: effectiveUserApi,
    });
    setCurrentUser(updatedUser);
    return updatedUser;
  }

  function openChatFromContact(profile: UserProfile) {
    setPendingChatProfile({ ...profile });
    switchTab('messages');
  }

  function openGroupFromContact(group: Group) {
    setPendingGroup({ ...group });
    switchTab('messages');
  }

  function clearPendingChatProfile() {
    setPendingChatProfile((current) => {
      if (!current) {
        return current;
      }
      window.setTimeout(() => setPendingChatProfile(null), 0);
      return current;
    });
  }

  function clearPendingGroup() {
    setPendingGroup((current) => {
      if (!current) {
        return current;
      }
      window.setTimeout(() => setPendingGroup(null), 0);
      return current;
    });
  }

  function switchTab(nextTab: TabKey) {
    setActiveTab(nextTab);
    setMountedTabs((current) => {
      if (current.has(nextTab)) {
        return current;
      }
      return new Set(current).add(nextTab);
    });
  }

  return (
    <main className="app-shell" aria-label="Agents IM Material 3-inspired 微信式主框架">
      <section className="phone-frame">
        <TopBar
          title={activeLabel}
          onAdd={activeTab === 'messages' ? () => setStartChatSignal((current) => current + 1) : undefined}
        />

        <section className="content-area">
          {tabs.map((tab) => {
            const isActive = tab.key === activeTab;
            if (!isActive && !mountedTabs.has(tab.key)) {
              return null;
            }

            return (
              <section
                className={`tab-panel ${isActive ? 'tab-panel-active' : 'tab-panel-inactive'}`}
                data-active={isActive ? 'true' : 'false'}
                role="tabpanel"
                aria-label={tab.label}
                aria-hidden={isActive ? undefined : true}
                inert={isActive ? undefined : true}
                hidden={!isActive}
                key={tab.key}
              >
                {renderPage(
                  tab.key,
                  currentUser,
                  updateProfile,
                  logout,
                  effectiveUserApi,
                  contactsApi,
                  groupsApi,
                  messageApi,
                  mediaApi,
                  uploadAvatar,
                  webSocketToken ?? session?.token,
                  startChatSignal,
                  pendingChatProfile,
                  pendingGroup,
                  clearPendingChatProfile,
                  clearPendingGroup,
                  openChatFromContact,
                  openGroupFromContact,
                )}
              </section>
            );
          })}
        </section>

        <TabBar tabs={tabs} activeTab={activeTab} onChange={switchTab} />
      </section>
    </main>
  );
}

function AuthPage() {
  const { login, register } = useAuth();
  const loginUserApi = useMemo(() => createUserApi(createApiClient()), []);
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [identifier, setIdentifier] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [identifierCheckMessage, setIdentifierCheckMessage] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const identifierCheckRequest = useRef(0);
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
    identifierCheckRequest.current += 1;
    setMode(nextMode);
    setError('');
    setIdentifierCheckMessage('');
  }

  function handleIdentifierChange(event: ChangeEvent<HTMLInputElement>) {
    identifierCheckRequest.current += 1;
    setIdentifier(event.target.value);
    setIdentifierCheckMessage('');
  }

  async function checkLoginIdentifierExists() {
    if (isRegister) {
      return;
    }
    const query = identifier.trim();
    identifierCheckRequest.current += 1;
    const requestID = identifierCheckRequest.current;
    setIdentifierCheckMessage('');
    if (!query) {
      return;
    }

    try {
      const result = await loginUserApi.identifierExists(query);
      if (identifierCheckRequest.current !== requestID) {
        return;
      }
      setIdentifierCheckMessage(result.exists ? '' : '账号不存在，请检查后再输入密码');
    } catch {
      if (identifierCheckRequest.current !== requestID) {
        return;
      }
      setIdentifierCheckMessage('暂时无法确认账号是否存在，请稍后重试');
    }
  }

  return (
    <main className="app-shell auth-app-shell" aria-label="Agents IM 认证">
      <section className="phone-frame auth-frame">
        <div className="auth-hero">
          <Avatar label={<ShieldCheck size={30} />} color="green" size="large" />
          <div className="auth-hero-copy">
            <p className="auth-kicker">Agents IM</p>
            <h1>{isRegister ? '注册 Agents IM' : '登录 Agents IM'}</h1>
          </div>
        </div>

        <form className="auth-form" onSubmit={handleSubmit}>
          <TextField
            id="auth-identifier"
            label="账号"
            value={identifier}
            onChange={handleIdentifierChange}
            autoComplete="username"
            required
            fieldClassName="auth-field"
          />

          {isRegister ? (
            <TextField
              id="auth-display-name"
              label="昵称"
              value={displayName}
              onChange={(event) => setDisplayName(event.target.value)}
              autoComplete="nickname"
              required
              fieldClassName="auth-field"
            />
          ) : null}

          <TextField
            id="auth-password"
            label="密码"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            onFocus={checkLoginIdentifierExists}
            type="password"
            autoComplete={isRegister ? 'new-password' : 'current-password'}
            required
            fieldClassName="auth-field"
          />

          {!isRegister && identifierCheckMessage ? (
            <p className="auth-error" role="alert">
              {identifierCheckMessage}
            </p>
          ) : null}

          {error ? (
            <p className="auth-error" role="alert">
              {error}
            </p>
          ) : null}

          <Button className="auth-submit" type="submit" disabled={submitting}>
            {submitting ? '请稍候' : isRegister ? '注册并登录' : '登录'}
          </Button>
        </form>

        <div className="auth-switch">
          {isRegister ? (
            <Button variant="text" onClick={() => switchMode('login')}>
              已有账号，去登录
            </Button>
          ) : (
            <Button variant="text" onClick={() => switchMode('register')}>
              注册账号
            </Button>
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
  groupsApi: GroupsApi,
  messageApi: MessageApi,
  mediaApi: MediaApi,
  onUploadAvatar: (file: File) => Promise<UserProfile>,
  webSocketToken: string | undefined,
  startChatSignal: number,
  pendingChatProfile: UserProfile | null,
  pendingGroup: Group | null,
  onPendingChatConsumed: () => void,
  onPendingGroupConsumed: () => void,
  onOpenChatFromContact: (profile: UserProfile) => void,
  onOpenGroupFromContact: (group: Group) => void,
) {
  if (tab === 'messages') {
    return (
      <MessagesPage
        currentUserId={currentUser.user_id}
        userApi={userApi}
        messageApi={messageApi}
        mediaApi={mediaApi}
        contactsApi={contactsApi}
        groupsApi={groupsApi}
        webSocketToken={webSocketToken}
        startChatSignal={startChatSignal}
        pendingChatProfile={pendingChatProfile}
        pendingGroup={pendingGroup}
        onPendingChatConsumed={onPendingChatConsumed}
        onPendingGroupConsumed={onPendingGroupConsumed}
      />
    );
  }

  if (tab === 'contacts') {
    return (
      <ContactsPage
        userApi={userApi}
        contactsApi={contactsApi}
        groupsApi={groupsApi}
        onStartChat={onOpenChatFromContact}
        onOpenGroup={onOpenGroupFromContact}
      />
    );
  }

  if (tab === 'discover') {
    return <DiscoverPage />;
  }

  return (
    <>
      <MePage profile={currentUser} onUpdateProfile={onUpdateProfile} onUploadAvatar={onUploadAvatar} />
      <Button variant="tonal" className="logout-button" onClick={onLogout}>
        退出登录
      </Button>
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
    birth_date: user.birth_date ?? '',
    region: user.region ?? '',
  };
}

export default App;
