# Gateway to Message Service Contract

Status: Draft
Owner: Gateway / IM backend
Related product spec: [`../product-specs/gateway-message-contract.md`](../product-specs/gateway-message-contract.md)
Depends on: [`message-chain-contract.md`](./message-chain-contract.md), [`websocket-reliability.md`](./websocket-reliability.md)

## Background

Gateway and Message Service need a stable boundary so WebSocket work and message-chain work can proceed in parallel. Message Service owns message persistence, `server_msg_id`, per-conversation `seq`, conversation `max_seq`, and `user_id + conversation_id -> has_read_seq`. Gateway owns authenticated long connections, command decoding, command ACKs, and online delivery.

This document defines only the WebSocket command to Message Service RPC mapping for phase 1. It intentionally avoids importing generated message-service code so the gateway contract branch can be merged independently.

## Goals

- Define the first WebSocket command names for message operations.
- Map each command payload to the corresponding Message Service RPC request.
- Define first-phase command response and ACK semantics.
- Keep Gateway free of message persistence, seq allocation, and read-state ownership.

## Non-goals

- Do not implement a real WebSocket server in this contract.
- Do not save messages or read state in Gateway.
- Do not allocate message `seq` or generate `server_msg_id` in Gateway.
- Do not implement offline push, retry queues, Kafka, or fanout workers.
- Do not depend on `feature/message-service-contract` generated code.

## Boundary

Gateway responsibilities:

- Authenticate or receive the authenticated user context for the connection.
- Decode WebSocket command envelopes.
- Inject the connection user into Message Service RPC requests as `sender_id` or `user_id`.
- Forward command payloads to Message Service RPC.
- Return command ACKs to the client.
- Later, deliver accepted/read events to online connections.

Message Service responsibilities:

- Validate sender/user/conversation membership.
- Resolve `conversation_id`.
- Enforce send idempotency through `sender_id + client_msg_id`.
- Generate `server_msg_id` and conversation `seq`.
- Persist messages and conversation state.
- Maintain `has_read_seq` and `unread_count` semantics.

## WebSocket command envelope

The first-phase command envelope is transport-level and stable across commands:

```json
{
  "requestId": "client-request-id",
  "command": "send_message",
  "payload": {}
}
```

`requestId` is client-generated and scoped to one connection. It is used only to correlate command ACKs. It is not the message idempotency key. Message idempotency uses `clientMsgId` inside `send_message.payload`.

## ACK envelope

Gateway replies with one ACK per command:

```json
{
  "requestId": "client-request-id",
  "command": "send_message",
  "status": "ok",
  "code": "OK",
  "message": "",
  "payload": {}
}
```

First-phase ACK meaning:

- `status=ok` means Gateway received a successful Message Service RPC result and returned it to the client.
- For `send_message`, `status=ok` means the message was accepted and stored by Message Service.
- It does not mean every recipient received the message online.
- Recipient delivery ACK is a later delivery-layer ACK and must not be conflated with command ACK.

## Command mapping summary

| WebSocket command | Message Service RPC | Gateway-owned fields | Message-owned fields |
| --- | --- | --- | --- |
| `send_message` | `SendMessage` | connection user as `sender_id` | `server_msg_id`, `conversation_id`, `seq`, storage |
| `pull_messages` | `PullMessages` | connection user as `user_id` | range validation, message read model |
| `get_conversation_seqs` | `GetConversationSeqs` | connection user as `user_id` | `max_seq`, `has_read_seq`, `unread_count` |
| `mark_conversation_read` | `MarkConversationAsRead` | connection user as `user_id` | monotonic read-state update |

## `send_message`

Command payload:

```json
{
  "chatType": "single",
  "receiverId": "user_b",
  "groupId": "",
  "clientMsgId": "client-generated-message-id",
  "contentType": "text",
  "content": "hello"
}
```

RPC request mapping:

| Command source | RPC field |
| --- | --- |
| connection `user_id` | `sender_id` |
| `payload.receiverId` | `receiver_id` |
| `payload.groupId` | `group_id` |
| `payload.chatType` | `chat_type` |
| `payload.clientMsgId` | `client_msg_id` |
| `payload.contentType` | `content_type` |
| `payload.content` | `content` |

