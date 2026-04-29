import { ChevronLeft, Search, SendHorizontal } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { ChatMessage, Conversation, MessageStatus } from '../../models/messages';
import { cloneMockConversations, currentUserId } from './mockConversations';

type SendMessageHandler = (message: ChatMessage) => Promise<ChatMessage>;

type MessagesPageProps = {
  initialConversations?: Conversation[];
  sendMessage?: SendMessageHandler;
};

const statusLabels: Record<MessageStatus, string> = {
  sending: '发送中',
  sent: '已发送',
  failed: '发送失败',
};

export function MessagesPage({
  initialConversations = cloneMockConversations(),
  sendMessage = sendMessageWithMockAck,
}: MessagesPageProps) {
  const [conversations, setConversations] = useState(() => cloneConversations(initialConversations));
  const [selectedConversationId, setSelectedConversationId] = useState<string | null>(null);
  const selectedConversation = conversations.find((conversation) => conversation.id === selectedConversationId) ?? null;

  function handleSend(content: string) {
    if (!selectedConversation) {
      return;
    }

    const pendingMessage = createPendingMessage(selectedConversation, content);
    setConversations((current) => appendMessage(current, selectedConversation.id, pendingMessage));

    void sendMessage(pendingMessage)
      .then((sentMessage) => {
        setConversations((current) => updateMessage(current, selectedConversation.id, pendingMessage.id, sentMessage));
      })
      .catch(() => {
        setConversations((current) =>
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

  return (
    <ConversationList
      conversations={conversations}
      onSelect={(conversationId) => setSelectedConversationId(conversationId)}
    />
  );
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
            <button
              type="button"
              className="conversation-row conversation-button"
              onClick={() => onSelect(item.id)}
            >
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
                <span className={`message-status message-status-${message.status}`}>
                  {statusLabels[message.status]}
                </span>
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
      <input
        aria-label="输入消息"
        value={draft}
        placeholder="输入消息"
        onChange={(event) => setDraft(event.target.value)}
      />
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

function updateMessage(
  conversations: Conversation[],
  conversationId: string,
  messageId: string,
  nextMessage: ChatMessage,
) {
  return conversations.map((conversation) => {
    if (conversation.id !== conversationId) {
      return conversation;
    }

    return {
      ...conversation,
      messages: conversation.messages.map((message) => (message.id === messageId ? nextMessage : message)),
    };
  });
}

function createPendingMessage(conversation: Conversation, content: string): ChatMessage {
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
