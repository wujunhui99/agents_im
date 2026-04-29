export type UserProfile = {
  user_id: string;
  identifier: string;
  display_name: string;
  name?: string;
  gender: string;
  age: number;
  region: string;
  created_at?: string;
  updated_at?: string;
};

export type UserProfilePatch = Partial<Pick<UserProfile, 'display_name' | 'name' | 'gender' | 'age' | 'region'>>;

export type UserApi = {
  patchCurrentUser: (patch: UserProfilePatch) => Promise<UserProfile>;
};

type UserApiOptions = {
  baseUrl?: string;
  token?: string | (() => string | null | undefined) | null;
  fetcher?: typeof fetch;
};

type ApiEnvelope<T> = {
  code: string;
  message: string;
  data: T | null;
};

const mutableProfileKeys = ['display_name', 'name', 'gender', 'age', 'region'] as const;

export function toUserProfilePatch(input: Record<string, unknown>): UserProfilePatch {
  const patch = mutableProfileKeys.reduce<Record<string, unknown>>((nextPatch, key) => {
    const value = input[key];
    if (value !== undefined) {
      nextPatch[key] = value;
    }
    return nextPatch;
  }, {});

  return patch as UserProfilePatch;
}

export function createUserApi({ baseUrl = '', token = readStoredAccessToken, fetcher = globalThis.fetch.bind(globalThis) }: UserApiOptions = {}): UserApi {
  return {
    async patchCurrentUser(input) {
      const payload = toUserProfilePatch(input as Record<string, unknown>);
      const response = await fetcher(`${baseUrl}/me`, {
        method: 'PATCH',
        headers: createJsonHeaders(resolveToken(token)),
        body: JSON.stringify(payload),
      });

      let envelope: ApiEnvelope<UserProfile>;
      try {
        envelope = (await response.json()) as ApiEnvelope<UserProfile>;
      } catch {
        throw new Error('Invalid /me response');
      }

      if (!response.ok || envelope.code !== 'OK' || !envelope.data) {
        throw new Error(envelope.message || 'Failed to update profile');
      }

      return envelope.data;
    },
  };
}

export const defaultUserApi = createUserApi();

function createJsonHeaders(token?: string) {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };

  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  return headers;
}

function resolveToken(token: UserApiOptions['token']) {
  if (typeof token === 'function') {
    return token() ?? undefined;
  }

  return token ?? undefined;
}

function readStoredAccessToken() {
  if (typeof window === 'undefined') {
    return undefined;
  }

  return window.localStorage.getItem('agents_im_access_token') ?? undefined;
}
