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

export type WebSocketAuthFailure =
  | {
      source: 'close';
      code: number;
      reason: string;
    }
  | {
      source: 'event';
      type: string;
      data?: unknown;
    }
  | {
      source: 'ack';
      code: string;
      message: string;
    }
  | {
      source: 'error';
      message: string;
    };

export type MessageWebSocketClientOptions = {
  url: string;
  token?: string;
  tokenQueryFallback?: boolean;
  webSocketFactory?: WebSocketFactory;
  heartbeatIntervalMs?: number;
  heartbeatAckTimeoutMs?: number;
  reconnect?: boolean;
  reconnectDelayMs?: number;
  maxReconnectDelayMs?: number;
  requestIdFactory?: () => string;
  onOpen?: (event: Event) => void;
  onAck?: (ack: WebSocketAck) => void;
  onEvent?: (event: WebSocketServerEvent) => void;
  onError?: (event: Event) => void;
  onClose?: (event: CloseEvent) => void;
  onAuthFailure?: (failure: WebSocketAuthFailure) => void;
  onHeartbeatTimeout?: (requestId: string) => void;
  onMalformedMessage?: (data: string) => void;
  onReconnecting?: (attemptNumber: number, delayMs: number) => void;
};

export class MessageWebSocketClient {
  private socket: WebSocketLike | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private heartbeatAckTimer: ReturnType<typeof setTimeout> | null = null;
  private pendingHeartbeatRequestId: string | null = null;
  private heartbeatSequence = 0;
  private closed = false;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectDelay = 0;
  private reconnectAttempt = 0;

  constructor(private readonly options: MessageWebSocketClientOptions) {
    this.reconnectDelay = options.reconnectDelayMs ?? 1000;
  }

  connect() {
    if (this.socket && this.socket.readyState <= 1) {
      return;
    }

    const socket = this.getFactory()(this.buildUrl());
    socket.onopen = (event) => {
      this.reconnectDelay = this.options.reconnectDelayMs ?? 1000;
      this.reconnectAttempt = 0;
      this.startHeartbeat();
      this.options.onOpen?.(event);
    };
    socket.onerror = (event) => {
      const message = errorEventMessage(event);
      if (message && isAuthFailureText(message)) {
        this.options.onAuthFailure?.({ source: 'error', message });
      }
      this.options.onError?.(event);
    };
    socket.onclose = (event) => {
      this.stopHeartbeat();
      const authFailure = isAuthFailureClose(event);
      if (authFailure) {
        this.options.onAuthFailure?.({ source: 'close', code: event.code, reason: event.reason });
      }
      this.options.onClose?.(event);
      if (!this.closed && !authFailure && this.options.reconnect !== false) {
        this.scheduleReconnect();
      }
    };
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
    this.closed = true;
    this.clearReconnectTimer();
    this.stopHeartbeat();
    this.socket?.close(code, reason);
    this.socket = null;
  }

