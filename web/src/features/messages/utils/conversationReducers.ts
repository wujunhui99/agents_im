import type { MessageApi } from '../../../api/messages';
import type { ChatMessage, Conversation } from '../../../models/messages';
import type { PendingMessageInput } from '../types';
import { attachGroupSenderDisplayName } from './groupUtils';
import { messageDisplayText } from './mediaUtils';
import { canonicalChatMessages, maxReadableSeq, nextConversationMaxSeq, upsertCanonicalMessage } from './messageOrdering';
import { serverMessageToChatMessage } from './serverMessageParser';
import {
  conversationPreview,
  conversationsRepresentSameMessageThread,
  liveMessagePeerTarget,
  liveMessageToConversation,
  requiredField,
} from './conversationUtils';

// ── Read-seq ────────────────────────────────────────────────────────────────

export function applyReadSeq(conversation: Conversation, hasReadSeq: number): Conversation {
  const nextHasReadSeq = Math.max(conversation.hasReadSeq ?? 0, hasReadSeq);
  return { ...conversation, hasReadSeq: nextHasReadSeq, unread: unreadAfterRead(conversation, nextHasReadSeq) };
}

function unreadAfterRead(conversation: Conversation, hasReadSeq: number) {
  const maxSeq = conversation.maxSeq ?? maxReadableSeq(conversation.messages) ?? 0;
  if (hasReadSeq >= maxSeq) return 0;
  const previousReadSeq = conversation.hasReadSeq ?? 0;
  const readDelta = Math.max(0, hasReadSeq - previousReadSeq);
  return Math.max(0, conversation.unread - readDelta);
}

export function markConversationRead(conversations: Conversation[], conversationId: string, hasReadSeq: number) {
  return conversations.map((c) => (c.id !== conversationId ? c : applyReadSeq(c, hasReadSeq)));
}

// ── State updaters ──────────────────────────────────────────────────────────

export function upsertStartedConversation(conversations: Conversation[], existingConversationId: string | undefined, draft: Conversation) {
  if (existingConversationId) {
    return conversations.map((c) =>
      c.id === existingConversationId
        ? { ...c, title: draft.title, avatar: draft.avatar, avatarUrl: draft.avatarUrl, receiverId: draft.receiverId }
        : c,
    );
  }
  if (conversations.some((c) => c.id === draft.id)) {
    return conversations.map((c) => (c.id === draft.id ? { ...c, ...draft, messages: c.messages } : c));
  }
  return [draft, ...conversations];
}

export function appendMessage(conversations: Conversation[], conversationId: string, message: ChatMessage) {
  return conversations.map((conversation) => {
    if (conversation.id !== conversationId) return conversation;
    const nextMessages = canonicalChatMessages([...conversation.messages, message]);
    return {
      ...conversation,
      preview: conversationPreview(nextMessages, messageDisplayText(message)),
      previewOrigin: message.messageOrigin,
      time: '刚刚',
      unread: 0,
      maxSeq: nextConversationMaxSeq(conversation, message),
      messages: nextMessages,
    };
  });
}

export function updateMessage(conversations: Conversation[], conversationId: string, messageId: string, nextMessage: ChatMessage) {
  return conversations.map((conversation) => {
    if (conversation.id !== conversationId && !conversation.messages.some((m) => m.id === messageId)) return conversation;
    const nextMessages = upsertCanonicalMessage(conversation.messages, messageId, nextMessage);
    return {
      ...conversation,
      preview: conversationPreview(nextMessages, messageDisplayText(nextMessage)),
      time: '刚刚',
      maxSeq: nextConversationMaxSeq(conversation, nextMessage),
      messages: nextMessages,
    };
  });
}

export function upsertLiveServerMessage(conversations: Conversation[], message: ChatMessage) {
  let matched = false;
  const nextConversations = conversations.map((conversation) => {
    if (!conversationsRepresentSameMessageThread(conversation, message)) return conversation;
    matched = true;
    const nextMessage = attachGroupSenderDisplayName(message, conversation.groupMemberDisplayNames);
    const nextMessages = upsertCanonicalMessage(conversation.messages, nextMessage.id, nextMessage);
    return {
      ...conversation,
      id: nextMessage.conversationId,
      preview: conversationPreview(nextMessages, messageDisplayText(nextMessage)),
      previewOrigin: nextMessage.messageOrigin,
      time: '刚刚',
      unread: nextMessage.direction === 'incoming' ? conversation.unread + 1 : conversation.unread,
      maxSeq: nextConversationMaxSeq(conversation, nextMessage),
      receiverId: liveMessagePeerTarget(nextMessage) ?? conversation.receiverId,
      groupId: nextMessage.groupId ?? conversation.groupId,
      messages: nextMessages,
    };
  });

  if (matched) return nextConversations;
  return [liveMessageToConversation(message), ...conversations];
}

export function confirmSentMessage(conversations: Conversation[], conversationId: string, messageId: string, nextMessage: ChatMessage) {
  return conversations.map((conversation) => {
    if (
      conversation.id !== conversationId &&
      conversation.id !== nextMessage.conversationId &&
      !conversation.messages.some((m) => m.id === messageId)
    ) {
      return conversation;
    }
    return {
      ...conversation,
      id: nextMessage.conversationId,
      preview: messageDisplayText(nextMessage),
      previewOrigin: nextMessage.messageOrigin,
      time: '刚刚',
      unread: 0,
      maxSeq: nextConversationMaxSeq(conversation, nextMessage),
      hasReadSeq: conversation.hasReadSeq,
      receiverId: nextMessage.receiverId ?? conversation.receiverId,
      groupId: nextMessage.groupId ?? conversation.groupId,
      messages: conversation.messages.map((m) => (m.id === messageId ? nextMessage : m)),
    };
  });
}

// ── Message creation & send ─────────────────────────────────────────────────

export function createPendingMessage(conversation: Conversation, messageInput: PendingMessageInput, currentUserId: string): ChatMessage {
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
    contentType: messageInput.contentType,
    content: messageInput.content,
    messageOrigin: 'human',
    sendTime: now,
    direction: 'outgoing',
    status: 'sending',
  };
}

export function sendMessageWithApi(messageApi: MessageApi, message: ChatMessage): Promise<ChatMessage> {
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

export function sendErrorMessage(error: unknown, chatType: Conversation['chatType']) {
  const message = error instanceof Error ? error.message : '';
  if (chatType === 'group' && /group member|群|forbidden|permission/i.test(message)) {
    return '没有群聊权限，无法发送消息';
  }
  return message || '发送消息失败';
}
