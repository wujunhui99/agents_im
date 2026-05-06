# AGENTS.md

本文件是 Codex/AI Agent 在本仓库的自动加载入口。根据 OpenAI Codex 源码，Codex 会从项目根到当前工作目录自动收集 `AGENTS.md` 并作为 model-visible instructions；因此本文件应保持为**短目录 + 必要约束**，不要写成大而全百科。详细知识放到 `docs/` 专题文档，按任务读取。

## 必须遵守

1. **禁止假实现**：业务/API/持久化/消息链路能力不能用 mock、stub、硬编码、空实现、假成功冒充。mock 只允许在测试 fixture、demo/mock mode 或明确视觉占位中使用。
2. **失败优先**：接口不通、配置缺失、依赖不可用、测试失败时要显式失败并报告根因假设，不能静默 fallback。
3. **根因优先**：修复前先复现、读完整错误、追踪数据流；不要未理解原因就堆补丁。
4. **验证优先**：声称完成必须给出可重复命令。没有真实启动/请求时，只能说 contract/unit/static verification。
5. **密钥脱敏**：不要输出 token、JWT、密码、cookie、DSN、server host/user/port/key、MinIO/JWT/DB secret；统一写 `[REDACTED]`。
6. **文档按需读取**：先读本文件，再按任务类型读下面的专题文档，不要一次性读完整 `docs/`。

## 项目一句话

`agents_im` 是 Go/go-zero + React/Vite 的实时 IM 系统，逐步加入 Agent/AI 能力。核心是账号/Profile、好友/群聊、消息存储、WebSocket 实时推送、媒体/对象存储和 Agent/AI runtime。

## 工作流

- 用户负责目标、约束和验收；Agent 负责实现、验证和修复。
- 复杂任务必须产出或更新可版本化 docs/plan。
- 默认使用 `git worktree` 并行：一个 Codex 一个独立 worktree/branch。
- 用户偏好：Hermes/Helios 做 planner/architect/reviewer/integrator，编码测试尽量委派 Codex。
- feature 原则：`feature/* -> develop -> main`；紧急生产 hotfix 可从 `main -> fix/* -> main`。
- Codex 是否允许 commit/push 必须由任务说明明确；未明确时不要 push。
- Codex 相关长期规则尽量直接写入本 `AGENTS.md`，因为 Codex 会自动读取；不要只放在聊天或临时 prompt 里。
- Codex 解决 GitHub Issue 后必须在对应 Issue 评论一次：评论要**简洁但不丢信息**，不要求一句话；方便 Controller 快速 review。
  - Bug：说明 root cause、fix、测试摘要、分支/commit/PR、blockers（如有）。
  - Feature / 新需求：说明实现的用户可见行为、关键文件/API/数据流、范围边界、测试摘要、分支/commit/PR、blockers（如有）。
  - Research / 调研任务：可以更详细；说明调研结论、证据来源（文件/命令/链接）、可选方案与 tradeoff、推荐方案、风险和未决问题。
- Controller 必须复核 Codex 的 diff、测试和分支状态，不能只信自述。Codex 完成后，Hermes 第一轮验收先看 `git diff`/业务逻辑是否偏离需求，再跑/核验测试；review 通过后应尽快 MR/merge 到 `develop`。
- Hermes 在 Issue 评论验证结果后再关闭，Issue 关闭必须等 `develop` 集成成功。
- Codex commit 前验证门禁：按改动范围运行 gofmt、git diff --check、go test ./...、scripts/verify-static.sh；web 改动加前端测试/build；DB/repository SQL 改动加 PostgreSQL integration。
- 数据库 schema/data 变更必须新增 `db/change_log/*.sql`；`.md` 只作说明，SQL 是事实源。

## 快速导航

