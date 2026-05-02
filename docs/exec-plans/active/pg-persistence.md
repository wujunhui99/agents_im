# PostgreSQL Persistence Execution Plan

Status: Completed for branch validation; remains under active pending review/merge
Owner: PostgreSQL Persistence Codex Agent
Branch: feature/pg-persistence

## Goal

Add PostgreSQL persistence for phase-1 user, auth credential, friends, groups, and message storage while keeping memory repositories available for tests and local fallback. JWT middleware and protected route policy are intentionally out of scope for this branch.

## Planner

- Read required repository map and go-zero references:
  - `AGENTS.md`
  - `ARCHITECTURE.md`
  - `.ai-context/zero-skills/SKILL.md`
  - `.ai-context/zero-skills/references/database-patterns.md`
  - `.ai-context/zero-skills/references/goctl-commands.md`
  - `.ai-context/zero-skills/references/rest-api-patterns.md`
  - `.ai-context/zero-skills/references/rpc-patterns.md`
  - `docs/references/go-zero/database-patterns.md`
  - `docs/design-docs/message-storage.md`
  - `docs/design-docs/user-auth-friends-groups-boundaries.md`
- Preserve existing repository interfaces and memory implementations.
- Use PostgreSQL as the durable source of truth for production-style configuration.
- Use docker-compose for local middleware. Do not require host PostgreSQL.
- Keep normal `go test ./...` independent from Docker/PostgreSQL; PG integration tests require `-tags integration` and skip without a DSN.
- Do not implement JWT middleware and do not modify protected route authorization strategy.

## Generator Tasks

- Add `docker-compose.yml`, `.env.example`, and `.gitignore` rules for local PG.
- Add SQL migrations under `db/migrations/`.
- Add DataSource/storage-driver config support and wire API/RPC service contexts to PG only when explicitly configured.
- Implement:
  - `internal/repository` PostgreSQL user/friends repository.
  - `internal/auth/repository` PostgreSQL credential repository.
  - `internal/repository` PostgreSQL groups repository.
  - `internal/repository` PostgreSQL message repository with idempotent send and monotonic read state.
- Add `scripts/migrate-postgres.sh`.
- Add `docs/design-docs/postgres-persistence.md` and update `docs/design-docs/message-storage.md`.
- Update `scripts/verify-static.sh` to assert PG persistence files exist.
- Add integration-style PG tests with a build tag.

## Evaluator Checklist

Run:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name "*.go" -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
git status --short --branch
```

If Docker is unavailable, record it as a blocker for the optional compose validation only.

## Evaluator Results

Completed on branch `feature/pg-persistence`:

- `goctl --version`: `1.10.1 linux/amd64`
- `for f in api/*.api; do goctl api validate -api "$f"; done`: passed for all API specs.
- `gofmt -w $(find . -name "*.go" -print)`: completed.
- `go test ./...`: passed without external PostgreSQL.
- `go test -tags integration ./tests`: passed; tests skip when no PostgreSQL DSN is configured.
- `bash scripts/verify-static.sh`: passed.
- `docker compose config >/dev/null`: passed without printing local dev credentials.
- `git status --short --branch`: branch `feature/pg-persistence` with expected PG persistence changes only.

## goctl Model Decision

`goctl model pg datasource` is not used during initial implementation because it requires a running PostgreSQL datasource with the migration already applied. The repository is implemented directly against go-zero `sqlx`/PostgreSQL connection primitives so normal tests remain external-dependency free.

Follow-up command after `scripts/migrate-postgres.sh` has started and migrated local PG:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl model pg datasource \
  -url "$DATABASE_URL" \
  -table "accounts,profiles,auth_credentials,friendships,groups,group_members,messages,conversation_threads,user_conversation_states,message_idempotency_keys" \
  -dir ./internal/model/pg \
  --style go_zero
```

This follow-up should only be adopted if generated models reduce maintenance without replacing the domain repository interfaces.
