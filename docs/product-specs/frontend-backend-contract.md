# Frontend Backend Handoff Contract

This document is the MVP frontend handoff for the Go backend. It describes the frontend-visible REST and WebSocket contracts that are implemented or explicitly identified as local demo gaps in this worktree.

## Conventions

- Local API defaults are documented in [../DEVELOPMENT.md](../DEVELOPMENT.md).
- Protected REST endpoints require `Authorization: Bearer <access_token>`.
- REST responses use a shared envelope:

```json
{
  "code": "OK",
  "message": "ok",
  "data": {}
}
```

- REST error responses use the same envelope with `data: null`:

```json
{
  "code": "INVALID_ARGUMENT",
  "message": "identifier is required",
  "data": null
}
```

- Common REST error codes are `INVALID_ARGUMENT`, `UNAUTHENTICATED`, `NOT_FOUND`, `ALREADY_EXISTS`, and `INTERNAL`.
- REST field names are snake_case for account/social services and camelCase for message payloads. Frontend adapters should preserve the field names shown below.
- Example passwords and tokens are redacted. Do not commit real credentials.

## Auth

### Register

```http
POST /auth/register
Content-Type: application/json
```

```json
{
  "identifier": "alice_001",
  "password": "[REDACTED]",
  "display_name": "Alice",
  "gender": "female",
  "age": 30,
  "region": "Shanghai"
}
```

Success:

```json
{
  "code": "OK",
  "message": "ok",
  "data": {
    "user_id": "usr_000001",
    "identifier": "alice_001",
    "token": "[REDACTED]",
    "expires_at": "2026-04-30T12:00:00Z"
  }
}
```

### Login

```http
POST /auth/login
Content-Type: application/json
```

```json
{
  "identifier": "alice_001",
  "password": "[REDACTED]"
}
```

Success returns the same `data` shape as register. The `token` value is the bearer token used by REST and WebSocket clients.

### Validate Token

```http
POST /auth/validate
Content-Type: application/json
```

```json
{
  "token": "[REDACTED]"
}
```

## Profile And User Search

### Current User

```http
GET /me
Authorization: Bearer <access_token>
```

```json
{
  "code": "OK",
  "message": "ok",
  "data": {
    "user_id": "usr_000001",
    "identifier": "alice_001",
    "display_name": "Alice",
    "name": "Alice",
    "gender": "female",
    "age": 30,
    "region": "Shanghai",
    "account_type": "normal",
    "created_at": "2026-04-29T12:00:00Z",
    "updated_at": "2026-04-29T12:00:00Z"
  }
}
```

`account_type` is one of `normal`, `agent`, or `admin`. Public registration and public user creation always create `normal`; clients cannot self-select `agent` or `admin` through the frontend-visible REST API.

### Update Current User

```http
PATCH /me
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "display_name": "Alice Chen",
  "region": "Hangzhou"
}
```

The backend rejects attempts to patch immutable `user_id` or `identifier`.

### Identifier Exists

```http
GET /users/exists?identifier=alice_001
```

### Public Profile By Identifier

```http
GET /users/alice_001
```

This is the current MVP user search path.

## Friends

Friendship is immediately accepted in MVP. Duplicate add is idempotent, self-add returns `INVALID_ARGUMENT`, and non-existent users return `NOT_FOUND`.

### Add Friend

```http
POST /friends
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "user_id": "usr_000002"
}
```

Success:

```json
{
  "code": "OK",
  "message": "ok",
  "data": {
    "friendship": {
      "user_id": "usr_000001",
      "friend_id": "usr_000002",
      "status": "active",
      "is_friend": true,
      "created_at": "2026-04-29T12:01:00Z",
      "updated_at": "2026-04-29T12:01:00Z"
    },
    "created": true
  }
}
```

### List Friends

```http
GET /friends
Authorization: Bearer <access_token>
```

### Get Friendship

