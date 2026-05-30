# Kafka Transfer Consumer

> ⚠️ **已弃用（2026-05-30）**：Kafka consumer 分支在 V1 未启用（生产用 `Consumer.Driver: outbox`），`internal/transfer/kafka_consumer.go` 等已移除。message-transfer 现直读 PostgreSQL `message_outbox`，见 [`message-transfer-worker.md`](./message-transfer-worker.md)。本文保留为历史设计记录。

Status: Superseded (Kafka consumer branch removed)
Owner: Message Transfer
Related docs:
- [`kafka-message-events.md`](./kafka-message-events.md)
- [`message-transfer-worker.md`](./message-transfer-worker.md)
- [`message-outbox.md`](./message-outbox.md)
- [`gateway-push-delivery.md`](./gateway-push-delivery.md)

## Background

Message Service still accepts messages synchronously and keeps PostgreSQL as the source of truth for message history, conversation seqs, and read state. Kafka-compatible Redpanda provides the async event stream after outbox publication. Message Transfer needs a real consumer for `message.events.v1` without changing send, pull, read, or worker tests.

## Goals

- Add a Kafka-backed `transfer.EventConsumer` using `github.com/segmentio/kafka-go`.
- Consume canonical `messaging.MessageEvent` JSON from `message.events.v1`.
- Map `message.accepted` events into `transfer.Envelope` and `transfer.MessageEvent`.
- Make broker, topic, and group config explicit through `MessageTransferConfig` and `KafkaConfig`.
- Keep default `go test ./...` independent from Redpanda; integration tests must skip unless explicitly enabled.

## Non-goals

- Do not implement outbox polling or outbox-to-Kafka publishing.
- Do not implement Gateway push dispatch, Redis presence routing, offline push, or delivery ACK persistence.
- Do not move Message Service synchronous writes from PostgreSQL to Kafka.
- Do not consume `message.read` for delivery fanout in this branch.

## Runtime Configuration

`etc/message-transfer.yaml` keeps `Consumer.Driver: memory` as the safe local default. To run against Redpanda, set the driver to `kafka` and provide Kafka settings:

```yaml
Consumer:
  Driver: kafka
  Topic: ${KAFKA_MESSAGE_EVENTS_TOPIC}
  Group: ${KAFKA_CONSUMER_GROUP}

Kafka:
  Brokers: ${KAFKA_BROKERS}
  MessageEventsTopic: ${KAFKA_MESSAGE_EVENTS_TOPIC}
  ConsumerGroup: ${KAFKA_CONSUMER_GROUP}
```

If `Consumer.Topic` or `Consumer.Group` is omitted, the loader maps them from `Kafka.MessageEventsTopic` and `Kafka.ConsumerGroup`. Defaults remain:

```text
brokers: localhost:19092
topic:   message.events.v1
group:   message-transfer-worker
```

## Event Mapping

The consumer decodes [`internal/messaging.MessageEvent`](../../internal/messaging/event.go) and requires `event_type = message.accepted`.

| Kafka / messaging field | Transfer field |
| --- | --- |
| `event_id` | `Envelope.ID`, `MessageEvent.EventID` |
| `event_type` | `MessageEvent.EventType` |
| `conversation_id` | `Envelope.Key` fallback, `MessageEvent.ConversationID` |
| Kafka topic/key/partition/offset | `Envelope.Topic`, `Key`, `Partition`, `Offset` |
| Kafka value | `Envelope.RawPayload` |
| `server_msg_id`, `seq`, `sender_id`, `created_at` | matching `MessageEvent` fields |
| `payload.receiver_ids` plus `payload.receiver_id` | de-duplicated `MessageEvent.ReceiverIDs` |
| `payload.trace_id` | `MessageEvent.TraceID` |

Invalid JSON, schema validation errors, and non-`message.accepted` events are returned as receive errors and are not acknowledged. `message.read` remains a future consumer concern.

## Ack Semantics

The Kafka implementation uses `Reader.FetchMessage`, not auto-commit reads.

- `MarkSuccessful` commits the fetched offset through `CommitMessages` after dispatch and idempotency marking succeed.
- `MarkRetry` does not commit. This keeps the event eligible for redelivery after process restart or group rebalance, but it does not provide an in-place delay queue.
- `MarkFailed` does not commit in this branch. A later retry-topic or dead-letter policy must decide when terminal failures can be committed without silently dropping delivery work.

This keeps the existing `EventConsumer` seam explicit. The worker already calls `MarkSuccessful`, `MarkRetry`, and `MarkFailed`; only the Kafka-side retry and dead-letter storage policy remains open.

## Testing

Unit tests cover:

- Kafka message decode and transfer envelope mapping.
- Invalid JSON, invalid `message.accepted`, and unsupported `message.read` handling.
- Constructor behavior without a live broker.
- Broker/config mapping from `KafkaConfig` into `MessageTransferConfig`.
- Offset commit on success and no commit on retry/failure.

The Redpanda integration test is skipped unless both gates are present:

```bash
KAFKA_REDPANDA_INTEGRATION=1 KAFKA_BROKERS=localhost:19092 go test ./internal/transfer
```

## Risks and Follow-ups

- Retry without a retry topic means a running worker can continue to later offsets after a retryable dispatch result. Production deployment should add retry-topic, outbox-backed retry, or dead-letter handling before relying on delayed redelivery.
- In-memory idempotency remains process-local. Kafka redelivery across worker restarts needs Redis or durable idempotency before multi-worker production use.
- Gateway fanout, Redis presence routing, and Push dispatch remain separate branches.
