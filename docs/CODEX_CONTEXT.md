# Codex Context for `agents_im`

本文档是给 Codex/子 Agent 的高密度项目上下文。它不替代 `AGENTS.md`、`ARCHITECTURE.md`、产品规格和设计文档；它的目标是把近期反复踩坑、跨层约定和实现优先级集中放到一个 Codex 容易看到的位置。

Codex 开始任何任务前，至少阅读：

1. `AGENTS.md`
2. `docs/CODEX_CONTEXT.md`（本文）
3. 与任务直接相关的 product/design docs
4. 相关代码路径和测试

## 1. 项目一句话

`agents_im` 是一个 Go/go-zero + React/Vite 的实时 IM 系统，正在逐步加入 Agent/AI 能力。核心要求是：真实消息链路、真实账号/好友/群聊/媒体/Agent 数据模型、失败可见、不要用 mock/fake success 掩盖生产缺口。

## 2. 当前架构地图

### 前端

- 路径：`web/`
- 技术栈：React + TypeScript + Vite + Vitest/Testing Library
- UI 方向：微信式四 Tab：`消息`、`联系人`、`发现`、`我的`
- API client：优先使用已有 typed API client，不要在组件里散落裸 `fetch`。
- 真实集成任务：不能静默 fallback 到 mock/demo；API 失败要可见。

重要文件：

- `web/src/App.tsx`：登录态、四 Tab、API 实例注入、token 下发。
- `web/src/api/*.ts`：typed API client。
- `web/src/features/messages/MessagesPage.tsx`：会话/消息 UI、WebSocket subscription、发送消息。
- `web/src/components/ContactsPage.tsx`：好友列表、好友申请。
- `web/vite.config.ts`：本地代理；`/ws -> ws://127.0.0.1:8084`。

### 后端

- 语言：Go
- REST/go-zero 风格文件：`api/*.api`、`internal/handler/**`、`internal/logic/**`、`internal/types/types.go`
- RPC/generated：`proto/**`、`internal/rpcgen/**`
- 领域/仓储：`internal/model/**`、`internal/repository/**`
- 运行入口：`cmd/*`
- 配置：`internal/config/**`、`etc/**`、`deploy/k8s/etc/**`

重要服务端口（本地）：

- user/account API：`8080`
- auth API：`8081`
- friends API：`8082`
- message API：`8083`
- gateway-ws：`8084`
- groups API：`8085`
- agent API：`8086`

### 数据层

- PostgreSQL 是主要持久化。
- Redis/Redpanda/MinIO 用于 presence、事件、对象存储等扩展。
- 数据库 schema 以 `db/migrations/*.sql` 为准；不要只改 repository 不改 migration，反之亦然。

## 3. 账号/Profile/好友当前语义

### Account/Profile

- 用户/Agent/Admin 都是 IM account。
- ID 使用 Snowflake 风格数字字符串，不使用 `usr_`/`agt_` 前缀。
- `account_type` 目标语义：
  - `0 = admin`
  - `1 = user`
  - `2 = agent`
- Profile 存 `birth_date`，不要存派生的 `age`。
- 前端不要展示内部 `user_id/account_id`。

### Friend Request

当前好友申请语义：

```text
A 添加 B：只写 A -> B pending
B 查询 A：GetFriendship 返回 synthetic pending view
B 同意 A：A -> B accepted，B -> A accepted
B 拒绝 A：A -> B rejected
ListFriends：只返回 accepted
Incoming：friend_id = 当前用户 AND status = pending
Outgoing：user_id = 当前用户 AND status = pending
重复添加：已有 pending/accepted 时复用，created=false
```

REST 端点：

```text
GET /friends/requests
POST /friends/requests/:user_id/accept
POST /friends/requests/:user_id/reject
```

前端：pending 不进入好友列表；incoming 可以同意/拒绝；outgoing 显示等待对方确认。

## 4. 消息链路当前重点

### 存储/历史

- Message Service 负责持久化、conversation seq、read state、outbox。
- 刷新后能看到消息，通常说明落库和历史查询是好的。
- `conversation_id + seq` 是显示顺序权威；不要依赖网络到达顺序或时间戳排序。

### WebSocket/live push

关键链路：

```text
message_outbox -> publisher/transfer -> gateway-ws -> frontend websocketClient -> React state update
```

如果“刷新能看到，不刷新看不到”：优先排查 live path，而不是消息存储。

前端已使用 browser-native `WebSocket`，浏览器不能自定义 `Authorization` header，因此生产同源 WS 当前使用：

```text
wss://agenticim.xyz/ws?token=[REDACTED]
```

这要求 gateway 配置启用 query token：

```yaml
AllowQueryToken: true
GATEWAY_WS_ALLOW_QUERY_TOKEN: "true"
```

如果生产禁用 query token，会出现：

```text
HTTP Authentication failed; no valid credentials available
close code 1006
```

这是握手认证失败，不是前端 state update 问题。

### 当前已知 live-push 次级风险

即使 WS 握手成功，实时投递还可能被这些缺口阻断：

