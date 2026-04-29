import { createContext, useContext, useMemo, useState, type ReactNode } from 'react';
import { ApiError, createApiClient } from '../api/client';
import { clearStoredSession, readStoredSession, writeStoredSession, type AuthSession } from './session';

type AuthResponse = {
  user_id: string;
  identifier: string;
  display_name?: string;
  name?: string;
  gender?: string;
  age?: number;
  region?: string;
  token: string;
  expires_at?: string;
};

type LoginInput = {
  identifier: string;
  password: string;
};

type RegisterInput = LoginInput & {
  displayName: string;
};

type AuthContextValue = {
  session: AuthSession | null;
  login(input: LoginInput): Promise<AuthSession>;
  register(input: RegisterInput): Promise<AuthSession>;
  logout(): void;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<AuthSession | null>(() => readStoredSession());
  const api = useMemo(
    () =>
      createApiClient({
        getToken: () => session?.token,
      }),
    [session?.token],
  );

  async function login(input: LoginInput) {
    const response = await api.post<AuthResponse>(
      '/auth/login',
      { identifier: input.identifier, password: input.password },
      { auth: false },
    );
    return persistSession(toSession(response));
  }

  async function register(input: RegisterInput) {
    const response = await api.post<AuthResponse>(
      '/auth/register',
      {
        identifier: input.identifier,
        password: input.password,
        display_name: input.displayName,
      },
      { auth: false },
    );
    return persistSession(toSession(response, input.displayName));
  }

  function logout() {
    clearStoredSession();
    setSession(null);
  }

  function persistSession(nextSession: AuthSession) {
    writeStoredSession(nextSession);
    setSession(nextSession);
    return nextSession;
  }

  const value = useMemo(
    () => ({
      session,
      login,
      register,
      logout,
    }),
    [session],
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
    return error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return '请求失败，请稍后重试';
}

function toSession(response: AuthResponse, displayNameFallback?: string): AuthSession {
  return {
    token: response.token,
    expiresAt: response.expires_at,
    user: {
      userId: response.user_id,
      identifier: response.identifier,
      displayName: response.display_name || response.name || displayNameFallback || response.identifier,
      gender: response.gender,
      age: response.age,
      region: response.region,
    },
  };
}
