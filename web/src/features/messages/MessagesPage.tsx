import { ChevronLeft, MessageCircle, Search, SendHorizontal } from 'lucide-react';
import { useEffect, useMemo, useState, type FormEvent } from 'react';
import type { ConversationSeqState, MessageApi, ServerMessage } from '../../api/messages';
import { createMessageApi } from '../../api/messages';
import type { UserApi, UserProfile } from '../../api/user';
import { createUserApi } from '../../api/user';
import type { ChatMessage, Conversation, MessageStatus } from '../../models/messages';

type MessagesPageProps = {
  currentUserId: string;
  messageApi?: MessageApi;
  userApi?: UserApi;
  startChatSignal?: number;
};

const statusLabels: Record<MessageStatus, string> = {
  sending: '发送中',
  sent: '已发送',
  failed: '发送失败',
};

export function MessagesPage({
  currentUserId,
  messageApi = createMessageApi(),
  userApi = createUserApi(),
  startChatSignal = 0,
}: MessagesPageProps) {
  const [items, setItems] = useState<Conversation[]>([]);
  const [status, setStatus] = useState('正在加载会话');
  const [selectedConversationId, setSelectedConversationId] = useState<string | null>(null);
  const [showStartChat, setShowStartChat] = useState(false);
  const selectedConversation = items.find((conversation) => conversation.id === selectedConversationId) ?? null;

  useEffect(() => {
    let cancelled = false;
    async function loadConversations() {
      setStatus('正在加载会话');
      try {
        const response = await messageApi.getConversationSeqs([]);
        const states = response.states ?? response.conversations ?? response.seqs ?? [];
        const conversations = await Promise.all(states.map((state) => loadConversation(state, currentUserId, messageApi)));
        if (!cancelled) {
          setItems(conversations);
          setStatus(conversations.length > 0 ? `已加载 ${conversations.length} 个会话` : '暂无会话');
        }
      } catch (error) {
        if (!cancelled) {
          setStatus(error instanceof Error ? error.message : '加载会话失败');
        }
      }
    }

    void loadConversations();
    return () => {
      cancelled = true;
    };
  }, [currentUserId, messageApi]);

  useEffect(() => {
    if (startChatSignal > 0) {
      setShowStartChat(true);
      setSelectedConversationId(null);
    }
  }, [startChatSignal]);

  function handleStartChat(profile: UserProfile) {
    const draftConversation = userProfileToDraftConversation(profile);
    const existingConversation = items.find(
      (conversation) => conversation.chatType === 'single' && conversation.receiverId === profile.user_id,
    );
    const selectedId = existingConversation?.id ?? draftConversation.id;

    setItems((current) => upsertStartedConversation(current, existingConversation?.id, draftConversation));
    setSelectedConversationId(selectedId);
    setShowStartChat(false);
    setStatus(`已打开 ${draftConversation.title} 的聊天`);
  }

  function handleSend(content: string) {
    if (!selectedConversation) {
      return;
    }

    const pendingMessage = createPendingMessage(selectedConversation, content, currentUserId);
    setItems((current) => appendMessage(current, selectedConversation.id, pendingMessage));

    void Promise.resolve()
      .then(() => sendMessageWithApi(messageApi, pendingMessage))
      .then((sentMessage) => {
        setItems((current) => confirmSentMessage(current, selectedConversation.id, pendingMessage.id, sentMessage));
        setSelectedConversationId(sentMessage.conversationId);
      })
      .catch((error) => {
        setStatus(error instanceof Error ? error.message : '发送消息失败');
        setItems((current) =>
          updateMessage(current, selectedConversation.id, pendingMessage.id, {
            ...pendingMessage,
            status: 'failed',
          }),
        );
      });
  }

  if (selectedConversation) {
    return (
      <ChatWindow
        conversation={selectedConversation}
        onBack={() => setSelectedConversationId(null)}
        onSend={handleSend}
        status={status}
      />
    );
  }

  return (
    <ConversationList
      conversations={items}
      status={status}
      userApi={userApi}
      showStartChat={showStartChat}
      onOpenStartChat={() => setShowStartChat(true)}
      onCloseStartChat={() => setShowStartChat(false)}
      onStartChat={handleStartChat}
      onSelect={(conversationId) => setSelectedConversationId(conversationId)}
    />
  );
}

