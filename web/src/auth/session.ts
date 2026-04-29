export const AUTH_STORAGE_KEY = 'agents_im.auth.v1';

export type AuthUser = {
  userId: string;
  identifier: string;
  displayName: string;
  gender?: string;
  age?: number;
  region?: string;
};

export type AuthSession = {
  token: string;
  user: AuthUser;
  expiresAt?: string;
};

export function readStoredSession(storage: Storage = localStorage): AuthSession | null {
  const raw = storage.getItem(AUTH_STORAGE_KEY);
  if (!raw) {
    return null;
  }

  try {
    const parsed = JSON.parse(raw) as Partial<AuthSession>;
    if (!parsed.token || !parsed.user?.userId || !parsed.user.identifier) {
      storage.removeItem(AUTH_STORAGE_KEY);
      return null;
    }

    return {
      token: parsed.token,
      expiresAt: parsed.expiresAt,
      user: {
        userId: parsed.user.userId,
        identifier: parsed.user.identifier,
        displayName: parsed.user.displayName || parsed.user.identifier,
        gender: parsed.user.gender,
        age: parsed.user.age,
        region: parsed.user.region,
      },
    };
  } catch {
    storage.removeItem(AUTH_STORAGE_KEY);
    return null;
  }
}

export function writeStoredSession(session: AuthSession, storage: Storage = localStorage) {
  storage.setItem(AUTH_STORAGE_KEY, JSON.stringify(session));
}

export function clearStoredSession(storage: Storage = localStorage) {
  storage.removeItem(AUTH_STORAGE_KEY);
}
