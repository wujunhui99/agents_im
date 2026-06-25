import type { ContactsApi, Friendship } from '../../../api/contacts';
import type { Group, GroupMember, GroupsApi } from '../../../api/groups';
import type { ConversationSeqState, MessageApi, ServerMessage } from '../../../api/messages';
import type { ChatMessage, Conversation } from '../../../models/messages';
import type { UserProfile } from '../../../api/user';
import { friendshipToUserProfile } from '../../../components/ContactsPage';
import { UNKNOWN_CONTACT_LABEL, avatarText, profileDisplayName } from '../../../utils/profileDisplay';
import { attachGroupSenderDisplayName, groupMemberDisplayNameMap } from './groupUtils';
import { messageDisplayText } from './mediaUtils';
import { canonicalChatMessages, hasAuthoritativeSeq, isServerCanonical, maxReadableSeq, orderedChatMessages } from './messageOrdering';
import { serverLastMessage, serverMessageToChatMessage } from './serverMessageParser';

// ── ID helpers ─────────────────────────────────────────────────────────────

export function draftConversationId(userId: string) {
  return `draft-single:${userId}`;
}

export function groupConversationId(groupId: string) {
  return `group:${groupId}`;
}

export function draftPeerId(conversationId: string) {
  return conversationId.replace(/^draft-single:/, '');
}

export function requiredField(value: string | undefined, fieldName: string) {
  if (!value) throw new Error(`${fieldName} is required`);
  return value;
}

export function isSingleConversationWithPeer(conversation: Conversation, peerId: string) {
  return conversation.chatType === 'single' && conversation.receiverId === peerId;
}

export function canUseNativeWebSocket() {
  return typeof window !== 'undefined' && typeof window.WebSocket === 'function';
}

// ── Conversation predicates ─────────────────────────────────────────────────

export function conversationHasInFlightSend(conversation: Conversation) {
  return conversation.messages.some((message) => message.status === 'sending' && !hasAuthoritativeSeq(message));
}

export function conversationSupportsAIHosting(conversation: Conversation) {
  return conversation.chatType === 'single' && !conversation.id.startsWith('draft-single:') && conversation.id.startsWith('single:');
}

export function conversationsRepresentSameThread(left: Conversation, right: Conversation) {
  if (left.id === right.id) return true;
  if (left.chatType === 'single' && right.chatType === 'single') {
    return Boolean(left.receiverId && left.receiverId === right.receiverId);
  }
  if (left.chatType === 'group' && right.chatType === 'group') {
    return Boolean(left.groupId && left.groupId === right.groupId);
  }
  return false;
}

export function conversationsRepresentSameMessageThread(conversation: Conversation, message: ChatMessage) {
  if (conversation.id === message.conversationId) return true;
  if (conversation.chatType === 'single' && message.chatType === 'single') {
    return Boolean(conversation.receiverId && (conversation.receiverId === message.senderId || conversation.receiverId === message.receiverId));
  }
  if (conversation.chatType === 'group' && message.chatType === 'group') {
    return Boolean(conversation.groupId && conversation.groupId === message.groupId);
  }
  return false;
}

export function isLocalConversation(conversation: Conversation) {
  return (
    conversation.id.startsWith('draft-') ||
    conversation.messages.some((message) => message.status !== 'sent' || !isServerCanonical(message))
  );
}

export function shouldPreserveMissingCurrentConversation(conversation: Conversation) {
  return isLocalConversation(conversation) || conversation.messages.length > 0;
}

// ── View model creation ─────────────────────────────────────────────────────

export function inferPeerId(conversationId: string, currentUserId: string, lastMessage?: ChatMessage) {
  if (lastMessage) {
    if (lastMessage.senderId && lastMessage.senderId !== currentUserId) return lastMessage.senderId;
    if (lastMessage.receiverId && lastMessage.receiverId !== currentUserId) return lastMessage.receiverId;
  }
  if (conversationId.startsWith('single:')) {
    return conversationId.replace(/^single:/, '').split(':').find((part) => part && part !== currentUserId);
  }
  return undefined;
}

export function conversationPreview(messages: ChatMessage[], fallback: string) {
  const ordered = orderedChatMessages(messages);
  const last = ordered[ordered.length - 1];
  return last ? messageDisplayText(last) : fallback;
}

export function liveMessagePeerTarget(message: ChatMessage) {
  if (message.chatType !== 'single') return undefined;
  return message.direction === 'incoming' ? message.senderId : message.receiverId;
}

