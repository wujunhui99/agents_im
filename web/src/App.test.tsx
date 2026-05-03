import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { readFileSync } from 'node:fs';
import { vi } from 'vitest';
import App from './App';
import type { UserProfile, UserProfilePatch } from './api/user';
import { AUTH_STORAGE_KEY, type AuthSession } from './auth/session';

const stylesCss = readFileSync('src/styles.css', 'utf8');

const initialProfile: UserProfile = {
  user_id: '1001',
  identifier: 'alice_001',
  display_name: 'Alice',
  name: 'Alice',
  gender: 'female',
  birth_date: '1996-05-02',
  region: 'Shanghai',
  account_type: 'user',
  created_at: '2026-04-29T12:00:00Z',
  updated_at: '2026-04-29T12:00:00Z',
};

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
}

function storeSession(overrides?: Partial<AuthSession>) {
  const session: AuthSession = {
    token: 'test-token',
    user: {
      userId: '1001',
      identifier: 'alice_001',
      displayName: 'Alice Chen',
      gender: 'female',
      birth_date: '1996-05-02',
      region: 'Shanghai',
    },
    ...overrides,
  };

  localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(session));
  return session;
}

function emptySeqsResponse() {
  return jsonResponse({ code: 'OK', message: 'ok', data: { states: [] } });
}

function emptyFriendRequestsResponse() {
  return jsonResponse({ code: 'OK', message: 'ok', data: { incoming: [], outgoing: [] } });
}

function fetchUrl(input: RequestInfo | URL) {
  if (typeof input === 'string') {
    return input;
  }
  if (input instanceof URL) {
    return `${input.pathname}${input.search}`;
  }
  return input.url;
}

function fetchMethod(init?: RequestInit) {
  return (init?.method ?? 'GET').toUpperCase();
}

function bobProfile(overrides: Partial<UserProfile> = {}) {
  return {
    user_id: '2002',
    identifier: 'bob_002',
    display_name: 'Bob',
    name: 'Bob',
    gender: '',
    birth_date: '',
    region: '',
    ...overrides,
  };
}

function friendshipWith(profile: UserProfile, status = 'accepted') {
  return {
    user_id: '1001',
    friend_id: profile.user_id,
    status,
    is_friend: status === 'accepted',
    friend: profile,
    created_at: '2026-04-29T12:00:00Z',
    updated_at: '2026-04-29T12:00:00Z',
  };
}

function friendsResponse(friends: unknown[]) {
  return jsonResponse({ code: 'OK', message: 'ok', data: { friends } });
}

function conversationSeqResponse(content: string) {
  return jsonResponse({
    code: 'OK',
    message: 'ok',
    data: {
      states: [
        {
          conversationId: 'single:1001:2002',
          maxSeq: 1,
          hasReadSeq: 0,
          unreadCount: 1,
          maxSeqTime: 1777464300000,
          lastMessage: {
            serverMsgId: 'srv-existing-1',
            clientMsgId: 'client-existing-1',
            conversationId: 'single:1001:2002',
            seq: 1,
            senderId: '2002',
            receiverId: '1001',
            groupId: '',
            chatType: 'single',
            contentType: 'text',
            content,
            sendTime: 1777464300000,
            createdAt: 1777464300000,
          },
        },
      ],
    },
  });
}

function conversationMessagesResponse(content: string) {
  return jsonResponse({
    code: 'OK',
    message: 'ok',
    data: {
      conversationId: 'single:1001:2002',
      messages: [
        {
          serverMsgId: 'srv-existing-1',
          clientMsgId: 'client-existing-1',
          conversationId: 'single:1001:2002',
          seq: 1,
          senderId: '2002',
          receiverId: '1001',
          groupId: '',
          chatType: 'single',
          contentType: 'text',
          content,
          sendTime: 1777464300000,
          createdAt: 1777464300000,
        },
      ],
      isEnd: true,
      nextSeq: 2,
    },
  });
}

