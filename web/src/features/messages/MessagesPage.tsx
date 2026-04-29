import { ChevronLeft, Search, SendHorizontal } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import type { MessageApi, ServerMessage } from '../../api/messages';
import { createMessageApi } from '../../api/messages';
import type { ChatMessage, Conversation, MessageStatus } from '../../models/messages';
import { cloneMockConversations, currentUserId as mockCurrentUserId } from './mockConversations';

type SendMessageHandler = (message: ChatMessage) => Promise<ChatMessage>;

type ConversationSeed = Pick<Conversation, 'id' | 'title' | 'chatType'> &
  Partial<Pick<Conversation, 'avatar' | 'color' | 'receiverId' | 'groupId'>>;

type MessagesPageProps = {
  mode?: 'mock' | 'real';
  currentUserId?: string;
  conversations?: ConversationSeed[];
  initialConversations?: Conversation[];
  messageApi?: MessageApi;
  sendMessage?: SendMessageHandler;
};

const statusLabels: Record<MessageStatus, string> = {
  sending: '发送中',
  sent: '已发送',
  failed: '发送失败',
};

export function MessagesPage({
  mode = 'mock',
  currentUserId = mockCurrentUserId,
  conversations,
  initialConversations,
  messageApi = createMessageApi(),
  sendMessage,
}: MessagesPageProps) {
  const [items, setItems] = useState(() =>
    mode === 'real'
      ? conversationSeedsToViews(conversations ?? [], currentUserId)
      : cloneConversations(initialConversations ?? cloneMockConversations()),
  );
  const [selectedConversationId, setSelectedConversationId] = useState<string | null>(null);
  const selectedConversation = items.find((conversation) => conversation.id === selectedConversationId) ?? null;

  useEffect(() => {
    if (mode !== 'real' || !conversations?.length) {
      return;
    }

    const conversationSeeds = conversations;
    let cancelled = false;
    async function loadConversations() {
      const nextItems = await Promise.all(
        conversationSeeds.map(async (conversation) => {
          const pulled = await messageApi.pullMessages(conversation.id, { fromSeq: 1, limit: 50, order: 'asc' });
          return conversationSeedToView(conversation, currentUserId, pulled.messages);
        }),
      );
      if (!cancelled) {
        setItems(nextItems);
      }
    }

    void loadConversations();
    return () => {
      cancelled = true;
    };
  }, [mode, conversations, currentUserId, messageApi]);

  function handleSend(content: string) {
    if (!selectedConversation) {
      return;
    }

    const pendingMessage = createPendingMessage(selectedConversation, content, currentUserId);
    setItems((current) => appendMessage(current, selectedConversation.id, pendingMessage));

    const sender =
      mode === 'real'
        ? sendMessageWithApi(messageApi, pendingMessage)
        : (sendMessage ?? sendMessageWithMockAck)(pendingMessage);

    void sender
      .then((sentMessage) => {
        setItems((current) => updateMessage(current, selectedConversation.id, pendingMessage.id, sentMessage));
      })
      .catch(() => {
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
      />
    );
  }

  return <ConversationList conversations={items} onSelect={(conversationId) => setSelectedConversationId(conversationId)} />;
}

function ConversationList({
  conversations,
  onSelect,
}: {
  conversations: Conversation[];
  onSelect: (conversationId: string) => void;
}) {
  return (
    <div className="page-stack">
      <SearchBox placeholder="搜索" />
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
}: {
  conversation: Conversation;
  onBack: () => void;
  onSend: (content: string) => void;
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

function cloneConversations(conversations: Conversation[]) {
  return conversations.map((conversation) => ({
    ...conversation,
    messages: conversation.messages.map((message) => ({ ...message })),
  }));
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

function sendMessageWithMockAck(message: ChatMessage): Promise<ChatMessage> {
  return new Promise((resolve, reject) => {
    window.setTimeout(() => {
      if (message.content.includes('/fail')) {
        reject(new Error('mock send failed'));
        return;
      }

      resolve({
        ...message,
        serverMsgId: `mock-${message.clientMsgId}`,
        createdAt: Date.now(),
        status: 'sent',
      });
    }, 20);
  });
}

function conversationSeedsToViews(seeds: ConversationSeed[], currentUserId: string) {
  return seeds.map((seed) => conversationSeedToView(seed, currentUserId, []));
}

function conversationSeedToView(seed: ConversationSeed, currentUserId: string, messages: ServerMessage[]): Conversation {
  const chatMessages = messages.map((message) => serverMessageToChatMessage(message, currentUserId));
  const lastMessage = chatMessages[chatMessages.length - 1];
  return {
    id: seed.id,
    title: seed.title,
    avatar: seed.avatar ?? seed.title.slice(0, 2).toUpperCase(),
    preview: lastMessage?.content ?? '暂无消息',
    time: lastMessage ? '刚刚' : '',
    unread: 0,
    color: seed.color ?? 'blue',
    chatType: seed.chatType,
    receiverId: seed.receiverId,
    groupId: seed.groupId,
    messages: chatMessages,
  };
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

function requiredField(value: string | undefined, fieldName: string) {
  if (!value) {
    throw new Error(`${fieldName} is required`);
  }
  return value;
}
