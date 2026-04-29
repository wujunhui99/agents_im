# IM Backend MVP Completion Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Bring the `agents_im` backend from “main message pipeline skeleton” to a practical MVP backend that is ready for frontend integration.

**Architecture:** Keep the existing go-zero/goctl service structure, PostgreSQL as source of truth, Redis for short-lived presence/session state, and Redpanda/Kafka for async message delivery. Finish only the minimum backend behavior needed for an MVP: reliable online delivery, reconnect sync, social/group rules, basic admin/dev operations, observability, and frontend-friendly API/WebSocket contracts. Agent system remains out of MVP unless explicitly reprioritized.

**Tech Stack:** Go 1.22, go-zero/goctl, PostgreSQL, Redis, Redpanda/Kafka, WebSocket Gateway, Docker Compose, GitHub Actions.

---

## Current Context

Repository: `/home/ws/project/agents_im`

Current `main`:

```text
8ffce5dd3fb911ab01f0bbfc61d3e6b53c2b01ab
```

Current `develop`:

```text
e89010ef9f0583e03840db89ad633801904d40df
```

Already completed:

- User/Auth/Friends/Groups base services.
- goctl/go-zero REST/RPC scaffold migration.
- JWT auth middleware.
- PostgreSQL persistence and docker-compose middleware.
- Redis Presence.
- WebSocket Gateway first phase.
- Kafka/Redpanda message event contract.
- PostgreSQL transactional outbox.
- Outbox → Kafka publisher.
- Kafka → Transfer consumer.
- Transfer → Gateway dispatcher.
- Gateway → Presence routing seam.

Known MVP gaps from `ARCHITECTURE.md` and design docs:

- Real cross-process Gateway delivery is not implemented.
- Delivery ACK/retry/DLQ is not complete.
- WebSocket reconnect/resume is still client-driven by seq pull, not a polished MVP contract.
- Offline push notifications are not implemented; for MVP we can defer native mobile push but must ensure missed-message sync works.
- Friends/groups rules exist as base service skeletons but need MVP-level behavior and validation.
- Observability is documented but not implemented.
- Frontend contract documentation/OpenAPI examples are not yet packaged for frontend handoff.

## MVP Definition

Backend MVP is “done” when a frontend can build against it and demo:

1. Register/login with JWT.
2. Fetch and update own profile.
3. Search users by unique identifier.
4. Add/remove/list friends with deterministic MVP semantics.
5. Create/join/leave/list groups and members with deterministic MVP semantics.
6. Open WebSocket connection with JWT.
7. Send one-to-one and group messages.
8. Receive online messages in another connected client.
9. Disconnect/reconnect and fetch missed messages by conversation seq.
10. Mark conversations read and see unread counts.
11. Run backend locally with `docker compose` and a documented bootstrap script.
12. Debug basic failures with logs, health endpoints, metrics, and trace/request IDs.

Native app push, full admin console, enterprise permission model, end-to-end encryption, media upload/CDN, and production Kubernetes are explicitly non-MVP unless the user changes scope.

---

## Branch / Worktree Strategy

Use parallel worktrees from latest `main`, then merge into `develop`, validate, then merge tested `develop` to `main`.

Suggested branches:

1. `feature/mvp-delivery-reliability`
2. `feature/mvp-reconnect-sync`
3. `feature/mvp-social-group-rules`
4. `feature/mvp-observability-health`
5. `feature/mvp-frontend-contracts`

Merge order:

1. `mvp-delivery-reliability`
2. `mvp-reconnect-sync`
3. `mvp-social-group-rules`
4. `mvp-observability-health`
5. `mvp-frontend-contracts`

Expected conflict files:

- `ARCHITECTURE.md`
- `docs/design-docs/index.md`
- `docs/product-specs/index.md`
- `scripts/verify-static.sh`
- `internal/config/config.go`
- `docker-compose.yml`
- WebSocket tests under `tests/`

---

# Wave 1: Message Delivery Reliability MVP

## Task 1: Define delivery status model

**Objective:** Add a minimal delivery state contract for online delivery attempts.

**Files:**

- Create/modify: `docs/design-docs/message-delivery-reliability.md`
- Modify: `docs/design-docs/index.md`
- Modify: `ARCHITECTURE.md`
- Modify: `scripts/verify-static.sh`

**Required behavior:**

Define MVP status semantics:

- `accepted`: Message Service persisted message and outbox event.
- `published`: outbox publisher published to Kafka-compatible topic.
- `delivered`: Gateway pushed event to at least one online recipient connection.
- `offline`: recipient had no local/known online route.
- `failed`: delivery attempt failed and is retryable or final depending on error.

