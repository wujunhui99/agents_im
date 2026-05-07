import { render, screen, waitFor, within, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi, type Mock } from 'vitest';
import { MessagesPage } from './MessagesPage';
import type { ContactsApi } from '../../api/contacts';
import type { Group, GroupMember, GroupsApi } from '../../api/groups';
import type { CompleteMediaUploadResponse, CreateMediaUploadRequest, CreateMediaUploadResponse, MediaApi } from '../../api/media';
import type { AIHostingState, MessageApi, SendMessageRequest, SendMessageResponse, ServerMessage } from '../../api/messages';
import type { WebSocketFactory, WebSocketLike } from '../../api/websocketClient';
import type { UserApi, UserProfile, UserProfilePatch } from '../../api/user';

const conversationId = 'single:1001:2002';
const currentUserId = '1001';
const peerUserId = '2002';
const groupId = 'grp_team';
const groupConversationId = `group:${groupId}`;

const bobProfile: UserProfile = {
  user_id: peerUserId,
  identifier: 'bob_002',
  display_name: 'Bob Lin',
  name: 'Bob Lin',
  gender: '',
  birth_date: '',
  region: '',
  avatar_media_id: 'med_bob_avatar',
  avatar_url: 'https://storage.test/avatar/bob.png',
  avatar_url_expires_at: 1777550400000,
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
    getAIHosting: vi.fn(async (nextConversationId) => ({
      conversationId: nextConversationId,
      chatType: 'single' as const,
      enabled: false,
      available: true,
      peerEnabled: false,
      unavailableReason: '',
      maxRecentMessages: 30,
      summaryEnabled: false,
    })),
    updateAIHosting: vi.fn(async (nextConversationId, request) => ({
      conversationId: nextConversationId,
      chatType: 'single' as const,
      enabled: request.enabled,
      available: true,
      peerEnabled: false,
      unavailableReason: '',
      maxRecentMessages: 30,
      summaryEnabled: false,
    })),
  };
}

