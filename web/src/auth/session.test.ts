import { describe, expect, it } from 'vitest';
import { AUTH_STORAGE_KEY, readStoredSession, type AuthSession } from './session';

class MemoryStorage implements Storage {
  private values = new Map<string, string>();

  get length() {
    return this.values.size;
  }

  clear() {
    this.values.clear();
  }

  getItem(key: string) {
    return this.values.get(key) ?? null;
  }

  key(index: number) {
    return Array.from(this.values.keys())[index] ?? null;
  }

  removeItem(key: string) {
    this.values.delete(key);
  }

  setItem(key: string, value: string) {
    this.values.set(key, value);
  }
}

describe('auth session storage', () => {
  it('preserves durable avatar fields when restoring a persisted session', () => {
    const storage = new MemoryStorage();
    storage.setItem(
      AUTH_STORAGE_KEY,
      JSON.stringify({
        token: 'test-token',
        expiresAt: '2026-05-07T00:00:00Z',
        user: {
          userId: '1001',
          identifier: 'alice_001',
          displayName: 'Alice',
          avatarMediaId: 'med_avatar_1',
          avatarUrl: '/media/avatars/med_avatar_1',
        },
      } satisfies AuthSession),
    );

    const session = readStoredSession(storage);

    expect(session?.user.avatarMediaId).toBe('med_avatar_1');
    expect(session?.user.avatarUrl).toBe('/media/avatars/med_avatar_1');
  });
});
