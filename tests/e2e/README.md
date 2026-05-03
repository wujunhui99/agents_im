# WebSocket live-push E2E regression

`ws_live_push_regression.mjs` is a reusable browser E2E harness for the WebSocket live-push regression:

```text
B WebSocket reaches 101, A sends B a unique message with 200 OK, B can pull the message from history, but B receives no matching message_received frame and the no-refresh UI does not update.
```

The script creates fresh QA accounts, establishes friendship, loads the real frontend with the real session storage key (`agents_im.auth.v1`), captures B-side browser/CDP WebSocket evidence, sends a unique A->B text message through `/messages`, pulls B history using `single:<lowerUserID>:<higherUserID>`, and writes redacted artifacts.

## Playwright setup

Playwright is intentionally not added as a repo dependency. If it is not already available, install it outside the checkout and expose it with `NODE_PATH`:

```bash
mkdir -p /tmp/ws-e2e-run
npm --prefix /tmp/ws-e2e-run install playwright
NODE_PATH=/tmp/ws-e2e-run/node_modules npx --prefix /tmp/ws-e2e-run playwright install chromium
```

Run the script from the repo root. Local mode assumes all local services and the Vite frontend are already running; the script never starts services.

For local mode, `http://127.0.0.1:5173` must be the Vite server for the current checkout. As a proxy sanity check, `POST /messages` through Vite without a token should return a `401` JSON envelope from `message-api`; a plain `404 page not found` response means the harness will fail before reaching the live-push regression.

When default backend ports are occupied by another worktree, start the local backend on alternate ports with `scripts/dev-up.sh --services-only` and start Vite with the same `USER_API_PORT`, `AUTH_API_PORT`, `FRIENDS_API_PORT`, `MESSAGE_API_PORT`, `GATEWAY_WS_PORT`, and `GROUPS_API_PORT` environment variables. The local transfer worker reads the Postgres outbox directly and dispatches to `gateway-ws`, so this regression does not require a separate Kafka publisher process.

## Production

```bash
AGENTS_IM_E2E_TARGET=production \
AGENTS_IM_E2E_BASE_URL=https://agenticim.xyz \
NODE_PATH=/tmp/ws-e2e-run/node_modules \
node tests/e2e/ws_live_push_regression.mjs
```

## Local

```bash
AGENTS_IM_E2E_TARGET=local \
AGENTS_IM_E2E_BASE_URL=http://127.0.0.1:5173 \
NODE_PATH=/tmp/ws-e2e-run/node_modules \
node tests/e2e/ws_live_push_regression.mjs
```

## Evidence mode

The harness exits `0` only for `live-push-success`. To collect evidence while the known production failure still exists, keep the failure classification but allow a zero exit for regression classifications. Setup or harness failures still exit non-zero.

```bash
AGENTS_IM_E2E_TARGET=production \
AGENTS_IM_E2E_BASE_URL=https://agenticim.xyz \
AGENTS_IM_E2E_ALLOW_REPRO_FAILURE=1 \
NODE_PATH=/tmp/ws-e2e-run/node_modules \
node tests/e2e/ws_live_push_regression.mjs
```

Artifacts default to `/tmp/agents-im-ws-live-push-e2e/<timestamp>/` and can be overridden with `AGENTS_IM_E2E_OUTPUT_DIR`. The artifact set includes `report.txt`, `observations.redacted.json`, `ws-events.redacted.json`, `console.redacted.json`, and screenshots when available.

Do not commit generated evidence, screenshots, secrets, real passwords, JWTs, cookies, or account credentials.

# Auth register-login E2E regression

`auth_register_login_regression.mjs` is an API-only regression harness for the auth credential persistence path. It creates a fresh unique account through the real `/auth/register`, then logs in through the real `/auth/login` with the exact same identifier/password. Artifacts are redacted and written to `/tmp/agents-im-auth-register-login-e2e/<timestamp>/` by default.

Classifications:

- `register-login-success`
- `login-invalid-after-register`
- `register-failed`
- `setup-or-harness-failed`

## Auth production

```bash
AGENTS_IM_E2E_TARGET=production \
AGENTS_IM_E2E_BASE_URL=https://agenticim.xyz \
node tests/e2e/auth_register_login_regression.mjs
```

## Auth local

```bash
AGENTS_IM_E2E_TARGET=local \
AGENTS_IM_E2E_BASE_URL=http://127.0.0.1:5173 \
node tests/e2e/auth_register_login_regression.mjs
```

## Auth evidence mode

Use evidence mode only when collecting proof of the known register-then-login regression. It allows a zero exit for `login-invalid-after-register`; setup and register failures still exit non-zero.

```bash
AGENTS_IM_E2E_TARGET=production \
AGENTS_IM_E2E_BASE_URL=https://agenticim.xyz \
AGENTS_IM_E2E_ALLOW_REPRO_FAILURE=1 \
node tests/e2e/auth_register_login_regression.mjs
```

The script also accepts `AGENTS_IM_E2E_API_BASE_URL`, `AGENTS_IM_E2E_OUTPUT_DIR`, and `AGENTS_IM_E2E_REQUEST_TIMEOUT_MS`. Do not commit generated evidence, secrets, real passwords, JWTs, cookies, or account credentials.
