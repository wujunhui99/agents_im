# WebSocket Gateway Phase 1

Status: Implemented
Owner: Gateway
Related docs:
- [`gateway-message-contract.md`](./gateway-message-contract.md)
- [`message-chain-contract.md`](./message-chain-contract.md)
- [`jwt-auth-middleware.md`](./jwt-auth-middleware.md)
- [`websocket-reliability.md`](./websocket-reliability.md)
- [`gateway-presence-routing.md`](./gateway-presence-routing.md)

## Background

Gateway is the long-connection entry point for IM clients. The previous Gateway work defined the command-to-message-service contract but did not run a real WebSocket endpoint. Phase 1 adds the first executable Gateway while preserving the existing Message Service ownership of message persistence, idempotency, conversation seq, and read state.

## Goals

- Expose `GET /ws` as a WebSocket upgrade endpoint through `cmd/gateway-ws`.
- Authenticate the WebSocket handshake with the existing HS256 JWT token format.
- Accept token from `Authorization: Bearer <token>`; `?token=<token>` is available only when explicitly enabled in Gateway config.
- Enforce an explicit browser `Origin` policy instead of relying on `gorilla/websocket` defaults.
- Maintain an in-memory connection manager keyed by `user_id`, supporting multiple `connection_id` values per user.
- Route client commands: `send_message`, `pull_messages`, `mark_conversation_read`, `get_conversation_seqs`, and `heartbeat`.
- Call existing `MessageLogic` and repository contracts for message commands.
- Keep normal `go test ./...` independent from PostgreSQL and Redis.

## Non-goals

- No Kafka fanout, Push worker, offline push, retry queue, or delivery ACK worker.
- Redis Presence integration is handled by the follow-up Gateway presence routing seam; this phase does not implement cross-instance RPC.
- No docker-compose Redis or CI workflow changes.
- No changes to user/auth/friends/groups/message business ownership.
- No replacement of goctl REST/RPC service structure.

## Runtime Shape

Entry point:

```text
cmd/gateway-ws/main.go -f etc/gateway-ws.yaml
```

Routes:

```text
GET /healthz
GET /ws
```

Configuration reuses the existing flat API config loader:

```yaml
Name: gateway-ws
Host: 0.0.0.0
Port: 8084
Auth:
  AccessSecret: dev-jwt-secret-change-me
  AccessExpire: 86400
StorageDriver: memory
DataSource: ${DATABASE_URL}
GatewayWS:
  AllowedOrigins: http://localhost:5173,http://127.0.0.1:5173
  AllowQueryToken: true
  PingIntervalSeconds: 30
  HeartbeatTimeoutSeconds: 75
  CommandRateLimitPerSecond: 20
  CommandRateLimitBurst: 40
```

`StorageDriver: memory` is the default first-phase mode. `StorageDriver: postgres` can use the existing message repository implementation after database migrations are applied, but normal tests remain memory-only.

`GatewayWS` keys:

| Key | Default | Meaning |
| --- | --- | --- |
| `AllowedOrigins` | empty | Comma-separated exact `http(s)://host[:port]` origins. Empty means no browser cross-origin access; same-origin requests are accepted. Wildcards are rejected. |
| `AllowQueryToken` | `false` | Enables `?token=` only for controlled local/dev or constrained clients. Production should prefer the `Authorization` header. |
| `PingIntervalSeconds` | `30` | Server WebSocket ping interval. |
| `HeartbeatTimeoutSeconds` | `75` | Read deadline extended by pong frames; dead or non-responsive peers are closed after this timeout. |
| `CommandRateLimitPerSecond` | `20` | Per-connection command token refill rate. |
| `CommandRateLimitBurst` | `40` | Per-connection command burst capacity. |

Equivalent environment overrides use `GATEWAY_WS_*` names: `GATEWAY_WS_ALLOWED_ORIGINS`, `GATEWAY_WS_ALLOW_QUERY_TOKEN`, `GATEWAY_WS_PING_INTERVAL_SECONDS`, `GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS`, `GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND`, and `GATEWAY_WS_COMMAND_RATE_LIMIT_BURST`. `AGENTS_IM_GATEWAY_WS_*` aliases are also accepted by the config resolver.

## Handshake

Gateway validates the token before upgrading:

1. Read `Authorization: Bearer <token>`.
2. If absent and `GatewayWS.AllowQueryToken=true`, read `token` query param.
3. Validate the browser `Origin` during upgrade. If `GatewayWS.AllowedOrigins` is non-empty, the origin must exactly match one configured origin. If it is empty, only same-origin browser requests are accepted. Requests without an `Origin` header are treated as non-browser clients and are allowed to proceed to JWT auth.
4. Validate with `internal/auth/token.HMACTokenManager` using `Auth.AccessSecret` and `Auth.AccessExpire`.
5. Reject missing, malformed, invalid, or expired tokens with HTTP 401.
6. Register the upgraded connection under the token `user_id`.

