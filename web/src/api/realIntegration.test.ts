import { describe, expect, it, vi } from 'vitest';
import { createApiClient } from './client';
import { createContactsApi } from './contacts';
import { createGroupsApi } from './groups';
import { createMessageApi } from './messages';
import { createUserApi } from './user';

function jsonResponse(data: unknown) {
  return new Response(JSON.stringify({ code: 'OK', message: 'ok', data }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('frontend real API integration adapters', () => {
  it('routes every service through the shared client with one bearer token source', async () => {
    const fetcher = vi.fn<typeof fetch>(async (input) => {
      const url = String(input);
      if (url.endsWith('/me')) {
        return jsonResponse({ user_id: '1001', identifier: 'alice_001', display_name: 'Alice' });
      }
      if (url.endsWith('/users/bob_002')) {
        return jsonResponse({ user_id: '2002', identifier: 'bob_002', display_name: 'Bob' });
      }
      if (url.endsWith('/friends')) {
        return jsonResponse({ friends: [] });
      }
      if (url.endsWith('/groups/grp_000001')) {
        return jsonResponse({ group_id: 'grp_000001', name: 'Frontend Demo' });
      }
      if (url.endsWith('/conversations/seqs?conversationIds=single%3A1001%3A2002')) {
        return jsonResponse({ conversations: [] });
      }
      if (url.endsWith('/conversations/single%3A1001%3A2002/ai-hosting')) {
        return jsonResponse({
          conversationId: 'single:1001:2002',
          chatType: 'single',
          enabled: false,
          available: true,
          peerEnabled: false,
          maxRecentMessages: 30,
          summaryEnabled: false,
        });
      }
      return jsonResponse({});
    });
    const token = vi.fn(() => 'shared-session-token');
    const client = createApiClient({ baseUrl: '/api', getToken: token, fetchImpl: fetcher });

    await createUserApi(client).getCurrentUser();
    await createUserApi(client).getPublicProfileByIdentifier('bob_002');
    await createContactsApi(client).listFriends();
    await createGroupsApi(client).getGroup('grp_000001');
    await createMessageApi(client).getConversationSeqs(['single:1001:2002']);
    await createMessageApi(client).getAIHosting('single:1001:2002');
    await createMessageApi(client).updateAIHosting('single:1001:2002', { enabled: true });

    expect(fetcher.mock.calls.map(([input]) => String(input))).toEqual([
      '/api/me',
      '/api/users/bob_002',
      '/api/friends',
      '/api/groups/grp_000001',
      '/api/conversations/seqs?conversationIds=single%3A1001%3A2002',
      '/api/conversations/single%3A1001%3A2002/ai-hosting',
      '/api/conversations/single%3A1001%3A2002/ai-hosting',
    ]);
    expect(fetcher.mock.calls[0]?.[1]?.headers).toEqual(
      expect.objectContaining({ Authorization: 'Bearer shared-session-token' }),
    );
    expect(fetcher.mock.calls[1]?.[1]?.headers).not.toEqual(
      expect.objectContaining({ Authorization: 'Bearer shared-session-token' }),
    );
    expect(fetcher.mock.calls[2]?.[1]?.headers).toEqual(
      expect.objectContaining({ Authorization: 'Bearer shared-session-token' }),
    );
    expect(fetcher.mock.calls[3]?.[1]?.headers).toEqual(
      expect.objectContaining({ Authorization: 'Bearer shared-session-token' }),
    );
    expect(fetcher.mock.calls[4]?.[1]?.headers).toEqual(
      expect.objectContaining({ Authorization: 'Bearer shared-session-token' }),
    );
    expect(fetcher.mock.calls[5]?.[1]?.headers).toEqual(
      expect.objectContaining({ Authorization: 'Bearer shared-session-token' }),
    );
    expect(fetcher.mock.calls[6]?.[1]?.method).toBe('PUT');
    expect(fetcher.mock.calls[6]?.[1]?.headers).toEqual(
      expect.objectContaining({ Authorization: 'Bearer shared-session-token' }),
    );
    expect(token).toHaveBeenCalledTimes(6);
  });
});
