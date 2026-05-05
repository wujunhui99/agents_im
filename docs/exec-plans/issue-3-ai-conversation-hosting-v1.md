# Issue 3 AI Conversation Hosting V1 Plan

## Goal

Complete direct-chat AI hosting where one normal user can enable AI replies on their own behalf, the peer is blocked from enabling until it is disabled, and hosted replies are stored as ordinary `message_origin=ai` messages through Message Service.

## Scope

- Direct conversations only: `single:<account_id>:<account_id>`.
- Setting key: `(owner_account_id, conversation_id)`.
- Default disabled.
- At most one enabled owner per direct conversation.
- AI replies are triggered only by peer human messages and never by AI/system messages.
- Model/provider failures are visible errors and never produce fake production text.
- Runtime context is bounded recent messages plus a summary placeholder flag.

## Files

- `api/message.api`: add `GET/PUT /conversations/:conversation_id/ai-hosting` contract.
- `internal/types/types.go`: generated types for the new REST contract.
- `internal/handler/message/*ai_hosting*_handler.go`: go-zero handlers.
- `internal/logic/message/gozero_logic.go`: go-zero adapter methods.
- `internal/logic/aihostinglogic.go`: direct-chat setting business logic and response shaping.
- `internal/repository/conversation_ai_hosting_repository.go`: repository contract and normalization.
- `internal/repository/conversation_ai_hosting_memory.go`: memory implementation and concurrency-safe mutual exclusion.
- `internal/repository/postgres_conversation_ai_hosting.go`: PostgreSQL implementation backed by partial unique index.
- `internal/repository/postgres_common.go`: storage factory wiring.
- `internal/agentim/hosting.go`: target enabled direct-chat owner when peer sends.
- `internal/agentim/ai_hosting_request_builder.go`: bounded recent-message runtime request builder.
- `internal/agentruntime/eino/deepseek_runtime.go`: fail-closed DeepSeek chat runtime adapter.
- `internal/svc/service_context.go` and `cmd/message-api/main.go`: production wiring.
- `db/migrations/005_conversation_ai_hosting_settings.sql` and matching `db/change_log/*.sql`: schema source of truth.
- `web/src/api/messages.ts`, `web/src/models/messages.ts`, `web/src/features/messages/MessagesPage.tsx`, `web/src/styles.css`: frontend API and direct-chat toggle UI.
- Tests in `internal/logic`, `internal/repository`, `internal/agentim`, `web/src/api`, and `web/src/features/messages`.

## Tasks

- [x] Add failing repository and logic tests for default disabled, enable/disable, non-participant rejection, group rejection, peer conflict, and concurrent enable.
- [x] Add failing Agent-IM tests for peer human trigger, AI recursion skip, missing provider/config failure without fake messages, and bounded context.
- [x] Add failing frontend tests for persisted toggle load, enable/disable, peer-hosted unavailable reason, errors/retry, and group hiding.
- [x] Implement setting repository, migration, and factory wiring.
- [x] Implement business logic and REST handlers.
- [x] Extend hosting trigger selection and bounded runtime request builder.
- [x] Wire message-api production hosting with a fail-closed DeepSeek runtime.
- [x] Implement frontend adapter and UI.
- [x] Update contract docs where Issue #3 differs from the older manual-draft AI reply V1 note.
- [x] Run required verification commands.

## Verification

Required before commit:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name '*.go' -print)
go test ./...
npm --prefix web run test:run -- --reporter=dot
npm --prefix web run build
bash scripts/verify-static.sh
git diff --check
```

PostgreSQL integration is required if a disposable local PostgreSQL database is available:

```bash
AGENTS_IM_CONFIRM_TRUNCATE=1 scripts/verify-postgres-local.sh
```
