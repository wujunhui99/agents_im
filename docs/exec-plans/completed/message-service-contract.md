# Message Service Contract Implementation Plan

Status: Completed  
Completed: 2026-04-29

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Create the first message-service interface contract so message storage, gateway mapping, and read receipts can be developed in parallel.

**Architecture:** Start with a go-zero style message service skeleton. Phase 1 keeps send/pull/read behavior synchronous inside message service while preserving future MQ/gateway event contracts. The service uses deterministic conversation IDs, per-conversation seq, idempotent client message IDs, and monotonic read state.

**Tech Stack:** Go, go-zero style API/RPC layout, `.api`, `.proto`, in-memory repository for first tests, future PostgreSQL/Redis/Kafka.

---

## References

Read these before implementing:

- `AGENTS.md`
- `ARCHITECTURE.md`
- `docs/product-specs/message-chain.md`
- `docs/design-docs/message-chain-contract.md`
- `docs/design-docs/user-auth-friends-groups-boundaries.md`
- existing service examples under `api/`, `proto/`, `internal/logic/`, `internal/repository/`, `tests/`

## Branch and worktree

Create a new feature branch from latest `main`:

```bash
git fetch origin
git worktree add /home/ws/project/worktrees/message-contract -b feature/message-service-contract origin/main
```

Run Codex with yolo mode for autonomous commit/push:

```bash
codex exec --yolo '<task prompt>'
```

Codex must commit and push the feature branch after verification.

## Task 1: Add message proto contract

**Objective:** Define service-to-service message RPC contract.

**Files:**

- Create: `proto/message.proto`

**Steps:**

1. Create `proto/message.proto` with package `message` and go package `github.com/wujunhui99/agents_im/proto/messagepb`.
2. Define `MessageService` with:
   - `SendMessage`
   - `PullMessages`
   - `GetConversationSeqs`
   - `MarkConversationAsRead`
3. Define shared `Message`, `ConversationSeqState`, and request/response messages exactly following `docs/design-docs/message-chain-contract.md`.
4. Run static validation script after it is updated in a later task.

**Verification:**

```bash
grep -q 'service MessageService' proto/message.proto
grep -q 'rpc SendMessage' proto/message.proto
grep -q 'rpc MarkConversationAsRead' proto/message.proto
```

## Task 2: Add message HTTP API contract

**Objective:** Define HTTP endpoints for phase 1 clients and future gateway mapping.

**Files:**

- Create: `api/message.api`

**Endpoints:**

```text
POST /messages
GET  /conversations/:conversation_id/messages
GET  /conversations/seqs
POST /conversations/:conversation_id/read
```

**Requirements:**

- Use `X-User-Id` in phase 1 to mirror existing skeleton services until auth gateway wiring is complete.
- Keep JSON field names camelCase.
- Do not include password/auth secret fields.

**Verification:**

```bash
grep -q 'post /messages' api/message.api
grep -q 'get /conversations/:conversation_id/messages' api/message.api
grep -q 'post /conversations/:conversation_id/read' api/message.api
```

## Task 3: Add message domain types and repository interface

**Objective:** Create internal Go types matching the contract.

**Files:**

- Create or modify: `internal/repository/message_repository.go`
- Create or modify: `internal/logic/messagelogic.go`

**Repository interface operations:**

```go
CreateMessageIdempotent(ctx context.Context, input CreateMessageInput) (Message, bool, error)
GetMessages(ctx context.Context, conversationID string, fromSeq, toSeq int64, limit int, order string) ([]Message, bool, int64, error)
GetConversationSeqStates(ctx context.Context, userID string, conversationIDs []string) ([]ConversationSeqState, error)
SetUserHasReadSeqMax(ctx context.Context, userID, conversationID string, seq int64) (ConversationSeqState, bool, error)
```

**Verification:**

```bash
PATH=/tmp/go/bin:$PATH gofmt -w internal/repository/message_repository.go internal/logic/messagelogic.go
```

## Task 4: Implement memory repository

**Objective:** Provide deterministic in-memory behavior for first tests.

**Files:**

- Create or modify: `internal/repository/message_memory.go`

**Required behavior:**

