# Multi-Agent Branch, Commit, and Attribution Standard

本文档定义 `agents_im` 多 Agent 并行开发时必须遵守的分支、Git identity、commit message、CI/CD 归因和通知规范。目标是让 Drone/GitHub/Telegram 通知能稳定判断：**哪个 Agent 对当前 commit / CI 结果负责**。

## 1. 核心原则

1. **一个任务一个独立 worktree / branch**：每个 Agent 只能在分配给自己的 worktree 内工作，不共用同一个工作目录。
2. **一个 Issue 一个 PR**：开发 PR 默认只解决一个 GitHub Issue；PR body 必须包含且只包含一个 closing keyword，例如 `Closes #152`。
3. **merge 到 develop 即完成 Issue**：本项目把 PR merge 到 `develop` 作为 Issue completed 点；`main` 发布失败另开 release/deploy issue 追踪。
4. **Git author email 是第一归因来源**：即使所有 Agent 通过同一个 GitHub 账号 push，commit 本身必须带有该 Agent 的专用 Git identity。
5. **commit trailer 是第二归因来源**：每个 commit 必须包含 `Issue`、`Agent`、`Human-Owner` trailers。
6. **分支名必须在第二段带可信 Agent 名**：分支格式是 `<type>/<agent-name>/<issue>-<task-desc>`；CI 会硬性校验 `agent-name` 属于当前 Agent 团队。
7. **归因冲突时宁可不 @ 人**：如果 Git author email 与 `Agent:` trailer 不一致，CI/CD 必须标记 attribution mismatch，并避免自动 @ 任何 Agent。

## 2. 分支命名规范

格式：

```text
<type>/<agent-name>/<issue>-<task-desc>
```

字段规则：

- `type`：使用小写英文，允许值：`feature`、`fix`、`refactor`、`docs`、`test`、`chore`、`ci`、`perf`、`style`、`hotfix`。
- `agent-name`：**硬性必填，且必须是第二个路径段**。可信 Agent id 只有：`eino`、`helios`、`hermes`、`achilles`、`furies`、`gaia`。
- `issue`：必须使用 GitHub Issue 编号，格式为 `issue-<number>`。
- `task-desc`：简短英文 slug，单词用 `-` 分隔。
- 全部分支名使用小写，不使用空格、下划线或中文。

CI 硬门禁：`scripts/ci/verify-agent-branch-name.sh` 会在 Drone `backend-verification` 开始时校验 PR source branch。没有在第二段上报可信 `agent-name` 的分支直接失败。

示例：

```text
feature/eino/issue-131-admin-console-lists
fix/helios/issue-128-drone-postgres-url
docs/gaia/issue-140-agent-git-standard
ci/hermes/issue-142-drone-notifier-routing
```

推荐 worktree 创建方式：

```bash
git fetch origin
mkdir -p /home/ws/project/agents_im_worktrees

git worktree add \
  -b feature/eino/issue-131-admin-console-lists \
  /home/ws/project/agents_im_worktrees/eino-issue-131-admin-console-lists \
  origin/develop
```

## 3. Agent Git identity 规范

每个 Agent 必须使用自己的专用 Git author / committer identity。不要使用其他 Agent 的 identity，也不要用默认的人类 Git identity 冒充 Agent commit。

当前约定：

- `eino`：`Eino (AI Agent) <eino@agents.noreply.local>`
- `helios`：`Helios (AI Agent) <helios@agents.noreply.local>`
- `hermes`：`Hermes (AI Agent) <hermes@agents.noreply.local>`
- `achilles`：`Achilles (AI Agent) <achilles@agents.noreply.local>`
- `furies`：`Furies (AI Agent) <furies@agents.noreply.local>`
- `gaia`：`Gaia (AI Agent) <gaia@agents.noreply.local>`

提交示例：

```bash
GIT_AUTHOR_NAME="Eino (AI Agent)" \
GIT_AUTHOR_EMAIL="eino@agents.noreply.local" \
GIT_COMMITTER_NAME="Eino (AI Agent)" \
GIT_COMMITTER_EMAIL="eino@agents.noreply.local" \
git commit
```

也可以在各自 worktree 内设置 local config：

```bash
git config user.name "Eino (AI Agent)"
git config user.email "eino@agents.noreply.local"
```

## 4. Commit message 规范

Subject 格式：

```text
<type>(<scope>)[<agent-name>]: <short title>
```

规则：

- `type`：使用 Conventional Commit 类型：`feat`、`fix`、`refactor`、`docs`、`test`、`chore`、`style`、`perf`、`ci`。
- `scope`：建议填写受影响模块，例如 `auth`、`message`、`web`、`ci`、`docs`。
- `[agent-name]`：必填，必须和 Git identity / `Agent:` trailer 一致。
- title 简短明确，建议不超过 50 个英文字符。

示例：

```text
feat(web)[eino]: add admin console lists
fix(ci)[helios]: repair postgres integration dsn
docs(git)[gaia]: add multi-agent git standard
ci(notify)[hermes]: route drone failures to owner dm
```

## 5. Required trailers

每个 Agent commit 末尾必须包含：

```text
Issue: #<number>
Agent: <agent-name>
Human-Owner: junhui
```

可选 trailer：

```text
Reason: <why this commit exists>
Agent-Run: <session-or-task-id-if-available>
Co-authored-by: <name> <email>
```

规则：

