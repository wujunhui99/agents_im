import type { ChatType, MessageContentType } from '../models/messages';

export type ApiEnvelope<T> = {
  code: string;
  message: string;
  data: T;
};

export type MessageApiOptions = {
  baseUrl?: string;
  token?: string;
  fetchImpl?: typeof fetch;
};

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
};

export type ConversationSeqsResponse = {
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

export class MessageApiError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = 'MessageApiError';
  }
}

export function sendMessage(request: SendMessageRequest, options: MessageApiOptions = {}) {
  return requestJson<SendMessageResponse>('/messages', options, {
    method: 'POST',
    body: JSON.stringify(request),
  });
}

export function getConversationSeqs(conversationIds: string[], options: MessageApiOptions = {}) {
  const params = new URLSearchParams();
  params.set('conversationIds', conversationIds.join(','));

  return requestJson<ConversationSeqsResponse>(`/conversations/seqs?${params.toString()}`, options);
}

export function pullMessages(
  conversationId: string,
  request: PullMessagesRequest,
  options: MessageApiOptions = {},
) {
  const params = new URLSearchParams();
  params.set('fromSeq', String(request.fromSeq));
  params.set('limit', String(request.limit ?? 50));
  params.set('order', request.order ?? 'asc');
  if (request.toSeq !== undefined) {
    params.set('toSeq', String(request.toSeq));
  }

  return requestJson<PullMessagesResponse>(
    `/conversations/${encodeURIComponent(conversationId)}/messages?${params.toString()}`,
    options,
  );
}

export function markRead(
  conversationId: string,
  request: MarkReadRequest,
  options: MessageApiOptions = {},
) {
  return requestJson<MarkReadResponse>(`/conversations/${encodeURIComponent(conversationId)}/read`, options, {
    method: 'POST',
    body: JSON.stringify(request),
  });
}

async function requestJson<T>(path: string, options: MessageApiOptions, init: RequestInit = {}): Promise<T> {
  const response = await getFetch(options)(`${normalizeBaseUrl(options.baseUrl)}${path}`, {
    ...init,
    headers: buildHeaders(options, init),
  });
  const envelope = (await response.json()) as ApiEnvelope<T | null>;

  if (!response.ok || envelope.code !== 'OK' || envelope.data === null) {
    throw new MessageApiError(envelope.code, envelope.message, response.status);
  }

  return envelope.data;
}

function getFetch(options: MessageApiOptions) {
  return options.fetchImpl ?? globalThis.fetch.bind(globalThis);
}

function buildHeaders(options: MessageApiOptions, init: RequestInit) {
  const headers = new Headers(init.headers);

  if (init.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  if (options.token) {
    headers.set('Authorization', `Bearer ${options.token}`);
  }

  return headers;
}

function normalizeBaseUrl(baseUrl = '') {
  return baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl;
}
