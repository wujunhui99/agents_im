# Codex Worker Handoff Contract

Use this document when Hermes dispatches Codex or another implementation worker. A worker should be able to read this file plus the issue and know how to produce a reviewable handoff.

## Required prompt fields

Every Codex task prompt should include:

```text
Issue: #<number> <URL>
Branch: <type>/<agent>/issue-<number>-<short-desc>
Worktree: <absolute path>
Base branch: origin/develop or origin/main
Allowed git operations: commit? push? open PR?
Scope: what files/features are in scope
Out of scope: what must not be changed
Required docs to read: AGENTS.md plus task-specific docs
Acceptance criteria: observable behavior, tests, docs
Required verification commands: exact commands or best-effort fallback
Handoff requirements: issue comment, commit trailers, session/token info if available
```

If the prompt does not explicitly allow push or PR creation, Codex should not push.

## Standard implementation rules

1. Read `AGENTS.md` first.
2. Read task-specific docs before editing.
3. For go-zero work, read `docs/design-docs/go-zero-service-layout.md` and the `.claude/skills/refactor-domain-to-service/` skill.
4. For frontend work, read `.claude/skills/frontend-skills/`, `docs/FRONTEND.md`, and `docs/product-specs/frontend-backend-contract.md`.
5. For deployment/observability work, read `deploy/README.md`, `docs/GIT_WORKFLOW.md`, `docs/RELIABILITY.md`, `.drone.yml`, and relevant scripts under `scripts/ci/`.
6. Never invent a fake implementation or silent fallback.
7. Never expose secrets, tokens, DSNs, cookies, server SSH details, or real credentials. Use `[REDACTED]` in notes.
8. Keep scope tight. Do not opportunistically refactor unrelated areas.
9. If verification is impossible because a dependency is unavailable, state the blocker and run the closest static/unit checks.

## Test-first expectations

For behavior changes:

- Add or update a failing test first.
- Run the focused test and confirm it fails for the expected reason.
- Implement the minimal change.
- Re-run the focused test and then broader checks.

For bugs:

- Reproduce or explain why reproduction is unavailable.
- Add a regression test where practical.
- Do not patch blindly before understanding the root cause.

For docs-only changes:

- No product test is required, but run static verification when it checks docs or links.

## Commit convention

Use the project agent identity and trailers:

```bash
git config user.name "Hermes (AI Agent)"
git config user.email "hermes@agents.noreply.local"
```

Commit subject format:

```text
<type>(<scope>)[<agent>]: <short title>
```

Commit body trailers:

```text
Issue: #<number>
Agent: <AgentName>
Human-Owner: wujunhui99
```

Examples:

```text
feat(feedback)[hermes]: polish user feedback form
fix(message)[codex]: preserve duplicate message sends
```

## Required Issue completion comment

Before handoff, comment on the GitHub Issue. Keep it concise but complete.

### Feature comment shape

```text
Implemented in <branch/commit/PR>.

User-visible behavior:
- ...

Key files / data flow:
- ...

Verification:
- `command` -> result
- `command` -> result

Blockers / caveats:
- None, or list clearly.
```

### Bug comment shape

```text
Root cause:
- ...

Fix:
- ...

Regression coverage:
- ...

Verification:
- ...

Branch/commit/PR:
- ...

Blockers:
- ...
```

### Research comment shape

```text
Conclusion:
- ...

Evidence:
- files/commands/links

Options and tradeoffs:
- ...

Recommendation:
- ...

Risks/open questions:
- ...
```

## Worker final report to Hermes

When Codex finishes, report these fields:

- issue number and title,
- branch and worktree,
- commit SHA,
- PR URL if created,
- tests/commands run,
- any failing/skipped checks and why,
- files changed summary,
- blockers or uncertainty,
- whether the Issue comment was posted.

## What not to do

- Do not close the Issue unless explicitly instructed. Hermes closes after integration/verification.
- Do not promote `develop -> main`; Hermes handles production release by default.
- Do not edit secrets or print secret values.
- Do not turn broken API calls into mock/demo fallback.
- Do not delete Kubernetes resources or run destructive infra commands unless the task explicitly allows it.
- Do not widen a feature into multiple unrelated fixes.

## Example dispatch prompt

```text
You are Codex implementing agents_im Issue #123.

Branch/worktree:
- Worktree: /home/ws/project/worktrees/example
- Branch: feat/codex/issue-123-example
- Base: origin/develop
- You may commit and push. Open a PR against develop when done.

Read first:
- AGENTS.md
- docs/agent-ops/hermes-codex-operating-model.md
- docs/agent-ops/codex-worker-handoff.md
- <task-specific docs>

Scope:
- Implement only Issue #123.
- Do not change deployment config.

Acceptance:
- <criteria>

Verification:
- <focused test>
- bash scripts/verify-static.sh

Handoff:
- Commit with project trailers.
- Comment on Issue #123 with summary, tests, PR, blockers.
- Report branch/worktree/commit/PR/tests/blockers to Hermes.
```
