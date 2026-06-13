# Development

适用场景：本地启动后端/前端、调试端口、运行本机 smoke 或定位开发环境问题。

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
make status            # show PID files and ports 8080-8089/5173
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
MSG_API_PORT=18090 \
GATEWAY_WS_PORT=18084 \
GROUPS_API_PORT=18085 \
AGENT_API_PORT=18086 \
MEDIA_API_PORT=18089 \
MEDIA_RPC_PORT=19096 \
ADMIN_API_PORT=18088 \
ADMIN_RPC_PORT=19097 \
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
MSG_API_PORT=18090 \
GATEWAY_WS_PORT=18084 \
GROUPS_API_PORT=18085 \
MEDIA_API_PORT=18089 \
ADMIN_API_PORT=18088 \
make frontend-start FRONTEND_HOST=127.0.0.1 FRONTEND_PORT=5173
```

The local message chain mirrors production (03 §9 B3b single rail): `docker compose up -d redpanda` provides Kafka, msg-rpc publishes `msg.toTransfer.v1`, and `msgtransfer` allocates seqs, persists to Postgres, and dispatches `message_received` to `msggateway`. msg-rpc and msgtransfer fail fast at startup when `KAFKA_BROKERS` (default `localhost:19092`) is unreachable or unset.

## Ports

本地服务名和 package path 以 `Makefile` 为准；启动顺序、生成配置和默认端口以 `scripts/dev-up.sh` 为准。下表只作常用端口概览。

| Service | URL |
| --- | --- |
| Account API (V0 `user-api`) | `http://127.0.0.1:8080` |
| Auth API | `http://127.0.0.1:8081` |
| Friends API | `http://127.0.0.1:8082` |
| Msg API | `http://127.0.0.1:8090` |
| WebSocket Gateway | `ws://127.0.0.1:8084/ws` |
| Groups API | `http://127.0.0.1:8085` |
| Agent API | `http://127.0.0.1:8086` |
| Message Transfer health | `http://127.0.0.1:8087/healthz` |
| Admin API | `http://127.0.0.1:8088` |
| Media API | `http://127.0.0.1:8089` |
| PostgreSQL | `localhost:5432` |
| Redis | `localhost:6379` |
| MinIO API | `http://localhost:9000` |
| MinIO Console | `http://localhost:9001` |
| Tempo (OTLP gRPC / HTTP UI) | `localhost:4317` / `http://localhost:3200` |

`scripts/dev-up.sh` also starts Tempo (`grafana/tempo`, same image as prod) as the local tracing backend and exports `AGENTS_IM_OTLP_ENDPOINT=127.0.0.1:4317` so local services report traces just like production. Services using `pkg/observability` read that env; goctl-native services (e.g. `groups`) carry a `Telemetry` block in their generated config. Config lives in `deploy/local/tempo.yaml`.

`scripts/dev-up.sh` uses PostgreSQL storage so the separate local API processes share account profiles (V0 `users` table), credentials, friendships, groups, Agent profiles, media metadata, and message history. It also starts MinIO for local object storage and wires media object access through `media-api` + `media-rpc`. Agent creation verifies `account_type=agent` through the Account Service profile repository; unavailable verification fails closed.

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

The MinIO API is available at `http://localhost:9000`; the console is available at `http://localhost:9001`. The bucket is private and is created by `media-rpc` at startup. Unit tests use the explicit memory object store and do not require live MinIO.

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
npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json $(find . -name "*.md" -not -path "./.git/*" -not -path "./docs/references/*" -print)
```
