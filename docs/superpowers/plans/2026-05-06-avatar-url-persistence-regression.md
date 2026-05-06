# Avatar URL Persistence Regression Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix Issue #4 so uploaded avatars remain visible after refresh and relogin.

**Architecture:** Store a durable `profiles.avatar_url` beside `profiles.avatar_media_id` because avatar display is profile-owned data, while identity remains in `accounts`. The stored URL is a stable same-origin reference (`/media/avatars/{media_id}`), not image bytes, a raw object key, credentials, or an expiring presigned URL. Profile/auth/friend responses return this persisted URL; the stable media route validates avatar media and redirects to a fresh object-store display URL at request time.

**Tech Stack:** Go/go-zero, PostgreSQL/sqlx, React/Vite, Vitest.

---

### Task 1: Backend Regression Tests

**Files:**
- Modify: `internal/repository/postgres_account_profiles_test.go`
- Modify: `internal/logic/user/gozero_logic_test.go`
- Add: `internal/auth/logic/authlogic_test.go`

- [ ] **Step 1: Write the failing repository test**

Add a test that expects `PostgresRepository.UpdateAvatar(ctx, accountID, mediaID, avatarURL)` to update both `avatar_media_id` and `avatar_url`, then return the persisted URL from `GetByID`.

- [ ] **Step 2: Write the failing user API test**

Update the avatar go-zero test so `PATCH /me/avatar` returns `/media/avatars/{media_id}`, and a later `GET /me` returns the same durable URL without object-store URL regeneration.

- [ ] **Step 3: Write the failing auth logic test**

Add a login regression proving auth responses include `display_name`, profile fields, `avatar_media_id`, and durable `avatar_url` from the account/profile client.

- [ ] **Step 4: Run targeted backend tests to verify RED**

Run:

```bash
go test ./internal/repository ./internal/logic/user ./internal/auth/logic -run 'Avatar|Auth'
```

Expected before implementation: FAIL/compile failure because `avatar_url` is not modeled or persisted.

### Task 2: Frontend Regression Tests

**Files:**
- Modify: `web/src/App.test.tsx`
- Add: `web/src/auth/session.test.ts`

- [ ] **Step 1: Write the failing refresh test**

Add an App test that uploads an avatar, verifies the session was updated with the returned durable avatar URL, remounts the app from persisted session, and verifies the Me page still renders the avatar image without a manual profile edit.

- [ ] **Step 2: Write the failing login/session hydration test**

Add tests proving auth login response `avatar_url` is stored in `AuthSession` and `readStoredSession` preserves avatar fields.

- [ ] **Step 3: Run targeted frontend tests to verify RED**

Run:

```bash
npm --prefix web run test:run -- --reporter=dot src/App.test.tsx src/auth
```

Expected before implementation: FAIL because `AuthUser` and `userProfileFromAuth` do not carry avatar fields and avatar upload does not persist updated profile into the auth session.

### Task 3: Backend Implementation

**Files:**
- Add: `db/migrations/007_profile_avatar_url.sql`
- Add: `db/change_log/2026-05-06-profile-avatar-url.sql`
- Modify: `internal/model/user.go`
- Modify: `internal/repository/repository.go`
- Modify: `internal/repository/postgres_user_friends.go`
- Modify: `internal/repository/memory.go`
- Modify: `internal/logic/userlogic.go`
- Modify: `internal/logic/user/gozero_logic.go`
- Modify: `internal/logic/friends/gozero_logic.go`
- Modify: `internal/auth/useradapter/user_client.go`
- Modify: `internal/auth/logic/authlogic.go`
- Modify: `internal/auth/logic/auth/gozero_logic.go`
- Modify: `api/auth.api`
- Modify: `internal/types/types.go`

- [ ] **Step 1: Add profile-owned durable avatar URL storage**

Add `profiles.avatar_url text not null default ''` in the incremental SQL migration, then wire the column through model structs, repository rows, memory repository, and PostgreSQL insert/select/update paths. Leave the already-published `001_init_postgres.sql` unchanged to avoid migration checksum mismatch on existing databases.

- [ ] **Step 2: Return persisted avatar URLs from profile surfaces**

Map `AvatarURL` through `UserProfile`, `/me`, `/users/:identifier`, `/friends`, and auth login/register responses. Stop generating avatar presigned URLs in profile read responses.

- [ ] **Step 3: Preserve validation**

Keep `PATCH /me/avatar` owner/status/purpose/content-type/size validation through `MediaLogic.ValidateAvatarMedia`, then set `avatar_media_id` and `avatar_url` together.

### Task 4: Stable Avatar Media URL

**Files:**
- Modify: `api/media.api`
- Add: `internal/handler/media/get_avatar_handler.go`
- Modify: `internal/handler/gozero_routes.go`
- Modify: `deploy/k8s/ingress.yaml`
- Modify: `docs/product-specs/frontend-backend-contract.md`
- Modify: `docs/design-docs/database-schema-v2.md`

- [ ] **Step 1: Add stable route**

Add unauthenticated `GET /media/avatars/:media_id` on user-api. The handler parses `media_id`, calls media logic to validate avatar media and mint a fresh object-store display URL, sets `Cache-Control: no-store`, and redirects to that URL.

- [ ] **Step 2: Route production traffic**

Add `/media` ingress routing to user-api so stable avatar URLs work in production alongside local Vite proxy routing.

- [ ] **Step 3: Update docs**

Document `profiles.avatar_url` as profile-owned display data and clarify that persisted `avatar_url` is durable while short-lived presigned URLs are only generated behind the stable media route.

### Task 5: Frontend Implementation

**Files:**
- Modify: `web/src/auth/session.ts`
- Modify: `web/src/auth/AuthContext.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Extend session model**

Add `avatarMediaId` and `avatarUrl` to `AuthUser`, preserve them in storage reads, and map auth response `avatar_media_id/avatar_url` into session state.

- [ ] **Step 2: Persist profile updates into auth session**

Expose an AuthContext helper to update session user display/profile/avatar fields after `PATCH /me` and `PATCH /me/avatar`, then use it in `AuthenticatedApp`.

- [ ] **Step 3: Hydrate current user from session avatar fields**

Map `AuthUser.avatarMediaId/avatarUrl` into `userProfileFromAuth` so refresh/relogin starts with a non-empty avatar when the session contains one.

### Task 6: Verification And Handoff

**Files:**
- Commit all changed files.

- [ ] **Step 1: Run required verification**

Run the required backend, frontend, static, and diff checks from the task. For DB/repository SQL changes, run PostgreSQL integration if a disposable PostgreSQL DSN is available; otherwise record the blocker explicitly.

- [ ] **Step 2: Commit, push, PR, issue comment**

Commit with `fix(avatar): persist display url across refresh`, push `feature/issue-4-avatar-url-persistence`, open/update a PR to `develop`, and comment on Issue #4 with root cause, fix summary, tests, branch/commit/PR, and blockers.
