# Message Storage Product Spec

Status: Draft
Owner: IM backend
Related message chain spec: [`./message-chain.md`](./message-chain.md)

## Goal

Define the product guarantees users and clients can rely on when messages are stored, retried, pulled, and marked as read.

This spec focuses on user-visible behavior. Storage engine details are covered by [`../design-docs/message-storage.md`](../design-docs/message-storage.md).

## Scope

Message storage must guarantee:

- idempotent send retries;
- stable message ordering inside each conversation;
- pullable message history by sequence range;
- monotonic conversation-level read state.
- persisted `message_origin=human|ai|system` plus AI trigger metadata for Agent replies.

## Non-goals

Phase 1 storage does not guarantee:

- online delivery to every recipient;
- push notification delivery;
- cross-device per-device read cursors;
- message recall/delete/edit behavior;
- reaction or typing state;
- media binary storage;
- end-to-end encryption semantics.

## User-Visible Guarantees

### Idempotent sends

When a client retries the same send request with the same `sender_id + client_msg_id` and the same message payload, the system returns the already accepted message instead of creating a duplicate.

The returned message keeps the original:

- `server_msg_id`;
- `conversation_id`;
- `seq`;
- `send_time`;
- content snapshot.

If a client reuses the same `client_msg_id` for a different payload, the system rejects the request as an idempotency conflict. The conflict protects users from accidentally sending a different message under an old retry key.

### Stable ordering

Every conversation has its own message sequence.

Product behavior:

- the first stored message in a conversation has `seq = 1`;
- each later stored message increases the conversation seq by exactly 1;
- two messages in the same conversation never share the same seq;
- seq order is the display and sync order inside that conversation;
- seq values are not comparable across different conversations.

If two users send at the same time in one conversation, the service chooses one durable order. Clients should display messages according to `seq`, not local send time.

### Pullable history

After a send response succeeds, the message is durable and can be pulled by `conversation_id + seq`.

Conversation participants who are allowed to see the message can discover the updated `max_seq` through conversation state and then pull the missing seq range.

Clients can recover missed messages with:

```text
from_seq = local_latest_seq + 1
to_seq = server_max_seq
```

Pull behavior:

- returned messages are ordered by seq unless another order is requested;
- pulling an empty valid range returns an empty list;
- retrying a pull does not change read state;
- clients can de-duplicate pulled messages by `conversation_id + seq` or `server_msg_id`.

### Conversation state

For each visible conversation, clients can query:

- `max_seq`: latest stored message seq;
- `has_read_seq`: latest seq the user has read;
- `unread_count`: derived as `max(0, max_seq - has_read_seq)`;
- `last_message`: latest message summary when available.

If a user has not read a visible conversation, `has_read_seq` behaves as `0`.

### Monotonic read state

Marking a conversation as read moves the user's `has_read_seq` forward only.

Product behavior:

- marking read to a higher seq advances `has_read_seq`;
- marking read to the same seq is idempotent;
- marking read to a lower seq leaves the stored `has_read_seq` unchanged;
- marking read beyond current `max_seq` is rejected;
- unread count never becomes negative.

This protects users from stale clients overwriting newer read progress.

## Send Acceptance Meaning

A successful send response means:

- the message has been accepted by message service;
- the message has been durably stored;
- a conversation seq has been assigned;
- the sender's own read state is at least the sent message seq.

It does not mean:

- every recipient is online;
- every recipient device received the message;
- push notification has been delivered;
- every recipient has read the message.

## Error Cases

Storage-backed behavior should distinguish:

- invalid or empty `client_msg_id`;
- idempotency conflict;
- invalid seq range;
- conversation not found or not visible to caller;
- read seq greater than max seq;
- storage temporarily unavailable.

The API/RPC layer can map these to user-facing error codes, but the product behavior must remain stable.

## Acceptance Criteria

Message storage behavior is ready for implementation when:

- storage design documents PostgreSQL and Redis responsibilities;
- idempotency uses `sender_id + client_msg_id`;
- ordering uses per-conversation `seq`;
- pull APIs can recover missed messages by seq range;
- `has_read_seq` updates are monotonic;
- static verification requires the storage spec and design document.
