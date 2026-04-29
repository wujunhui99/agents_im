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

## Phase 2: Tools And Skills

### Task 5: Add tool registry schema and API

**Objective:** Store MCP/local/builtin tool metadata and bind tools to agents.

**Files:**
- Modify: `db/migrations/00X_agent_system.sql`
- Modify: `api/agent.api`
- Test: tool binding authorization tests

**Key rule:** Database stores `handler_key`; code owns the executable whitelist. Do not execute scripts stored in DB.

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

## Phase 3: Runtime And Python Execution

### Task 7: Add Agent run and audit records

**Objective:** Record Agent runs, tool calls, skill file reads, and Python execution attempts.

**Files:**
- Modify: migration
- Add: repository interfaces and tests

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

### Task 10: Add loop prevention

**Objective:** Prevent Agent messages from recursively triggering Agent runs unless explicitly enabled.

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