Do not claim user has “read” message from delivery status. Read state remains `has_read_seq`.

**Verification:**

```bash
bash scripts/verify-static.sh
```

Expected: `static verification passed`.

## Task 2: Add delivery attempt repository contract

**Objective:** Persist minimal delivery attempts for retry/debugging.

**Files:**

- Modify: `db/migrations/001_init_postgres.sql`
- Modify: `internal/repository/message_repository.go`
- Modify: `internal/repository/postgres_*.go`
- Modify: `internal/repository/message_memory.go`
- Test: `tests/message_delivery_reliability_test.go` or package-level repository tests

**Schema suggestion:**

```sql
CREATE TABLE IF NOT EXISTS message_delivery_attempts (
  id BIGSERIAL PRIMARY KEY,
  server_msg_id TEXT NOT NULL,
  conversation_id TEXT NOT NULL,
  recipient_user_id TEXT NOT NULL,
  status TEXT NOT NULL,
  attempt_count INTEGER NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT '',
  next_retry_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(server_msg_id, recipient_user_id)
);
```

**Verification:**

```bash
go test ./internal/repository ./tests
```

Expected: pass without live PostgreSQL unless integration DSN is set.

## Task 3: Wire Transfer dispatcher delivery result persistence

**Objective:** Record `delivered/offline/failed` after Transfer → Gateway dispatch.

**Files:**

- Modify: `internal/transfer/gateway/dispatcher.go`
- Modify: `internal/transfer/worker.go`
- Modify: `internal/repository/message_repository.go`
- Test: `internal/transfer/gateway/dispatcher_test.go`

**Rules:**

- Online local delivery returns `delivered`.
- Known route but not local returns `routed` or `offline` depending existing route semantics; for MVP avoid claiming remote delivered until remote Gateway RPC/PubSub exists.
- Retryable dispatcher error increments attempt count and sets `next_retry_at`.

**Verification:**

```bash
go test ./internal/transfer/... ./tests
```

Expected: pass.

## Task 4: Add retry/DLQ MVP policy

**Objective:** Define and implement minimal retry classification for Transfer delivery failures.

**Files:**

- Modify: `internal/transfer/worker.go`
- Modify: `internal/transfer/interfaces.go`
- Modify: `internal/transfer/memory.go`
- Modify: `docs/design-docs/message-delivery-reliability.md`
- Test: `internal/transfer/worker_test.go`

**MVP policy:**

- Retryable errors retry up to 3 attempts with simple exponential or fixed backoff.
- Non-retryable errors become `failed`.
- DLQ can be a persisted failed state/documented contract, not necessarily a separate Kafka topic in MVP.

**Verification:**

```bash
go test ./internal/transfer/...
bash scripts/verify-static.sh
```

---

# Wave 2: Reconnect / Missed Message Sync MVP

## Task 5: Document frontend sync contract

**Objective:** Make reconnect behavior frontend-ready.

**Files:**

- Create: `docs/product-specs/frontend-sync-contract.md`
- Create: `docs/design-docs/websocket-reconnect-sync.md`
- Modify: `docs/product-specs/index.md`
- Modify: `docs/design-docs/index.md`
- Modify: `scripts/verify-static.sh`

**Contract:**

On login/connect/reconnect frontend calls:

1. `get_conversation_seqs` to get `max_seq`, `has_read_seq`, `unread_count`.
2. For each conversation with missing seq, call `pull_messages` from local last seq + 1.
3. WebSocket online events are best-effort; PostgreSQL message history is authoritative.

**Verification:**

```bash
bash scripts/verify-static.sh
```

## Task 6: Add WebSocket sync command if missing

**Objective:** Ensure WebSocket has a direct command for sync bootstrap.

**Files:**

- Modify: `docs/product-specs/gateway-message-contract.md`
- Modify: `internal/gateway/ws/server.go`
- Modify: `tests/websocket_gateway_test.go`

**Required commands:**

- Existing `get_conversation_seqs` must be stable and documented.
- Existing `pull_messages` must support `conversation_id` and `from_seq`.
- Response examples should be frontend-friendly and stable.

**Verification:**

```bash
go test ./tests -run Gateway
```

## Task 7: Add frontend-friendly error envelopes

**Objective:** Normalize command errors so frontend can handle them.

**Files:**

- Modify: `internal/gateway/ws/server.go`
- Modify: `internal/apperror` or response helpers if applicable
- Test: `tests/websocket_gateway_test.go`

**Required shape:**

```json
{
  "requestId": "...",
  "status": "error",
  "error": {
    "code": "UNAUTHORIZED|VALIDATION_ERROR|NOT_FOUND|INTERNAL",
    "message": "..."
  }
}
```

