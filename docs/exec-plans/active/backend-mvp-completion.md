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

### mvp-reconnect-sync

Owns WebSocket reconnect/sync command polish, stable error envelope, missed-message sync docs/tests.

Must not implement delivery persistence or social/group rules.

### mvp-social-group-rules

Owns friends/group MVP behavior and message group membership enforcement.

Must not implement delivery pipeline or observability.

### mvp-observability-health

Owns health/readiness/metrics/request-trace-id docs and implementation.

Must not change core business semantics.

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

### feature/mvp-social-group-rules verification record

Date: 2026-04-29

Scope completed:

- Friends MVP semantics: immediate accepted friendship, duplicate add idempotency, self-add rejection, missing-user rejection, delete/list/status behavior.
- Groups MVP semantics: creator owner/member, open join, active-member list, owner-only leave rejection.
- Group message membership enforcement: non-members and left members fail, active members succeed.

Commands run:

- `goctl --version` -> `goctl version 1.10.1 linux/amd64`
- `for f in api/*.api; do goctl api validate -api "$f"; done` -> five `api format ok` lines.
- `gofmt -w $(find . -name '*.go' -print)` -> passed with no output.
- `go test ./...` -> passed; final rerun package test output ended with `ok github.com/wujunhui99/agents_im/tests (cached)`.
- `bash scripts/verify-static.sh` -> `static verification passed`.
- `docker compose config` -> passed and rendered postgres, redis, and redpanda services.
- Markdown link check excluding `.ai-context` and `docs/references` -> passed.

## Merge Order

1. `feature/mvp-delivery-reliability`
2. `feature/mvp-reconnect-sync`
3. `feature/mvp-social-group-rules`
4. `feature/mvp-observability-health`
5. `feature/mvp-frontend-contracts`

Then verify `develop`, push, merge tested `develop` into `main`, verify again, and push.
