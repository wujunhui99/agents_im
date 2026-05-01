import { ChevronLeft, SendHorizontal } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import type { ConversationSeqState, MessageApi, ServerMessage } from '../../api/messages';
import { createMessageApi } from '../../api/messages';
import { Avatar } from '../../components/ui/Avatar';
import { Badge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';
import { Card } from '../../components/ui/Card';
import { ListItem } from '../../components/ui/ListItem';
import { MessageBubble } from '../../components/ui/MessageBubble';
import { SearchBox } from '../../components/ui/SearchBox';
import { TextField } from '../../components/ui/TextField';
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
      <Card className="list-card conversation-list" role="list" aria-label="消息列表">
        {conversations.map((item) => (
          <div className="conversation-list-item" role="listitem" key={item.id}>
            <ListItem
              className="conversation-row conversation-button"
              onClick={() => onSelect(item.id)}
              leading={<Avatar label={item.avatar} color={item.color} />}
              headline={
                <span className="row-title-line">
                  <span>{item.title}</span>
                  <time>{item.time}</time>
                </span>
              }
              supportingText={item.preview}
              trailing={
                item.unread > 0 ? (
                  <Badge tone="error" className="unread-badge">
                    {item.unread}
                  </Badge>
                ) : null
              }
            />
          </div>
        ))}
      </Card>
    </div>
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
        <Button variant="icon" className="chat-back-button" aria-label="返回消息列表" onClick={onBack}>
          <ChevronLeft size={24} />
        </Button>
        <h2>{conversation.title}</h2>
      </header>
      <p className="inline-status" role="status">
        {status}
      </p>
      <div className="message-thread" role="log" aria-label="聊天消息">
        {sortedMessages.map((message) => (
          <article className={`message-row message-${message.direction}`} key={message.id}>
            <div className="message-body">
              <MessageBubble
                direction={message.direction}
                status={
                  message.direction === 'outgoing' ? (
                    <span className={`message-status message-status-${message.status}`}>{statusLabels[message.status]}</span>
                  ) : null
                }
              >
                {message.content}
              </MessageBubble>
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
      <TextField
        label="输入消息"
        hideLabel
        value={draft}
        placeholder="输入消息"
        onChange={(event) => setDraft(event.target.value)}
        fieldClassName="message-composer-field"
      />
      <Button className="message-send-button" type="submit" disabled={!trimmedDraft}>
        <SendHorizontal size={17} />
        <span>发送</span>
      </Button>
    </form>
  );
}

async function loadConversation(state: ConversationSeqState, currentUserId: string, messageApi: MessageApi): Promise<Conversation> {
  const pulled = await messageApi.pullMessages(state.conversationId, { fromSeq: 1, limit: 50, order: 'asc' });
  return conversationStateToView(state, currentUserId, pulled.messages);
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
