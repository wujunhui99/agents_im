# Group Chat V1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete GitHub Issue #2 group chat V1 for groups up to 200 active members across REST, message storage, WebSocket live push, React UI, and E2E harnesses.

**Architecture:** Keep one canonical `messages` row per group message and reuse `conversation_id + seq` ordering. Resolve active group participants in `MessageLogic`, pass them to storage for visibility/read state and to Gateway for immediate WebSocket push, and keep reconnect/history recovery through existing pull APIs. Add group list/bulk-create contracts and hydrate frontend group titles/member display names from real group APIs.

**Tech Stack:** Go/go-zero REST handlers and logic, in-memory and PostgreSQL repositories, WebSocket Gateway delivery dispatcher, React/Vite/TypeScript, Vitest/Testing Library, Node E2E scripts.

---

### Task 1: Backend Group Contracts And Validation

**Files:**
- Modify: `api/groups.api`
- Modify: `internal/types/types.go`
- Modify: `internal/repository/groups_repository.go`
- Modify: `internal/repository/groups_memory.go`
- Modify: `internal/repository/postgres_groups.go`
- Modify: `internal/logic/groupslogic.go`
- Modify: `internal/logic/groups/gozero_logic.go`
- Modify: `internal/handler/gozero_routes.go`
- Create: `internal/handler/groups/list_groups_handler.go`
- Test: `tests/groups_service_test.go`

- [x] Add tests proving `POST /groups` accepts selected `member_user_ids`, dedupes creator/duplicates, rejects more than 200 total members, returns/list groups for creator and selected members, and includes human-readable member profile fields.
- [x] Extend the group REST contract with optional `member_user_ids` and `GET /groups`.
- [x] Enforce `maxGroupMembersV1 = 200` in create and add-member paths before repository writes.
- [x] Make group creation transactional for creator plus selected active members in memory and PostgreSQL repositories.
- [x] Add `ListGroupsForUser` repository/logic/handler support.
- [x] Hydrate `GroupMemberInfo` with profile display fields when `GroupsLogic` has a user profile lookup.

### Task 2: Message Visibility And Push Recipients

**Files:**
- Modify: `internal/repository/message_repository.go`
- Modify: `internal/repository/message_memory.go`
- Modify: `internal/repository/postgres_message.go`
- Modify: `internal/repository/message_storage_contract.go`
- Modify: `internal/logic/messagelogic.go`
- Test: `internal/logic/messagelogic_test.go`
- Test: `internal/repository/message_repository_contract_test.go`
- Test: `internal/gateway/ws/server_test.go`
- Test: `tests/message_service_test.go`
- Test: `tests/websocket_gateway_test.go`

- [x] Add tests proving new group members only pull messages with `seq > visible_start_seq`.
- [x] Add tests proving active group recipients excluding sender are returned for outbox/live push.
- [x] Add tests proving non-members and inactive members cannot send or explicitly read group conversations.
- [x] Treat `visible_start_seq` as a lower bound, preserving it on existing user conversation states.
- [x] For newly visible participants in an existing group conversation, initialize `visible_start_seq` to the conversation max seq before the newly accepted message.
- [x] Filter group conversation reads through active membership checks in `MessageLogic`.
- [x] Keep direct chat seq, idempotency, history, and push behavior unchanged.

### Task 3: Frontend Group UX

**Files:**
- Modify: `web/src/api/groups.ts`
- Modify: `web/src/models/messages.ts`
- Modify: `web/src/components/ContactsPage.tsx`
- Modify: `web/src/features/messages/MessagesPage.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/styles.css`
- Test: `web/src/components/ContactsPage.test.tsx`
- Test: `web/src/features/messages/MessagesPage.test.tsx`
- Test: `web/src/App.test.tsx`

- [x] Add tests for group creation with selected friends through real `groupsApi.createGroup`, visible Chinese validation for zero selections and max members, and opening the new group.
- [x] Add tests for group conversation loading using `GET /groups`, `GET /groups/:id/members`, history pull, title hydration, sender display names, live group event merging, and Chinese send/permission errors.
- [x] Make the 联系人 tab `群聊` entry open the group creation/list panel instead of a disabled placeholder.
- [x] Add typed group list/create/member APIs and wire authenticated API clients through `App`.
- [x] Let `MessagesPage` accept/open a pending group, hydrate group conversations, render non-self group sender display names, and send group messages through `POST /messages`.
- [x] Preserve the four-tab shell and direct-chat behavior.

### Task 4: E2E Harness And Docs

**Files:**
- Create: `tests/e2e/group_chat_regression.mjs`
- Modify: `tests/e2e/README.md`
- Modify: `docs/product-specs/frontend-backend-contract.md`
- Modify: `docs/FRONTEND.md`

- [x] Add a production/local-capable Node harness with classifications `group-chat-success`, `group-chat-history-success`, `group-chat-permission-denied`, and `group-chat-max-members-rejected`.
- [x] Redact tokens, passwords, cookies, and bearer headers from observations.
- [x] Document the new group list/create contract and frontend group UX.

### Task 5: Verification And Delivery

**Files:**
- All changed files

- [x] Run focused backend tests while iterating.
- [x] Run focused frontend tests while iterating.
- [x] Run `go test ./...`.
- [x] Run `npm --prefix web test -- --run`.
- [x] Run `npm --prefix web run build`.
- [x] Run `bash scripts/verify-static.sh`.
- [x] Run `git diff --check`.
- [ ] Commit with `feat(groups): complete group chat v1`.
- [ ] Comment on GitHub Issue #2 with summary, verification, commit SHA, and risks/blockers.
