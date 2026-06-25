import type { ServerMessage } from '../../../api/messages';
import type { ChatMessage, MessageContentType } from '../../../models/messages';
import type { WebSocketServerEvent } from '../../../api/websocketClient';

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

export function parseMessageContentType(contentType: string): MessageContentType {
  return contentType === 'image' || contentType === 'file' ? contentType : 'text';
}

export function normalizeMessageContent(contentType: ServerMessage['contentType'], content: string) {
  if (contentType !== 'text') return content;
  try {
    const parsed = JSON.parse(content) as unknown;
    if (isRecord(parsed) && typeof parsed.text === 'string') return parsed.text;
  } catch {
    return content;
  }
  return content;
}

export function serverMessageToChatMessage(message: ServerMessage, currentUserId: string): ChatMessage {
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
    content: normalizeMessageContent(message.contentType, message.content),
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

export function serverLastMessage(message: ServerMessage | undefined, currentUserId: string) {
  return message ? serverMessageToChatMessage(message, currentUserId) : undefined;
}

export function webSocketEventToServerMessage(event: WebSocketServerEvent): ServerMessage | null {
  if (event.type !== 'message_received' || !isRecord(event.data)) return null;
  const data = event.data;
  const serverMsgId = stringField(data, 'serverMsgId', 'server_msg_id');
  const conversationId = stringField(data, 'conversationId', 'conversation_id');
  const senderId = stringField(data, 'senderId', 'sender_id');
  const chatType = stringField(data, 'chatType', 'chat_type');
  const contentType = stringField(data, 'contentType', 'content_type');
  const content = stringField(data, 'content');
  const seq = numberField(data, 'seq');
  if (!serverMsgId || !conversationId || !senderId || !chatType || !contentType || content === undefined || seq === undefined) {
    return null;
  }
  const parsedChatType = chatType === 'group' ? 'group' : 'single';
  const groupId =
    stringField(data, 'groupId', 'group_id') ??
    (parsedChatType === 'group' ? conversationId.replace(/^group:/, '') : undefined);

  return {
    serverMsgId,
    clientMsgId: stringField(data, 'clientMsgId', 'client_msg_id') ?? '',
    conversationId,
    seq,
    senderId,
    receiverId: stringField(data, 'receiverId', 'receiver_id'),
    groupId,
    chatType: parsedChatType,
    contentType: parseMessageContentType(contentType),
    content,
    messageOrigin: messageOriginField(data),
    agentAccountId: stringField(data, 'agentAccountId', 'agent_account_id'),
    triggerServerMsgId: stringField(data, 'triggerServerMsgId', 'trigger_server_msg_id'),
    agentRunId: stringField(data, 'agentRunId', 'agent_run_id'),
    allowRecursiveTrigger: booleanField(data, 'allowRecursiveTrigger', 'allow_recursive_trigger'),
    sendTime: numberField(data, 'sendTime', 'send_time') ?? Date.now(),
    createdAt: numberField(data, 'createdAt', 'created_at') ?? Date.now(),
  };
}

export function conversationBelongsToCurrentUser(message: ServerMessage, currentUserId: string) {
  if (message.chatType === 'group') return true;
  return message.senderId === currentUserId || message.receiverId === currentUserId;
}

function stringField(value: Record<string, unknown>, ...keys: string[]) {
  for (const key of keys) {
    const field = value[key];
    if (typeof field === 'string') return field;
  }
  return undefined;
}

function numberField(value: Record<string, unknown>, ...keys: string[]) {
  for (const key of keys) {
    const field = value[key];
    if (typeof field === 'number' && Number.isFinite(field)) return field;
  }
  return undefined;
}

function booleanField(value: Record<string, unknown>, ...keys: string[]) {
  for (const key of keys) {
    const field = value[key];
    if (typeof field === 'boolean') return field;
  }
  return undefined;
}

function messageOriginField(value: Record<string, unknown>) {
  const origin = stringField(value, 'messageOrigin', 'message_origin');
  return origin === 'ai' || origin === 'system' || origin === 'human' ? origin : undefined;
}
