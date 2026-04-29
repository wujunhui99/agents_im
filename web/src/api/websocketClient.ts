export type WebSocketLike = {
  readyState: number;
  onopen: ((event: Event) => void) | null;
  onmessage: ((event: MessageEvent<string>) => void) | null;
  onerror: ((event: Event) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  send(data: string): void;
  close(code?: number, reason?: string): void;
};

export type WebSocketFactory = (url: string) => WebSocketLike;

export type WebSocketCommandName =
  | 'send_message'
  | 'get_conversation_seqs'
  | 'pull_messages'
  | 'mark_conversation_read'
  | 'heartbeat'
  | (string & {});

export type WebSocketCommandEnvelope<Payload extends object = Record<string, unknown>> = {
  requestId: string;
  command: WebSocketCommandName;
  payload: Payload;
};

export type WebSocketAckStatus = 'ok' | 'error';

export type WebSocketAck = {
  requestId: string;
  type: string;
  status: WebSocketAckStatus;
  data?: unknown;
  error?: {
    code: string;
    message: string;
  };
};

export type WebSocketServerEvent = {
  type: string;
  data?: unknown;
};

export type MessageWebSocketClientOptions = {
  url: string;
  token?: string;
  tokenQueryFallback?: boolean;
  webSocketFactory?: WebSocketFactory;
  onOpen?: (event: Event) => void;
  onAck?: (ack: WebSocketAck) => void;
  onEvent?: (event: WebSocketServerEvent) => void;
  onError?: (event: Event) => void;
  onClose?: (event: CloseEvent) => void;
  onMalformedMessage?: (data: string) => void;
};

export class MessageWebSocketClient {
  private socket: WebSocketLike | null = null;

  constructor(private readonly options: MessageWebSocketClientOptions) {}

  connect() {
    if (this.socket && this.socket.readyState <= 1) {
      return;
    }

    const socket = this.getFactory()(this.buildUrl());
    socket.onopen = (event) => this.options.onOpen?.(event);
    socket.onerror = (event) => this.options.onError?.(event);
    socket.onclose = (event) => this.options.onClose?.(event);
    socket.onmessage = (event) => this.handleMessage(event.data);
    this.socket = socket;
  }

  send<Payload extends object>(envelope: WebSocketCommandEnvelope<Payload>) {
    if (!this.socket || this.socket.readyState !== 1) {
      throw new Error('WebSocket is not connected');
    }

    this.socket.send(JSON.stringify(envelope));
  }

  close(code?: number, reason?: string) {
    this.socket?.close(code, reason);
    this.socket = null;
  }

  private getFactory(): WebSocketFactory {
    return this.options.webSocketFactory ?? ((url: string) => new WebSocket(url));
  }

  private buildUrl() {
    const url = new URL(this.options.url, getWebSocketBaseUrl());

    if (url.protocol === 'http:') {
      url.protocol = 'ws:';
    }
    if (url.protocol === 'https:') {
      url.protocol = 'wss:';
    }

    if (this.options.token && this.options.tokenQueryFallback !== false) {
      url.searchParams.set('token', this.options.token);
    }

    return url.toString();
  }

  private handleMessage(data: string) {
    const parsed = parseJsonObject(data);
    if (!parsed) {
      this.options.onMalformedMessage?.(data);
      return;
    }

    if (isAckEnvelope(parsed)) {
      this.options.onAck?.(normalizeAck(parsed));
      return;
    }

    if (typeof parsed.type === 'string') {
      this.options.onEvent?.({
        type: parsed.type,
        data: parsed.data,
      });
    }
  }
}

export function createMessageWebSocketClient(options: MessageWebSocketClientOptions) {
  return new MessageWebSocketClient(options);
}

function getWebSocketBaseUrl() {
  if (typeof window === 'undefined') {
    return 'ws://127.0.0.1';
  }

  const url = new URL(window.location.href);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  return url.toString();
}

function parseJsonObject(data: string) {
  try {
    const parsed = JSON.parse(data) as unknown;
    return isRecord(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function isAckEnvelope(value: Record<string, unknown>) {
  return (
    (typeof value.request_id === 'string' || typeof value.requestId === 'string') &&
    typeof value.type === 'string' &&
    (value.status === 'ok' || value.status === 'error')
  );
}

function normalizeAck(value: Record<string, unknown>): WebSocketAck {
  const ack: WebSocketAck = {
    requestId: typeof value.request_id === 'string' ? value.request_id : String(value.requestId),
    type: String(value.type),
    status: value.status === 'error' ? 'error' : 'ok',
  };

  if ('data' in value) {
    ack.data = value.data;
  }

  if (isErrorObject(value.error)) {
    ack.error = {
      code: value.error.code,
      message: value.error.message,
    };
  }

  return ack;
}

function isErrorObject(value: unknown): value is { code: string; message: string } {
  return isRecord(value) && typeof value.code === 'string' && typeof value.message === 'string';
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