- 架构总览：[`ARCHITECTURE.md`](./ARCHITECTURE.md)
- 产品规格索引：[`docs/product-specs/index.md`](./docs/product-specs/index.md)
- 设计文档索引：[`docs/design-docs/index.md`](./docs/design-docs/index.md)
- 本地开发：[`docs/DEVELOPMENT.md`](./docs/DEVELOPMENT.md)
- 部署：[`deploy/README.md`](./deploy/README.md)
- Git 工作流：[`docs/GIT_WORKFLOW.md`](./docs/GIT_WORKFLOW.md)
- Agentic GitHub Project / Issues 工作流：[`docs/AGENTIC_DEVELOPMENT_WORKFLOW.md`](./docs/AGENTIC_DEVELOPMENT_WORKFLOW.md)
- 安全：[`docs/SECURITY.md`](./docs/SECURITY.md)
- 可靠性：[`docs/RELIABILITY.md`](./docs/RELIABILITY.md)
- 前端约定：[`docs/FRONTEND.md`](./docs/FRONTEND.md)
- 产品判断：[`docs/PRODUCT_SENSE.md`](./docs/PRODUCT_SENSE.md)
- 执行计划规范：[`docs/PLANS.md`](./docs/PLANS.md)
- 外层 workspace 文档迁移记录：[`docs/workspace-migration/outer-project-docs-migration-2026-05-05.md`](./docs/workspace-migration/outer-project-docs-migration-2026-05-05.md)

## 按任务读取

### Go/go-zero API/RPC/backend

读：

- `.ai-context/zero-skills/SKILL.md`
- `.ai-context/zero-skills/references/goctl-commands.md`
- `.ai-context/zero-skills/references/rest-api-patterns.md`
- `.ai-context/zero-skills/references/rpc-patterns.md`
- `.ai-context/zero-skills/references/database-patterns.md`
- [`docs/design-docs/user-auth-friends-groups-boundaries.md`](./docs/design-docs/user-auth-friends-groups-boundaries.md)
- [`docs/design-docs/message-chain-contract.md`](./docs/design-docs/message-chain-contract.md)

常见文件：`api/*.api`、`proto/**/*.proto`、`internal/handler/**`、`internal/logic/**`、`internal/svc/service_context.go`、`internal/types/types.go`。

### Frontend React/Vite

读：

- `.ai-context/frontend-skills/SKILL.md`
- `.ai-context/frontend-skills/references/react-vite-patterns.md`
- [`docs/FRONTEND.md`](./docs/FRONTEND.md)
- [`docs/product-specs/frontend-backend-contract.md`](./docs/product-specs/frontend-backend-contract.md)

常见文件：`web/src/App.tsx`、`web/src/api/*.ts`、`web/src/features/messages/MessagesPage.tsx`、`web/src/components/ContactsPage.tsx`、`web/src/styles.css`。

UI 方向：保留微信式四 Tab：`消息`、`联系人`、`发现`、`我的`。真实联调不能静默切 mock/demo。

### Account/Profile/Friends/Groups

读：

- [`docs/product-specs/account-social-core.md`](./docs/product-specs/account-social-core.md)
- [`docs/design-docs/account-service-terminology.md`](./docs/design-docs/account-service-terminology.md)
- [`docs/design-docs/user-auth-friends-groups-boundaries.md`](./docs/design-docs/user-auth-friends-groups-boundaries.md)
- [`docs/design-docs/database-schema-v2.md`](./docs/design-docs/database-schema-v2.md)

当前关键语义：Profile 存 `birth_date` 不存 `age`；前端不展示内部 ID；好友申请是一条单向 pending，accepted 后双向；好友列表只显示 accepted。

### Message Storage / Ordering / Outbox

读：

- [`docs/product-specs/message-chain.md`](./docs/product-specs/message-chain.md)
- [`docs/design-docs/message-chain-contract.md`](./docs/design-docs/message-chain-contract.md)
- [`docs/design-docs/message-storage.md`](./docs/design-docs/message-storage.md)
- [`docs/design-docs/message-outbox.md`](./docs/design-docs/message-outbox.md)
- [`docs/design-docs/database-schema-v2.md`](./docs/design-docs/database-schema-v2.md)
- [`docs/design-docs/gateway-message-contract.md`](./docs/design-docs/gateway-message-contract.md)

当前关键语义：`conversation_id + seq` 是显示顺序权威；`payload_hash` 不是唯一性；连续相同消息必须保存为两条；V2 方向是 `messages + message_outbox`。

### WebSocket / Live Push / 生产复现

读：

