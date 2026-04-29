# Backend MVP Interface Contract

## Overview

This document freezes the contracts needed before frontend development starts. Feature branches may extend implementations, but they must keep these frontend-visible contracts stable unless this document is updated first.

## REST Auth Contract

### Register

```http
POST /auth/register
Content-Type: application/json
```

```json
{
  "identifier": "alice001",
  "password": "[REDACTED]",
  "displayName": "Alice"
}
```

Response returns user identity and an access token when supported by the existing auth flow.

### Login

```http
POST /auth/login
Content-Type: application/json
```

```json
{
  "identifier": "alice001",
  "password": "[REDACTED]"
}
```

Success returns:

```json
{
  "accessToken": "[REDACTED]",
  "expiresIn": 86400,
  "user": {
    "userId": "user_xxx",
    "identifier": "alice001",
    "displayName": "Alice"
  }
}
```

## REST User Contract

Protected endpoints use:

```http
Authorization: Bearer <access_token>
```

Required MVP operations:

- query current user profile.
- update own profile fields: display name, gender, age, region.
- query public user profile by user id or unique identifier.
- check whether identifier exists.

## Friends Contract

MVP semantics are immediate accepted friendship.

Required operations:

- add friend by user id or identifier.
- delete friend.
- list friends.
- query relation status.

Rules:

- self-add returns validation error.
- duplicate add is idempotent success.
- non-existent target returns not found.

## Groups Contract

Required operations:

- create group.
- join group.
- leave group.
- list my groups.
- list group members.

Rules:

- creator becomes owner/member.
- open join by default.
- group message send requires membership.

## WebSocket Connection Contract

Endpoint:

```text
GET /ws
Authorization: Bearer <access_token>
```

Query token is allowed only for frontend environments that cannot set headers:

```text
/ws?token=<access_token>
```

Server assigns:

- `connection_id`
- `gateway_id` / `instance_id`
- authenticated `user_id`

## WebSocket Command Envelope

Client sends:

```json
{
  "requestId": "req-001",
  "command": "send_message",
  "payload": {}
}
```

Success ACK:

```json
{
  "requestId": "req-001",
  "status": "ok",
  "payload": {}
}
```

Error ACK:

```json
{
  "requestId": "req-001",
  "status": "error",
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "invalid payload"
  }
}
```

Error codes:

- `UNAUTHORIZED`
- `VALIDATION_ERROR`
- `NOT_FOUND`
- `CONFLICT`
- `FORBIDDEN`
- `INTERNAL`

## Required WebSocket Commands

### heartbeat

```json
{"requestId":"req-heartbeat","command":"heartbeat","payload":{}}
```

### send_message

```json
{
  "requestId": "req-send-1",
  "command": "send_message",
  "payload": {
    "chatType": "single",
    "receiverId": "user_b",
    "clientMsgId": "client-generated-id",
    "contentType": "text",
    "content": "hello"
  }
}
```

For group chat use `chatType: "group"` and `groupId`.

ACK means message was accepted/persisted, not necessarily delivered/read.

### get_conversation_seqs

Returns per-conversation server max seq, current user read seq, and unread count.

### pull_messages

Pulls messages by conversation and seq range/from seq.

### mark_conversation_read

Advances `has_read_seq` monotonically.

## Server Push Events

### message_received

```json
{
  "event": "message_received",
  "payload": {
    "serverMsgId": "msg_xxx",
    "conversationId": "single:user_a:user_b",
    "seq": 10,
    "senderId": "user_a",
    "chatType": "single",
    "contentType": "text",
    "content": "hello",
    "sendTime": "2026-04-29T00:00:00Z"
  }
}
```

### message_delivered

```json
{
  "event": "message_delivered",
  "payload": {
    "serverMsgId": "msg_xxx",
    "recipientUserId": "user_b",
    "status": "delivered"
  }
}
```

## Reconnect Sync Contract

Detailed frontend reconnect behavior is defined in [`../product-specs/frontend-sync-contract.md`](../product-specs/frontend-sync-contract.md) and [`websocket-reconnect-sync.md`](./websocket-reconnect-sync.md).

After reconnect:

1. Client sends/calls `get_conversation_seqs`.
2. Client compares local last seq per conversation.
3. Client sends/calls `pull_messages` for missing ranges.
4. Client updates local unread state.

WebSocket delivery may be duplicated. Client must deduplicate by `serverMsgId` or `conversationId + seq`.

## Delivery Attempt Contract

Backend should expose or at least persist delivery attempt status for debugging and retry:

- server message id
- recipient user id
- status
- attempt count
- last error
- timestamps

## Health Contract

- `GET /healthz`: alive, no auth.
- `GET /readyz`: ready/dependency config, no secrets, no auth for local MVP unless deployment requires otherwise.
- `GET /metrics`: Prometheus text format if metrics are implemented.
