# AGENTS.md

本文件是 AI Agent 在本仓库中工作的入口地图。不要把它当成百科全书；详细知识应放到 `docs/` 或专题文档中，并在这里保留链接。

## 核心原则

- 人类负责目标、约束和验收标准；Agent 负责规划、实现、验证和修复。
- 仓库是记录系统。重要上下文必须写入仓库，不能只停留在聊天记录或口头约定中。
- 优先使用渐进式披露：先读本文件，再按任务类型读取更具体的文档。
- 复杂任务必须产出可版本化的计划、决策记录和验证结果。
- 变更前先理解现有结构，变更后必须自测并记录验证方式。

## 严格执行规则

1. **禁止假实现**：不得用 mock、stub、硬编码、空实现、假成功返回、跳过错误、仅改测试等方式冒充真实能力。mock 只能用于明确标注的测试 fixture、demo/mock mode 或视觉占位；凡是业务/API/持久化/消息链路能力，都必须接真实实现或在计划中明确标为未完成。
2. **失败优先 / Fail-first**：A visible error is better than a hidden incorrect success. 每次任务执行只能真实成功或明确失败；遇到错误、异常、测试失败、接口不通、依赖缺失时，必须优先暴露并定位失败，不能用兜底逻辑吞掉错误、不能静默降级成 mock、不能为了让流程继续而返回成功。如果错误在当前上下文无法解决，必须以明确失败结束并报告已验证事实、根因假设和阻塞点。可恢复错误也必须记录原因、边界和验证方式。
3. **根因优先**：修复失败前先复现、读完整错误、追踪数据流并形成根因假设；禁止未理解原因就堆叠补丁。
4. **验证优先**：任何声称完成的实现都必须有可重复验证命令。没有启动真实依赖/服务并实际请求时，不得声称端到端成功，只能说明为 contract/proxy/unit verification。

## 快速导航

- 项目架构总览：[`ARCHITECTURE.md`](./ARCHITECTURE.md)
- Codex 高密度项目上下文：[`docs/CODEX_CONTEXT.md`](./docs/CODEX_CONTEXT.md)
- 设计文档索引：[`docs/design-docs/index.md`](./docs/design-docs/index.md)
- Account Service 术语与 V0 compatibility：[`docs/design-docs/account-service-terminology.md`](./docs/design-docs/account-service-terminology.md)
- 账号/认证/好友/群聊服务边界：[`docs/design-docs/user-auth-friends-groups-boundaries.md`](./docs/design-docs/user-auth-friends-groups-boundaries.md)
- Agent-first 核心理念：[`docs/design-docs/core-beliefs.md`](./docs/design-docs/core-beliefs.md)
- 产品规格索引：[`docs/product-specs/index.md`](./docs/product-specs/index.md)
- 前后端 MVP 交接契约：[`docs/product-specs/frontend-backend-contract.md`](./docs/product-specs/frontend-backend-contract.md)
- 账号社交基础产品规格：[`docs/product-specs/account-social-core.md`](./docs/product-specs/account-social-core.md)
- Account Service 第一阶段产品规格：[`docs/product-specs/user-service.md`](./docs/product-specs/user-service.md)
- 消息链路产品规格：[`docs/product-specs/message-chain.md`](./docs/product-specs/message-chain.md)
- 消息链路接口契约：[`docs/design-docs/message-chain-contract.md`](./docs/design-docs/message-chain-contract.md)
- Account Service go-zero 实现设计：[`docs/design-docs/user-service-go-zero.md`](./docs/design-docs/user-service-go-zero.md)
- 本地开发启动说明：[`docs/DEVELOPMENT.md`](./docs/DEVELOPMENT.md)
- 部署说明：[`deploy/README.md`](./deploy/README.md)
- 执行计划规范：[`docs/PLANS.md`](./docs/PLANS.md)
- 活跃执行计划：[`docs/exec-plans/active/`](./docs/exec-plans/active/)
- 已完成执行计划：[`docs/exec-plans/completed/`](./docs/exec-plans/completed/)
- 技术债追踪：[`docs/exec-plans/tech-debt-tracker.md`](./docs/exec-plans/tech-debt-tracker.md)
- 可靠性要求：[`docs/RELIABILITY.md`](./docs/RELIABILITY.md)
- 安全要求：[`docs/SECURITY.md`](./docs/SECURITY.md)
- 质量评分：[`docs/QUALITY_SCORE.md`](./docs/QUALITY_SCORE.md)
- Git 工作流：[`docs/GIT_WORKFLOW.md`](./docs/GIT_WORKFLOW.md)
- 前端约定：[`docs/FRONTEND.md`](./docs/FRONTEND.md)
- 产品判断原则：[`docs/PRODUCT_SENSE.md`](./docs/PRODUCT_SENSE.md)

## 项目一句话介绍

