# Read Receipts Product Spec

Status: Partially implemented
Owner: IM backend
Related design: [`../design-docs/read-receipts.md`](../design-docs/read-receipts.md)
Related message spec: [`message-chain.md`](./message-chain.md)

## Goal

Define client-visible behavior for marking conversations as read, computing unread counts, handling repeated or stale requests, and rejecting requests that exceed the current conversation `max_seq`.

## Scope

Phase 1 supports conversation-level read receipts:

```text
user_id + conversation_id -> has_read_seq
```

The client marks a conversation as read up to a sequence number. The service returns the resulting read cursor, conversation `max_seq`, and derived unread count.

Implementation status as of 2026-05-01:

- Current `main` supports repository/logic mutation, HTTP `POST /conversations/:conversation_id/read`, RPC `MarkConversationAsRead`, and Gateway `mark_conversation_read` command ACK.
- Current `main` does not complete `message.read` notification plumbing, server-pushed read receipt events, or client push ACKs.

## Non-goals

Remaining phase 1 gaps:

- `message.read` event emission only when the read cursor actually advances;
- Gateway read receipt server push and client push ACK;
- offline push notification;
- per-device read cursors;
- exact group read member lists;
- message recall/delete interactions.

## User-visible Concepts

### Read Cursor

`has_read_seq` is the highest contiguous message sequence the user has marked as read in a conversation.

If the user has never read the conversation:

```text
has_read_seq = 0
```

### Unread Count

The unread count shown in the conversation list is:

```text
unread_count = max(0, max_seq - has_read_seq)
```

Examples:

| max_seq | has_read_seq | unread_count |
| --- | --- | --- |
| 0 | 0 | 0 |
| 10 | 0 | 10 |
| 10 | 7 | 3 |
| 10 | 10 | 0 |
| 10 | 12 | 0, but this state is invalid and should be repaired |

## Mark as Read

Clients should mark a conversation as read when the user has actually viewed or intentionally cleared messages.

Common triggers:

- User opens a conversation and messages are rendered.
- User scrolls through older unread messages.
- User uses a "mark all as read" action for a conversation.
- Client restores foreground state and confirms the visible message range.

Clients should send the highest contiguous visible or locally synced `seq`. If there is a gap in local messages, the client should mark only up to the last contiguous visible `seq`, then pull missing messages before advancing further.

Request shape follows the message contract:

```text
POST /conversations/:conversation_id/read
```

```json
{
  "hasReadSeq": 10
}
```

Successful response:

```json
{
  "conversationId": "single:user_a:user_b",
  "hasReadSeq": 10,
  "maxSeq": 10,
  "unreadCount": 0,
  "updated": true
}
```

The server response is authoritative. Clients should update local conversation state from the response.

## Duplicate Requests

Repeated requests with the same `hasReadSeq` are valid.

Behavior:

- The server keeps the same `has_read_seq`.
- `updated = false`.
- `unread_count` is recomputed and returned.
- The client should treat the response as success.

This allows clients to retry safely after network timeouts.

## Rollback Requests

A request lower than the current read cursor is a rollback request.

Example:

```text
current has_read_seq = 8
request hasReadSeq = 5
```

Behavior:

- The server keeps `has_read_seq = 8`.
- `updated = false`.
- `unread_count` is recomputed from the kept cursor.
- The client should replace its local state with the returned server state.

The product does not support manually making already read messages unread in phase 1.

## Requests Greater Than max_seq

A request greater than the current conversation `max_seq` is invalid.

Example:

```text
max_seq = 10
request hasReadSeq = 11
```

Behavior:

- The server rejects the request.
- The read cursor is not changed.
- The client should refresh conversation seq state and pull missing messages if needed.

The server must reject this request instead of clamping it to `max_seq`. Rejecting prevents clients from hiding unread messages they have not synced.

## Sender and Receiver Behavior

### Sender

After a user sends a message successfully, the sender should not see that sent message as unread. The send path should advance the sender's own read cursor to at least the sent message `seq`.

### Receiver

When the receiver reads a single-chat conversation, the sender may see messages as read when the receiver's `has_read_seq` reaches those message sequences.

For display:

- Only messages authored by the viewing user should show peer-read status.
- A read receipt means the peer marked the conversation as read up to that `seq`.
- It does not prove every device displayed the message.

## Multi-device Behavior

Phase 1 read state is account-level, not device-level.

If one device marks a conversation as read to seq `20`, another device sending seq `15` later does not roll the cursor back. The second device should accept the server response and update its local state to seq `20`.

## Offline and Resync Behavior

On login or reconnect, clients should query conversation seq state:

```text
max_seq
has_read_seq
unread_count
```

If `max_seq > local_max_seq`, the client should pull missing messages. If `has_read_seq` returned by the server is higher than local read state, the client should update local state without sending a rollback.

## Future Group Behavior

Group conversations use the same account-level read cursor:

```text
member_user_id + group_conversation_id -> has_read_seq
```

Phase 1 clients may use this only for the current user's unread count. Future clients may show group read counts or member lists after privacy and aggregation rules are defined.

## Acceptance Criteria

- The client can mark a conversation read up to a valid seq.
- Unread count is derived as `max(0, max_seq - has_read_seq)`.
- Duplicate mark-as-read requests are successful and idempotent.
- Lower mark-as-read requests do not reduce read state.
- Requests greater than `max_seq` are rejected.
- HTTP/RPC and Gateway command ACK return the authoritative read state when the mark-read command succeeds.
- Read receipt server push and push ACK remain unfinished until `message.read` notification plumbing is connected.
