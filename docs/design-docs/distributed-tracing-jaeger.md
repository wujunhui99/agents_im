# Distributed Tracing with OpenTelemetry and Jaeger

Status: Implemented for Issue #143

## Purpose

OpenTelemetry trace IDs are now the canonical trace IDs for new backend requests. Operators can take `X-Trace-Id` from an HTTP response, WebSocket ACK, structured log line, or Admin Console LLM trace row and search the matching trace in Jaeger.

This is separate from `/admin/llm-traces`. The admin endpoint remains the AI audit detail view for Agent runs, tool calls, file reads, and Python executions. Jaeger shows the distributed span graph across HTTP, WebSocket, RPC, message persistence, outbox transfer, gateway delivery, and Agent runtime boundaries.

## Configuration

go-zero v1.10.1 has native `ServiceConf.Telemetry` support, but `agents_im` intentionally uses the repo-level `Tracing` config plus `internal/observability.InitServiceTracing` because it already handles HTTP, WebSocket, RPC, Kafka/outbox, gateway delivery, Agent runtime boundaries, Jaeger deep links, privacy filtering, and local fail-closed validation consistently. Do not add a parallel `Telemetry` block to only some services; all service coverage must stay in the unified `Tracing` config.

Tracing is disabled by default for local/unit-test friendliness. Enabling tracing requires a real OTLP endpoint; bad enabled configuration fails service startup.

Supported config/env fields:

- `Tracing.Enabled` / `AGENTS_IM_TRACING_ENABLED`
- `Tracing.ServiceName` / `AGENTS_IM_TRACING_SERVICE_NAME` / `OTEL_SERVICE_NAME`
- `Tracing.Environment` / `AGENTS_IM_ENV`
- `Tracing.OTLPEndpoint` / `AGENTS_IM_OTLP_ENDPOINT`
- `Tracing.Protocol` / `AGENTS_IM_OTLP_PROTOCOL` (`grpc` or `http/protobuf`)
- `Tracing.SamplerRatio` / `AGENTS_IM_TRACING_SAMPLER_RATIO`
- `Tracing.JaegerBaseURL` / `AGENTS_IM_JAEGER_BASE_URL`

Production k3s config exports OTLP to the internal service:

```text
jaeger-collector.agents-im.svc.cluster.local:4317
```

No secrets are required for the current internal Jaeger all-in-one deployment.

## Propagation

Inbound HTTP accepts W3C `traceparent` / `tracestate` plus compatibility `X-Trace-Id` / `X-Request-Id`. Responses keep:

- `X-Trace-Id`
- `X-Request-Id`
- `traceparent` when a valid W3C context exists
- `tracestate` when present

Async message boundaries carry trace metadata through message outbox payloads, Kafka JSON payloads, Kafka headers, transfer worker envelopes, gateway delivery HTTP headers, and WebSocket delivery events.

## Privacy Rules

Tracing code records safe route templates and operational metadata only. It must not put these values into logs or span attributes:

- raw query strings
- `Authorization`
- JWTs or cookies
- passwords, DSNs, API keys, presigned URLs
- request or response bodies
- message content

Health, readiness, and metrics routes are intentionally suppressed as span/log noise.

## Runbook

1. Get a trace ID:
   - HTTP: read `X-Trace-Id` from the response headers.
   - WebSocket: read `trace_id` from ACK/error frames.
   - Logs: search structured log fields named `trace_id`.
   - Admin Console: open an LLM trace detail and use the `Open in Jaeger` link when present.
2. Query Jaeger:
   - Authenticated public access: `https://jaeger.agenticim.xyz/search?traceID=<trace_id>`
   - Private access: `kubectl -n agents-im port-forward svc/jaeger-collector 16686:16686`
   - Private browser URL: `http://127.0.0.1:16686/search?traceID=<trace_id>`
3. Expected span path for a WebSocket send:
   - `websocket.handshake`
   - `websocket.command.send_message`
   - `message.send`
   - `message.transfer.process`
   - `message.transfer.gateway_dispatch`
   - `gateway.delivery.conversation`

## Security Decision

`jaeger.agenticim.xyz` is exposed only through the Traefik `observability-basic-auth` middleware. The basic-auth Secret is created outside tracked source, and trace data can reveal topology, route names, internal IDs, latency, and error details. Do not remove the middleware or publish Jaeger as an unauthenticated route.

Acceptable Jaeger access models are:

- authenticated reverse proxy middleware,
- VPN/private network only, or
- IP allowlist with TLS and operational owner approval.

Operators may still use `kubectl port-forward` or another private access path when debugging from the server/network.