- `message-transfer` 使用 `Dispatcher.Driver: noop`
- `cmd/message-transfer` 构造 `transfer.NoopDispatcher{}`
- outbox publisher/transfer/gateway dispatcher 未完整生产 wiring

不要把“WS 能 open”直接等同于“消息会实时 push”。验证必须包含 A 发 B、B 不刷新收到 `message_received`。

## 5. 生产复现/QA 约定

生产 URL：

```text
https://agenticim.xyz/
```

复现生产问题时：

- reproduction-only 不改代码，不启动本地服务。
- 使用生产站点和真实 API 行为。
- 不输出 token、密码、JWT、cookie、连接串；统一 `[REDACTED]`。
- 生产 WS/live-push QA 优先复用已有测试账号，且这些测试账号应已互为好友，避免每次注册/加好友拖慢复现。
- 若必须创建临时账号，报告里也要脱敏密码和 token。

推荐 live-push 复现步骤：

1. 登录已有测试账号 A/B。
2. B 打开消息页并记录 WS：open/error/close/message。
3. A 给 B 发唯一文本。
4. B 等待 5～10 秒，不刷新。
5. 检查 B UI 是否出现消息。
6. 作为对照，B 调 `/conversations/seqs` 或刷新页面，确认历史是否能看到。

判定：

- `POST /messages = 200` 且历史能看到，但 WS error/1006：握手/ingress/gateway auth。
- WS open 但没有 `message_received`：outbox/transfer/dispatcher/gateway fanout。
- WS 收到 `message_received` 但 UI 不更新：前端 payload parsing/state update。

## 6. Codex 工作流约定

### 默认分工

用户偏好：Hermes/Helios 做 planner/architect/reviewer/integrator，编码测试尽量交给 Codex worktree 并行。

Codex 任务必须包含：

- worktree path
- branch name
- 任务目标和非目标
- 要读的 docs/code
- 验收标准
- 必跑验证命令
- 是否允许 commit/push

### Worktree 规则

- 每个 Codex 用独立 `git worktree`。
- feature 分支原则上先合入 `develop`，再合入 `main`。
- 紧急生产 hotfix 可从 `main -> fix/* -> main`。
- Codex 可以 commit 时要明确说；不能 commit/push 时也要明确说。
- Controller 必须复核 diff、测试和分支状态，不能只信 Codex 自述。

### 常用验证

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name '*.go' -print)
go test ./...
npm --prefix web run test:run -- --reporter=dot
npm --prefix web run build
bash scripts/verify-static.sh
git diff --check
```

本 WSL 默认 PATH 可能没有 `go`，要显式设置：

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
```

本地 Docker 可能无权限，不要声称已用 Docker PostgreSQL 完整验证，除非命令真实成功。

## 7. AI/Agent 功能开发原则

Agent/AI 功能必须走 IM seam：

- AI 最终要作为消息写回时，必须通过 Message Service/MessageLogic，不要绕过直接写表。
- 生产缺 LLM/provider config 时 fail visibly，不要返回假 AI 回复。
- 测试可以用 fake provider，但必须在测试边界内。
- 不要持久化完整 prompt/secrets，除非产品/安全文档明确允许；日志默认脱敏。
- 群聊里 Agent/AI 不应制造噪音；优先手动触发或明确触发。

### AI 回复 V1 已定策略

V1 做“AI 帮我回”草稿，不做自动代发：

- 每用户、每会话独立设置。
- 默认关闭。
- mode：`disabled` / `suggest_only`；`auto_reply` 仅预留，不实现。
- 用户点击“AI 回复/AI 帮我回”生成草稿。
- 草稿可编辑，发送仍走现有 `POST /messages`。
- 上下文只取 bounded recent messages，默认约 30 条；不要塞全量历史。
- 预留 rolling summary schema/interface，但 V1 可以先用空摘要或后续异步摘要。
- 缺 model config 返回可见错误。

详细设计见：`docs/design-docs/ai-reply-v1.md`。

## 8. 当前容易踩坑

- `account_type=0` 是合法 admin 值，API/RPC 需要能区分“未提供”和“显式 0”。
- protobuf 生成路径/包名错误可能导致 k3s 运行时 `proto: file "*.proto" is already registered`。
- 前端测试 mock 顺序容易因为新增 API 请求改变而失败，尤其 `App.test.tsx`。
- WebSocket jsdom 测试必须注入 fake WebSocket factory，避免测试环境误连真实 `/ws`。
- 生产部署 workflow 成功不等于 runtime 成功；必要时查 k3s pod、logs、rollout、live HTTP/WS 行为。
- 不要泄露 `server-ssh-tls` 背后的真实 host/user/port/key，也不要输出 DB DSN、JWT secret、MinIO credentials。

## 9. 文档更新原则

如果任务发现新的跨层事实或生产坑：

1. 更新相关设计/产品/部署文档。
2. 必要时更新 `docs/CODEX_CONTEXT.md`，让未来 Codex 能直接看到。
3. 更新 `AGENTS.md` 导航，但保持 `AGENTS.md` 短。
4. 跑 `git diff --check`，涉及 Markdown 链接时检查链接。
