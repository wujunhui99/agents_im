import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { AdminConsole } from './AdminConsole';
import type { AdminApi, AdminConversationMessagesResponse, AdminDashboard, AdminUserSearchResponse } from '../api/admin';

const dashboard: AdminDashboard = {
  totals: {
    users: 3,
    conversations: 2,
    messages: 4,
    aiRuns: 1,
    failedAiRuns: 1,
  },
  recentTraces: [
    {
      traceId: 'trace_admin_1',
      runId: 'run_admin_1',
      traceUrl: 'https://grafana.agenticim.xyz/explore?left=tempo-trace-4bf92f3577b34da6a3ce929d0e0e4736',
      status: 'failed',
      conversationId: 'single:1001:2002',
      agentId: 'agent_1',
      model: 'deepseek-chat',
      provider: 'deepseek',
      promptHash: 'prompt-hash-1',
      startedAt: '2026-05-18T08:00:00Z',
      createdAt: '2026-05-18T08:00:00Z',
    },
  ],
  recentConversations: [
    {
      conversationId: 'single:1001:2002',
      maxSeq: 2,
      unreadCount: 0,
      maxSeqTime: 1777464010000,
      lastMessage: {
        serverMsgId: 'msg_2',
        clientMsgId: 'client_2',
        conversationId: 'single:1001:2002',
        seq: 2,
        senderId: '2002',
        receiverId: '1001',
        chatType: 'single',
        contentType: 'text',
        content: 'AI response for inspector',
        messageOrigin: 'ai',
        agentRunId: 'run_admin_1',
        triggerServerMsgId: 'msg_1',
        sendTime: 1777464010000,
        createdAt: 1777464010000,
      },
    },
  ],
};

const conversationMessages: AdminConversationMessagesResponse = {
  conversationId: 'single:1001:2002',
  messages: [
    {
      serverMsgId: 'msg_1',
      clientMsgId: 'client_1',
      conversationId: 'single:1001:2002',
      seq: 1,
      senderId: '1001',
      receiverId: '2002',
      chatType: 'single',
      contentType: 'text',
      content: 'hello from admin inspector',
      messageOrigin: 'human',
      sendTime: 1777464000000,
      createdAt: 1777464000000,
    },
    {
      serverMsgId: 'msg_2',
      clientMsgId: 'client_2',
      conversationId: 'single:1001:2002',
      seq: 2,
      senderId: '2002',
      receiverId: '1001',
      chatType: 'single',
      contentType: 'text',
      content: 'AI response for inspector',
      messageOrigin: 'ai',
      agentRunId: 'run_admin_1',
      triggerServerMsgId: 'msg_1',
      sendTime: 1777464010000,
      createdAt: 1777464010000,
    },
  ],
  isEnd: true,
  nextSeq: 3,
};

const userSearch: AdminUserSearchResponse = {
  users: [
    {
      userId: '1001',
      identifier: 'alice_001',
      displayName: 'Alice',
      name: 'Alice',
      gender: 'female',
      birthDate: '1996-05-02',
      region: 'Shanghai',
      accountType: 'user',
      createdAt: '2026-05-01T00:00:00Z',
      updatedAt: '2026-05-01T00:00:00Z',
    },
  ],
};

const bobSearch: AdminUserSearchResponse = {
  users: [
    {
      userId: '2002',
      identifier: 'bob_002',
      displayName: 'Bob',
      name: 'Bob Zhang',
      gender: '',
      birthDate: '',
      region: 'Hangzhou',
      accountType: 'agent',
      createdAt: '2026-05-02T00:00:00Z',
      updatedAt: '2026-05-03T00:00:00Z',
    },
  ],
};

