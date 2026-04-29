# Kafka Message Events

Status: Accepted  
Owner: IM backend / Messaging infrastructure  
Related docs:
- [`message-chain-contract.md`](./message-chain-contract.md)
- [`websocket-gateway.md`](./websocket-gateway.md)
- [`redis-presence.md`](./redis-presence.md)
- [`../product-specs/message-chain.md`](../product-specs/message-chain.md)

## Background

Message Service currently owns the synchronous write path: validate sender and conversation policy, assign `server_msg_id`, assign conversation `seq`, persist to PostgreSQL, and update read state. Gateway returns command ACKs after Message Service completes. That behavior remains unchanged.

The next pipeline steps need a Kafka-compatible event contract before separate branches implement the outbox, Message Transfer worker, Gateway fanout, and Push delivery. Local development uses Redpanda because it exposes Kafka APIs without a ZooKeeper dependency.

## Goals

- Provide local Redpanda in [`../../docker-compose.yml`](../../docker-compose.yml).
- Define a stable `message.events.v1` topic and JSON event schema.
- Provide a producer abstraction under [`../../internal/messaging/producer.go`](../../internal/messaging/producer.go).
- Keep ordinary `go test ./...` independent from a running broker.
- Preserve PostgreSQL as the source of truth for durable messages, conversation seqs, and read state.

## Non-goals

- Do not add an outbox table in this branch.
- Do not implement a transfer worker consume loop.
- Do not implement Gateway push fanout or delivery ACK persistence.
- Do not move current Message Service writes from PostgreSQL to Kafka.

## Local Redpanda

Compose service:

```text
redpanda
  internal Kafka listener: redpanda:9092
  host Kafka listener:     localhost:19092
  admin API:               localhost:9644
```

Development environment defaults are recorded in [`../../.env.example`](../../.env.example):

```text
KAFKA_BROKERS=localhost:19092
KAFKA_MESSAGE_EVENTS_TOPIC=message.events.v1
KAFKA_CONSUMER_GROUP=message-transfer-worker
```

Host processes should use `localhost:19092`. Services running inside the same compose network can use `redpanda:9092`.

## Topic

Canonical topic:

```text
message.events.v1
```

Producer key:

```text
conversation_id
```

Using `conversation_id` as the key keeps events for a conversation ordered within one partition. Global ordering across conversations is not guaranteed and should not be required.

## Event Schema

The canonical Go contract is [`../../internal/messaging/event.go`](../../internal/messaging/event.go). Events are encoded as JSON with snake_case field names:

```json
{
  "event_id": "evt_...",
  "event_type": "message.accepted",
  "conversation_id": "single:user_a:user_b",
  "server_msg_id": "msg_...",
  "seq": 1,
  "sender_id": "user_a",
  "chat_type": "single",
  "created_at": 1710000000000,
  "payload": {
    "client_msg_id": "client-generated-id",
    "receiver_id": "user_b",
    "receiver_ids": ["user_b"],
    "content_type": "text",
    "content": {"text": "hello"},
    "trace_id": "trace_..."
  }
}
```

Canonical top-level fields:

```text
event_id
event_type
conversation_id
server_msg_id
seq
sender_id
chat_type
created_at
payload
```

`message.accepted` requires `server_msg_id`, positive `seq`, and `sender_id`. `message.read` can leave message-specific fields empty and uses `payload.user_id`, `payload.has_read_seq`, and `payload.read_at`.

## Event Types

### message.accepted

Emitted after Message Service has durably accepted a message. In this branch, the event schema and producer abstraction exist, but Message Service is not yet wired to Kafka.

Consumers should treat `server_msg_id` and `conversation_id + seq` as durable message identity. The payload `receiver_ids` is a routing hint for transfer/push work, not an authorization source.

### message.read

Emitted after a user's `has_read_seq` advances. Consumers should treat read events as monotonic hints and remain idempotent by `event_id` and `user_id + conversation_id + has_read_seq`.

## Delivery Semantics

The intended pipeline is at-least-once:

1. Message Service writes PostgreSQL in the synchronous path.
2. A future outbox records the matching message event in the same database transaction.
3. A future outbox publisher writes the event to Redpanda/Kafka using `conversation_id` as key.
4. Message Transfer and Push consumers process events idempotently.

Consumers must deduplicate by `event_id`. For message delivery, they should also tolerate repeated `server_msg_id` or repeated `conversation_id + seq`.

## Producer Abstraction

[`../../internal/messaging/producer.go`](../../internal/messaging/producer.go) defines:

```text
Publish(ctx, event) error
Close() error
```

Available implementations:

- `NoopProducer`: validates events and performs no network I/O.
- `InMemoryProducer`: stores cloned events for unit tests.
- `KafkaProducer`: writes JSON events to a Kafka-compatible broker through `segmentio/kafka-go`.

The Redpanda integration test skips by default. Run it only when local Redpanda is available:

```bash
docker compose up -d redpanda
KAFKA_REDPANDA_INTEGRATION=1 KAFKA_BROKERS=localhost:19092 go test ./internal/messaging
```

## Ownership Boundaries

- Message Service owns durable message creation, seq allocation, and read state.
- Outbox owns transactional event capture and eventual publish.
- Message Transfer worker owns consuming `message.events.v1` and preparing delivery work.
- Gateway owns online connection fanout and delivery ACK behavior.
- Push service owns offline/device push channels.
- Redis Presence remains short-lived routing state and never becomes durable message authority.

## Verification

Required validation for this branch:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name "*.go" -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json $(find . -name "*.md" -not -path "./.git/*" -not -path "./.ai-context/*" -not -path "./docs/references/*" -print)
```