- [`docs/design-docs/websocket-gateway.md`](./docs/design-docs/websocket-gateway.md)
- [`docs/design-docs/websocket-reconnect-sync.md`](./docs/design-docs/websocket-reconnect-sync.md)
- [`docs/design-docs/gateway-push-delivery.md`](./docs/design-docs/gateway-push-delivery.md)
- [`docs/design-docs/message-transfer-worker.md`](./docs/design-docs/message-transfer-worker.md)
- [`docs/design-docs/transfer-gateway-dispatcher.md`](./docs/design-docs/transfer-gateway-dispatcher.md)
- [`docs/qa/websocket-live-push-reproduction.md`](./docs/qa/websocket-live-push-reproduction.md)

当前关键事实：浏览器原生 WebSocket 不能设置 `Authorization`，生产同源使用 `/ws?token=[REDACTED]`，因此需要 `GATEWAY_WS_ALLOW_QUERY_TOKEN=true` 和 `GatewayWS.AllowQueryToken=true`。WS open 不等于 live push 成功，必须验证 A 发 B、B 不刷新收到。

生产复现优先用已有互为好友的测试账号，避免每次注册/加好友。reproduction-only 不改代码、不启动本地服务。

### AI / Agent / AI Reply

读：

- [`docs/design-docs/core-beliefs.md`](./docs/design-docs/core-beliefs.md)
- [`docs/design-docs/agent-system-architecture.md`](./docs/design-docs/agent-system-architecture.md)
- [`docs/design-docs/agent-runtime-eino.md`](./docs/design-docs/agent-runtime-eino.md)
- [`docs/design-docs/agent-conversation-hosting.md`](./docs/design-docs/agent-conversation-hosting.md)
- [`docs/design-docs/im-agent-contract.md`](./docs/design-docs/im-agent-contract.md)
- [`docs/design-docs/ai-reply-v1.md`](./docs/design-docs/ai-reply-v1.md)
- [`docs/product-specs/message-chain.md`](./docs/product-specs/message-chain.md)

AI 回复 V1：每用户每会话设置，默认关闭；只做 `suggest_only` 手动草稿；不自动发送；草稿可编辑并走现有发送路径；上下文必须 bounded recent messages + optional summary，不能发送全量历史；缺 provider/model config 要可见失败。

### Media / Avatar / Object Storage

读：

- [`docs/design-docs/database-schema-v2.md`](./docs/design-docs/database-schema-v2.md)
- [`docs/product-specs/frontend-backend-contract.md`](./docs/product-specs/frontend-backend-contract.md)
- `internal/model/media.go`
- `internal/repository/postgres_media.go`

当前产品方向：头像 >3M 客户端压缩，OSS 头像展示 URL 不应短期过期；文件 <20M；图片默认压缩，可选原图且原图 <15M。

### Deployment / CI / k3s

读：

- [`deploy/README.md`](./deploy/README.md)
- [`docs/GIT_WORKFLOW.md`](./docs/GIT_WORKFLOW.md)
- [`docs/RELIABILITY.md`](./docs/RELIABILITY.md)
- [`docs/SECURITY.md`](./docs/SECURITY.md)
- `.github/workflows/ci.yml`
- `.github/workflows/deploy.yml`
- `scripts/deploy-k3s.sh`

当前关键事实：`main` push 触发 deploy；k3s namespace 是 `agents-im`；可使用 SSH alias `server-ssh-tls` 但不要暴露真实连接信息；Actions 绿不等于 runtime 绿，必要时查 pod/log/rollout/live API/WS。

## 常用验证命令

按任务选择子集，不要对只读/复现任务盲目跑全量。

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name '*.go' -print)
go test ./...
npm --prefix web run test:run -- --reporter=dot
npm --prefix web run build
bash scripts/verify-static.sh
git diff --check
# DB/repository SQL changes only, against a disposable local/test DB:
AGENTS_IM_CONFIRM_TRUNCATE=1 scripts/verify-postgres-local.sh
```

如果 Docker 不可用，不要声称已完成 Docker/PostgreSQL 集成验证；DB/repository SQL 改动可用 `DATABASE_URL`/`AGENTS_IM_POSTGRES_DSN` + `scripts/verify-postgres-local.sh` 做本机 PostgreSQL integration。
