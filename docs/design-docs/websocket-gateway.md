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
- Accept token from `Authorization: Bearer <token>` or `?token=<token>`.
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
```

`StorageDriver: memory` is the default first-phase mode. `StorageDriver: postgres` can use the existing message repository implementation after database migrations are applied, but normal tests remain memory-only.

## Handshake

Gateway validates the token before upgrading:

1. Read `Authorization: Bearer <token>`.
2. If absent, read `token` query param.
3. Validate with `internal/auth/token.HMACTokenManager` using `Auth.AccessSecret` and `Auth.AccessExpire`.
4. Reject missing, malformed, invalid, or expired tokens with HTTP 401.
5. Register the upgraded connection under the token `user_id`.

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

Canonical response:

```json
{
  "request_id": "client-request-id",
  "type": "send_message",
  "status": "ok",
  "data": {}
}
```

Error response:

```json
{
  "request_id": "client-request-id",
  "type": "send_message",
  "status": "error",
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "command payload is invalid"
  }
}
```

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
- WebSocket pong updates connection last-seen time.
- Reconnect compensation remains client-driven through `get_conversation_seqs` and `pull_messages`.
- Delivery ACK and server-pushed message fanout are future work tied to Kafka/Push/Presence integration.

## Verification

Required validation:

- `goctl --version`
- `for f in api/*.api; do goctl api validate -api "$f"; done`
- `gofmt -w $(find . -name "*.go" -print)`
- `go test ./...`
- `bash scripts/verify-static.sh`
- `docker compose config`

Tests cover missing/invalid token rejection, valid token from header/query, heartbeat, `send_message`, and `pull_messages` with memory storage.
