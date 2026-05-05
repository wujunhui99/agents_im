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
- Account is the identity/profile domain. V0 public fields named `user_id` are account id aliases and are intentionally preserved for frontend compatibility.

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
  "birth_date": "1996-05-02",
  "region": "Shanghai"
}
```

Success:

```json
{
  "code": "OK",
  "message": "ok",
  "data": {
    "user_id": "1001",
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

## Profile And Account Search

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
    "user_id": "1001",
    "identifier": "alice_001",
    "display_name": "Alice",
    "name": "Alice",
    "gender": "female",
    "birth_date": "1996-05-02",
    "region": "Shanghai",
    "account_type": "user",
    "avatar_media_id": "med_000001",
    "created_at": "2026-04-29T12:00:00Z",
    "updated_at": "2026-04-29T12:00:00Z"
  }
}
```

`account_type` is one of `user`, `agent`, or `admin`. Public registration and public account creation always create `user`; clients cannot self-select `agent` or `admin` through the frontend-visible REST API. Legacy server data that still contains `normal` is invalid and must be migrated before use.

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

### Update Avatar

```http
PATCH /me/avatar
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "mediaId": "med_000001"
}
```

The media object must belong to the current user, have `purpose=avatar`, `status=ready`, an allowed image MIME type, and be no larger than 5 MiB.

### Identifier Exists

```http
GET /users/exists?identifier=alice_001
```

Account alias:

```http
GET /accounts/exists?identifier=alice_001
```

### Public Profile By Identifier

```http
GET /users/alice_001
```

This is the current MVP user search path. The account alias is:

```http
GET /accounts/alice_001
```

`POST /accounts` is also accepted as an alias of `POST /users` for internal/profile creation flows. Existing frontend code may continue using `/users/*` and `user_id`.

### Current Gap

A frontend-visible `GET /users/id/:user_id` public-profile endpoint is not present in this worktree. Message conversation IDs contain internal account IDs, so clients can only show a friendly conversation title when they already have profile data from `/users/:identifier` or another real source. Otherwise, clients must show `未知联系人` instead of exposing the internal account ID or inventing a lookup path.

## Friends

Friendship is immediately accepted in MVP. Duplicate add is idempotent, self-add returns `INVALID_ARGUMENT`, and non-existent users return `NOT_FOUND`.
`user_id` and `friend_id` are account id aliases.

### Add Friend

```http
POST /friends
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "user_id": "2002"
}
```

Success:

```json
{
  "code": "OK",
  "message": "ok",
  "data": {
    "friendship": {
      "user_id": "1001",
      "friend_id": "2002",
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
GET /friends/2002
Authorization: Bearer <access_token>
```

### Delete Friend

```http
DELETE /friends/2002
Authorization: Bearer <access_token>
```

## Groups

The group creator is automatically an active member. Group chat V1 supports up to 200 active members total, including the creator.
Group detail and member-list reads require a bearer token and only active members can read them. Adding a different user requires the group creator/owner.
`creator_user_id`, `operator_user_id`, and member `user_id` are account id aliases.

### Create Group

```http
POST /groups
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "name": "Frontend Demo",
  "description": "MVP smoke room",
  "member_user_ids": ["2002", "2003"]
}
```

The backend deduplicates the creator and duplicate `member_user_ids`. More than 200 total active members returns `INVALID_ARGUMENT`.

### List Groups

```http
GET /groups
Authorization: Bearer <access_token>
```

Response data:

```json
{
  "groups": [
    {
      "group_id": "grp_000001",
      "name": "Frontend Demo",
      "description": "MVP smoke room",
      "creator_user_id": "1001",
      "created_at": "2026-05-05T12:00:00Z",
      "updated_at": "2026-05-05T12:00:00Z"
    }
  ]
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
  "user_id": "2002"
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

Member rows include human-readable profile fields when available: `identifier`, `display_name`, `name`, and `avatar_media_id`. Frontend UI must prefer those fields over raw internal account IDs.

## Media REST

Media endpoints are served by `user-api` in phase 1 and require `Authorization: Bearer <access_token>`.

### Create Upload Intent

```http
POST /media/uploads
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "purpose": "message_image",
  "filename": "cat.jpg",
  "contentType": "image/jpeg",
  "sizeBytes": 123456,
  "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "width": 1080,
  "height": 720
}
```

Success:

```json
{
  "code": "OK",
  "message": "ok",
  "data": {
    "mediaId": "med_000001",
    "objectKey": "users/1001/media/med_000001/cat.jpg",
    "uploadUrl": "http://localhost:9000/agents-im-media/...",
    "expiresAt": 1777464000000
  }
}
```

Clients must upload the bytes to `uploadUrl` with the declared `Content-Type`. Object keys are generated by the backend; clients must not send or persist arbitrary object keys as authority.

Supported purposes and limits:

- `avatar`: image JPEG/PNG/WebP/GIF, max 5 MiB.
- `message_image`: image JPEG/PNG/WebP/GIF, max 15 MiB. `POST /media/uploads` creates upload intents for images that can later be sent as message attachments.
- `message_file`: allowed document/archive/plain/octet-stream MIME types, max 20 MiB. `POST /media/uploads` creates upload intents for files that can later be sent as message attachments. HTML and SVG are not allowed in phase 1.

The frontend should check file size before requesting an upload intent so users get immediate feedback, but backend validation in `POST /media/uploads`, upload completion, and message send remains the source of truth.

### Complete Upload

```http
POST /media/uploads/med_000001/complete
Authorization: Bearer <access_token>
```

The backend verifies owner, pending status, object existence, size, and content type before changing status to `ready`. If MinIO/object storage is unreachable, the request fails visibly.

### Download URL

```http
GET /media/med_000001/download-url
Authorization: Bearer <access_token>
```

```json
{
  "code": "OK",
  "message": "ok",
  "data": {
    "mediaId": "med_000001",
    "downloadUrl": "http://localhost:9000/agents-im-media/...",
    "expiresAt": 1777464000000
  }
}
```

Download URLs require the requester to be the media owner or a conversation participant who can see a message attachment referencing the media. Non-message media remains owner-only.

## Messages REST

Message history is authoritative in the message service. WebSocket delivery is best-effort, may be duplicated, and may arrive out of order. Frontend display order must not use network arrival order.

Confirmed messages are ordered by numeric `seq` inside each `conversationId`; the pair `conversationId + seq` is the authoritative timeline position. `sendTime` is display metadata and is not a sorting authority. Clients should deduplicate repeated deliveries by `serverMsgId`; while a send is pending, the eventual server response should replace the optimistic item by `clientMsgId` and preserve canonical server fields including `serverMsgId` and `seq`.

Pending local messages without `seq` may remain after confirmed messages in local enqueue order. Same-conversation sends should be queued or the composer should be disabled with a visible sending state until the prior send is accepted or fails.

Message snapshots include `messageOrigin`, one of `human`, `ai`, or `system`. Frontend UI must visibly label `ai` messages as AI/Agent messages. AI messages also expose `agentAccountId`, `triggerServerMsgId`, `agentRunId`, and `allowRecursiveTrigger`; clients display these as metadata only and must not use them as authorization facts.

### Send Message

```http
POST /messages
Authorization: Bearer <access_token>
Content-Type: application/json
```

Single chat:

```json
{
  "receiverId": "2002",
  "chatType": "single",
  "clientMsgId": "web-uuid-001",
  "contentType": "text",
  "content": "hello"
}
```

Image message after media upload is completed:

```json
{
  "receiverId": "2002",
  "chatType": "single",
  "clientMsgId": "web-image-uuid-001",
  "contentType": "image",
  "content": "{\"mediaId\":\"med_000001\",\"width\":1080,\"height\":720}"
}
```

The `content` value is a JSON string with this image shape:

```json
{
  "mediaId": "med_000001",
  "width": 1080,
  "height": 720
}
```

File message after media upload is completed:

```json
{
  "receiverId": "2002",
  "chatType": "single",
  "clientMsgId": "web-file-uuid-001",
  "contentType": "file",
  "content": "{\"mediaId\":\"med_000002\",\"filename\":\"report.pdf\",\"sizeBytes\":123456,\"contentType\":\"application/pdf\"}"
}
```

The `content` value is a JSON string with this file shape:

```json
{
  "mediaId": "med_000002",
  "filename": "report.pdf",
  "sizeBytes": 123456,
  "contentType": "application/pdf"
}
```

For image/file messages, `mediaId` must reference a ready media object owned by the sender. Image media must have `purpose=message_image`, an allowed image MIME type, and size <= 15 MiB. File media must have `purpose=message_file`, allowed file MIME type, size <= 20 MiB, and the message `filename`, `sizeBytes`, and `contentType` metadata must match the media record. The message service rejects missing, not-ready, wrong-purpose, wrong-owner, over-limit, or metadata-mismatched media.

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
      "conversationId": "single:1001:2002",
      "seq": 1,
      "senderId": "1001",
      "receiverId": "2002",
      "chatType": "single",
      "contentType": "text",
      "content": "hello",
      "messageOrigin": "human",
      "agentAccountId": "",
      "triggerServerMsgId": "",
      "agentRunId": "",
      "allowRecursiveTrigger": false,
      "sendTime": 1777464000000,
      "createdAt": 1777464000000
    },
    "deduplicated": false
  }
}
```

### Get Conversation Seq States

```http
GET /conversations/seqs?conversationIds=single:1001:2002
Authorization: Bearer <access_token>
```

### Pull Messages

```http
GET /conversations/single:1001:2002/messages?fromSeq=1&limit=50&order=asc
Authorization: Bearer <access_token>
```

### Mark Conversation Read

```http
POST /conversations/single:1001:2002/read
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

The gateway authenticates the same JWT used by REST. Missing or invalid tokens fail the handshake with HTTP 401. Query-token auth is disabled by default and only works when `GatewayWS.AllowQueryToken=true`; the k3s production gateway enables it for browser-native WebSocket connections to `/ws?token=[REDACTED]` and configures the production browser origin explicitly. Browser access must match `GatewayWS.AllowedOrigins` exactly when origins are configured. Empty allowed origins only permit same-origin browser requests when the Gateway sees the same host/proto as the browser `Origin`, so production behind ingress should configure explicit public origins instead of relying on the empty-list fallback.

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
    "user_id": "1001",
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
    "receiverId": "2002",
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
    "conversationIds": ["single:1001:2002"]
  }
}
```

### pull_messages

```json
{
  "requestId": "req-pull-1",
  "command": "pull_messages",
  "payload": {
    "conversationId": "single:1001:2002",
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
    "conversationId": "single:1001:2002",
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
    "conversation_id": "single:1001:2002",
    "seq": 1,
    "sender_id": "1001",
    "receiver_id": "2002",
    "chat_type": "single",
    "content_type": "text",
    "content": "hello",
    "message_origin": "human",
    "agent_account_id": "",
    "trigger_server_msg_id": "",
    "agent_run_id": "",
    "allow_recursive_trigger": false,
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
    "conversation_id": "single:1001:2002",
    "seq": 1,
    "sender_id": "1001",
    "receiver_id": "2002",
    "chat_type": "single",
    "content_type": "text",
    "message_origin": "human"
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
