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

function pendingResponse() {
  return new Promise<Response>(() => {});
}

function fetchPath(input: RequestInfo | URL) {
  if (typeof input === 'string') {
    return input;
  }
  if (input instanceof URL) {
    return `${input.pathname}${input.search}`;
  }
  return input.url;
}

function fetchMethod(init: RequestInit | undefined) {
  return init?.method ?? 'GET';
}

function countFetchCalls(mock: { mock: { calls: unknown[][] } }, path: string, method = 'GET') {
  return mock.mock.calls.filter(
    ([input, init]) => fetchPath(input as RequestInfo | URL) === path && fetchMethod(init as RequestInit | undefined) === method,
  ).length;
}

function installAppStyles(extraCss = '') {
  const style = document.createElement('style');
  style.dataset.testStyle = 'app-shell';
  style.textContent = `${stylesCss}\n${extraCss}`;
  document.head.append(style);
  return style;
}

function removeInstalledStyles() {
  document.head.querySelectorAll('style[data-test-style="app-shell"]').forEach((style) => style.remove());
}

function getTabPanel(label: string) {
  const panel = screen
    .getAllByRole('tabpanel', { hidden: true })
    .find((candidate) => candidate.getAttribute('aria-label') === label);

  expect(panel).toBeDefined();
  return panel as HTMLElement;
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
    removeInstalledStyles();
  });

  it('shows a WeChat-style login page before authentication and saves the login token', async () => {
    const user = userEvent.setup();
    fetchMock.mockResolvedValue(emptySeqsResponse());
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ code: 'OK', message: 'ok', data: { exists: true, identifier: 'alice_001' } }))
      .mockResolvedValueOnce(
        jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            user_id: '1001',
            identifier: 'alice_001',
            display_name: 'Alice Chen',
            avatar_media_id: 'med_avatar_1',
            avatar_url: '/media/avatars/med_avatar_1',
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
      '/users/exists?identifier=alice_001',
      expect.objectContaining({
        method: 'GET',
      }),
    );
    expect(fetchMock).toHaveBeenCalledWith(
      '/auth/login',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ identifier: 'alice_001', password: 'test-password' }),
      }),
    );
    expect(JSON.parse(localStorage.getItem(AUTH_STORAGE_KEY) ?? '{}')).toMatchObject({
      token: 'login-token',
      user: {
        identifier: 'alice_001',
        displayName: 'Alice Chen',
        avatarMediaId: 'med_avatar_1',
        avatarUrl: '/media/avatars/med_avatar_1',
      },
    });

    await user.click(screen.getByRole('tab', { name: /我的/i }));
    expect(await screen.findByRole('img', { name: 'Alice Chen 头像' })).toHaveAttribute('src', '/media/avatars/med_avatar_1');
  });

  it('checks whether the login identifier exists when the password field receives focus', async () => {
    const user = userEvent.setup();
    fetchMock.mockResolvedValueOnce(jsonResponse({ code: 'OK', message: 'ok', data: { exists: false, identifier: 'missing_001' } }));

    render(<App />);

    await user.type(screen.getByLabelText('账号'), 'missing_001');
    await user.click(screen.getByLabelText('密码'));

    expect(await screen.findByText('账号不存在，请检查后再输入密码')).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      '/users/exists?identifier=missing_001',
      expect.objectContaining({
        method: 'GET',
      }),
    );
    expect(fetchMock).not.toHaveBeenCalledWith('/auth/login', expect.anything());
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
    removeInstalledStyles();
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

  it('keeps stacked page cards at their intrinsic height inside the scrollable mobile viewport', () => {
    expect(stylesCss).toMatch(/\.page-stack\s*>\s*\.md-card\s*{[\s\S]*flex-shrink:\s*0/);
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

  it('hides inactive kept-alive tab panels so previous page content cannot remain visible', async () => {
    const user = userEvent.setup();
    installAppStyles(`
      .tab-panel[hidden] {
        display: flex !important;
      }
    `);
    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    expect(screen.getByRole('heading', { name: '联系人' })).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: /我的/i }));
    expect(screen.getByRole('heading', { name: '我的' })).toBeInTheDocument();

    const messagesPanel = getTabPanel('消息');
    const contactsPanel = getTabPanel('联系人');
    const mePanel = getTabPanel('我的');
    const inactivePanels = [messagesPanel, contactsPanel];

    expect(mePanel).toHaveAttribute('data-active', 'true');
    expect(mePanel).not.toHaveAttribute('hidden');
    expect(mePanel).not.toHaveAttribute('aria-hidden');
    expect(mePanel).not.toHaveAttribute('inert');
    expect(getComputedStyle(mePanel).visibility).toBe('visible');
    expect(within(mePanel).getAllByText(/alice_001/).length).toBeGreaterThan(0);

    for (const panel of inactivePanels) {
      expect(panel).toHaveAttribute('data-active', 'false');
      expect(panel).toHaveAttribute('hidden');
      expect(panel).toHaveAttribute('aria-hidden', 'true');
      expect(panel).toHaveAttribute('inert');
      expect(getComputedStyle(panel).position).toBe('absolute');
      expect(getComputedStyle(panel).visibility).toBe('hidden');
      expect(getComputedStyle(panel).pointerEvents).toBe('none');
      expect(getComputedStyle(panel).overflow).toBe('hidden');
    }
    expect(stylesCss).toMatch(/\.tab-panel\[hidden\]\s*{[\s\S]*display:\s*none\s*!important/);
    expect(stylesCss).toMatch(/\.tab-panel\[data-active="false"\][\s\S]*visibility:\s*hidden/);
    expect(stylesCss).toMatch(/\.tab-panel\[data-active="false"\][\s\S]*pointer-events:\s*none/);
  });

  it('keeps the loaded contacts tab state when switching away and back without refetching friends or requests', async () => {
    const user = userEvent.setup();
    const blockedRefetch = pendingResponse();
    fetchMock.mockReset();
    fetchMock.mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const path = fetchPath(input);
      if (path === '/conversations/seqs?conversationIds=') {
        return Promise.resolve(emptySeqsResponse());
      }
      if (path === '/friends' && fetchMethod(init) === 'GET') {
        if (countFetchCalls(fetchMock, '/friends') === 1) {
          return Promise.resolve(
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
                      gender: '',
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
        }
        return blockedRefetch;
      }
      if (path === '/friends/requests' && fetchMethod(init) === 'GET') {
        if (countFetchCalls(fetchMock, '/friends/requests') === 1) {
          return Promise.resolve(
            jsonResponse({
              code: 'OK',
              message: 'ok',
              data: {
                incoming: [
                  {
                    user_id: '2003',
                    friend_id: '1001',
                    status: 'pending',
                    is_friend: false,
                    profile: {
                      user_id: '2003',
                      identifier: 'carol_003',
                      display_name: 'Carol Wu',
                      name: 'Carol Wu',
                      gender: '',
                      birth_date: '',
                      region: '',
                    },
                    created_at: '2026-04-29T12:10:00Z',
                    updated_at: '2026-04-29T12:10:00Z',
                  },
                ],
                outgoing: [],
              },
            }),
          );
        }
        return blockedRefetch;
      }
      return Promise.reject(new Error(`Unhandled fetch: ${path}`));
    });

    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    const friendsRegion = screen.getByRole('region', { name: '好友列表' });
    expect(await within(friendsRegion).findByRole('button', { name: '和 bob_002 聊天' })).toBeInTheDocument();
    expect(await screen.findByRole('button', { name: '同意 carol_003' })).toBeInTheDocument();
    await user.type(screen.getByLabelText('按账号搜索用户'), 'cached-search');

    await user.click(screen.getByRole('tab', { name: /发现/i }));
    await user.click(screen.getByRole('tab', { name: /联系人/i }));

    expect(within(screen.getByRole('region', { name: '好友列表' })).getByRole('button', { name: '和 bob_002 聊天' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '同意 carol_003' })).toBeInTheDocument();
    expect(screen.getByLabelText('按账号搜索用户')).toHaveValue('cached-search');
    expect(countFetchCalls(fetchMock, '/friends')).toBe(1);
    expect(countFetchCalls(fetchMock, '/friends/requests')).toBe(1);
  });

  it('keeps the loaded messages tab state when switching away and back without refetching conversation seqs', async () => {
    const user = userEvent.setup();
    const blockedRefetch = pendingResponse();
    fetchMock.mockReset();
    fetchMock.mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const path = fetchPath(input);
      if (path === '/conversations/seqs?conversationIds=' && fetchMethod(init) === 'GET') {
        if (countFetchCalls(fetchMock, '/conversations/seqs?conversationIds=') === 1) {
          return Promise.resolve(
            jsonResponse({
              code: 'OK',
              message: 'ok',
              data: {
                states: [
                  {
                    conversationId: 'single:1001:2002',
                    maxSeq: 1,
                    hasReadSeq: 1,
                    unreadCount: 0,
                    maxSeqTime: 1777464300000,
                    lastMessage: {
                      serverMsgId: 'srv-cache-1',
                      clientMsgId: 'client-cache-1',
                      conversationId: 'single:1001:2002',
                      seq: 1,
                      senderId: '2002',
                      receiverId: '1001',
                      groupId: '',
                      chatType: 'single',
                      contentType: 'text',
                      content: 'cached conversation preview',
                      sendTime: 1777464300000,
                      createdAt: 1777464300000,
                    },
                  },
                ],
              },
            }),
          );
        }
        return blockedRefetch;
      }
      if (path === '/conversations/single%3A1001%3A2002/messages?fromSeq=1&limit=50&order=asc' && fetchMethod(init) === 'GET') {
        return Promise.resolve(
          jsonResponse({
            code: 'OK',
            message: 'ok',
            data: {
              conversationId: 'single:1001:2002',
              messages: [
                {
                  serverMsgId: 'srv-cache-1',
                  clientMsgId: 'client-cache-1',
                  conversationId: 'single:1001:2002',
                  seq: 1,
                  senderId: '2002',
                  receiverId: '1001',
                  groupId: '',
                  chatType: 'single',
                  contentType: 'text',
                  content: 'cached conversation preview',
                  sendTime: 1777464300000,
                  createdAt: 1777464300000,
                },
              ],
            },
          }),
        );
      }
      if (path === '/friends' && fetchMethod(init) === 'GET') {
        return Promise.resolve(
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
                  },
                  created_at: '2026-04-29T12:00:00Z',
                  updated_at: '2026-04-29T12:00:00Z',
                },
              ],
            },
          }),
        );
      }
      return Promise.reject(new Error(`Unhandled fetch: ${path}`));
    });

    render(<App />);

    const row = await screen.findByRole('button', { name: /Bob Lin/ });
    expect(within(row).getByText('cached conversation preview')).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: /发现/i }));
    await user.click(screen.getByRole('tab', { name: /消息/i }));

    expect(within(screen.getByRole('button', { name: /Bob Lin/ })).getByText('cached conversation preview')).toBeInTheDocument();
    expect(countFetchCalls(fetchMock, '/conversations/seqs?conversationIds=')).toBe(1);
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
      patchCurrentUserAvatar: vi.fn(async () => initialProfile),
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

  it('keeps an uploaded avatar visible after remounting from the persisted session', async () => {
    const user = userEvent.setup();
    const uploadedProfile: UserProfile = {
      ...initialProfile,
      display_name: 'Alice Chen',
      name: 'Alice Chen',
      avatar_media_id: 'med_avatar_2',
      avatar_url: '/media/avatars/med_avatar_2',
    };
    const patchCurrentUserAvatar = vi.fn(async () => uploadedProfile);
    const userApi = {
      getCurrentUser: vi.fn(async () => uploadedProfile),
      identifierExists: vi.fn(async () => ({ identifier: uploadedProfile.identifier, exists: true })),
      getPublicProfileByIdentifier: vi.fn(async () => uploadedProfile),
      patchCurrentUser: vi.fn(async () => uploadedProfile),
      patchCurrentUserAvatar,
    };

    fetchMock.mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = fetchPath(input);
      const method = fetchMethod(init);
      if (path === '/media/uploads' && method === 'POST') {
        return jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            mediaId: 'med_avatar_2',
            objectKey: 'users/1001/media/med_avatar_2/avatar.jpg',
            uploadUrl: 'https://storage.test/upload/avatar',
            expiresAt: 1777550400000,
          },
        });
      }
      if (path === 'https://storage.test/upload/avatar' && method === 'PUT') {
        return new Response(null, { status: 200 });
      }
      if (path === '/media/uploads/med_avatar_2/complete' && method === 'POST') {
        return jsonResponse({
          code: 'OK',
          message: 'ok',
          data: {
            media: {
              mediaId: 'med_avatar_2',
              ownerUserId: '1001',
              bucket: 'agents-im-media',
              objectKey: 'users/1001/media/med_avatar_2/avatar.jpg',
              sha256: '',
              contentType: 'image/jpeg',
              sizeBytes: 1024,
              originalFilename: 'avatar.jpg',
              purpose: 'avatar',
              status: 'ready',
              createdAt: '2026-05-06T10:00:00Z',
              updatedAt: '2026-05-06T10:00:00Z',
            },
          },
        });
      }
      return emptySeqsResponse();
    });

    const firstRender = render(<App userApi={userApi} />);
    await user.click(screen.getByRole('tab', { name: /我的/i }));

    await user.upload(screen.getByLabelText('上传头像'), new File([new Uint8Array(1024)], 'avatar.jpg', { type: 'image/jpeg' }));

    await waitFor(() => expect(patchCurrentUserAvatar).toHaveBeenCalledWith('med_avatar_2'));
    expect(JSON.parse(localStorage.getItem(AUTH_STORAGE_KEY) ?? '{}')).toMatchObject({
      user: {
        avatarMediaId: 'med_avatar_2',
        avatarUrl: '/media/avatars/med_avatar_2',
      },
    });

    firstRender.unmount();
    render(<App userApi={userApi} />);

    await user.click(screen.getByRole('tab', { name: /我的/i }));
    expect(await screen.findByRole('img', { name: 'Alice Chen 头像' })).toHaveAttribute('src', '/media/avatars/med_avatar_2');
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
          data: { friends: [] },
        }),
      )
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
            gender: '',
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
                user_id: '1001',
                friend_id: '2002',
                status: 'accepted',
                is_friend: true,
                friend: {
                  user_id: '2002',
                  identifier: 'bob_002',
                  display_name: 'Bob',
                  name: 'Bob',
                  gender: '',
                  birth_date: '',
                  region: '',
                },
                created_at: '2026-04-29T12:00:00Z',
                updated_at: '2026-04-29T12:00:00Z',
              },
            ],
          },
        }),
      );

    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    await user.type(screen.getByLabelText('按账号搜索用户'), 'bob_002');
    await user.click(screen.getByRole('button', { name: '搜索用户' }));

    const searchRegion = screen.getByRole('region', { name: '账号搜索' });
    await waitFor(() => expect(screen.getAllByRole('status').map((node) => node.textContent).join(' ')).toContain('找到 Bob Lin'));
    expect(within(searchRegion).getByText('bob_002')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '添加好友 bob_002' }));

    await waitFor(() => expect(screen.getAllByRole('status').map((node) => node.textContent).join(' ')).toContain('已添加好友：bob_002'));
    expect(screen.getByRole('button', { name: '已添加' })).toBeDisabled();
    expect(fetchMock).toHaveBeenCalledWith('/users/bob_002', expect.objectContaining({ method: 'GET' }));
    expect(fetchMock).toHaveBeenCalledWith(
      '/friends',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ user_id: '2002' }),
      }),
    );
    expect(screen.queryByText('2002')).not.toBeInTheDocument();
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
                friend: {
                  user_id: '2002',
                  identifier: 'bob_002',
                  display_name: 'Bob',
                  name: 'Bob',
                  gender: '',
                  birth_date: '',
                  region: '',
                },
                created_at: '2026-04-29T12:00:00Z',
                updated_at: '2026-04-29T12:00:00Z',
              },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(emptyFriendRequestsResponse())
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
                  display_name: 'Bob',
                  name: 'Bob',
                  gender: '',
                  birth_date: '',
                  region: '',
                },
                created_at: '2026-04-29T12:00:00Z',
                updated_at: '2026-04-29T12:00:00Z',
              },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(emptyFriendRequestsResponse())
      .mockResolvedValueOnce(emptySeqsResponse())
      .mockResolvedValueOnce(emptySeqsResponse());

    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));

    expect(await screen.findByRole('button', { name: '和 bob_002 聊天' })).toBeInTheDocument();
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
      );

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

  it('opens a chat from a friend without remounting or refetching the messages page', async () => {
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
                friend: {
                  user_id: '2002',
                  identifier: 'bob_002',
                  display_name: 'Bob',
                  name: 'Bob',
                  gender: '',
                  birth_date: '',
                  region: '',
                },
                created_at: '2026-04-29T12:00:00Z',
                updated_at: '2026-04-29T12:00:00Z',
              },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(emptyFriendRequestsResponse())
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
                  clientMsgId: 'client-existing-1',
                  conversationId: 'single:1001:2002',
                  seq: 1,
                  senderId: '2002',
                  receiverId: '1001',
                  groupId: '',
                  chatType: 'single',
                  contentType: 'text',
                  content: 'existing chat from Bob',
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
                serverMsgId: 'srv-existing-1',
                clientMsgId: 'client-existing-1',
                conversationId: 'single:1001:2002',
                seq: 1,
                senderId: '2002',
                receiverId: '1001',
                groupId: '',
                chatType: 'single',
                contentType: 'text',
                content: 'existing chat from Bob',
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
            conversationId: 'single:1001:2002',
            hasReadSeq: 1,
          },
        }),
      );

    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    await user.click(await screen.findByRole('button', { name: '和 bob_002 聊天' }));

    expect(await screen.findByRole('heading', { name: 'Bob', level: 2 })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: '输入消息' })).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith('/conversations/seqs?conversationIds=', expect.objectContaining({ method: 'GET' }));
    expect(countFetchCalls(fetchMock, '/conversations/seqs?conversationIds=')).toBe(1);
    expect(fetchMock).not.toHaveBeenCalledWith('/users/2002', expect.objectContaining({ method: 'GET' }));
  });

  it('opens group management from a group chat started in Contacts', async () => {
    const user = userEvent.setup();
    fetchMock.mockReset();
    fetchMock.mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const path = fetchPath(input);
      const method = fetchMethod(init);

      if (path === '/conversations/seqs?conversationIds=' && method === 'GET') {
        return Promise.resolve(emptySeqsResponse());
      }
      if (path === '/friends' && method === 'GET') {
        return Promise.resolve(jsonResponse({ code: 'OK', message: 'ok', data: { friends: [] } }));
      }
      if (path === '/friends/requests' && method === 'GET') {
        return Promise.resolve(emptyFriendRequestsResponse());
      }
      if (path === '/groups' && method === 'GET') {
        return Promise.resolve(
          jsonResponse({
            code: 'OK',
            message: 'ok',
            data: {
              groups: [
                {
                  group_id: 'grp_team',
                  name: '项目群',
                  description: '联系人入口',
                  announcement: '联系人入口',
                  avatar_media_id: '',
                  avatar_url: '',
                  creator_user_id: '1001',
                  current_user_role: 'owner',
                  created_at: '2026-05-05T12:00:00Z',
                  updated_at: '2026-05-05T12:00:00Z',
                },
              ],
            },
          }),
        );
      }
      if (path === '/groups/grp_team' && method === 'GET') {
        return Promise.resolve(
          jsonResponse({
            code: 'OK',
            message: 'ok',
            data: {
              group_id: 'grp_team',
              name: '项目群',
              description: '联系人入口',
              announcement: '联系人入口',
              avatar_media_id: '',
              avatar_url: '',
              creator_user_id: '1001',
              current_user_role: 'owner',
              created_at: '2026-05-05T12:00:00Z',
              updated_at: '2026-05-05T12:00:00Z',
            },
          }),
        );
      }
      if (path === '/groups/grp_team/members' && method === 'GET') {
        return Promise.resolve(
          jsonResponse({
            code: 'OK',
            message: 'ok',
            data: {
              group_id: 'grp_team',
              members: [
                {
                  group_id: 'grp_team',
                  user_id: '1001',
                  role: 'owner',
                  state: 'active',
                  joined_at: '2026-05-05T12:00:00Z',
                  left_at: '',
                  identifier: 'alice_001',
                  display_name: 'Alice Chen',
                  name: 'Alice Chen',
                },
              ],
            },
          }),
        );
      }
      return Promise.reject(new Error(`Unhandled fetch: ${method} ${path}`));
    });

    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    await user.click(await screen.findByRole('button', { name: '打开群聊' }));
    await user.click(await screen.findByRole('button', { name: '打开群聊 项目群' }));

    expect(await screen.findByRole('heading', { name: '项目群', level: 2 })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '打开群管理 项目群' }));

    expect(await screen.findByRole('heading', { name: '群管理' })).toBeInTheDocument();
    expect(screen.getByTestId('group-member-grid')).toHaveClass('group-member-grid');
    expect(screen.getByText('联系人入口')).toBeInTheDocument();
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

    expect(screen.getByRole('button', { name: /未知联系人/ })).toBeInTheDocument();
    expect(screen.getByText('hello alice')).toBeInTheDocument();
    expect(countFetchCalls(fetchMock, '/conversations/seqs?conversationIds=')).toBe(1);
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
            conversationId: 'single:1001:2002',
            chatType: 'single',
            enabled: false,
            available: true,
            peerEnabled: false,
            maxRecentMessages: 30,
            summaryEnabled: false,
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
    await waitFor(() => expect(screen.getByRole('img', { name: '发送成功' })).toHaveTextContent('✔'));
    expect(screen.queryByText('已发送')).not.toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      '/messages',
      expect.objectContaining({
        method: 'POST',
        body: expect.stringContaining('这是测试消息'),
      }),
    );
  });
});