Command response payload:

```json
{
  "message": {
    "serverMsgId": "msg_...",
    "clientMsgId": "client-generated-message-id",
    "conversationId": "single:user_a:user_b",
    "seq": 1,
    "senderId": "user_a",
    "receiverId": "user_b",
    "groupId": "",
    "chatType": "single",
    "contentType": "text",
    "content": "hello",
    "sendTime": 1710000000000,
    "createdAt": 1710000000000
  },
  "deduplicated": false
}
```

Gateway must not retry a failed send by inventing a new `clientMsgId`. If the client retries the same send command with the same `clientMsgId`, Message Service owns idempotency.

## `pull_messages`

Command payload:

```json
{
  "conversationId": "single:user_a:user_b",
  "fromSeq": 1,
  "toSeq": 50,
  "limit": 50,
  "order": "asc"
}
```

RPC request mapping:

| Command source | RPC field |
| --- | --- |
| connection `user_id` | `user_id` |
| `payload.conversationId` | `conversation_id` |
| `payload.fromSeq` | `from_seq` |
| `payload.toSeq` | `to_seq` |
| `payload.limit` | `limit` |
| `payload.order` | `order` |

Command response payload:

```json
{
  "messages": [],
  "isEnd": true,
  "nextSeq": 51
}
```

Gateway does not mark messages as read when pulling. Read state changes only through `mark_conversation_read` or Message Service send-side behavior.

## `get_conversation_seqs`

Command payload:

```json
{
  "conversationIds": ["single:user_a:user_b", "group:g1"]
}
```

RPC request mapping:

| Command source | RPC field |
| --- | --- |
| connection `user_id` | `user_id` |
| `payload.conversationIds` | `conversation_ids` |

If `conversationIds` is empty or omitted, Message Service may return all visible conversation states for the user, following `message-chain-contract.md`.

Command response payload:

```json
{
  "states": [
    {
      "conversationId": "single:user_a:user_b",
      "maxSeq": 10,
      "hasReadSeq": 7,
      "unreadCount": 3,
      "maxSeqTime": 1710000000000,
      "lastMessage": {}
    }
  ]
}
```

## `mark_conversation_read`

Command payload:

```json
{
  "conversationId": "single:user_a:user_b",
  "hasReadSeq": 10
}
```

RPC request mapping:

| Command source | RPC field |
| --- | --- |
| connection `user_id` | `user_id` |
| `payload.conversationId` | `conversation_id` |
| `payload.hasReadSeq` | `has_read_seq` |

Command response payload:

```json
{
  "conversationId": "single:user_a:user_b",
  "hasReadSeq": 10,
  "maxSeq": 10,
  "unreadCount": 0,
  "updated": true
}
```

Gateway does not store `has_read_seq`; it forwards the monotonic update to Message Service and returns the resulting state.

## Error mapping

Gateway should preserve Message Service error categories in the command ACK `code`. Recommended phase 1 codes:

| Code | Meaning |
| --- | --- |
| `OK` | Command succeeded |
| `UNAUTHENTICATED` | Connection has no authenticated user |
| `INVALID_ARGUMENT` | Command payload is malformed or has invalid fields |
| `FORBIDDEN` | User is not allowed to access the conversation |
| `NOT_FOUND` | Target user, group, or conversation was not found |
| `IDEMPOTENCY_CONFLICT` | `clientMsgId` was reused with different send payload |
| `INTERNAL` | Unexpected Gateway or Message Service failure |

Gateway may validate envelope shape before RPC. Business validation remains in Message Service.

## Reliability notes

This contract aligns with [`websocket-reliability.md`](./websocket-reliability.md):

- Command ACK confirms command processing, not recipient delivery.
- Delivery ACK for server-pushed messages is a separate future path.
- Reconnect compensation should use `get_conversation_seqs` and `pull_messages`.
- Gateway connection state must remain separate from Message Service read state.

## Verification

The first contract is verifiable when:

- design and product docs define the four command mappings;
- `internal/gateway` exposes command constants and pure mapping functions;
- tests verify command names and mapped fields without importing message-service code;
- static verification requires the gateway-message docs and tests.
