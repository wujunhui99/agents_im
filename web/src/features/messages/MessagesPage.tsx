import { ChevronLeft, Search, SendHorizontal } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import type { ConversationSeqState, MessageApi, ServerMessage } from '../../api/messages';
import { createMessageApi } from '../../api/messages';
import type { ChatMessage, Conversation, MessageStatus } from '../../models/messages';

type MessagesPageProps = {
  currentUserId: string;
  messageApi?: MessageApi;
};

const statusLabels: Record<MessageStatus, string> = {
  sending: '发送中',
  sent: '已发送',
  failed: '发送失败',
};

export function MessagesPage({ currentUserId, messageApi = createMessageApi() }: MessagesPageProps) {
  const [items, setItems] = useState<Conversation[]>([]);
  const [status, setStatus] = useState('正在加载会话');
  const [selectedConversationId, setSelectedConversationId] = useState<string | null>(null);
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

  function handleSend(content: string) {
    if (!selectedConversation) {
      return;
    }
    if (conversationHasInFlightSend(selectedConversation)) {
      setStatus('上一条消息发送中');
      return;
    }

    const pendingMessage = createPendingMessage(selectedConversation, content, currentUserId);
    setItems((current) => appendMessage(current, selectedConversation.id, pendingMessage));

    void sendMessageWithApi(messageApi, pendingMessage)
      .then((sentMessage) => {
        setItems((current) => updateMessage(current, selectedConversation.id, pendingMessage.id, sentMessage));
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
        sending={conversationHasInFlightSend(selectedConversation)}
      />
    );
  }

  return <ConversationList conversations={items} status={status} onSelect={(conversationId) => setSelectedConversationId(conversationId)} />;
}

function ConversationList({
  conversations,
  status,
  onSelect,
}: {
  conversations: Conversation[];
  status: string;
  onSelect: (conversationId: string) => void;
}) {
  return (
    <div className="page-stack">
      <SearchBox placeholder="搜索" />
      <p className="inline-status" role="status">
        {status}
      </p>
      {conversations.length === 0 ? <p className="empty-state">暂无会话</p> : null}
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

function ChatWindow({
  conversation,
  onBack,
  onSend,
  status,
  sending,
}: {
  conversation: Conversation;
  onBack: () => void;
  onSend: (content: string) => void;
  status: string;
  sending: boolean;
}) {
  const sortedMessages = useMemo(() => orderedChatMessages(conversation.messages), [conversation.messages]);

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

      <SendMessageComposer onSend={onSend} sending={sending} />
    </section>
  );
}

function SendMessageComposer({ onSend, sending }: { onSend: (content: string) => void; sending: boolean }) {
  const [draft, setDraft] = useState('');
  const trimmedDraft = draft.trim();

  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (sending || !trimmedDraft) {
      return;
    }

    onSend(trimmedDraft);
    setDraft('');
  }

  return (
    <form className="message-composer" aria-label="发送消息" onSubmit={handleSubmit}>
      <input
        aria-label="输入消息"
        value={draft}
        placeholder="输入消息"
        disabled={sending}
        onChange={(event) => setDraft(event.target.value)}
      />
      <button type="submit" disabled={sending || !trimmedDraft}>
        <SendHorizontal size={17} />
        <span>{sending ? '发送中' : '发送'}</span>
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

function conversationStateToView(state: ConversationSeqState, currentUserId: string, messages: ServerMessage[]): Conversation {
  const chatMessages = canonicalChatMessages(messages.map((message) => serverMessageToChatMessage(message, currentUserId)));
  const orderedMessages = orderedChatMessages(chatMessages);
  const lastMessage = orderedMessages[orderedMessages.length - 1] ?? serverLastMessage(state.lastMessage, currentUserId);
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

    const nextMessages = canonicalChatMessages([...conversation.messages, message]);
    return {
      ...conversation,
      preview: conversationPreview(nextMessages, message.content),
      time: '刚刚',
      unread: 0,
      messages: nextMessages,
    };
  });
}

function updateMessage(conversations: Conversation[], conversationId: string, messageId: string, nextMessage: ChatMessage) {
  return conversations.map((conversation) => {
    if (conversation.id !== conversationId) {
      return conversation;
    }

    const nextMessages = upsertCanonicalMessage(conversation.messages, messageId, nextMessage);
    return {
      ...conversation,
      preview: conversationPreview(nextMessages, nextMessage.content),
      time: '刚刚',
      messages: nextMessages,
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

function avatarText(value: string) {
  return value.trim().slice(0, 2).toUpperCase() || 'C';
}

function conversationHasInFlightSend(conversation: Conversation) {
  return conversation.messages.some((message) => message.status === 'sending' && !hasAuthoritativeSeq(message));
}

function conversationPreview(messages: ChatMessage[], fallback: string) {
  const orderedMessages = orderedChatMessages(messages);
  return orderedMessages[orderedMessages.length - 1]?.content ?? fallback;
}

function orderedChatMessages(messages: ChatMessage[]) {
  return canonicalMessageEntries(messages)
    .sort((left, right) => compareMessageEntries(left, right))
    .map((entry) => entry.message);
}

function canonicalChatMessages(messages: ChatMessage[]) {
  return canonicalMessageEntries(messages).map((entry) => entry.message);
}

function upsertCanonicalMessage(messages: ChatMessage[], pendingMessageId: string, nextMessage: ChatMessage) {
  let replaced = false;
  const nextIdentityKeys = new Set(messageIdentityKeys(nextMessage));
  const updatedMessages = messages.map((message) => {
    if (message.id === pendingMessageId || messageIdentityKeys(message).some((key) => nextIdentityKeys.has(key))) {
      replaced = true;
      return chooseCanonicalMessage(message, nextMessage);
    }
    return message;
  });

  if (!replaced) {
    updatedMessages.push(nextMessage);
  }
  return canonicalChatMessages(updatedMessages);
}

type MessageEntry = {
  message: ChatMessage;
  index: number;
};

function canonicalMessageEntries(messages: ChatMessage[]) {
  const entries = new Map<string, MessageEntry>();
  const aliases = new Map<string, string>();

  messages.forEach((message, index) => {
    const identityKeys = messageIdentityKeys(message);
    const existingKey =
      identityKeys.map((key) => aliases.get(key)).find((key): key is string => Boolean(key)) ??
      identityKeys.find((key) => entries.has(key)) ??
      identityKeys[0];
    const existing = entries.get(existingKey);
    if (!existing) {
      entries.set(existingKey, { message, index });
    } else {
      entries.set(existingKey, {
        message: chooseCanonicalMessage(existing.message, message),
        index: Math.min(existing.index, index),
      });
    }

    identityKeys.forEach((key) => aliases.set(key, existingKey));
  });

  return Array.from(entries.values());
}

function compareMessageEntries(left: MessageEntry, right: MessageEntry) {
  const leftSeq = authoritativeSeq(left.message);
  const rightSeq = authoritativeSeq(right.message);
  if (leftSeq !== undefined && rightSeq !== undefined && leftSeq !== rightSeq) {
    return leftSeq - rightSeq;
  }
  if (leftSeq !== undefined && rightSeq === undefined) {
    return -1;
  }
  if (leftSeq === undefined && rightSeq !== undefined) {
    return 1;
  }
  if (leftSeq === undefined && rightSeq === undefined) {
    return left.index - right.index;
  }
  return stableMessageTieBreaker(left.message).localeCompare(stableMessageTieBreaker(right.message));
}

function hasAuthoritativeSeq(message: ChatMessage): message is ChatMessage & { seq: number } {
  return authoritativeSeq(message) !== undefined;
}

function authoritativeSeq(message: ChatMessage) {
  return typeof message.seq === 'number' && Number.isFinite(message.seq) && message.seq > 0 ? message.seq : undefined;
}

function messageIdentityKeys(message: ChatMessage) {
  return uniqueStrings([
    message.serverMsgId ? `server:${message.serverMsgId}` : '',
    message.clientMsgId ? `client:${message.clientMsgId}` : '',
    `local:${message.id}`,
  ]);
}

function chooseCanonicalMessage(current: ChatMessage, candidate: ChatMessage) {
  const currentCanonical = isServerCanonical(current);
  const candidateCanonical = isServerCanonical(candidate);
  if (candidateCanonical && !currentCanonical) {
    return candidate;
  }
  if (currentCanonical && !candidateCanonical) {
    return current;
  }
  if (candidate.status === 'sent' && current.status !== 'sent') {
    return candidate;
  }
  return candidate;
}

function isServerCanonical(message: ChatMessage) {
  return Boolean(message.serverMsgId) || hasAuthoritativeSeq(message);
}

function stableMessageTieBreaker(message: ChatMessage) {
  return message.serverMsgId ?? message.clientMsgId ?? message.id;
}

function uniqueStrings(values: string[]) {
  return values.filter((value, index) => value !== '' && values.indexOf(value) === index);
}