function createAdminApi(overrides?: Partial<AdminApi>): AdminApi {
  return {
    getDashboard: vi.fn(async () => dashboard),
    listLLMTraces: vi.fn(async () => ({ traces: dashboard.recentTraces })),
    getLLMTraceDetail: vi.fn(async () => ({
      trace: dashboard.recentTraces[0],
      toolCalls: [],
      fileReads: [],
      pythonExecs: [],
    })),
    getConversationMessages: vi.fn(async () => conversationMessages),
    searchUsers: vi.fn(async () => userSearch),
    getUserDetail: vi.fn(async () => ({ user: userSearch.users[0] })),
    getUserFriends: vi.fn(async () => ({
      friends: [
        {
          userId: '1001',
          friendId: '2002',
          status: 'accepted',
          isFriend: true,
          createdAt: '2026-05-01T00:00:00Z',
          updatedAt: '2026-05-01T00:00:00Z',
          friend: {
            userId: '2002',
            identifier: 'bob_002',
            displayName: 'Bob',
            name: 'Bob',
            gender: '',
            birthDate: '',
            region: '',
            accountType: 'user',
          },
        },
      ],
    })),
    getUserConversations: vi.fn(async () => ({
      conversations: [
        {
          conversationId: 'single:1001:2002',
          maxSeq: 2,
          unreadCount: 0,
          maxSeqTime: 1777464000000,
        },
      ],
    })),
    listFeedback: vi.fn(async () => ({
      items: [
        {
          feedbackId: 'fb_1',
          userId: '1001',
          category: 'bug',
          status: 'new',
          title: '消息发送失败',
          content: '点击发送后没有响应',
          contact: 'alice@example.com',
          createdAt: '2026-05-25T01:00:00Z',
          updatedAt: '2026-05-25T01:00:00Z',
        },
      ],
    })),
    getFeedback: vi.fn(async () => ({
      feedback: {
        feedbackId: 'fb_1',
        userId: '1001',
        category: 'bug',
        status: 'new',
        title: '消息发送失败',
        content: '点击发送后没有响应',
        contact: 'alice@example.com',
        createdAt: '2026-05-25T01:00:00Z',
        updatedAt: '2026-05-25T01:00:00Z',
      },
    })),
    updateFeedback: vi.fn(async () => ({
      feedback: {
        feedbackId: 'fb_1',
        userId: '1001',
        category: 'bug',
        status: 'triaged',
        title: '消息发送失败',
        content: '点击发送后没有响应',
        adminNote: '已分派',
        createdAt: '2026-05-25T01:00:00Z',
        updatedAt: '2026-05-25T01:05:00Z',
      },
    })),
    ...overrides,
  };
}

