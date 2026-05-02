# PostgreSQL Persistence Design

Status: Implemented
Owner: PostgreSQL Persistence
Related docs:
- [`message-storage.md`](./message-storage.md)
- [`message-outbox.md`](./message-outbox.md)
- [`user-auth-friends-groups-boundaries.md`](./user-auth-friends-groups-boundaries.md)

## Background

Phase 1 previously used in-memory repositories for user, auth, friends, groups, and message behavior. That remains useful for unit tests and isolated local runs, but production-style configuration needs PostgreSQL as the durable store.

## Goals

- Provide local PostgreSQL through `docker-compose.yml`.
- Store phase-1 user, auth credential, friendship, group, message, conversation, read-state, idempotency, outbox, and delivery-attempt data in PostgreSQL.
- Keep normal `go test ./...` independent from Docker/PostgreSQL.
- Preserve existing domain repository interfaces.
- Keep auth secrets owned by auth storage, not user/message storage.

## Non-goals

- JWT middleware or protected route policy changes.
- Redis, Kafka, WebSocket delivery, push, or worker implementation.
- Host-machine PostgreSQL installation.
- Sharding, cross-region replication, or generated goctl model adoption.

## Schema

Migration SQL lives under [`../../db/migrations/001_init_postgres.sql`](../../db/migrations/001_init_postgres.sql).

Phase-1 tables:

- `users`: user profile authority; contains no password, hash, salt, token, or credential fields.
- `auth_credentials`: auth-owned credential row keyed by identifier; stores password hash, salt, and hash version.
- `friendships`: directional friendship rows; repository writes reciprocal rows in one transaction.
- `groups`: group metadata.
- `group_members`: group membership state.
- `conversation_threads`: per-conversation ordering state and last-message pointer.
- `messages`: immutable accepted messages.
- `user_conversation_states`: per-user visibility and read progress.
- `message_idempotency_keys`: explicit sender/client idempotency record.
- `message_outbox`: transactional outbox rows for accepted message events.
- `delivery_attempts`: per-recipient delivery state for accepted/published/delivered/offline/failed outcomes.

## Service Boundary Constraints

Strong foreign keys are only used inside a service-owned aggregate:

- `group_members.group_id -> groups.group_id`
- `messages.conversation_id -> conversation_threads.conversation_id`
- `user_conversation_states.conversation_id -> conversation_threads.conversation_id`
- `message_idempotency_keys.server_msg_id -> messages.server_msg_id`
- `message_outbox.server_msg_id -> messages.server_msg_id`
- `delivery_attempts.server_msg_id -> messages.server_msg_id`

Cross-service references are logical constraints:

- `auth_credentials.user_id` references an account created through the auth/Account Service registration flow, but has no database FK to V0 `users`.
- `friendships.user_id` and `friendships.friend_id` are account id aliases validated by the friends logic through Account Service lookup.
- `groups.creator_user_id` and `group_members.user_id` are account id aliases validated by groups logic through Account Service lookup.
- Message sender/receiver/member IDs are validated before repository writes when validators are configured.

This keeps the database usable during microservice extraction and avoids coupling service ownership through cross-service FK cascades.

## Configuration

Services default to memory repositories:

```yaml
StorageDriver: memory
DataSource: ${DATABASE_URL}
```

To use PostgreSQL:

```yaml
StorageDriver: postgres
DataSource: ${DATABASE_URL}
```

`DATABASE_URL`, `AGENTS_IM_POSTGRES_DSN`, or `POSTGRES_DSN` may provide the PostgreSQL DSN. Local development values are documented in `.env.example` and are marked development-only.

## Local Migration

Start and migrate local PostgreSQL with:

```bash
scripts/migrate-postgres.sh
```

The script uses `docker compose` by default and does not require host PostgreSQL. If a host `psql` client is intentionally used, run:

```bash
DATABASE_URL=... scripts/migrate-postgres.sh --host-psql
```

## Repository Implementation

The implementation uses go-zero `sqlx` through `github.com/zeromicro/go-zero/core/stores/postgres`.

Repository files:

- `internal/repository/postgres_user_friends.go`
- `internal/auth/repository/postgres.go`
- `internal/repository/postgres_groups.go`
- `internal/repository/postgres_message.go`
- `internal/repository/postgres_outbox.go`

Normal tests continue to use memory repositories. PostgreSQL integration tests are build-tagged with `integration` and skip when no DSN is configured.

## Message Consistency

Message writes use one PostgreSQL transaction:

1. Check `message_idempotency_keys`.
2. Upsert and lock the `conversation_threads` row.
3. Allocate `seq = max_seq + 1`.
4. Insert `messages`.
5. Insert `message_idempotency_keys`.
6. Upsert visible `user_conversation_states`.
7. Advance sender `has_read_seq`.
8. Update conversation max seq and last-message fields.
9. Insert accepted `delivery_attempts` rows for message recipients, excluding the sender.
10. Insert one `message_outbox` row for the `message.created` event.

The locked conversation row is the serialization point, preserving contiguous per-conversation seq values.

The outbox row is committed atomically with the accepted message. It is a reliable asynchronous event source for later Kafka/Message Transfer/Push workers and does not change the synchronous send response semantics.

## goctl Model Decision

`goctl model pg datasource` was not used for the initial repository implementation because the command requires a running PostgreSQL datasource with migrations already applied. Direct repository code keeps the current domain interfaces stable and avoids making ordinary tests depend on local middleware.

Follow-up command after local PG is running and migrated:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl model pg datasource \
  -url "$DATABASE_URL" \
  -table "users,auth_credentials,friendships,groups,group_members,messages,conversation_threads,user_conversation_states,message_idempotency_keys,message_outbox,delivery_attempts" \
  -dir ./internal/model/pg \
  --style go_zero
```

Generated models should only be adopted if they reduce maintenance without weakening service boundaries or replacing domain repository contracts.

## Validation

Required validation:

- `goctl --version`
- `for f in api/*.api; do goctl api validate -api "$f"; done`
- `gofmt -w $(find . -name "*.go" -print)`
- `go test ./...`
- `bash scripts/verify-static.sh`
- `docker compose config` when Docker is available
