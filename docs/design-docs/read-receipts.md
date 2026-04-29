# Read Receipts Design

Status: Draft
Owner: IM backend
Related product spec: [`../product-specs/read-receipts.md`](../product-specs/read-receipts.md)
Related message contract: [`message-chain-contract.md`](./message-chain-contract.md)

## Background

The message-chain contract defines conversation sequence, message pulling, and conversation-level read state. This document narrows the read receipt contract so read-state repository, notification, and gateway work can proceed in parallel without depending on a complete message handler.

Phase 1 uses conversation-level read state:

```text
user_id + conversation_id -> has_read_seq
```

`has_read_seq` means the highest contiguous conversation `seq` the user has marked as read.

## Goals

- Define the read-state model and monotonic update rules.
- Define boundary behavior for duplicate, rollback, and out-of-range mark-as-read requests.
- Define sender and receiver read semantics for single chat.
- Keep the contract compatible with future group read receipts.
- Keep Gateway, notification, and repository responsibilities separate.

## Non-goals

- Implement message send, message persistence, or Gateway handlers.
- Track per-device read cursors.
- Track per-message read rows in phase 1.
- Guarantee online delivery or client rendering through read state alone.
- Define final privacy rules for group read member lists.

## State Model

### Conversation Sequence

Each conversation has an authoritative `max_seq`.

```text
conversation_id -> max_seq
```

Rules:

- `max_seq` starts at `0` before the first persisted message.
- The first persisted message has `seq = 1`.
- `max_seq` is monotonically increasing.
- A valid read cursor must never exceed the current authoritative `max_seq`.

### User Read State

Each participant has one account-level read cursor per conversation.

```text
user_id         string
conversation_id string
has_read_seq   int64
updated_at      int64
```

Recommended uniqueness:

```text
unique(user_id, conversation_id)
```

Default state:

- If no row exists, `has_read_seq = 0`.
- `0` means the user has not marked any message in this conversation as read.

Derived state:

```text
unread_count = max(0, max_seq - has_read_seq)
```

`unread_count` is derived from authoritative conversation state and user read state. It may be cached later, but the cache must be invalidated or recomputed when either `max_seq` or `has_read_seq` changes.

## Mark-as-Read Normalization

Inputs:

```text
current_has_read_seq
requested_has_read_seq
max_seq
```

Validation:

- Reject negative `current_has_read_seq`.
- Reject negative `requested_has_read_seq`.
- Reject negative `max_seq`.
- Reject `requested_has_read_seq > max_seq`.
- Treat `current_has_read_seq > max_seq` as a storage invariant violation that must not be silently accepted.

Normalization:

```text
if requested_has_read_seq > current_has_read_seq:
    next_has_read_seq = requested_has_read_seq
    updated = true
else:
    next_has_read_seq = current_has_read_seq
    updated = false

unread_count = max(0, max_seq - next_has_read_seq)
```

Important boundary cases:

- Repeating the same request is idempotent and returns `updated = false`.
- Sending a lower seq is a rollback request and does not change state.
- Sending `requested_has_read_seq == max_seq` is valid and produces `unread_count = 0`.
- Sending `requested_has_read_seq > max_seq` is rejected, not clamped.

## Monotonic Persistence Rule

Repository implementations should update by max operation, not by blind assignment.

SQL-style shape:

```text
UPDATE user_conversation_read_state
SET has_read_seq = GREATEST(has_read_seq, :requested_has_read_seq),
    updated_at = :now
WHERE user_id = :user_id
  AND conversation_id = :conversation_id
  AND :requested_has_read_seq <= :max_seq;
```

If no row exists, insert with `has_read_seq = requested_has_read_seq`. Concurrent writers must not allow a lower request to overwrite a higher cursor.

Event emission:

- Emit `message.read` only when the persisted cursor advances.
- Do not emit an event for duplicate or rollback requests.
- If the update races with a new message, the new message remains unread unless the request explicitly included its `seq`.

## Sender and Receiver Semantics

### Sender's Own Read State

When a user sends a message, the send path should advance the sender's own `has_read_seq` to at least the sent message `seq`.

This prevents the sender from seeing their own newly sent message as unread in the conversation list.

### Receiver Read Meaning

When a receiver marks a single-chat conversation as read up to `N`, the system records:

```text
receiver_user_id + conversation_id -> has_read_seq = N
```

For the sender UI, a message sent by the sender can be displayed as read by the receiver when:

```text
receiver_has_read_seq >= message.seq
```

This read indication applies only to messages authored by the viewer's peer. It does not mean every participant or every receiver device acknowledged online delivery.

### Viewer-specific Display

In a single chat between `user_a` and `user_b`:

- `user_a` uses `user_a`'s own read state to compute `user_a`'s unread count.
- `user_b` uses `user_b`'s own read state to compute `user_b`'s unread count.
- `user_a` may use `user_b`'s read state to display read status for messages authored by `user_a`.
- `user_b` may use `user_a`'s read state to display read status for messages authored by `user_b`.

## Event Contract

The future read notification event is:

```json
{
  "eventId": "evt_...",
  "eventType": "message.read",
  "conversationId": "single:user_a:user_b",
  "userId": "user_b",
  "hasReadSeq": 10,
  "readAt": 1710000000000
}
```

Ownership:

- Message service owns read-state mutation and `message.read` event creation.
- Notification or message-transfer workers own durable fanout when introduced.
- Gateway owns WebSocket command mapping and online delivery.
- Gateway ACK confirms receipt of a pushed event, not mutation of read state.

## Error Contract

The read path should distinguish:

- unauthenticated caller;
- caller is not a conversation participant;
- conversation does not exist;
- negative read seq;
- read seq greater than current `max_seq`;
- storage invariant violation, such as persisted `has_read_seq > max_seq`;
- transient repository failure.

Client-visible APIs should return a validation error for `requested_has_read_seq > max_seq`. Clients should refresh conversation state instead of retrying the same invalid request.

## Future Group Read Extensions

The primary state model remains:

```text
user_id + conversation_id -> has_read_seq
```

This already supports group member read cursors. Future group features should add query and aggregation layers rather than replacing the primary key.

Possible extensions:

- Per-message group read summary:

```text
conversation_id
seq
read_count
member_count
unread_count
updated_at
```

- Per-message read member query:

```text
conversation_id
seq
page_token
limit
```

- Privacy policy flags controlling whether clients can see exact readers, counts only, or no group read state.
- Membership-version snapshots so historical read counts are calculated against the member set that could receive the message.

The phase 1 contract should avoid storing one row per message per member unless a later product requirement needs exact historical read membership.

## Validation

Minimum verification for this contract:

- `unread_count` never goes below zero.
- Mark-as-read advances from lower to higher seq.
- Duplicate mark-as-read is idempotent.
- Rollback mark-as-read does not reduce `has_read_seq`.
- Mark-as-read greater than `max_seq` is rejected.
