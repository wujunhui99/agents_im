import type { ChatMessage, Conversation } from '../../../models/messages';

type MessageEntry = {
  message: ChatMessage;
  index: number;
};

export function canonicalMessageEntries(messages: ChatMessage[]) {
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

export function canonicalChatMessages(messages: ChatMessage[]) {
  return canonicalMessageEntries(messages).map((entry) => entry.message);
}

export function orderedChatMessages(messages: ChatMessage[]) {
  return canonicalMessageEntries(messages)
    .sort(compareMessageEntries)
    .map((entry) => entry.message);
}

function compareMessageEntries(left: MessageEntry, right: MessageEntry) {
  const leftSeq = authoritativeSeq(left.message);
  const rightSeq = authoritativeSeq(right.message);
  if (leftSeq !== undefined && rightSeq !== undefined && leftSeq !== rightSeq) {
    return leftSeq - rightSeq;
  }
  if (leftSeq !== undefined && rightSeq === undefined) return -1;
  if (leftSeq === undefined && rightSeq !== undefined) return 1;
  if (leftSeq === undefined && rightSeq === undefined) return left.index - right.index;
  return stableMessageTieBreaker(left.message).localeCompare(stableMessageTieBreaker(right.message));
}

export function upsertCanonicalMessage(messages: ChatMessage[], pendingMessageId: string, nextMessage: ChatMessage) {
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

export function hasAuthoritativeSeq(message: ChatMessage): message is ChatMessage & { seq: number } {
  return authoritativeSeq(message) !== undefined;
}

export function authoritativeSeq(message: ChatMessage) {
  return typeof message.seq === 'number' && Number.isFinite(message.seq) && message.seq > 0 ? message.seq : undefined;
}

export function isServerCanonical(message: ChatMessage) {
  return Boolean(message.serverMsgId) || hasAuthoritativeSeq(message);
}

export function maxReadableSeq(messages: ChatMessage[]) {
  return orderedChatMessages(messages).reduce<number | undefined>((maxSeq, message) => {
    const seq = authoritativeSeq(message);
    if (seq === undefined) return maxSeq;
    return maxSeq === undefined ? seq : Math.max(maxSeq, seq);
  }, undefined);
}

export function nextConversationMaxSeq(conversation: Conversation, message: ChatMessage) {
  const nextSeq = authoritativeSeq(message) ?? 0;
  return Math.max(conversation.maxSeq ?? 0, nextSeq);
}

export function shouldRenderDateSeparator(message: ChatMessage, previousMessage: ChatMessage | undefined) {
  return !previousMessage || messageLocalDayKey(message.sendTime) !== messageLocalDayKey(previousMessage.sendTime);
}

export function formatMessageTime(timestamp: number) {
  const date = new Date(timestamp);
  return `${padTwoDigits(date.getHours())}:${padTwoDigits(date.getMinutes())}`;
}

export function formatMessageDate(timestamp: number) {
  const date = new Date(timestamp);
  return `${date.getFullYear()}年${date.getMonth() + 1}月${date.getDate()}日`;
}

function messageLocalDayKey(timestamp: number) {
  const date = new Date(timestamp);
  return `${date.getFullYear()}-${padTwoDigits(date.getMonth() + 1)}-${padTwoDigits(date.getDate())}`;
}

function padTwoDigits(value: number) {
  return value.toString().padStart(2, '0');
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
  if (candidateCanonical && !currentCanonical) return candidate;
  if (currentCanonical && !candidateCanonical) return current;
  if (candidate.status === 'sent' && current.status !== 'sent') return candidate;
  return candidate;
}

function stableMessageTieBreaker(message: ChatMessage) {
  return message.serverMsgId ?? message.clientMsgId ?? message.id;
}

function uniqueStrings(values: string[]) {
  return values.filter((value, index) => value !== '' && values.indexOf(value) === index);
}
