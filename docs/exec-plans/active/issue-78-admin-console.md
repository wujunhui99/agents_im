# Issue 78 Read-only Admin Console

状态：Active

## 背景

Issue #78 requires an internal read-only operator console for LLM/Agent traces, conversation inspection, account/profile inspection, accepted friends, user conversation lists, and simple dashboard summaries. Admin access must fail closed and must not expose secrets, tokens, provider keys, DSNs, or mutation controls.

## 目标

- Add protected `/admin/...` REST endpoints served by `message-api`.
- Add a React admin console at `/admin`.
- Reuse repository/logic layers and keep handlers thin.
- Keep all admin behavior read-only.
- Cover backend and frontend behavior with strict TDD.

## 非目标

- No user/profile/friend/message mutation endpoints.
- No message impersonation.
- No production mock data fallback.
- No separate admin deployable unless the existing service boundary cannot support the read-only surface.

## 任务拆分

- [ ] Add failing backend logic and route tests for admin conversation lookup, user detail/friends/conversations, trace list/detail, admin-only auth, and response redaction.
- [ ] Add failing frontend tests for dashboard/navigation, conversation lookup, user detail/friends/conversations, trace links, loading/error/empty states, and absence of mutation/send controls.
- [ ] Implement repository read helpers for admin counts/search/list operations in memory and PostgreSQL.
- [ ] Implement `AdminLogic`, admin service context, read-only handlers, and `/admin/...` route registration with JWT plus account_type=`admin` guard.
- [ ] Wire admin routes into `message-api`, Vite proxy, and production ingress without breaking `/admin` SPA navigation.
- [ ] Implement `web/src/api/admin.ts` and `AdminConsole` UI under `/admin`.
- [ ] Update docs for usage and security boundary.
- [ ] Run focused tests, full backend/frontend verification, static checks, commit, push, PR, and Issue comment.

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-18 | Serve `/admin/...` APIs from `message-api`. | `message-api` already owns message and agent audit data and can construct account/friend repositories from the same storage config. This avoids adding a new deployment service while keeping a clear read-only prefix. |
| 2026-05-18 | Use `/admin` for the SPA route and specific `/admin/dashboard`, `/admin/llm-traces`, `/admin/conversations`, `/admin/users` prefixes for API proxy/ingress. | Keeps the operator URL simple while avoiding a prefix conflict between SPA navigation and API calls. |
| 2026-05-18 | Enforce admin by JWT identity plus Account `account_type=admin`. | The existing token does not carry role claims, so the guard must resolve the current account from the repository and fail closed when unavailable. |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
for f in api/*.api; do goctl api validate -api "$f"; done
go test ./internal/handler ./internal/logic ./internal/repository ./tests
go test ./...
npm --prefix web run test:run -- --reporter=dot
npm --prefix web run build
bash scripts/verify-static.sh
git diff --check
```

## 风险与回滚

- The admin API is attached to `message-api`; rollback is removing admin route registration and frontend route/proxy additions.
- If storage is misconfigured, the admin guard fails closed rather than returning partial data.
- PostgreSQL read helpers only query existing schema; no schema migration is expected.

## 结果记录

Pending implementation and verification.
