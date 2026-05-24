# Distributed Tracing with OpenTelemetry and Grafana Tempo

OpenTelemetry trace IDs are the canonical trace IDs for backend requests. Operators can take `X-Trace-Id` from an HTTP response, WebSocket ACK, structured log line, or Admin Console LLM trace row and search the matching trace in Grafana's Tempo datasource.

This is separate from `/admin/llm-traces`. The admin endpoint remains the AI audit detail view for Agent runs, tool calls, file reads, and Python executions. Tempo/Grafana shows the distributed span graph across HTTP, WebSocket, RPC, message persistence, outbox transfer, gateway delivery, and Agent runtime boundaries.

## Architecture

```text
agents_im services
  -> OTLP gRPC
  -> otel-collector.agents-im.svc.cluster.local:4317
  -> tempo.agents-im.svc.cluster.local:4317
  -> Grafana Tempo datasource
```

go-zero v1.10.1 has native `ServiceConf.Telemetry` support, but `agents_im` intentionally uses the repo-level `Tracing` config plus `internal/observability.InitServiceTracing` because it already handles HTTP, WebSocket, RPC, Kafka/outbox, gateway delivery, Agent runtime boundaries, trace UI links, privacy filtering, and local fail-closed validation consistently. Do not add a parallel `Telemetry` block to only some services; all service coverage must stay in the unified `Tracing` config.

Tracing is disabled by default for local/unit-test friendliness. Enabling tracing requires a real OTLP endpoint; bad enabled configuration fails service startup.

## Runtime configuration

Important fields/env:

- `Tracing.Enabled` / `AGENTS_IM_TRACING_ENABLED`
- `Tracing.ServiceName` / `AGENTS_IM_TRACING_SERVICE_NAME` / `OTEL_SERVICE_NAME`
- `Tracing.Environment` / `AGENTS_IM_ENV` / `DEPLOYMENT_ENVIRONMENT`
- `Tracing.OTLPEndpoint` / `AGENTS_IM_OTLP_ENDPOINT`
- `Tracing.Protocol` / `AGENTS_IM_OTLP_PROTOCOL` (`grpc` or `http/protobuf`)
- `Tracing.SamplerRatio` / `AGENTS_IM_TRACING_SAMPLER_RATIO`
- `Tracing.TraceUIBaseURL` / `AGENTS_IM_TRACE_UI_BASE_URL`

Production k3s config exports OTLP to the internal collector:

```text
otel-collector.agents-im.svc.cluster.local:4317
```

The collector exports to Tempo at `tempo.agents-im.svc.cluster.local:4317`. Tempo stores traces on the `tempo-data` PVC with bounded retention configured in `deploy/k8s/tempo.yaml`. Do not replace this with memory-only trace storage.

## Query workflow

1. Get a trace ID from one of:
   - HTTP response header: `X-Trace-Id`
   - WebSocket ACK field/header propagated through the message path
   - Structured logs: `trace_id=...`
   - Admin Console: open an LLM trace detail and use the `Open in Tempo` link when present.
2. Query in Grafana:
   - Open `https://grafana.agenticim.xyz/`.
   - Use the `Tempo` datasource in Explore.
   - Search by trace ID or follow the generated Admin Console link.
3. Correlate logs using the same `trace_id` in Grafana Loki Explore.

## Privacy/security constraints

Trace/span/log attributes must not include:

- raw query strings
- Authorization/JWT/cookie headers
- passwords, DSNs, API keys, provider keys
- presigned object-storage URLs
- request/response bodies
- message text or prompt content

Grafana is the public observability UI and is protected by its own login (`grafana-admin` Secret). Tempo itself is internal-only and must not be exposed through public ingress.

## Operational checks

```bash
kubectl -n agents-im get deploy,svc,pvc tempo otel-collector
kubectl -n agents-im logs deploy/otel-collector --tail=100
kubectl -n agents-im logs deploy/tempo --tail=100
kubectl -n agents-im port-forward svc/tempo 3200:3200
```

Expected steady state:

- `otel-collector` is ready and listens on OTLP gRPC `4317` and HTTP `4318`.
- `tempo` is ready, has a bound `tempo-data` PVC, and is reachable from Grafana at `http://tempo.agents-im.svc.cluster.local:3200`.
- `grafana-provisioning` includes the `Tempo` datasource.
