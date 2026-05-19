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
      maxSeqTime: 1777464000000,
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
    ...overrides,
  };
}

describe('AdminConsole', () => {
  it('renders dashboard overview cards, trace table, and navigation', async () => {
    render(<AdminConsole adminApi={createAdminApi()} />);

    expect(screen.getByText('Loading admin dashboard')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Admin Console' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Dashboard' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'LLM Traces' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Conversation' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Users' })).toBeInTheDocument();
    expect(screen.getByText('Users')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('Failed AI runs')).toBeInTheDocument();
    expect(screen.getByText('trace_admin_1')).toBeInTheDocument();
  });

  it('loads messages after entering a conversation id', async () => {
    const user = userEvent.setup();
    const adminApi = createAdminApi();

    render(<AdminConsole adminApi={adminApi} />);

    await user.click(await screen.findByRole('button', { name: 'Conversation' }));
    await user.type(screen.getByLabelText('Conversation ID'), 'single:1001:2002');
    await user.click(screen.getByRole('button', { name: 'Load conversation' }));

    expect(await screen.findByText('hello from admin inspector')).toBeInTheDocument();
    expect(screen.getByText('AI response for inspector')).toBeInTheDocument();
    expect(screen.getByText('seq 2')).toBeInTheDocument();
    expect(adminApi.getConversationMessages).toHaveBeenCalledWith('single:1001:2002');
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
    expect(screen.getByText('AI response for inspector')).toBeInTheDocument();
  });

  it('does not render user mutation or impersonated send controls', async () => {
    render(<AdminConsole adminApi={createAdminApi()} />);

    await screen.findByRole('heading', { name: 'Admin Console' });
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
