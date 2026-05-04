# Single Device Presence Heartbeat Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce one active login/session and one active WebSocket connection per account while hardening presence, heartbeat, and ACK handling.

**Architecture:** Auth issues JWTs with a session id and persists the active session in the auth credential store. Auth validation, protected REST middleware, and Gateway WebSocket handshakes/commands validate the token session against that shared store. Gateway registration replaces older same-user connections, presence remains backed by `PresenceStore`, and the frontend WebSocket wrapper sends heartbeat commands with ACK timeouts.

**Tech Stack:** Go, go-zero REST, gorilla/websocket, PostgreSQL/memory repositories, React/Vite, Vitest.

**Limitation:** Cross-service active-session enforcement requires shared auth storage. Production/k3s PostgreSQL mode is enforced across services; standalone multi-process `StorageDriver: memory` logs that shared validation is disabled and can only enforce active sessions inside the auth service or tests that inject the same memory repository.

---

### Task 1: Active Auth Sessions

**Files:**
- Modify: `internal/auth/token/token.go`
- Modify: `internal/auth/model/credential.go`
- Modify: `internal/auth/repository/repository.go`
- Modify: `internal/auth/repository/memory.go`
- Modify: `internal/auth/repository/postgres.go`
- Modify: `internal/auth/logic/authlogic.go`
- Test: `tests/auth_service_test.go`

- [ ] **Step 1: Write failing test:** login twice for one identifier; the first login token must fail `ValidateToken`, and the second token must pass.
- [ ] **Step 2: Run targeted auth test:** `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./tests -run TestAuthLogicLoginReplacesActiveSession -count=1`; expect failure before implementation.
- [ ] **Step 3: Implement session id issuance and active-session persistence:** add `sid` to token claims, store active session on register/login, and reject non-active sessions in `ValidateToken`.
- [ ] **Step 4: Re-run targeted auth test:** expect pass.

### Task 2: Gateway Session And Presence Enforcement

**Files:**
- Modify: `internal/gateway/ws/server.go`
- Modify: `internal/gateway/ws/connection_manager.go`
- Modify: `cmd/gateway-ws/main.go`
- Test: `internal/gateway/ws/server_test.go`
- Test: `internal/presence/memory_test.go`

- [ ] **Step 1: Write failing WebSocket tests:** a later same-user connection closes the older connection, and heartbeat timeout unregisters presence.
- [ ] **Step 2: Run targeted gateway/presence tests:** `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./internal/gateway/ws ./internal/presence -run 'TestWebSocketLaterConnectionReplacesExistingUserConnection|TestWebSocketHeartbeatTimeoutUnregistersPresence|TestMemoryStoreIsUserOnlineExpiresAfterTTL' -count=1`; expect the replacement test to fail before implementation.
- [ ] **Step 3: Implement minimal gateway changes:** close replaced connections with an application close code, validate active sessions on handshake/commands when configured, and expose internal online lookup from the gateway presence store.
- [ ] **Step 4: Re-run targeted gateway/presence tests:** expect pass.

### Task 3: Protected REST Session Check

**Files:**
- Modify: `internal/svc/service_context.go`
- Modify: `internal/handler/gozero_routes.go`
- Modify: `cmd/user-api/main.go`
- Modify: `cmd/friends-api/main.go`
- Modify: `cmd/groups-api/main.go`
- Modify: `cmd/message-api/main.go`

- [ ] **Step 1: Wrap protected routes after JWT auth:** parse bearer token through the shared auth session validator and reject stale sessions with `UNAUTHENTICATED`.
- [ ] **Step 2: Keep memory-mode tests explicit:** only shared memory repos can enforce this inside one process; multi-process memory mode is not a shared session store.

### Task 4: Frontend Heartbeat ACK

**Files:**
- Modify: `web/src/api/websocketClient.ts`
- Test: `web/src/api/websocketClient.test.ts`

- [ ] **Step 1: Write failing Vitest:** fake timers prove periodic heartbeat frames include `requestId`, ACK clears the timeout, and missing ACK closes the socket.
- [ ] **Step 2: Run targeted Vitest:** `npm --prefix web run test:run -- web/src/api/websocketClient.test.ts --reporter=dot`; expect failure before implementation.
- [ ] **Step 3: Implement minimal heartbeat timer logic:** configurable interval/ACK timeout/request id factory with cleanup on close.
- [ ] **Step 4: Re-run targeted Vitest:** expect pass.

### Task 5: Verification And Commit

**Files:**
- Verify all modified files.

- [ ] **Step 1: Run required backend tests:** `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./internal/auth/... ./internal/gateway/ws ./internal/presence ./tests`.
- [ ] **Step 2: Run required frontend tests/build:** `npm --prefix web run test:run -- --reporter=dot` and `npm --prefix web run build`.
- [ ] **Step 3: Run static checks:** `bash scripts/verify-static.sh` and `git diff --check`.
- [ ] **Step 4: Commit:** `git commit -m "fix(auth): enforce single active device sessions"`.
