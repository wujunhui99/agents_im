# Default Agent Creator Assistant Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the canonical `agent_creator` assistant and ensure every human account has it as an accepted contact.

**Architecture:** Add a backend provisioner that uses the real account/profile, friendship, agent, and prompt registry repositories. PostgreSQL receives an idempotent executable data migration for existing deployments, while user/auth account creation paths call the same provisioner for new human accounts.

**Tech Stack:** Go, go-zero service contexts, in-memory and PostgreSQL repositories, PostgreSQL SQL migrations/change log, GitHub CLI.

---

### Task 1: Failing Provisioner Tests

**Files:**
- Test: `internal/logic/default_assistant_test.go`

- [ ] **Step 1: Write RED tests** for missing `agent_creator` creation, human-user friendship backfill, idempotency, registration-time provisioning, legacy `agent_father` rename, and admin/agent exclusion.
- [ ] **Step 2: Run focused tests** with `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./internal/logic -run 'TestDefaultAssistant|TestUserLogicCreateUserEnsuresDefaultAssistantFriend' -count=1`; expect compile/test failure before implementation.

### Task 2: Repository Support

**Files:**
- Modify: `internal/repository/repository.go`
- Modify: `internal/repository/memory.go`
- Modify: `internal/repository/postgres_user_friends.go`
- Modify: `internal/repository/agent_registry_repository.go`
- Modify: `internal/repository/agent_registry_memory.go`
- Modify: `internal/repository/postgres_agent_registry.go`

- [ ] **Step 1: Add real repository methods** for listing accounts by type, renaming an identifier, accepted friendship upsert, and prompt lookup by name/version.
- [ ] **Step 2: Re-run focused tests** and keep expected failures limited to missing provisioner logic.

### Task 3: Provisioner And Account Creation

**Files:**
- Create: `internal/logic/default_assistant.go`
- Modify: `internal/logic/userlogic.go`

- [ ] **Step 1: Implement `DefaultAssistantProvisioner`** with `agent_creator` constants, legacy rename, active Agent config creation, active prompt creation/binding, and accepted friendship backfill for `account_type=user` only.
- [ ] **Step 2: Add opt-in UserLogic wiring** so configured account creation provisions `agent_creator` for new human users and skips agent/admin accounts.
- [ ] **Step 3: Re-run focused tests** and expect pass.

### Task 4: Runtime Wiring And SQL Backfill

**Files:**
- Modify: `internal/servicecontext/user/service_context.go`
- Modify: `cmd/user-api/main.go`
- Modify: `cmd/auth-api/main.go`
- Modify: `internal/rpcgen/user/internal/svc/service_context.go`
- Modify: `internal/rpcgen/auth/internal/svc/service_context.go`
- Create: `db/migrations/009_default_agent_creator_assistant.sql`
- Create: `db/change_log/2026-05-18-default-agent-creator-assistant.sql`
- Modify: `docs/product-specs/agent-chat.md`

- [ ] **Step 1: Wire user/auth services** to construct agent and registry repositories, configure UserLogic, and run startup backfill fail-visible.
- [ ] **Step 2: Add executable PostgreSQL migration/change-log SQL** that renames legacy `agent_father`, creates or updates `agent_creator`, creates active Agent/prompt/binding records, and inserts accepted friendships for all `account_type=user` accounts.
- [ ] **Step 3: Document user-visible default assistant behavior** in `docs/product-specs/agent-chat.md`.

### Task 5: Verification And Handoff

**Files:**
- Verify all modified files.

- [ ] **Step 1: Run required verification:** `PATH=/tmp/go/bin:$HOME/go/bin:$PATH; for f in api/*.api; do goctl api validate -api "$f"; done; go test ./internal/logic ./internal/repository ./tests; go test ./...; bash scripts/verify-static.sh; git diff --check`.
- [ ] **Step 2: If PostgreSQL is configured, run:** `AGENTS_IM_CONFIRM_TRUNCATE=1 scripts/verify-postgres-local.sh`; otherwise record the blocker.
- [ ] **Step 3: Commit, push `feature/issue-77-agent-creator-default-friend`, open PR to `develop`, and comment issue #77 with summary, tests, branch, commit, PR, and blockers.
