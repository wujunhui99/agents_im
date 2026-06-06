# Message Transfer Worker Phase 1

Status: Implemented for in-memory/noop phase 1  
Owner: IM backend  
Related docs:
- [`message-chain-contract.md`](./message-chain-contract.md)
- [`message-storage.md`](./message-storage.md)
- [`websocket-gateway.md`](./websocket-gateway.md)
- [`redis-presence.md`](./redis-presence.md)
- [`kafka-transfer-consumer.md`](./kafka-transfer-consumer.md)
- [`message-delivery-reliability.md`](./message-delivery-reliability.md)

## Background

Message Service phase 1 still owns synchronous send, pull, conversation seq, message storage, and read-state behavior. The future outbox and Kafka/Redpanda branches will emit accepted-message events after durable storage commits. Message Transfer consumes those events and dispatches delivery work without changing Message Service API behavior.

This phase adds the worker shape and testable contracts only. It does not require the real outbox repository, Kafka consumer, Redis presence lookup, or Gateway fanout to exist first.

## Goals

- Add `cmd/message-transfer` as the worker entry point.
- Define worker-facing event, consumer, dispatcher, retry, and idempotency interfaces under `internal/transfer`.
- Keep the default runtime as in-memory consumer plus noop dispatcher.
- Document how future OutboxRepository, EventBus, and DeliveryDispatcher implementations plug into the worker.
- Cover consume, dispatch, retryable failure, idempotency, and cancellation behavior with in-memory tests.

## Non-goals

- Do not remove or replace Message Service synchronous send, pull, or mark-as-read behavior.
- Do not implement a real Kafka/Redpanda consumer in this branch.
- Do not implement real Gateway push fanout, offline push, or delivery ACK persistence.
- Do not make Redis Presence authoritative for message history or read state.

## Runtime Shape

Entry point:

```text
cmd/message-transfer/main.go -f etc/message-transfer.yaml
```

Default config:

```yaml
Name: message-transfer
WorkerID: ${HOSTNAME}
DryRun: true
Consumer:
  Driver: memory
  Topic: message.accepted.v1
  Group: message-transfer
Dispatcher:
  Driver: noop
Worker:
  PollIntervalMillis: 100
  RetryBackoffMillis: 1000
  MaxAttempts: 5
```

The default `memory` consumer starts with an empty queue. The default `noop` dispatcher treats events as successfully accepted by the dispatch layer and does not call Gateway.

## Event Contract

`internal/transfer.MessageEvent` mirrors the future `message.accepted` event from [`message-chain-contract.md`](./message-chain-contract.md):

```text
event_id
event_type
conversation_id
seq
server_msg_id
sender_id
receiver_ids
created_at
trace_id
```

`Envelope` adds transport metadata used by Kafka/Redpanda or outbox consumers:

```text
topic
key
partition
offset
attempt
received_at
raw_payload
```

The idempotency key is resolved in this order: event id, envelope id, server message id, transport key. Future implementations should keep event id stable across retries.

## Interfaces

`EventConsumer` owns event intake and acknowledgement:

```text
Receive(ctx) -> Envelope
MarkSuccessful(ctx, envelope)
MarkRetry(ctx, envelope, retry_decision)
MarkFailed(ctx, envelope, process_result)
```

Future mappings:

- Outbox repository: `Receive` claims the next due row, `MarkSuccessful` marks delivered, `MarkRetry` writes `next_attempt_at`, and `MarkFailed` marks dead-letter or terminal failure.
- Kafka/Redpanda: `Receive` reads from a topic partition, `MarkSuccessful` commits offset after successful dispatch, and `MarkRetry` leaves the event uncommitted or publishes to a retry topic according to the final event-bus design.

`DeliveryDispatcher` owns delivery side effects:

```text
Dispatch(ctx, envelope) -> DispatchResult
```

Future mappings:

- Gateway dispatcher: resolve online connections through Redis Presence, route to the owning Gateway, and record delivery attempts.
- Push dispatcher: enqueue offline push after online dispatch rules are defined.

`IdempotencyStore` is the worker hook that prevents duplicate dispatch for the same accepted-message event. Phase 1 provides noop and in-memory stores; integration should replace this with Redis or durable idempotency when events can be redelivered across worker restarts.

`DeliveryAttemptRecorder` is the worker hook for durable delivery outcomes. The repository-backed implementation records Gateway per-recipient results as `delivered`, `offline`, or `failed` after retry policy has been applied. It does not record read state; read progress remains `has_read_seq`.

## Processing Semantics

`Worker.RunOnce` processes at most one event:

1. Receive one envelope from the consumer.
2. Check the idempotency key.
3. Dispatch if the key has not already been processed.
4. Record per-recipient delivery attempts when the dispatcher returns delivery results.
5. On success, mark the key processed and call `MarkSuccessful`.
6. On retryable failure, call `MarkRetry` with attempt, max attempts, backoff, and next attempt time.
7. When max attempts is reached, call `MarkFailed`.

`Worker.Run` repeats `RunOnce` until context cancellation. `Worker.Start` starts the loop in a goroutine and `Worker.Stop` cancels and waits for shutdown.

## Reliability Notes

- Message Service remains the durable write authority for accepted messages and conversation seq.
- Message Transfer delivery is at-least-once at the event level. Dispatchers must be idempotent, and the worker idempotency hook prevents duplicate work when the same key is seen again.
- Retry decisions are explicit data, not sleeps hidden inside the dispatcher. This lets future outbox and retry-topic implementations schedule retries outside the worker loop.
- Context cancellation must stop the loop without marking an in-flight failed event as successful.

## Verification

Required validation:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name "*.go" -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json $(find . -name "*.md" -not -path "./.git/*" -not -path "./docs/references/*" -print)
```

Current tests live in [`../../internal/transfer/worker_test.go`](../../internal/transfer/worker_test.go) and cover successful consume/dispatch, duplicate idempotency behavior, retryable failure, and context cancellation.

## Risks and Follow-ups

- In-memory idempotency is process-local. It is sufficient for phase 1 tests but must be replaced before a real multi-worker deployment.
- The noop dispatcher verifies worker control flow only. Real Gateway fanout needs presence-aware routing and delivery ACK design.
- Kafka/Redpanda retry behavior must be aligned with the final outbox/event-bus contract before production use.
