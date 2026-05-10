import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { createAuthApi, type AuthResponse, type RegistrationEmailCodeData } from '../api/auth';
import { ApiError, createApiClient } from '../api/client';
import { clearStoredSession, readStoredSession, writeStoredSession, type AuthSession, type AuthUser } from './session';

type LoginInput = {
  identifier: string;
  password: string;
};

type RegisterInput = LoginInput & {
  email: string;
  emailVerificationCode: string;
  displayName: string;
};

type AuthContextValue = {
  session: AuthSession | null;
  authPrompt: string;
  login(input: LoginInput): Promise<AuthSession>;
  register(input: RegisterInput): Promise<AuthSession>;
  requestRegistrationEmailCode(email: string): Promise<RegistrationEmailCodeData>;
  updateSessionUser(input: Partial<AuthUser>): void;
  logout(): void;
  handleAuthFailure(input?: AuthFailureInput): void;
};

const AuthContext = createContext<AuthContextValue | null>(null);
export const KICKED_SESSION_MESSAGE = '账号已在其他设备登录，请重新登录';

type AuthFailureInput = {
  token?: string | null;
  message?: string;
};

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<AuthSession | null>(() => readStoredSession());
  const [authPrompt, setAuthPrompt] = useState('');
  const sessionRef = useRef<AuthSession | null>(session);
  const api = useMemo(
    () =>
      createAuthApi(
        createApiClient({
          getToken: () => session?.token,
        }),
      ),
    [session?.token],
  );

  useEffect(() => {
    sessionRef.current = session;
  }, [session]);

  async function login(input: LoginInput) {
    const response = await api.login(input);
    return persistSession(toSession(response));
  }

  async function register(input: RegisterInput) {
    const response = await api.register(input);
    return persistSession(toSession(response, input.displayName));
  }

  async function requestRegistrationEmailCode(email: string) {
    return api.requestRegistrationEmailCode(email);
  }

  function logout() {
    clearStoredSession();
    sessionRef.current = null;
    setAuthPrompt('');
    setSession(null);
  }

  const handleAuthFailure = useCallback((input: AuthFailureInput = {}) => {
    const currentSession = sessionRef.current;
    if (input.token && (!currentSession || input.token !== currentSession.token)) {
      return;
    }

    clearStoredSession();
    sessionRef.current = null;
    setSession(null);
    setAuthPrompt(input.message?.trim() || KICKED_SESSION_MESSAGE);
  }, []);

  function updateSessionUser(input: Partial<AuthUser>) {
    setSession((current) => {
      if (!current) {
        return current;
      }
      const nextSession = {
        ...current,
        user: {
          ...current.user,
          ...input,
          displayName: input.displayName || current.user.displayName,
        },
      };
      writeStoredSession(nextSession);
      sessionRef.current = nextSession;
      return nextSession;
    });
  }

  function persistSession(nextSession: AuthSession) {
    writeStoredSession(nextSession);
    sessionRef.current = nextSession;
    setAuthPrompt('');
    setSession(nextSession);
    return nextSession;
  }

  const value = useMemo(
    () => ({
      session,
      authPrompt,
      login,
      register,
      requestRegistrationEmailCode,
      updateSessionUser,
      logout,
      handleAuthFailure,
    }),
    [authPrompt, handleAuthFailure, session],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
}

export function authErrorMessage(error: unknown) {
  if (error instanceof ApiError) {
    if (isRawTokenAuthMessage(error.message)) {
      return KICKED_SESSION_MESSAGE;
    }
    return error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return '请求失败，请稍后重试';
}

function isRawTokenAuthMessage(message: string) {
  const normalized = message.toLowerCase();
  return (
    normalized.includes('invalid or missing bearer token') ||
    normalized.includes('token session is not active') ||
    normalized.includes('session inactive') ||
    normalized.includes('session invalid') ||
    normalized.includes('session replaced')
  );
}

function toSession(response: AuthResponse, displayNameFallback?: string): AuthSession {
  return {
    token: response.token,
    expiresAt: response.expires_at,
    user: {
      userId: response.user_id,
      identifier: response.identifier,
      displayName: response.display_name || response.name || displayNameFallback || response.identifier,
      name: response.name,
      gender: response.gender,
      birth_date: response.birth_date,
      region: response.region,
      accountType: response.account_type,
      avatarMediaId: response.avatar_media_id,
      avatarUrl: response.avatar_url,
    },
  };
}
