# Message Outbox Design

Status: Implemented for repository contract  
Owner: Message Outbox  
Related docs:
- [`message-chain-contract.md`](./message-chain-contract.md)
- [`message-storage.md`](./message-storage.md)
- [`postgres-persistence.md`](./postgres-persistence.md)
- [`websocket-gateway.md`](./websocket-gateway.md)
- [`outbox-kafka-publisher.md`](./outbox-kafka-publisher.md)
- [`message-delivery-reliability.md`](./message-delivery-reliability.md)

## Background

Message Service currently accepts a send command synchronously, allocates `server_msg_id` and conversation `seq`, stores the message, updates conversation/read state, and returns an ACK to HTTP or WebSocket callers. Kafka fanout, Message Transfer, and Gateway Push delivery are separate follow-up branches.

The outbox adds a durable event source inside PostgreSQL so workers can publish the already-accepted message without coupling delivery success to the send response. The first publisher module is described in [`outbox-kafka-publisher.md`](./outbox-kafka-publisher.md).

## Goals

- Write one `message.created` outbox event in the same PostgreSQL transaction as the accepted message.
- Preserve current synchronous `send_message` response behavior.
- Provide a repository contract for workers to poll pending events, lock them, mark them published, or schedule retry after failure.
- Keep normal `go test ./...` independent from PostgreSQL through the in-memory repository.

## Non-goals

- No Kafka producer or consumer in this branch.
- No Message Transfer worker in this branch.
- No Gateway push fanout or delivery ACK in this branch.
- No removal of existing synchronous message persistence or synchronous command ACK behavior.

## Schema

Migration: [`../../db/migrations/001_init_postgres.sql`](../../db/migrations/001_init_postgres.sql); Agent message origin metadata is extended in [`../../db/migrations/003_agent_conversation_hosting.sql`](../../db/migrations/003_agent_conversation_hosting.sql).

Table: `message_outbox`

Required fields:

```text
event_id
event_type
aggregate_type
aggregate_id
conversation_id
server_msg_id
seq
payload jsonb
status
attempt_count
next_attempt_at
locked_by
locked_until
created_at
updated_at
published_at
```

Additional field:

```text
last_error
```

Constraints:

- `event_id` is the primary key.
- `(event_type, aggregate_type, aggregate_id)` is unique, giving one logical event per message aggregate and event type.
- `server_msg_id` references `messages(server_msg_id)`.
- `status` is one of `pending`, `published`, or `failed`.
- `attempt_count >= 0` and `seq > 0`.

Indexes:

- pending poll index on due `pending` events: `(next_attempt_at, created_at, event_id)`.
- lock expiry index on `locked_until` for pending rows.
- lookup indexes on `(conversation_id, seq)` and `server_msg_id`.

## Event Contract

Current event:

```text
event_type: message.created
aggregate_type: message
aggregate_id: server_msg_id
```

Payload shape:

```json
{
  "message": {
    "serverMsgId": "msg_000001",
    "clientMsgId": "client-1",
    "conversationId": "single:usr_a:usr_b",
    "seq": 1,
    "senderId": "usr_a",
    "receiverId": "usr_b",
    "groupId": "",
    "chatType": "single",
    "contentType": "text",
    "content": "hello",
    "messageOrigin": "human",
    "agentAccountId": "",
    "triggerServerMsgId": "",
    "agentRunId": "",
    "allowRecursiveTrigger": false,
    "sendTime": 1710000000000,
    "createdAt": 1710000000000
  },
  "visible_user_ids": ["usr_a", "usr_b"]
}
```

The payload is for downstream Kafka/Transfer/Push workers. It does not mean the recipient has received the message.

## Transaction Boundary

`PostgresMessageRepository.CreateMessageIdempotent` owns the send transaction:

1. Check idempotency.
2. Upsert and lock `conversation_threads`.
3. Allocate the next conversation `seq`.
4. Insert `messages`.
5. Insert `message_idempotency_keys`.
6. Upsert visible `user_conversation_states`.
7. Advance sender `has_read_seq`.
8. Update conversation `max_seq` and last message fields.
9. Insert `message_outbox`.
10. Commit.

If the outbox insert fails, the whole send transaction rolls back. Idempotent retries return the original message and do not create another outbox row.

The same transaction also creates accepted `delivery_attempts` rows for message recipients. When a worker successfully marks the outbox row `published`, the repository advances those attempts from `accepted` to `published` without downgrading later delivery outcomes.

## Repository Contract

Interface: `internal/repository.OutboxRepository`

Methods:

- `PollPending(ctx, workerID, limit, lockDuration)` locks due pending rows and returns them to one worker.
- `MarkPublished(ctx, eventID, workerID)` marks a locked event as published and clears the lock.
- `MarkFailed(ctx, eventID, workerID, failure)` increments `attempt_count`, records `last_error`, clears the lock, and either schedules retry as `pending` or marks the event `failed`.

The PostgreSQL implementation uses `FOR UPDATE SKIP LOCKED` so multiple workers can poll concurrently without selecting the same pending event. A worker can only mark rows it currently owns through `locked_by` and an unexpired `locked_until`.

The in-memory message repository implements the same interface for unit tests and isolated local runs.

## Publisher Semantics

Outbox publisher workers should treat outbox events as at-least-once inputs:

- A published Kafka message may be retried if the worker fails before `MarkPublished`.
- Downstream consumers must use `event_id` or `(event_type, aggregate_type, aggregate_id)` for idempotency.
- `send_message` ACK remains an acceptance/storage ACK, not a delivery ACK.

## Validation

Required validation for this branch:

- `goctl --version`
- `for f in api/*.api; do goctl api validate -api "$f"; done`
- `gofmt -w $(find . -name "*.go" -print)`
- `go test ./...`
- `bash scripts/verify-static.sh`
- `docker compose config`
- markdown link check for changed docs
