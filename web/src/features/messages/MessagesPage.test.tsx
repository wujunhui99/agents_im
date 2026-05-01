import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { MessagesPage } from './MessagesPage';
import type { MessageApi, ServerMessage } from '../../api/messages';
import type { UserApi, UserProfile, UserProfilePatch } from '../../api/user';

const bobProfile: UserProfile = {
  user_id: 'usr_000002',
  identifier: 'bob_002',
  display_name: 'Bob Lin',
  name: 'Bob Lin',
  gender: '',
  age: 0,
  region: '',
};

function createMessageApi(states = [{ conversationId: 'single:usr_000001:usr_000002', maxSeq: 1, hasReadSeq: 0, unreadCount: 1 }]) {
  const api: MessageApi = {
    sendMessage: vi.fn(async (request) => ({
      deduplicated: false,
      message: {
        serverMsgId: 'srv_000001',
        clientMsgId: request.clientMsgId,
        conversationId: request.chatType === 'group' ? `group:${request.groupId}` : `single:usr_000001:${request.receiverId}`,
        seq: 1,
        senderId: 'usr_000001',
        receiverId: request.chatType === 'single' ? request.receiverId : undefined,
        groupId: request.chatType === 'group' ? request.groupId : undefined,
        chatType: request.chatType,
        contentType: request.contentType,
        content: request.content,
        sendTime: 1777464300000,
        createdAt: 1777464300000,
      },
    })),
    getConversationSeqs: vi.fn(async () => ({
      conversations: states,
    })),
    pullMessages: vi.fn(async (conversationId) => {
      const messages: ServerMessage[] = [
        {
          serverMsgId: 'srv_seed_1',
          clientMsgId: 'client_seed_1',
          conversationId,
          seq: 1,
          senderId: 'usr_000002',
          receiverId: 'usr_000001',
          chatType: 'single',
          contentType: 'text',
          content: '真实后端会话消息',
          sendTime: 1777464000000,
          createdAt: 1777464000000,
        },
      ];

      return { conversationId, messages };
    }),
    markRead: vi.fn(async (conversationId, request) => ({ conversationId, hasReadSeq: request.hasReadSeq })),
  };

  return api;
}

function createUserApi(profile: UserProfile = bobProfile): UserApi {
  return {
    getCurrentUser: vi.fn(async () => profile),
    patchCurrentUser: vi.fn(async (patch: UserProfilePatch) => ({ ...profile, ...patch })),
    identifierExists: vi.fn(async (identifier) => ({ identifier, exists: true })),
    getPublicProfileByIdentifier: vi.fn(async () => profile),
  };
}

describe('MessagesPage real API mode', () => {
  it('loads conversations from the message API and sends through POST /messages', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi();

    render(<MessagesPage currentUserId="usr_000001" messageApi={messageApi} />);

    expect(await screen.findByText('真实后端会话消息')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /usr_000002/ }));
    await user.type(screen.getByRole('textbox', { name: '输入消息' }), '你好 Bob');
    await user.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(messageApi.sendMessage).toHaveBeenCalled());
    expect(messageApi.sendMessage).toHaveBeenCalledWith(
      expect.objectContaining({
        receiverId: 'usr_000002',
        chatType: 'single',
        contentType: 'text',
        content: '你好 Bob',
      }),
    );
    const log = screen.getByRole('log', { name: '聊天消息' });
    expect(within(log).getByText('已发送')).toBeInTheDocument();
  });

  it('exposes a useful start-chat action when there are no conversations', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi([]);

    render(<MessagesPage currentUserId="usr_000001" messageApi={messageApi} userApi={createUserApi()} />);

    expect(await screen.findByText('暂无会话')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '发起聊天' }));

    expect(screen.getByRole('region', { name: '发起聊天' })).toBeInTheDocument();
    expect(screen.getByLabelText('按 identifier 搜索聊天对象')).toBeInTheDocument();
  });

  it('starts a single chat by identifier, keeps the friendly title, and sends the first message through the real adapter', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi([]);
    const userApi = createUserApi();

    render(
      <MessagesPage
        currentUserId="usr_000001"
        messageApi={messageApi}
        userApi={userApi}
        startChatSignal={1}
      />,
    );

    await user.type(await screen.findByLabelText('按 identifier 搜索聊天对象'), 'bob_002');
    await user.click(screen.getByRole('button', { name: '搜索聊天对象' }));

    expect(await screen.findByText('Bob Lin')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '发起聊天 Bob Lin' }));

    expect(screen.getByRole('heading', { name: 'Bob Lin' })).toBeInTheDocument();
    expect(messageApi.sendMessage).not.toHaveBeenCalled();

    await user.type(screen.getByRole('textbox', { name: '输入消息' }), '第一条消息');
    await user.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(messageApi.sendMessage).toHaveBeenCalledTimes(1));
    expect(messageApi.sendMessage).toHaveBeenCalledWith(
      expect.objectContaining({
        receiverId: 'usr_000002',
        chatType: 'single',
        contentType: 'text',
        content: '第一条消息',
      }),
    );
    expect(await screen.findByText('已发送')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Bob Lin' })).toBeInTheDocument();
  });

  it('uses the identifier as the friendly conversation title when display name is unavailable', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi([]);
    const userApi = createUserApi({ ...bobProfile, display_name: '', name: '' });

    render(<MessagesPage currentUserId="usr_000001" messageApi={messageApi} userApi={userApi} />);

    await user.click(await screen.findByRole('button', { name: '发起聊天' }));
    await user.type(screen.getByLabelText('按 identifier 搜索聊天对象'), 'bob_002');
    await user.click(screen.getByRole('button', { name: '搜索聊天对象' }));
    await user.click(await screen.findByRole('button', { name: '发起聊天 bob_002' }));

    expect(screen.getByRole('heading', { name: 'bob_002' })).toBeInTheDocument();
  });
});
