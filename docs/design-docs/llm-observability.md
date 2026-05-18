# LLM Observability for AI Hosting

Status: Accepted / Issue #74 implementation boundary

## Background

AI hosting auto-replies now run through `internal/agentim.AgentRunOrchestrator` and the Eino DeepSeek runtime. Existing `internal/observability` remains system-level health, request tracing, and Prometheus metrics. LLM observability needs a separate boundary because prompt/model/tool traces can contain product-sensitive content and should be exportable to an LLM-focused UI such as Langfuse without scattering vendor calls through business logic.

## Goals

- Emit AI hosting run lifecycle metadata with `trace_id`, `request_id`, `agent_run_id`, conversation/message linkage, account actors, model, prompt version/hash, runtime mode, status, latency, and sanitized errors.
- Use Eino callbacks as the model/generation instrumentation entry point.
- Keep `agent_runs` and related audit tables authoritative for in-system audit state.
- Make Langfuse the primary intended backend, but keep live network export opt-in and explicit.
- Provide deterministic local tests and an LLM-as-judge harness that do not call a live LLM by default.

## Non-Goals

- No provider key, JWT, cookie, DSN, raw auth header, or unbounded conversation history is logged or exported.
- No live Langfuse SDK/API exporter is claimed in this boundary. Selecting Langfuse with complete config currently fails explicitly with `ErrLangfuseExportNotImplemented`.
- No database schema changes. `agent_runs` remains the durable application audit source.

## Design

`internal/llmobs` owns the LLM observability abstraction:

- `Sink` receives normalized `Event` values.
- `NoopSink` is the default and reports `Exported=false` with a disabled reason.
- `MemorySink` is the deterministic test sink.
- Langfuse config is validated through `LANGFUSE_HOST`, `LANGFUSE_PUBLIC_KEY`, and `LANGFUSE_SECRET_KEY`; missing or placeholder credentials return `ErrLangfuseConfigMissing`.
- Live Langfuse export returns `ErrLangfuseExportNotImplemented` until a real exporter is implemented, preventing fake successful remote export.

`internal/agentim` emits run-level events around `AgentRunOrchestrator.Run`:

```text
runtime request normalized
-> llmobs run started
-> runtime.Run
-> audit/writeback
-> llmobs run succeeded or failed
```

Success events include `response_server_msg_id` after Message Service writeback. Failure events include sanitized `error_class` / `error_message` and never include a response message id.

`internal/agentruntime/eino` registers an Eino callback handler when a sink is configured. The callback emits generation started/succeeded/failed metadata from the model component, including bounded context count, trigger-in-context, token usage, finish reason, latency, and a hash/size output summary. Raw final output capture is disabled by default and gated by `LLM_OBSERVABILITY_CAPTURE_OUTPUT`.

## Configuration

Default behavior is safe noop:

```text
LLM_OBSERVABILITY_ENABLED=false
LLM_OBSERVABILITY_BACKEND=noop
LLM_OBSERVABILITY_CAPTURE_OUTPUT=false
LLM_OBSERVABILITY_MAX_OUTPUT_BYTES=2048
```

Langfuse placeholders may be present in `.env.example`, but real credentials must come from local env/secrets only.

## LLM-as-Judge Eval

`internal/agenteval` contains the deterministic case:

```text
ai_hosting.python_go_performance.v1
```

The default rule judge fails vague replies such as `可以，你简单说说吧。` and passes answers that compare execution speed, compiled Go vs Python interpreter/VM behavior, concurrency, startup/deployment, memory/runtime overhead, ecosystem fit, and use-case-dependent choice. Result objects contain `case_id`, `score`, `pass`, `reason`, plus optional trace/run linkage.

## Security Notes

- Metrics labels must not include user IDs, message IDs, prompts, or message content.
- LLM observability events can include IDs for trace correlation in sinks but do not add those IDs to Prometheus labels.
- Prompt text and final output are not persisted by default; output capture is opt-in and bounded.
- Recent conversation context is bounded by the AI hosting runtime request builder; full history is not exported.

## Validation

Focused tests:

```bash
PATH=/home/ws/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.24.1.linux-amd64/bin:$HOME/go/bin:$PATH GOCACHE=/tmp/go-build-cache go test ./internal/llmobs ./internal/agenteval ./internal/agentim ./internal/config
```

Full branch verification should also run:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
for f in api/*.api; do goctl api validate -api "$f"; done
go test ./...
bash scripts/verify-static.sh
git diff --check
```
