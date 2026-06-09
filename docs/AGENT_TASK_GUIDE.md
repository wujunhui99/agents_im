# Agent Task Guide

按任务类型读取，不要一次性读完整 `docs/`。

## 快速上手

1. 先读根目录 [`AGENTS.md`](../AGENTS.md)，确认硬约束和本次任务是否允许 push/PR/merge。
2. 读 [`docs/AGENTIC_DEVELOPMENT_WORKFLOW.md`](./AGENTIC_DEVELOPMENT_WORKFLOW.md)，需要 Git 细节时再读 [`docs/AGENT_GIT_STANDARD.md`](./AGENT_GIT_STANDARD.md) 与 [`docs/GIT_WORKFLOW.md`](./GIT_WORKFLOW.md)。
3. 按任务类型读取下方专题文档；不要先扫完整 `docs/`。
4. 开发前用 `make services`、`scripts/dev-up.sh --help`、`scripts/detect-deploy-changes.py` 定位当前服务/端口/部署事实源。
5. 完成时报告真实验证命令；涉及 Issue 时评论一次，简要说明实现方式。

## 服务清单事实源

- 本地服务名与 package path：`Makefile` 的 `BACKEND_SERVICES` / `PKG_*`，快速查看用 `make services`。
- 本地启动顺序、生成配置和端口：`scripts/dev-up.sh`。
- 部署镜像与变更选择：`scripts/detect-deploy-changes.py`。
- k3s 可部署对象和 rollout 等待：`scripts/deploy-k3s.sh`、`deploy/k8s/`。
- 文档里的服务表只作导航；发现冲突时先以脚本为准，并在同一变更里修文档。

## Go/go-zero API/RPC/backend

适用：修改 Go 服务、API/RPC contract、repository、service context、shared backend package 或 go-zero 生成代码。

读 [`.claude/skills/refactor-domain-to-service/SKILL.md`](../.claude/skills/refactor-domain-to-service/SKILL.md)、[`docs/design-docs/go-zero-service-layout.md`](./design-docs/go-zero-service-layout.md)、[`docs/design-docs/user-auth-friends-groups-boundaries.md`](./design-docs/user-auth-friends-groups-boundaries.md)、[`docs/design-docs/message-chain-contract.md`](./design-docs/message-chain-contract.md)。

常见文件：`service/<domain>/{api,rpc}/**`、`api/*.api`、`proto/*pb/**`、`internal/{handler,logic,servicecontext}/**`、`common/share/types/types.go`。

## Frontend React/Vite

适用：修改 `web/`、前端 API adapter、页面交互、样式、Vite proxy 或前端测试。

读 [`.claude/skills/frontend-skills/SKILL.md`](../.claude/skills/frontend-skills/SKILL.md)、[`.claude/skills/frontend-skills/references/react-vite-patterns.md`](../.claude/skills/frontend-skills/references/react-vite-patterns.md)、[`docs/FRONTEND.md`](./FRONTEND.md)、[`docs/product-specs/frontend-backend-contract.md`](./product-specs/frontend-backend-contract.md)。

常见文件：`web/src/App.tsx`、`web/src/api/*.ts`、`web/src/features/messages/MessagesPage.tsx`、`web/src/components/ContactsPage.tsx`、`web/src/styles.css`。保留微信式四 Tab：`消息`、`联系人`、`发现`、`我的`；真实联调不能静默切 mock/demo。

## Account/Profile/Friends/Groups

适用：修改账号资料、认证后的 profile 展示、好友申请/列表、群聊成员或相关 API/RPC。

读 [`docs/product-specs/account-social-core.md`](./product-specs/account-social-core.md)、[`docs/design-docs/account-service-terminology.md`](./design-docs/account-service-terminology.md)、[`docs/design-docs/user-auth-friends-groups-boundaries.md`](./design-docs/user-auth-friends-groups-boundaries.md)、[`docs/design-docs/database-schema-v2.md`](./design-docs/database-schema-v2.md)。

关键语义：Profile 存 `birth_date` 不存 `age`；前端不展示内部 ID；好友申请单向 pending，accepted 后双向；好友列表只显示 accepted。

## Message Storage / Ordering / Outbox

适用：修改消息写入、拉取、排序、去重、已读、outbox、消息 repository 或相关迁移。

