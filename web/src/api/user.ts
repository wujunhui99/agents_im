import { createApiClient, type ApiClient } from './client';

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

export type IdentifierExistsResponse = {
  exists: boolean;
  identifier: string;
};

export type UserApi = {
  getCurrentUser: () => Promise<UserProfile>;
  patchCurrentUser: (patch: UserProfilePatch) => Promise<UserProfile>;
  identifierExists: (identifier: string) => Promise<IdentifierExistsResponse>;
  getPublicProfileByIdentifier: (identifier: string) => Promise<UserProfile>;
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

export function createUserApi(api: ApiClient = createApiClient()): UserApi {
  return {
    getCurrentUser() {
      return api.get<UserProfile>('/me');
    },
    patchCurrentUser(input) {
      return api.patch<UserProfile>('/me', toUserProfilePatch(input as Record<string, unknown>));
    },
    identifierExists(identifier) {
      const params = new URLSearchParams({ identifier });
      return api.get<IdentifierExistsResponse>(`/users/exists?${params.toString()}`, { auth: false });
    },
    getPublicProfileByIdentifier(identifier) {
      return api.get<UserProfile>(`/users/${encodeURIComponent(identifier)}`, { auth: false });
    },
  };
}

export const defaultUserApi = createUserApi();
