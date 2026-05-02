# Transfer Gateway Dispatcher

Status: Implemented  
Owner: Message Transfer / Gateway  
Related docs:
- [`message-transfer-worker.md`](./message-transfer-worker.md)
- [`gateway-push-delivery.md`](./gateway-push-delivery.md)
- [`kafka-message-events.md`](./kafka-message-events.md)
- [`message-chain-contract.md`](./message-chain-contract.md)

## Background

Message Transfer owns accepted-message event processing, idempotency, and retry decisions. Gateway owns WebSocket connection fanout through `internal/gateway/delivery.Dispatcher`. This adapter bridges those two contracts so a transfer `Envelope` can be dispatched to Gateway push delivery without adding Kafka consumption, outbox publishing, Redis cross-instance routing, or remote Gateway network calls.

## Goals

- Implement a `transfer.DeliveryDispatcher` adapter under `internal/transfer/gateway`.
- Convert `message.accepted` transfer events into Gateway `message_received` delivery events.
- Deliver only to direct recipient user IDs already present on the transfer event.
- Make offline and malformed-recipient outcomes deterministic.
- Preserve the worker's existing idempotency and `RetryDecision` control flow.

## Non-goals

- No real Kafka or Redpanda consumer.
- No outbox publisher or outbox polling worker.
- No Redis presence lookup or cross-instance Gateway routing.
- No remote calls to other Gateway processes.
- No delivery ACK persistence or mobile push worker.

## Adapter Contract

Package: `internal/transfer/gateway`

`Dispatcher` wraps an existing `internal/gateway/delivery.Dispatcher` and implements:

```go
Dispatch(ctx context.Context, envelope transfer.Envelope) transfer.DispatchResult
```

For `message.accepted`, it builds:

```go
delivery.NewMessageEvent(delivery.EventMessageReceived, delivery.Message{...})
```

The message payload is copied from the transfer event fields: `server_msg_id`, `client_msg_id`, `conversation_id`, `seq`, `sender_id`, direct receiver, `chat_type`, `content_type`, `content`, `message_origin`, `agent_account_id`, `trigger_server_msg_id`, `agent_run_id`, `allow_recursive_trigger`, `send_time`, `created_at`, and `trace_id`. `content_metadata` is shallow-cloned so Gateway callers cannot mutate the transfer event after dispatch.

Recipient resolution is intentionally narrow:

- `receiver_ids` is authoritative when present.
- `receiver_id` is a fallback for single-recipient events.
- Empty and duplicate recipient IDs are removed.
- Gateway is not asked to resolve conversation membership.

The adapter calls `DeliverToConversation(ctx, conversation_id, recipient_user_ids, event)` because that Gateway contract already accepts resolved recipient IDs and returns per-recipient status.

## Result Mapping

- Delivered recipients produce `transfer.StatusSucceeded` with `DeliveredUserIDs` populated.
- Offline recipients produce `transfer.StatusSucceeded` with no delivered user for that recipient. Offline is not retryable in this branch because durable message history remains available through pull-on-reconnect compensation.
- A `message.accepted` event with no direct recipients produces terminal `transfer.StatusFailed` and does not call Gateway.
- Gateway dispatcher errors produce `transfer.StatusRetryable`.
- Gateway `failed` recipient statuses produce `transfer.StatusRetryable`, even if `DeliverToConversation` returned no Go error.
- Gateway `routed` recipient statuses produce `transfer.StatusRetryable` for MVP because remote Gateway delivery transport is not implemented in this branch.
- Unsupported transfer event types produce terminal `transfer.StatusFailed`.

The adapter also copies Gateway per-recipient outcomes into `DispatchResult.RecipientResults` so the worker can persist delivery attempts as `delivered`, `offline`, or `failed` through [`message-delivery-reliability.md`](./message-delivery-reliability.md).

The worker remains responsible for idempotency. When the same accepted event is received again, `MemoryIdempotencyStore` or a future durable idempotency store skips a second Gateway call and still marks the envelope successful.

## Retry Semantics

The adapter returns retryable dispatch results for Gateway dispatcher errors and failed recipient statuses. `Worker.RunOnce` converts those results into `RetryDecision` data containing attempt, max attempts, backoff, next attempt time, and reason. Future outbox or Kafka retry implementations should persist or translate that `RetryDecision` outside the adapter.

## Verification

Required validation for this branch:

- `goctl --version`
- `for f in api/*.api; do goctl api validate -api "$f"; done`
- `gofmt -w $(find . -name "*.go" -print)`
- `go test ./...`
- `bash scripts/verify-static.sh`
- `docker compose config`

Targeted tests live in `internal/transfer/gateway/dispatcher_test.go` and cover successful delivery conversion, offline handling, no-recipient handling, worker idempotency for duplicate events, and retry classification with `RetryDecision`.
