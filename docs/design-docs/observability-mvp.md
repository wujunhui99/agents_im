# Observability MVP

Status: Implemented

## Background

Backend MVP needs a small, local-first observability foundation before frontend integration starts. The goal is to make service liveness, readiness, metrics, and trace correlation available without requiring Prometheus, Grafana, Jaeger, Redis, Kafka, or PostgreSQL to be live during unit tests.

## Goals

- Expose unauthenticated `GET /healthz` for process liveness.
- Expose unauthenticated `GET /readyz` for safe configuration/dependency readiness checks.
- Expose Prometheus text-format `GET /metrics` where an HTTP surface exists.
- Track basic MVP counters and gauges for message sends, gateway delivery attempts, transfer worker processing, and WebSocket connections.
- Propagate `trace_id` / `request_id` through HTTP headers and WebSocket ACK envelopes.
- Keep logs useful for correlation while never logging passwords, JWTs, bearer tokens, request bodies, or query strings.

## Non-Goals

- No production tracing backend is added.
- No Prometheus, Grafana, or Jaeger container is required.
- No core friends, groups, or message delivery semantics are changed.
- Readiness checks do not perform live network pings in unit tests.

## Design

### Shared Packages

- `internal/health` owns liveness/readiness report shapes and HTTP handlers.
- `internal/observability` owns trace context propagation, HTTP trace middleware, and a dependency-free Prometheus text metrics registry.

The health package returns JSON reports:

```json
{
  "status": "ready",
  "service": "message-api",
  "timestamp": "2026-04-29T00:00:00Z",
  "checks": [
    {"name": "message_logic", "status": "ready", "message": "configured"}
  ]
}
```

`/readyz` returns HTTP 200 when all checks are ready and HTTP 503 when any check is not ready. Checks report only safe component state such as `configured` or `missing`; they do not expose DSNs, credentials, JWT secrets, or passwords.

### HTTP Surfaces

go-zero REST services expose:

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

Gateway exposes the same endpoints on the Gateway HTTP server. Message Transfer now exposes a lightweight observability HTTP listener when `Observability.Enabled` is true in `etc/message-transfer.yaml`; it serves the same three endpoints on `Observability.Host:Observability.Port`.

### Metrics

Metrics are emitted in Prometheus text format. Current names:

- `agents_im_message_sends_total{status,chat_type}`
- `agents_im_delivery_attempts_total{status}`
- `agents_im_transfer_events_total{result}`
- `agents_im_websocket_connections`
- `agents_im_websocket_connection_events_total{event}`
- `agents_im_http_requests_total{method,path,status}`

Metric labels intentionally avoid user IDs, message IDs, connection IDs, JWTs, request bodies, and message content. HTTP request metrics use coarse route families such as `users`, `friends`, `groups`, `messages`, `conversations`, `ws`, `healthz`, `readyz`, and `metrics` instead of raw paths.

### Trace IDs

HTTP middleware accepts `X-Trace-Id`, `X-Request-Id`, or W3C `traceparent`. If none is present, it generates a local trace ID and writes both response headers:

- `X-Trace-Id`
- `X-Request-Id`

WebSocket handshake reuses the HTTP trace context. Command ACK and error frames include `trace_id`, and command logs include `trace_id`, `request_id`, `connection_id`, `user_id`, command type, status, and error code only. Payloads, auth headers, tokens, and query strings are not logged.

## Security Notes

- Logging code records `URL.Path`, not `RequestURI`, so `/ws?token=[REDACTED]` query tokens are not written by the new trace middleware.
- New observability logs do not dump headers or bodies.
- Readiness reports use safe component names and generic messages.

## Risks

- The in-process metrics registry is intentionally minimal. It is suitable for MVP counters/gauges but not a replacement for a production metrics library.
- Readiness is shallow by design. A service can be ready by configuration while a downstream dependency later fails at request time.
- Message Transfer observability HTTP startup is non-fatal; if its port is unavailable, the worker continues and logs the observability listener error.

## Validation

Required branch verification:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name '*.go' -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
```

Because this document changes docs, run markdown link check excluding `docs/references`.

Latest validation on 2026-04-29:

- `goctl --version`: `goctl version 1.10.1 linux/amd64`
- `for f in api/*.api; do goctl api validate -api "$f"; done`: five API specs returned `api format ok`
- `gofmt -w $(find . -name '*.go' -print)`: completed with no output
- `go test ./...`: passed
- `bash scripts/verify-static.sh`: `static verification passed`
- `docker compose config`: passed and rendered config successfully
- markdown link check excluding `.ai-context/` and `docs/references/`: passed