describe('AdminConsole', () => {
  it('renders dashboard overview cards, trace table, and navigation', async () => {
    render(<AdminConsole adminApi={createAdminApi()} />);

    expect(screen.getByText('Loading admin dashboard')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'AgenticIM Management' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Dashboard' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'LLM Traces' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Conversation' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Users' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Feedback' })).toBeInTheDocument();
    expect(screen.getByText('Users')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('Failed AI runs')).toBeInTheDocument();
    expect(screen.getByText('trace_admin_1')).toBeInTheDocument();
  });

  it('shows Tempo span graph link on LLM trace detail when provided', async () => {
    const user = userEvent.setup();
    render(<AdminConsole adminApi={createAdminApi()} />);

    await user.click(await screen.findByRole('button', { name: 'LLM Traces' }));
    await user.click(await screen.findByRole('button', { name: /trace_admin_1/ }));

    const link = await screen.findByRole('link', { name: 'Open in Tempo' });
    expect(link).toHaveAttribute('href', 'https://grafana.agenticim.xyz/explore?left=tempo-trace-4bf92f3577b34da6a3ce929d0e0e4736');
  });

  it('loads messages after entering a conversation id', async () => {
    const user = userEvent.setup();
    const adminApi = createAdminApi();

    render(<AdminConsole adminApi={adminApi} />);

    await user.click(await screen.findByRole('button', { name: 'Conversation' }));
    await user.type(screen.getByLabelText('Conversation ID'), 'single:1001:2002');
    await user.click(screen.getByRole('button', { name: 'Load conversation' }));

    const messages = await screen.findByLabelText('Conversation messages');
    expect(within(messages).getByText('hello from admin inspector')).toBeInTheDocument();
    expect(within(messages).getByText('AI response for inspector')).toBeInTheDocument();
    expect(within(messages).getByText('seq 2')).toBeInTheDocument();
    expect(adminApi.getConversationMessages).toHaveBeenCalledWith('single:1001:2002');
  });

  it('renders a default conversation list and opens a listed conversation', async () => {
    const user = userEvent.setup();
    const adminApi = createAdminApi();

    render(<AdminConsole adminApi={adminApi} />);

    await user.click(await screen.findByRole('button', { name: 'Conversation' }));

    const browseList = await screen.findByRole('table', { name: 'Conversation browse list' });
    expect(within(browseList).getByText('single:1001:2002')).toBeInTheDocument();
    expect(within(browseList).getByText('max seq 2')).toBeInTheDocument();
    expect(within(browseList).getByText('AI response for inspector')).toBeInTheDocument();
    expect(within(browseList).getByText(/2026/)).toBeInTheDocument();

    await user.click(within(browseList).getByRole('button', { name: 'Open conversation single:1001:2002' }));

    const messages = await screen.findByLabelText('Conversation messages');
    expect(within(messages).getByText('hello from admin inspector')).toBeInTheDocument();
    expect(within(messages).getByText('AI response for inspector')).toBeInTheDocument();
    expect(adminApi.getConversationMessages).toHaveBeenCalledWith('single:1001:2002');
  });

  it('renders a default user list and opens the existing user drill-down', async () => {
    const user = userEvent.setup();
    const adminApi = createAdminApi();

    render(<AdminConsole adminApi={adminApi} />);

    await user.click(await screen.findByRole('button', { name: 'Users' }));

    await waitFor(() => expect(adminApi.searchUsers).toHaveBeenCalledWith(''));
    const browseList = await screen.findByRole('table', { name: 'User browse results' });
    expect(within(browseList).getByText('Alice')).toBeInTheDocument();
    expect(within(browseList).getByText('alice_001')).toBeInTheDocument();
    expect(within(browseList).getByText('user')).toBeInTheDocument();
    expect(within(browseList).getByText('Shanghai')).toBeInTheDocument();
    expect(within(browseList).getByText(/2026-05-01/)).toBeInTheDocument();

    await user.click(within(browseList).getByRole('button', { name: 'Open alice_001' }));

    expect(await screen.findByRole('heading', { name: 'Alice' })).toBeInTheDocument();
    expect(screen.getByText('bob_002')).toBeInTheDocument();
    expect(adminApi.getUserDetail).toHaveBeenCalledWith('1001');
    expect(adminApi.getUserFriends).toHaveBeenCalledWith('1001');
    expect(adminApi.getUserConversations).toHaveBeenCalledWith('1001');
  });

  it('keeps user search working and restores the default list for an empty query', async () => {
    const user = userEvent.setup();
    const adminApi = createAdminApi({
      searchUsers: vi.fn(async (query) => (query.trim() === 'bob' ? bobSearch : userSearch)),
    });

    render(<AdminConsole adminApi={adminApi} />);

    await user.click(await screen.findByRole('button', { name: 'Users' }));
    await waitFor(() => expect(adminApi.searchUsers).toHaveBeenCalledWith(''));

    await user.type(screen.getByLabelText('User query'), 'bob');
    await user.click(screen.getByRole('button', { name: 'Search users' }));
    const searchList = await screen.findByRole('table', { name: 'User browse results' });
    expect(within(searchList).getByText('Bob')).toBeInTheDocument();
    expect(within(searchList).getByText('bob_002')).toBeInTheDocument();

    await user.clear(screen.getByLabelText('User query'));
    await user.click(screen.getByRole('button', { name: 'Search users' }));

    expect(await screen.findByText('Alice')).toBeInTheDocument();
    expect(screen.getByText('alice_001')).toBeInTheDocument();
    await waitFor(() => expect(adminApi.searchUsers).toHaveBeenLastCalledWith(''));
  });

  it('searches a user, shows friends and conversations, and opens a clicked conversation', async () => {
    const user = userEvent.setup();
    const adminApi = createAdminApi();

    render(<AdminConsole adminApi={adminApi} />);

    await user.click(await screen.findByRole('button', { name: 'Users' }));
    await user.type(screen.getByLabelText('User query'), 'alice');
    await user.click(screen.getByRole('button', { name: 'Search users' }));
    await user.click(await screen.findByRole('button', { name: 'Open alice_001' }));

    expect(await screen.findByText('Alice')).toBeInTheDocument();
    expect(screen.getByText('bob_002')).toBeInTheDocument();
    const conversationsRegion = screen.getByRole('region', { name: 'User conversations' });
    await user.click(within(conversationsRegion).getByRole('button', { name: 'Open single:1001:2002' }));

    expect(await screen.findByRole('heading', { name: 'Conversation Inspector' })).toBeInTheDocument();
    await waitFor(() => expect(adminApi.getConversationMessages).toHaveBeenCalledWith('single:1001:2002'));
    const messages = await screen.findByLabelText('Conversation messages');
    expect(within(messages).getByText('AI response for inspector')).toBeInTheDocument();
  });

  it('lists feedback and allows admin status updates', async () => {
    const user = userEvent.setup();
    const adminApi = createAdminApi();

    render(<AdminConsole adminApi={adminApi} />);

    await user.click(await screen.findByRole('button', { name: 'Feedback' }));
    await waitFor(() => expect(adminApi.listFeedback).toHaveBeenCalledWith({ status: 'new' }));
    const feedbackTable = await screen.findByRole('table', { name: 'Feedback list' });
    expect(within(feedbackTable).getByText('消息发送失败')).toBeInTheDocument();
    expect(within(feedbackTable).getByText('bug')).toBeInTheDocument();
    expect(within(feedbackTable).getByText('New')).toBeInTheDocument();

    await user.click(within(feedbackTable).getByRole('button', { name: 'Open feedback fb_1' }));
    expect(await screen.findByRole('heading', { name: '消息发送失败' })).toBeInTheDocument();
    await user.selectOptions(screen.getByLabelText('反馈状态'), 'triaged');
    await user.type(screen.getByLabelText('管理员备注'), '已分派');
    await user.click(screen.getByRole('button', { name: '保存反馈处理' }));

    await waitFor(() => expect(adminApi.updateFeedback).toHaveBeenCalledWith('fb_1', { status: 'triaged', adminNote: '已分派' }));
    expect(await screen.findByRole('status')).toHaveTextContent('反馈已更新');
  });

  it('shows a polished empty feedback state without treating empty data as an error', async () => {
    const user = userEvent.setup();
    const adminApi = createAdminApi({
      listFeedback: vi.fn(async () => ({ items: [] })),
    });

    render(<AdminConsole adminApi={adminApi} />);

    await user.click(await screen.findByRole('button', { name: 'Feedback' }));

    await waitFor(() => expect(adminApi.listFeedback).toHaveBeenCalledWith({ status: 'new' }));
    expect(screen.getByLabelText('Feedback status')).toHaveClass('admin-select-control');
    const emptyState = await screen.findByRole('region', { name: 'Empty feedback state' });
    expect(within(emptyState).getByText('No feedback yet')).toBeInTheDocument();
    expect(within(emptyState).getByText('没有反馈')).toBeInTheDocument();
    expect(screen.queryByText('Could not load feedback')).not.toBeInTheDocument();
  });

  it('opens feedback directly for the /admin/feedback SPA route', async () => {
    window.history.pushState({}, '', '/admin/feedback');
    const adminApi = createAdminApi();

    render(<AdminConsole adminApi={adminApi} />);

    expect(await screen.findByRole('heading', { name: 'Feedback' })).toBeInTheDocument();
    await waitFor(() => expect(adminApi.listFeedback).toHaveBeenCalledWith({ status: 'new' }));
    window.history.pushState({}, '', '/');
  });

  it('does not render user mutation or impersonated send controls', async () => {
    render(<AdminConsole adminApi={createAdminApi()} />);

    await screen.findByRole('heading', { name: 'AgenticIM Management' });
    expect(screen.queryByRole('button', { name: /edit user/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /delete friend/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /send message/i })).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/message composer/i)).not.toBeInTheDocument();
  });

  it('shows empty and error states', async () => {
    const user = userEvent.setup();
    const adminApi = createAdminApi({
      searchUsers: vi.fn(async () => ({ users: [] })),
      getConversationMessages: vi.fn(async () => {
        throw new Error('conversation not found');
      }),
    });

    render(<AdminConsole adminApi={adminApi} />);

    await user.click(await screen.findByRole('button', { name: 'Users' }));
    await user.type(screen.getByLabelText('User query'), 'missing');
    await user.click(screen.getByRole('button', { name: 'Search users' }));
    expect(await screen.findByText('No users found')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Conversation' }));
    await user.type(screen.getByLabelText('Conversation ID'), 'missing-conversation');
    await user.click(screen.getByRole('button', { name: 'Load conversation' }));
    expect(await screen.findByText('Could not load conversation')).toBeInTheDocument();
  });
});
