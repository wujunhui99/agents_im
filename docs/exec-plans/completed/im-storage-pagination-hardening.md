# IM Storage Pagination Hardening

状态：Completed

## 背景

Message storage already supports memory and PostgreSQL implementations, but pagination and input edge cases were mostly covered through higher-level tests. The storage boundary needed regression tests for order validation, range normalization, read-state visibility, and deterministic conversation IDs.

## 目标

- Add repository contract tests that run against memory by default.
- Add opt-in PostgreSQL contract coverage without requiring local middleware for ordinary tests.
- Pin pagination semantics for asc/desc order, limit+1, max-limit clipping, empty ranges, invalid order, and seq edge cases.
- Ensure conversation ID delimiter and length assumptions fail visibly.
- Add logic/API-level boundary tests for `client_msg_id`, `content`, `limit`, and non-participant state access.

## 非目标

- Add new message product features.
- Change the message delivery/outbox contract.
- Require PostgreSQL for default `go test` runs.

## 任务拆分

- [x] Review existing message logic, memory repository, PostgreSQL repository, and migrations.
- [x] Add shared validation for repository pagination/order and conversation ID component safety where needed.
- [x] Add memory and opt-in PostgreSQL repository contract tests.
- [x] Add message logic boundary tests.
- [x] Update storage/message documentation for ID constraints and PostgreSQL contract test opt-in.
- [x] Run required verification and record results.

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-01 | PostgreSQL repository contract tests require `AGENTS_IM_TEST_POSTGRES_CONTRACT=1` plus a DSN. | Keeps default local tests green without live middleware while preserving an explicit integration path. |
| 2026-05-01 | Repository-level `order`, seq range, limit, and create-input validation now mirrors message logic. | Prevents direct storage callers from silently diverging from API behavior and keeps PostgreSQL SQL order construction limited to validated constants. |
| 2026-05-01 | Message ID components reject `:` and NUL; derived `conversation_id` must fit 256 characters. | `:` is the deterministic conversation ID delimiter and generated conversation IDs must remain pullable through API validation. |

## 验证方式

- `PATH=/tmp/go/bin:$PATH gofmt -w internal/logic/messagelogic.go internal/logic/messagelogic_test.go internal/repository/message_memory.go internal/repository/postgres_message.go internal/repository/message_validation.go internal/repository/message_repository_contract_test.go` -> passed.
- `PATH=/tmp/go/bin:$PATH go test ./internal/logic ./internal/repository` -> passed.
- `PATH=/tmp/go/bin:$PATH go test ./...` -> passed.
- `PATH=/tmp/go/bin:$PATH go test ./internal/repository -run TestMessageRepositoryContractPostgresOptIn -count=1 -v` -> passed with the PostgreSQL contract test skipped by default because `AGENTS_IM_TEST_POSTGRES_CONTRACT=1` was not set.
- `git diff --check` -> passed.
- `PATH=/tmp/go/bin:$PATH bash scripts/verify-static.sh` -> passed.

PostgreSQL live contract execution was not run in this environment because no opt-in DSN was configured. Documented command:

```bash
AGENTS_IM_TEST_POSTGRES_CONTRACT=1 \
DATABASE_URL=postgres://agents_im:agents_im_dev_password@localhost:5432/agents_im?sslmode=disable \
go test ./internal/repository
```

## 风险与回滚

The behavior changes are intentionally fail-first for invalid repository inputs. If a caller depends on repository-level negative seq, negative limit, invalid order, delimiter-containing user/group IDs, or overlong derived conversation IDs being accepted, that caller should be fixed to send the documented message contract. Rollback is a normal revert of this branch.

## 结果记录

Implemented shared repository validation for message create inputs and pull pagination. Added memory-default and PostgreSQL-opt-in repository contract tests covering pagination, malicious order rejection, limit clipping, empty ranges, invalid ranges, and participant-scoped conversation state. Added message logic boundary tests for client message ID length, content length, limit clipping, invalid order, unsafe conversation components, overlong derived conversation IDs, and non-participant state/pull/read access. Updated message product and storage design docs with accepted ID constraints and the opt-in PostgreSQL contract test workflow.
