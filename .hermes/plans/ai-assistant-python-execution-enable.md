# AI Assistant Python Execution Enablement Plan

> **For Hermes:** Use Codex/subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Make the built-in default AI assistant (`agent_creator` / AI助手) able to use `python.execute` through the safe tool registry + sandbox executor path.

**Architecture:** Seed `python.execute` as an admin local tool, bind it idempotently only to the default assistant agent, and wire the DeepSeek/Eino runtime to execute approved tool calls through the existing `agentruntime/tools` adapters. The sandbox remains opt-in via `PythonExecutor.Backend: k8s`; disabled executor returns a visible error.

**Tech Stack:** Go, go-zero service contexts, existing Agent registry repository, CloudWeGo Eino adapter boundary, Python executor package, PostgreSQL migrations/seed helpers.

---

## Current gaps

1. `python.execute` exists as a safe local adapter foundation, but AI助手 is not guaranteed to have a registry tool + binding.
2. The DeepSeek runtime currently calls `cm.Generate(...)` once and reads `resp.Content`; it does not execute model tool calls.
3. `agent-api` owns PythonExecutor config, but actual direct AI assistant replies are likely built under message service / agentim orchestration. The runtime builder/service context must pass a tool provider with `NewDefaultLocalAdapterCatalog(pythonExecutor)`.
4. Production must remain disabled until k8s sandbox infra is explicitly configured and smoke-tested.

## Task 1: Seed python.execute tool and bind it only to agent_creator

**Objective:** Ensure default AI助手 has `python.execute` binding idempotently, without granting all agents.

**Files to inspect/modify:**
- `internal/servicecontext/user/service_context.go`
- default assistant creation/binding code found near `ConfigureDefaultAssistant`
- `internal/repository/*agent_registry*`
- tests for default assistant provisioning
- if schema/data seed is needed: `db/migrations/*.sql` and Markdown audit note under `db/change_log/`

**TDD:**
- Add test: new user provisioning creates/binds default assistant and `python.execute` tool binding exists.
- Add test: a regular newly-created non-default agent does not get `python.execute` implicitly.
- Verify RED first.

**Implementation notes:**
- Use stable tool name/key: `python.execute`.
- Tool fields:
  - `tool_type='local'`
  - `local_handler_key='python.execute'`
  - `permission_level='restricted'`
  - admin_configured true
  - input schema matches existing adapter: `code`, optional `timeout_seconds`, optional `files`.
- Binding should be idempotent and safe on existing users/default assistant.

## Task 2: Wire PythonExecutor into the runtime path used by direct AI助手 replies

**Objective:** The runtime invoked by `agent_creator` direct conversations must have a tool provider/catalog backed by the configured PythonExecutor.

**Files to inspect/modify:**
- `internal/servicecontext/message/*`
- `internal/agentim/*orchestrator*`
- `internal/agentruntime/eino/deepseek_runtime.go`
- `cmd/message-api/main.go` or whichever service initializes direct assistant runtime
- `internal/agentruntime/tools/catalog.go`

**TDD:**
- Add service/orchestrator test proving the runtime/tool provider receives a PythonExecuteAdapter when `python.execute` is bound and executor is injected.
- Disabled/default executor should remain fail-closed.

## Task 3: Add tool-call loop to DeepSeek/Eino runtime

**Objective:** If model returns a `python.execute` tool call, invoke approved adapter, append tool result, and continue generation to final answer.

**Files:**
- `internal/agentruntime/eino/deepseek_runtime.go`
- `internal/agentruntime/eino/deepseek_runtime_test.go`
- maybe `internal/agentruntime/llm/deepseek/*` if model abstraction needs a fakeable interface

**TDD:**
- Use fake tool-calling model + fake executor. No real DeepSeek/network.
- Test: first model response requests `python.execute` with `print(1+1)`, adapter returns stdout `2`, second model response returns final text mentioning `2`.
- Test: disabled executor/tool error returns visible runtime error or a final tool error according to current runtime contract; do not fake success.
- Test: max tool calls policy is enforced.

**Implementation notes:**
- Respect `RuntimePolicy.MaxToolCalls`; default finite limit.
- Tool resolution must use registry-approved `ToolRef`s and `RequireAdapters=true`.
- Record `RunResult.ToolCalls` with status/duration/error metadata.
- Do not introduce shell/os exec.

## Task 4: Production enablement manifests/runbook, still explicit opt-in

**Objective:** Make it clear how to enable for AI助手 in production without accidentally enabling unsafe execution.

**Files:**
- `deploy/README.md`
- `deploy/k8s/*`
- `docs/design-docs/python-executor-sandbox.md`

**Acceptance:**
- Docs list exact required steps: build sandbox image, namespace, RBAC, NetworkPolicy deny-all, agent-api/message-api config, smoke test.
- Config examples keep `Backend: disabled` unless clearly marked opt-in.

## Final verification

Run:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
gofmt -w <changed-go-files>
go test ./internal/agentim ./internal/agentruntime/... ./internal/servicecontext/... ./internal/config
go test ./...
bash scripts/verify-static.sh
git diff --check
```

If DB/repository SQL changed and Docker/Postgres is available:

```bash
AGENTS_IM_CONFIRM_TRUNCATE=1 scripts/verify-postgres-local.sh
```

## Controller notes

- Do not merge unless runtime actually invokes tool adapters in a fake test.
- Do not claim production is enabled until live k8s sandbox image/namespace/RBAC/network policy and smoke test pass.
- If production is configured later, verify by chatting with AI助手 and asking a calculation that requires Python; inspect tool/run audit and final reply.

## Codex execution notes

- RED verified with focused tests: missing default assistant binding, missing message runtime tool-provider helper, and missing Eino runtime tool options/loop.
- Implemented idempotent `python.execute` seed/bind for `agent_creator` only, plus PostgreSQL migration/change-log SQL.
- Wired `message-api` direct assistant runtime to a registry-backed tool provider and configured PythonExecutor; defaults remain disabled.
- Added fake model/fake executor runtime tests for successful Python tool call continuation, disabled executor failure, and max tool-call enforcement.
