# Python Execution Tool Foundation Implementation Plan

> **For Hermes:** Use Codex/subagent-driven-development to implement this plan task-by-task.

**Goal:** Add a safe foundation for agent Python execution via the local `python.execute` tool while keeping real execution disabled until a sandbox executor service exists.

**Architecture:** V1 exposes `python.execute` as a local runtime tool handler, not as MCP. A `PythonExecuteAdapter` validates tool input, builds a restrictive `pythonexec.Policy`, invokes an injected `pythonexec.Executor`, and returns structured stdout/stderr/result output. The default executor remains `DisabledExecutor`, preserving fail-closed behavior and avoiding in-process Python, shell, Docker, or network execution.

**Tech Stack:** Go, existing `internal/agentruntime/tools` contracts, existing `internal/agent/pythonexec` executor contract, PostgreSQL metadata via existing agent registry models, Markdown docs.

---

## Task 1: Allow `python.execute` as a local tool handler

**Objective:** Extend the model/resolver whitelist so the registry can safely represent the Python execution tool without opening arbitrary command execution.

**Files:**
- Modify: `internal/model/agent_registry.go` or the file containing `LocalToolHandler*` constants.
- Modify: `internal/agentruntime/tools/resolver.go`
- Test: `internal/agentruntime/tools/resolver_test.go`

**Steps:**
1. Write a failing resolver test that seeds a local tool with handler key `python.execute` and resolves metadata successfully.
2. Run:
   ```bash
   PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./internal/agentruntime/tools -run TestResolverAllowsPythonExecuteLocalTool -count=1
   ```
   Expected: FAIL because handler key is not whitelisted / constant missing.
3. Add `LocalToolHandlerPythonExecute = "python.execute"` in model constants.
4. Add it to resolver local handler whitelist.
5. Re-run the specific test and package tests.

## Task 2: Implement `PythonExecuteAdapter`

**Objective:** Add an explicit safe adapter for `python.execute` that converts tool calls into `pythonexec.Request` and calls an injected executor.

**Files:**
- Create: `internal/agentruntime/tools/python_execute_adapter.go`
- Create/modify: `internal/agentruntime/tools/python_execute_adapter_test.go`

**Behavior:**
- Adapter only matches local tool specs with handler key `python.execute`.
- Input JSON schema:
  ```json
  {
    "code": "print(1 + 1)",
    "timeout_seconds": 10,
    "files": []
  }
  ```
- `code` required and non-empty.
- `timeout_seconds` optional, default 10, max 30.
- `files` optional; V1 only allows empty array unless caller supplies explicit allowlist mapping in adapter config. Do not read host paths.
- Policy defaults:
  - network disabled
  - timeout from input/default
  - CPU limit <= timeout
  - memory 256 MiB default
  - max output 64 KiB default
  - explicit file allowlist, empty by default
- Return `ToolResult` with JSON containing stdout, stderr, result_json, exit_code, timed_out, output_truncated, error.
- If executor is disabled or errors, return a visible error; do not return fake success.

**TDD:**
1. Test fake executor success: adapter passes code/policy and returns output JSON.
2. Test missing code fails before executor call.
3. Test disabled executor returns error containing `python executor is disabled`.
4. Run:
   ```bash
   PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./internal/agentruntime/tools -run 'TestPythonExecuteAdapter' -count=1
   ```

## Task 3: Provide an adapter catalog helper

**Objective:** Make it easy for runtime wiring to register safe local adapters without inventing adapters inside resolver.

**Files:**
- Create/modify: `internal/agentruntime/tools/catalog.go`
- Test: `internal/agentruntime/tools/catalog_test.go`

**Behavior:**
- Provide `NewStaticAdapterCatalog(adapters ...ToolAdapter) AdapterCatalog` or similar.
- Provide helper for Python adapter registration, e.g. `NewDefaultLocalAdapterCatalog(pythonExecutor pythonexec.Executor, opts ...)`.
- If `pythonExecutor` is nil, use `pythonexec.NewDefaultExecutor()` which is disabled.
- Resolver with `RequireAdapters=true` should resolve `python.execute` when this catalog is provided.

## Task 4: Document sandbox architecture and MCP positioning

**Objective:** Encode the decision from research so future implementation does not drift into unsafe in-process exec or treating MCP as sandbox.

**Files:**
- Create: `docs/design-docs/python-executor-sandbox.md`
- Modify: `docs/design-docs/agent-system-architecture.md`
- Maybe modify: `docs/design-docs/agent-runtime-eino.md`

**Required content:**
- V1 uses local `python.execute` + explicit adapter + disabled default executor.
- Real execution must be independent sandbox executor service / k8s pod; not Agent Service process.
- MCP can be optional transport/protocol later, not safety boundary.
- Prohibit shell, os/exec in Agent Service, Docker socket, host mounts, default network, arbitrary DB command/script execution.
- Recommended sandbox policy: timeout, CPU/memory, network disabled, no host dirs, read-only allowlisted files, stdout/stderr truncation, audit.

## Task 5: Final verification

Run:
```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
gofmt -w internal/model/*.go internal/agentruntime/tools/*.go internal/agent/pythonexec/*.go
go test ./internal/agentruntime/tools ./internal/agent/pythonexec ./internal/domain/agentaudit
go test ./...
bash scripts/verify-static.sh
git diff --check
```

No DB migration is expected unless implementation chooses to change persisted schema. If schema changes are made, add executable SQL under `db/migrations` and verify PostgreSQL integration when possible.
