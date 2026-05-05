# Agentic GitHub Project Workflow

This repository uses GitHub Issues + GitHub Projects as the source of truth for Hermes + Codex work.

The goal is to prevent coding agents from implementing only the literal technical slice of a user request while missing the usable product loop. A user-visible feature is not done when an endpoint returns `200`; it is done when the sender, receiver, history, error states, tests, and deployment checks satisfy the issue's acceptance criteria.

## Roles

- **User**: describes intent, constraints, priorities, and final acceptance.
- **Hermes**: controller/scheduler. It turns user intent into issue/project workflow, gates readiness, launches Codex modes, reviews results, and performs acceptance.
- **Codex Spec Mode**: writes or updates GitHub Issues. It does not change code, create branches, commit, or open PRs.
- **Codex Dev Mode**: develops only from a Ready for Dev issue, creates a branch/PR, runs tests, and writes execution results back to the issue.
- **GitHub Issue**: product requirement, bug, research task, refactor, regression, E2E task, or infra task.
- **GitHub Project**: scheduling/status board and agent lock surface.
- **PR**: code delivery linked to an issue.

## Project

Preferred Project name:

```text
Agentic Development
```

For this repository the initialization script records discovered project metadata in:

```text
docs/github-project-init.md
```

If GitHub Project API scopes are unavailable, the workflow still uses Issues, labels, templates, and docs; Project creation/field updates must be completed after refreshing `gh` with `project` and `read:project` scopes.

## Status Field

Single-select options:

- `Backlog`: captured but not yet specified.
- `Spec Drafting`: Codex Spec Mode or Hermes is drafting the issue.
- `Spec Ready`: spec is complete enough for Hermes review but not yet allowed for development.
- `Ready for Dev`: Hermes checked the spec gate and the issue may be assigned to Codex Dev Mode.
- `In Dev`: Codex Dev Mode is running or a development branch/PR is active.
- `Dev Done`: Codex completed implementation, tests, PR, and issue comment.
- `Accepted`: Hermes checked the PR/result against acceptance criteria.
- `Done`: issue is closed and the requirement is complete.
- `Blocked`: cannot proceed without dependency, information, failed CI, or environment access.
- `Need Human Review`: user/product decision required.
- `Rejected`: not planned or intentionally rejected.

Important:

```text
Spec Ready != Ready for Dev
```

A spec may be written by an agent, but Hermes must run the Spec Gate before setting `Ready for Dev`.

## Other Project Fields

Recommended MVP fields:

- `Type`: `Feature`, `Bug`, `Research`, `Refactor`, `Tech Debt`, `Regression`, `E2E`, `Docs`, `Infra`
- `Priority`: `P0`, `P1`, `P2`, `P3`
- `Agent Mode`: `Spec`, `Dev`, `Review`, `Research`, `E2E`
- `Need Research`: `Yes`, `No`, `Unknown`
- `Module`: single-select or text. Suggested values: `需求池`, `消息模块`, `文件与媒体模块`, `用户与会话模块`, `系统级测试`, `技术调研`, `基础设施`
- `Branch`: text
- `PR`: text
- `Codex Run ID`: text

## Labels

Required labels:

- `type:feature`
- `type:bug`
- `type:research`
- `type:refactor`
- `type:e2e`
- `type:regression`
- `agent:spec`
- `agent:dev`
- `agent:review`
- `status:blocked`
- `priority:p0`
- `priority:p1`
- `priority:p2`
- `priority:p3`
- `need:human-review`

Create/repair labels with:

```bash
python3 scripts/github-project/ensure_labels.py
```

## Normal Product Feature Flow

```text
User intent
  -> Hermes classifies as product/development work
  -> Codex Spec Mode creates/updates a Product Requirement issue
  -> Issue is added to GitHub Project
  -> Status = Spec Ready
  -> Hermes runs Spec Gate
  -> Status = Ready for Dev
  -> Hermes checks lock/dependencies and sets In Dev + Codex Run ID
  -> Codex Dev Mode reads the issue and implements the full stack loop
  -> Codex opens PR linked with Closes #<issue>
  -> Codex comments execution result and sets Dev Done
  -> Hermes checks acceptance criteria, tests, CI, E2E, deployment if required
  -> Status = Accepted
  -> merge/deploy/final verification
  -> Status = Done and issue closed
```

## Spec Gate

Hermes must not set an issue to `Ready for Dev` unless the issue body contains the following sections and they are non-empty:

- Background
- User Story or user impact
- Goals or expected behavior
- Non-goals for product features
- Functional Scope
- Interaction Flow for user-visible features
- Product Usability Requirements for user-visible features
- Data / API Impact when contracts/storage/protocols may change
- Edge Cases
- Acceptance Criteria
- Test Plan
- Dependencies, or an explicit `None`

Acceptance criteria must be concrete and user-visible. For user-facing features, they should normally include sender/actor behavior, receiver/target behavior, history/refresh behavior, failure states, and required tests.

Bad:

```md
- [ ] Support image sending.
```

Good:

```md
- [ ] Sender can select an image and sees an image bubble after send succeeds.
- [ ] Online receiver sees the image without refreshing.
- [ ] Reopening or refreshing the conversation still renders the historical image.
- [ ] Clicking the image opens a preview and downloading the original works.
- [ ] Upload/load failures show visible error states and do not fake success.
```

## Codex Spec Mode Rules

