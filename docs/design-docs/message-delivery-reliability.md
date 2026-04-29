# Message Delivery Reliability

Status: Implemented for MVP  
Owner: Message Transfer / Message Storage  
Related docs:
- [`backend-mvp-contract.md`](./backend-mvp-contract.md)
- [`message-transfer-worker.md`](./message-transfer-worker.md)
- [`transfer-gateway-dispatcher.md`](./transfer-gateway-dispatcher.md)
- [`message-outbox.md`](./message-outbox.md)
- [`gateway-push-delivery.md`](./gateway-push-delivery.md)

## Background

Message Service already durably accepts messages and writes a transactional outbox row. The outbox publisher can publish `message.accepted` events, and Message Transfer can dispatch those events to Gateway push delivery. Before this branch, delivery outcomes were visible only as in-memory dispatcher results and were not modeled as durable state.

MVP frontend behavior still treats send ACK as accepted/persisted only. Delivery state is for backend reliability, retry decisions, debugging, and future operator views. It is not a read receipt.

## Goals

- Model delivery attempts per `server_msg_id + recipient_user_id`.
- Support both in-memory and PostgreSQL repositories.
- Record `accepted`, `published`, `delivered`, `offline`, and `failed` status transitions.
- Keep `delivered` strictly separate from `has_read_seq`.
- Preserve ordinary `go test ./...` without live PostgreSQL, Redis, Kafka, or Gateway middleware.

## Non-goals

- No reconnect sync command changes.
- No friends/groups policy changes.
- No native mobile push/APNs/FCM.
- No frontend handoff document beyond delivery-specific examples.
- No cross-Gateway remote delivery transport.

## Data Model

Table: `delivery_attempts`

Required fields:

```text
server_msg_id
conversation_id
recipient_user_id
status
attempt_count
last_error
next_retry_at
created_at
updated_at
```

Primary key:

```text
(server_msg_id, recipient_user_id)
```

PostgreSQL stores `server_msg_id` as a foreign key to `messages(server_msg_id)`. In-memory storage uses the same repository contract without external dependencies.

## Status Semantics

- `accepted`: Message Service persisted the message and created recipient delivery attempt rows in the same write path.
- `published`: Outbox publisher successfully published the message event and marked the outbox row published.
- `delivered`: Gateway pushed the message to at least one local online WebSocket connection for that recipient.
- `offline`: Gateway found no live route or connection for that recipient.
- `failed`: Delivery failed. A non-null `next_retry_at` means retry is scheduled; a null `next_retry_at` means terminal for the current MVP policy.

`delivered` does not mean read. Read state remains `user_id + conversation_id -> has_read_seq`.

## Transition Rules

1. `CreateMessageIdempotent` inserts one accepted delivery attempt for each recipient, excluding the sender.
2. `MarkPublished` updates attempts for the message from `accepted` to `published`. It does not downgrade `delivered`, `offline`, or `failed`.
3. Transfer dispatch records per-recipient Gateway results:
   - local online write success -> `delivered`;
   - no presence route/local connection -> `offline`;
   - dispatcher error, write failure, or routed remote-only result without remote transport -> `failed`.
4. Retryable failures set `next_retry_at = now + retry_after`.
5. Max-attempt failures set `failed` with empty `next_retry_at`.

## Worker Integration

`transfer.DispatchResult` now carries per-recipient delivery results from the Gateway adapter. `Worker.RunOnce` records those results after the final retry decision is known:

- success records `delivered`/`offline` before marking the idempotency key processed;
- retryable failure records `failed` with `next_retry_at` before `MarkRetry`;
- terminal failure records `failed` with no retry time before `MarkFailed`.

If recording delivery attempts fails, the worker treats that as retryable so it does not commit or mark the envelope successful while delivery state is unknown.

## Retry Policy

MVP retry policy is intentionally minimal:

- Offline is not retried because message history is authoritative and reconnect sync can pull missed messages.
- Gateway dispatcher errors and failed recipient statuses are retryable until `Worker.MaxAttempts`.
- `attempt_count` tracks dispatch attempts and is monotonic.
- `last_error` is trimmed to a bounded backend diagnostic string.

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
npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json $(find . -name "*.md" -not -path "./.git/*" -not -path "./.ai-context/*" -not -path "./docs/references/*" -print)
```

Targeted tests cover accepted/published attempt transitions, delivered/offline/failed Transfer recordings, retry scheduling, and terminal max-attempt failure behavior.
