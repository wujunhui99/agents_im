import { ChevronLeft, MessageCircle, Search, SendHorizontal } from 'lucide-react';
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import type { ConversationSeqState, MessageApi, ServerMessage } from '../../api/messages';
import { createMessageApi } from '../../api/messages';
import type { UserApi, UserProfile } from '../../api/user';
import { createUserApi } from '../../api/user';
import { Avatar } from '../../components/ui/Avatar';
import { Badge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';
import { Card } from '../../components/ui/Card';
import { ListItem } from '../../components/ui/ListItem';
import { MessageBubble } from '../../components/ui/MessageBubble';
import { SearchBox } from '../../components/ui/SearchBox';
import { TextField } from '../../components/ui/TextField';
import type { ChatMessage, Conversation, MessageStatus } from '../../models/messages';
import { UNKNOWN_CONTACT_LABEL, accountTypeLabel, avatarText, profileDisplayName, profileIdentifier } from '../../utils/profileDisplay';

type MessagesPageProps = {
  currentUserId: string;
  messageApi?: MessageApi;
  userApi?: UserApi;
  startChatSignal?: number;
  pendingChatProfile?: UserProfile | null;
  onPendingChatConsumed?: () => void;
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
  pendingChatProfile = null,
  onPendingChatConsumed,
}: MessagesPageProps) {
  const [items, setItems] = useState<Conversation[]>([]);
  const [status, setStatus] = useState('正在加载会话');
  const [selectedConversationId, setSelectedConversationId] = useState<string | null>(null);
  const [showStartChat, setShowStartChat] = useState(false);
  const readSyncsInFlight = useRef<Set<string>>(new Set());
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
          setItems((current) => mergeLoadedConversations(current, conversations));
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

  useEffect(() => {
    if (!pendingChatProfile) {
      return;
    }
    handleStartChat(pendingChatProfile);
    onPendingChatConsumed?.();
  }, [pendingChatProfile, onPendingChatConsumed]);

  useEffect(() => {
    if (!selectedConversationId?.startsWith('draft-single:')) {
      return;
    }

    const peerId = draftPeerId(selectedConversationId);
    const loadedConversation = items.find(
      (conversation) => conversation.id !== selectedConversationId && isSingleConversationWithPeer(conversation, peerId),
    );
    if (loadedConversation) {
      setSelectedConversationId(loadedConversation.id);
    }
  }, [items, selectedConversationId]);

  useEffect(() => {
    if (!selectedConversation) {
      return;
    }
    if (selectedConversation.unread <= 0) {
      return;
    }

    const visibleMaxSeq = maxReadableSeq(selectedConversation.messages);
    if (visibleMaxSeq === undefined || visibleMaxSeq <= (selectedConversation.hasReadSeq ?? 0)) {
      return;
    }

    const syncKey = `${selectedConversation.id}:${visibleMaxSeq}`;
    if (readSyncsInFlight.current.has(syncKey)) {
      return;
    }

    readSyncsInFlight.current.add(syncKey);
    void messageApi
      .markRead(selectedConversation.id, { hasReadSeq: visibleMaxSeq })
      .then((response) => {
        const confirmedReadSeq = response.hasReadSeq ?? visibleMaxSeq;
        setItems((current) => markConversationRead(current, selectedConversation.id, confirmedReadSeq));
        setStatus(`已读到 ${confirmedReadSeq}`);
      })
      .catch((error) => {
        setStatus(error instanceof Error ? error.message : '标记已读失败');
      })
      .finally(() => {
        readSyncsInFlight.current.delete(syncKey);
      });
  }, [messageApi, selectedConversation]);

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
    if (conversationHasInFlightSend(selectedConversation)) {
      setStatus('上一条消息发送中');
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
        sending={conversationHasInFlightSend(selectedConversation)}
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
          <Button className="compact-command" type="button" onClick={onOpenStartChat}>
            <MessageCircle size={17} />
            <span>发起聊天</span>
          </Button>
        </div>
      ) : null}
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
              supportingText={
                <>
                  {item.previewOrigin === 'ai' ? <span className="conversation-origin-badge">AI/Agent</span> : null}
                  {item.previewOrigin === 'system' ? <span className="conversation-origin-badge conversation-origin-system">系统</span> : null}
                  {item.preview}
                </>
              }
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
        <Button type="button" className="text-command" variant="text" onClick={onClose}>
          关闭
        </Button>
      </div>
      <form className="identifier-search-form" onSubmit={handleSubmit}>
        <TextField
          label="按 identifier 搜索聊天对象"
          hideLabel
          placeholder="输入唯一 identifier"
          value={identifier}
          onChange={(event) => setIdentifier(event.target.value)}
          leadingIcon={<Search size={17} />}
          fieldClassName="search-box identifier-field"
        />
        <Button className="compact-command" type="submit" aria-label="搜索聊天对象" disabled={submitting}>
          <Search size={17} />
          <span>搜索</span>
        </Button>
      </form>
      <p className="inline-status" role="status">
        {status}
      </p>
      {result ? (
        <ListItem
          className="search-result"
          leading={<Avatar label={avatarText(profileDisplayName(result))} color="blue" />}
          headline={profileDisplayName(result)}
          supportingText={
            <span className="friend-supporting-lines">
              <span>{profileIdentifier(result) ?? '资料未同步'}</span>
              <span>{accountTypeLabel(result.account_type)}</span>
            </span>
          }
          trailing={
            <Button
              className="text-command"
              variant="tonal"
              size="small"
              aria-label={`发起聊天 ${profileDisplayName(result)}`}
              onClick={() => onStartChat(result)}
            >
              发起聊天
            </Button>
          }
        />
      ) : null}
    </section>
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
          <article
            className={`message-row message-${message.direction} message-origin-${message.messageOrigin}`}
            key={message.id}
            aria-label={messageAriaLabel(message)}
          >
            <div className="message-body">
{message.messageOrigin === 'ai' ? <span className="message-origin-badge">AI/Agent</span> : null}
              {message.messageOrigin === 'system' ? <span className="message-origin-badge message-origin-system">系统</span> : null}
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
<TextField
        label="输入消息"
        hideLabel
        value={draft}
        placeholder="输入消息"
        disabled={sending}
        onChange={(event) => setDraft(event.target.value)}
        fieldClassName="message-composer-field"
      />
      <Button className="message-send-button" type="submit" disabled={sending || !trimmedDraft}>
        <SendHorizontal size={17} />
        <span>{sending ? '发送中' : '发送'}</span>
      </Button>
    </form>
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
    maxSeq: 0,
    hasReadSeq: 0,
    color: 'blue',
    chatType: 'single',
    receiverId: profile.user_id,
    messages: [],
  };
}

