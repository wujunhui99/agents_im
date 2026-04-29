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
    token: 'stored-mock-token',
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
          token: 'mock-login-token',
          expires_at: '2026-04-30T12:00:00Z',
        },
      }),
    );

    render(<App />);

    expect(screen.getByRole('heading', { name: '登录 Agents IM' })).toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: /消息/i })).not.toBeInTheDocument();

    await user.type(screen.getByLabelText('账号'), 'alice_001');
    await user.type(screen.getByLabelText('密码'), 'mock-password');
    await user.click(screen.getByRole('button', { name: '登录' }));

    expect(await screen.findByRole('heading', { name: '消息' })).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      '/auth/login',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ identifier: 'alice_001', password: 'mock-password' }),
      }),
    );
    expect(JSON.parse(localStorage.getItem(AUTH_STORAGE_KEY) ?? '{}')).toMatchObject({
      token: 'mock-login-token',
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
          token: 'mock-register-token',
          expires_at: '2026-04-30T12:00:00Z',
        },
      }),
    );

    render(<App />);

    await user.click(screen.getByRole('button', { name: '注册账号' }));
    expect(screen.getByRole('heading', { name: '注册 Agents IM' })).toBeInTheDocument();

    await user.type(screen.getByLabelText('账号'), 'new_user');
    await user.type(screen.getByLabelText('昵称'), 'New User');
    await user.type(screen.getByLabelText('密码'), 'mock-password');
    await user.click(screen.getByRole('button', { name: '注册并登录' }));

    expect(await screen.findByRole('heading', { name: '消息' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /我的/i })).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      '/auth/register',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          identifier: 'new_user',
          password: 'mock-password',
          display_name: 'New User',
        }),
      }),
    );
    expect(JSON.parse(localStorage.getItem(AUTH_STORAGE_KEY) ?? '{}')).toMatchObject({
      token: 'mock-register-token',
      user: { identifier: 'new_user', displayName: 'New User' },
    });
  });

  it('logs out and returns to the login page', async () => {
    const user = userEvent.setup();
    storeSession();

    render(<App />);

    await user.click(screen.getByRole('tab', { name: /我的/i }));
    expect(screen.getAllByText('Alice Chen').length).toBeGreaterThan(0);

    await user.click(screen.getByRole('button', { name: '退出登录' }));

    expect(screen.getByRole('heading', { name: '登录 Agents IM' })).toBeInTheDocument();
    expect(localStorage.getItem(AUTH_STORAGE_KEY)).toBeNull();
  });
});

describe('WeChat-inspired app shell', () => {
  beforeEach(() => {
    localStorage.clear();
    storeSession();
  });

  afterEach(() => {
    localStorage.clear();
  });

  it('renders the four primary tabs', () => {
    render(<App />);

    expect(screen.getByRole('tab', { name: /消息/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /联系人/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /发现/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /我的/i })).toBeInTheDocument();
  });

  it('defaults to the messages page and switches pages from the bottom navigation', async () => {
    const user = userEvent.setup();
    render(<App />);

    expect(screen.getByRole('heading', { name: '消息' })).toBeInTheDocument();
    expect(screen.getByText('产品讨论群')).toBeInTheDocument();

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

  it('shows contact entry points and groups friends by initial', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));

    expect(screen.getByRole('button', { name: /新的朋友/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /群聊/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /标签/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /公众号/i })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'A' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'B' })).toBeInTheDocument();
    expect(screen.getByText('Alice Chen')).toBeInTheDocument();
    expect(screen.getByText('Bob Lin')).toBeInTheDocument();
  });

  it('searches users by unique identifier', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    await user.type(screen.getByLabelText('按 identifier 搜索用户'), 'bob_002');
    await user.click(screen.getByRole('button', { name: '搜索用户' }));

    const searchRegion = screen.getByRole('region', { name: '账号搜索' });
    expect(screen.getByRole('status')).toHaveTextContent('找到 Bob Lin');
    expect(within(searchRegion).getByText('bob_002')).toBeInTheDocument();
  });

  it('exposes an add friend action from identifier search results', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    await user.type(screen.getByLabelText('按 identifier 搜索用户'), 'bob_002');
    await user.click(screen.getByRole('button', { name: '搜索用户' }));
    await user.click(screen.getByRole('button', { name: '添加好友 bob_002' }));

    expect(screen.getByRole('status')).toHaveTextContent('已发送添加请求');
  });

  it('opens group details and renders the member list', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    await user.click(screen.getByRole('button', { name: /群聊/i }));
    await user.click(screen.getByRole('button', { name: /查看 Frontend Demo/i }));

    expect(screen.getByRole('heading', { name: 'Frontend Demo' })).toBeInTheDocument();
    expect(screen.getByRole('list', { name: '群成员列表' })).toBeInTheDocument();
    expect(screen.getByText('Alice Chen')).toBeInTheDocument();
    expect(screen.getByText('Bob Lin')).toBeInTheDocument();
  });

  it('shows the seeded conversation list on the messages tab', () => {
    render(<App />);

    const list = screen.getByRole('list', { name: '消息列表' });
    expect(within(list).getByRole('button', { name: /产品讨论群/ })).toBeInTheDocument();
    expect(within(list).getByRole('button', { name: /junhui/ })).toBeInTheDocument();
    expect(screen.getByText('后端 MVP 已发布，开始搭前端主框架。')).toBeInTheDocument();
  });

  it('opens a conversation and returns to the list on mobile navigation', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: /产品讨论群/ }));

    expect(screen.getByRole('heading', { name: '产品讨论群' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: '输入消息' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '返回消息列表' }));

    expect(screen.getByRole('list', { name: '消息列表' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /产品讨论群/ })).toBeInTheDocument();
  });

  it('appends a sending message from the composer before marking it sent', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: /junhui/ }));
    await user.type(screen.getByRole('textbox', { name: '输入消息' }), '这是测试消息');
    await user.click(screen.getByRole('button', { name: '发送' }));

    expect(screen.getByText('这是测试消息')).toBeInTheDocument();
    expect(screen.getByText('发送中')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByText('已发送')).toBeInTheDocument());
  });
});
