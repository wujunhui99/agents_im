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
make status            # show PID files and ports 8080-8085/5173
make test              # run frontend tests/build/lint plus Go tests
make verify            # run test plus static checks, docker compose config, and git diff check
```

- `make frontend-start` writes logs and PID files under `.dev/logs/frontend.log` and `.dev/pids/frontend.pid`. Override the frontend bind address or port when needed:
- `make test` runs frontend tests/build/lint plus Go tests while excluding accidental Go packages under `web/node_modules`.

```bash
make frontend-start FRONTEND_HOST=127.0.0.1 FRONTEND_PORT=5174
```

## Ports

| Service | URL |
| --- | --- |
| User API | `http://127.0.0.1:8080` |
| Auth API | `http://127.0.0.1:8081` |
| Friends API | `http://127.0.0.1:8082` |
| Message API | `http://127.0.0.1:8083` |
| WebSocket Gateway | `ws://127.0.0.1:8084/ws` |
| Groups API | `http://127.0.0.1:8085` |
| PostgreSQL | `localhost:5432` |
| Redis | `localhost:6379` |
| Redpanda Kafka | `localhost:19092` |

`scripts/dev-up.sh` uses PostgreSQL storage so the separate local API processes share users, credentials, friendships, groups, and message history.

## Demo Data

After `scripts/dev-up.sh` succeeds, seed two users, one friendship, one group, and one single-chat message:

```bash
scripts/dev-demo-data.sh
```

The script prints demo IDs and a conversation ID. It does not print tokens or passwords.

## Manual Migration Command

When you only need database middleware:

```bash
docker compose up -d postgres redis redpanda
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