function ConversationList({
  conversations,
  status,
  userApi,
  showStartChat,
  onOpenStartChat,
  onCloseStartChat,
  onStartChat,
  onSelect,
}: {
  conversations: Conversation[];
  status: string;
  userApi: UserApi;
  showStartChat: boolean;
  onOpenStartChat: () => void;
  onCloseStartChat: () => void;
  onStartChat: (profile: UserProfile) => void;
  onSelect: (conversationId: string) => void;
}) {
  return (
    <div className="page-stack">
      <SearchBox placeholder="搜索" />
      {showStartChat ? <StartChatPanel userApi={userApi} onStartChat={onStartChat} onClose={onCloseStartChat} /> : null}
      <p className="inline-status" role="status">
        {status}
      </p>
      {conversations.length === 0 ? (
        <div className="empty-state empty-state-action">
          <p>暂无会话</p>
          <button type="button" className="compact-command" onClick={onOpenStartChat}>
            <MessageCircle size={17} />
            <span>发起聊天</span>
          </button>
        </div>
      ) : null}
      <section className="list-card conversation-list" role="list" aria-label="消息列表">
        {conversations.map((item) => (
          <div className="conversation-list-item" role="listitem" key={item.id}>
            <button type="button" className="conversation-row conversation-button" onClick={() => onSelect(item.id)}>
              <div className={`avatar avatar-${item.color}`}>{item.avatar}</div>
              <div className="row-main">
                <div className="row-title-line">
                  <strong>{item.title}</strong>
                  <time>{item.time}</time>
                </div>
                <p>{item.preview}</p>
              </div>
              {item.unread > 0 ? <span className="unread-badge">{item.unread}</span> : null}
            </button>
          </div>
        ))}
      </section>
    </div>
  );
}

function StartChatPanel({
  userApi,
  onStartChat,
  onClose,
}: {
  userApi: UserApi;
  onStartChat: (profile: UserProfile) => void;
  onClose: () => void;
}) {
  const [identifier, setIdentifier] = useState('');
  const [result, setResult] = useState<UserProfile | null>(null);
  const [status, setStatus] = useState('输入 identifier 搜索用户');
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const query = identifier.trim();

    if (!query) {
      setResult(null);
      setStatus('请输入 identifier');
      return;
    }

    setSubmitting(true);
    setStatus('正在搜索用户');
    try {
      const profile = await userApi.getPublicProfileByIdentifier(query);
      setResult(profile);
      setStatus(`找到 ${profileDisplayName(profile)}`);
    } catch (error) {
      setResult(null);
      setStatus(error instanceof Error ? error.message : `未找到 ${query}`);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <section className="start-chat-card" aria-label="发起聊天">
      <div className="start-chat-heading">
        <h2>发起聊天</h2>
        <button type="button" className="text-command" onClick={onClose}>
          关闭
        </button>
      </div>
      <form className="identifier-search-form" onSubmit={handleSubmit}>
        <label className="search-box identifier-field">
          <Search size={17} />
          <input
            placeholder="输入唯一 identifier"
            aria-label="按 identifier 搜索聊天对象"
            value={identifier}
            onChange={(event) => setIdentifier(event.target.value)}
          />
        </label>
        <button className="compact-command" type="submit" aria-label="搜索聊天对象" disabled={submitting}>
          <Search size={17} />
          <span>搜索</span>
        </button>
      </form>
      <p className="inline-status" role="status">
        {status}
      </p>
      {result ? (
        <article className="search-result">
          <div className="avatar avatar-blue">{avatarText(profileDisplayName(result))}</div>
          <div className="row-main">
            <strong>{profileDisplayName(result)}</strong>
            <p>{result.identifier}</p>
            <p>{result.user_id}</p>
          </div>
          <button
            className="text-command"
            type="button"
            aria-label={`发起聊天 ${profileDisplayName(result)}`}
            onClick={() => onStartChat(result)}
          >
            发起聊天
          </button>
        </article>
      ) : null}
    </section>
  );
}

