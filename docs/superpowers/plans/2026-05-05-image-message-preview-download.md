# Image Message Preview Download Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete Issue #1 image-message loop across composer upload, send, live/history rendering, preview, download, and participant authorization.

**Architecture:** Keep message sending on the existing Media API + Message API path: upload intent, presigned PUT, complete upload, then `POST /messages` with `contentType=image`. Frontend image bubbles resolve short-lived media download URLs through `MediaApi.getDownloadURL`; backend keeps object storage private while allowing the uploader or a visible conversation participant to obtain a presigned download URL.

**Tech Stack:** React, TypeScript, Vite, Vitest + Testing Library, Go/go-zero, existing repository abstractions, Node/Playwright-style E2E harnesses.

---

### Task 1: Frontend Red Tests

**Files:**
- Modify: `web/src/features/messages/MessagesPage.test.tsx`
- Modify: `web/src/api/media.ts`

- [ ] Add tests proving `contentType=image` history/live messages render an image bubble, not raw JSON.
- [ ] Add tests proving unsupported image MIME and `>15 MiB` images are rejected before upload intent.
- [ ] Add tests proving image upload calls intent -> presigned PUT -> complete -> message send in order.
- [ ] Add tests proving image preview opens/closes.
- [ ] Add tests proving download fetches authorized URL and calls an injected download handler.

Run:

```bash
npm --prefix web test -- --run web/src/features/messages/MessagesPage.test.tsx
```

Expected before implementation: FAIL because `getDownloadURL`, image preview, and image card rendering are missing.

### Task 2: Backend Red Tests

**Files:**
- Modify: `internal/logic/medialogic_test.go`

- [ ] Add a failing test showing a receiver who can see a message containing `mediaId` can call media download URL logic.
- [ ] Add a failing test or assertion showing an unrelated user is still denied.

Run:

```bash
go test ./internal/logic -run 'TestMessageParticipantCanGetMediaDownloadURL|TestMediaCompleteAndDownloadRequireOwnerAndObjectStat' -count=1
```

Expected before implementation: FAIL because download URL authorization is owner-only.

### Task 3: Frontend Implementation

**Files:**
- Modify: `web/src/api/media.ts`
- Modify: `web/src/features/messages/MessagesPage.tsx`
- Modify: `web/src/components/ui/MessageBubble.tsx` only if text bubble structure needs a safe non-text variant
- Modify: `web/src/styles.css`
- Modify: `web/vite.config.ts`

- [ ] Add `MediaApi.getDownloadURL(mediaId)`.
- [ ] Add `/media` Vite proxy to `user-api`.
- [ ] Reject unsupported image MIME before `createUploadIntent`.
- [ ] Render image messages through a dedicated image card using authorized download URLs.
- [ ] Show Chinese copy for thumbnail, preview, download, upload, and send failures.
- [ ] Add lightbox preview with close control.
- [ ] Add download button using an injected handler for tests and a default anchor-click handler in production.

Run:

```bash
npm --prefix web test -- --run web/src/features/messages/MessagesPage.test.tsx
```

Expected after implementation: PASS.

### Task 4: Backend Implementation

**Files:**
- Modify: `internal/repository/message_repository.go`
- Modify: `internal/repository/message_memory.go`
- Modify: `internal/repository/postgres_message.go`
- Modify: `internal/logic/medialogic.go`
- Modify: `internal/svc/service_context.go`
- Modify: `cmd/user-api/main.go`

- [ ] Add a repository capability to determine whether `user_id` has visible image/file message access to `mediaId`.
- [ ] Let `MediaLogic.GetDownloadURL` allow owner access or participant access for ready message media.
- [ ] Keep avatar and non-message media owner-only.
- [ ] Build a message repository in `user-api` so `/media/:media_id/download-url` can authorize received message attachments from the same PostgreSQL source of truth.

Run:

```bash
go test ./internal/logic ./internal/repository ./internal/handler -count=1
```

Expected after implementation: PASS.

### Task 5: E2E Harness

**Files:**
- Create: `tests/e2e/image_message_regression.mjs`
- Modify: `tests/e2e/README.md`

- [ ] Add a production/local-capable harness that registers A/B, establishes friendship, sends one image from A, checks sender UI, receiver no-refresh UI, history replay, preview, and download URL behavior.
- [ ] Classify `image-message-success`, `image-upload-validation-failed`, `image-preview-download-failed`, and setup failures.
- [ ] Redact bearer tokens, passwords, cookies, JWTs, and presigned URL query strings in artifacts and console output.

Run when services and Playwright are available:

```bash
AGENTS_IM_E2E_TARGET=local \
AGENTS_IM_E2E_BASE_URL=http://127.0.0.1:5173 \
NODE_PATH=/tmp/ws-e2e-run/node_modules \
node tests/e2e/image_message_regression.mjs
```

### Task 6: Final Verification And Delivery

Run the required verification commands:

```bash
npm --prefix web test -- --run
npm --prefix web run build
go test ./...
bash scripts/verify-static.sh
git diff --check
```

Then commit with:

```bash
git commit -m "feat(media): complete image message preview and download"
```

After commit, comment on Issue #1 with implementation summary, tests run/results, commit SHA, and known risks/blockers.
