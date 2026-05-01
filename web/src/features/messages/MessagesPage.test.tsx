import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { MessagesPage } from './MessagesPage';
import type { MessageApi, SendMessageRequest, SendMessageResponse, ServerMessage } from '../../api/messages';

const conversationId = 'single:usr_000001:usr_000002';
const currentUserId = 'usr_000001';
const peerUserId = 'usr_000002';

function serverMessage(overrides: Partial<ServerMessage> & Pick<ServerMessage, 'seq' | 'content'>): ServerMessage {
  const { seq, content, ...rest } = overrides;
  return {
    serverMsgId: `srv_${seq}`,
    clientMsgId: `client_${seq}`,
    conversationId,
    seq,
    senderId: peerUserId,
    receiverId: currentUserId,
    groupId: '',
    chatType: 'single',
    contentType: 'text',
    content,
    sendTime: 1777464000000 + seq * 1000,
    createdAt: 1777464000000 + seq * 1000,
    ...rest,
  };
}

function createMessageApi(messages: ServerMessage[], sendMessage = vi.fn()): MessageApi {
  return {
    sendMessage,
    getConversationSeqs: vi.fn(async () => ({
      states: [
        {
          conversationId,
          maxSeq: messages.length,
          hasReadSeq: 0,
          unreadCount: messages.length,
          maxSeqTime: messages[messages.length - 1]?.sendTime,
          lastMessage: messages[messages.length - 1],
        },
      ],
    })),
    pullMessages: vi.fn(async () => ({ conversationId, messages })),
    markRead: vi.fn(async (nextConversationId, request) => ({ conversationId: nextConversationId, hasReadSeq: request.hasReadSeq })),
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((promiseResolve, promiseReject) => {
    resolve = promiseResolve;
    reject = promiseReject;
  });
  return { promise, resolve, reject };
}

async function openSeededConversation(messageApi: MessageApi) {
  render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} />);
  await userEvent.click(await screen.findByRole('button', { name: /usr_000002/ }));
  return screen.findByRole('log', { name: '聊天消息' });
}

