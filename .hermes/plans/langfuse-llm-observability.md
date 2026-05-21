# Langfuse LLM Observability Implementation Plan

> **For Hermes:** Use Codex as implementation worker, then controller-review diff/tests before PR.

**Goal:** Add Langfuse export for AI/LLM assistant observability at `https://langfuse.agenticim.xyz` without replacing or deleting existing local trace/request-id/admin LLM trace behavior.

**Architecture:** Keep existing `internal/llmobs` abstraction. Replace the current explicit `ErrLangfuseExportNotImplemented` path with a real Langfuse ingestion sink using Langfuse public ingestion HTTP API. Use config/env for host and keys, with no committed secrets. Missing credentials must be visible and must not fake successful export.

**Tech Stack:** Go 1.24, existing go-zero services, existing `internal/llmobs`, HTTP client with stdlib, JSON, existing tests.

---

## Requirements

- Additive only: do not delete existing trace middleware, trace IDs, audit repository, or admin LLM trace APIs.
- Default Langfuse host: `https://langfuse.agenticim.xyz`.
- Required secret env names: `LANGFUSE_PUBLIC_KEY`, `LANGFUSE_SECRET_KEY`.
- Config may enable via `LLMObservability.Enabled=true` and `LLMObservability.Backend=langfuse` or env equivalents.
- Missing credentials/config must be explicit; no fake success.
- Sanitization must preserve existing behavior: no JWTs, bearer tokens, cookies, passwords, DSNs, API keys, or raw secret values in exported metadata.
- Existing AI flows should continue if Langfuse is disabled. If enabled but credentials are missing, service configuration should fail visibly as it currently does for required AI config failures.

## Implementation Tasks

### Task 1: Update RED tests for real Langfuse sink

**Files:**
- Modify: `internal/llmobs/sink_test.go`

**Steps:**
1. Replace `TestLangfuseLiveExportIsExplicitlyNotImplemented` with tests using `httptest.Server`.
2. Verify `NewSink` returns a Langfuse sink when host/public/secret are provided.
3. Verify `Observe` sends a POST to `/api/public/ingestion` with HTTP Basic auth (`publicKey:secretKey`) and returns `Exported=true` on 2xx.
4. Verify non-2xx returns an explicit error and `Exported=false`.
5. Verify event payload redacts sensitive metadata and does not include raw final output unless `CaptureOutput=true`.

Run first and expect failure:

```bash
go test ./internal/llmobs -run 'TestLangfuse' -count=1
```

### Task 2: Implement Langfuse HTTP sink

**Files:**
- Modify: `internal/llmobs/sink.go`
- Create if useful: `internal/llmobs/langfuse.go`

**Steps:**
1. Add an HTTP client to `LangfuseSink` with timeout.
2. Normalize host and ingestion endpoint `/api/public/ingestion`.
3. Convert `llmobs.Event` into Langfuse ingestion batch JSON. Minimal acceptable shape: a trace event plus generation/event observation, using deterministic IDs from run/trace/request fields where possible.
4. Use Basic auth with public/secret key; never log or return secret values.
5. Return explicit errors on request build, network, JSON, or non-2xx responses.
6. Preserve Noop and Memory behavior.

Run:

```bash
go test ./internal/llmobs -count=1
```

### Task 3: Config defaults and examples

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `etc/message-api.yaml`, `etc/agent-api.yaml`
- Modify: `deploy/k8s/etc/message-api.yaml`, `deploy/k8s/etc/agent-api.yaml`

**Steps:**
1. Set default Langfuse host to `https://langfuse.agenticim.xyz` in config resolution.
2. Keep default enabled=false/backend=noop unless explicitly enabled.
3. Add tests that default host resolves and env/config can override.
4. Add config stanzas with env placeholders, no real keys.

Run:

```bash
go test ./internal/config -run 'LLMObservability|Langfuse' -count=1
```

### Task 4: Deployment docs and secret names

**Files:**
- Modify: `deploy/README.md`
- Modify if present/relevant: docs design observability doc

**Steps:**
1. Document Langfuse host and the secret names only.
2. State values are stored in Drone/k3s/server secrets only, not Git/chat.
3. State this is additive and existing admin LLM traces remain.

### Task 5: Full verification and commit

Run:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
gofmt -w $(find . -name '*.go' -print)
go test ./internal/llmobs ./internal/config -count=1
go test ./...
bash scripts/verify-static.sh
git diff --check
```

Commit with:

```text
feat(observability)[achilles]: add langfuse llm export

Issue: #142
Agent: achilles
Human-Owner: junhui
Reason: add Langfuse as additive AI observability backend
```

Do not push unless controller asks or performs review.
