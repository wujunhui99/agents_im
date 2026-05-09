# Issue 50 Group Management Plan

## Goal

Add a focused group management screen reachable from group chats, backed by server-side group permissions for detail, metadata update, and member removal.

## Approach

- Treat existing `groups.description` as the V1 group announcement and expose `announcement` as a frontend-friendly alias.
- Add role-aware group member data while preserving existing owner/member storage semantics.
- Add `PATCH /groups/:group_id` for group name/announcement and `DELETE /groups/:group_id/members/:user_id` for owner/admin kick.
- Keep avatar upload out of scope; render `avatar_url` when present and otherwise use the existing avatar placeholder.
- Implement tests first for backend permissions/removal and frontend title navigation/read-only/admin controls/grid layout.

## Files

- Backend contract and adapters: `api/groups.api`, `internal/types/types.go`, `internal/handler/gozero_routes.go`, `internal/handler/groups/*`, `internal/logic/groups/*_logic.go`.
- Backend domain and persistence: `internal/model/group.go`, `internal/repository/groups_repository.go`, `internal/repository/groups_memory.go`, `internal/repository/postgres_groups.go`, `internal/logic/groupslogic.go`.
- Backend tests: `internal/logic/groupslogic_acl_test.go`, `tests/groups_service_test.go`.
- Frontend API/UI/tests: `web/src/api/groups.ts`, `web/src/models/messages.ts`, `web/src/features/messages/MessagesPage.tsx`, `web/src/features/messages/MessagesPage.test.tsx`, `web/src/App.test.tsx`, `web/src/styles.css`.
- Docs: `docs/product-specs/frontend-backend-contract.md`.

## Verification

Focused tests first:

```bash
go test ./internal/logic ./tests -run 'TestGroupsLogic|TestGroupsHTTPHandlers' -count=1
npm --prefix web run test:run -- --reporter=dot web/src/features/messages/MessagesPage.test.tsx web/src/App.test.tsx
```

Final gates:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
for f in api/*.api; do goctl api validate -api "$f"; done
go test ./internal/logic ./internal/repository ./tests
npm --prefix web run test:run -- --reporter=dot
npm --prefix web run build
bash scripts/verify-static.sh
git diff --check
```
