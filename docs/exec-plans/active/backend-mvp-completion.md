# Backend MVP Completion Execution Plan

## Goal

Complete the minimum backend features required before starting frontend development.

## Parallel Feature Branches

Create all branches from latest `origin/main`:

1. `feature/mvp-delivery-reliability`
2. `feature/mvp-reconnect-sync`
3. `feature/mvp-social-group-rules`
4. `feature/mvp-observability-health`
5. `feature/mvp-frontend-contracts`

## Branch Boundaries

### mvp-delivery-reliability

Owns delivery attempt persistence, status transitions, retry/failed policy, and docs.

Must not implement social/group rules or frontend docs beyond delivery examples.

Implementation note for this branch:

- Adds [`message-delivery-reliability.md`](../../design-docs/message-delivery-reliability.md).
- Models `delivery_attempts` with `accepted`, `published`, `delivered`, `offline`, and `failed` states.
- Records Transfer/Gateway dispatcher outcomes without changing reconnect sync, friends/groups policy, frontend handoff scope, or native push.

Verification record for `feature/mvp-delivery-reliability` on 2026-04-29:

- `goctl --version`: `goctl version 1.10.1 linux/amd64`.
- `for f in api/*.api; do goctl api validate -api "$f"; done`: five `api format ok` results.
- `gofmt -w $(find . -name '*.go' -print)`: completed with exit code 0.
- `go test ./...`: completed with exit code 0.
- `bash scripts/verify-static.sh`: `static verification passed`.
- `docker compose config`: completed with exit code 0 and rendered `postgres`, `redis`, and `redpanda` services.
- `npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json $(find . -name '*.md' -not -path './.git/*' -not -path './.ai-context/*' -not -path './docs/references/*' -print)`: completed with exit code 0.

### mvp-reconnect-sync

Owns WebSocket reconnect/sync command polish, stable error envelope, missed-message sync docs/tests.

Must not implement delivery persistence or social/group rules.

Implementation note for this branch:

- Adds frontend reconnect sync product contract and WebSocket reconnect sync design doc.
- Updates WebSocket command ACKs to emit frontend fields `requestId`, `command`, `payload`, and nested `error` while keeping legacy `request_id`, `type`, and `data` aliases.
- Maps WebSocket command errors to MVP frontend codes: `UNAUTHORIZED`, `VALIDATION_ERROR`, `NOT_FOUND`, `CONFLICT`, and `INTERNAL`.
- Adds WebSocket tests for reconnect sync, duplicate-safe pull, missing seq pull, and invalid command error envelope.
- Extends static verification to require the new docs and reconnect sync test/code patterns.

Verification record for `feature/mvp-reconnect-sync` on 2026-04-29:

- `goctl --version`: passed, `goctl version 1.10.1 linux/amd64`.
- `for f in api/*.api; do goctl api validate -api "$f"; done`: passed, five `api format ok` results.
- `gofmt -w $(find . -name '*.go' -print)`: passed.
- `go test ./...`: passed.
- `bash scripts/verify-static.sh`: passed, `static verification passed`.
- `docker compose config`: passed.
- `npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json $(find . -name "*.md" -not -path "./.git/*" -not -path "./.ai-context/*" -not -path "./docs/references/*" -print)`: passed.

### mvp-social-group-rules

Owns friends/group MVP behavior and message group membership enforcement.

Must not implement delivery pipeline or observability.

Implementation note for this branch:

- Friends MVP semantics: immediate accepted friendship, duplicate add idempotency, self-add rejection, missing-user rejection, delete/list/status behavior.
- Groups MVP semantics: creator owner/member, open join, active-member list, owner-only leave rejection.
- Group message membership enforcement: non-members and left members fail, active members succeed.

Verification record for `feature/mvp-social-group-rules` on 2026-04-29:

- `goctl --version` -> `goctl version 1.10.1 linux/amd64`.
- `for f in api/*.api; do goctl api validate -api "$f"; done` -> five `api format ok` lines.
- `gofmt -w $(find . -name '*.go' -print)` -> passed with no output.
- `go test ./...` -> passed; final rerun package test output ended with `ok github.com/wujunhui99/agents_im/tests (cached)`.
- `bash scripts/verify-static.sh` -> `static verification passed`.
- `docker compose config` -> passed and rendered postgres, redis, and redpanda services.
- Markdown link check excluding `.ai-context` and `docs/references` -> passed.

### mvp-observability-health

Owns health/readiness/metrics/request-trace-id docs and implementation.

Must not change core business semantics.

Implementation note for this branch:

- Adds health/readiness endpoints and observability configuration for API, gateway, and transfer worker processes.
- Adds Prometheus-style MVP metrics, including message send counters.
- Adds trace ID propagation in HTTP and WebSocket envelopes, including heartbeat ACK trace IDs.
- Extends static verification to ensure observability helpers do not inspect request URI, raw query, authorization headers, passwords, tokens, or request bodies.

Verification record for `feature/mvp-observability-health` on 2026-04-29:

- `goctl --version`: passed, `goctl version 1.10.1 linux/amd64`.
- `for f in api/*.api; do goctl api validate -api "$f"; done`: passed, five `api format ok` results.
- `go test ./...`: passed.
- `bash scripts/verify-static.sh`: passed, `static verification passed`.
- `docker compose config`: passed.
- Markdown link check excluding `docs/references`: passed.

### mvp-frontend-contracts

Owns final frontend-facing contract document, local dev bootstrap scripts, and MVP acceptance smoke tests. It may add docs/tests only against contracts already implemented by the other branches. If it needs implementation changes, keep them minimal and documented.

## Required Context for Codex

Every Codex agent must read:

- `AGENTS.md`
- `ARCHITECTURE.md`
- `.ai-context/zero-skills/SKILL.md`
- `.ai-context/zero-skills/references/goctl-commands.md`
- `.ai-context/zero-skills/references/rest-api-patterns.md`
- `.ai-context/zero-skills/references/rpc-patterns.md`
- `.ai-context/zero-skills/references/database-patterns.md`
- `docs/product-specs/backend-mvp.md`
- `docs/design-docs/backend-mvp-contract.md`
- this execution plan

## Verification

Every branch must run:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name '*.go' -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
```

If docs changed, run markdown link check excluding `docs/references`.

## Merge Order

1. `feature/mvp-delivery-reliability`
2. `feature/mvp-reconnect-sync`
3. `feature/mvp-social-group-rules`
4. `feature/mvp-observability-health`
5. `feature/mvp-frontend-contracts`

Then verify `develop`, push, merge tested `develop` into `main`, verify again, and push.
