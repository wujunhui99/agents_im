import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent, type FormEvent } from 'react';
import { Compass, Contact, MessageCircle, ShieldCheck, UserRound } from 'lucide-react';
import { AuthProvider, authErrorMessage, useAuth } from './auth/AuthContext';
import type { AuthUser } from './auth/session';
import { createAdminApi } from './api/admin';
import { createApiClient } from './api/client';
import { createContactsApi, type ContactsApi } from './api/contacts';
import { createFeedbackApi, type SubmitFeedbackRequest } from './api/feedback';
import { createGroupsApi, type Group, type GroupsApi } from './api/groups';
import { createMediaApi, type MediaApi } from './api/media';
import { createMessageApi, type MessageApi } from './api/messages';
import { createUserApi, type UserApi, type UserProfile, type UserProfilePatch } from './api/user';
import type { WebSocketFactory } from './api/websocketClient';
import ContactsPage from './components/ContactsPage';
import { Avatar } from './components/ui/Avatar';
import { Button } from './components/ui/Button';
import { TabBar, type TabDefinition } from './components/ui/TabBar';
import { TextField } from './components/ui/TextField';
import { TopBar } from './components/ui/TopBar';
import { MessagesPage } from './features/messages/MessagesPage';
import { DiscoverPage } from './pages/DiscoverPage';
import { AdminConsole } from './pages/AdminConsole';
import { FeedbackPage } from './pages/FeedbackPage';
import { MePage } from './pages/MePage';
import { uploadAvatarForProfile } from './utils/avatarUpload';

type TabKey = 'messages' | 'contacts' | 'discover' | 'me';
type AppRoute = 'main' | 'feedback';

const tabs: TabDefinition<TabKey>[] = [
  { key: 'messages', label: '消息', icon: MessageCircle },
  { key: 'contacts', label: '联系人', icon: Contact },
  { key: 'discover', label: '发现', icon: Compass },
  { key: 'me', label: '我的', icon: UserRound },
];

type AppProps = {
  initialUser?: UserProfile;
  userApi?: UserApi;
  webSocketUrl?: string;
  webSocketToken?: string;
  webSocketFactory?: WebSocketFactory;
};

function App(props: AppProps) {
  return (
    <AuthProvider>
      <AuthGate {...props} />
    </AuthProvider>
  );
}

function AuthGate(props: AppProps) {
  const { authPrompt, handleAuthFailure, session } = useAuth();
  const adminApiClient = useMemo(
    () =>
      createApiClient({
        getToken: () => session?.token,
        onAuthFailure: handleAuthFailure,
      }),
    [handleAuthFailure, session?.token],
  );
  const adminApi = useMemo(() => createAdminApi(adminApiClient), [adminApiClient]);
  const adminMediaApi = useMemo(() => createMediaApi(adminApiClient), [adminApiClient]);

  if (isAdminRoute()) {
    if (!session || session.user.accountType !== 'admin') {
      return <AuthPage prompt={session ? '请使用管理员账号登录管理后台' : authPrompt} adminMode />;
    }
    return <AdminConsole adminApi={adminApi} mediaApi={adminMediaApi} />;
  }

  if (!session) {
    return <AuthPage prompt={authPrompt} />;
  }

  return <AuthenticatedApp {...props} authUser={session.user} />;
}

type AuthenticatedAppProps = AppProps & {
  authUser: AuthUser;
};

