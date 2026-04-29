# AGENTS.md

本文件是 AI Agent 在本仓库中工作的入口地图。不要把它当成百科全书；详细知识应放到外层 workspace 文档中，并在这里保留链接。

## 工作区结构

- 应用代码仓库：`/home/ws/project/agents_im`
- 当前 feature worktree 示例：`/home/ws/project/worktrees/user-service`
- 项目文档根目录：`/home/ws/project`
- 顶层架构：`/home/ws/project/ARCHITECTURE.md`
- 产品规格：`/home/ws/project/docs/product-specs/`
- 设计文档：`/home/ws/project/docs/design-docs/`
- 执行计划：`/home/ws/project/docs/exec-plans/`

## 核心原则

- 人类负责目标、约束和验收标准；Agent 负责规划、实现、验证和修复。
- 仓库和文档是记录系统。重要上下文必须写入 Markdown，不能只停留在聊天记录中。
- 变更前先阅读本文件和相关外层文档；变更后必须自测并记录验证方式。
- 使用 go-zero 作为 Go 微服务框架。
- `user` 服务先开发；`auth`、`friends`、`groups` 在 `user` 基础接口稳定后可并行开发。

## 必读文档

- `/home/ws/project/AGENTS.md`
- `/home/ws/project/ARCHITECTURE.md`
- `/home/ws/project/docs/PLANS.md`
- `/home/ws/project/docs/product-specs/account-social-core.md`
- `/home/ws/project/docs/design-docs/user-auth-friends-groups-boundaries.md`

## Agent 执行流程

必须按三阶段推进：

1. Planner：生成或完善需求文档、实现文档和任务 Task 文档。
2. Generator：按 Planner 文档实现代码，并完成自测。
3. Evaluator：检查实现、测试和文档一致性；如有问题，修复后再验证。

## Git 工作流

```text
feature/* -> develop -> main
```

- 每个 Agent 使用独立 `git worktree`。
- 功能分支先合并到 `develop`，不得直接合并到 `main`。
- 提交使用仓库配置：`junhui <344686925@qq.com>`。

## 当前任务要求

先开发 `user-rpc` 和 `user-api`：

- `user-rpc` 负责用户资料权威能力。
- `user-api` 对外提供 HTTP 接口。
- `user` 不保存密码、不验证密码、不维护好友/群成员关系。
- 第一阶段至少覆盖：创建用户资料、按唯一标识查询是否存在、查询公开资料、`/me`、更新自己的资料。
