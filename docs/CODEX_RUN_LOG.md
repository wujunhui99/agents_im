# Codex run log

This file tracks autonomous Codex runs that are relevant to repository work. Record completed runs here after controller-side verification; do not store secrets, tokens, passwords, cookies, DSNs, or raw logs containing credentials.

## What to record for each run

- **Task / branch / worktree**: enough to find the work later.
- **Start time / end time / duration**: include timezone when known.
- **Base commit / final commit**: record commit hashes; note if no commit was made.
- **Outcome**: completed, failed, killed, superseded, or abandoned.
- **Codex token usage**: copy exactly as printed by Codex. If Codex only prints total tokens, record only total.
- **Verification**: commands and pass/fail results. Distinguish Codex self-report from controller-side verification.
- **Files changed / summary**: concise summary, not full diff.
- **Follow-ups / blockers**: anything the next controller or Codex should know.

## Runs

### 2026-05-03 — ws-local-regression-pass

- **Task**: make local WebSocket live-push regression pass.
- **Branch**: `fix/ws-local-regression-pass`.
- **Worktree**: `/home/ws/project/worktrees/ws-local-regression-pass`.
- **Controller session**: Hermes Telegram session.
- **Codex session**: `proc_c5eaeabec73c`.
- **Started**: 2026-05-03, exact local timestamp not recorded at launch.
- **Ended**: 2026-05-03T23:30:10+08:00 approximate, from Codex completion notification timestamp.
- **Duration**: approximately 21 minutes from controller-observed process start.
- **Base commit**: `7d83678 docs(ws): document production websocket pitfalls`.
- **Final commit**: `76c1ad3 fix local websocket live push regression`.
- **Outcome**: Codex exited 0 and committed changes, but controller verification failed; do not treat as accepted.
- **Token usage**: `tokens used 290,195`.
- **Codex completion warning**: `failed to record rollout items: thread ... not found`; treated as non-blocking only for process completion, not for acceptance.
- **Controller verification**:
  - `NODE_PATH=/tmp/ws-e2e-run/node_modules AGENTS_IM_E2E_TARGET=local AGENTS_IM_E2E_BASE_URL=http://127.0.0.1:5173 node tests/e2e/ws_live_push_regression.mjs` before frontend restart failed with `classification: setup-or-harness-failed` because frontend was not listening on `:5173`.
  - `make frontend-start` succeeded.
  - Same regression command after frontend start failed with `classification: live-push-still-fails`: B WebSocket reached 101 and A send returned 200; B history contained the message, but B received no matching `message_received` frame and no-refresh UI did not update.
- **Files changed by final commit**:
  - `.env.example`, `Makefile`, `cmd/message-transfer/main.go`, `db/migrations/001_init_postgres.sql`, `docs/DEVELOPMENT.md`, `etc/message-transfer.yaml`, `internal/config/config.go`, `internal/config/config_test.go`, `internal/transfer/kafka_consumer.go`, `internal/transfer/outbox_consumer.go`, `internal/transfer/outbox_consumer_test.go`, `scripts/dev-up.sh`, `scripts/verify-static.sh`, `tests/e2e/README.md`, `tests/e2e/ws_live_push_regression.mjs`, `web/vite.config.test.ts`, `web/vite.config.ts`.
- **Current assessment**:
  - Codex did not satisfy the user acceptance criterion: final local regression did not reach `classification: live-push-success` under controller-side verification.
  - The branch may still contain useful fixes, but it needs another iteration or manual correction before merge.
- **Follow-ups**:
  - Continue from `fix/ws-local-regression-pass` or spawn a corrective Codex with the controller failure evidence above.
  - Check whether local `message-transfer` is actually consuming outbox events and dispatching to gateway; current failure indicates handshake and REST send/history work, but live push fanout still does not deliver a matching frame.

### 2026-05-03 — controller correction for ws-local-regression-pass

- **Task**: controller-side correction after Codex `76c1ad3` failed verification.
- **Branch**: `fix/ws-local-regression-pass`.
- **Worktree**: `/home/ws/project/worktrees/ws-local-regression-pass`.
- **Controller session**: Hermes Telegram session; no new Codex run.
- **Started**: 2026-05-03T23:50+08:00 approximate.
- **Ended**: 2026-05-03T23:55+08:00 approximate.
- **Duration**: approximately 5 minutes.
- **Base commit**: `76c1ad3 fix local websocket live push regression`.
- **Final commit**: pending at time of this log entry.
- **Outcome**: local controller verification passed. Final acceptance used the requested frontend URL `http://127.0.0.1:5173` with isolated backend ports because unrelated main-worktree services occupied canonical 808x ports.
- **Codex token usage**: not applicable; no Codex process was spawned.
- **Root cause evidence**:
  - Earlier E2E classification counted the Vite HMR WebSocket as a successful business WebSocket, while the real business `/ws` connection could fail separately.
  - `message_outbox` stores status/event-type fields as `smallint`; the PostgreSQL outbox repository now casts enum bind parameters explicitly.
  - Local non-default frontend ports need matching gateway `AllowedOrigins`; `scripts/dev-up.sh` now derives default origins from `FRONTEND_PORT`.
- **Verification**:
  - `NODE_PATH=/tmp/ws-e2e-run/node_modules AGENTS_IM_E2E_TARGET=local AGENTS_IM_E2E_BASE_URL=http://127.0.0.1:5173 node tests/e2e/ws_live_push_regression.mjs` passed with `classification: live-push-success`; evidence: `/tmp/agents-im-ws-live-push-e2e/2026-05-03T15-56-02-238Z`.
  - Isolated-port diagnostic run also passed at `http://127.0.0.1:15173`; evidence: `/tmp/agents-im-ws-live-push-e2e/2026-05-03T15-54-38-293Z`.
  - `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./internal/repository ./internal/transfer ./tests -run 'TestOutbox|TestWebSocket|TestVite|TestMessage|TestPostgresMessageOutbox' -count=1` passed.
  - `git diff --check` passed.
  - `bash scripts/verify-static.sh` passed.
- **Files changed / summary**:
  - `tests/e2e/ws_live_push_regression.mjs`: classify and summarize only the business `/ws` WebSocket, excluding Vite HMR frames.
  - `scripts/dev-up.sh`: derive default gateway allowed origins from `FRONTEND_PORT`.
  - `Makefile`: add `--strictPort` for frontend startup to fail visibly instead of silently moving to a different port.
  - `internal/repository/postgres_outbox.go`: cast outbox enum bind parameters to `smallint`.
- **Follow-ups / blockers**:
  - Canonical 808x backend ports are still occupied by unrelated main-worktree services owned by another user/session; this branch was verified with isolated 1818x backend ports behind the requested `http://127.0.0.1:5173` frontend.

