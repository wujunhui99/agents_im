import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi } from 'vitest';
import App from './App';
import type { UserProfile, UserProfilePatch } from './api/user';
import { AUTH_STORAGE_KEY, type AuthSession } from './auth/session';

const initialProfile: UserProfile = {
  user_id: 'usr_000001',
  identifier: 'alice_001',
  display_name: 'Alice',
  name: 'Alice',
  gender: 'female',
  age: 30,
  region: 'Shanghai',
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
      userId: 'usr_000001',
      identifier: 'alice_001',
      displayName: 'Alice Chen',
      gender: 'female',
      age: 30,
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
          user_id: 'usr_000001',
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
          user_id: 'usr_000002',
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

    expect(screen.getByText('usr_000001')).toBeInTheDocument();
    expect(screen.getByText('alice_001')).toBeInTheDocument();
    expect(screen.getByText('female')).toBeInTheDocument();
    expect(screen.getByText('30')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '编辑个人资料' }));
    await user.clear(screen.getByLabelText('display_name'));
    await user.type(screen.getByLabelText('display_name'), 'Alice Chen');
    await user.clear(screen.getByLabelText('region'));
    await user.type(screen.getByLabelText('region'), 'Hangzhou');
    await user.clear(screen.getByLabelText('age'));
    await user.type(screen.getByLabelText('age'), '31');
    await user.click(screen.getByRole('button', { name: '保存' }));

    expect(patchCurrentUser).toHaveBeenCalledWith({
      display_name: 'Alice Chen',
      gender: 'female',
      age: 31,
      region: 'Hangzhou',
    });
    expect((await screen.findAllByText('Alice Chen')).length).toBeGreaterThan(0);
    expect(screen.getAllByText('Hangzhou').length).toBeGreaterThan(0);
  });

  it('searches users by unique identifier and sends add-friend through real APIs', async () => {
    const user = userEvent.setup();
    fetchMock.mockReset();
    fetchMock
      .mockResolvedValueOnce(emptySeqsResponse())
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            user_id: 'usr_000002',
            identifier: 'bob_002',
            display_name: 'Bob Lin',
            name: 'Bob Lin',
            gender: '',
            age: 0,
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
              user_id: 'usr_000001',
              friend_id: 'usr_000002',
              status: 'accepted',
              is_friend: true,
              created_at: '2026-04-29T12:00:00Z',
              updated_at: '2026-04-29T12:00:00Z',
            },
            created: true,
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            friends: [
              {
                user_id: 'usr_000001',
                friend_id: 'usr_000002',
                status: 'accepted',
                is_friend: true,
                created_at: '2026-04-29T12:00:00Z',
                updated_at: '2026-04-29T12:00:00Z',
              },
            ],
          },
        }),
      );

    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    await user.type(screen.getByLabelText('按 identifier 搜索用户'), 'bob_002');
    await user.click(screen.getByRole('button', { name: '搜索用户' }));

    const searchRegion = screen.getByRole('region', { name: '账号搜索' });
    await waitFor(() => expect(screen.getAllByRole('status').map((node) => node.textContent).join(' ')).toContain('找到 Bob Lin'));
    expect(within(searchRegion).getByText('bob_002')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '添加好友 bob_002' }));

    await waitFor(() => expect(screen.getAllByRole('status').map((node) => node.textContent).join(' ')).toContain('已添加好友：bob_002'));
    expect(fetchMock).toHaveBeenCalledWith('/users/bob_002', expect.objectContaining({ method: 'GET' }));
    expect(fetchMock).toHaveBeenCalledWith(
      '/friends',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ user_id: 'usr_000002' }),
      }),
    );
    expect(await screen.findByText('usr_000002')).toBeInTheDocument();
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
                conversationId: 'single:usr_000001:usr_000002',
                maxSeq: 1,
                hasReadSeq: 0,
                unreadCount: 1,
                maxSeqTime: 1777464300000,
                lastMessage: {
                  serverMsgId: 'srv-1',
                  clientMsgId: 'client-1',
                  conversationId: 'single:usr_000001:usr_000002',
                  seq: 1,
                  senderId: 'usr_000002',
                  receiverId: 'usr_000001',
                  groupId: '',
                  chatType: 'single',
                  contentType: 'text',
                  content: 'hello alice',
                  messageOrigin: 'ai',
                  agentAccountId: 'usr_000002',
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
            conversationId: 'single:usr_000001:usr_000002',
            messages: [
              {
                serverMsgId: 'srv-1',
                clientMsgId: 'client-1',
                conversationId: 'single:usr_000001:usr_000002',
                seq: 1,
                senderId: 'usr_000002',
                receiverId: 'usr_000001',
                groupId: '',
                chatType: 'single',
                contentType: 'text',
                content: 'hello alice',
                messageOrigin: 'ai',
                agentAccountId: 'usr_000002',
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
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            message: {
              serverMsgId: 'srv-2',
              clientMsgId: 'web-client-2',
              conversationId: 'single:usr_000001:usr_000002',
              seq: 2,
              senderId: 'usr_000001',
              receiverId: 'usr_000002',
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

    await user.click(await screen.findByRole('button', { name: /usr_000002/ }));
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
