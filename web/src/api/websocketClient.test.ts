import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  createMessageWebSocketClient,
  type WebSocketAck,
  type WebSocketFactory,
  type WebSocketLike,
} from './websocketClient';

class FakeWebSocket implements WebSocketLike {
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent<string>) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  readyState = 1;
  readonly sent: string[] = [];
  closed = false;

  constructor(readonly url: string) {}

  send(data: string) {
    this.sent.push(data);
  }

  close() {
    this.readyState = 3;
    this.closed = true;
  }

  emitMessage(payload: unknown) {
    this.onmessage?.({ data: JSON.stringify(payload) } as MessageEvent<string>);
  }
}

describe('message WebSocket client', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('uses token query fallback, sends command envelopes, and parses ACK envelopes', () => {
    const sockets: FakeWebSocket[] = [];
    const acks: WebSocketAck[] = [];
    const webSocketFactory: WebSocketFactory = (url) => {
      const socket = new FakeWebSocket(url);
      sockets.push(socket);
      return socket;
    };
    const client = createMessageWebSocketClient({
      url: 'ws://127.0.0.1:8084/ws',
      token: '***',
      webSocketFactory,
      onAck: (ack) => acks.push(ack),
    });

    client.connect();
    expect(sockets).toHaveLength(1);
    expect(sockets[0].url).toBe('ws://127.0.0.1:8084/ws?token=***');

    client.send({
      requestId: 'req-send-1',
      command: 'send_message',
      payload: {
        chatType: 'single',
        receiverId: '2002',
        clientMsgId: 'web-uuid-003',
        contentType: 'text',
        content: 'hello over websocket',
      },
    });

    expect(JSON.parse(sockets[0].sent[0])).toEqual({
      requestId: 'req-send-1',
      command: 'send_message',
      payload: {
        chatType: 'single',
        receiverId: '2002',
        clientMsgId: 'web-uuid-003',
        contentType: 'text',
        content: 'hello over websocket',
      },
    });

    sockets[0].emitMessage({
      request_id: 'req-send-1',
      type: 'send_message',
      status: 'ok',
      data: {
        server_msg_id: 'msg_000001',
      },
    });

    expect(acks).toEqual([
      {
        requestId: 'req-send-1',
        type: 'send_message',
        status: 'ok',
        data: {
          server_msg_id: 'msg_000001',
        },
      },
    ]);

    client.close();
    expect(sockets[0].closed).toBe(true);
  });

  it('sends heartbeat commands and closes when heartbeat ACK is missed', () => {
    const sockets: FakeWebSocket[] = [];
    const heartbeatTimeouts: string[] = [];
    const webSocketFactory: WebSocketFactory = (url) => {
      const socket = new FakeWebSocket(url);
      sockets.push(socket);
      return socket;
    };
    const requestIds = ['heartbeat-1', 'heartbeat-2'];
    const client = createMessageWebSocketClient({
      url: 'ws://127.0.0.1:8084/ws',
      token: '***',
      webSocketFactory,
      heartbeatIntervalMs: 1000,
      heartbeatAckTimeoutMs: 500,
      requestIdFactory: () => requestIds.shift() ?? 'heartbeat-extra',
      onHeartbeatTimeout: (requestId) => heartbeatTimeouts.push(requestId),
    });

    client.connect();
    sockets[0].onopen?.(new Event('open'));

    vi.advanceTimersByTime(1000);
    expect(JSON.parse(sockets[0].sent[0])).toEqual({
      requestId: 'heartbeat-1',
      command: 'heartbeat',
      payload: {},
    });

    sockets[0].emitMessage({
      requestId: 'heartbeat-1',
      type: 'heartbeat',
      status: 'ok',
      data: {
        connection_id: 'conn_test',
      },
    });
    vi.advanceTimersByTime(500);
    expect(sockets[0].closed).toBe(false);

    vi.advanceTimersByTime(500);
    expect(JSON.parse(sockets[0].sent[1])).toEqual({
      requestId: 'heartbeat-2',
      command: 'heartbeat',
      payload: {},
    });

    vi.advanceTimersByTime(499);
    expect(sockets[0].closed).toBe(false);
    vi.advanceTimersByTime(1);

    expect(sockets[0].closed).toBe(true);
    expect(heartbeatTimeouts).toEqual(['heartbeat-2']);
  });
});