function createUserApi(profile: UserProfile = bobProfile): UserApi {
  return {
    getCurrentUser: vi.fn(async () => profile),
    patchCurrentUser: vi.fn(async (patch: UserProfilePatch) => ({ ...profile, ...patch })),
    patchCurrentUserAvatar: vi.fn(async () => profile),
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

function createGroupsApi(overrides?: Partial<GroupsApi>): GroupsApi {
  const group: Group = {
    group_id: groupId,
    name: '项目群',
    description: '',
    creator_user_id: currentUserId,
    created_at: '2026-05-05T12:00:00Z',
    updated_at: '2026-05-05T12:00:00Z',
  };
  const members: GroupMember[] = [
    {
      group_id: groupId,
      user_id: currentUserId,
      display_name: 'Alice Chen',
      name: 'Alice Chen',
      identifier: 'alice_001',
      avatar_media_id: '',
      state: 'active',
      joined_at: '2026-05-05T12:00:00Z',
      left_at: '',
    },
    {
      group_id: groupId,
      user_id: peerUserId,
      display_name: 'Bob Lin',
      name: 'Bob Lin',
      identifier: 'bob_002',
      avatar_media_id: bobProfile.avatar_media_id,
      avatar_url: bobProfile.avatar_url,
      avatar_url_expires_at: bobProfile.avatar_url_expires_at,
      state: 'active',
      joined_at: '2026-05-05T12:00:00Z',
      left_at: '',
    },
  ];
  return {
    listGroups: vi.fn(async () => ({ groups: [group] })),
    getGroup: vi.fn(async () => group),
    createGroup: vi.fn(async () => group),
    joinGroup: vi.fn(),
    leaveGroup: vi.fn(),
    listMembers: vi.fn(async () => ({ group_id: groupId, members })),
    ...overrides,
  };
}

function groupServerMessage(overrides: Partial<ServerMessage> & Pick<ServerMessage, 'seq' | 'content'>): ServerMessage {
  return serverMessage({
    conversationId: groupConversationId,
    senderId: peerUserId,
    receiverId: '',
    groupId,
    chatType: 'group',
    ...overrides,
  });
}

function createContactsApiWithAcceptedPeerAvatar(): ContactsApi {
  const api = createContactsApi();
  api.listFriends = vi.fn(async () => ({
    friends: [
      {
        user_id: currentUserId,
        friend_id: peerUserId,
        status: 'accepted',
        is_friend: true,
        created_at: '2026-04-29T12:00:00Z',
        updated_at: '2026-04-29T12:00:00Z',
        friend: bobProfile,
      },
    ],
  }));
  return api;
}

type TestMediaApi = MediaApi & {
  createUploadIntent: Mock<(request: CreateMediaUploadRequest) => Promise<CreateMediaUploadResponse>>;
  completeUpload: Mock<(mediaId: string) => Promise<CompleteMediaUploadResponse>>;
  getDownloadURL: Mock<(mediaId: string) => Promise<{ mediaId: string; downloadUrl: string; expiresAt: number }>>;
};

function createMediaApi(overrides?: Partial<TestMediaApi>): TestMediaApi {
  const mediaApi: TestMediaApi = {
    createUploadIntent: vi.fn(async (request: CreateMediaUploadRequest): Promise<CreateMediaUploadResponse> => ({
      mediaId: request.purpose === 'message_image' ? 'med_image_1' : 'med_file_1',
      objectKey: `objects/${request.filename}`,
      uploadUrl: `https://storage.test/upload/${request.filename}`,
      expiresAt: 1777465000000,
    })),
    completeUpload: vi.fn(async (mediaId: string): Promise<CompleteMediaUploadResponse> => ({
      media: {
        mediaId,
        ownerUserId: currentUserId,
        bucket: 'agents-im-media',
        objectKey: `objects/${mediaId}`,
        sha256: '',
        contentType: mediaId === 'med_image_1' ? 'image/jpeg' : 'application/pdf',
        sizeBytes: 1024,
        originalFilename: mediaId === 'med_image_1' ? 'cat.jpg' : 'report.pdf',
        purpose: mediaId === 'med_image_1' ? 'message_image' : 'message_file',
        status: 'ready',
        createdAt: '2026-05-04T12:00:00Z',
        updatedAt: '2026-05-04T12:00:00Z',
      },
    })),
    getDownloadURL: vi.fn(async (mediaId: string) => ({
      mediaId,
      downloadUrl: `https://media.test/download/${mediaId}`,
      expiresAt: 1777465000000,
    })),
  };

  return { ...mediaApi, ...overrides };
}

function sizedFile(name: string, type: string, sizeBytes: number) {
  return new File([new Uint8Array(sizeBytes)], name, { type });
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

function localTimestamp(year: number, month: number, day: number, hour: number, minute: number) {
  return new Date(year, month - 1, day, hour, minute, 0, 0).getTime();
}

function expectedLocalDate(timestamp: number) {
  return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' }).format(new Date(timestamp));
}

function expectedLocalTime(timestamp: number) {
  return new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(timestamp));
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

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('MessagesPage real API mode', () => {
  it('loads persisted AI hosting state and updates the direct chat toggle', async () => {
    const user = userEvent.setup();
    const aiState: AIHostingState = {
      conversationId,
      chatType: 'single',
      enabled: true,
      available: true,
      peerEnabled: false,
      unavailableReason: '',
      maxRecentMessages: 30,
      summaryEnabled: false,
    };
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: '开启过托管的会话' })]);
    messageApi.getAIHosting = vi.fn(async () => aiState);
    messageApi.updateAIHosting = vi.fn(async (_nextConversationId, request) => ({ ...aiState, enabled: request.enabled }));

    render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} contactsApi={createContactsApiWithAcceptedPeerAvatar()} />);

    await user.click(await screen.findByRole('button', { name: /Bob Lin/ }));
    const hostingSwitch = await screen.findByRole('switch', { name: 'AI 托管' });
    expect(hostingSwitch).toBeChecked();
    expect(messageApi.getAIHosting).toHaveBeenCalledWith(conversationId);

    await user.click(hostingSwitch);
    await waitFor(() => expect(messageApi.updateAIHosting).toHaveBeenCalledWith(conversationId, { enabled: false }));
    expect(hostingSwitch).not.toBeChecked();
  });

  it('shows a Chinese peer-hosted reason and disables the AI hosting toggle', async () => {
    const aiState: AIHostingState = {
      conversationId,
      chatType: 'single',
      enabled: false,
      available: false,
      peerEnabled: true,
      unavailableReason: '对方已开启 AI 托管，本会话暂时只能由一方开启',
      maxRecentMessages: 30,
      summaryEnabled: false,
    };
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: '对方托管中' })]);
    messageApi.getAIHosting = vi.fn(async () => aiState);
    messageApi.updateAIHosting = vi.fn();

    render(
      <MessagesPage currentUserId={currentUserId} messageApi={messageApi} contactsApi={createContactsApiWithAcceptedPeerAvatar()} />,
    );

    await userEvent.click(await screen.findByRole('button', { name: /Bob Lin/ }));
    const hostingSwitch = await screen.findByRole('switch', { name: 'AI 托管' });
    expect(hostingSwitch).toBeDisabled();
    expect(hostingSwitch).not.toBeChecked();
    expect(screen.getByText('对方已开启 AI 托管，本会话暂时只能由一方开启')).toBeInTheDocument();
  });

  it('surfaces AI hosting load errors with retry and hides the control for groups', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: '托管状态加载失败' })]);
    messageApi.getAIHosting = vi.fn().mockRejectedValueOnce(new Error('AI 托管状态加载失败')).mockResolvedValueOnce({
      conversationId,
      chatType: 'single',
      enabled: false,
      available: true,
      peerEnabled: false,
      unavailableReason: '',
      maxRecentMessages: 30,
      summaryEnabled: false,
    } satisfies AIHostingState);
    messageApi.updateAIHosting = vi.fn();

    const { unmount } = render(
      <MessagesPage currentUserId={currentUserId} messageApi={messageApi} contactsApi={createContactsApiWithAcceptedPeerAvatar()} />,
    );

    await user.click(await screen.findByRole('button', { name: /Bob Lin/ }));
    expect(await screen.findByText('AI 托管状态加载失败')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '重试 AI 托管状态' }));
    await waitFor(() => expect(messageApi.getAIHosting).toHaveBeenCalledTimes(2));
    expect(await screen.findByRole('switch', { name: 'AI 托管' })).toBeEnabled();
    unmount();

    const groupMessage = groupServerMessage({ seq: 1, content: '群聊不显示托管' });
    vi.mocked(messageApi.getConversationSeqs).mockResolvedValueOnce({
      states: [
        {
          conversationId: groupConversationId,
          maxSeq: 1,
          hasReadSeq: 0,
          unreadCount: 1,
          maxSeqTime: groupMessage.sendTime,
          lastMessage: groupMessage,
        },
      ],
    });
    vi.mocked(messageApi.pullMessages).mockResolvedValueOnce({ conversationId: groupConversationId, messages: [groupMessage] });
    render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} groupsApi={createGroupsApi()} />);
    await user.click(await screen.findByRole('button', { name: /项目群/ }));
    expect(screen.queryByRole('switch', { name: 'AI 托管' })).not.toBeInTheDocument();
  });

  it('keeps the chat header and composer outside the scrollable message history region', async () => {
    const manyMessages = Array.from({ length: 80 }, (_, index) =>
      serverMessage({ seq: index + 1, content: `历史消息 ${index + 1}` }),
    );
    const messageApi = createMessageApi(manyMessages);

    render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} contactsApi={createContactsApiWithAcceptedPeerAvatar()} />);

    await userEvent.click(await screen.findByRole('button', { name: /Bob Lin/ }));

    const chatWindow = screen.getByRole('region', { name: 'Bob Lin 聊天窗口' });
    const header = within(chatWindow).getByRole('banner', { name: 'Bob Lin 聊天头部' });
    const messageThread = within(chatWindow).getByTestId('message-thread-scroll-region');
    const composer = within(chatWindow).getByRole('form', { name: '发送消息' });

    expect(header.compareDocumentPosition(messageThread) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(messageThread.compareDocumentPosition(composer) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(header).toHaveTextContent('Bob Lin');
    expect(within(header).getByRole('button', { name: '返回消息列表' })).toBeInTheDocument();
    expect(within(composer).getByRole('textbox', { name: '输入消息' })).toBeInTheDocument();
    expect(messageThread).toHaveClass('message-thread');
    expect(messageThread).toHaveTextContent('历史消息 1');
    expect(messageThread).toHaveTextContent('历史消息 80');
  });

  it('hydrates group title and renders sender display names for group history', async () => {
    const user = userEvent.setup();
    const groupMessage = groupServerMessage({ seq: 1, content: '大家好' });
    const sendMessage = vi.fn(async (request: SendMessageRequest): Promise<SendMessageResponse> => ({
      deduplicated: false,
      message: groupServerMessage({
        serverMsgId: 'srv_group_reply',
        clientMsgId: request.clientMsgId,
        seq: 2,
        senderId: currentUserId,
        content: request.content,
      }),
    }));
    const messageApi = createMessageApi([groupMessage], sendMessage);
    vi.mocked(messageApi.getConversationSeqs).mockResolvedValueOnce({
      states: [
        {
          conversationId: groupConversationId,
          maxSeq: 1,
          hasReadSeq: 0,
          unreadCount: 1,
          maxSeqTime: groupMessage.sendTime,
          lastMessage: groupMessage,
        },
      ],
    });
    vi.mocked(messageApi.pullMessages).mockResolvedValueOnce({ conversationId: groupConversationId, messages: [groupMessage] });

    render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} groupsApi={createGroupsApi()} />);

    const row = await screen.findByRole('button', { name: /项目群/ });
    expect(within(row).getByText('项目群')).toBeInTheDocument();
    await user.click(row);

    const log = await screen.findByRole('log', { name: '聊天消息' });
    expect(within(log).getByText('Bob Lin')).toBeInTheDocument();
    expect(within(log).getByText('大家好')).toBeInTheDocument();
    expect(screen.queryByText(peerUserId)).not.toBeInTheDocument();

    await user.type(screen.getByRole('textbox', { name: '输入消息' }), '收到');
    await user.click(screen.getByRole('button', { name: '发送' }));
    await waitFor(() =>
      expect(sendMessage).toHaveBeenCalledWith(
        expect.objectContaining({
          groupId,
          chatType: 'group',
          content: '收到',
        }),
      ),
    );
  });

  it('shows a Chinese group permission error when group send is rejected', async () => {
    const user = userEvent.setup();
    const groupMessage = groupServerMessage({ seq: 1, content: 'seed group' });
    const messageApi = createMessageApi([groupMessage]);
    vi.mocked(messageApi.getConversationSeqs).mockResolvedValueOnce({
      states: [{ conversationId: groupConversationId, maxSeq: 1, hasReadSeq: 0, unreadCount: 1, lastMessage: groupMessage }],
    });
    vi.mocked(messageApi.pullMessages).mockResolvedValueOnce({ conversationId: groupConversationId, messages: [groupMessage] });
    vi.mocked(messageApi.sendMessage).mockRejectedValueOnce(new Error('requester is not a group member'));

    render(<MessagesPage currentUserId={currentUserId} messageApi={messageApi} groupsApi={createGroupsApi()} />);

    await user.click(await screen.findByRole('button', { name: /项目群/ }));
    await user.type(screen.getByRole('textbox', { name: '输入消息' }), '我还能发吗');
    await user.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('没有群聊权限，无法发送消息'));
  });

  it('applies live group messages to the current conversation without refresh and keeps sender names', async () => {
    const user = userEvent.setup();
    const groupMessage = groupServerMessage({ seq: 1, content: 'seed group' });
    const messageApi = createMessageApi([groupMessage]);
    vi.mocked(messageApi.getConversationSeqs).mockResolvedValueOnce({
      states: [{ conversationId: groupConversationId, maxSeq: 1, hasReadSeq: 0, unreadCount: 0, lastMessage: groupMessage }],
    });
    vi.mocked(messageApi.pullMessages).mockResolvedValueOnce({ conversationId: groupConversationId, messages: [groupMessage] });
    const { sockets, factory } = createFakeWebSocketFactory();
    const groupsApi = createGroupsApi({
      listMembers: vi.fn(async () => ({
        group_id: groupId,
        members: [
          {
            group_id: groupId,
            user_id: currentUserId,
            display_name: 'Alice Chen',
            name: 'Alice Chen',
            identifier: 'alice_001',
            avatar_media_id: '',
            state: 'active',
            joined_at: '',
            left_at: '',
          },
          {
            group_id: groupId,
            user_id: '3003',
            display_name: 'Carol Wu',
            name: 'Carol Wu',
            identifier: 'carol_003',
            avatar_media_id: '',
            state: 'active',
            joined_at: '',
            left_at: '',
          },
        ],
      })),
    });

    render(
      <MessagesPage
        currentUserId={currentUserId}
        messageApi={messageApi}
        groupsApi={groupsApi}
        webSocketFactory={factory}
        webSocketUrl="ws://127.0.0.1/ws"
        webSocketToken="test-token"
      />,
    );

    await waitFor(() => expect(sockets).toHaveLength(1));
    await user.click(await screen.findByRole('button', { name: /项目群/ }));
    act(() => {
      sockets[0].open();
      sockets[0].receive(
        messageReceivedEvent({
          serverMsgId: 'srv_group_live',
          conversationId: groupConversationId,
          seq: 2,
          senderId: '3003',
          receiverId: '',
          groupId,
          chatType: 'group',
          content: '实时群消息',
        }),
      );
    });

    const log = await screen.findByRole('log', { name: '聊天消息' });
    expect(within(log).getByText('Carol Wu')).toBeInTheDocument();
    expect(within(log).getByText('实时群消息')).toBeInTheDocument();
  });

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

  it('renders the accepted peer avatar in the direct conversation list and header', async () => {
    const user = userEvent.setup();
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: '来自 Bob 的历史消息' })]);

    render(
      <MessagesPage
        currentUserId={currentUserId}
        messageApi={messageApi}
        contactsApi={createContactsApiWithAcceptedPeerAvatar()}
      />,
    );

    const row = await screen.findByRole('button', { name: /Bob Lin/ });
    expect(within(row).getByRole('img', { name: 'Bob Lin 头像' })).toHaveAttribute('src', bobProfile.avatar_url);

    await user.click(row);

    const header = await screen.findByRole('banner', { name: 'Bob Lin 聊天头部' });
    expect(within(header).getByRole('img', { name: 'Bob Lin 头像' })).toHaveAttribute('src', bobProfile.avatar_url);
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

  it('blocks oversized images locally before upload or send', async () => {
    const user = userEvent.setup();
    const sendMessage = vi.fn<MessageApi['sendMessage']>();
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: 'seed before image upload' })], sendMessage);
    const mediaApi = createMediaApi();
    const uploadFetch = vi.fn();
    vi.stubGlobal('fetch', uploadFetch);

    render(
      <MessagesPage currentUserId={currentUserId} messageApi={messageApi} mediaApi={mediaApi} contactsApi={createContactsApi()} />,
    );
    await user.click(await screen.findByRole('button', { name: /未知联系人/ }));

    await user.upload(screen.getByLabelText('发送图片'), sizedFile('huge.jpg', 'image/jpeg', 15 * 1024 * 1024 + 1));

    expect(await screen.findByRole('status')).toHaveTextContent('图片不能超过 15 MiB');
    expect(mediaApi.createUploadIntent).not.toHaveBeenCalled();
    expect(mediaApi.completeUpload).not.toHaveBeenCalled();
    expect(uploadFetch).not.toHaveBeenCalled();
    expect(sendMessage).not.toHaveBeenCalled();
  });

  it('blocks unsupported image MIME locally before upload or send', async () => {
    const user = userEvent.setup({ applyAccept: false });
    const sendMessage = vi.fn<MessageApi['sendMessage']>();
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: 'seed before invalid image upload' })], sendMessage);
    const mediaApi = createMediaApi();
    const uploadFetch = vi.fn();
    vi.stubGlobal('fetch', uploadFetch);

    render(
      <MessagesPage currentUserId={currentUserId} messageApi={messageApi} mediaApi={mediaApi} contactsApi={createContactsApi()} />,
    );
    await user.click(await screen.findByRole('button', { name: /未知联系人/ }));

    await user.upload(screen.getByLabelText('发送图片'), sizedFile('notes.txt', 'text/plain', 512));

    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('请选择 JPG、PNG、WebP 或 GIF 图片'));
    expect(mediaApi.createUploadIntent).not.toHaveBeenCalled();
    expect(mediaApi.completeUpload).not.toHaveBeenCalled();
    expect(uploadFetch).not.toHaveBeenCalled();
    expect(sendMessage).not.toHaveBeenCalled();
  });

  it('blocks oversized files locally before upload or send', async () => {
    const user = userEvent.setup();
    const sendMessage = vi.fn<MessageApi['sendMessage']>();
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: 'seed before file upload' })], sendMessage);
    const mediaApi = createMediaApi();
    const uploadFetch = vi.fn();
    vi.stubGlobal('fetch', uploadFetch);

    render(
      <MessagesPage currentUserId={currentUserId} messageApi={messageApi} mediaApi={mediaApi} contactsApi={createContactsApi()} />,
    );
    await user.click(await screen.findByRole('button', { name: /未知联系人/ }));

    await user.upload(screen.getByLabelText('发送文件'), sizedFile('huge.pdf', 'application/pdf', 20 * 1024 * 1024 + 1));

    expect(await screen.findByRole('status')).toHaveTextContent('文件不能超过 20 MiB');
    expect(mediaApi.createUploadIntent).not.toHaveBeenCalled();
    expect(mediaApi.completeUpload).not.toHaveBeenCalled();
    expect(uploadFetch).not.toHaveBeenCalled();
    expect(sendMessage).not.toHaveBeenCalled();
  });

  it('uploads a valid image before sending an image message with media metadata', async () => {
    const user = userEvent.setup();
    const sendMessage = vi.fn<MessageApi['sendMessage']>(async (request) => ({
      deduplicated: false,
      message: serverMessage({
        serverMsgId: 'srv_image_2',
        clientMsgId: request.clientMsgId,
        seq: 2,
        senderId: currentUserId,
        receiverId: request.chatType === 'single' ? request.receiverId : undefined,
        groupId: request.chatType === 'group' ? request.groupId : undefined,
        chatType: request.chatType,
        contentType: request.contentType,
        content: request.content,
        sendTime: 1777464500000,
        createdAt: 1777464500000,
      }),
    }));
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: 'seed before image upload' })], sendMessage);
    const mediaApi = createMediaApi();
    const uploadFetch = vi.fn(async () => new Response('', { status: 200 }));
    vi.stubGlobal('fetch', uploadFetch);
    const image = sizedFile('cat.jpg', 'image/jpeg', 1024);

    render(
      <MessagesPage currentUserId={currentUserId} messageApi={messageApi} mediaApi={mediaApi} contactsApi={createContactsApi()} />,
    );
    await user.click(await screen.findByRole('button', { name: /未知联系人/ }));
    await user.upload(screen.getByLabelText('发送图片'), image);

    await waitFor(() => expect(sendMessage).toHaveBeenCalledTimes(1));
    expect(mediaApi.createUploadIntent).toHaveBeenCalledWith({
      purpose: 'message_image',
      filename: 'cat.jpg',
      contentType: 'image/jpeg',
      sizeBytes: image.size,
    });
    expect(uploadFetch).toHaveBeenCalledWith(
      'https://storage.test/upload/cat.jpg',
      expect.objectContaining({
        method: 'PUT',
        body: image,
        headers: { 'Content-Type': 'image/jpeg' },
      }),
    );
    expect(mediaApi.completeUpload).toHaveBeenCalledWith('med_image_1');
    expect(mediaApi.createUploadIntent.mock.invocationCallOrder[0]).toBeLessThan(uploadFetch.mock.invocationCallOrder[0]);
    expect(uploadFetch.mock.invocationCallOrder[0]).toBeLessThan(mediaApi.completeUpload.mock.invocationCallOrder[0]);
    expect(mediaApi.completeUpload.mock.invocationCallOrder[0]).toBeLessThan(sendMessage.mock.invocationCallOrder[0]);

    const request = sendMessage.mock.calls[0][0];
    expect(request).toEqual(
      expect.objectContaining({
        receiverId: peerUserId,
        chatType: 'single',
        contentType: 'image',
      }),
    );
    expect(JSON.parse(request.content)).toMatchObject({ mediaId: 'med_image_1' });
    expect(await screen.findByRole('img', { name: '图片 cat.jpg' })).toHaveAttribute('src', 'https://media.test/download/med_image_1');
  });

  it('uploads a valid file before sending a file message with media metadata', async () => {
    const user = userEvent.setup();
    const sendMessage = vi.fn<MessageApi['sendMessage']>(async (request) => ({
      deduplicated: false,
      message: serverMessage({
        serverMsgId: 'srv_file_2',
        clientMsgId: request.clientMsgId,
        seq: 2,
        senderId: currentUserId,
        receiverId: request.chatType === 'single' ? request.receiverId : undefined,
        groupId: request.chatType === 'group' ? request.groupId : undefined,
        chatType: request.chatType,
        contentType: request.contentType,
        content: request.content,
        sendTime: 1777464500000,
        createdAt: 1777464500000,
      }),
    }));
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: 'seed before file upload' })], sendMessage);
    const mediaApi = createMediaApi();
    const uploadFetch = vi.fn(async () => new Response('', { status: 200 }));
    vi.stubGlobal('fetch', uploadFetch);
    const file = sizedFile('report.pdf', 'application/pdf', 4096);

    render(
      <MessagesPage currentUserId={currentUserId} messageApi={messageApi} mediaApi={mediaApi} contactsApi={createContactsApi()} />,
    );
    await user.click(await screen.findByRole('button', { name: /未知联系人/ }));
    await user.upload(screen.getByLabelText('发送文件'), file);

    await waitFor(() => expect(sendMessage).toHaveBeenCalledTimes(1));
    expect(mediaApi.createUploadIntent).toHaveBeenCalledWith({
      purpose: 'message_file',
      filename: 'report.pdf',
      contentType: 'application/pdf',
      sizeBytes: file.size,
    });
    expect(uploadFetch).toHaveBeenCalledWith(
      'https://storage.test/upload/report.pdf',
      expect.objectContaining({
        method: 'PUT',
        body: file,
        headers: { 'Content-Type': 'application/pdf' },
      }),
    );
    expect(mediaApi.completeUpload).toHaveBeenCalledWith('med_file_1');

    const request = sendMessage.mock.calls[0][0];
    expect(request).toEqual(
      expect.objectContaining({
        receiverId: peerUserId,
        chatType: 'single',
        contentType: 'file',
      }),
    );
    expect(JSON.parse(request.content)).toEqual({
      mediaId: 'med_file_1',
      filename: 'report.pdf',
      sizeBytes: file.size,
      contentType: 'application/pdf',
    });
    expect(screen.getByText('文件 report.pdf')).toBeInTheDocument();
  });

  it('downloads an outgoing file message after upload through authorized media URL', async () => {
    const user = userEvent.setup();
    const sendMessage = vi.fn<MessageApi['sendMessage']>(async (request) => ({
      deduplicated: false,
      message: serverMessage({
        serverMsgId: 'srv_download_file_2',
        clientMsgId: request.clientMsgId,
        seq: 2,
        senderId: currentUserId,
        receiverId: request.chatType === 'single' ? request.receiverId : undefined,
        groupId: request.chatType === 'group' ? request.groupId : undefined,
        chatType: request.chatType,
        contentType: request.contentType,
        content: request.content,
        sendTime: 1777464500000,
        createdAt: 1777464500000,
      }),
    }));
    const messageApi = createMessageApi([serverMessage({ seq: 1, content: 'seed before file download' })], sendMessage);
    const mediaApi = createMediaApi();
    const downloadMedia = vi.fn();
    const uploadFetch = vi.fn(async () => new Response('', { status: 200 }));
    vi.stubGlobal('fetch', uploadFetch);
    const file = sizedFile('report.pdf', 'application/pdf', 4096);

    render(
      <MessagesPage
        currentUserId={currentUserId}
        messageApi={messageApi}
        mediaApi={mediaApi}
        contactsApi={createContactsApi()}
        downloadMedia={downloadMedia}
      />,
    );
    await user.click(await screen.findByRole('button', { name: /未知联系人/ }));
    await user.upload(screen.getByLabelText('发送文件'), file);

    await waitFor(() => expect(sendMessage).toHaveBeenCalledTimes(1));
    const log = await screen.findByRole('log', { name: '聊天消息' });
    const downloadButton = await within(log).findByRole('button', { name: '下载文件 report.pdf' });
    expect(mediaApi.getDownloadURL).not.toHaveBeenCalled();

    await user.click(downloadButton);

    await waitFor(() => expect(mediaApi.getDownloadURL).toHaveBeenCalledWith('med_file_1'));
    expect(downloadMedia).toHaveBeenCalledWith('https://media.test/download/med_file_1', 'report.pdf');
    expect(screen.getByRole('status')).toHaveTextContent('已获取文件下载链接');
  });

  it('renders image history messages as image bubbles instead of raw JSON text', async () => {
    const user = userEvent.setup();
    const mediaApi = createMediaApi();
    const imageContent = JSON.stringify({ mediaId: 'med_history_image', filename: 'cat.jpg', width: 640, height: 480 });
    const messageApi = createMessageApi([serverMessage({ seq: 1, contentType: 'image', content: imageContent })]);

    render(
      <MessagesPage currentUserId={currentUserId} messageApi={messageApi} mediaApi={mediaApi} contactsApi={createContactsApi()} />,
    );

    await user.click(await screen.findByRole('button', { name: /图片 cat.jpg/ }));
    const log = await screen.findByRole('log', { name: '聊天消息' });

    expect(await within(log).findByRole('img', { name: '图片 cat.jpg' })).toHaveAttribute(
      'src',
      'https://media.test/download/med_history_image',
    );
    expect(within(log).queryByText(imageContent)).not.toBeInTheDocument();
    expect(mediaApi.getDownloadURL).toHaveBeenCalledWith('med_history_image');
  });

  it('renders live websocket image messages as image bubbles without refresh', async () => {
    const user = userEvent.setup();
    const mediaApi = createMediaApi();
    const messageApi = createMessageApi([]);
    const { sockets, factory } = createFakeWebSocketFactory();
    const imageContent = JSON.stringify({ mediaId: 'med_live_image', filename: 'live.jpg' });

    render(
      <MessagesPage
        currentUserId={currentUserId}
        messageApi={messageApi}
        mediaApi={mediaApi}
        contactsApi={createContactsApi()}
        webSocketFactory={factory}
        webSocketUrl="ws://127.0.0.1/ws"
        webSocketToken="test-token"
      />,
    );

    await waitFor(() => expect(sockets).toHaveLength(1));
    act(() => {
      sockets[0].open();
      sockets[0].receive(messageReceivedEvent({ serverMsgId: 'srv_live_image', seq: 1, contentType: 'image', content: imageContent }));
    });

    await user.click(await screen.findByRole('button', { name: /图片 live.jpg/ }));
    const log = await screen.findByRole('log', { name: '聊天消息' });
    expect(await within(log).findByRole('img', { name: '图片 live.jpg' })).toHaveAttribute(
      'src',
      'https://media.test/download/med_live_image',
    );
  });

  it('opens and closes image preview from the image bubble', async () => {
    const user = userEvent.setup();
    const imageContent = JSON.stringify({ mediaId: 'med_preview_image', filename: 'preview.jpg' });
    const mediaApi = createMediaApi();
    const messageApi = createMessageApi([serverMessage({ seq: 1, contentType: 'image', content: imageContent })]);

    render(
      <MessagesPage currentUserId={currentUserId} messageApi={messageApi} mediaApi={mediaApi} contactsApi={createContactsApi()} />,
    );

    await user.click(await screen.findByRole('button', { name: /图片 preview.jpg/ }));
    await user.click(await screen.findByRole('button', { name: '预览图片 preview.jpg' }));

    const dialog = await screen.findByRole('dialog', { name: '图片预览' });
    expect(within(dialog).getByRole('img', { name: '预览图片 preview.jpg' })).toHaveAttribute(
      'src',
      'https://media.test/download/med_preview_image',
    );

    await user.click(within(dialog).getByRole('button', { name: '关闭预览' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: '图片预览' })).not.toBeInTheDocument());
  });

  it('downloads an image through authorized media URL and an injected download handler', async () => {
    const user = userEvent.setup();
    const imageContent = JSON.stringify({ mediaId: 'med_download_image', filename: 'download.jpg' });
    const mediaApi = createMediaApi();
    const downloadMedia = vi.fn();
    const messageApi = createMessageApi([serverMessage({ seq: 1, contentType: 'image', content: imageContent })]);

    render(
      <MessagesPage
        currentUserId={currentUserId}
        messageApi={messageApi}
        mediaApi={mediaApi}
        contactsApi={createContactsApi()}
        downloadMedia={downloadMedia}
      />,
    );

    await user.click(await screen.findByRole('button', { name: /图片 download.jpg/ }));
    await user.click(await screen.findByRole('button', { name: '下载图片 download.jpg' }));

    await waitFor(() => expect(mediaApi.getDownloadURL).toHaveBeenCalledWith('med_download_image'));
    expect(downloadMedia).toHaveBeenCalledWith('https://media.test/download/med_download_image', 'download.jpg');
  });

  it('downloads an incoming historical file message through authorized media URL and an injected download handler', async () => {
    const user = userEvent.setup();
    const fileContent = JSON.stringify({
      mediaId: 'med_history_file',
      filename: 'history.pdf',
      sizeBytes: 4096,
      contentType: 'application/pdf',
    });
    const mediaApi = createMediaApi();
    const downloadMedia = vi.fn();
    const messageApi = createMessageApi([serverMessage({ seq: 1, contentType: 'file', content: fileContent })]);

    render(
      <MessagesPage
        currentUserId={currentUserId}
        messageApi={messageApi}
        mediaApi={mediaApi}
        contactsApi={createContactsApi()}
        downloadMedia={downloadMedia}
      />,
    );

    await user.click(await screen.findByRole('button', { name: /文件 history.pdf/ }));
    const log = await screen.findByRole('log', { name: '聊天消息' });
    expect(within(log).getByText('文件 history.pdf')).toBeInTheDocument();
    expect(within(log).queryByText(fileContent)).not.toBeInTheDocument();

    await user.click(within(log).getByRole('button', { name: '下载文件 history.pdf' }));

    await waitFor(() => expect(mediaApi.getDownloadURL).toHaveBeenCalledWith('med_history_file'));
    expect(downloadMedia).toHaveBeenCalledWith('https://media.test/download/med_history_file', 'history.pdf');
    expect(screen.getByRole('status')).toHaveTextContent('已获取文件下载链接');
  });

  it('renders live websocket file messages with a working download button without refresh', async () => {
    const user = userEvent.setup();
    const mediaApi = createMediaApi();
    const downloadMedia = vi.fn();
    const messageApi = createMessageApi([]);
    const { sockets, factory } = createFakeWebSocketFactory();
    const fileContent = JSON.stringify({
      mediaId: 'med_live_file',
      filename: 'live.pdf',
      sizeBytes: 8192,
      contentType: 'application/pdf',
    });

    render(
      <MessagesPage
        currentUserId={currentUserId}
        messageApi={messageApi}
        mediaApi={mediaApi}
        contactsApi={createContactsApi()}
        downloadMedia={downloadMedia}
        webSocketFactory={factory}
        webSocketUrl="ws://127.0.0.1/ws"
        webSocketToken="test-token"
      />,
    );

    await waitFor(() => expect(sockets).toHaveLength(1));
    act(() => {
      sockets[0].open();
      sockets[0].receive(messageReceivedEvent({ serverMsgId: 'srv_live_file', seq: 1, contentType: 'file', content: fileContent }));
    });

    await user.click(await screen.findByRole('button', { name: /文件 live.pdf/ }));
    const log = await screen.findByRole('log', { name: '聊天消息' });
    await user.click(await within(log).findByRole('button', { name: '下载文件 live.pdf' }));

    await waitFor(() => expect(mediaApi.getDownloadURL).toHaveBeenCalledWith('med_live_file'));
    expect(downloadMedia).toHaveBeenCalledWith('https://media.test/download/med_live_file', 'live.pdf');
  });

  it('shows an error for file messages missing mediaId and does not request a download URL', async () => {
    const user = userEvent.setup();
    const fileContent = JSON.stringify({
      filename: 'missing-media.pdf',
      sizeBytes: 512,
      contentType: 'application/pdf',
    });
    const mediaApi = createMediaApi();
    const downloadMedia = vi.fn();
    const messageApi = createMessageApi([serverMessage({ seq: 1, contentType: 'file', content: fileContent })]);

    render(
      <MessagesPage
        currentUserId={currentUserId}
        messageApi={messageApi}
        mediaApi={mediaApi}
        contactsApi={createContactsApi()}
        downloadMedia={downloadMedia}
      />,
    );

    await user.click(await screen.findByRole('button', { name: /文件 missing-media.pdf/ }));
    const log = await screen.findByRole('log', { name: '聊天消息' });
    await user.click(await within(log).findByRole('button', { name: '下载文件 missing-media.pdf' }));

    expect(await within(log).findByRole('alert')).toHaveTextContent('文件信息缺失，无法下载');
    expect(screen.getByRole('status')).toHaveTextContent('文件信息缺失，无法下载');
    expect(mediaApi.getDownloadURL).not.toHaveBeenCalled();
    expect(downloadMedia).not.toHaveBeenCalled();
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

  it('renders one centered date separator for messages from the same local day', async () => {
    const sameDay = localTimestamp(2026, 4, 29, 9, 12);
    const laterSameDay = localTimestamp(2026, 4, 29, 21, 35);
    const dateLabel = expectedLocalDate(sameDay);
    const log = await openSeededConversation(
      createMessageApi([
        serverMessage({ seq: 1, content: 'same-day first', sendTime: sameDay, createdAt: sameDay }),
        serverMessage({ seq: 2, content: 'same-day second', sendTime: laterSameDay, createdAt: laterSameDay }),
      ]),
    );

    const separators = within(log).getAllByRole('separator', { name: dateLabel });
    expect(separators).toHaveLength(1);
    expect(separators[0]).toHaveClass('message-date-separator');
    expectTextOrder(log, [dateLabel, 'same-day first', 'same-day second']);
  });

  it('renders centered date separators above the first message of each local day', async () => {
    const firstDay = localTimestamp(2026, 4, 28, 22, 10);
    const secondDay = localTimestamp(2026, 4, 29, 8, 5);
    const firstDayLabel = expectedLocalDate(firstDay);
    const secondDayLabel = expectedLocalDate(secondDay);
    const log = await openSeededConversation(
      createMessageApi([
        serverMessage({ seq: 1, content: 'previous local day', sendTime: firstDay, createdAt: firstDay }),
        serverMessage({ seq: 2, content: 'next local day', sendTime: secondDay, createdAt: secondDay }),
      ]),
    );

    expect(within(log).getByRole('separator', { name: firstDayLabel })).toHaveClass('message-date-separator');
    expect(within(log).getByRole('separator', { name: secondDayLabel })).toHaveClass('message-date-separator');
    expectTextOrder(log, [firstDayLabel, 'previous local day', secondDayLabel, 'next local day']);
  });

  it('renders outgoing message metadata as local time with a compact success checkmark', async () => {
    const sentAt = localTimestamp(2026, 4, 29, 20, 5);
    const log = await openSeededConversation(
      createMessageApi(
        [
          serverMessage({
            seq: 1,
            content: 'outgoing with compact metadata',
            senderId: currentUserId,
            receiverId: peerUserId,
            sendTime: sentAt,
            createdAt: sentAt,
          }),
        ],
        undefined,
        { hasReadSeq: 0, unreadCount: 0 },
      ),
    );

    const outgoingMessage = within(log).getByRole('article', { name: '我发送的消息：outgoing with compact metadata' });
    expect(within(outgoingMessage).getByText(expectedLocalTime(sentAt))).toBeInTheDocument();
    expect(within(outgoingMessage).getByLabelText('发送成功')).toHaveTextContent('✔');
    expect(within(outgoingMessage).queryByText('已发送')).not.toBeInTheDocument();
  });

  it('renders incoming message metadata as local time without outgoing checkmarks', async () => {
    const receivedAt = localTimestamp(2026, 4, 29, 8, 3);
    const log = await openSeededConversation(
      createMessageApi([serverMessage({ seq: 1, content: 'incoming with time only', sendTime: receivedAt, createdAt: receivedAt })]),
    );
    const incomingMessage = within(log).getByRole('article', { name: '收到的消息：incoming with time only' });

    expect(within(incomingMessage).getByText(expectedLocalTime(receivedAt))).toBeInTheDocument();
    expect(within(incomingMessage).queryByLabelText('发送成功')).not.toBeInTheDocument();
    expect(within(incomingMessage).queryByLabelText('对方已读')).not.toBeInTheDocument();
    expect(within(incomingMessage).queryByText('✔')).not.toBeInTheDocument();
  });

  it('keeps read receipts as checkmarks without rendering residual read-to text', async () => {
    const messageApi = createMessageApi(
      [serverMessage({ seq: 1, content: 'incoming read sync trigger' })],
      undefined,
      { hasReadSeq: 0, unreadCount: 1 },
    );

    await openSeededConversation(messageApi);

    await waitFor(() => expect(messageApi.markRead).toHaveBeenCalledWith(conversationId, { hasReadSeq: 1 }));
    expect(screen.queryByText(/已读到\s*1/)).not.toBeInTheDocument();
    expect(screen.queryByText(/已读到/)).not.toBeInTheDocument();
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
