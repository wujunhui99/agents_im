# Hermes / Codex Operating Model

This document records the durable project operating model for `agents_im`. It exists because Codex workers do not automatically inherit Hermes chat memory, skills, or prior controller context. Codex should read this before implementing non-trivial GitHub Issues.

## Roles

### Human owner

- Owns product direction, priorities, constraints, and final acceptance.
- May correct architecture or product direction at any time.
- Does not need to repeat project conventions if they are versioned in this repo.

### Hermes controller

Hermes is the default architect, integrator, reviewer, and release owner.

Hermes responsibilities:

1. Clarify product intent only when ambiguity changes implementation direction.
2. Create or refine one GitHub Issue per implementation unit.
3. Define acceptance criteria, contracts, risk boundaries, and verification expectations.
4. Dispatch Codex workers or specialized agents when implementation can be isolated.
5. Review worker diffs and reject self-reported completion without evidence.
6. Run or verify tests, static checks, integration checks, and production smoke where relevant.
7. Merge feature PRs to `develop`, then promote `develop -> main` for production release.
8. Comment concise verification evidence on the Issue before closing.
9. Update docs when the task discovers a reusable rule, pitfall, or operating convention.

Hermes should not be the only code writer. For routine feature work, Hermes should prefer planning, dispatch, review, and integration over directly editing every file.

### Codex worker

Codex is the default implementation worker for one issue-scoped task.

Codex responsibilities:

1. Read `AGENTS.md` and the task-specific docs it points to.
2. Work in the assigned branch/worktree only.
3. Implement exactly one GitHub Issue unless explicitly told otherwise.
4. Follow test-first behavior for bugs and feature behavior changes.
5. Keep production code real; no fake success, stubbed business behavior, or silent fallback.
6. Run the requested verification commands.
7. Commit using the repo commit convention and trailers.
8. Push/open/update a PR only if the task explicitly allows it.
9. Add one concise completion comment to the Issue before handoff.
10. Report session id, branch, worktree, commit, PR, tests, blockers, and any deviations.

Codex exit code `0` is not acceptance. Hermes still performs controller-side review.

### Specialized agents

Specialized agents may be added when the scope is long-lived and separable:

- Flutter/mobile agent: `apps/flutter_client` only; owns mobile UI, local cache, mobile WebSocket lifecycle, and Android/iOS/Web client builds.
- QA/dogfood agent: browser/live reproduction, screenshots, console/network evidence, E2E reports, and production smoke. It should not change code unless explicitly assigned a fix.
- DevOps/observability agent: Drone, k3s, Grafana, Prometheus, Loki, Tempo, deployment scripts, and live infrastructure checks.
- Backend agent: Go/go-zero/API/DB changes within an issue-scoped branch.
- Frontend agent: React Web / Management System UI within an issue-scoped branch.

Specialized agents are not free-floating architects by default. Hermes remains the integration and release owner unless the human explicitly reassigns that role.

## Default workflow

```text
Human goal
  -> Hermes creates/refines Issue + acceptance criteria
  -> Hermes dispatches Codex/specialized worker(s)
  -> Worker implements in isolated worktree/branch
  -> Worker tests, commits, pushes, opens PR, comments handoff
  -> Hermes reviews diff + runs targeted verification
  -> PR merges to develop
  -> develop promotes to main when production release is required
  -> Hermes verifies deployed SHA/rollout/smoke
  -> Hermes comments evidence and closes Issue
```

## One issue, one implementation PR

- A normal development PR should solve exactly one GitHub Issue.
- The PR body should contain exactly one closing keyword for that Issue, for example `Closes #123`.
- Merge to `develop` is integration completion, not production completion.
- Production completion requires `develop -> main` promotion and live verification when the task affects deployed behavior.

## Worktree rules

- Use one independent worktree per worker branch.
- Branch format: `<type>/<agent>/issue-<number>-<short-desc>`.
- Allowed agent names are governed by `docs/AGENT_GIT_STANDARD.md`.
- Do not reuse a dirty or conflicted worktree for a new task.
- Do not start from stale local branches; fetch the target base first.

## Evidence hierarchy

Accept evidence in this order:

1. Controller-side command output and live checks.
2. CI/Drone/GitHub checks for the relevant SHA.
3. Worker-run test output if reproducible and tied to the commit.
4. Worker self-report only as a lead, never as final acceptance.

## Memory vs repository docs

Use repository docs for durable project knowledge that Codex must know. Use Hermes memory only for compact cross-session facts, not detailed procedures.

Put in repo docs:

- workflow rules,
- contract decisions,
- deployment and verification pitfalls,
- design system conventions,
- reusable Codex prompt requirements,
- project architecture decisions.

Do not put stale one-off artifacts in durable docs:

- transient PR numbers,
- commit SHAs,
- temporary session ids,
- “phase N done” notes,
- obsolete incident details that will be false in a week.

## Direction corrections

When the human corrects direction after work has started:

1. Stop affected workers quickly.
2. Inspect their worktree status/diff/log.
3. Delete or quarantine wrong-direction branches if pushed.
4. Restart from the corrected baseline and contract.
5. Report discarded work as discarded, not completed.

## When to update these docs

Update this `docs/agent-ops/` set when a task reveals a reusable rule or repeated pitfall. Prefer small patches over large rewrites so future agents can see what changed.
