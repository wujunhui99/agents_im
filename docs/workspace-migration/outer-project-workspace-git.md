# Project workspace Git repository

The outer workspace `/home/ws/project` is a documentation/workspace repository.

## Tracked

- `AGENTS.md`
- `ARCHITECTURE.md`
- `docs/` Markdown documentation and lightweight generated docs
- `.gitignore`

## Not tracked

- `agents_im/` application source repository
- `worktrees/` Codex/Hermes feature worktrees
- `codex-tasks/` local task prompts and scratch task files
- vendored/reference code checkouts under `docs/references/*/`
- local runtime files, logs, env files, dependency caches

Application code stays in its own Git repository under `agents_im/`. The outer repo records planning, architecture, product, reliability, and agent-readable documentation only.
