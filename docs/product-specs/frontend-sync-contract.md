# Frontend Reconnect Sync Contract

Status: Implemented
Owner: Gateway / frontend integration
Related design: [`../design-docs/websocket-reconnect-sync.md`](../design-docs/websocket-reconnect-sync.md)
Related backend contract: [`../design-docs/backend-mvp-contract.md`](../design-docs/backend-mvp-contract.md)

## Goal

Define the frontend-visible WebSocket behavior for reconnecting, detecting missed messages, pulling history, and marking messages read in the backend MVP.

The product rule is simple: WebSocket push is best effort, while Message Service history in PostgreSQL is authoritative. A reconnecting client must be able to rebuild local conversation state from server sequence state and message pulls.

## Scope

This contract covers:

- WebSocket command ACK envelopes used by reconnect and sync commands.
- `get_conversation_seqs` response shape.
- `pull_messages` response shape for missing ranges and duplicate-safe pulls.
- `mark_conversation_read` response shape.
- Client deduplication and read-state expectations.

## Non-goals

- No delivery attempt persistence, retry topic, or dead-letter queue.
- No per-device sync cursor owned by the backend.
- No friends, groups, or membership rule changes.
- No change to online push semantics beyond command correlation fields.

## Command ACK Envelope

Clients send commands with the frontend envelope:

```json
{
  "requestId": "req-001",
  "command": "get_conversation_seqs",
  "payload": {}
}
```

Successful ACKs return the same `requestId`, the command name, `status=ok`, and a command-specific `payload`:

```json
{
  "requestId": "req-001",
  "command": "get_conversation_seqs",
  "status": "ok",
  "payload": {}
}
```

Error ACKs return a stable error object:

```json
{
  "requestId": "req-001",
  "command": "pull_messages",
  "status": "error",
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "conversation_id is required"
  }
}
```

Frontend-visible WebSocket error codes for this MVP are:

- `UNAUTHORIZED`
- `VALIDATION_ERROR`
- `NOT_FOUND`
- `CONFLICT`
- `FORBIDDEN`
- `RATE_LIMITED`
- `INTERNAL`

## Reconnect Flow

After a fresh connect or reconnect, the client should:

1. Send `get_conversation_seqs`.
2. Compare local `lastSeq` with each server `maxSeq`.
3. For every conversation where `lastSeq < maxSeq`, send `pull_messages` with `fromSeq = lastSeq + 1` and `toSeq = maxSeq`.
4. Merge messages by `serverMsgId` or by `conversationId + seq`.
5. After the user views the conversation, send `mark_conversation_read` with the highest visible seq.

The client must tolerate duplicate server push and duplicate pull responses. Message identity is stable by `serverMsgId`; `conversationId + seq` is also stable inside one conversation.

## `get_conversation_seqs`

Request payload:

```json
{
  "conversationIds": ["single:user_a:user_b"]
}
```

If `conversationIds` is omitted or empty, the backend returns all conversations visible to the authenticated user.

Success payload:

```json
{
  "states": [
    {
      "conversationId": "single:user_a:user_b",
      "maxSeq": 10,
      "hasReadSeq": 7,
      "unreadCount": 3,
      "maxSeqTime": 1710000000000,
      "lastMessage": {
        "serverMsgId": "msg_010",
        "clientMsgId": "client-msg-010",
        "conversationId": "single:user_a:user_b",
        "seq": 10,
        "senderId": "user_a",
        "receiverId": "user_b",
        "chatType": "single",
        "contentType": "text",
        "content": "latest message",
        "sendTime": 1710000000000,
        "createdAt": 1710000000000
      }
    }
  ]
}
```

`hasReadSeq` is the server read state for the authenticated user. It is not the same as the client's local sync cursor.

## `pull_messages`

Request payload:

```json
{
  "conversationId": "single:user_a:user_b",
  "fromSeq": 8,
  "toSeq": 10,
  "limit": 50,
  "order": "asc"
}
```

Success payload:

```json
{
  "messages": [
    {
      "serverMsgId": "msg_008",
      "clientMsgId": "client-msg-008",
      "conversationId": "single:user_a:user_b",
      "seq": 8,
      "senderId": "user_a",
      "receiverId": "user_b",
      "chatType": "single",
      "contentType": "text",
      "content": "missed message",
      "sendTime": 1710000000000,
      "createdAt": 1710000000000
    }
  ],
  "isEnd": true,
  "nextSeq": 11
}
```

Rules:

- Pulling a range that overlaps local messages is valid. The client deduplicates during merge.
- Pulling an empty range returns `messages: []` with a usable `nextSeq`.
- Pulling does not mark messages read.
- `order` should be `asc` for reconnect sync unless the UI explicitly needs reverse pagination.

## `mark_conversation_read`

Request payload:

```json
{
  "conversationId": "single:user_a:user_b",
  "hasReadSeq": 10
}
```

Success payload:

```json
{
  "conversationId": "single:user_a:user_b",
  "hasReadSeq": 10,
  "maxSeq": 10,
  "unreadCount": 0,
  "updated": true
}
```

Read state is monotonic. Sending a lower `hasReadSeq` than the stored value returns the stored value and `updated=false`. Sending a seq greater than `maxSeq` returns `VALIDATION_ERROR`.

## Client Storage Expectations

The frontend should persist, per conversation:

- highest locally merged message seq;
- message records keyed by `serverMsgId` or `conversationId + seq`;
- last server `hasReadSeq` for unread display;
- pending command `requestId` only for in-flight command correlation.

The backend does not store a per-device local cursor in MVP.

## Acceptance Criteria

- Reconnect flow can recover messages sent while the client was offline.
- Repeating the same pull range returns stable message identities.
- Pulling from `localLastSeq + 1` returns only the missing seq range.
- Invalid commands return `requestId`, `status=error`, `error.code`, and `error.message`.