function expectTextOrder(container: HTMLElement, labels: string[]) {
  const nodes = labels.map((label) => within(container).getByText(label));
  for (let index = 0; index < nodes.length - 1; index++) {
    expect(nodes[index].compareDocumentPosition(nodes[index + 1]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  }
}

describe('MessagesPage real API mode', () => {
  it('loads conversations from the message API and sends through POST /messages', async () => {
    const user = userEvent.setup();
    const sendMessage = vi.fn(async (request: SendMessageRequest): Promise<SendMessageResponse> => ({
      deduplicated: false,
      message: serverMessage({
        serverMsgId: 'srv_000001',
        clientMsgId: request.clientMsgId,
        seq: 2,
        senderId: currentUserId,
        receiverId: request.chatType === 'single' ? request.receiverId : undefined,
        groupId: request.chatType === 'group' ? request.groupId : undefined,
        chatType: request.chatType,
        content: request.content,
        sendTime: 1777464300000,
        createdAt: 1777464300000,
      }),
    }));
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: '真实后端会话消息' })], sendMessage);

    render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} />);

    expect(await screen.findByText('真实后端会话消息')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /usr_000002/ }));
    await user.type(screen.getByRole('textbox', { name: '输入消息' }), '你好 Bob');
    await user.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(sendMessage).toHaveBeenCalled());
    expect(sendMessage).toHaveBeenCalledWith(
      expect.objectContaining({
        receiverId: peerUserId,
        chatType: 'single',
        contentType: 'text',
        content: '你好 Bob',
      }),
    );
    const log = screen.getByRole('log', { name: '聊天消息' });
    expect(within(log).getByText('已发送')).toBeInTheDocument();
  });

  it('renders shuffled server messages by authoritative seq instead of arrival or send time', async () => {
    const messages = [
      serverMessage({ seq: 3, content: 'seq three', sendTime: 1777464000000 }),
      serverMessage({ seq: 1, content: 'seq one', sendTime: 1777464300000 }),
      serverMessage({ seq: 2, content: 'seq two', sendTime: 1777464100000 }),
    ];
    const log = await openSeededConversation(createMessageApi(messages));

    expectTextOrder(log, ['seq one', 'seq two', 'seq three']);
  });

  it('replaces an optimistic pending message with canonical server fields and repositions by seq', async () => {
    const user = userEvent.setup();
    const sendDeferred = deferred<SendMessageResponse>();
    let capturedRequest: SendMessageRequest | undefined;
    const sendMessage = vi.fn((request: SendMessageRequest) => {
      capturedRequest = request;
      return sendDeferred.promise;
    });
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: 'seq one', sendTime: 1777464300000 })], sendMessage);
    const log = await openSeededConversation(messageApi);

    await user.type(screen.getByRole('textbox', { name: '输入消息' }), 'pending becomes seq two');
    await user.click(screen.getByRole('button', { name: '发送' }));

    expect(within(log).getByText('发送中')).toBeInTheDocument();
    sendDeferred.resolve({
      deduplicated: false,
      message: serverMessage({
        serverMsgId: 'srv_confirmed_2',
        clientMsgId: capturedRequest?.clientMsgId ?? '',
        seq: 2,
        senderId: currentUserId,
        receiverId: peerUserId,
        content: 'pending becomes seq two',
        sendTime: 1777463000000,
        createdAt: 1777463000000,
      }),
    });

    await waitFor(() => expect(within(log).getByText('已发送')).toBeInTheDocument());
    expect(within(log).queryByText('发送中')).not.toBeInTheDocument();
    expect(within(log).getAllByText('pending becomes seq two')).toHaveLength(1);
    expectTextOrder(log, ['seq one', 'pending becomes seq two']);
  });

  it('allows only one in-flight send per conversation and keeps final display in seq order', async () => {
    const user = userEvent.setup();
    const sends = [deferred<SendMessageResponse>(), deferred<SendMessageResponse>()];
    const requests: SendMessageRequest[] = [];
    const sendMessage = vi.fn((request: SendMessageRequest) => {
      requests.push(request);
      return sends[requests.length - 1].promise;
    });
    const log = await openSeededConversation(
      createMessageApi([serverMessage({ seq: 1, content: 'seq one', sendTime: 1777464300000 })], sendMessage),
    );

    await user.type(screen.getByRole('textbox', { name: '输入消息' }), 'first outgoing');
    await user.click(screen.getByRole('button', { name: '发送' }));

    expect(await screen.findByRole('button', { name: '发送中' })).toBeDisabled();
    expect(screen.getByRole('textbox', { name: '输入消息' })).toBeDisabled();
    await user.click(screen.getByRole('button', { name: '发送中' }));
    expect(sendMessage).toHaveBeenCalledTimes(1);

    sends[0].resolve({
      deduplicated: false,
      message: serverMessage({
        serverMsgId: 'srv_outgoing_2',
        clientMsgId: requests[0].clientMsgId,
        seq: 2,
        senderId: currentUserId,
        receiverId: peerUserId,
        content: 'first outgoing',
        sendTime: 1777462000000,
        createdAt: 1777462000000,
      }),
    });
    await waitFor(() => expect(screen.getByRole('textbox', { name: '输入消息' })).toBeEnabled());
    expect(screen.getByRole('button', { name: '发送' })).toBeDisabled();

    await user.type(screen.getByRole('textbox', { name: '输入消息' }), 'second outgoing');
    await user.click(screen.getByRole('button', { name: '发送' }));
    expect(sendMessage).toHaveBeenCalledTimes(2);
    sends[1].resolve({
      deduplicated: false,
      message: serverMessage({
        serverMsgId: 'srv_outgoing_3',
        clientMsgId: requests[1].clientMsgId,
        seq: 3,
        senderId: currentUserId,
        receiverId: peerUserId,
        content: 'second outgoing',
        sendTime: 1777461000000,
        createdAt: 1777461000000,
      }),
    });

    await waitFor(() => expect(within(log).getAllByText('已发送')).toHaveLength(2));
    expectTextOrder(log, ['seq one', 'first outgoing', 'second outgoing']);
  });
});
