import { render, screen, waitFor, within, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { MessagesPage } from './MessagesPage';
import type { ContactsApi } from '../../api/contacts';
import type { MessageApi, SendMessageRequest, SendMessageResponse, ServerMessage } from '../../api/messages';
import type { WebSocketFactory, WebSocketLike } from '../../api/websocketClient';
import type { UserApi, UserProfile, UserProfilePatch } from '../../api/user';

const conversationId = 'single:1001:2002';
const currentUserId = '1001';
const peerUserId = '2002';

const bobProfile: UserProfile = {
  user_id: peerUserId,
  identifier: 'bob_002',
  display_name: 'Bob Lin',
  name: 'Bob Lin',
  gender: '',
  birth_date: '',
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
  options: { hasReadSeq?: number; unreadCount?: number } = {},
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
                hasReadSeq: options.hasReadSeq ?? 0,
                unreadCount: options.unreadCount ?? messages.length,
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

function createContactsApi(): ContactsApi {
  return {
    listFriends: vi.fn(async () => ({ friends: [] })),
    addFriend: vi.fn(),
    deleteFriend: vi.fn(),
    listFriendRequests: vi.fn(async () => ({ incoming: [], outgoing: [] })),
    acceptFriendRequest: vi.fn(),
    rejectFriendRequest: vi.fn(),
  };
}

class FakeWebSocket implements WebSocketLike {
  readyState = 0;
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent<string>) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  sent: string[] = [];

  send(data: string) {
    this.sent.push(data);
  }

  close() {
    this.readyState = 3;
  }

  open() {
    this.readyState = 1;
    this.onopen?.(new Event('open'));
  }

  receive(payload: unknown) {
    this.onmessage?.({ data: JSON.stringify(payload) } as MessageEvent<string>);
  }
}

function createFakeWebSocketFactory() {
  const sockets: FakeWebSocket[] = [];
  const factory: WebSocketFactory = () => {
    const socket = new FakeWebSocket();
    sockets.push(socket);
    return socket;
  };
  return { sockets, factory };
}

function messageReceivedEvent(message: Partial<ServerMessage> & Pick<ServerMessage, 'serverMsgId' | 'seq' | 'content'>) {
  return {
    type: 'message_received',
    data: {
      client_msg_id: message.clientMsgId ?? `client_${message.seq}`,
      server_msg_id: message.serverMsgId,
      conversation_id: message.conversationId ?? conversationId,
      seq: message.seq,
      sender_id: message.senderId ?? peerUserId,
      receiver_id: message.receiverId ?? currentUserId,
      chat_type: message.chatType ?? 'single',
      content_type: message.contentType ?? 'text',
      content: message.content,
      send_time: message.sendTime ?? 1777464300000,
      created_at: message.createdAt ?? 1777464300000,
    },
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
  await userEvent.click(await screen.findByRole('button', { name: /未知联系人/ }));
  return screen.findByRole('log', { name: '聊天消息' });
}

function expectTextOrder(container: HTMLElement, labels: string[]) {
  const nodes = labels.map((label) => within(container).getByText(label));
  for (let index = 0; index < nodes.length - 1; index++) {
    expect(nodes[index].compareDocumentPosition(nodes[index + 1]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  }
}

describe('MessagesPage real API mode', () => {
  it('receives live websocket message_received events without requiring a manual refresh', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi([]);
    const contactsApi = createContactsApi();
    const { sockets, factory } = createFakeWebSocketFactory();

    render(
      <MessagesPage
        currentUserId={currentUserId}
        messageApi={messageApi}
        contactsApi={contactsApi}
        webSocketFactory={factory}
        webSocketUrl="ws://127.0.0.1/ws"
        webSocketToken="test-token"
      />,
    );

    await waitFor(() => expect(sockets).toHaveLength(1));
    act(() => {
      sockets[0].open();
      sockets[0].receive(messageReceivedEvent({ serverMsgId: 'srv_live_1', seq: 1, content: 'live hello from Bob' }));
    });

    const row = await screen.findByRole('button', { name: /live hello from Bob/ });
    expect(within(row).getByText('live hello from Bob')).toBeInTheDocument();
    await user.click(row);
    expect(await screen.findByText('live hello from Bob')).toBeInTheDocument();
    expect(messageApi.getConversationSeqs).toHaveBeenCalledTimes(1);
  });

  it('renders JSON text payloads from live websocket message_received events as plain text', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi([]);
    const { sockets, factory } = createFakeWebSocketFactory();

    render(
      <MessagesPage
        currentUserId={currentUserId}
        messageApi={messageApi}
        contactsApi={createContactsApi()}
        webSocketFactory={factory}
        webSocketUrl="ws://127.0.0.1/ws"
        webSocketToken="test-token"
      />,
    );

    await waitFor(() => expect(sockets).toHaveLength(1));
    act(() => {
      sockets[0].open();
      sockets[0].receive(messageReceivedEvent({ serverMsgId: 'srv_live_json_text', seq: 1, content: '{"text":"1"}' }));
    });

    const row = await screen.findByRole('button', { name: /未知联系人/ });
    expect(within(row).getByText('1', { selector: 'p' })).toBeInTheDocument();
    expect(within(row).queryByText('{"text":"1"}')).not.toBeInTheDocument();

    await user.click(row);
    const log = await screen.findByRole('log', { name: '聊天消息' });
    expect(within(log).getByText('1')).toBeInTheDocument();
    expect(within(log).queryByText('{"text":"1"}')).not.toBeInTheDocument();
  });

  it('keeps the peer as the send target after an incoming live message in an open single chat', async () => {
    const user = userEvent.setup();
    const sendMessage = vi.fn(async (request: SendMessageRequest): Promise<SendMessageResponse> => ({
      deduplicated: false,
      message: serverMessage({
        serverMsgId: 'srv_reverse_reply',
        clientMsgId: request.clientMsgId,
        seq: 3,
        senderId: currentUserId,
        receiverId: request.chatType === 'single' ? request.receiverId : undefined,
        groupId: request.chatType === 'group' ? request.groupId : undefined,
        chatType: request.chatType,
        content: request.content,
        sendTime: 1777464500000,
        createdAt: 1777464500000,
      }),
    }));
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: 'seed existing conversation' })], sendMessage);
    const { sockets, factory } = createFakeWebSocketFactory();

    render(
      <MessagesPage
        currentUserId={currentUserId}
        messageApi={messageApi}
        contactsApi={createContactsApi()}
        webSocketFactory={factory}
        webSocketUrl="ws://127.0.0.1/ws"
        webSocketToken="test-token"
      />,
    );

    await waitFor(() => expect(sockets).toHaveLength(1));
    await user.click(await screen.findByRole('button', { name: /未知联系人/ }));
    act(() => {
      sockets[0].open();
      sockets[0].receive(messageReceivedEvent({ serverMsgId: 'srv_live_before_reply', seq: 2, content: 'live message before reply' }));
    });
    expect(await screen.findByText('live message before reply')).toBeInTheDocument();

    await user.type(screen.getByRole('textbox', { name: '输入消息' }), 'reply from same open chat');
    await user.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(sendMessage).toHaveBeenCalledTimes(1));
    expect(sendMessage).toHaveBeenCalledWith(
      expect.objectContaining({
        receiverId: peerUserId,
        chatType: 'single',
        contentType: 'text',
        content: 'reply from same open chat',
      }),
    );
    expect(sendMessage).not.toHaveBeenCalledWith(expect.objectContaining({ receiverId: currentUserId }));
  });

  it('uses an unknown label instead of an internal id when conversation profiles are unavailable', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: '来自历史会话' })]);

    render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} />);

    const row = await screen.findByRole('button', { name: /未知联系人/ });
    expect(within(row).getByText('未知联系人')).toBeInTheDocument();
    expect(screen.queryByText(peerUserId)).not.toBeInTheDocument();

    await user.click(row);

    expect(await screen.findByRole('heading', { name: '未知联系人' })).toBeInTheDocument();
    expect(screen.queryByText(peerUserId)).not.toBeInTheDocument();
  });

  it('shows start-chat profile labels without exposing the internal profile id', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi([]);

    render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} userApi={createUserApi()} />);

    await user.click(await screen.findByRole('button', { name: '发起聊天' }));
    await user.type(screen.getByLabelText('按账号搜索聊天对象'), 'bob_002');
    await user.click(screen.getByRole('button', { name: '搜索聊天对象' }));

    const startChatRegion = screen.getByRole('region', { name: '发起聊天' });
    expect(await within(startChatRegion).findByText('Bob Lin')).toBeInTheDocument();
    expect(within(startChatRegion).getByText('bob_002')).toBeInTheDocument();
    expect(within(startChatRegion).queryByText(peerUserId)).not.toBeInTheDocument();
  });

  it('marks visible unread messages as read when a conversation is opened', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi([
      serverMessage({ seq: 1, content: 'older unread' }),
      serverMessage({ seq: 3, content: 'newest unread' }),
    ]);

    render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} />);

    expect(await screen.findByText('2')).toBeInTheDocument();
    await user.click(await screen.findByRole('button', { name: /未知联系人/ }));

    await waitFor(() => expect(messageApi.markRead).toHaveBeenCalledWith(conversationId, { hasReadSeq: 3 }));
    await user.click(screen.getByRole('button', { name: '返回消息列表' }));

    const row = await screen.findByRole('button', { name: /未知联系人/ });
    expect(within(row).queryByText('2')).not.toBeInTheDocument();
  });

  it('keeps mark-read failures visible instead of faking success', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: 'unread that fails read ack' })]);
    vi.mocked(messageApi.markRead).mockRejectedValueOnce(new Error('mark read failed'));

    render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} />);

    await user.click(await screen.findByRole('button', { name: /未知联系人/ }));

    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('mark read failed'));
  });

  it('keeps a sent preview and cleared unread state when a stale conversation reload merges afterward', async () => {
    const user = userEvent.setup();
    const seqs = deferred<Awaited<ReturnType<MessageApi['getConversationSeqs']>>>();
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: 'stale server preview' })]);
    vi.mocked(messageApi.getConversationSeqs).mockReturnValueOnce(seqs.promise);

    render(
      <MessagesPage
        currentUserId={currentUserId}
        messageApi={messageApi}
        pendingChatProfile={bobProfile}
        onPendingChatConsumed={vi.fn()}
      />,
    );

    expect(await screen.findByRole('heading', { name: 'Bob Lin' })).toBeInTheDocument();
    await user.type(screen.getByRole('textbox', { name: '输入消息' }), 'fresh outgoing');
    await user.click(screen.getByRole('button', { name: '发送' }));
    await waitFor(() => expect(screen.getByLabelText('发送成功')).toHaveTextContent('✔'));

    seqs.resolve({
      conversations: [
        {
          conversationId,
          maxSeq: 1,
          hasReadSeq: 0,
          unreadCount: 1,
          maxSeqTime: 1777464000000,
          lastMessage: serverMessage({ seq: 1, content: 'stale server preview' }),
        },
      ],
    });

    await waitFor(() => expect(messageApi.pullMessages).toHaveBeenCalledWith(conversationId, expect.anything()));
    await waitFor(() => expect(screen.getByRole('article', { name: '收到的消息：stale server preview' })).toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: '返回消息列表' }));

    const row = await screen.findByRole('button', { name: /Bob Lin|未知联系人/ });
    expect(within(row).getByText('fresh outgoing')).toBeInTheDocument();
    expect(within(row).queryByText('1')).not.toBeInTheDocument();
  });

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
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: '真实后端会话消息', messageOrigin: 'ai' })], sendMessage);

    render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} />);

    expect(await screen.findByText('真实后端会话消息')).toBeInTheDocument();
    expect(await screen.findByText('AI/Agent')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /未知联系人/ }));
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
    const outgoingMessage = within(log).getByRole('article', { name: '我发送的消息：你好 Bob' });
    const sentStatus = within(outgoingMessage).getByLabelText('发送成功');
    expect(sentStatus.textContent).toBe('✔');
    expect(within(outgoingMessage).queryByText('已发送')).not.toBeInTheDocument();
  });

  it('renders a double checkmark when an outgoing sent message is covered by the read threshold', async () => {
    const messageApi = createMessageApi(
      [
        serverMessage({
          seq: 1,
          content: 'outgoing covered by read seq',
          senderId: currentUserId,
          receiverId: peerUserId,
        }),
      ],
      undefined,
      { hasReadSeq: 1, unreadCount: 0 },
    );

    const log = await openSeededConversation(messageApi);
    const outgoingMessage = within(log).getByRole('article', { name: '我发送的消息：outgoing covered by read seq' });
    const readStatus = within(outgoingMessage).getByLabelText('对方已读');

    expect(readStatus.textContent).toBe('✔✔');
  });

  it('keeps pending and failed outgoing message states understandable', async () => {
    const user = userEvent.setup();
    const sendDeferred = deferred<SendMessageResponse>();
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: 'seed before failed send' })], () => sendDeferred.promise);
    const log = await openSeededConversation(messageApi);

    await user.type(screen.getByRole('textbox', { name: '输入消息' }), 'message that will fail');
    await user.click(screen.getByRole('button', { name: '发送' }));

    expect(within(log).getByText('发送中')).toBeInTheDocument();
    sendDeferred.reject(new Error('send failed'));

    await waitFor(() => expect(within(log).getByText('发送失败')).toBeInTheDocument());
  });

  it('does not render outgoing checkmarks for incoming messages', async () => {
    const log = await openSeededConversation(createMessageApi([serverMessage({ seq: 1, content: 'incoming without checkmarks' })]));
    const incomingMessage = within(log).getByRole('article', { name: '收到的消息：incoming without checkmarks' });

    expect(within(incomingMessage).queryByLabelText('发送成功')).not.toBeInTheDocument();
    expect(within(incomingMessage).queryByLabelText('对方已读')).not.toBeInTheDocument();
    expect(within(incomingMessage).queryByText('✔')).not.toBeInTheDocument();
  });

  it('exposes a useful start-chat action when there are no conversations', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi([]);

    render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} userApi={createUserApi()} />);

    expect(await screen.findByText('暂无会话')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '发起聊天' }));

    expect(screen.getByRole('region', { name: '发起聊天' })).toBeInTheDocument();
    expect(screen.getByLabelText('按账号搜索聊天对象')).toBeInTheDocument();
  });

  it('starts a single chat by identifier, keeps the friendly title, and sends the first message through the real adapter', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi([]);
    const userApi = createUserApi();

    render(
      <MessagesPage currentUserId={currentUserId} messageApi={messageApi} userApi={userApi} startChatSignal={1} />,
    );

    await user.type(await screen.findByLabelText('按账号搜索聊天对象'), 'bob_002');
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
    expect(await screen.findByLabelText('发送成功')).toHaveTextContent('✔');
    expect(screen.getByRole('heading', { name: 'Bob Lin' })).toBeInTheDocument();
  });

  it('uses the identifier as the friendly conversation title when display name is unavailable', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi([]);
    const userApi = createUserApi({ ...bobProfile, display_name: '', name: '' });

    render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} userApi={userApi} />);

    await user.click(await screen.findByRole('button', { name: '发起聊天' }));
    await user.type(screen.getByLabelText('按账号搜索聊天对象'), 'bob_002');
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

    await waitFor(() => expect(within(log).getByLabelText('发送成功')).toHaveTextContent('✔'));
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

    await waitFor(() => expect(within(log).getAllByLabelText('发送成功')).toHaveLength(2));
    expectTextOrder(log, ['seq one', 'first outgoing', 'second outgoing']);
  });
});