- deterministic single/group conversation ID helper;
- per-conversation seq allocation;
- `sender_id + client_msg_id` idempotency;
- idempotency conflict detection;
- sender read seq advances after send;
- monotonic mark-as-read.

**Verification:**

Run targeted tests from Task 5.

## Task 5: Add message service tests

**Objective:** Lock contract behavior before gateway/storage parallel work starts.

**Files:**

- Create: `tests/message_service_test.go`

**Test cases:**

1. Single chat send creates conversation and seq 1.
2. Retrying same `sender_id + client_msg_id` returns deduplicated response.
3. Reusing same idempotency key with different content fails.
4. Pull messages returns ascending seq range.
5. Sender read seq advances after send.
6. Mark as read rejects seq greater than max seq.
7. Mark as read is monotonic.
8. Conversation seq query returns max/read/unread.

**Verification:**

```bash
PATH=/tmp/go/bin:$PATH go test ./...
```

Expected: all tests pass.

## Task 6: Update service context and static verification

**Objective:** Register message logic/repository and ensure contract files are required.

**Files:**

- Modify: `internal/svc/service_context.go`
- Modify: `scripts/verify-static.sh`

**Requirements:**

- Add message repository/logic to service context without breaking user/auth/friends/groups.
- Static script must require:
  - `api/message.api`
  - `proto/message.proto`
  - `docs/product-specs/message-chain.md`
  - `docs/design-docs/message-chain-contract.md`
  - `docs/exec-plans/active/message-service-contract.md`
- Static script must ensure message docs do not assign password/auth secrets to message service.

**Verification:**

```bash
bash scripts/verify-static.sh
```

Expected:

```text
static verification passed
```

## Task 7: Run final verification and commit

**Objective:** Produce a clean pushed feature branch.

**Commands:**

```bash
PATH=/tmp/go/bin:$PATH gofmt -w $(find . -name '*.go' -print)
PATH=/tmp/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
git status --short
git add .
git commit -m "feat(message): add message service contract"
git push -u origin feature/message-service-contract
```

**Completion criteria:**

- Branch is clean after commit.
- Branch is pushed to origin.
- Final response includes commit SHA and verification output.

## Result Record

### Planner

- Read required context: `AGENTS.md`, `ARCHITECTURE.md`, `docs/product-specs/message-chain.md`, `docs/design-docs/message-chain-contract.md`, and this execution plan.
- Confirmed phase 1 scope: synchronous message service skeleton, deterministic conversation IDs, per-conversation seq, send idempotency, pull-by-seq, conversation read state, and future-compatible API/RPC contracts.
- Confirmed `goctl` is unavailable in this worktree runtime, so files were handwritten using the existing user/auth/friends/groups skeleton style.

### Generator

- Added `proto/message.proto` with `MessageService` RPCs and shared `Message` / `ConversationSeqState` schemas.
- Added `api/message.api` for `POST /messages`, `GET /conversations/:conversation_id/messages`, `GET /conversations/seqs`, and `POST /conversations/:conversation_id/read`.
- Added message logic and in-memory repository with deterministic `single:{lower_user_id}:{higher_user_id}` / `group:{group_id}` conversation IDs, idempotent send, seq allocation, sender read advancement, pull, seq query, and monotonic mark-read behavior.
- Added message HTTP handler skeleton and mounted `MessageLogic` / `MessageRepository` in `internal/svc/service_context.go` without changing user/auth/friends/groups contracts.
- Added `tests/message_service_test.go` covering single-chat seq, idempotent retry, idempotency conflict, seq pull, sender read advancement, mark-read rejection above max, monotonic mark-read, and unread seq query.
- Updated `scripts/verify-static.sh` to require message API/proto/docs/test/source checks and guard message contract sources/docs from auth-secret ownership.

### Evaluator

Verification run:

```bash
PATH=/tmp/go/bin:$PATH gofmt -w $(find . -name "*.go" -print)
PATH=/tmp/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
```

Results:

- `go test ./...`: passed.
- `bash scripts/verify-static.sh`: `static verification passed`.

Residual risk:

- Group-chat validation is adapter-ready through `GroupMemberLister`, but the default message context does not wire a groups service instance. Full group membership enforcement should be completed when service composition is finalized.