本项目是一个高性能、分布式、实时聊天系统，采用微服务架构，提供 IM 核心能力与 Agent 服务能力，支持 Agent 生命周期管理、Agent 单聊和 Agent 群聊。

## 当前技术栈

- 后端：Go / Python
- 通信：gRPC / WebSocket / Webhook
- 存储：PostgreSQL / Redis
- 消息：Kafka
- Agent 框架：CloudWeGo Eino（当前提供 DeepSeek ChatModel adapter/config；runtime orchestration 待实现）
- Python API：FastAPI
- 可观测性：Prometheus / Grafana / Jaeger
- CI/CD：GitHub Actions；`main` 分支触发部署 workflow，也支持手动 `workflow_dispatch`；deploy workflow 包含 `detect-changes`，可区分完整构建部署、config-only deploy 和文档-only 不部署。
- `deploy-k3s.sh` 支持 config-only 开关：`SKIP_SET_IMAGE`、`SKIP_MIDDLEWARE`、`SKIP_MIGRATIONS`、`ROLLOUT_SERVICES`、`RESTART_ROLLOUT`。
- 部署：k3s 承载应用工作负载，Docker Compose 承载 PostgreSQL / Redis / Redpanda 中间件
- 镜像仓库：GHCR

## Agent 执行流程

1. Planner：生成需求文档、实现文档和执行计划。
2. Generator：按计划实现代码，并完成自测。
3. Evaluator：检查代码、测试、设计一致性和潜在缺陷；如有问题，推动修复。

详细规范见 [`docs/PLANS.md`](./docs/PLANS.md)。

## Git 工作流

本项目采用支持多 Agent 并行开发的 `git worktree` 工作流：

```text
feature/* -> develop -> main
```

- 每个 Agent 开发时必须使用独立 `git worktree` 启动新的工作实例。
- 功能分支先合并到 `develop`，不得直接合并到 `main`。
- `develop` 用于集成多个 feature 分支，并处理跨功能冲突。
- `develop` 通过集成测试后，再合并到 `main`。

详细规范见 [`docs/GIT_WORKFLOW.md`](./docs/GIT_WORKFLOW.md)。

## 工作要求

- 新增或修改重要行为时，同步更新相关文档。
- 发现假实现、静默兜底、吞错、只为通过测试而绕过真实逻辑时，必须立即改为失败优先并补充验证。
- 涉及架构变更时，更新 `ARCHITECTURE.md` 或 `docs/design-docs/`。
- 涉及产品行为时，更新 `docs/product-specs/`。
- 涉及复杂任务时，在 `docs/exec-plans/active/` 下创建执行计划。
- 完成任务后，将执行计划移到 `docs/exec-plans/completed/`，并补充结果与验证记录。
- PR/MR 描述必须包含测试结果与风险说明。
- 前端联调相关变更必须同步检查 [`docs/product-specs/frontend-backend-contract.md`](./docs/product-specs/frontend-backend-contract.md)、[`docs/DEVELOPMENT.md`](./docs/DEVELOPMENT.md)、`scripts/dev-up.sh`、`scripts/dev-demo-data.sh` 和 `tests/mvp_backend_test.go`。

## Frontend AI Knowledge

This repository now includes a React/Vite frontend. Before any frontend task, Codex agents must read:

- `.ai-context/frontend-skills/SKILL.md`
- `.ai-context/frontend-skills/references/react-vite-patterns.md`
- `docs/FRONTEND.md`
- `docs/product-specs/frontend-backend-contract.md`

Follow the WeChat-style four-tab product direction: `消息`, `联系人`, `发现`, `我的`. Use TDD with Vitest + Testing Library and verify with `npm run frontend:test`, `npm run frontend:build`, `npm run frontend:lint`, backend tests, and `scripts/verify-static.sh`.

Frontend API integration work must not replace real backend behavior with silent mocks or fake-success fallbacks. Mock data is allowed only for visual scaffolding, test fixtures, or explicit demo/mock modes. Codex agents must use the unified `createApiClient`, keep bearer token injection consistent, verify Vite proxy/API contract paths, and clearly report whether validation was real E2E or frontend contract/proxy verification. If a real API call fails, surface the failure and fix the cause instead of silently switching to demo data.

## go-zero / goctl AI Knowledge

This repository uses go-zero. Before any go-zero refactor or code generation task, Codex agents must read:

- `.ai-context/zero-skills/SKILL.md`
- `.ai-context/zero-skills/references/goctl-commands.md`
- `.ai-context/zero-skills/references/rest-api-patterns.md`
- `.ai-context/zero-skills/references/rpc-patterns.md`
- `.ai-context/zero-skills/references/database-patterns.md`

Local copies of the key references are also versioned under `docs/references/go-zero/` for stable review. Follow spec-first workflow: update `.api` / `.proto`, validate with `goctl`, generate boilerplate with `goctl`, keep business logic in `internal/logic`, and verify with `go test ./...` plus `scripts/verify-static.sh`.