```http
GET /friends/usr_000002
Authorization: Bearer <access_token>
```

### Delete Friend

```http
DELETE /friends/usr_000002
Authorization: Bearer <access_token>
```

## Groups

The group creator is automatically an active member. Groups are open join in MVP.
Group detail and member-list reads require a bearer token and only active members can read them. Adding a different user requires the group creator/owner.

### Create Group

```http
POST /groups
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "name": "Frontend Demo",
  "description": "MVP smoke room"
}
```

### Get Group

```http
GET /groups/grp_000001
Authorization: Bearer <access_token>
```

### Join Or Add Member

```http
POST /groups/grp_000001/members
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "user_id": "usr_000002"
}
```

If `user_id` is omitted, the authenticated user joins the group.
If `user_id` names another user, the authenticated user must be the group creator/owner.

### Leave Group

```http
DELETE /groups/grp_000001/members/me
Authorization: Bearer <access_token>
```

### List Members

```http
GET /groups/grp_000001/members
Authorization: Bearer <access_token>
```

### Current Gap

A dedicated `GET /groups` or `ListGroups` endpoint is not present in this worktree. For the local demo, keep group IDs from create/join responses or known fixture data and call `GET /groups/:group_id/members`.

## Messages REST

Message history is authoritative in the message service. WebSocket delivery is best-effort, may be duplicated, and may arrive out of order. Frontend display order must not use network arrival order.

Confirmed messages are ordered by numeric `seq` inside each `conversationId`; the pair `conversationId + seq` is the authoritative timeline position. `sendTime` is display metadata and is not a sorting authority. Clients should deduplicate repeated deliveries by `serverMsgId`; while a send is pending, the eventual server response should replace the optimistic item by `clientMsgId` and preserve canonical server fields including `serverMsgId` and `seq`.

Pending local messages without `seq` may remain after confirmed messages in local enqueue order. Same-conversation sends should be queued or the composer should be disabled with a visible sending state until the prior send is accepted or fails.

### Send Message

```http
POST /messages
Authorization: Bearer <access_token>
Content-Type: application/json
```

Single chat:

```json
{
  "receiverId": "usr_000002",
  "chatType": "single",
  "clientMsgId": "web-uuid-001",
  "contentType": "text",
  "content": "hello"
}
```

Group chat:

```json
{
  "groupId": "grp_000001",
  "chatType": "group",
  "clientMsgId": "web-uuid-002",
  "contentType": "text",
  "content": "hello group"
}
```

Success:

```json
{
  "code": "OK",
  "message": "ok",
  "data": {
    "message": {
      "serverMsgId": "msg_000001",
      "clientMsgId": "web-uuid-001",
      "conversationId": "single:usr_000001:usr_000002",
      "seq": 1,
      "senderId": "usr_000001",
      "receiverId": "usr_000002",
      "chatType": "single",
      "contentType": "text",
      "content": "hello",
      "sendTime": 1777464000000,
      "createdAt": 1777464000000
    },
    "deduplicated": false
  }
}
```

### Get Conversation Seq States

```http
GET /conversations/seqs?conversationIds=single:usr_000001:usr_000002
Authorization: Bearer <access_token>
```

### Pull Messages

```http
GET /conversations/single:usr_000001:usr_000002/messages?fromSeq=1&limit=50&order=asc
Authorization: Bearer <access_token>
```

### Mark Conversation Read

```http
POST /conversations/single:usr_000001:usr_000002/read
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "hasReadSeq": 1
}
```

## WebSocket

### Connect

Preferred:

```http
GET /ws
Authorization: Bearer <access_token>
```

Fallback for clients that cannot set headers:

```text
ws://127.0.0.1:8084/ws?token=<access_token>
```

The gateway authenticates the same JWT used by REST. Missing or invalid tokens fail the handshake with HTTP 401. Query-token auth is disabled by default and only works when `GatewayWS.AllowQueryToken=true`; browser cross-origin access must match `GatewayWS.AllowedOrigins` exactly, while empty allowed origins only permit same-origin browser requests.

