# WebSocket Reconnect Sync

Status: Implemented
Owner: Gateway / Message Service integration
Related product spec: [`../product-specs/frontend-sync-contract.md`](../product-specs/frontend-sync-contract.md)
Related contracts:
- [`backend-mvp-contract.md`](./backend-mvp-contract.md)
- [`gateway-message-contract.md`](./gateway-message-contract.md)
- [`websocket-gateway.md`](./websocket-gateway.md)
- [`message-chain-contract.md`](./message-chain-contract.md)

## Background

The backend MVP needs a frontend-ready reconnect path before UI work starts. Gateway already routes message commands to `MessageLogic`, and Message Service already owns message history, per-conversation `seq`, `max_seq`, and `has_read_seq`. The missing polish is a stable frontend WebSocket ACK shape and explicit reconnect behavior for missed-message sync.

## Goals

- Emit frontend ACK fields: `requestId`, `command`, `status`, `payload`, and nested `error`.
- Keep existing Gateway aliases `request_id`, `type`, and `data` during the transition.
- Support reconnect sync through `get_conversation_seqs`, `pull_messages`, and `mark_conversation_read`.
- Make duplicate pulls safe by preserving stable message identities.
- Test invalid command errors with `requestId`, `status=error`, `error.code`, and `error.message`.

## Non-goals

- Do not persist delivery attempts or add retry/DLQ behavior.
- Do not implement per-device sync cursors.
- Do not change friends, groups, or membership business rules.
- Do not add observability beyond the existing command correlation fields.

## Command Envelope

Gateway accepts both current frontend fields and legacy aliases:

```json
{
  "requestId": "req-001",
  "command": "pull_messages",
  "payload": {}
}
```

Legacy accepted aliases are:

- `request_id` for `requestId`
- `type` for `command`

The response emits frontend fields and keeps the old fields for compatibility:

```json
{
  "requestId": "req-001",
  "request_id": "req-001",
  "command": "pull_messages",
  "type": "pull_messages",
  "status": "ok",
  "payload": {},
  "data": {}
}
```

New frontend code should read `requestId`, `command`, and `payload`.

## Error Envelope

All command errors use the backend MVP shape:

```json
{
  "requestId": "req-001",
  "command": "unknown_command",
  "status": "error",
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "unsupported command type"
  }
}
```

Gateway maps internal app errors to frontend codes:

| Internal code | WebSocket code |
| --- | --- |
| `UNAUTHENTICATED` | `UNAUTHORIZED` |
| `INVALID_ARGUMENT` | `VALIDATION_ERROR` |
| `NOT_FOUND` | `NOT_FOUND` |
| `ALREADY_EXISTS` | `CONFLICT` |
| `RATE_LIMITED` | `RATE_LIMITED` |
| other errors | `INTERNAL` |

`FORBIDDEN` is reserved for future authorization checks when those errors are introduced.

## Reconnect Algorithm

On reconnect, the frontend sends `get_conversation_seqs` with no conversation filter or with the conversations it has cached. Message Service returns `maxSeq`, `hasReadSeq`, `unreadCount`, `maxSeqTime`, and optional `lastMessage` for each visible conversation.

For each conversation:

1. Let `localLastSeq` be the highest seq merged in the client store.
2. If `localLastSeq < maxSeq`, send `pull_messages` with `fromSeq = localLastSeq + 1`, `toSeq = maxSeq`, `limit`, and `order=asc`.
3. Merge each returned message by `serverMsgId` or `conversationId + seq`.
4. When the user has viewed the conversation, send `mark_conversation_read` with the highest visible seq.

Gateway does not store local sync cursors. It only forwards the authenticated `user_id` to Message Service.

## Command Semantics

### `get_conversation_seqs`

The command injects the connection `user_id` and calls `MessageLogic.GetConversationSeqs`. With an empty `conversationIds` payload, the repository returns all visible conversation states for the user. The response `states` array is stable for frontend comparison and unread display.

### `pull_messages`

The command injects the connection `user_id` and calls `MessageLogic.PullMessages`. If `toSeq` is zero, Message Logic resolves it to the current `maxSeq`. Pulling the same range repeatedly is safe because message identity and `seq` do not change, and pull does not advance `hasReadSeq`.

### `mark_conversation_read`

The command injects the connection `user_id` and calls `MessageLogic.MarkConversationAsRead`. Message Service applies a monotonic read update. Lower seq values keep the current read state; seq values greater than `maxSeq` return `VALIDATION_ERROR`.

## Risks

- During the transition, responses include both frontend and legacy field names, which increases frame size. This is acceptable for MVP command ACKs and protects existing tests/tools.
- Gateway memory mode does not share state across processes. Local multi-service demos should use the PostgreSQL message repository when message state must be shared across processes.
- Online push can duplicate messages already recovered by pull. The frontend must deduplicate by stable message identity.

## Verification

Implementation is verified by:

- `tests/websocket_gateway_test.go` reconnect sync flow.
- duplicate-safe `pull_messages` test.
- missing-seq `pull_messages` test.
- invalid command error envelope test.
- required branch verification commands from [`../exec-plans/active/backend-mvp-completion.md`](../exec-plans/active/backend-mvp-completion.md).
