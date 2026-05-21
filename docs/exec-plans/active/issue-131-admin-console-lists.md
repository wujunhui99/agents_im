# Issue 131 Admin Console Lists

状态：Completed

## 背景

Issue #131 requires the production admin console to expose browseable Conversation and Users lists before an operator enters a search key or conversation ID. Existing admin APIs are read-only and JWT-protected.

## 目标

- Conversation page shows a bounded default conversation list with ID, max seq, last-message summary, and time, and each row opens the existing message inspector.
- Users page shows a bounded default user list using the existing empty-query admin user search contract, and each row opens the existing user detail, friends, and conversations drill-down.
- Existing conversation ID lookup and user search continue to work.

## 非目标

- No admin mutations, delete actions, impersonated sending, or fake production data.
- No database schema change.
- No new backend route unless existing bounded read-only contracts cannot satisfy the acceptance criteria.

## 任务拆分

- [x] Add failing `AdminConsole` tests for default conversation list rendering and row drill-down.
- [x] Add failing `AdminConsole` tests for default user list rendering, row drill-down, search, and empty-query reset behavior.
- [x] Confirm existing typed admin frontend API can use `GET /admin/users?query=&limit=20` for browsing.
- [x] Update `AdminConsole` to render dashboard recent conversations on the Conversation page and load default users when entering Users or submitting an empty query.
- [x] Run required frontend build/test, static verification, diff check, and backend tests.
- [ ] Commit, push, open PR to `develop`, and comment on Issue #131.

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-21 | Reuse `GET /admin/dashboard` recent conversations for the Conversation default list. | It is already guarded, read-only, bounded, and includes `AdminConversation` with last-message metadata. |
| 2026-05-21 | Reuse `GET /admin/users?query=&limit=20` for default users. | Existing repository logic treats empty query as a bounded account list, so no backend route is needed. |

## 验证方式

```bash
npm --prefix web run test:run -- --reporter=dot src/pages/AdminConsole.test.tsx
npm --prefix web run build
bash scripts/verify-static.sh
git diff --check
```

Backend files did not change, but full backend tests were run with the documented Go PATH:

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...
```

## 风险与回滚

The main risk is coupling the Conversation page browse list to dashboard availability. If that becomes insufficient, add a dedicated read-only `GET /admin/conversations?limit=&offset=` route in a follow-up. Rollback is limited to reverting the frontend console/API adapter changes.

## 结果记录

Implemented with no backend route or schema changes. Verification passed:

```bash
npm --prefix web run test:run -- --reporter=dot src/pages/AdminConsole.test.tsx
npm --prefix web run build
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
git diff --check
```