function mergeLoadedConversations(current: Conversation[], loaded: Conversation[]) {
  if (loaded.length === 0) {
    return current;
  }

  const mergedLoaded = loaded.map((loadedConversation) => {
    const currentConversation = current.find((conversation) => conversationsRepresentSameThread(conversation, loadedConversation));
    return currentConversation ? mergeConversation(currentConversation, loadedConversation) : loadedConversation;
  });
  const preservedCurrentConversations = current.filter(
    (conversation) =>
      shouldPreserveMissingCurrentConversation(conversation) &&
      !mergedLoaded.some((loadedConversation) => conversationsRepresentSameThread(conversation, loadedConversation)),
  );

  return [...mergedLoaded, ...preservedCurrentConversations];
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
  const chatMessages = canonicalChatMessages(messages.map((message) => serverMessageToChatMessage(message, currentUserId)));
  const orderedMessages = orderedChatMessages(chatMessages);
  const lastMessage = orderedMessages[orderedMessages.length - 1] ?? serverLastMessage(state.lastMessage, currentUserId);
  const peerId = inferPeerId(state.conversationId, currentUserId, lastMessage);
  const isGroup = state.conversationId.startsWith('group:');
  const title = isGroup ? '群聊' : UNKNOWN_CONTACT_LABEL;
  return {
    id: state.conversationId,
    title,
    avatar: avatarText(title),
    preview: lastMessage?.content ?? '暂无消息',
    previewOrigin: lastMessage?.messageOrigin,
    time: state.maxSeqTime ? '刚刚' : '',
    unread: state.unreadCount ?? 0,
    maxSeq: state.maxSeq,
    hasReadSeq: state.hasReadSeq ?? 0,
    color: isGroup ? 'green' : 'blue',
    chatType: isGroup ? 'group' : 'single',
    receiverId: isGroup ? undefined : peerId,
    groupId: isGroup ? state.conversationId.replace(/^group:/, '') : undefined,
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
      previewOrigin: message.messageOrigin,
      time: '刚刚',
      unread: 0,
      maxSeq: nextConversationMaxSeq(conversation, message),
      messages: nextMessages,
    };
  });
}