**Verification:**

```bash
go test ./tests -run Gateway
```

---

# Wave 3: Social / Group MVP Rules

## Task 8: Define friends MVP semantics

**Objective:** Make friend behavior predictable enough for frontend MVP.

**Files:**

- Modify: `docs/product-specs/account-social-core.md`
- Modify: `docs/design-docs/user-auth-friends-groups-boundaries.md`
- Modify: `scripts/verify-static.sh`

**MVP choice:**

Use one of these explicit semantics. Recommended MVP default:

- `AddFriend` creates an accepted friendship immediately.
- Duplicate add is idempotent.
- Delete friend removes both directions or marks deleted.
- Friend list returns current accepted friends.

Friend request approval can be future work.

## Task 9: Implement/verify friend rules in logic and PG repository

**Objective:** Ensure friend API/RPC and PG behavior match documented MVP semantics.

**Files:**

- Modify: `internal/logic/friends/*`
- Modify: `internal/repository/postgres_*friends*` or equivalent
- Modify: `internal/repository/message_memory.go` only if shared repository currently hosts friends
- Test: `tests/friends_service_test.go`

**Tests:**

- Add friend succeeds.
- Duplicate add is idempotent.
- Delete friend succeeds.
- Friend list reflects changes.
- Adding non-existent user fails.
- Self-add fails.

**Verification:**

```bash
go test ./tests -run Friends
go test ./...
```

## Task 10: Define groups MVP semantics

**Objective:** Make group behavior predictable enough for frontend MVP.

**Files:**

- Modify: `docs/product-specs/account-social-core.md`
- Modify: `docs/design-docs/user-auth-friends-groups-boundaries.md`

**MVP choice:**

- Creator becomes owner/member.
- Join group is open by default.
- Leave group removes membership; owner cannot leave if sole owner unless group is dissolved or ownership transferred.
- Send group message requires sender to be group member.

## Task 11: Implement/verify group rules and message group membership validation

**Objective:** Enforce group membership on group message send.

**Files:**

- Modify: `internal/logic/groups/*`
- Modify: `internal/logic/message/*`
- Modify: `internal/svc/service_context.go`
- Test: `tests/groups_service_test.go`
- Test: `tests/message_service_test.go`

**Tests:**

- Create group sets creator as member.
- Join/leave/list members works.
- Non-member group send fails.
- Member group send succeeds.
- Pull group messages works by seq.

**Verification:**

```bash
go test ./tests -run 'Groups|Message'
go test ./...
```

---

# Wave 4: Observability / Health MVP

## Task 12: Add service health/readiness endpoints

**Objective:** Frontend/devops can know whether services are alive and dependencies are configured.

**Files:**

- Modify API specs under `api/*.api` if endpoints are REST service-specific.
- Add shared health helper under `internal/health/`.
- Modify command entrypoints under `cmd/*-api` or go-zero handlers as needed.
- Test: `tests/health_test.go`

**Endpoints:**

- `GET /healthz`: process alive.
- `GET /readyz`: config/dependency readiness where safe.

**Verification:**

```bash
for f in api/*.api; do goctl api validate -api "$f"; done
go test ./tests -run Health
```

## Task 13: Add basic metrics

**Objective:** Add MVP Prometheus-compatible metrics for message path.

**Files:**

- Create: `internal/observability/metrics.go`
- Modify: `cmd/*/main.go` or middleware wiring.
- Modify: `docker-compose.yml` only if adding Prometheus local service.
- Modify: `.env.example`
- Docs: `docs/design-docs/observability-mvp.md`

**Metrics:**

- WebSocket connections current count.
- Message send count.
- Message delivery attempts count by status.
- Outbox publish count/failure count.
- Transfer consume count/failure count.

**MVP caveat:** If adding full Prometheus is too much, expose `/metrics` and document how frontend/dev can inspect it.

## Task 14: Add structured request/trace IDs

**Objective:** Make debugging frontend/backend flows possible.

**Files:**

- Create/modify: `internal/observability/trace.go`
- Modify HTTP middleware / WebSocket command handling.
- Tests: unit tests for trace ID propagation where practical.

**Rules:**

- Accept `X-Request-Id` from HTTP clients, generate if absent.
- WebSocket command `requestId` remains command-level correlation.
- Logs include request/connection/user identifiers without secrets.

---

# Wave 5: Frontend Handoff Contracts

## Task 15: Create frontend API/WebSocket contract document

**Objective:** Give frontend exact endpoints, request/response examples, auth behavior, and WebSocket commands.

**Files:**