  private scheduleReconnect() {
    this.clearReconnectTimer();
    const delay = this.reconnectDelay;
    this.reconnectAttempt += 1;
    const attempt = this.reconnectAttempt;
    this.options.onReconnecting?.(attempt, delay);
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      if (!this.closed) {
        this.socket = null;
        this.connect();
      }
    }, delay);
    const nextDelay = delay * 2;
    this.reconnectDelay = Math.min(nextDelay, this.options.maxReconnectDelayMs ?? 30000);
  }

  private clearReconnectTimer() {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
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
      const ack = normalizeAck(parsed);
      this.handleAck(ack);
      if (isAuthFailureAck(ack)) {
        this.closed = true;
        this.clearReconnectTimer();
        this.options.onAuthFailure?.({
          source: 'ack',
          code: ack.error?.code ?? '',
          message: ack.error?.message ?? '',
        });
      }
      this.options.onAck?.(ack);
      return;
    }

    if (typeof parsed.type === 'string') {
      const event = {
        type: parsed.type,
        data: parsed.data,
      };
      if (isAuthFailureServerEvent(event)) {
        this.closed = true;
        this.clearReconnectTimer();
        this.options.onAuthFailure?.({ source: 'event', type: event.type, data: event.data });
        return;
      }
      this.options.onEvent?.(event);
    }
  }

  private startHeartbeat() {
    this.stopHeartbeat();
    const intervalMs = this.options.heartbeatIntervalMs ?? 30000;
    if (intervalMs <= 0) {
      return;
    }
    this.heartbeatTimer = setInterval(() => this.sendHeartbeat(), intervalMs);
  }

  private stopHeartbeat() {
    if (this.heartbeatTimer !== null) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
    this.clearHeartbeatAckTimer();
    this.pendingHeartbeatRequestId = null;
  }

  private sendHeartbeat() {
    if (!this.socket || this.socket.readyState !== 1) {
      return;
    }
    if (this.pendingHeartbeatRequestId) {
      this.handleHeartbeatTimeout(this.pendingHeartbeatRequestId);
      return;
    }

    const requestId = this.nextHeartbeatRequestId();
    this.pendingHeartbeatRequestId = requestId;
    this.socket.send(
      JSON.stringify({
        requestId,
        command: 'heartbeat',
        payload: {},
      }),
    );
    this.heartbeatAckTimer = setTimeout(() => {
      if (this.pendingHeartbeatRequestId === requestId) {
        this.handleHeartbeatTimeout(requestId);
      }
    }, this.options.heartbeatAckTimeoutMs ?? 10000);
  }

  private handleAck(ack: WebSocketAck) {
    if (ack.type !== 'heartbeat' || ack.requestId !== this.pendingHeartbeatRequestId) {
      return;
    }
    this.pendingHeartbeatRequestId = null;
    this.clearHeartbeatAckTimer();
  }

  private handleHeartbeatTimeout(requestId: string) {
    this.options.onHeartbeatTimeout?.(requestId);
    this.stopHeartbeat();
    this.socket?.close(4000, 'heartbeat ack timeout');
  }

  private clearHeartbeatAckTimer() {
    if (this.heartbeatAckTimer !== null) {
      clearTimeout(this.heartbeatAckTimer);
      this.heartbeatAckTimer = null;
    }
  }

  private nextHeartbeatRequestId() {
    if (this.options.requestIdFactory) {
      return this.options.requestIdFactory();
    }
    this.heartbeatSequence += 1;
    return `heartbeat-${Date.now()}-${this.heartbeatSequence}`;
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

const authFailureCloseCodes = new Set([4001, 4002]);
const authFailureAckCodes = new Set([
  'UNAUTHENTICATED',
  'UNAUTHORIZED',
  'SESSION_INACTIVE',
  'SESSION_INVALID',
  'SESSION_REPLACED',
  'SESSION_EXPIRED',
  'TOKEN_INVALID',
  'TOKEN_EXPIRED',
]);
const authFailureEventTypes = new Set([
  'session_replaced',
  'session_invalid',
  'session_inactive',
  'session_expired',
  'session_logout',
  'force_logout',
  'logout',
]);

function isAuthFailureClose(event: CloseEvent) {
  return authFailureCloseCodes.has(event.code) || isAuthFailureText(event.reason);
}

function isAuthFailureAck(ack: WebSocketAck) {
  const code = ack.error?.code.toUpperCase() ?? '';
  return authFailureAckCodes.has(code) || isAuthFailureText(ack.error?.message ?? '');
}

function isAuthFailureServerEvent(event: WebSocketServerEvent) {
  const normalizedType = normalizeEventType(event.type);
  return authFailureEventTypes.has(normalizedType) || isAuthFailureData(event.data);
}

function isAuthFailureData(data: unknown): boolean {
  if (typeof data === 'string') {
    return isAuthFailureText(data);
  }
  if (!isRecord(data)) {
    return false;
  }

  return ['code', 'reason', 'message', 'type'].some((key) => {
    const value = data[key];
    return typeof value === 'string' && isAuthFailureText(value);
  });
}

function isAuthFailureText(value: string) {
  const normalized = value.toLowerCase();
  return (
    normalized.includes('unauthenticated') ||
    normalized.includes('unauthorized') ||
    normalized.includes('invalid or missing bearer token') ||
    normalized.includes('token session is not active') ||
    normalized.includes('session inactive') ||
    normalized.includes('session invalid') ||
    normalized.includes('session replaced') ||
    normalized.includes('session expired') ||
    normalized.includes('token expired')
  );
}

function normalizeEventType(type: string) {
  return type.trim().toLowerCase().replace(/[\s-]+/g, '_');
}

function errorEventMessage(event: Event) {
  if ('message' in event && typeof event.message === 'string') {
    return event.message;
  }
  return '';
}