### Command Envelope

The gateway accepts either camel or snake command keys. Frontend code should send camel keys:

```json
{
  "requestId": "req-001",
  "command": "heartbeat",
  "payload": {}
}
```

The current server ACK shape is snake_case:

```json
{
  "request_id": "req-001",
  "type": "heartbeat",
  "status": "ok",
  "data": {
    "connection_id": "conn_abc",
    "user_id": "usr_000001",
    "instance_id": "gateway-local",
    "server_time": 1777464000000
  }
}
```

Command error ACK:

```json
{
  "request_id": "req-002",
  "type": "send_message",
  "status": "error",
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "content is required"
  }
}
```

WebSocket error codes include the REST application codes plus `IDEMPOTENCY_CONFLICT` for conflicting `clientMsgId` retries and `RATE_LIMITED` for per-connection command throttling.

### send_message

```json
{
  "requestId": "req-send-1",
  "command": "send_message",
  "payload": {
    "chatType": "single",
    "receiverId": "usr_000002",
    "clientMsgId": "web-uuid-003",
    "contentType": "text",
    "content": "hello over websocket"
  }
}
```

The ACK means the message command completed and history can be pulled. It does not mean the peer has read the message.

### get_conversation_seqs

```json
{
  "requestId": "req-seqs-1",
  "command": "get_conversation_seqs",
  "payload": {
    "conversationIds": ["single:usr_000001:usr_000002"]
  }
}
```

### pull_messages

```json
{
  "requestId": "req-pull-1",
  "command": "pull_messages",
  "payload": {
    "conversationId": "single:usr_000001:usr_000002",
    "fromSeq": 1,
    "toSeq": 0,
    "limit": 50,
    "order": "asc"
  }
}
```

### mark_conversation_read

```json
{
  "requestId": "req-read-1",
  "command": "mark_conversation_read",
  "payload": {
    "conversationId": "single:usr_000001:usr_000002",
    "hasReadSeq": 1
  }
}
```

### heartbeat

```json
{
  "requestId": "req-heartbeat-1",
  "command": "heartbeat",
  "payload": {}
}
```

### Server Push Events

Push event envelope:

```json
{
  "type": "message_received",
  "data": {
    "server_msg_id": "msg_000001",
    "client_msg_id": "web-uuid-001",
    "conversation_id": "single:usr_000001:usr_000002",
    "seq": 1,
    "sender_id": "usr_000001",
    "receiver_id": "usr_000002",
    "chat_type": "single",
    "content_type": "text",
    "content": "hello",
    "send_time": 1777464000000,
    "created_at": 1777464000000
  }
}
```

Delivery status push event:

```json
{
  "type": "message_delivered",
  "data": {
    "server_msg_id": "msg_000001",
    "conversation_id": "single:usr_000001:usr_000002",
    "seq": 1,
    "sender_id": "usr_000001",
    "receiver_id": "usr_000002",
    "chat_type": "single",
    "content_type": "text"
  }
}
```

## Reconnect And Sync

After connect or reconnect:

1. Send `get_conversation_seqs`.
2. Compare each returned `maxSeq` with the local last seq.
3. If the client detects a gap, send `pull_messages` from the highest local contiguous seq plus one through server `maxSeq`.
4. Apply messages idempotently.
5. Send `mark_conversation_read` when the user has viewed the conversation.

Push events must be applied through the same ordering path: deduplicate by message identity, place confirmed messages by `conversationId + seq`, and repair missing seq ranges with `pull_messages`.

## Local Acceptance

The representative in-memory MVP smoke tests live in [../../tests/mvp_backend_test.go](../../tests/mvp_backend_test.go). Local backend startup and demo data scripts are documented in [../DEVELOPMENT.md](../DEVELOPMENT.md).
