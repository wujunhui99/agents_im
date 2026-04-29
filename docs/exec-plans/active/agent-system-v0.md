# Agent System V0 Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Build the first Agent System foundation: account types, Agent metadata, prompt/tool/skill registries, MinIO-backed skill files, and a safe Python execution seam without shell execution.

**Parallel baseline:** See [`agent-infrastructure-parallel-baseline.md`](./agent-infrastructure-parallel-baseline.md) for the current multi-Codex branch split, shared contracts, quality bars, and integration order.

**Architecture:** Keep IM Backend as the identity/message source of truth. Add Agent System management and runtime components that integrate through existing IM events and Message Service write-back. Store metadata in PostgreSQL, skill files in MinIO/S3-compatible storage, and execute Python only through a sandbox boundary.

**Tech Stack:** Go/go-zero for existing IM backend and management APIs unless otherwise decided; PostgreSQL for metadata; MinIO for skill objects; Redis/Kafka where existing runtime/event seams apply; Python executor as a separate sandbox service or isolated worker.

---

## Phase 0: Freeze Contract And Data Model

### Task 1: Add account type contract tests

**Objective:** Define `normal`, `agent`, and `admin` account type behavior before implementation.

**Files:**
- Modify: `api/user.api`
- Modify: `db/migrations/001_init_postgres.sql` or new migration
- Test: `tests/user_account_type_test.go`

**Verification:**

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./tests -run AccountType -count=1
```

Expected first result before implementation: failing tests that show `account_type` is missing.

### Task 2: Implement account type persistence

**Objective:** Persist and return `account_type` while preserving existing normal-user registration behavior.

**Files:**
- Modify: user repository/model files
- Modify: auth registration path to default to `normal`
- Modify: frontend/backend contract docs if response shape changes

**Verification:**

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
```

**Feature branch note (`feature/agent-account-types`):**

- `account_type` foundation is implemented only for the user domain: `normal` default plus validated internal `agent`/`admin` creation.
- Public HTTP `POST /users` and `auth` register do not accept account type elevation and continue to create `normal` users by default.
- No Agent config tables, prompt/tool/skill CRUD, Python executor, shell execution, or LLM integration are part of this branch.
- Validation recorded for this branch: `goctl --version`, `goctl api validate` for all `api/*.api`, `gofmt`, `go test ./...`, `go test -tags=integration ./tests -run TestPostgresUserAuthFriendsGroupsRepositories -count=1` with no DSN, `bash scripts/verify-static.sh`, `docker compose config`, and `git diff --check`.

## Phase 1: Agent Metadata And Prompt Management

### Task 3: Add Agent and prompt schema

**Objective:** Create PostgreSQL schema for `agents`, `agent_prompts`, and prompt snapshots in `agent_runs`.

**Files:**
- Create: `db/migrations/00X_agent_system.sql`
- Create/Modify: `docs/generated/db-schema.md`

**Verification:**

```bash
docker compose config
bash scripts/migrate-postgres.sh
```

### Task 4: Add Agent management API contract

**Objective:** Define go-zero `.api` contract for Agent CRUD and prompt CRUD.

**Files:**
- Create: `api/agent.api`
- Create: `docs/product-specs/agent-system.md` updates if contract changes

**Verification:**

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH goctl api validate -api api/agent.api
```

**Current branch note (2026-04-30):** `feature/agent-core-management` implements the Agent profile slice only: `api/agent.api`, `cmd/agent-api`, Agent logic/repository, and `agents` schema. Prompt CRUD remains out of scope for this branch and is still planned under later prompt/tool/skill work.

## Phase 2: Tools And Skills

### Task 5: Add tool registry schema and API

**Objective:** Store MCP/local/builtin tool metadata and bind tools to agents.

**Files:**
- Modify: `db/migrations/00X_agent_system.sql`
- Modify: `api/agent.api`
- Test: tool binding authorization tests

**Key rule:** Database stores `handler_key`; code owns the executable whitelist. Do not execute scripts stored in DB.

**2026-04-30 progress (`feature/agent-prompts-tools-skills`):**

- Added prompt/tool/skill metadata models, registry repository interface, in-memory repository, PostgreSQL repository methods, and fail-first business validation in `internal/logic`.
- Added PostgreSQL schema for `agent_prompts`, `mcp_servers`, `agent_tools`, `agent_skills`, and prompt/tool/skill binding tables in `db/migrations/001_init_postgres.sql`.
- Encoded first-version MCP constraints: admin-configured server/tool metadata only, no stdio command/args transport, and Agent whitelist binding required before `CanAgentUseTool` returns true.
- API exposure remains pending for a later Agent Management API surface; current branch provides domain/repository/metadata contract only.

### Task 6: Add MinIO-backed skill registry

**Objective:** Store skill metadata in PostgreSQL and skill files in MinIO/S3-compatible storage.

**Files:**
- Modify: `docker-compose.yml`
- Modify: `.env.example`
- Modify: migration for `agent_skills` and `agent_skill_files`
- Add: MinIO client/storage package

**Verification:**

```bash
docker compose config
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...
```

Integration tests requiring MinIO must skip unless explicit MinIO environment variables are set.

**2026-04-30 progress (`feature/agent-prompts-tools-skills`):**

- Added the skill metadata contract in PostgreSQL with required `object_key`, `sha256`, `content_type`, and `size_bytes`.
- Did not implement MinIO binary upload/download; file content remains out of PostgreSQL and out of scope for this branch.

## Phase 3: Runtime And Python Execution

### Task 7: Add Agent run and audit records

**Objective:** Record Agent runs, tool calls, skill file reads, and Python execution attempts.

**Files:**
- Modify: migration
- Add: repository interfaces and tests

**Branch note (`feature/agent-audit-log`):**

- Added append-only audit foundation for `agent_runs`, `agent_tool_calls`, `agent_file_reads`, and `agent_python_execs`.
- Audit repository/logic supports create/get/list-by-run-id; update/delete are intentionally absent.
- PostgreSQL migration adds append-only triggers that reject direct update/delete.
- Summary fields are recursively redacted; Python code is stored only as `sha256` and `size_bytes`.
- This task does not implement LLM execution, tool execution, or Python execution.

**Verification target:**

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./internal/domain/agentaudit ./internal/repository ./internal/logic -run 'AgentAudit|Redact|PythonCode' -count=1
```

