# Development

This document covers local backend startup for frontend integration and MVP smoke testing.

## Prerequisites

- Go toolchain.
- Docker Compose.
- `goctl` on `PATH`; the required task environment is:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
```

Do not put real passwords or tokens in committed files. Use `.env` for local overrides and keep `.env` untracked.

## Local Backend

Start local middleware, run PostgreSQL migrations, build the host binaries, and launch the REST APIs plus WebSocket gateway:

```bash
scripts/dev-up.sh
```

Start only Docker middleware and migrations:

```bash
scripts/dev-up.sh --middleware-only
```

Stop host services started by the script:

```bash
scripts/dev-up.sh --stop
```

The script writes generated local config, binaries, logs, and PID files under `.dev/`, which is ignored by git.

## Local Makefile Shortcuts

The repository also provides a `Makefile` for common local lifecycle commands:

```bash
make start             # start backend services and frontend dev server
make stop              # stop frontend dev server and backend host services
make restart           # restart both frontend and backend
make backend-start     # start Docker middleware, migrations, and backend APIs/WebSocket gateway
make backend-stop      # stop backend host services started by scripts/dev-up.sh
make backend-restart   # restart backend services
make frontend-start    # start Vite dev server at http://127.0.0.1:5173
make frontend-stop     # stop the Vite dev server started by Makefile
make frontend-restart  # restart frontend dev server
make status            # show PID files and ports 8080-8086/5173
make test              # run frontend tests/build/lint plus Go tests
make verify            # run test plus static checks, docker compose config, and git diff check
```

- `make frontend-start` writes logs and PID files under `.dev/logs/frontend.log` and `.dev/pids/frontend.pid`. Override the frontend bind address or port when needed:
- `make test` runs frontend tests/build/lint plus Go tests while excluding accidental Go packages under `web/node_modules`.

```bash
make frontend-start FRONTEND_HOST=127.0.0.1 FRONTEND_PORT=5174
```


## Services-only Restart And Alternate Ports

When Docker middleware is already running, or when middleware is managed externally, restart only the host Go services:

```bash
scripts/dev-up.sh --services-only
```

This mode skips Docker middleware startup and PostgreSQL migrations, rebuilds host binaries, and restarts only the REST APIs plus WebSocket gateway. It is useful for local E2E debugging when Postgres/Redis are already available.

Each service port can be overridden for single-machine debugging, especially when default ports are occupied by stale/root-owned processes:

```bash
USER_API_PORT=18080 \
AUTH_API_PORT=18081 \
FRIENDS_API_PORT=18082 \
MESSAGE_API_PORT=18083 \
GATEWAY_WS_PORT=18084 \
GROUPS_API_PORT=18085 \
AGENT_API_PORT=18086 \
MESSAGE_TRANSFER_OBSERVABILITY_PORT=18087 \
AGENTS_IM_DEV_STATE_DIR=/tmp/agents-im-dev-e2e \
PATH=/tmp/go/bin:$HOME/go/bin:$PATH \
scripts/dev-up.sh --services-only
```

Use a separate `AGENTS_IM_DEV_STATE_DIR` when running alternate services so logs, PID files, configs, and binaries do not conflict with the default `.dev/` directory.
Start the frontend with the same port overrides so Vite proxies to that alternate backend stack:

```bash
USER_API_PORT=18080 \
AUTH_API_PORT=18081 \
FRIENDS_API_PORT=18082 \
MESSAGE_API_PORT=18083 \
GATEWAY_WS_PORT=18084 \
GROUPS_API_PORT=18085 \
make frontend-start FRONTEND_HOST=127.0.0.1 FRONTEND_PORT=5173
```

Local `message-transfer` uses the Postgres outbox consumer by default and dispatches to `gateway-ws`, so HTTP `POST /messages` can produce live WebSocket pushes without requiring a separate Kafka publisher process.

## Single-machine E2E Smoke Command

A fast in-process smoke command exists for the core single-machine flow when the external dev environment is unavailable or polluted by stale processes:

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go run ./test/e2e/single-machine
```

It uses real business logic and a real WebSocket gateway test server in one process to validate:

1. register Alice;
2. register Bob;
3. add Bob as Alice's friend;
4. send a single-chat message through message logic;
5. pull the message as Bob;
6. connect Alice and Bob to WebSocket;
7. send `send_message` over WebSocket;
8. assert Alice receives ACK and Bob receives live `message_received` push.