Spec Mode must:

- understand user intent;
- create or update a GitHub Issue;
- write product requirements, interaction flow, edge cases, acceptance criteria, and test plan;
- decide whether research is needed;
- create a Research Issue if uncertainty blocks specification;
- add the issue to the Project and set fields when API access is available.

Spec Mode must not:

- modify code;
- create development branches;
- commit;
- open PRs;
- mark an issue `Ready for Dev` unless explicitly acting as Hermes with the Spec Gate completed;
- over-split ordinary product work into frontend/backend/test micro-issues.

## Codex Dev Mode Rules

Dev Mode may only execute an issue if:

1. Project Status is `Ready for Dev`.
2. The issue is open.
3. `Codex Run ID` is empty.
4. No active branch/PR already owns the issue.
5. `Blocked by` dependencies are done or explicitly waived.
6. Acceptance Criteria and Test Plan are present.

Dev Mode must:

- read the issue body, comments, and linked docs;
- create a branch named like `feature/issue-<number>-<short-name>` or `fix/issue-<number>-<short-name>`;
- implement the complete full-stack product loop unless the issue explicitly scopes otherwise;
- add focused regression tests before or with the fix;
- run tests and static checks before commit, using the Codex commit 前验证门禁 in `docs/GIT_WORKFLOW.md`;
- for DB/schema/repository SQL changes, add executable `db/change_log/*.sql` and run PostgreSQL integration when a test DSN is available;
- open a PR with `Closes #<issue-number>`;
- comment the execution result on the issue;
- set `Status = Dev Done`, `Branch`, `PR`, and clear/set `Codex Run ID` according to the result.

Dev Mode must not:

- implement from the user's original one-line request instead of the issue;
- skip acceptance criteria;
- use mock/fallback fake success on real product paths;
- mark complete without tests or a documented reason tests could not run;
- split ordinary feature work into separate frontend/backend/test PRs unless the issue says to.

## Task Granularity

Default:

```text
One product requirement -> one full-stack issue -> one Codex Dev Mode run/PR
```

Split only when:

1. there is technical uncertainty that requires research first;
2. the requirement is too large for one Codex context;
3. sub-capabilities can be independently shipped with low integration cost;
4. several shipped features need a later system-level E2E issue;
5. architecture or product design needs human confirmation first.

Keep DAGs short. Prefer:

```text
Research Issue -> Full-stack Coding Issue
```

or:

```text
Feature A/B/C -> System E2E Regression Issue
```

Avoid long serial chains and frontend/backend/test micro-issues for ordinary product work.

## Dependencies

Use GitHub issue dependencies when available. If unavailable, put this in the issue body:

```md
## Dependencies

Blocked by:
- #123

Blocking:
- #456
```

Hermes must not dispatch an issue with incomplete `Blocked by` dependencies.

## Locking and Concurrency

Before launching Codex Dev Mode, Hermes must check:

- Status is still `Ready for Dev`;
- issue is open;
- `Codex Run ID` is empty;
- no active branch or PR is linked;
- dependencies are satisfied.

Then Hermes immediately sets:

- `Status = In Dev`
- `Codex Run ID = <run/session id>`

If the run fails:

- set `Status = Blocked` with a comment explaining root cause; or
- reset to `Ready for Dev` and clear `Codex Run ID` if retry is safe.

## Hotfix Exception

Hermes defaults to not writing product feature code directly. Exceptions are allowed for:

- GitHub Project/workflow/template initialization;
- docs/process changes;
- small CI/CD/script fixes;
- controller-side minor corrections after Codex output;
- P0/P1 production hotfixes.

For hotfixes:

1. Create or update a Bug/Hotfix issue first when practical.
2. Include minimal reproduction, impact, acceptance criteria, and test plan.
3. Allow `Ready for Dev` or direct Hermes fix if production recovery would otherwise be delayed.
4. Add regression test or production smoke verification.
5. Backfill missing issue details after recovery.

## Dev Done Issue Comment Template

```md
## Codex Execution Result

### Branch

`feature/issue-<issue-number>-<short-name>`

### Pull Request

PR: #<number>

### Completed

- [x] ...

### Acceptance Criteria Check

- [x] ...
- [ ] ... (if any, explain why not complete)

### Tests

Commands:

```bash
...
```

Result:

```text
...
```

### Risks

- ...

### Remaining Work

- ...
```

## PR Requirements

PR title:

```text
feat: implement <requirement name>
```

or:

```text
fix: resolve <bug name>
```

PR body must include:

```md
Closes #<issue-number>

## Summary

## Changes

## Tests

## Acceptance Criteria

## Risks
```

A merged PR is not automatically enough for `Done`. `Done` requires Hermes acceptance and any required deployment/production verification.

## Done Definition

An issue may move to `Done` only after:

- acceptance criteria are checked;
- required tests pass or skipped tests are explicitly justified;
- PR is merged or the task is docs/process-only and committed as intended;
- CI is green for the relevant SHA when code changed;
- deployment and runtime/E2E smoke are verified for production-impacting tasks;
- issue is commented with final evidence and closed.

## Product Usability Rule

For user-visible functionality, Hermes and Codex must ask:

```text
Can a real user complete the intended goal end to end?
```

For example, media sending is not done at `upload complete`. It is done when sender can send, receiver can view without refresh, history can render after refresh, preview/download works, permissions are respected, and failure states are visible.