function ChatWindow({
  conversation,
  onBack,
  onSend,
  status,
}: {
  conversation: Conversation;
  onBack: () => void;
  onSend: (content: string) => void;
  status: string;
}) {
  const sortedMessages = useMemo(
    () => [...conversation.messages].sort((left, right) => left.sendTime - right.sendTime),
    [conversation.messages],
  );

  return (
    <section className="chat-window" aria-label={`${conversation.title} 聊天窗口`}>
      <header className="chat-header">
        <button type="button" className="chat-back-button" aria-label="返回消息列表" onClick={onBack}>
          <ChevronLeft size={24} />
        </button>
        <h2>{conversation.title}</h2>
      </header>
      <p className="inline-status" role="status">
        {status}
      </p>
      <div className="message-thread" role="log" aria-label="聊天消息">
        {sortedMessages.map((message) => (
          <article className={`message-row message-${message.direction}`} key={message.id}>
            <div className="message-body">
              <p className="message-bubble">{message.content}</p>
              {message.direction === 'outgoing' ? (
                <span className={`message-status message-status-${message.status}`}>{statusLabels[message.status]}</span>
              ) : null}
            </div>
          </article>
        ))}
      </div>

      <SendMessageComposer onSend={onSend} />
    </section>
  );
}

function SendMessageComposer({ onSend }: { onSend: (content: string) => void }) {
  const [draft, setDraft] = useState('');
  const trimmedDraft = draft.trim();

  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!trimmedDraft) {
      return;
    }

    onSend(trimmedDraft);
    setDraft('');
  }

  return (
    <form className="message-composer" aria-label="发送消息" onSubmit={handleSubmit}>
      <input aria-label="输入消息" value={draft} placeholder="输入消息" onChange={(event) => setDraft(event.target.value)} />
      <button type="submit" disabled={!trimmedDraft}>
        <SendHorizontal size={17} />
        <span>发送</span>
      </button>
    </form>
  );
}

function SearchBox({ placeholder }: { placeholder: string }) {
  return (
    <label className="search-box">
      <Search size={17} />
      <input placeholder={placeholder} aria-label={placeholder} />
    </label>
  );
}

async function loadConversation(state: ConversationSeqState, currentUserId: string, messageApi: MessageApi): Promise<Conversation> {
  const pulled = await messageApi.pullMessages(state.conversationId, { fromSeq: 1, limit: 50, order: 'asc' });
  return conversationStateToView(state, currentUserId, pulled.messages);
}

function userProfileToDraftConversation(profile: UserProfile): Conversation {
  const title = profileDisplayName(profile);

  return {
    id: draftConversationId(profile.user_id),
    title,
    avatar: avatarText(title),
    preview: '暂无消息',
    time: '',
    unread: 0,
    color: 'blue',
    chatType: 'single',
    receiverId: profile.user_id,
    messages: [],
  };
}

function upsertStartedConversation(conversations: Conversation[], existingConversationId: string | undefined, draftConversation: Conversation) {
  if (existingConversationId) {
    return conversations.map((conversation) =>
      conversation.id === existingConversationId
        ? {
            ...conversation,
            title: draftConversation.title,
            avatar: draftConversation.avatar,
            receiverId: draftConversation.receiverId,
          }
        : conversation,
    );
  }

  if (conversations.some((conversation) => conversation.id === draftConversation.id)) {
    return conversations.map((conversation) =>
      conversation.id === draftConversation.id ? { ...conversation, ...draftConversation, messages: conversation.messages } : conversation,
    );
  }

  return [draftConversation, ...conversations];
}

function conversationStateToView(state: ConversationSeqState, currentUserId: string, messages: ServerMessage[]): Conversation {
  const chatMessages = messages.map((message) => serverMessageToChatMessage(message, currentUserId));
  const lastMessage = chatMessages[chatMessages.length - 1] ?? serverLastMessage(state.lastMessage, currentUserId);
  const peerId = inferPeerId(state.conversationId, currentUserId, lastMessage);
  return {
    id: state.conversationId,
    title: peerId || state.conversationId,
    avatar: avatarText(peerId || state.conversationId),
    preview: lastMessage?.content ?? '暂无消息',
    time: state.maxSeqTime ? '刚刚' : '',
    unread: state.unreadCount ?? 0,
    color: state.conversationId.startsWith('group:') ? 'green' : 'blue',
    chatType: state.conversationId.startsWith('group:') ? 'group' : 'single',
    receiverId: state.conversationId.startsWith('group:') ? undefined : peerId,
    groupId: state.conversationId.startsWith('group:') ? state.conversationId.replace(/^group:/, '') : undefined,
    messages: chatMessages,
  };
}

function appendMessage(conversations: Conversation[], conversationId: string, message: ChatMessage) {
  return conversations.map((conversation) => {
    if (conversation.id !== conversationId) {
      return conversation;
    }

    return {
      ...conversation,
      preview: message.content,
      time: '刚刚',
      unread: 0,
      messages: [...conversation.messages, message],
    };
  });
}

