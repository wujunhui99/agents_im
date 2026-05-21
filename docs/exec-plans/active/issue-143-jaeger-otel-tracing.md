# Issue 143 Jaeger OpenTelemetry Tracing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add production-grade OpenTelemetry distributed tracing with Jaeger-compatible OTLP export while preserving local developer defaults.

**Architecture:** `internal/observability` owns tracing config, provider startup/shutdown, ID propagation, sanitization, and helper span APIs. REST, WebSocket, message write/outbox/transfer/gateway delivery, and Agent runtime boundaries create spans and pass W3C context through existing headers or event metadata without recording secrets, raw query strings, or bodies. Deployment adds an internal Jaeger collector/query service and documents the security decision not to expose `jaeger.agenticim.xyz` without an auth boundary.

**Tech Stack:** Go, go-zero REST/RPC, OpenTelemetry Go SDK/OTLP exporters, Kafka headers, Kubernetes kustomize manifests, React admin console.

---

### Task 1: Tracing Core and Tests

**Files:**
- Modify: `internal/observability/trace.go`
- Create: `internal/observability/tracing.go`
- Test: `internal/observability/trace_test.go`

- [ ] Add tests for W3C `traceparent` extraction, generated OTel trace IDs, disabled tracing config validation, route template sanitization, and Jaeger URL construction.
- [ ] Implement disabled-by-default tracing config with service name, environment, OTLP endpoint, protocol, sampler ratio, and shutdown.
- [ ] Ensure bad enabled config fails visibly.
- [ ] Keep `X-Trace-Id` and `X-Request-Id` compatibility headers.

### Task 2: HTTP, WebSocket, and RPC Entry Points

**Files:**
- Modify: `cmd/*-api/main.go`
- Modify: `cmd/gateway-ws/main.go`
- Modify: `cmd/message-transfer/main.go`
- Modify: `internal/rpcgen/*/entry/entry.go`
- Modify: `internal/gateway/ws/server.go`

- [ ] Initialize tracing per service before starting servers and defer shutdown.
- [ ] Start HTTP server spans for REST/admin/gateway surfaces, suppressing span noise for `/healthz`, `/readyz`, and `/metrics`.
- [ ] Start WebSocket handshake and command spans with safe attributes only.
- [ ] Add gRPC server/client propagation where practical without changing RPC contracts.

### Task 3: Async Message Propagation

**Files:**
- Modify: `internal/repository/message_repository.go`
- Modify: `internal/repository/message_outbox_repository.go`
- Modify: `internal/logic/messagelogic.go`
- Modify: `internal/outboxpublisher/publisher.go`
- Modify: `internal/messaging/event.go`
- Modify: `internal/messaging/kafka.go`
- Modify: `internal/transfer/*.go`
- Test: `internal/transfer/outbox_consumer_test.go`
- Test: `internal/messaging/producer_test.go`

- [ ] Add trace context metadata to message create input and outbox payload.
- [ ] Inject W3C context into Kafka headers and JSON event metadata.
- [ ] Extract context in outbox/Kafka transfer consumers.
- [ ] Add spans around message persistence, outbox publish, worker consume, dispatch, and gateway HTTP delivery.

### Task 4: Agent Runtime Spans

**Files:**
- Modify: `internal/agentim/runner.go`
- Modify: `internal/agentruntime/eino/deepseek_runtime.go`
- Test: existing `internal/agentim/*` and `internal/agentruntime/eino/*` tests

- [ ] Start spans around agent orchestration, runtime request build, LLM generation, tool calls, and response write-back.
- [ ] Align new Agent audit `trace_id` values with OTel trace IDs while keeping existing `run_id` semantics.

### Task 5: Admin Link, Deployment, Docs, Verification

**Files:**
- Modify: `internal/logic/adminlogic.go`
- Modify: `internal/handler/admin/convert.go`
- Modify: `internal/types/types.go`
- Modify: `web/src/pages/AdminConsole.tsx`
- Modify: `web/src/pages/AdminConsole.test.tsx`
- Modify: `deploy/k8s/*.yaml`
- Create: `docs/design-docs/distributed-tracing-jaeger.md`
- Modify: `ARCHITECTURE.md`
- Modify: `deploy/README.md`
- Modify: `docs/SECURITY.md`
- Modify: `docs/RELIABILITY.md`

- [ ] Add a Jaeger search link field for LLM trace/admin details without replacing `/admin/llm-traces`.
- [ ] Add internal Jaeger manifests and OTLP config placeholders.
- [ ] Document that public `jaeger.agenticim.xyz` exposure is blocked until authentication or network restriction is installed.
- [ ] Run required verification and only then commit, push, open PR, and comment on Issue #143.
