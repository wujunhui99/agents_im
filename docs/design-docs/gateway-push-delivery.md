# Gateway Push Delivery Phase 1

Status: Implemented
Owner: Gateway
Related docs:
- [`websocket-gateway.md`](./websocket-gateway.md)
- [`gateway-message-contract.md`](./gateway-message-contract.md)
- [`redis-presence.md`](./redis-presence.md)
- [`gateway-presence-routing.md`](./gateway-presence-routing.md)
- [`message-chain-contract.md`](./message-chain-contract.md)

## Background

WebSocket Gateway phase 1 already accepts authenticated client commands and returns command ACKs. The next message-link step needs a server-side push path so the future Message Transfer worker can deliver accepted message events to online clients without changing `send_message`, `pull_messages`, or read commands.

This phase adds only the local Gateway delivery surface and in-memory fanout. A follow-up presence-routing seam lets the dispatcher query `PresenceStore` before local fanout, but durable outbox, Kafka or Redpanda consumption, a real Transfer worker, cross-process Gateway routing, offline push, and delivery ACKs stay outside this branch.

## Goals

- Define a Gateway delivery dispatcher contract for Message Transfer worker integration.
- Define a server push event envelope for message delivery events.
- Fan out a delivery event to every online WebSocket connection for one user in the current Gateway process.
- Return explicit offline status when a target user has no local connections.
- Keep command request and response behavior unchanged.
- Keep tests independent from PostgreSQL, Redis, Kafka, and Docker services.

## Non-goals

- No Kafka or Redpanda consumer.
- No durable outbox reader or retry queue.
- No Transfer worker implementation.
- No cross-instance Gateway RPC; presence route metadata is only a seam for future routing.
- No offline mobile push or delivery ACK worker.
- No Gateway ownership of message history, `server_msg_id`, conversation `seq`, or read state.

## Dispatcher Contract

The stable contract lives in `internal/gateway/delivery`.

```go
type Dispatcher interface {
    DeliverToUser(ctx context.Context, userID string, event Event) (Result, error)
    DeliverToConversation(ctx context.Context, conversationID string, recipientUserIDs []string, event Event) (Result, error)
}
```

`DeliverToConversation` deliberately receives resolved recipient user IDs. Gateway does not own conversation membership. The future Message Transfer worker or its upstream message pipeline should resolve recipients from Message Service or IM Core ownership and then call the dispatcher.

Delivery results are per-recipient and include:

- `delivered` when at least one local WebSocket connection received the event;
- `offline` when the user has no local connection;
- `failed` when local connection writes fail or the context is cancelled.

Offline is a normal result, not a panic or fatal error. Gateway presence routing adds a `routed` recipient status when Presence has live route metadata but the current process has no local connection to write.

## Push Envelope

Gateway writes server push events as WebSocket JSON frames with no `request_id`, so they do not collide with command ACKs:

```json
{
  "type": "message_received",
  "data": {
    "server_msg_id": "msg_...",
    "client_msg_id": "client-...",
    "conversation_id": "single:user_a:user_b",
    "seq": 10,
    "sender_id": "user_a",
    "receiver_id": "user_b",
    "chat_type": "single",
    "content_type": "text",
    "content": "hello",
    "content_metadata": {
      "encoding": "plain"
    },
    "send_time": 1710000000000,
    "created_at": 1710000000000,
    "trace_id": "trace_..."
  }
}
```

Supported first-phase event names:

- `message_received`: a message should be appended to the recipient's conversation stream.
- `message_delivered`: reserved for delivery-state notification when the later delivery ACK path exists.

Command responses keep the existing `request_id`, `type`, `status`, `data`, and `error` envelope documented in `websocket-gateway.md`.

## In-memory Fanout

The first implementation lives in `internal/gateway/ws` as `InMemoryDeliveryDispatcher`.

- It queries `PresenceStore.ListUserConnections(user_id)` when presence is configured, then uses `ConnectionManager.UserConnections(user_id)` to snapshot local connections for a user.
- It writes the same event to every connection with the connection's existing serialized write lock.
- If presence has no live route, it returns `offline`.
- If presence has a route but this process has no local connection, it returns `routed` with route metadata.
- If a write fails, it unregisters the stale connection from the local manager and records the failed connection ID in the result.
- It never persists messages and never advances read state.

`Server` exposes `DeliveryDispatcher()`, `PushToUser(...)`, and `PushToConversation(...)` so tests and future worker wiring can call the same surface without reaching into unexported WebSocket connection internals.

## Future Integration

The intended later flow is:

1. Message Service persists the message and emits or records a durable outbox event.
2. Message Transfer worker consumes the outbox or Kafka event.
3. Transfer worker resolves conversation recipients and routing hints.
4. If the recipient is on this Gateway instance, Transfer worker calls `Dispatcher.DeliverToUser`.
5. If Redis Presence shows the recipient on another Gateway instance, a cross-instance route or broker path forwards the event there.
6. If no online route exists, the worker leaves the durable message available for reconnect pull compensation and future offline push.

Redis Presence remains non-authoritative runtime state. PostgreSQL and Message Service remain authoritative for message history, seqs, and read state.

## Verification

Required validation:

- `goctl --version`
- `for f in api/*.api; do goctl api validate -api "$f"; done`
- `gofmt -w $(find . -name "*.go" -print)`
- `go test ./...`
- `bash scripts/verify-static.sh`
- `docker compose config`
- markdown link check when docs change