function updateMessage(conversations: Conversation[], conversationId: string, messageId: string, nextMessage: ChatMessage) {
  return conversations.map((conversation) => {
    if (conversation.id !== conversationId) {
      return conversation;
    }

    return {
      ...conversation,
      preview: nextMessage.content,
      time: '刚刚',
      messages: conversation.messages.map((message) => (message.id === messageId ? nextMessage : message)),
    };
  });
}

function confirmSentMessage(conversations: Conversation[], conversationId: string, messageId: string, nextMessage: ChatMessage) {
  return conversations.map((conversation) => {
    if (conversation.id !== conversationId) {
      return conversation;
    }

    return {
      ...conversation,
      id: nextMessage.conversationId,
      preview: nextMessage.content,
      time: '刚刚',
      unread: 0,
      receiverId: nextMessage.receiverId ?? conversation.receiverId,
      groupId: nextMessage.groupId ?? conversation.groupId,
      messages: conversation.messages.map((message) => (message.id === messageId ? nextMessage : message)),
    };
  });
}

function createPendingMessage(conversation: Conversation, content: string, currentUserId: string): ChatMessage {
  const now = Date.now();
  const clientMsgId = `web-${now}-${Math.random().toString(36).slice(2, 8)}`;

  return {
    id: clientMsgId,
    conversationId: conversation.id,
    clientMsgId,
    senderId: currentUserId,
    receiverId: conversation.receiverId,
    groupId: conversation.groupId,
    chatType: conversation.chatType,
    contentType: 'text',
    content,
    sendTime: now,
    direction: 'outgoing',
    status: 'sending',
  };
}

function sendMessageWithApi(messageApi: MessageApi, message: ChatMessage): Promise<ChatMessage> {
  const request =
    message.chatType === 'group'
      ? {
          groupId: requiredField(message.groupId, 'groupId'),
          chatType: 'group' as const,
          clientMsgId: requiredField(message.clientMsgId, 'clientMsgId'),
          contentType: message.contentType,
          content: message.content,
        }
      : {
          receiverId: requiredField(message.receiverId, 'receiverId'),
          chatType: 'single' as const,
          clientMsgId: requiredField(message.clientMsgId, 'clientMsgId'),
          contentType: message.contentType,
          content: message.content,
        };

  return messageApi.sendMessage(request).then((response) => serverMessageToChatMessage(response.message, message.senderId));
}

function serverLastMessage(message: ServerMessage | undefined, currentUserId: string) {
  return message ? serverMessageToChatMessage(message, currentUserId) : undefined;
}

function serverMessageToChatMessage(message: ServerMessage, currentUserId: string): ChatMessage {
  return {
    id: message.serverMsgId,
    conversationId: message.conversationId,
    clientMsgId: message.clientMsgId,
    serverMsgId: message.serverMsgId,
    seq: message.seq,
    senderId: message.senderId,
    receiverId: message.receiverId,
    groupId: message.groupId,
    chatType: message.chatType,
    contentType: message.contentType,
    content: message.content,
    sendTime: message.sendTime,
    createdAt: message.createdAt,
    direction: message.senderId === currentUserId ? 'outgoing' : 'incoming',
    status: 'sent',
  };
}

function inferPeerId(conversationId: string, currentUserId: string, lastMessage?: ChatMessage) {
  if (lastMessage) {
    if (lastMessage.senderId && lastMessage.senderId !== currentUserId) {
      return lastMessage.senderId;
    }
    if (lastMessage.receiverId && lastMessage.receiverId !== currentUserId) {
      return lastMessage.receiverId;
    }
  }
  if (conversationId.startsWith('single:')) {
    return conversationId
      .replace(/^single:/, '')
      .split(':')
      .find((part) => part && part !== currentUserId);
  }
  return undefined;
}

function requiredField(value: string | undefined, fieldName: string) {
  if (!value) {
    throw new Error(`${fieldName} is required`);
  }
  return value;
}

function draftConversationId(userId: string) {
  return `draft-single:${userId}`;
}

function profileDisplayName(profile: UserProfile) {
  return profile.display_name || profile.name || profile.identifier || profile.user_id;
}

function avatarText(value: string) {
  return value.trim().slice(0, 2).toUpperCase() || 'C';
}
