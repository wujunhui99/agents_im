# CLAUDE.md

Claude Code 专属指令，与 `AGENTS.md` 并列加载，后者约束优先。

## 工作流

**交付顺序**：创建 issue → 从 `main` 加 git worktree（在其中建 issue 对应分支）→ 提交 → PR → 立即 merge → Drone CI → 回归测试，部署成功才算完成。

- 分支：`fix/claude/<issue-N>-<short-desc>`，从 `main` 切出
- worktree：放仓库内 `.claude/worktrees/<branch>`，`git worktree add -b <branch> .claude/worktrees/<branch> main`
- Drone token：`secret/drone_token`，服务器：`https://drone.agenticim.xyz`
- CI 监控、回归测试步骤详见 `.claude/skills/fix-api/SKILL.md`

## 自我进化

- **改了项目就改文档**：结构 / 入口 / 接口 / 流程一旦变更，同一 PR 内同步更新受影响文档，不留过期描述（如下线 cmd 入口必须同步改 docs）
- 执行中发现 skill / 文档失真或不合适 → 立即修改
- 任务后 → 有对应 skill 考虑改进，没有考虑创建
- 修改文档时发现不简洁 → 顺手精简

## 文档索引

| 文档 | 内容 |
|------|------|
| [`AGENTS.md`](./AGENTS.md) | 项目总规范：禁止事项、工作流、Git 规范、快速导航 |
| [`ARCHITECTURE.md`](./ARCHITECTURE.md) | 系统架构总览：服务拓扑、技术栈、数据流 |
| [`docs/DEVELOPMENT.md`](./docs/DEVELOPMENT.md) | 本地开发环境搭建与启动 |
| [`deploy/README.md`](./deploy/README.md) | 部署流程：K8s、Drone CI、secrets 初始化 |
| [`docs/AGENT_GIT_STANDARD.md`](./docs/AGENT_GIT_STANDARD.md) | Agent 分支命名、commit 格式、PR 规范 |
| [`docs/GIT_WORKFLOW.md`](./docs/GIT_WORKFLOW.md) | Git 工作流：分支策略、merge queue |
| [`.claude/skills/fix-api/SKILL.md`](./.claude/skills/fix-api/SKILL.md) | 后端接口报错修复流程：诊断、扫漏洞、PR、CI、回归 |