function updateMessage(conversations: Conversation[], conversationId: string, messageId: string, nextMessage: ChatMessage) {
  return conversations.map((conversation) => {
    if (conversation.id !== conversationId && !conversation.messages.some((message) => message.id === messageId)) {
      return conversation;
    }

    const nextMessages = upsertCanonicalMessage(conversation.messages, messageId, nextMessage);
    return {
      ...conversation,
      preview: conversationPreview(nextMessages, nextMessage.content),
      time: '刚刚',
      maxSeq: nextConversationMaxSeq(conversation, nextMessage),
      messages: nextMessages,
    };
  });
}

function confirmSentMessage(conversations: Conversation[], conversationId: string, messageId: string, nextMessage: ChatMessage) {
  return conversations.map((conversation) => {
    if (
      conversation.id !== conversationId &&
      conversation.id !== nextMessage.conversationId &&
      !conversation.messages.some((message) => message.id === messageId)
    ) {
      return conversation;
    }

    return {
      ...conversation,
      id: nextMessage.conversationId,
      preview: nextMessage.content,
      previewOrigin: nextMessage.messageOrigin,
      time: '刚刚',
      unread: 0,
      maxSeq: nextConversationMaxSeq(conversation, nextMessage),
      hasReadSeq: nextMessage.seq ? Math.max(conversation.hasReadSeq ?? 0, nextMessage.seq) : conversation.hasReadSeq,
      receiverId: nextMessage.receiverId ?? conversation.receiverId,
      groupId: nextMessage.groupId ?? conversation.groupId,
      messages: conversation.messages.map((message) => (message.id === messageId ? nextMessage : message)),
    };
  });
}

