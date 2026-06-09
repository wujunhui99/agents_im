# AGENTS.md

本文件是 AI Agent(Codex,Claude Code) 在本仓库的唯一共享入口,本文件应保持为**项目目录 + 必要约束**。

## 工作流

- 端到端流程以 [`docs/AGENTIC_DEVELOPMENT_WORKFLOW.md`](./docs/AGENTIC_DEVELOPMENT_WORKFLOW.md) 为准：Issue -> `main` 任务分支/必要时独立 worktree -> 实现与验证 -> commit -> PR -> GitHub Merge Queue -> CI/部署/回归验证。
- 纯文档改动可按任务授权免 Issue、免独立 worktree、免产品回归；仍走任务分支、PR、CI/Merge Queue。
- 禁止直接 commit/push/merge 到 `main`；push、开 PR、merge 必须有任务明确授权。
- 分支、commit、PR 规则见 [`docs/AGENT_GIT_STANDARD.md`](./docs/AGENT_GIT_STANDARD.md)：分支第二段必须是可信 Agent 名；每个开发 PR 只解决一个 Issue，PR body 包含 `Closes #<issue>`；commit 使用 Agent identity、规范 subject 和 trailers。
- 解决 GitHub Issue 后必须评论一次，简要说明实现方式。
- Claude Code 后台执行 `scripts/drone-watch.sh`；Codex 前台执行或自行轮询后台日志，必须报告 Drone 结果。

## 自我进化

- 改了项目就改文档：结构、入口、接口、流程、部署或验证方式变更时，同一 PR 内更新受影响文档。
- 执行中发现入口文档、专题文档或 skill 文档与实际代码/流程不一致时，按任务范围修正事实源；skill 文档只在任务明确涉及该 skill 时修改。
- 任务后有对应 skill 就考虑改进，没有就考虑是否创建；修改文档时顺手精简冗长或重复描述。
- `AGENTS.md` 是唯一共享入口事实源；`CLAUDE.md` 只能保留一行 `@AGENTS.md`。

## 必须遵守

1. **禁止假实现**：业务/API/持久化/消息链路不能用 mock、stub、硬编码、空实现、假成功冒充；mock 只允许在测试 fixture、demo/mock mode 或明确视觉占位中使用。
2. **失败优先**：接口不通、配置缺失、依赖不可用、测试失败时显式失败并报告根因假设，不能静默 fallback。
3. **根因优先**：修复前先复现、读完整错误、追踪数据流；不要未理解原因就堆补丁。
4. **验证优先**：声称完成必须给出可重复命令；没有真实启动/请求时，只能说 contract/unit/static verification。
5. **敏感信息脱敏**：不要输出 token、JWT、密码、cookie、DSN、访问凭据等敏感值；统一写 `[REDACTED]`。
6. **文档按需读取**：先读本文件，再按任务类型读 [`docs/AGENT_TASK_GUIDE.md`](./docs/AGENT_TASK_GUIDE.md)；不要一次性读完整 `docs/`。
7. **数据库变更**：schema/data 变更必须新增 `db/migrations/*.sql`；已发布 migration 不可变。

## 项目目录

- 项目概览：`agents_im` 是 Go/go-zero + React/Vite 的实时 IM 系统，覆盖账号、社交、消息、WebSocket、媒体和 Agent/AI runtime。
- 任务专题与验证：[`docs/AGENT_TASK_GUIDE.md`](./docs/AGENT_TASK_GUIDE.md)
- 架构/开发：[`ARCHITECTURE.md`](./ARCHITECTURE.md)、[`docs/DEVELOPMENT.md`](./docs/DEVELOPMENT.md)、[`docs/design-docs/index.md`](./docs/design-docs/index.md)、[`docs/product-specs/index.md`](./docs/product-specs/index.md)
- 流程/协作：[`docs/AGENTIC_DEVELOPMENT_WORKFLOW.md`](./docs/AGENTIC_DEVELOPMENT_WORKFLOW.md)、[`docs/AGENT_GIT_STANDARD.md`](./docs/AGENT_GIT_STANDARD.md)、[`docs/GIT_WORKFLOW.md`](./docs/GIT_WORKFLOW.md)
- 质量/产品/部署：[`docs/FRONTEND.md`](./docs/FRONTEND.md)、[`docs/PRODUCT_SENSE.md`](./docs/PRODUCT_SENSE.md)、[`docs/SECURITY.md`](./docs/SECURITY.md)、[`docs/RELIABILITY.md`](./docs/RELIABILITY.md)、[`deploy/README.md`](./deploy/README.md)
