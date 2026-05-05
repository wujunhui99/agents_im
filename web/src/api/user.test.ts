import { describe, expect, it, vi } from 'vitest';
import { createApiClient } from './client';
import { createUserApi, type UserProfile } from './user';

const profile: UserProfile = {
  user_id: '1001',
  identifier: 'alice_001',
  display_name: 'Alice Chen',
  name: 'Alice Chen',
  gender: 'female',
  birth_date: '1996-05-02',
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
      user_id: '9999',
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

  it('updates the current user avatar through PATCH /me/avatar', async () => {
    const updatedProfile = {
      ...profile,
      avatar_media_id: 'med_avatar_1',
      avatar_url: 'https://storage.test/avatar/alice.png',
      avatar_url_expires_at: 1777550400000,
    };
    const fetcher = vi.fn<typeof fetch>(async () => {
      return new Response(JSON.stringify({ code: 'OK', message: 'ok', data: updatedProfile }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    });
    const client = createApiClient({ baseUrl: 'http://api.test', getToken: () => '***', fetchImpl: fetcher });
    const api = createUserApi(client);

    const result = await api.patchCurrentUserAvatar('med_avatar_1');

    expect(fetcher).toHaveBeenCalledWith(
      'http://api.test/me/avatar',
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
    expect(JSON.parse(requestInit.body as string)).toEqual({ mediaId: 'med_avatar_1' });
    expect(result.avatar_url).toBe('https://storage.test/avatar/alice.png');
  });
});
