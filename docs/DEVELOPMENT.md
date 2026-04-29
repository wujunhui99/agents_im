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
