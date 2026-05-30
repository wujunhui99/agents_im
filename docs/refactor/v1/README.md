# Refactor v1 — 重构任务完成表

本目录是 **v1 轮重构**的分析与决策文档集合。后续若有新一轮重构，另建 `docs/refactor/v2/`。

- [`00-decisions.md`](./00-decisions.md) 是跨文档决策的**唯一仲裁源**（D1~D10），不是重构任务本身。
- 下表 8 项（01~08）为本轮重构任务，逐项跟踪完成情况。

## 状态说明

| 状态 | 含义 |
|------|------|
| `Pending`  | 待办 / 未开始（含进行中，完成前都算 Pending） |
| `Done`     | 完成 |
| `Deferred` | 延后 |
| `Dropped`  | 废弃，不重构 |

## 完成表

| #  | 任务 | 文档 | 状态 | 备注 |
|----|------|------|------|------|
| 01 | 项目结构 / 目录结构 重构 | [`01-project-structure.md`](./01-project-structure.md) | `Pending` | 进行中，stage 进度见 `docs/exec-plans/active/refactor/01-project-structure/` |
| 02 | Auth / User / Friends / Groups 微服务技术债 | [`02-microservices.md`](./02-microservices.md) | `Pending` | |
| 03 | 消息转发机制重构（基于 OpenIM 实现版） | [`03-message-pipeline.md`](./03-message-pipeline.md) | `Pending` | |
| 04 | Agent 模块重构 | [`04-agent.md`](./04-agent.md) | `Pending` | |
| 05 | 可观测性 / Drone CI / 部署 重构 | [`05-observability-cicd.md`](./05-observability-cicd.md) | `Pending` | |
| 06 | 其他横切技术债 | [`06-cross-cutting.md`](./06-cross-cutting.md) | `Pending` | |
| 07 | msg-rpc 边界对比与改造 | [`07-msg-rpc-redesign.md`](./07-msg-rpc-redesign.md) | `Pending` | |
| 08 | 前端重构 | [`08-frontend.md`](./08-frontend.md) | `Pending` | |

> 维护规则：任务状态变化时更新本表「状态」列；具体执行仍以对应 GitHub Issue 为单一事实源。
