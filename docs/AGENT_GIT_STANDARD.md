# Agent Branch, Commit, and PR Standard

适用场景：需要创建或校验 Agent 分支名、commit subject/trailers、Git identity、PR body 或 CI 归因时读取本文。

本文档定义当前单 Agent 工作流下的分支、commit、PR 和 CI 归因规则。

## Branch

Format:

```text
<type>/<agent-id>/issue-<number>-<task-desc>
```

Rules:

- `type`: `feature`, `fix`, `refactor`, `docs`, `test`, `chore`, `ci`, `perf`, `style`, `hotfix`.
- `agent-id`: current allowed values are `claude` and `codex`.
- `issue`: must be `issue-<number>`.
- `task-desc`: lowercase English slug, separated by `-`.

Examples:

```text
docs/claude/issue-401-agent-docs-cleanup
fix/codex/issue-402-media-upload-error
```

CI enforces this through `scripts/ci/verify-agent-branch-name.sh`.

## Git Identity

Use the identity for the tool doing the work:

```text
Claude Code (AI Agent) <claude@agents.noreply.local>
Codex (AI Agent) <codex@agents.noreply.local>
```

Set it locally in the task worktree:

```bash
git config user.name "Codex (AI Agent)"
git config user.email "codex@agents.noreply.local"
```

## Commit Message

Subject:

```text
<type>(<scope>)[<agent-id>]: <short title>
```

Required trailers:

```text
Issue: #<number>
Agent: <agent-id>
Human-Owner: junhui
```

Example:

```text
docs(agent)[codex]: align workflow docs

Issue: #401
Agent: codex
Human-Owner: junhui
```

## PR

- PR target is `main`.
- PR body must contain exactly one closing keyword, for example `Closes #401`.
- A development PR should solve exactly one Issue.
- Merge only through GitHub Merge Queue.
- After solving an Issue, comment once with a short implementation summary.

`scripts/ci/verify-pr-issue-link.sh` enforces the one-closing-keyword rule in Drone PR verification.

## Checklist

- [ ] Branch is `<type>/<agent-id>/issue-<number>-<task-desc>`.
- [ ] Commit subject contains `[agent-id]`.
- [ ] Commit trailers include `Issue`, `Agent`, and `Human-Owner`.
- [ ] PR targets `main` and contains exactly one closing keyword.
- [ ] Verification commands and Drone result are reported.