export function liveMessageToConversation(message: ChatMessage): Conversation {
  const isGroup = message.chatType === 'group';
  const title = isGroup ? '群聊' : UNKNOWN_CONTACT_LABEL;
  return {
    id: message.conversationId,
    title,
    avatar: avatarText(title),
    avatarUrl: undefined,
    preview: messageDisplayText(message),
    previewOrigin: message.messageOrigin,
    time: '刚刚',
    unread: message.direction === 'incoming' ? 1 : 0,
    maxSeq: message.seq,
    hasReadSeq: 0,
    color: isGroup ? 'green' : 'blue',
    chatType: message.chatType,
    receiverId: isGroup ? undefined : liveMessagePeerTarget(message),
    groupId: message.groupId,
    messages: [message],
  };
}

export function userProfileToDraftConversation(profile: UserProfile): Conversation {
  const title = profileDisplayName(profile);
  return {
    id: draftConversationId(profile.user_id),
    title,
    avatar: avatarText(title),
    avatarUrl: profile.avatar_url,
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

export function groupToConversation(group: Group): Conversation {
  const title = group.name || '群聊';
  return {
    id: groupConversationId(group.group_id),
    title,
    avatar: avatarText(title),
    avatarUrl: group.avatar_url,
    preview: '暂无消息',
    time: '',
    unread: 0,
    maxSeq: 0,
    hasReadSeq: 0,
    color: 'green',
    chatType: 'group',
    groupId: group.group_id,
    messages: [],
  };
}

export function conversationStateToView(state: ConversationSeqState, currentUserId: string, messages: ServerMessage[]): Conversation {
  const chatMessages = canonicalChatMessages(messages.map((m) => serverMessageToChatMessage(m, currentUserId)));
  const ordered = orderedChatMessages(chatMessages);
  const lastMessage = ordered[ordered.length - 1] ?? serverLastMessage(state.lastMessage, currentUserId);
  const peerId = inferPeerId(state.conversationId, currentUserId, lastMessage);
  const isGroup = state.conversationId.startsWith('group:');
  const title = isGroup ? '群聊' : UNKNOWN_CONTACT_LABEL;
  return {
    id: state.conversationId,
    title,
    avatar: avatarText(title),
    avatarUrl: undefined,
    preview: lastMessage ? messageDisplayText(lastMessage) : '暂无消息',
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

// ── Hydration ──────────────────────────────────────────────────────────────

export function hydrateConversationTitles(conversations: Conversation[], friendProfiles: Map<string, UserProfile>) {
  if (friendProfiles.size === 0) return conversations;
  return conversations.map((conversation) => {
    if (conversation.chatType !== 'single' || !conversation.receiverId) return conversation;
    const profile = friendProfiles.get(conversation.receiverId);
    if (!profile) return conversation;
    const title = profileDisplayName(profile);
    return { ...conversation, title, avatar: avatarText(title), avatarUrl: profile.avatar_url };
  });
}

export function hydrateGroupConversationMembers(conversations: Conversation[], group: Group, members: GroupMember[] | Record<string, string>) {
  return conversations.map((conversation) => {
    if (conversation.chatType !== 'group' || conversation.groupId !== group.group_id) return conversation;
    return applyGroupMetadata(conversation, group, members);
  });
}

export function applyGroupMetadata(conversation: Conversation, group: Group, members: GroupMember[] | Record<string, string>): Conversation {
  const memberDisplayNames = Array.isArray(members) ? groupMemberDisplayNameMap(members) : members;
  const title = group.name || conversation.title || '群聊';
  return {
    ...conversation,
    title,
    avatar: avatarText(title),
    avatarUrl: group.avatar_url,
    groupId: group.group_id,
    groupMemberDisplayNames: memberDisplayNames,
    messages: conversation.messages.map((message) => attachGroupSenderDisplayName(message, memberDisplayNames)),
  };
}

// ── Merge helpers ──────────────────────────────────────────────────────────

function shouldKeepCurrentTitle(current: Conversation, loaded: Conversation) {
  if (!current.id.startsWith('draft-')) return false;
  return Boolean(current.title && current.title !== loaded.title && current.title !== current.receiverId);
}

function unreadAfterRead(conversation: Conversation, hasReadSeq: number) {
  const maxSeq = conversation.maxSeq ?? maxReadableSeq(conversation.messages) ?? 0;
  if (hasReadSeq >= maxSeq) return 0;
  const previousReadSeq = conversation.hasReadSeq ?? 0;
  const readDelta = Math.max(0, hasReadSeq - previousReadSeq);
  return Math.max(0, conversation.unread - readDelta);
}

export function mergeConversation(current: Conversation, loaded: Conversation): Conversation {
  const groupMemberDisplayNames =
    current.chatType === 'group' || loaded.chatType === 'group'
      ? { ...(loaded.groupMemberDisplayNames ?? {}), ...(current.groupMemberDisplayNames ?? {}) }
      : undefined;
  const messages = canonicalChatMessages([...current.messages, ...loaded.messages]).map((message) =>
    attachGroupSenderDisplayName(message, groupMemberDisplayNames),
  );
  const ordered = orderedChatMessages(messages);
  const lastMessage = ordered[ordered.length - 1];
  const title = shouldKeepCurrentTitle(current, loaded) ? current.title : loaded.title;
  const maxSeq = Math.max(current.maxSeq ?? 0, loaded.maxSeq ?? 0, maxReadableSeq(messages) ?? 0);
  const hasReadSeq = Math.max(current.hasReadSeq ?? 0, loaded.hasReadSeq ?? 0);
  const unread = unreadAfterRead({ ...loaded, maxSeq, messages }, hasReadSeq);

  return {
    ...loaded,
    title,
    avatar: title === current.title ? current.avatar : loaded.avatar,
    preview: lastMessage ? messageDisplayText(lastMessage) : loaded.preview,
    avatarUrl: title === current.title ? current.avatarUrl : loaded.avatarUrl,
    previewOrigin: lastMessage?.messageOrigin ?? loaded.previewOrigin,
    time: lastMessage ? '刚刚' : loaded.time,
    unread,
    maxSeq,
    hasReadSeq,
    groupMemberDisplayNames,
    messages,
  };
}

export function mergeLoadedConversations(current: Conversation[], loaded: Conversation[]) {
  if (loaded.length === 0) return current;
  const mergedLoaded = loaded.map((loadedConversation) => {
    const currentConversation = current.find((c) => conversationsRepresentSameThread(c, loadedConversation));
    return currentConversation ? mergeConversation(currentConversation, loadedConversation) : loadedConversation;
  });
  const preservedCurrent = current.filter(
    (c) => shouldPreserveMissingCurrentConversation(c) && !mergedLoaded.some((l) => conversationsRepresentSameThread(c, l)),
  );
  return [...mergedLoaded, ...preservedCurrent];
}

// ── API-level async helpers ─────────────────────────────────────────────────

export function isAcceptedFriendship(friendship: Friendship) {
  return friendship.status === 'accepted' || friendship.status === 'active' || friendship.is_friend;
}

export async function loadAcceptedFriendProfileMap(contactsApi: ContactsApi) {
  try {
    const response = await contactsApi.listFriends();
    const friendships = response.friends ?? [];
    return friendships.reduce<Map<string, UserProfile>>((profiles, friendship) => {
      if (!isAcceptedFriendship(friendship)) return profiles;
      const profile = friendshipToUserProfile(friendship);
      profiles.set(profile.user_id, profile);
      return profiles;
    }, new Map<string, UserProfile>());
  } catch {
    return new Map<string, UserProfile>();
  }
}

export async function loadConversation(
  state: ConversationSeqState,
  currentUserId: string,
  messageApi: MessageApi,
  groupsApi: GroupsApi,
): Promise<Conversation> {
  const previewMessages = state.lastMessage ? [state.lastMessage] : [];
  const previewConversation = conversationStateToView(state, currentUserId, previewMessages);
  const messagesPromise = messageApi.pullMessages(state.conversationId, { fromSeq: 1, limit: 50, order: 'asc' });

  if (previewConversation.chatType !== 'group' || !previewConversation.groupId) {
    const pulled = await messagesPromise;
    return conversationStateToView(state, currentUserId, pulled.messages);
  }

  try {
    const [pulled, group, members] = await Promise.all([
      messagesPromise,
      groupsApi.getGroup(previewConversation.groupId),
      groupsApi.listMembers(previewConversation.groupId),
    ]);
    const conversation = conversationStateToView(state, currentUserId, pulled.messages);
    return applyGroupMetadata(conversation, group, members.members ?? []);
  } catch {
    const pulled = await messagesPromise;
    return conversationStateToView(state, currentUserId, pulled.messages);
  }
}
