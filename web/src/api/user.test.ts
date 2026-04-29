import { describe, expect, it, vi } from 'vitest';
import { createApiClient } from './client';
import { createUserApi, type UserProfile } from './user';

const profile: UserProfile = {
  user_id: 'usr_000001',
  identifier: 'alice_001',
  display_name: 'Alice Chen',
  name: 'Alice Chen',
  gender: 'female',
  age: 30,
  region: 'Hangzhou',
  created_at: '2026-04-29T12:00:00Z',
  updated_at: '2026-04-29T12:30:00Z',
};

describe('user API adapter', () => {
  it('patches only mutable profile fields and never sends user_id or identifier', async () => {
    const fetcher = vi.fn<typeof fetch>(async () => {
      return new Response(JSON.stringify({ code: 'OK', message: 'ok', data: profile }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    });
    const client = createApiClient({ baseUrl: 'http://api.test', getToken: () => '***', fetchImpl: fetcher });
    const api = createUserApi(client);

    await api.patchCurrentUser({
      user_id: 'usr_changed',
      identifier: 'changed_identifier',
      display_name: 'Alice Chen',
      region: 'Hangzhou',
    } as never);

    expect(fetcher).toHaveBeenCalledWith(
      'http://api.test/me',
      expect.objectContaining({
        method: 'PATCH',
        headers: expect.objectContaining({
          Authorization: 'Bearer ***',
          'Content-Type': 'application/json',
        }),
      }),
    );
    const requestInit = fetcher.mock.calls[0]?.[1];
    expect(requestInit).toBeDefined();
    if (!requestInit) {
      throw new Error('missing request init');
    }
    expect(JSON.parse(requestInit.body as string)).toEqual({
      display_name: 'Alice Chen',
      region: 'Hangzhou',
    });
  });
});
