# Outbox Kafka Publisher

Status: Implemented for publisher module
Owner: Messaging infrastructure
Related docs:
- [`message-outbox.md`](./message-outbox.md)
- [`kafka-message-events.md`](./kafka-message-events.md)
- [`message-transfer-worker.md`](./message-transfer-worker.md)
- [`gateway-push-delivery.md`](./gateway-push-delivery.md)

## Background

Message Service synchronously stores accepted messages in PostgreSQL and writes a `message.created` row to `message_outbox` in the same transaction. That synchronous behavior stays unchanged: command ACKs still mean accepted and durably stored, not delivered to recipients.

This module bridges the already-implemented transactional outbox and the Kafka-compatible `message.events.v1` topic. It does not consume Kafka, route Gateway delivery, or implement Redis presence lookup.

## Goals

- Poll pending outbox rows through `repository.OutboxRepository`.
- Convert `message.created` outbox payloads into canonical `messaging.MessageEvent` values with `event_type=message.accepted`.
- Publish through the existing `messaging.Producer` abstraction, using the producer's configured `message.events.v1` topic.
- Preserve `conversation_id` as the event partition key through `MessageEvent.PartitionKey()`.
- Mark rows published only after producer success.
- Mark conversion and publish failures with retry metadata.
- Keep default `go test ./...` independent from PostgreSQL, Redis, Kafka, and Docker services.

## Non-goals

- No Kafka consumer loop.
- No Message Transfer consumer integration.
- No Gateway dispatcher integration.
- No Redis presence routing.
- No changes to synchronous send, pull, read, idempotency, or conversation seq behavior.

## Runtime Flow

`internal/outboxpublisher.Publisher.RunOnce` processes one batch:

1. Calls `PollPending(ctx, worker_id, batch_limit, lock_duration)`.
2. For each claimed `message.created` row, decodes `repository.MessageCreatedOutboxPayload`.
3. Builds `messaging.MessageEvent`:
   - `event_id` reuses the outbox `event_id`;
   - `event_type` becomes `message.accepted`;
   - `conversation_id`, `server_msg_id`, `seq`, `sender_id`, `chat_type`, and `created_at` come from the stored message with outbox metadata as fallback;
   - text content is encoded as JSON `{"text":"..."}`;
   - `message_origin`, `agent_account_id`, `trigger_server_msg_id`, `agent_run_id`, and `allow_recursive_trigger` are copied from the stored message;
   - `receiver_ids` excludes the sender and is derived from single-chat receiver or visible user IDs.
4. Calls `Producer.Publish(ctx, event)`.
5. Calls `MarkPublished` after successful publish.
6. Calls `MarkFailed` with `last_error` and `next_attempt_at` after conversion or publish errors.

Context cancellation is treated as worker shutdown. The publisher returns without marking the current row failed; the outbox lock expires and another run can retry it.

## Delivery Semantics

The pipeline is at-least-once:

- If publish succeeds but the process exits before `MarkPublished`, the row can be republished after lock expiry.
- If `MarkPublished` fails after broker success, the next worker can republish the same event.
- If publish fails before broker acknowledgement, the row is scheduled for retry.
- If conversion fails because payload is malformed or unsupported, the row is also marked failed for retry until its max attempts are exhausted.

Consumers must deduplicate by stable `event_id`. Delivery consumers should also tolerate repeated `server_msg_id` and repeated `conversation_id + seq`.

## Retry Policy

The publisher records a trimmed error string in `last_error`. When max attempts have not been reached, it sets `next_attempt_at = now + retry_backoff` and leaves the row pending. When max attempts are reached, `next_attempt_at` is zero and the repository marks the row terminal `failed`.

The PostgreSQL repository still owns the lock checks, `attempt_count` increment, and status transition. The publisher only supplies retry intent.

## Remaining Work

- Add a production entry point or config wiring when deployment ownership is decided.
- Add metrics for polled, published, failed, terminal failed, and publish latency.
- Add structured logs with `event_id`, `conversation_id`, `server_msg_id`, and worker ID.
- Add a Kafka consumer implementation for Message Transfer in a separate branch.
- Wire Message Transfer to Gateway delivery and Redis presence in separate branches.

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
```

Focused unit tests live in [`../../internal/outboxpublisher/publisher_test.go`](../../internal/outboxpublisher/publisher_test.go) and cover success, retryable producer failure, malformed payload handling, and context cancellation without PostgreSQL or Kafka.