describe('Auth flow', () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    localStorage.clear();
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  it('shows a WeChat-style login page before authentication and saves the login token', async () => {
    const user = userEvent.setup();
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        code: 'OK',
        message: 'ok',
        data: {
          user_id: '1001',
          identifier: 'alice_001',
          display_name: 'Alice Chen',
          token: 'login-token',
          expires_at: '2026-04-30T12:00:00Z',
        },
      }),
    );

    render(<App />);

    expect(screen.getByRole('heading', { name: '登录 Agents IM' })).toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: /消息/i })).not.toBeInTheDocument();

    await user.type(screen.getByLabelText('账号'), 'alice_001');
    await user.type(screen.getByLabelText('密码'), 'test-password');
    await user.click(screen.getByRole('button', { name: '登录' }));

    expect(await screen.findByRole('heading', { name: '消息' })).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      '/auth/login',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ identifier: 'alice_001', password: 'test-password' }),
      }),
    );
    expect(JSON.parse(localStorage.getItem(AUTH_STORAGE_KEY) ?? '{}')).toMatchObject({
      token: 'login-token',
      user: { identifier: 'alice_001', displayName: 'Alice Chen' },
    });
  });

  it('registers a new account and enters the four-tab shell', async () => {
    const user = userEvent.setup();
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        code: 'OK',
        message: 'ok',
        data: {
          user_id: '2002',
          identifier: 'new_user',
          token: 'register-token',
          expires_at: '2026-04-30T12:00:00Z',
        },
      }),
    );

    render(<App />);

    await user.click(screen.getByRole('button', { name: '注册账号' }));
    expect(screen.getByRole('heading', { name: '注册 Agents IM' })).toBeInTheDocument();

    await user.type(screen.getByLabelText('账号'), 'new_user');
    await user.type(screen.getByLabelText('昵称'), 'New User');
    await user.type(screen.getByLabelText('密码'), 'test-password');
    await user.click(screen.getByRole('button', { name: '注册并登录' }));

    expect(await screen.findByRole('heading', { name: '消息' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /我的/i })).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      '/auth/register',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          identifier: 'new_user',
          password: 'test-password',
          display_name: 'New User',
        }),
      }),
    );
    expect(JSON.parse(localStorage.getItem(AUTH_STORAGE_KEY) ?? '{}')).toMatchObject({
      token: 'register-token',
      user: { identifier: 'new_user', displayName: 'New User' },
    });
  });

  it('logs out and returns to the login page', async () => {
    const user = userEvent.setup();
    storeSession();
    fetchMock.mockResolvedValue(emptySeqsResponse());

    render(<App />);

    await user.click(screen.getByRole('tab', { name: /我的/i }));
    expect(screen.getAllByText('Alice Chen').length).toBeGreaterThan(0);

    await user.click(screen.getByRole('button', { name: '退出登录' }));

    expect(screen.getByRole('heading', { name: '登录 Agents IM' })).toBeInTheDocument();
    expect(localStorage.getItem(AUTH_STORAGE_KEY)).toBeNull();
  });
});

