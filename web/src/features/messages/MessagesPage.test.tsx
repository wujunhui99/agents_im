import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { MessagesPage } from './MessagesPage';
import type { MessageApi, SendMessageRequest, SendMessageResponse, ServerMessage } from '../../api/messages';
import type { UserApi, UserProfile, UserProfilePatch } from '../../api/user';

const conversationId = 'single:usr_000001:usr_000002';
const currentUserId = 'usr_000001';
const peerUserId = 'usr_000002';

const bobProfile: UserProfile = {
  user_id: peerUserId,
  identifier: 'bob_002',
  display_name: 'Bob Lin',
  name: 'Bob Lin',
  gender: '',
  age: 0,
  region: '',
};

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

function createMessageApi(
  messages: ServerMessage[] = [serverMessage({ seq: 1, content: '真实后端会话消息' })],
  sendMessage?: MessageApi['sendMessage'],
): MessageApi {
  const send =
    sendMessage ??
    vi.fn(async (request: SendMessageRequest): Promise<SendMessageResponse> => ({
      deduplicated: false,
      message: serverMessage({
        serverMsgId: 'srv_000001',
        clientMsgId: request.clientMsgId,
        conversationId: request.chatType === 'group' ? `group:${request.groupId}` : `single:${currentUserId}:${request.receiverId}`,
        seq: Math.max(1, messages.length + 1),
        senderId: currentUserId,
        receiverId: request.chatType === 'single' ? request.receiverId : undefined,
        groupId: request.chatType === 'group' ? request.groupId : undefined,
        chatType: request.chatType,
        contentType: request.contentType,
        content: request.content,
        sendTime: 1777464300000,
        createdAt: 1777464300000,
      }),
    }));

  return {
    sendMessage: send,
    getConversationSeqs: vi.fn(async () => ({
      conversations:
        messages.length > 0
          ? [
              {
                conversationId,
                maxSeq: messages.length,
                hasReadSeq: 0,
                unreadCount: messages.length,
                maxSeqTime: messages[messages.length - 1]?.sendTime,
                lastMessage: messages[messages.length - 1],
              },
            ]
          : [],
    })),
    pullMessages: vi.fn(async (nextConversationId) => ({ conversationId: nextConversationId, messages })),
    markRead: vi.fn(async (nextConversationId, request) => ({ conversationId: nextConversationId, hasReadSeq: request.hasReadSeq })),
  };
}

function createUserApi(profile: UserProfile = bobProfile): UserApi {
  return {
    getCurrentUser: vi.fn(async () => profile),
    patchCurrentUser: vi.fn(async (patch: UserProfilePatch) => ({ ...profile, ...patch })),
    identifierExists: vi.fn(async (identifier) => ({ identifier, exists: true })),
    getPublicProfileByIdentifier: vi.fn(async () => profile),
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
    expect(await screen.findByText('AI/Agent')).toBeInTheDocument();
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

  it('exposes a useful start-chat action when there are no conversations', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi([]);

    render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} userApi={createUserApi()} />);

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
      <MessagesPage currentUserId={currentUserId} messageApi={messageApi} userApi={userApi} startChatSignal={1} />,
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
        receiverId: peerUserId,
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

    render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} userApi={userApi} />);

    await user.click(await screen.findByRole('button', { name: '发起聊天' }));
    await user.type(screen.getByLabelText('按 identifier 搜索聊天对象'), 'bob_002');
    await user.click(screen.getByRole('button', { name: '搜索聊天对象' }));
    await user.click(await screen.findByRole('button', { name: '发起聊天 bob_002' }));

    expect(screen.getByRole('heading', { name: 'bob_002' })).toBeInTheDocument();
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
