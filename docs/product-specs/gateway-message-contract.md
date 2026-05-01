# Gateway Message Contract Product Spec

Status: Draft
Owner: Gateway / IM backend
Related design: [`../design-docs/gateway-message-contract.md`](../design-docs/gateway-message-contract.md)

## Goal

Define the first client-visible message behavior over Gateway WebSocket commands while Message Service owns persistence, sequence, conversation state, and read state.

This spec covers first-phase client behavior for:

- sending a message after connection;
- pulling messages by conversation sequence;
- querying conversation sequence/read state;
- marking a conversation as read;
- receiving command ACKs.

## Non-goals for phase 1

The first contract intentionally does not require:

- a real WebSocket server implementation;
- online recipient fanout;
- offline push notification;
- per-device sync state;
- delivery retry queues;
- message recall/delete/edit/reactions;
- Gateway-owned message storage or read-state storage.

## User-visible model

The client is connected to Gateway as one authenticated user. The client sends command envelopes with a `requestId`, `command`, and `payload`. Gateway replies with one command ACK using the same `requestId`.

The connection user is authoritative. The client does not choose `senderId` or `userId` in message commands.

## Command ACK behavior

A command ACK has:

- `requestId`: copied from the command;
- `command`: copied from the command;
- `status`: `ok` or `error`;
- `payload`: command-specific success payload for `status=ok`;
- `error.code`: stable machine-readable result code for `status=error`;
- `error.message`: human-readable diagnostic text for failures.

Client-visible meaning:

- `status=ok` means the command completed at Message Service and the response payload is usable.
- `status=error` means the command did not complete; the client may retry according to the command type.
- For send, a successful command ACK means the message is stored. It does not mean recipients have seen it.
- Delivery ACKs for pushed messages are future behavior and are separate from command ACKs.

## Sending messages

Client command: `send_message`

The client provides:

- `chatType`: `single` or `group`;
- `receiverId`: required for single chat;
- `groupId`: required for group chat;
- `clientMsgId`: client-generated idempotency key;
- `contentType`: `text` in phase 1;
- `content`: text content.

Successful behavior:

- Gateway forwards the request with the connection user as sender.
- Message Service validates the target, stores the message, assigns `serverMsgId`, resolves `conversationId`, and assigns `seq`.
- The ACK payload returns the stored message snapshot and `deduplicated`; snapshots include `messageOrigin=human|ai|system` and AI metadata when present.

Retry behavior:

- If the client does not receive an ACK, it may retry with the same `clientMsgId`.
- Reusing the same `clientMsgId` for the same send returns the existing message.
- Reusing the same `clientMsgId` with different content returns an idempotency conflict.

## Pulling messages

Client command: `pull_messages`

The client provides:

- `conversationId`;
- `fromSeq`;
- `toSeq`;
- `limit`;
- `order`, defaulting to ascending when omitted.

Successful behavior:

- Gateway forwards the request with the connection user as `user_id`.
- Message Service verifies participation and returns messages in the requested range.
- Empty ranges return an empty list, not a client-visible failure.

Pulling messages does not mark the conversation as read.

## Querying conversation sequence state

Client command: `get_conversation_seqs`

The client may provide a list of `conversationIds`. If the list is empty or omitted, Message Service may return all visible conversation states for the user.

Successful behavior:

- The response includes `maxSeq`, `hasReadSeq`, `unreadCount`, optional `maxSeqTime`, and optional `lastMessage`.
- The client can compare local sequence with `maxSeq` and call `pull_messages` for missing ranges.

## Marking a conversation as read

Client command: `mark_conversation_read`

The client provides:

- `conversationId`;
- `hasReadSeq`.

Successful behavior:

- Gateway forwards the update with the connection user as `user_id`.
- Message Service applies a monotonic read update.
- The ACK payload returns the resulting `hasReadSeq`, `maxSeq`, `unreadCount`, and whether the state changed.

Client-visible rules:

- Marking a lower seq than the current read seq keeps the existing read state.
- Marking a seq greater than `maxSeq` fails.
- Gateway does not store read progress locally.

## Recommended sync flow

After connecting or reconnecting:

```text
get_conversation_seqs
for each conversation where local_seq < maxSeq:
  pull_messages from local_seq + 1 to maxSeq
after user views a conversation:
  mark_conversation_read with the highest visible seq
```

## Error behavior

Clients should handle these first-phase error classes:

- unauthenticated connection;
- invalid command or payload;
- target user, group, or conversation not found;
- forbidden conversation access;
- invalid sequence range;
- read seq greater than max seq;
- idempotency conflict;
- internal failure.

Error ACKs must not expose secrets, tokens, or internal credentials.

## Acceptance criteria

This product contract is ready when:

- the four command names are stable;
- client-visible send, pull, sequence query, read, and ACK semantics are documented;
- Gateway is explicitly excluded from message persistence and read-state ownership;
- implementation tests verify command names and pure request mapping.
