# E2E regression harnesses

This directory contains permanent regression harnesses that can run against production or a local dev stack. Generated evidence belongs under `/tmp/...` and must never be committed.

## Playwright setup

Playwright is intentionally not added as a repo dependency. If it is not already available, install it outside the checkout and expose it with `NODE_PATH`:

```bash
mkdir -p /tmp/ws-e2e-run
npm --prefix /tmp/ws-e2e-run install playwright
NODE_PATH=/tmp/ws-e2e-run/node_modules npx --prefix /tmp/ws-e2e-run playwright install chromium
```

Run scripts from the repo root. Local mode assumes the relevant local backend services and, for browser harnesses, the Vite frontend are already running; scripts do not start services.

For local Vite mode, `http://127.0.0.1:5173` should be the Vite server for the current checkout. As a proxy sanity check, `POST /messages` through Vite without a token should return a `401` JSON envelope from `message-api`; a plain `404 page not found` response means the harness will fail before reaching the message regression.

When default backend ports are occupied by another worktree, start the local backend on alternate ports with `scripts/dev-up.sh --services-only` and start Vite with the same `USER_API_PORT`, `AUTH_API_PORT`, `FRIENDS_API_PORT`, `MESSAGE_API_PORT`, `GATEWAY_WS_PORT`, and `GROUPS_API_PORT` environment variables. The local transfer worker reads the Postgres outbox directly and dispatches to `gateway-ws`, so the WebSocket regressions do not require a separate Kafka publisher process.

## WebSocket live-push regression

`ws_live_push_regression.mjs` covers the case where B WebSocket reaches `101`, A sends B a unique message with `200 OK`, B can pull the message from history, but B receives no matching `message_received` frame and the no-refresh UI does not update.

Production:

```bash
AGENTS_IM_E2E_TARGET=production \
AGENTS_IM_E2E_BASE_URL=https://agenticim.xyz \
NODE_PATH=/tmp/ws-e2e-run/node_modules \
node tests/e2e/ws_live_push_regression.mjs
```

Local:

```bash
AGENTS_IM_E2E_TARGET=local \
AGENTS_IM_E2E_BASE_URL=http://127.0.0.1:5173 \
NODE_PATH=/tmp/ws-e2e-run/node_modules \
node tests/e2e/ws_live_push_regression.mjs
```

Evidence mode allows known live-push regression classifications to exit zero while setup failures still fail:

```bash
AGENTS_IM_E2E_TARGET=production \
AGENTS_IM_E2E_BASE_URL=https://agenticim.xyz \
AGENTS_IM_E2E_ALLOW_REPRO_FAILURE=1 \
NODE_PATH=/tmp/ws-e2e-run/node_modules \
node tests/e2e/ws_live_push_regression.mjs
```

Artifacts default to `/tmp/agents-im-ws-live-push-e2e/<timestamp>/` and can be overridden with `AGENTS_IM_E2E_OUTPUT_DIR`.

## Bidirectional no-refresh send regression

`ws_bidirectional_send_regression.mjs` covers the follow-up regression where B receives A's live message in an existing single conversation, stays on that same chat page, and then B's reply POSTs `/messages` with the wrong target and receives `400 Bad Request`.

The harness creates fresh QA accounts, establishes friendship through real APIs, seeds an existing A/B conversation, opens both A and B browser sessions on that conversation, sends a unique A->B message, verifies B sees it without refresh, then sends B->A from B's unchanged chat window.

Production:

```bash
AGENTS_IM_E2E_TARGET=production \
AGENTS_IM_E2E_BASE_URL=https://agenticim.xyz \
NODE_PATH=/tmp/ws-e2e-run/node_modules \
node tests/e2e/ws_bidirectional_send_regression.mjs
```

Local:

```bash
AGENTS_IM_E2E_TARGET=local \
AGENTS_IM_E2E_BASE_URL=http://127.0.0.1:5173 \
NODE_PATH=/tmp/ws-e2e-run/node_modules \
node tests/e2e/ws_bidirectional_send_regression.mjs
```

Classifications:

- `bidirectional-send-success`
- `reverse-send-bad-request`
- `reverse-send-ui-disabled-or-missing-target`
- `setup-or-harness-failed`

Artifacts default to `/tmp/agents-im-ws-bidirectional-send-e2e/<timestamp>/`.

## Image message preview/download regression

`image_message_regression.mjs` covers the full image-message loop. The harness creates fresh QA accounts, establishes friendship, seeds a conversation, opens A and B browser sessions, verifies unsupported and oversized image validation before upload, sends a real image through the UI upload flow, checks A's sent bubble, checks B's no-refresh live bubble, reloads B to verify history replay, opens/closes preview, and verifies the receiver can fetch the authorized media download URL and the presigned object bytes.

Production:

```bash
AGENTS_IM_E2E_TARGET=production \
AGENTS_IM_E2E_BASE_URL=https://agenticim.xyz \
NODE_PATH=/tmp/ws-e2e-run/node_modules \
node tests/e2e/image_message_regression.mjs
```

Local:

```bash
AGENTS_IM_E2E_TARGET=local \
AGENTS_IM_E2E_BASE_URL=http://127.0.0.1:5173 \
NODE_PATH=/tmp/ws-e2e-run/node_modules \
node tests/e2e/image_message_regression.mjs
```

Classifications:

- `image-message-success`
- `image-upload-validation-failed`
- `image-preview-download-failed`
- `setup-or-harness-failed`

Artifacts default to `/tmp/agents-im-image-message-e2e/<timestamp>/`. The harness redacts bearer tokens, JWT-like values, cookies, passwords, and presigned URL query strings from console output and artifacts.

## Auth register-login regression

`auth_register_login_regression.mjs` is an API-only regression harness for the auth credential persistence path. It creates a fresh unique account through the real `/auth/register`, then logs in through the real `/auth/login` with the exact same identifier/password.

Production:

```bash
AGENTS_IM_E2E_TARGET=production \
AGENTS_IM_E2E_BASE_URL=https://agenticim.xyz \
node tests/e2e/auth_register_login_regression.mjs
```

Local:

```bash
AGENTS_IM_E2E_TARGET=local \
AGENTS_IM_E2E_BASE_URL=http://127.0.0.1:5173 \
node tests/e2e/auth_register_login_regression.mjs
```

Evidence mode allows the known `login-invalid-after-register` classification to exit zero while setup/register failures still fail:

```bash
AGENTS_IM_E2E_TARGET=production \
AGENTS_IM_E2E_BASE_URL=https://agenticim.xyz \
AGENTS_IM_E2E_ALLOW_REPRO_FAILURE=1 \
node tests/e2e/auth_register_login_regression.mjs
```

Classifications:

- `register-login-success`
- `login-invalid-after-register`
- `register-failed`
- `setup-or-harness-failed`

The script also accepts `AGENTS_IM_E2E_API_BASE_URL`, `AGENTS_IM_E2E_OUTPUT_DIR`, and `AGENTS_IM_E2E_REQUEST_TIMEOUT_MS`. Artifacts default to `/tmp/agents-im-auth-register-login-e2e/<timestamp>/`.

Do not commit generated evidence, screenshots, secrets, real passwords, JWTs, cookies, or account credentials.