The Gateway never accepts a client-provided user id as identity.

## Command Envelope

Canonical WebSocket request:

```json
{
  "request_id": "client-request-id",
  "type": "send_message",
  "payload": {}
}
```

For compatibility with the existing Gateway contract document, the server also accepts `requestId` and `command` aliases.

Frontend response:

```json
{
  "requestId": "client-request-id",
  "command": "send_message",
  "status": "ok",
  "payload": {}
}
```

Error response:

```json
{
  "requestId": "client-request-id",
  "command": "send_message",
  "status": "error",
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "command payload is invalid"
  }
}
```

During the MVP transition, the server also emits legacy aliases `request_id`, `type`, and `data` for existing tools. New frontend code should read `requestId`, `command`, and `payload`.

`status=ok` means the command was accepted by the Gateway and completed successfully in the called Message logic. It does not mean recipients have received or seen the message.

## Command Routing

| Command | Gateway behavior |
| --- | --- |
| `heartbeat` | Return `connection_id`, `user_id`, optional `instance_id`, and server time; refresh presence TTL when presence is configured. |
| `send_message` | Inject connection `user_id` as sender and call `MessageLogic.SendMessage`. |
| `pull_messages` | Inject connection `user_id` and call `MessageLogic.PullMessages`. |
| `get_conversation_seqs` | Inject connection `user_id` and call `MessageLogic.GetConversationSeqs`. |
| `mark_conversation_read` | Inject connection `user_id` and call `MessageLogic.MarkConversationAsRead`. |

Gateway does not allocate `server_msg_id`, does not assign conversation `seq`, does not persist messages, and does not store read progress. Those remain Message Service responsibilities.

## Connection Manager

`internal/gateway/ws.ConnectionManager` keeps:

- `connection_id -> connection`
- `user_id -> connection_id -> connection`

It supports multiple connections per user for multi-device clients. It also exposes a `PresenceReporter` interface with `Connected` and `Disconnected` hooks. Gateway presence routing now writes the same lifecycle to `PresenceStore`, using the memory store by default and Redis only when configured.

## Reliability Notes

- App-level `heartbeat` is implemented in phase 1.
- The server sends WebSocket ping frames every `GatewayWS.PingIntervalSeconds`.
- WebSocket pong updates connection last-seen time and extends the read deadline by `GatewayWS.HeartbeatTimeoutSeconds`.
- Each connection has a token-bucket command limiter. Exceeding it returns the normal command ACK envelope with `status=error` and `error.code=RATE_LIMITED`.
- Reconnect compensation remains client-driven through `get_conversation_seqs` and `pull_messages`; the stable frontend contract is [`websocket-reconnect-sync.md`](./websocket-reconnect-sync.md).
- Delivery ACK and server-pushed message fanout are future work tied to Kafka/Push/Presence integration.

## Verification

Required validation:

- `goctl --version`
- `for f in api/*.api; do goctl api validate -api "$f"; done`
- `gofmt -w $(find . -name "*.go" -print)`
- `go test ./...`
- `bash scripts/verify-static.sh`
- `docker compose config`

Tests cover missing/invalid token rejection, valid token from header/configured query-token auth, origin policy, ping loop, rate limiting, heartbeat, `send_message`, and `pull_messages` with memory storage.


## Local Live Delivery On WebSocket Send

Status: Implemented for single-chat single-instance delivery.

When an authenticated WebSocket client sends `send_message`, Gateway calls Message Service logic to persist the message and returns the normal command ACK. After a successful non-deduplicated send, Gateway also emits a `message_received` server push to online single-chat recipients through the in-process delivery dispatcher.

Current behavior:

1. sender sends `send_message` over WebSocket;
2. Message Service persists the message and returns `server_msg_id`, `conversation_id`, and `seq`;
3. sender receives ACK with status `ok`;
4. for single-chat messages, Gateway derives the receiver from the stored message and excludes the sender;
5. online receiver connections on the same Gateway instance receive `message_received` immediately;
6. the message remains pullable through the normal pull/sync APIs.

The live push is best-effort for local online delivery. If push fails, Gateway logs the delivery error with trace context but does not fail the original send ACK, because the message has already been accepted and persisted. Offline delivery, cross-instance routing, retry, and delivery attempt recording remain owned by the broader Message Transfer / Delivery pipeline.

Group-chat immediate fanout is intentionally deferred until Gateway has an authoritative group-member lookup boundary for fanout recipients.

Verification:

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./tests -run 'TestWebSocketGatewaySendMessagePushesToOnlineReceiver|TestMVPBackendWebSocketSendPullMarkReadSmoke' -count=1
```
