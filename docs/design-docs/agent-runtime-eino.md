# Agent Runtime Eino Boundary

Status: Accepted

## Background

The Agent System V0 branches are adding Agent profile, prompt, tool, skill, audit, Python executor, and Agent-IM contracts in parallel. The runtime branch needs a stable Go boundary that later Eino and DeepSeek adapters can implement without leaking Eino concrete types into business logic.

The core runtime package is a pure contract. It does not import CloudWeGo Eino packages, does not call an LLM provider, does not execute tools or Python, and does not write IM messages.

## Goals

- Define `internal/agentruntime.Runtime` as the local Agent runtime interface.
- Carry the fields needed from `internal/agentim.AgentTrigger` into `agentruntime.RunRequest`.
- Carry registry-derived Agent config, prompt snapshot, tool refs, skill refs, model config, and runtime policy as plain domain structs.
- Normalize and validate requests/results fail-first with `apperror` errors.
- Keep default tests local, deterministic, and independent of `DEEPSEEK_API_KEY` or network.

## Non-Goals

- No Eino adapter implementation in this branch.
- No real DeepSeek/OpenAI-compatible client.
- No tool execution, Python execution, shell execution, or OS process startup.
- No Agent response write-back inside `internal/agentruntime` and no direct message repository/table writes.
- No audit repository implementation changes.

## Package Boundary

The stable runtime package is:

```text
internal/agentruntime
```

It exposes:

```go
type Runtime interface {
    Run(ctx context.Context, req RunRequest) (RunResult, error)
}
```

Future framework adapters should live behind this interface, for example:

```text
internal/agentruntime/eino
internal/agentruntime/llm/deepseek
```

Those adapter packages may import Eino later. Callers in Agent-IM workers, webhook consumers, or orchestration logic should depend on `agentruntime.Runtime`, not on Eino or provider-specific types.

## Run Request

`RunRequest` includes trigger fields compatible with the first Agent-IM contract:

- request, event, operation, and trace identifiers;
- trigger type: `user_private_message`, `group_mention`, or `admin_manual_run`;
- target `agent_user_id`, requesting user, conversation id/type, trigger message id/seq, prompt text, and reply target;
- recursion/source fields for Agent-originated messages;
- target Agent user ids from the original event;
- optional conversation context messages.

It also includes registry/runtime assembly data:

- `AgentConfig.AgentID` and `AgentConfig.AgentUserID`;
- `PromptRef` with prompt id, version metadata, and content snapshot;
- `ModelConfig` with provider/model metadata and credential reference, not raw API keys;
- `ToolRef` values for already-authorized registry tool bindings;
- `SkillRef` values for already-authorized skill bindings;
- runtime policy limits such as max tool calls, max duration, recursion depth, and Message Service write-back requirement.

## Validation Rules

`NormalizeRunRequest` returns a normalized copy or an `apperror.InvalidArgument` failure. Required fields include:

- `request_id`;
- supported `trigger_type`;
- `agent_user_id`;
- `requesting_user_id`;
- `conversation_id`;
- supported `conversation_type`;
- non-empty trigger `prompt_text`;
- `agent.agent_id`;
- `agent.agent_user_id`, which must match the trigger `agent_user_id`;
- `agent.prompt.prompt_id`;
- non-empty `agent.prompt.content`;
- `agent.model.provider`;
- `agent.model.model`.

Tool refs are shape-validated as metadata only. Runtime accepts only `mcp`, `local`, and `builtin` tool types and rejects invalid mixed metadata such as MCP fields on local tools. This does not authorize tool execution; tool authorization remains owned by registry/orchestration logic.

`NormalizeRunResult` requires:

- non-empty `run_id`;
- non-empty `final_text`;
- non-negative usage counters;
- non-negative tool-call durations.

## Runtime Flow

The intended V0 flow is:

```text
Agent-IM trigger
-> orchestration loads Agent registry config and conversation context
-> orchestration builds and validates agentruntime.RunRequest
-> Runtime.Run returns RunResult or error
-> orchestration records audit
-> Agent response writer sends final text through Message Service
```

The Agent-IM runner seam is `internal/agentim.AgentRunOrchestrator`. It depends on `agentruntime.Runtime` plus an explicit `RuntimeRequestBuilder`; it must not introduce a separate Eino/provider-specific runtime interface. The builder owns loading registry-derived Agent config and conversation context. Missing config, mismatched trigger fields, runtime failures, empty final text, and audit/write-back failures are visible errors.

The runtime does not own IM persistence. A successful `RunResult` is not an IM write-back success. End-to-end success requires the orchestration layer to write through `agentim.ResponseWriter` / Message Service and validate that Message Service returns a real message id, conversation id, and seq.

## Eino Adapter Rules

When the Eino adapter is implemented later:

- Eino imports stay inside adapter packages.
- Missing provider config, missing credentials, model errors, tool errors, and audit errors must return visible errors.
- Live provider tests must be opt-in and skipped unless explicit environment flags and keys are set.
- Default `go test ./...` must not require `DEEPSEEK_API_KEY` or network.
- Tool execution must use registry-approved refs and audit wrappers.
- Production Go code must not directly call shell, `os/exec`, or unsandboxed Python.

## Verification

This branch validates the contract with:

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./internal/agentruntime
```

Full branch verification remains:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -path './web/node_modules' -prune -o -name '*.go' -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
git diff --check
```
