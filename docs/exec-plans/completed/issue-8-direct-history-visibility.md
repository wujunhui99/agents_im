# issue-8-direct-history-visibility

状态：Completed

## 背景

Issue #8 reports that old direct-chat history became hidden after deploy. The message pull path validates group membership in logic, then PostgreSQL reads through `user_conversation_states`; direct conversations rely on repository state rows for visibility. Legacy direct conversations can have valid `conversation_threads` and `messages` rows while missing per-user state rows.

## 目标

- Restore old direct-chat history for canonical direct participants.
- Set repaired direct `visible_start_seq` to `0` so `fromSeq=1&order=asc` returns old messages.
- Keep group `visible_start_seq` join boundaries unchanged.
- Prevent unrelated users from reading or triggering direct-state repair.

## 非目标

- Do not change group membership policy.
- Do not delete or rewrite existing messages.
- Do not expose production credentials or host details.

## 任务拆分

- [x] Task 1：Trace `PullMessages`, seq-state lookup, user-scoped pull, and visibility-state writes.
- [x] Task 2：Add failing repository regression tests for missing direct state repair and group join boundary.
- [x] Task 3：Add participant-scoped direct conversation access validation in logic.
- [x] Task 4：Add PostgreSQL and memory repository direct-state repair for explicit and empty seq-state reads, pull, and mark-read.
- [x] Task 5：Add safe idempotent SQL backfill for existing direct conversation participants.
- [x] Task 6：Run required verification commands and commit locally.

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-05 | Repair only `conversation_type = 1` direct threads using `single_account_a/b` participants. | Prevents widening access and preserves group join visibility. |
| 2026-05-05 | Keep direct repair idempotent with `visible_start_seq = 0` and no message deletion. | Direct chats have no join-history boundary; repeated deploy migrations must be safe. |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
gofmt -w $(find . -name '*.go' -print)
go test ./internal/logic ./internal/repository ./tests -count=1
go test ./...
bash scripts/verify-static.sh
git diff --check
```

## 风险与回滚

The main risk is accidentally applying direct-chat repair semantics to groups. The implementation gates all repair SQL and memory fallback on direct conversation type only. Rollback is the code revert; the SQL backfill only inserts missing direct participant state rows or resets direct `visible_start_seq` to `0`, so it does not remove data.

## 结果记录

2026-05-05 verification:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
gofmt -w $(find . -name '*.go' -print)
go test ./internal/logic ./internal/repository ./tests -count=1
go test ./...
bash scripts/verify-static.sh
git diff --check
```

Runtime repair and migration backfill are both idempotent and direct-chat scoped.