- `Issue`、`Agent`、`Human-Owner` 是默认必填项。
- `Prompt-Version` 默认不使用，除非平台提供稳定且有意义的 prompt version。
- `Co-authored-by` 默认不填；只有确实存在共同作者时才填写。
- `Agent:` 必须与 subject 中的 `[agent-name]`、Git author email 对应的 Agent 一致。

完整示例：

```text
fix(ci)[helios]: repair postgres integration dsn

Use the Drone postgres service password in DATABASE_URL so the
postgres-integration job authenticates against the service correctly.

Issue: #128
Agent: helios
Human-Owner: junhui
Reason: fix Drone postgres-integration authentication failure
```

## 6. Issue / PR 生命周期规范

本项目采用 **one issue -> one PR -> develop merge closes issue**：

- 每个开发分支必须对应一个 GitHub Issue。
- 每个开发 PR 默认只解决一个 Issue；不要把多个 Issue 混在一个 PR 里。
- PR 目标分支通常是 `develop`；紧急生产 hotfix 例外可以指向 `main`。
- PR body 必须包含且只包含一个 GitHub closing keyword：

```text
Closes #<issue>
```

也允许使用 GitHub 支持的等价关键字：`Fixes #<issue>`、`Resolves #<issue>`。

语义约定：

- PR merge 到 `develop` = Issue completed，可以关闭 Issue。
- `main` 发布 / k3s deploy 属于 release 阶段；如果发布失败，另开 release/deploy issue，不重新打开已完成开发 Issue。
- Controller / CI bot 在关闭 Issue 前应评论完成摘要，包含 PR、commit、Agent、验证结果和 blockers。

推荐 Issue 完成评论格式：

```text
Completed via PR #<pr> / commit <sha>.

Agent: <agent>
Branch: <type>/<agent>/issue-<number>-<task>
Validation:
- <command/result>
Blockers: none
```

CI 硬门禁：`scripts/ci/verify-pr-issue-link.sh` 会在 PR verification 中读取 PR body，要求 exactly one closing issue keyword。缺失或出现多个 closing issue 会失败。

## 7. CI/CD 归因优先级

CI/CD、通知脚本和 reviewer 在判断负责 Agent 时，必须按以下优先级：

```text
1. Git author email
2. Commit trailer Agent:
3. Branch path <type>/<agent>/<issue>-<task>
4. PR label agent:<agent>
5. fallback to Junhui / human owner; do not auto-mention an agent
```

如果 Git author email 与 `Agent:` trailer 冲突：

- 标记 `attribution_mismatch=true`。
- 不自动 @ 任何 Agent。
- 通知中明确展示冲突来源，交由 human owner 或 controller 判断。

## 8. Drone / Telegram 通知文案规则

Drone 通知在 success 和 failure 都发送到 Telegram。通知脚本 `scripts/ci/drone-telegram-notify.py` 会解析负责 Agent 并在群里 @ 对应 bot。归因来源按当前硬门禁优先级：

```text
1. PR source branch path <type>/<agent>/<issue>-<task>
2. Commit trailer Agent:
3. Git author email
4. unresolved; do not auto-mention an agent
```

如果多个来源不一致，通知仍优先 @ 分支第二段对应的 Agent，但会展示 `Attribution mismatch` warning，方便 controller 追责。

Drone 通知中要区分两类 step：

- **Failed step / Upstream step**：真正失败的 pipeline step，例如 `postgres-integration`。
- **Notification step**：发送 Telegram 通知的当前步骤，例如 `notify telegram`。

不要把 Notification step 当成失败根因。通知文案应展示：

```text
CI failed ❌
Project: agents_im
Issue: #128
Branch: fix/helios/issue-128-drone-postgres-url
Commit: d920bb8
Author: Helios (AI Agent) <helios@agents.noreply.local>
Agent: helios @ws_ubuntu_claw_bot
Failed step: postgres-integration
Notification step: notify telegram
Logs: <drone build url>
```

## 9. Agent prompt 摘要

给任何 Agent 派活时，必须包含或引用以下规则：

```text
You must follow the agents_im multi-agent Git standard:
1. Work only inside your assigned worktree.
2. Branch name must be <type>/<agent-name>/<issue>-<task-desc>; the second path segment must be a trusted team agent id: eino, helios, hermes, achilles, furies, or gaia. CI rejects branches without this agent-name segment.
3. Every commit subject must be <type>(<scope>)[<agent-name>]: <short title>.
4. Every commit must use your dedicated Git author/committer name and email.
5. Every commit must include Issue, Agent, and Human-Owner trailers.
6. Do not fill Prompt-Version by default.
7. Do not fill Co-authored-by unless there is a real co-author.
8. Do not use another agent's Git identity.
9. Every PR must solve exactly one GitHub Issue and include exactly one closing keyword in the PR body, e.g. Closes #123.
10. PR merge to develop means the Issue is completed; main deploy failures should open a separate release/deploy issue.
```

## 10. Short checklist

提交前检查：

- [ ] 当前 worktree 只属于一个任务 / Issue。
- [ ] 分支名符合 `<type>/<agent-name>/<issue>-<task-desc>`。
- [ ] `git config user.name` / `user.email` 是当前 Agent 的 identity。
- [ ] commit subject 包含 `[agent-name]`。
- [ ] commit trailers 包含 `Issue`、`Agent`、`Human-Owner`。
- [ ] PR body 包含且只包含一个 closing keyword，例如 `Closes #152`。
- [ ] `Agent:` 与 author email、branch path、subject 中的 Agent 一致。
- [ ] 已按改动范围运行验证，并把命令与结果写入 Issue/PR。