function AuthenticatedApp({ authUser, initialUser, userApi, webSocketUrl, webSocketToken, webSocketFactory }: AuthenticatedAppProps) {
  const [activeTab, setActiveTab] = useState<TabKey>('messages');
  const [mountedTabs, setMountedTabs] = useState<Set<TabKey>>(() => new Set(['messages']));
  const [appRoute, setAppRoute] = useState<AppRoute>(() => appRouteFromLocation());
  const [currentUser, setCurrentUser] = useState<UserProfile>(() => initialUser ?? userProfileFromAuth(authUser));
  const [startChatSignal, setStartChatSignal] = useState(0);
  const [pendingChatProfile, setPendingChatProfile] = useState<UserProfile | null>(null);
  const [pendingGroup, setPendingGroup] = useState<Group | null>(null);
  const activeLabel = useMemo(() => tabs.find((tab) => tab.key === activeTab)?.label ?? '消息', [activeTab]);
  const { handleAuthFailure, session, logout, updateSessionUser } = useAuth();
  const effectiveUserApi = useMemo(
    () =>
      userApi ??
      createUserApi(
        createApiClient({
          getToken: () => session?.token,
          onAuthFailure: handleAuthFailure,
        }),
      ),
    [handleAuthFailure, session?.token, userApi],
  );
  const authedApiClient = useMemo(
    () =>
      createApiClient({
        getToken: () => session?.token,
        onAuthFailure: handleAuthFailure,
      }),
    [handleAuthFailure, session?.token],
  );
  const messageApi = useMemo(() => createMessageApi(authedApiClient), [authedApiClient]);
  const feedbackApi = useMemo(() => createFeedbackApi(authedApiClient), [authedApiClient]);
  const mediaApi = useMemo(() => createMediaApi(authedApiClient), [authedApiClient]);
  const contactsApi = useMemo(() => createContactsApi(authedApiClient), [authedApiClient]);
  const groupsApi = useMemo(() => createGroupsApi(authedApiClient), [authedApiClient]);
  const adminApi = useMemo(() => createAdminApi(authedApiClient), [authedApiClient]);
  const activeWebSocketToken = webSocketToken ?? session?.token;
  const handleWebSocketAuthFailure = useCallback(() => {
    handleAuthFailure({ token: activeWebSocketToken ?? null });
  }, [activeWebSocketToken, handleAuthFailure]);

  useEffect(() => {
    function handlePopState() {
      setAppRoute(appRouteFromLocation());
    }

    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  async function updateProfile(patch: UserProfilePatch) {
    const updatedUser = await effectiveUserApi.patchCurrentUser(patch);
    setCurrentUser(updatedUser);
    updateSessionUser(authUserFromProfile(updatedUser));
  }

  async function uploadAvatar(file: File) {
    const updatedUser = await uploadAvatarForProfile({
      file,
      mediaApi,
      userApi: effectiveUserApi,
    });
    setCurrentUser(updatedUser);
    updateSessionUser(authUserFromProfile(updatedUser));
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

  function openFeedbackPage() {
    if (window.location.pathname !== '/feedback') {
      window.history.pushState({ route: 'feedback' }, '', '/feedback');
    }
    setAppRoute('feedback');
  }

  function closeFeedbackPage() {
    if (window.location.pathname === '/feedback' && window.history.length > 1) {
      window.history.back();
      return;
    }
    window.history.replaceState({}, '', '/');
    setAppRoute('main');
  }

  if (isAdminRoute()) {
    return <AdminConsole adminApi={adminApi} />;
  }

  if (appRoute === 'feedback') {
    return (
      <main className="app-shell" aria-label="Agents IM Material 3-inspired 微信式主框架">
        <section className="phone-frame">
          <TopBar title="反馈" onBack={closeFeedbackPage} />
          <section className="content-area">
            <FeedbackPage
              mediaApi={mediaApi}
              onSubmitFeedback={(request: SubmitFeedbackRequest) => feedbackApi.submitFeedback(request).then(() => undefined)}
            />
          </section>
        </section>
      </main>
    );
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
                  openFeedbackPage,
                  webSocketUrl,
                  activeWebSocketToken,
                  webSocketFactory,
                  handleWebSocketAuthFailure,
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

function isAdminRoute() {
  const { hostname, pathname } = window.location;
  return hostname === 'ms.agenticim.xyz' || pathname === '/admin' || pathname.startsWith('/admin/');
}

function appRouteFromLocation(): AppRoute {
  return window.location.pathname === '/feedback' ? 'feedback' : 'main';
}

function AuthPage({ prompt = '', adminMode = false }: { prompt?: string; adminMode?: boolean }) {
  const { login, register, requestRegistrationEmailCode } = useAuth();
  const loginUserApi = useMemo(() => createUserApi(createApiClient()), []);
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [identifier, setIdentifier] = useState('');
  const [email, setEmail] = useState('');
  const [emailVerificationCode, setEmailVerificationCode] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [emailCodeFeedback, setEmailCodeFeedback] = useState<{ kind: 'error' | 'success'; message: string } | null>(null);
  const [sendingEmailCode, setSendingEmailCode] = useState(false);
  const [identifierCheckFeedback, setIdentifierCheckFeedback] = useState<{
    kind: 'error' | 'hint';
    message: string;
  } | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const identifierCheckRequest = useRef(0);
  const isRegister = !adminMode && mode === 'register';
  const canSubmitRegister =
    identifier.trim() !== '' &&
    email.trim() !== '' &&
    emailVerificationCode.trim() !== '' &&
    displayName.trim() !== '' &&
    password !== '';
  const isSubmitDisabled = submitting || (isRegister && !canSubmitRegister);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');

    if (isRegister && !canSubmitRegister) {
      setError('请填写账号、邮箱、验证码、昵称和密码');
      return;
    }

    setSubmitting(true);

    try {
      if (isRegister) {
        await register({
          identifier: identifier.trim(),
          email: email.trim(),
          emailVerificationCode: emailVerificationCode.trim(),
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
    setIdentifierCheckFeedback(null);
    setEmailCodeFeedback(null);
    setSendingEmailCode(false);
  }

  function handleIdentifierChange(event: ChangeEvent<HTMLInputElement>) {
    identifierCheckRequest.current += 1;
    setIdentifier(event.target.value);
    setIdentifierCheckFeedback(null);
  }

  async function checkLoginIdentifierExists() {
    if (isRegister) {
      return;
    }
    const query = identifier.trim();
    identifierCheckRequest.current += 1;
    const requestID = identifierCheckRequest.current;
    setIdentifierCheckFeedback(null);
    if (!query) {
      return;
    }

    try {
      const result = await loginUserApi.identifierExists(query);
      if (identifierCheckRequest.current !== requestID) {
        return;
      }
      setIdentifierCheckFeedback(result.exists ? null : { kind: 'error', message: '账号不存在，请检查后再输入密码' });
    } catch {
      if (identifierCheckRequest.current !== requestID) {
        return;
      }
      setIdentifierCheckFeedback({ kind: 'hint', message: '暂时无法确认账号是否存在，可继续输入密码登录' });
    }
  }

  function handleEmailChange(event: ChangeEvent<HTMLInputElement>) {
    setEmail(event.target.value);
    setEmailVerificationCode('');
    setEmailCodeFeedback(null);
  }

  async function handleSendRegistrationEmailCode() {
    setError('');
    setEmailCodeFeedback(null);
    const trimmedEmail = email.trim();
    if (!trimmedEmail) {
      setEmailCodeFeedback({ kind: 'error', message: '请输入邮箱后再发送验证码' });
      return;
    }

    setSendingEmailCode(true);
    try {
      const result = await requestRegistrationEmailCode(trimmedEmail);
      setEmailCodeFeedback({
        kind: 'success',
        message: `验证码已发送至 ${result.email}，${result.expire_minutes} 分钟内有效`,
      });
    } catch (caughtError) {
      setEmailCodeFeedback({ kind: 'error', message: authErrorMessage(caughtError) });
    } finally {
      setSendingEmailCode(false);
    }
  }

  return (
    <main className="app-shell auth-app-shell" aria-label="Agents IM 认证">
      <section className="phone-frame auth-frame">
        <div className="auth-hero">
          <Avatar label={<ShieldCheck size={30} />} color="green" size="large" />
          <div className="auth-hero-copy">
            <p className="auth-kicker">{adminMode ? 'AgenticIM Management' : 'Agents IM'}</p>
            <h1>{adminMode ? '登录 AgenticIM Management' : isRegister ? '注册 Agents IM' : '登录 Agents IM'}</h1>
          </div>
        </div>

        <form className="auth-form" onSubmit={handleSubmit}>
          {isRegister ? (
            <>
              <TextField
                id="auth-email"
                label="邮箱"
                value={email}
                onChange={handleEmailChange}
                type="email"
                autoComplete="email"
                required
                fieldClassName="auth-field"
              />

              <Button
                className="auth-code-button"
                type="button"
                variant="tonal"
                onClick={handleSendRegistrationEmailCode}
                disabled={sendingEmailCode}
              >
                {sendingEmailCode ? '发送中' : '发送验证码'}
              </Button>

              {emailCodeFeedback ? (
                <p
                  className={emailCodeFeedback.kind === 'error' ? 'auth-error' : 'auth-success'}
                  role={emailCodeFeedback.kind === 'error' ? 'alert' : 'status'}
                >
                  {emailCodeFeedback.message}
                </p>
              ) : null}

              <TextField
                id="auth-email-code"
                label="验证码"
                value={emailVerificationCode}
                onChange={(event) => setEmailVerificationCode(event.target.value)}
                autoComplete="one-time-code"
                inputMode="numeric"
                required
                fieldClassName="auth-field"
              />
            </>
          ) : null}

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

          {!isRegister && identifierCheckFeedback ? (
            <p
              className={identifierCheckFeedback.kind === 'error' ? 'auth-error' : 'auth-hint'}
              role={identifierCheckFeedback.kind === 'error' ? 'alert' : 'status'}
            >
              {identifierCheckFeedback.message}
            </p>
          ) : null}

          {prompt ? (
            <p className="auth-error" role="alert">
              {prompt}
            </p>
          ) : null}

          {error ? (
            <p className="auth-error" role="alert">
              {error}
            </p>
          ) : null}

          <Button className="auth-submit" type="submit" disabled={isSubmitDisabled}>
            {submitting ? '请稍候' : isRegister ? '注册并登录' : '登录'}
          </Button>
        </form>

        {!adminMode ? (
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
        ) : null}
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
  onOpenFeedback: () => void,
  webSocketUrl: string | undefined,
  webSocketToken: string | undefined,
  webSocketFactory: WebSocketFactory | undefined,
  onAuthFailure: (failure: unknown) => void,
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
        webSocketUrl={webSocketUrl}
        webSocketToken={webSocketToken}
        webSocketFactory={webSocketFactory}
        onAuthFailure={onAuthFailure}
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
      <MePage
        profile={currentUser}
        onUpdateProfile={onUpdateProfile}
        onUploadAvatar={onUploadAvatar}
        onOpenFeedback={onOpenFeedback}
      />
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
    name: user.name ?? user.displayName,
    gender: user.gender ?? '',
    birth_date: user.birth_date ?? '',
    region: user.region ?? '',
    account_type: user.accountType,
    avatar_media_id: user.avatarMediaId,
    avatar_url: user.avatarUrl,
  };
}

function authUserFromProfile(profile: UserProfile): AuthUser {
  return {
    userId: profile.user_id,
    identifier: profile.identifier,
    displayName: profile.display_name || profile.name || profile.identifier,
    name: profile.name,
    gender: profile.gender,
    birth_date: profile.birth_date,
    region: profile.region,
    accountType: profile.account_type,
    avatarMediaId: profile.avatar_media_id,
    avatarUrl: profile.avatar_url,
  };
}

export default App;