function markConversationRead(conversations: Conversation[], conversationId: string, hasReadSeq: number) {
  return conversations.map((conversation) => {
    if (conversation.id !== conversationId) {
      return conversation;
    }

    return applyReadSeq(conversation, hasReadSeq);
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
    messageOrigin: 'human',
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
    messageOrigin: message.messageOrigin ?? 'human',
    agentAccountId: message.agentAccountId,
    triggerServerMsgId: message.triggerServerMsgId,
    agentRunId: message.agentRunId,
    allowRecursiveTrigger: message.allowRecursiveTrigger,
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

function draftPeerId(conversationId: string) {
  return conversationId.replace(/^draft-single:/, '');
}

function conversationHasInFlightSend(conversation: Conversation) {
  return conversation.messages.some((message) => message.status === 'sending' && !hasAuthoritativeSeq(message));
}

function conversationPreview(messages: ChatMessage[], fallback: string) {
  const orderedMessages = orderedChatMessages(messages);
  return orderedMessages[orderedMessages.length - 1]?.content ?? fallback;
}

function nextConversationMaxSeq(conversation: Conversation, message: ChatMessage) {
  const nextSeq = authoritativeSeq(message) ?? 0;
  return Math.max(conversation.maxSeq ?? 0, nextSeq);
}

function maxReadableSeq(messages: ChatMessage[]) {
  return orderedChatMessages(messages).reduce<number | undefined>((maxSeq, message) => {
    const seq = authoritativeSeq(message);
    if (seq === undefined) {
      return maxSeq;
    }
    return maxSeq === undefined ? seq : Math.max(maxSeq, seq);
  }, undefined);
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

function messageAriaLabel(message: ChatMessage) {
  if (message.messageOrigin === 'ai') {
    return `AI Agent 消息：${message.content}`;
  }
  if (message.messageOrigin === 'system') {
    return `系统消息：${message.content}`;
  }
  return message.direction === 'outgoing' ? `我发送的消息：${message.content}` : `收到的消息：${message.content}`;
}

function conversationsRepresentSameThread(left: Conversation, right: Conversation) {
  if (left.id === right.id) {
    return true;
  }
  if (left.chatType === 'single' && right.chatType === 'single') {
    return Boolean(left.receiverId && left.receiverId === right.receiverId);
  }
  if (left.chatType === 'group' && right.chatType === 'group') {
    return Boolean(left.groupId && left.groupId === right.groupId);
  }
  return false;
}

function isLocalConversation(conversation: Conversation) {
  return (
    conversation.id.startsWith('draft-') ||
    conversation.messages.some((message) => message.status !== 'sent' || !isServerCanonical(message))
  );
}

function shouldPreserveMissingCurrentConversation(conversation: Conversation) {
  return isLocalConversation(conversation) || conversation.messages.length > 0;
}

function mergeConversation(current: Conversation, loaded: Conversation): Conversation {
  const messages = canonicalChatMessages([...current.messages, ...loaded.messages]);
  const orderedMessages = orderedChatMessages(messages);
  const lastMessage = orderedMessages[orderedMessages.length - 1];
  const title = shouldKeepCurrentTitle(current, loaded) ? current.title : loaded.title;
  const maxSeq = Math.max(current.maxSeq ?? 0, loaded.maxSeq ?? 0, maxReadableSeq(messages) ?? 0);
  const hasReadSeq = Math.max(current.hasReadSeq ?? 0, loaded.hasReadSeq ?? 0);
  const unread = unreadAfterRead({ ...loaded, maxSeq, messages }, hasReadSeq);

  return {
    ...loaded,
    title,
    avatar: title === current.title ? current.avatar : loaded.avatar,
    preview: lastMessage?.content ?? loaded.preview,
    previewOrigin: lastMessage?.messageOrigin ?? loaded.previewOrigin,
    time: lastMessage ? '刚刚' : loaded.time,
    unread,
    maxSeq,
    hasReadSeq,
    messages,
  };
}

function applyReadSeq(conversation: Conversation, hasReadSeq: number): Conversation {
  const nextHasReadSeq = Math.max(conversation.hasReadSeq ?? 0, hasReadSeq);
  return {
    ...conversation,
    hasReadSeq: nextHasReadSeq,
    unread: unreadAfterRead(conversation, nextHasReadSeq),
  };
}

function unreadAfterRead(conversation: Conversation, hasReadSeq: number) {
  const maxSeq = conversation.maxSeq ?? maxReadableSeq(conversation.messages) ?? 0;
  if (hasReadSeq >= maxSeq) {
    return 0;
  }

  const previousReadSeq = conversation.hasReadSeq ?? 0;
  const readDelta = Math.max(0, hasReadSeq - previousReadSeq);
  return Math.max(0, conversation.unread - readDelta);
}

function shouldKeepCurrentTitle(current: Conversation, loaded: Conversation) {
  if (!current.id.startsWith('draft-')) {
    return false;
  }
  return Boolean(current.title && current.title !== loaded.title && current.title !== current.receiverId);
}

function isSingleConversationWithPeer(conversation: Conversation, peerId: string) {
  return conversation.chatType === 'single' && conversation.receiverId === peerId;
}