describe('WeChat-inspired app shell', () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    localStorage.clear();
    storeSession();
    fetchMock.mockReset();
    fetchMock.mockResolvedValue(emptySeqsResponse());
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  it('renders the four primary tabs', async () => {
    render(<App />);

    expect(screen.getByRole('tab', { name: /消息/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /联系人/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /发现/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /我的/i })).toBeInTheDocument();
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith('/conversations/seqs?conversationIds=', expect.anything()));
  });

  it('defines a desktop shell breakpoint that expands beyond the mobile phone frame', () => {
    expect(stylesCss).toContain('@media (min-width: 900px)');
    expect(stylesCss).toMatch(/@media \(min-width: 900px\)[\s\S]*\.app-shell\s*{[\s\S]*place-items:\s*stretch/);
    expect(stylesCss).toMatch(/@media \(min-width: 900px\)[\s\S]*\.phone-frame\s*{[\s\S]*width:\s*100%/);
    expect(stylesCss).toMatch(/@media \(min-width: 900px\)[\s\S]*\.content-area\s*{[\s\S]*max-width:\s*none/);
  });

  it('defaults to the real messages page and switches pages from the bottom navigation', async () => {
    const user = userEvent.setup();
    render(<App />);

    expect(screen.getByRole('heading', { name: '消息' })).toBeInTheDocument();
    expect(screen.getByText('暂无会话')).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    expect(screen.getByRole('heading', { name: '联系人' })).toBeInTheDocument();
    expect(screen.getByText('新的朋友')).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: /发现/i }));
    expect(screen.getByRole('heading', { name: '发现' })).toBeInTheDocument();
    expect(screen.getByText('朋友圈')).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: /我的/i }));
    expect(screen.getByRole('heading', { name: '我的' })).toBeInTheDocument();
    expect(screen.getAllByText(/alice_001/).length).toBeGreaterThan(0);
  });

  it('wires the message top-bar add button to the start-chat panel', async () => {
    const user = userEvent.setup();
    render(<App />);

    expect(await screen.findByText('暂无会话')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '新增' }));

    expect(screen.getByRole('region', { name: '发起聊天' })).toBeInTheDocument();
    expect(screen.getByLabelText('按账号搜索聊天对象')).toBeInTheDocument();
  });

  it('shows MVP placeholder entrances on the discover page without real scan behavior', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('tab', { name: /发现/i }));

    expect(screen.getByText('朋友圈')).toBeInTheDocument();
    expect(screen.getByText('扫一扫')).toBeInTheDocument();
    expect(screen.getByText('小程序')).toBeInTheDocument();
    expect(screen.getAllByText('MVP 占位')).toHaveLength(4);
    expect(screen.getByText('暂不启动真实扫码')).toBeInTheDocument();
  });

  it('edits mutable profile fields from the me page through the user API adapter', async () => {
    const user = userEvent.setup();
    const patchCurrentUser = vi.fn(async (payload: UserProfilePatch) => ({
      ...initialProfile,
      ...payload,
      updated_at: '2026-04-29T12:30:00Z',
    }));
    const userApi = {
      getCurrentUser: vi.fn(async () => initialProfile),
      identifierExists: vi.fn(async () => ({ identifier: initialProfile.identifier, exists: true })),
      getPublicProfileByIdentifier: vi.fn(async () => initialProfile),
      patchCurrentUser,
    };

    render(<App initialUser={initialProfile} userApi={userApi} />);

    await user.click(screen.getByRole('tab', { name: /我的/i }));

    expect(screen.queryByText('user_id')).not.toBeInTheDocument();
    expect(screen.queryByText('1001')).not.toBeInTheDocument();
    expect(screen.getByText('alice_001')).toBeInTheDocument();
    expect(screen.getByText('用户')).toBeInTheDocument();
    expect(screen.getByText('女')).toBeInTheDocument();
    expect(screen.getByText('1996-05-02')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '编辑个人资料' }));
    await user.clear(screen.getByLabelText('昵称'));
    await user.type(screen.getByLabelText('昵称'), 'Alice Chen');
    await user.clear(screen.getByLabelText('地区'));
    await user.type(screen.getByLabelText('地区'), 'Hangzhou');
    await user.clear(screen.getByLabelText('生日'));
    await user.type(screen.getByLabelText('生日'), '1995-05-02');
    await user.click(screen.getByRole('button', { name: '保存' }));

    expect(patchCurrentUser).toHaveBeenCalledWith({
      display_name: 'Alice Chen',
      gender: 'female',
      birth_date: '1995-05-02',
      region: 'Hangzhou',
    });
    expect((await screen.findAllByText('Alice Chen')).length).toBeGreaterThan(0);
    expect(screen.getAllByText('Hangzhou').length).toBeGreaterThan(0);
  });

  it('searches users by unique identifier and sends add-friend through real APIs', async () => {
    const user = userEvent.setup();
    const bob = bobProfile({ display_name: 'Bob Lin', name: 'Bob Lin' });
    fetchMock.mockReset();
    fetchMock.mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = fetchUrl(input);
      const method = fetchMethod(init);

      if (url === '/conversations/seqs?conversationIds=' && method === 'GET') {
        return emptySeqsResponse();
      }
      if (url === '/friends' && method === 'GET') {
        return friendsResponse([]);
      }
      if (url === '/friends/requests' && method === 'GET') {
        return emptyFriendRequestsResponse();
      }
      if (url === '/users/bob_002' && method === 'GET') {
        return jsonResponse({ code: 'OK', message: 'ok', data: bob });
      }
      if (url === '/friends' && method === 'POST') {
        return jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            friendship: friendshipWith(bob, 'pending'),
            created: true,
          },
        });
      }

      throw new Error(`Unhandled test request: ${method} ${url}`);
    });

    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    await user.type(screen.getByLabelText('按账号搜索用户'), 'bob_002');
    await user.click(screen.getByRole('button', { name: '搜索用户' }));

    const searchRegion = screen.getByRole('region', { name: '账号搜索' });
    await waitFor(() => expect(screen.getAllByRole('status').map((node) => node.textContent).join(' ')).toContain('找到 Bob Lin'));
    expect(within(searchRegion).getByText('bob_002')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '添加好友 bob_002' }));

    await waitFor(() => expect(screen.getAllByRole('status').map((node) => node.textContent).join(' ')).toContain('已发送好友申请'));
    expect(screen.getByRole('button', { name: '等待对方确认' })).toBeDisabled();
    expect(fetchMock).toHaveBeenCalledWith('/users/bob_002', expect.objectContaining({ method: 'GET' }));
    expect(fetchMock).toHaveBeenCalledWith(
      '/friends',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ user_id: '2002' }),
      }),
    );
    expect(screen.queryByText('2002')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '和 Bob Lin 聊天' })).not.toBeInTheDocument();
    expect((await screen.findAllByText('Bob Lin')).length).toBeGreaterThan(0);
  });




  it('renders direct conversation peer names from embedded friend profiles instead of unknown contacts', async () => {
    const user = userEvent.setup();
    fetchMock.mockReset();
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            states: [
              {
                conversationId: 'single:1001:2002',
                maxSeq: 1,
                hasReadSeq: 0,
                unreadCount: 1,
                maxSeqTime: 1777464300000,
                lastMessage: {
                  serverMsgId: 'srv-existing-1',
                  conversationId: 'single:1001:2002',
                  seq: 1,
                  senderId: '2002',
                  receiverId: '1001',
                  chatType: 'single',
                  contentType: 'text',
                  content: 'hello from Bob',
                  sendTime: 1777464300000,
                  createdAt: 1777464300000,
                },
              },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: { messages: [] },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            friends: [
              {
                user_id: '1001',
                friend_id: '2002',
                status: 'accepted',
                is_friend: true,
                friend: {
                  user_id: '2002',
                  identifier: 'bob_002',
                  display_name: 'Bob Lin',
                  name: 'Bob Lin',
                  gender: 'male',
                  birth_date: '',
                  region: 'Hangzhou',
                },
                created_at: '2026-04-29T12:00:00Z',
                updated_at: '2026-04-29T12:00:00Z',
              },
            ],
          },
        }),
      );

    render(<App />);

    expect(await screen.findByText('Bob Lin')).toBeInTheDocument();
    expect(screen.queryByText('未知联系人')).not.toBeInTheDocument();
    await user.click(screen.getByText('Bob Lin'));
    expect(await screen.findByRole('heading', { name: 'Bob Lin', level: 2 })).toBeInTheDocument();
  });

  it('keeps new friendships pending until the other user accepts', async () => {
    const user = userEvent.setup();
    fetchMock.mockReset();
    fetchMock
      .mockResolvedValueOnce(emptySeqsResponse())
      .mockResolvedValueOnce(jsonResponse({ code: 'OK', message: 'ok', data: { friends: [] } }))
      .mockResolvedValueOnce(emptyFriendRequestsResponse())
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            user_id: '2002',
            identifier: 'bob_002',
            display_name: 'Bob Lin',
            name: 'Bob Lin',
            gender: 'male',
            birth_date: '',
            region: '',
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            friendship: {
              user_id: '1001',
              friend_id: '2002',
              status: 'pending',
              is_friend: false,
              created_at: '2026-04-29T12:00:00Z',
              updated_at: '2026-04-29T12:00:00Z',
            },
            created: true,
          },
        }),
      );

    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    await user.type(screen.getByLabelText('按账号搜索用户'), 'bob_002');
    await user.click(screen.getByRole('button', { name: '搜索用户' }));
    await screen.findByText('Bob Lin');

    await user.click(screen.getByRole('button', { name: '添加好友 bob_002' }));

    await waitFor(() => expect(screen.getAllByRole('status').map((node) => node.textContent).join(' ')).toContain('已发送好友申请'));
    expect(screen.getByRole('button', { name: '等待对方确认' })).toBeDisabled();
    expect(screen.queryByRole('button', { name: '和 Bob Lin 聊天' })).not.toBeInTheDocument();
  });

  it('shows profile labels and gender values in Chinese on the me page', async () => {
    const user = userEvent.setup();
    render(<App initialUser={initialProfile} />);

    await user.click(screen.getByRole('tab', { name: /我的/i }));

    expect(screen.getByText('账号')).toBeInTheDocument();
    expect(screen.getByText('昵称')).toBeInTheDocument();
    expect(screen.getByText('账号类型')).toBeInTheDocument();
    expect(screen.getByText('性别')).toBeInTheDocument();
    expect(screen.getByText('女')).toBeInTheDocument();
    expect(screen.getByText('地区')).toBeInTheDocument();
    expect(screen.queryByText('identifier')).not.toBeInTheDocument();
    expect(screen.queryByText('display_name')).not.toBeInTheDocument();
    expect(screen.queryByText('gender')).not.toBeInTheDocument();
    expect(screen.queryByText('region')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '编辑个人资料' }));
    expect(screen.getByLabelText('昵称')).toBeInTheDocument();
    expect(screen.getByLabelText('性别')).toBeInTheDocument();
    expect(screen.getByLabelText('地区')).toBeInTheDocument();
  });

  it('loads friends automatically when entering contacts and opens a friend chat from the contact row', async () => {
    const user = userEvent.setup();
    const bob = bobProfile();
    const bareFriendship = {
      user_id: '1001',
      friend_id: '2002',
      status: 'accepted',
      is_friend: true,
      created_at: '2026-04-29T12:00:00Z',
      updated_at: '2026-04-29T12:00:00Z',
    };
    let friendLoads = 0;
    fetchMock.mockReset();
    fetchMock.mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = fetchUrl(input);
      const method = fetchMethod(init);

      if (url === '/conversations/seqs?conversationIds=' && method === 'GET') {
        return emptySeqsResponse();
      }
      if (url === '/friends/requests' && method === 'GET') {
        return emptyFriendRequestsResponse();
      }
      if (url === '/friends' && method === 'GET') {
        friendLoads += 1;
        return friendsResponse([friendLoads === 1 ? bareFriendship : friendshipWith(bob)]);
      }

      throw new Error(`Unhandled test request: ${method} ${url}`);
    });

    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));

    expect(await screen.findByRole('button', { name: '和 未知联系人 聊天' })).toBeInTheDocument();
    expect(screen.queryByText('2002')).not.toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith('/friends', expect.objectContaining({ method: 'GET' }));

    await user.click(screen.getByRole('tab', { name: /发现/i }));
    await user.click(screen.getByRole('tab', { name: /联系人/i }));

    await user.click(screen.getByRole('button', { name: '和 bob_002 聊天' }));

    expect(await screen.findByRole('heading', { name: 'Bob', level: 2 })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: '输入消息' })).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalledWith('/users/2002', expect.objectContaining({ method: 'GET' }));
  });

  it('loads friends automatically again after session restore', async () => {
    const user = userEvent.setup();
    fetchMock.mockReset();
    fetchMock
      .mockResolvedValueOnce(emptySeqsResponse())
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            friends: [
              {
                user_id: '1001',
                friend_id: '2002',
                status: 'accepted',
                is_friend: true,
                created_at: '2026-04-29T12:00:00Z',
                updated_at: '2026-04-29T12:00:00Z',
              },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(emptyFriendRequestsResponse())
      .mockResolvedValueOnce(emptySeqsResponse())
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            friends: [
              {
                user_id: '1001',
                friend_id: '2003',
                status: 'accepted',
                is_friend: true,
                created_at: '2026-04-29T12:10:00Z',
                updated_at: '2026-04-29T12:10:00Z',
              },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(emptyFriendRequestsResponse());

    const firstRender = render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    expect(await screen.findByRole('button', { name: '和 未知联系人 聊天' })).toBeInTheDocument();
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith('/friends', expect.objectContaining({ method: 'GET' })));

    firstRender.unmount();

    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    expect(await screen.findByRole('button', { name: '和 未知联系人 聊天' })).toBeInTheDocument();
    await waitFor(() => {
      const friendLoads = fetchMock.mock.calls.filter(([url]) => url === '/friends');
      expect(friendLoads).toHaveLength(2);
    });
  });

  it('opens a chat from a friend by loading the existing direct conversation', async () => {
    const user = userEvent.setup();
    const bob = bobProfile();
    let seqLoads = 0;
    fetchMock.mockReset();
    fetchMock.mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = fetchUrl(input);
      const method = fetchMethod(init);

      if (url === '/conversations/seqs?conversationIds=' && method === 'GET') {
        seqLoads += 1;
        return seqLoads === 1 ? emptySeqsResponse() : conversationSeqResponse('existing chat from Bob');
      }
      if (url === '/friends' && method === 'GET') {
        return friendsResponse([friendshipWith(bob)]);
      }
      if (url === '/friends/requests' && method === 'GET') {
        return emptyFriendRequestsResponse();
      }
      if (url === '/conversations/single%3A1001%3A2002/messages?fromSeq=1&limit=50&order=asc' && method === 'GET') {
        return conversationMessagesResponse('existing chat from Bob');
      }
      if (url === '/conversations/single%3A1001%3A2002/read' && method === 'POST') {
        return jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            conversationId: 'single:1001:2002',
            hasReadSeq: 1,
          },
        });
      }

      throw new Error(`Unhandled test request: ${method} ${url}`);
    });

    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    await user.click(await screen.findByRole('button', { name: '和 bob_002 聊天' }));

    expect(await screen.findByRole('heading', { name: 'Bob', level: 2 })).toBeInTheDocument();
    expect(await screen.findByText('existing chat from Bob')).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith('/conversations/seqs?conversationIds=', expect.objectContaining({ method: 'GET' }));
    expect(fetchMock).toHaveBeenCalledWith(
      '/conversations/single%3A1001%3A2002/messages?fromSeq=1&limit=50&order=asc',
      expect.objectContaining({ method: 'GET' }),
    );
    expect(fetchMock).not.toHaveBeenCalledWith('/users/2002', expect.objectContaining({ method: 'GET' }));
    expect(fetchMock).toHaveBeenCalledWith(
      '/conversations/single%3A1001%3A2002/read',
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ hasReadSeq: 1 }) }),
    );
  });

  it('shows existing conversations immediately when opening the messages tab after navigation', async () => {
    const user = userEvent.setup();
    fetchMock.mockReset();
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            states: [
              {
                conversationId: 'single:1001:2002',
                maxSeq: 1,
                hasReadSeq: 0,
                unreadCount: 1,
                maxSeqTime: 1777464300000,
                lastMessage: {
                  serverMsgId: 'srv-1',
                  clientMsgId: 'client-1',
                  conversationId: 'single:1001:2002',
                  seq: 1,
                  senderId: '2002',
                  receiverId: '1001',
                  groupId: '',
                  chatType: 'single',
                  contentType: 'text',
                  content: 'hello alice',
                  sendTime: 1777464300000,
                  createdAt: 1777464300000,
                },
              },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            conversationId: 'single:1001:2002',
            messages: [
              {
                serverMsgId: 'srv-1',
                clientMsgId: 'client-1',
                conversationId: 'single:1001:2002',
                seq: 1,
                senderId: '2002',
                receiverId: '1001',
                groupId: '',
                chatType: 'single',
                contentType: 'text',
                content: 'hello alice',
                sendTime: 1777464300000,
                createdAt: 1777464300000,
              },
            ],
            isEnd: true,
            nextSeq: 2,
          },
        }),
      )
      .mockResolvedValueOnce(jsonResponse({ code: 'OK', message: 'ok', data: { friends: [] } }))
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            states: [
              {
                conversationId: 'single:1001:2002',
                maxSeq: 1,
                hasReadSeq: 0,
                unreadCount: 1,
                maxSeqTime: 1777464300000,
                lastMessage: {
                  serverMsgId: 'srv-1',
                  clientMsgId: 'client-1',
                  conversationId: 'single:1001:2002',
                  seq: 1,
                  senderId: '2002',
                  receiverId: '1001',
                  groupId: '',
                  chatType: 'single',
                  contentType: 'text',
                  content: 'hello alice',
                  sendTime: 1777464300000,
                  createdAt: 1777464300000,
                },
              },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            conversationId: 'single:1001:2002',
            messages: [
              {
                serverMsgId: 'srv-1',
                clientMsgId: 'client-1',
                conversationId: 'single:1001:2002',
                seq: 1,
                senderId: '2002',
                receiverId: '1001',
                groupId: '',
                chatType: 'single',
                contentType: 'text',
                content: 'hello alice',
                sendTime: 1777464300000,
                createdAt: 1777464300000,
              },
            ],
            isEnd: true,
            nextSeq: 2,
          },
        }),
      );

    render(<App />);

    expect(await screen.findByRole('button', { name: /未知联系人/ })).toBeInTheDocument();
    expect(screen.queryByText('2002')).not.toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    await user.click(screen.getByRole('tab', { name: /消息/i }));

    expect(await screen.findByText('暂无会话')).toBeInTheDocument();
  });

  it('loads real conversations and sends messages through the message API', async () => {
    const user = userEvent.setup();
    fetchMock.mockReset();
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            states: [
              {
                conversationId: 'single:1001:2002',
                maxSeq: 1,
                hasReadSeq: 0,
                unreadCount: 1,
                maxSeqTime: 1777464300000,
                lastMessage: {
                  serverMsgId: 'srv-1',
                  clientMsgId: 'client-1',
                  conversationId: 'single:1001:2002',
                  seq: 1,
                  senderId: '2002',
                  receiverId: '1001',
                  groupId: '',
                  chatType: 'single',
                  contentType: 'text',
                  content: 'hello alice',
                  messageOrigin: 'ai',
                  agentAccountId: '2002',
                  triggerServerMsgId: 'srv-human-1',
                  agentRunId: 'run-app-1',
                  sendTime: 1777464300000,
                  createdAt: 1777464300000,
                },
              },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            conversationId: 'single:1001:2002',
            messages: [
              {
                serverMsgId: 'srv-1',
                clientMsgId: 'client-1',
                conversationId: 'single:1001:2002',
                seq: 1,
                senderId: '2002',
                receiverId: '1001',
                groupId: '',
                chatType: 'single',
                contentType: 'text',
                content: 'hello alice',
                messageOrigin: 'ai',
                agentAccountId: '2002',
                triggerServerMsgId: 'srv-human-1',
                agentRunId: 'run-app-1',
                sendTime: 1777464300000,
                createdAt: 1777464300000,
              },
            ],
            isEnd: true,
            nextSeq: 2,
          },
        }),
      )
      .mockResolvedValueOnce(jsonResponse({ code: 'OK', message: 'ok', data: { friends: [] } }))
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            conversationId: 'single:1001:2002',
            hasReadSeq: 1,
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            message: {
              serverMsgId: 'srv-2',
              clientMsgId: 'web-client-2',
              conversationId: 'single:1001:2002',
              seq: 2,
              senderId: '1001',
              receiverId: '2002',
              groupId: '',
              chatType: 'single',
              contentType: 'text',
              content: '这是测试消息',
              sendTime: 1777464400000,
              createdAt: 1777464400000,
            },
            deduplicated: false,
          },
        }),
      );

    render(<App />);

    await user.click(await screen.findByRole('button', { name: /未知联系人/ }));
    expect(await screen.findByText('hello alice')).toBeInTheDocument();
    expect(await screen.findByText('AI/Agent')).toBeInTheDocument();

    await user.type(screen.getByRole('textbox', { name: '输入消息' }), '这是测试消息');
    await user.click(screen.getByRole('button', { name: '发送' }));

    expect(screen.getByText('这是测试消息')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText('已发送')).toBeInTheDocument());
    expect(fetchMock).toHaveBeenCalledWith(
      '/messages',
      expect.objectContaining({
        method: 'POST',
        body: expect.stringContaining('这是测试消息'),
      }),
    );
  });
});
