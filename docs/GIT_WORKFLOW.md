# Git Workflow

适用场景：需要创建分支、worktree、commit、PR 或排查 Git/CI 门禁时读取本文。

本文维护 Git 操作和门禁细节；端到端流程见 [`AGENTS.md`](../AGENTS.md)。本项目不使用 `develop` 集成分支。

## Branch Model

- `main`: 唯一长期主分支和发布分支，改动通过 PR 合入。
- Task branch: `<type>/<agent-id>/issue-<number>-<task-desc>`，例如 `fix/codex/issue-123-login-error`。
- Current agent ids: `claude`, `codex`.
- Branch `type`: `feature`, `fix`, `refactor`, `docs`, `test`, `chore`, `ci`, `perf`, `style`, `hotfix`.

CI gate `scripts/ci/verify-agent-branch-name.sh` enforces the task branch format for PR builds.

## Worktree

Use an isolated worktree when running parallel or risky work:

```bash
git fetch origin
git worktree add \
  -b fix/codex/issue-123-login-error \
  .claude/worktrees/issue-123-login-error \
  origin/main
```

The `.claude/worktrees/` directory is ignored; do not add worktrees as gitlinks.

## Commit And PR Rules

Commit 与 PR 规则：

- Commit subject: `<type>(<scope>)[<agent-id>]: <short title>`.
- Required trailers: `Issue`, `Agent`, `Human-Owner`.
- PR target: `main`.
- PR body: exactly one `Closes #<issue>`, `Fixes #<issue>`, or `Resolves #<issue>`.
- Every development PR solves one Issue.

## Local Verification

Choose the relevant subset; do not blindly run expensive checks for read-only reproduction.

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name '*.go' -print)
go test ./...
npm --prefix web run test:run
npm --prefix web run build
bash scripts/verify-static.sh
git diff --check
```

For DB/schema/repository SQL changes, add a new `db/migrations/*.sql` and run:

```bash
AGENTS_IM_CONFIRM_TRUNCATE=1 scripts/verify-postgres-local.sh
```

If Docker or PostgreSQL is unavailable, report the blocker and run the closest static/unit checks.

## Drone Checks

`.drone.yml` runs `verification` on PRs targeting `main`.

Key steps:

- `detect changes`: always runs PR policy gates, computes the PR diff, and writes whether frontend, Markdown, and backend verification are required.
- `backend-verification`: exits successfully without backend work when the diff is only `web/` and `.md` files; otherwise runs go-zero API validation, Go formatting check, `go test ./...`, `docker compose config -q`, and static verification.
- `frontend-verification`: exits successfully without npm work unless a `web/` path changed; otherwise runs `npm --prefix web ci`, lint, tests, and build.
- `markdown-link-check`: exits successfully without link checking unless a `.md` path changed.
- `postgres-integration`: exits successfully without PostgreSQL work when the diff is only `web/` and `.md` files; otherwise runs migrations against an isolated PostgreSQL service and then `go test -tags=integration ./tests`.

`deploy-main` runs only after `main` receives a merge. It detects changed files, builds selected images, applies k3s manifests, and waits for rollout when deployment is required.

## Failure Handling

- Read the failing Drone step log before patching.
- Fix root cause in the same task branch when it is in scope.
- If deployment/runtime fails for infrastructure unrelated to the PR logic, open or update a separate deploy issue.
- Never convert broken API/config/dependency behavior into mock or fake success.