This is a smoke check, not a replacement for full local runtime E2E with Docker middleware and bound HTTP ports. Full E2E should still use `make start`, real REST APIs, WebSocket gateway, PostgreSQL, Redis, and MinIO when the environment is clean.

### Local E2E Debug Notes

Recent single-machine E2E hardening work was consolidated into long-lived docs instead of keeping temporary fix notes under `docs/local-e2e*` or `docs/ws-live-delivery*`. The old temporary notes were removed after their durable content was merged here and into `docs/design-docs/websocket-gateway.md`.

The related git commits are:

```text
bbba8fe Document local e2e commit
a1a0b93 Add single-machine e2e fallback
ae6922d Wire websocket send to live delivery
e92e484 Fix dev demo cleanup trap
```

Environment note from the debug session: on one local machine, default ports `8080-8085` were occupied by root-owned `.dev/bin/*` processes. Without elevated permissions, the current user could not stop or replace those processes. Use alternate ports with `AGENTS_IM_DEV_STATE_DIR` as shown above, or clean the root-owned processes externally before using default ports.

## Ports

| Service | URL |
| --- | --- |
| Account API (V0 `user-api`) | `http://127.0.0.1:8080` |
| Auth API | `http://127.0.0.1:8081` |
| Friends API | `http://127.0.0.1:8082` |
| Message API | `http://127.0.0.1:8083` |
| WebSocket Gateway | `ws://127.0.0.1:8084/ws` |
| Groups API | `http://127.0.0.1:8085` |
| Agent API | `http://127.0.0.1:8086` |
| PostgreSQL | `localhost:5432` |
| Redis | `localhost:6379` |
| MinIO API | `http://localhost:9000` |
| MinIO Console | `http://localhost:9001` |

`scripts/dev-up.sh` uses PostgreSQL storage so the separate local API processes share account profiles (V0 `users` table), credentials, friendships, groups, Agent profiles, media metadata, and message history. It also starts MinIO for local object storage and writes `ObjectStorage` config into the generated `user-api` config. Agent creation verifies `account_type=agent` through the Account Service profile repository; unavailable verification fails closed.

## Local Object Storage

Local media uploads use MinIO/S3-compatible object storage. Development defaults are in `.env.example` using local-only placeholder credentials; do not reuse them outside development.

```bash
MINIO_ROOT_USER=agents_im_minio
MINIO_ROOT_PASSWORD=agents...word
MINIO_API_PORT=9000
MINIO_CONSOLE_PORT=9001
OBJECT_STORAGE_DRIVER=minio
OBJECT_STORAGE_ENDPOINT=localhost:9000
OBJECT_STORAGE_EXTERNAL_ENDPOINT=localhost:9000
OBJECT_STORAGE_BUCKET=agents-im-media
OBJECT_STORAGE_REGION=us-east-1
OBJECT_STORAGE_USE_SSL=false
OBJECT_STORAGE_EXTERNAL_USE_SSL=false
OBJECT_STORAGE_ACCESS_KEY_ID=agents_im_minio
OBJECT_STORAGE_SECRET_ACCESS_KEY=agents...word
```

The MinIO API is available at `http://localhost:9000`; the console is available at `http://localhost:9001`. The bucket is private and is created by `user-api` at startup. Unit tests use the explicit memory object store and do not require live MinIO.

Message API responses include `messageOrigin=human|ai|system` and Agent metadata when present. Local dev does not enable a production LLM by default; Agent conversation hosting must be wired with an explicit runtime/provider config and otherwise fail closed instead of returning fake AI replies.

## Demo Data

After `scripts/dev-up.sh` succeeds, seed two user-type accounts, one friendship, one group, and one single-chat message:

```bash
scripts/dev-demo-data.sh
```

The script prints demo IDs and a conversation ID. It does not print tokens or passwords.

## Manual Migration Command

When you only need database middleware:

```bash
docker compose up -d postgres redis minio
bash scripts/migrate-postgres.sh
```

## Verification

Before pushing backend changes, run:

```bash
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name '*.go' -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json $(find . -name "*.md" -not -path "./.git/*" -not -path "./.ai-context/*" -not -path "./docs/references/*" -print)
```
