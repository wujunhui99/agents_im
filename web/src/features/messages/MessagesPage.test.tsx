import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { MessagesPage } from './MessagesPage';
import type { MessageApi, ServerMessage } from '../../api/messages';

const messageApi: MessageApi = {
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
    conversations: [
      { conversationId: 'single:usr_000001:usr_000002', maxSeq: 1, hasReadSeq: 0, unreadCount: 1 },
    ],
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

describe('MessagesPage real API mode', () => {
  it('loads conversations from the message API and sends through POST /messages', async () => {
    const user = userEvent.setup();

    render(
      <MessagesPage
        currentUserId="usr_000001"
        messageApi={messageApi}
      />,
    );

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
});
