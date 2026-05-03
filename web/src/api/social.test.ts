import { describe, expect, it } from 'vitest';
import { createApiClient } from './client';
import { createContactsApi } from './contacts';
import { createGroupsApi } from './groups';

type RecordedFetchCall = {
  input: RequestInfo | URL;
  init?: RequestInit;
};

function createFetchRecorder(data: unknown = {}) {
  const calls: RecordedFetchCall[] = [];
  const fetcher = (async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ input, init });
    return new Response(JSON.stringify({ code: 'OK', message: 'ok', data }), {
      headers: { 'Content-Type': 'application/json' },
    });
  }) as typeof fetch;

  return { calls, fetcher };
}

function headersFor(call: RecordedFetchCall) {
  return new Headers(call.init?.headers);
}

describe('contacts API adapter', () => {
  it('uses the friends contract paths and bearer token', async () => {
    const { calls, fetcher } = createFetchRecorder({ friends: [] });
    const client = createApiClient({ baseUrl: 'http://api.test', getToken: () => '***', fetchImpl: fetcher });
    const api = createContactsApi(client);

    await api.listFriends();
    await api.listFriendRequests();
    await api.addFriend('2002');
    await api.acceptFriend('2002');
    await api.rejectFriend('2002');
    await api.deleteFriend('2002');

    expect(calls.map((call) => [String(call.input), call.init?.method ?? 'GET'])).toEqual([
      ['http://api.test/friends', 'GET'],
      ['http://api.test/friends/requests', 'GET'],
      ['http://api.test/friends', 'POST'],
      ['http://api.test/friends/2002/accept', 'POST'],
      ['http://api.test/friends/2002/reject', 'POST'],
      ['http://api.test/friends/2002', 'DELETE'],
    ]);
    expect(headersFor(calls[0]).get('Authorization')).toBe('Bearer ***');
    expect(JSON.parse(String(calls[2].init?.body))).toEqual({ user_id: '2002' });
  });
});

describe('groups API adapter', () => {
  it('uses the groups contract paths and typed member operations', async () => {
    const { calls, fetcher } = createFetchRecorder({ members: [] });
    const client = createApiClient({ baseUrl: 'http://api.test/', getToken: () => '***', fetchImpl: fetcher });
    const api = createGroupsApi(client);

    await api.getGroup('grp_000001');
    await api.createGroup({ name: 'Frontend Demo', description: 'MVP smoke room' });
    await api.joinGroup('grp_000001', '2002');
    await api.leaveGroup('grp_000001');
    await api.listMembers('grp_000001');

    expect(calls.map((call) => [String(call.input), call.init?.method ?? 'GET'])).toEqual([
      ['http://api.test/groups/grp_000001', 'GET'],
      ['http://api.test/groups', 'POST'],
      ['http://api.test/groups/grp_000001/members', 'POST'],
      ['http://api.test/groups/grp_000001/members/me', 'DELETE'],
      ['http://api.test/groups/grp_000001/members', 'GET'],
    ]);
    expect(headersFor(calls[2]).get('Authorization')).toBe('Bearer ***');
    expect(JSON.parse(String(calls[2].init?.body))).toEqual({ user_id: '2002' });
  });
});
