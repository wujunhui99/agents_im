# Agentic Development Workflow

适用场景：任何需要从需求/Issue 推进到代码、验证、PR、Merge Queue、CI/部署结果报告的任务。

本项目当前采用单 Agent 执行流：一个任务由一个 Agent 从 Issue 做到 PR、验证和交付说明。GitHub Issue 是需求、范围和验收标准的事实源；GitHub Project 只作为可选看板。

## 标准流程

```text
User goal
  -> create or update one GitHub Issue
  -> create task branch from main
  -> implement in the task branch / worktree
  -> run required local verification
  -> commit and push when authorized
  -> open PR to main with Closes #<issue>
  -> Drone PR verification passes
  -> merge through GitHub Merge Queue
  -> main deploy pipeline runs when deployment is needed
  -> comment on the Issue with a short implementation summary
```

## Issue Requirements

For non-trivial product, bug, refactor, E2E, research, or infra work, the Issue should contain:

- background or user impact,
- scope and non-goals,
- acceptance criteria,
- test or verification plan,
- dependencies or `None`.

Acceptance criteria should be observable. User-visible work is not complete when an endpoint returns `200`; it is complete when the intended user flow works, history/refresh behavior is correct, errors are visible, and required checks pass.

## Branch And PR

分支、commit identity、trailers 和 PR body 细则见 [`docs/AGENT_GIT_STANDARD.md`](./AGENT_GIT_STANDARD.md)；Git 操作细节见 [`docs/GIT_WORKFLOW.md`](./GIT_WORKFLOW.md)。最低规则：

- Branch format: `<type>/<agent-id>/issue-<number>-<task-desc>`.
- Current agent ids: `claude`, `codex`.
- PR target branch: `main`.
- PR body must contain exactly one GitHub closing keyword, for example `Closes #123`.
- One development PR should solve one Issue.

## Agent Duties

- Read `AGENTS.md` first, then the task-specific docs from `docs/AGENT_TASK_GUIDE.md`.
- Keep scope tight; do not opportunistically refactor unrelated areas.
- No fake implementation, fake success, silent fallback, or production-path mock.
- Add or update tests when behavior changes.
- Run the requested checks and report any skipped checks with the reason.
- After solving an Issue, comment once with a brief implementation summary and verification result.

## Drone Result Check

Claude Code 后台执行 `scripts/drone-watch.sh`；Codex 前台执行或自行轮询后台日志，必须报告 Drone 结果。

Use Drone/GitHub checks as evidence, but do not treat a local self-report as acceptance when CI or deploy evidence is required.
