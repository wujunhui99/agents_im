import { createApiClient, type ApiClient } from './client';
import type { ChatType, MessageContentType, MessageOrigin } from '../models/messages';

export type ServerMessage = {
  serverMsgId: string;
  clientMsgId: string;
  conversationId: string;
  seq: number;
  senderId: string;
  receiverId?: string;
  groupId?: string;
  chatType: ChatType;
  contentType: MessageContentType;
  content: string;
  messageOrigin?: MessageOrigin;
  agentAccountId?: string;
  triggerServerMsgId?: string;
  agentRunId?: string;
  allowRecursiveTrigger?: boolean;
  sendTime: number;
  createdAt: number;
};

export type SendMessageRequest =
  | {
      receiverId: string;
      chatType: 'single';
      clientMsgId: string;
      contentType: MessageContentType;
      content: string;
    }
  | {
      groupId: string;
      chatType: 'group';
      clientMsgId: string;
      contentType: MessageContentType;
      content: string;
    };

export type SendMessageResponse = {
  message: ServerMessage;
  deduplicated: boolean;
};

export type ConversationSeqState = {
  conversationId: string;
  maxSeq: number;
  hasReadSeq?: number;
  unreadCount?: number;
  maxSeqTime?: number;
  lastMessage?: ServerMessage;
};

export type ConversationSeqsResponse = {
  states?: ConversationSeqState[];
  conversations?: ConversationSeqState[];
  seqs?: ConversationSeqState[];
};

export type PullMessagesRequest = {
  fromSeq: number;
  toSeq?: number;
  limit?: number;
  order?: 'asc' | 'desc';
};

export type PullMessagesResponse = {
  conversationId: string;
  messages: ServerMessage[];
  hasMore?: boolean;
  nextSeq?: number;
};

export type MarkReadRequest = {
  hasReadSeq: number;
};

export type MarkReadResponse = {
  conversationId?: string;
  hasReadSeq?: number;
};

export type MessageApi = {
  sendMessage: (request: SendMessageRequest) => Promise<SendMessageResponse>;
  getConversationSeqs: (conversationIds: string[]) => Promise<ConversationSeqsResponse>;
  pullMessages: (conversationId: string, request: PullMessagesRequest) => Promise<PullMessagesResponse>;
  markRead: (conversationId: string, request: MarkReadRequest) => Promise<MarkReadResponse>;
};

export function createMessageApi(api: ApiClient = createApiClient()): MessageApi {
  return {
    sendMessage(request) {
      return api.post<SendMessageResponse>('/messages', request);
    },
    getConversationSeqs(conversationIds) {
      const params = new URLSearchParams();
      params.set('conversationIds', conversationIds.join(','));
      return api.get<ConversationSeqsResponse>(`/conversations/seqs?${params.toString()}`);
    },
    pullMessages(conversationId, request) {
      const params = new URLSearchParams();
      params.set('fromSeq', String(request.fromSeq));
      params.set('limit', String(request.limit ?? 50));
      params.set('order', request.order ?? 'asc');
      if (request.toSeq !== undefined) {
        params.set('toSeq', String(request.toSeq));
      }
      return api.get<PullMessagesResponse>(`/conversations/${encodeURIComponent(conversationId)}/messages?${params.toString()}`);
    },
    markRead(conversationId, request) {
      return api.post<MarkReadResponse>(`/conversations/${encodeURIComponent(conversationId)}/read`, request);
    },
  };
}

const defaultMessageApi = createMessageApi();

export function sendMessage(request: SendMessageRequest) {
  return defaultMessageApi.sendMessage(request);
}

export function getConversationSeqs(conversationIds: string[]) {
  return defaultMessageApi.getConversationSeqs(conversationIds);
}

export function pullMessages(conversationId: string, request: PullMessagesRequest) {
  return defaultMessageApi.pullMessages(conversationId, request);
}

export function markRead(conversationId: string, request: MarkReadRequest) {
  return defaultMessageApi.markRead(conversationId, request);
}