### Task 7A: Add Eino runtime core contract

**Objective:** Freeze the local Agent runtime boundary before adding any Eino/DeepSeek adapter.

**Branch note (`feature/eino-runtime-core`):**

- Added `internal/agentruntime.Runtime` with `Run(ctx, RunRequest) (RunResult, error)`.
- Added plain domain structs for `AgentConfig`, prompt snapshots, model config, tool refs, skill refs, runtime policy, conversation context, usage, model metadata, and tool-call result metadata.
- `RunRequest` carries fields needed from `internal/agentim.AgentTrigger`, including request/event/trace ids, trigger type, target Agent user id, requesting user, conversation id/type, trigger message id/seq, prompt text, recursion/source fields, and target Agent user ids.
- Added fail-first `NormalizeRunRequest` / `NormalizeRunResult` validation with `apperror.InvalidArgument` for missing agent ids, prompt id/content, prompt text, model provider/model, invalid tool metadata shapes, negative counters, and empty successful final text.
- This branch intentionally does not import Eino packages, call DeepSeek, execute tools/Python, orchestrate IM write-back, or write message repositories directly.
- Design boundary documented in [`../../design-docs/agent-runtime-eino.md`](../../design-docs/agent-runtime-eino.md).

**Verification target:**

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./internal/agentruntime
```

### Task 8: Add Python executor seam without shell

**Objective:** Add `python.execute` as a tool interface with timeout/resource policy and no shell support.

**Files:**
- Add: Python executor service or adapter package
- Add: policy validation tests

**Required constraints:**

- no shell/command execution API;
- timeout required;
- no host filesystem by default;
- no Docker socket;
- no network by default;
- explicit audit record for every execution.

**Implementation note (2026-04-30, `feature/agent-python-sandbox-contract`):**

- Added `internal/agent/pythonexec` contract types and `Executor.Execute(ctx, Request)` interface.
- `Policy` now validates `run_id`, `audit_id`, timeout, CPU/memory limits, disabled/default network policy, explicit read-only relative file allowlist, and `max_output_bytes`.
- `NewDefaultExecutor()` returns the disabled executor; valid requests fail with `ErrPythonExecutorDisabled` until a real sandbox service is intentionally wired.
- Added static test and `scripts/verify-static.sh` guard to reject production Go imports/calls/literals that would directly execute shell or Python from `cmd/` or `internal/`.

## Phase 4: IM Integration

### Task 9: Connect IM event to Agent run

**Objective:** Consume or receive `message.created` events, detect active Agent recipients, run Agent, and write response via Message Service.

**Files:**
- Modify: IM-Agent contract implementation area
- Test: single Agent receives message and writes response through Message Service

**Current feature branch note (`feature/agent-im-integration-contract`):**

- Added `internal/agentim` contract types for `message.created` triggers, group mentions, admin manual runs, Agent response metadata, and Message Service response writing.
- This branch intentionally does not implement LLM runtime execution or an event consumer. It freezes the trigger/writeback contract used by later runtime wiring.

**Current feature branch note (`feature/eino-im-runner`, reconciled on develop):**

- Added `internal/agentim.AgentRunOrchestrator` as the first runner seam from `AgentTrigger` to the shared `internal/agentruntime.Runtime` interface, append-only run audit, and `ResponseWriter`.
- Runner order is fail-first: validate trigger, build and validate an `agentruntime.RunRequest` through an explicit `RuntimeRequestBuilder`, call runtime, create a final `agent_runs` audit record, then write non-empty final text through `AgentResponseRequest` / Message Service response writer. Request builder failures, runtime failures, invalid runtime results, and empty final text create failed audit records and return explicit errors; audit failures stop response write-back.
- The runner does not include a concrete Eino/DeepSeek adapter, tool registry resolution, direct message repository writes, shell execution, Python execution, or live provider calls.
- Unit coverage includes success, request-builder failure, runtime failure, audit failure, empty final text rejection, recursion policy gating, and response-writer failure propagation.

### Task 10: Add loop prevention

**Objective:** Prevent Agent messages from recursively triggering Agent runs unless explicitly enabled.

**Current feature branch note (`feature/agent-im-integration-contract`):**

- Loop prevention is encoded in `AgentMessageMetadata`: Agent responses suppress recursive triggers unless both runtime policy and message metadata explicitly allow recursion.
- Unit tests cover default suppression and explicit opt-in behavior.

**Verification:**

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
docker compose config
```

## Non-goals For V0

- shell/command execution;
- user-provided stdio MCP server startup;
- public skill marketplace;
- multi-agent autonomous negotiation;
- long-term memory;
- front-end visual Agent builder.
