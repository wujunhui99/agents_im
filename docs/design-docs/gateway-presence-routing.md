# Gateway Presence Routing

Status: Implemented
Owner: Gateway
Related docs:
- [`websocket-gateway.md`](./websocket-gateway.md)
- [`redis-presence.md`](./redis-presence.md)
- [`gateway-push-delivery.md`](./gateway-push-delivery.md)
- [`message-transfer-worker.md`](./message-transfer-worker.md)
- [`message-chain-contract.md`](./message-chain-contract.md)

## Background

The WebSocket Gateway already owns authenticated client connections and local in-process push delivery. Redis Presence already defines the short-lived connection metadata contract, but the Gateway did not yet write connection lifecycle state to that store.

This phase connects those seams without changing the client command protocol or adding cross-Gateway RPC. Message persistence, conversation seqs, read state, outbox publishing, Kafka consumption, and offline push remain outside Gateway.

## Goals

- Register each accepted WebSocket connection in `PresenceStore`.
- Refresh presence TTL on app-level `heartbeat` commands and WebSocket pong activity.
- Unregister presence records when a WebSocket connection closes.
- Store routing metadata for future cross-instance delivery, including `instance_id`, `gateway_id`, and `connection_id`.
- Let the delivery dispatcher query presence before local fanout.
- Keep ordinary `go test ./...` independent from Redis by using the memory presence store in tests.

## Non-goals

- No Redis-only default test path.
- No Kafka consumer, outbox publisher, or Message Transfer production wiring.
- No cross-process RPC between Gateway instances.
- No change to existing `send_message`, `pull_messages`, `get_conversation_seqs`, `mark_conversation_read`, or command ACK semantics.

## Runtime Behavior

`cmd/gateway-ws` creates a presence store from the existing API config:

```text
Presence.Driver: memory | redis
Presence.HeartbeatTTLSeconds: 60
Redis.Addr / Redis.Password / Redis.DB
```

The default remains `memory`, so local service starts and tests do not require Redis. Setting `Presence.Driver: redis` uses the Redis implementation from [`../../internal/presence`](../../internal/presence).

Connection lifecycle:

1. The Gateway authenticates the JWT and upgrades the WebSocket.
2. It creates a server-owned `connection_id` and attaches the Gateway `instance_id`.
3. It registers `ConnectionMetadata` in presence with the configured heartbeat TTL.
4. App `heartbeat` commands and WebSocket pong frames refresh `last_heartbeat_at` and `expires_at`.
5. On disconnect, Gateway unregisters the `user_id + connection_id` presence record.

Presence write failures do not change durable message state and do not mutate the WebSocket command contract. The connection can continue to use synchronous send, pull, and read commands. Delivery routing failures are surfaced in delivery results because Message Transfer needs to know whether online routing was evaluated.

## Routing Seam

`internal/gateway/ws.InMemoryDeliveryDispatcher` is still the only delivery implementation. It now accepts a `PresenceStore` and resolves presence routes before local fanout:

```text
PresenceStore.ListUserConnections(user_id)
  -> []ConnectionMetadata{instance_id, gateway_id, connection_id, ...}
  -> local WebSocket fanout in this process
```

Recipient results include optional `routes` metadata. A route is marked `local` when its `instance_id` or `gateway_id` matches the current Gateway instance. This is only a routing hint; this branch still writes only to WebSocket connections in the current process.

Recipient status meanings:

- `delivered`: at least one local connection received the push event.
- `offline`: presence has no live connection metadata for the user.
- `routed`: presence has live route metadata but this Gateway process has no local connection to write.
- `failed`: presence lookup failed, context was cancelled, or local writes failed.

The future cross-instance dispatcher can use `routed` plus route metadata to forward to the owning Gateway instance. This branch deliberately does not implement that transport.

## Protocol Compatibility

Client command request payloads are unchanged. The `heartbeat` response keeps existing fields and may include `instance_id` as additive metadata:

```json
{
  "request_id": "req-heartbeat",
  "type": "heartbeat",
  "status": "ok",
  "data": {
    "connection_id": "conn_...",
    "user_id": "740000000000000000",
    "instance_id": "gateway-a",
    "server_time": 1710000000000
  }
}
```

Server push events keep the existing `type/data` envelope from [`gateway-push-delivery.md`](./gateway-push-delivery.md).

## Verification

Required validation:

- `goctl --version`
- `for f in api/*.api; do goctl api validate -api "$f"; done`
- `gofmt -w $(find . -name "*.go" -print)`
- `go test ./...`
- `bash scripts/verify-static.sh`
- `docker compose config`
- markdown link check when docs change

Tests cover connect registration, heartbeat refresh, disconnect unregister, offline delivery status, multiple connections, and presence lookup failure handling with the in-memory test path.