读 [`docs/product-specs/message-chain.md`](./product-specs/message-chain.md)、[`docs/design-docs/message-chain-contract.md`](./design-docs/message-chain-contract.md)、[`docs/design-docs/message-storage.md`](./design-docs/message-storage.md)、[`docs/design-docs/message-outbox.md`](./design-docs/message-outbox.md)、[`docs/design-docs/database-schema-v2.md`](./design-docs/database-schema-v2.md)、[`docs/design-docs/gateway-message-contract.md`](./design-docs/gateway-message-contract.md)。

关键语义：`conversation_id + seq` 是显示顺序权威；`payload_hash` 不是唯一性；连续相同消息必须保存为两条；V2 方向是 `messages + message_outbox`。

## WebSocket / Live Push / 生产复现

适用：修改 WebSocket gateway、实时推送、断线重连、message-transfer、生产 live push 复现或运行时排障。

读 [`docs/design-docs/websocket-gateway.md`](./design-docs/websocket-gateway.md)、[`docs/design-docs/websocket-reconnect-sync.md`](./design-docs/websocket-reconnect-sync.md)、[`docs/design-docs/gateway-push-delivery.md`](./design-docs/gateway-push-delivery.md)、[`docs/design-docs/message-transfer-worker.md`](./design-docs/message-transfer-worker.md)、[`docs/design-docs/transfer-gateway-dispatcher.md`](./design-docs/transfer-gateway-dispatcher.md)、[`docs/qa/websocket-live-push-reproduction.md`](./qa/websocket-live-push-reproduction.md)。

关键语义：浏览器原生 WebSocket 不能设置 `Authorization`，生产同源使用 `/ws?token=[REDACTED]`，需允许 query token；WS open 不等于 live push 成功，必须验证 A 发 B、B 不刷新收到。生产复现优先用既有互为好友测试账号；reproduction-only 不改代码、不启动本地服务。

## AI / Agent / AI Reply

适用：修改 Agent 账号、AI runtime、AI 托管/建议回复、LLM provider/model 配置或 Agent 消息语义。

读 [`docs/design-docs/core-beliefs.md`](./design-docs/core-beliefs.md)、[`docs/design-docs/agent-system-architecture.md`](./design-docs/agent-system-architecture.md)、[`docs/design-docs/agent-runtime-eino.md`](./design-docs/agent-runtime-eino.md)、[`docs/design-docs/agent-conversation-hosting.md`](./design-docs/agent-conversation-hosting.md)、[`docs/design-docs/im-agent-contract.md`](./design-docs/im-agent-contract.md)、[`docs/design-docs/ai-reply-v1.md`](./design-docs/ai-reply-v1.md)、[`docs/product-specs/message-chain.md`](./product-specs/message-chain.md)。

AI 回复 V1：每用户每会话设置，默认关闭；只做 `suggest_only` 手动草稿；不自动发送；上下文 bounded recent messages + optional summary；缺 provider/model config 要可见失败。

## Media / Avatar / Object Storage

适用：修改头像、上传 intent、对象存储、media metadata、media-api/media-rpc 或前端媒体展示。

读 [`docs/design-docs/database-schema-v2.md`](./design-docs/database-schema-v2.md)、[`docs/product-specs/frontend-backend-contract.md`](./product-specs/frontend-backend-contract.md)、[`common/share/model/media.go`](../common/share/model/media.go)、[`internal/repository/postgres_media.go`](../internal/repository/postgres_media.go)、[`service/media/api`](../service/media/api)、[`service/media/rpc`](../service/media/rpc)。

关键语义：头像 >3M 客户端压缩；OSS 头像展示 URL 不应短期过期；文件 <20M；图片默认压缩，可选原图且原图 <15M。

## Deployment / CI / k3s

适用：修改 `.drone.yml`、CI 脚本、部署脚本、k3s manifest、生产配置、运行手册或 CI/部署失败排查。

读 [`deploy/README.md`](../deploy/README.md)、[`docs/GIT_WORKFLOW.md`](./GIT_WORKFLOW.md)、[`docs/RELIABILITY.md`](./RELIABILITY.md)、[`docs/SECURITY.md`](./SECURITY.md)、`scripts/ci/*.sh`、`scripts/deploy-k3s.sh`；后端接口报错修复按需读 [`.claude/skills/fix-api/SKILL.md`](../.claude/skills/fix-api/SKILL.md)。

关键语义：`main` 进入发布流水线；CI 绿不等于 runtime 绿，必要时核验 rollout、日志、live API/WS。不要在文档、日志、Issue、PR 或聊天里记录凭据或连接串。

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

如果 Docker 不可用，不要声称已完成 Docker/PostgreSQL 集成验证；DB/repository SQL 改动可用本机 PostgreSQL integration。
