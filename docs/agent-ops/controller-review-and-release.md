# Controller Review and Release Checklist

Hermes uses this checklist after a Codex/specialized worker reports completion. It is intentionally stricter than worker self-report because production readiness requires integration evidence.

## First response to a worker handoff

1. Confirm branch/worktree/commit/PR exist.
2. Inspect `git status --short --branch`.
3. Inspect changed files and diff stats.
4. Read the core diff, especially business logic, API contracts, DB migrations, deployment scripts, and security-sensitive paths.
5. Compare implementation against the Issue acceptance criteria.
6. Check that the worker posted an Issue comment.

Do not accept a worker handoff only because the process exited successfully.

## Review focus by area

### Backend / go-zero

- Prefer generated goctl REST/RPC scaffold over hand-written compatibility layers when the task is a go-zero/goctl refactor.
- Keep service boundaries consistent with `docs/design-docs/user-auth-friends-groups-boundaries.md` and message docs.
- DB changes must use `db/migrations` as the single executable SQL source of truth (published migrations are immutable; add the next-numbered file).
- Repository SQL changes should have PostgreSQL verification when possible; if local PG/Docker is unavailable, state that clearly.

### Message / WebSocket

- `conversation_id + seq` is display-order authority.
- Duplicate consecutive user messages are valid and must not be collapsed by payload hash.
- Browser WebSocket cannot set `Authorization`; production same-origin uses `/ws?token=[REDACTED]` query fallback.
- WebSocket open is not proof of live push. Verify A sends, B receives without refresh when testing live delivery.
- Mark trigger messages read before async AI reply where applicable; avoid AI loops.

### Frontend React / Management System

- Preserve the four-tab user shell: `消息 / 联系人 / 发现 / 我的`.
- The design language is WeChat-style information architecture plus a lightweight Material 3-inspired visual system.
- Use existing tokens and components before inventing raw DOM controls.
- Do not expose internal account/user IDs in user-facing UI unless the contract explicitly says so.
- Production frontend must use real APIs; mock/demo fallback is allowed only in tests or explicit demo modes.
- Management System pages should show field labels/table headers and should not look like unlabeled value dumps.

### Mobile / Flutter client

- Flutter may own user-facing mobile clients and possibly future user Web.
- React remains the better default for Management System/admin/ops pages.
- Mobile contract must respect `conversation_id + seq`, `client_msg_id` reconciliation, reconnect + backfill, and secure local session storage.
- First Flutter MVP should prioritize login, session restore, conversation list, single-chat text, foreground WebSocket, backfill, Me page, and feedback.

### Observability / operations

- Jaeger all-in-one memory storage is not production-grade. Current tracing direction is OTel Collector + Grafana Tempo with durable storage.
- Grafana is the observability UI; Tempo/Loki should remain internal unless a design explicitly changes that.
- Management System may aggregate status summaries and links, but should not become an unsafe shell/Kubernetes dashboard.
- Server status pages should be read-only, summary-oriented, and secret-free.

### Deployment / CI

- Merging a feature PR to `main` is not yet a verified production release; confirm the deployed SHA.
- The `develop` integration branch has been removed; all changes go via task-branch PR + GitHub Merge Queue into `main`, which triggers the Drone deploy.
- Drone CI (not GitHub Actions) is the active deploy path; check Drone/k3s/project docs.
- CI green is not runtime green. Verify rollout, image tags, pod readiness, and live HTTP/API/WS smoke when relevant.
- `kubectl apply` does not prune deleted resources by default; removed domains/resources require explicit cleanup plus old-endpoint smoke.

## Minimum local verification

Choose commands based on changed files:

### Docs-only

```bash
git diff --check
bash scripts/verify-static.sh
```

### Frontend

```bash
npm --prefix web test -- <focused-test> --run
npm run frontend:test
npm run frontend:build
npm run frontend:lint
git diff --check
bash scripts/verify-static.sh
```

### Backend/API

```bash
gofmt -w <changed-go-files>
go test ./...
git diff --check
bash scripts/verify-static.sh
```

Add PostgreSQL integration for DB/repository changes when available.

### Deployment/observability

```bash
bash scripts/verify-static.sh
docker compose config
# render/deploy script checks as relevant
```

Then verify live k3s/Drone behavior when the change reaches production.

## PR merge rules

1. Feature branches merge to `develop`.
2. Use squash merge for normal feature PRs.
3. Delete short-lived feature branches after merge.
4. Do not delete long-lived branches such as `develop` during promotion.
5. Promotion PRs from `develop -> main` may use merge commits to preserve integration history.
6. Close the Issue only after the relevant integration/release stage is complete.

## Production smoke evidence

For production-affecting changes, record:

- promotion PR or deploy branch commit,
- deployed image tag/SHA for affected services,
- rollout status,
- pod readiness/restart concerns,
- live HTTP/API/WS smoke result,
- stale resource cleanup evidence if resources were removed,
- any skipped checks and why.

Keep public comments secret-free. Do not include real tokens, JWTs, cookies, DSNs, SSH host/user/port/key, or raw provider credentials.

## Duplicate or already-implemented work

Before creating architecture/ops issues, check whether the direction already exists in docs/manifests/live deployment. If it is already implemented, comment evidence instead of creating duplicate work. If a duplicate Issue was accidentally created, close it with concise evidence.

## Stale worker processes

A Codex process may hang or continue after producing a commit. If the worktree has the expected commit, status is clean, and controller-side verification passes, treat the process as stale; kill it and proceed. Record the kill as non-blocking process cleanup, not a code failure.

## Final report shape

When reporting to the human, distinguish:

- Issue created/refined,
- implementation PR merged to `develop`,
- promotion PR/commit to `main`,
- deploy status,
- deployed SHA/image evidence,
- live smoke evidence,
- remaining caveats or follow-up issues.

Avoid saying “done” when only code merged but production was not verified and the task affects production behavior.