- Create: `docs/product-specs/frontend-backend-contract.md`
- Modify: `docs/product-specs/index.md`
- Modify: `AGENTS.md`
- Modify: `scripts/verify-static.sh`

**Include:**

- Auth register/login examples.
- `/me` and user profile examples.
- Friend APIs examples.
- Group APIs examples.
- Message APIs examples.
- WebSocket connect URL and JWT options.
- WebSocket command examples:
  - `heartbeat`
  - `send_message`
  - `pull_messages`
  - `get_conversation_seqs`
  - `mark_conversation_read`
- Server push event examples:
  - `message_received`
  - `message_delivered`
- Error envelope examples.

## Task 16: Add local demo bootstrap script

**Objective:** Frontend developer can start backend locally with one documented flow.

**Files:**

- Create: `scripts/dev-up.sh`
- Create: `scripts/dev-seed.sh` or `scripts/dev-demo-data.sh`
- Modify: `README.md` or `docs/DEVELOPMENT.md`
- Modify: `.env.example`

**Behavior:**

- `docker compose up -d postgres redis redpanda` or all dependencies.
- Run migrations.
- Optionally seed two users, a friend relation, a group, and example tokens for local dev only.
- Never commit real secrets.

**Verification:**

```bash
docker compose config
bash -n scripts/dev-up.sh
bash -n scripts/dev-demo-data.sh
```

## Task 17: Add MVP acceptance test suite

**Objective:** One command proves backend MVP still works.

**Files:**

- Create: `tests/mvp_backend_test.go`
- Modify: `scripts/verify-static.sh`
- Modify: `.github/workflows/ci.yml` if needed

**Test scenarios:**

- register/login profile flow.
- friend add/list/delete flow.
- group create/join/send flow.
- WebSocket two-client single chat online delivery.
- reconnect sync via seq pull.
- mark-read unread count flow.

Keep default tests independent of external middleware; live PG/Redis/Kafka tests must skip unless env vars are set.

---

## Validation for Every Feature Branch

Run from each feature worktree:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name '*.go' -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
python3 - <<'PY'
from pathlib import Path
import re, sys
root=Path('.')
missing=[]
for p in list(root.glob('*.md')) + list(root.glob('docs/**/*.md')):
    if 'docs/references' in p.as_posix():
        continue
    text=p.read_text(errors='ignore')
    for m in re.finditer(r'\[[^\]]+\]\(([^)]+)\)', text):
        link=m.group(1).split('#',1)[0]
        if not link or re.match(r'^[a-z]+:', link) or link.startswith('mailto:'):
            continue
        target=(p.parent/link).resolve()
        if not target.exists():
            missing.append((str(p), link))
if missing:
    print('missing markdown links:')
    for item in missing:
        print(item)
    sys.exit(1)
print('markdown links ok')
PY
```

Expected:

- `goctl version 1.10.1 linux/amd64`
- all `api format ok`
- `go test ./...` passes
- `static verification passed`
- `docker compose config` passes
- markdown links ok

## Final Integration Validation

After merging all MVP branches into `develop`, run the full validation suite on `develop`, push, then merge tested `develop` into `main`, run the suite again, and push `main`.

Final report must include:

- feature branch SHAs
- `origin/develop` SHA
- `origin/main` SHA
- validation command outputs summary
- MVP acceptance scenarios covered
- explicit non-MVP items left for later

---

## Risks and Tradeoffs

1. **Cross-Gateway delivery complexity**
   - True remote Gateway RPC/PubSub can grow large. MVP can use single Gateway instance plus presence route metadata and documented limitation, or add Redis PubSub/NATS only if needed.

2. **Delivery ACK ambiguity**
   - Avoid confusing delivered/read. Delivered means pushed to connection, not read by user.

3. **Offline push scope**
   - Native push is not needed for frontend MVP if reconnect sync works. Defer APNs/FCM/vendor push.

4. **Friend request workflow**
   - Approval workflow can wait. Use idempotent immediate-add MVP unless user wants real request/accept now.

5. **Integration test stability**
   - Keep external middleware tests opt-in so CI remains reliable.

## Open Questions

These are the only decisions that might change implementation scope:

1. For MVP friends: immediate add, or request/accept workflow?
2. For MVP groups: open join, invite-only, or owner approval?
3. For MVP Gateway deployment: single Gateway instance acceptable, or must support cross-instance delivery now?
4. For frontend: web first, mobile first, or both? This affects token storage and WebSocket reconnect examples.

Recommended defaults if no answer is provided:

- Friends: immediate idempotent add.
- Groups: open join for MVP.
- Gateway: single instance supported, cross-instance contract documented; add true cross-instance after frontend demo.
- Frontend: web first.
