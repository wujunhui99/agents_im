# ARCHITECTURE.md

适用场景：需要理解系统边界、核心模块、数据流或跨服务影响面时先读本文。

本文档提供项目的顶层架构地图，帮助人类和 AI Agent 快速理解系统边界、核心模块和关键数据流。

## 系统目标

构建一个高性能、分布式、实时聊天系统，同时提供 Agent 服务能力。系统需要支持：

- Account 单聊与群聊，其中 Account 可代表 human user、agent、admin，未来可扩展 service/official accounts
- Agent 创建、销毁、持久化和运行时管理
- 用户与 Agent 单聊
- 多用户与多 Agent 群聊
- Agent 工具调用，包括代码执行、网络搜索和 IM 工具调用
- 高可靠消息投递
- 可观测、可追踪、可扩展的微服务架构

## 参考实现

本项目有明确参考实现，但参考仓库只作为设计输入，不直接决定本项目实现。

- IM 系统主要参考：[`openimsdk/open-im-server`](https://github.com/openimsdk/open-im-server.git)，本地目录为 `docs/references/open-im-server/`。
- Agent 系统参考：[`bytedance/deer-flow`](https://github.com/bytedance/deer-flow.git)，本地目录为 `docs/references/deer-flow/`。
- Agent 系统参考：[`HKUDS/nanobot`](https://github.com/HKUDS/nanobot.git)，本地目录为 `docs/references/nanobot/`。

参考仓库说明见 [`docs/references/README.md`](./docs/references/README.md)。涉及具体设计借鉴时，应在 `docs/design-docs/` 中记录取舍原因。

## 系统边界

IM 后端、Agent 系统和前端系统的职责边界见 [`docs/design-docs/system-boundaries.md`](./docs/design-docs/system-boundaries.md)。当前结论是：IM 后端负责实时通信底座和消息可靠性；Agent 系统负责 Agent 生命周期、推理和工具调用；前端负责用户交互和实时消息展示。IM 与 Agent 通过事件/Webhook 和消息写回接口解耦，因此 IM 后端与 Agent 系统可以基于契约并行开发。

IM 与 Agent 第一阶段最小 API/Event Contract 见 [`docs/design-docs/im-agent-contract.md`](./docs/design-docs/im-agent-contract.md)。该契约参考 OpenIM webhook 设计，定义了 `callbackAfterSendSingleMsgCommand`、`callbackAfterSendGroupMsgCommand`、Agent 消息写回、会话上下文查询、幂等、签名和重试规则。

IM 后端 MVP 范围和前端对接契约见 [`docs/product-specs/backend-mvp.md`](./docs/product-specs/backend-mvp.md) 与 [`docs/design-docs/backend-mvp-contract.md`](./docs/design-docs/backend-mvp-contract.md)。

## 顶层模块

### Account Service

负责账号资料的权威数据，不管理密码或认证秘密。Account 是身份与资料主体，可代表 human user、agent、admin，未来可扩展 service/official accounts。核心能力包括 Snowflake account id、唯一标识符（类似微信号）、名称、性别、年龄、地区、头像、`account_type=user|agent|admin` 等资料维护，`/me` 查询，公开资料查询，以及供 `auth` 注册流程使用的账号存在性检查。PostgreSQL source-of-truth 为 `accounts` + `profiles`；V0 public/API compatibility 继续保留 `/users`、`user-api`、`user-rpc` 和 `user_id` 字段，这些 `user_id` 均是 account id alias。术语边界见 [`docs/design-docs/account-service-terminology.md`](./docs/design-docs/account-service-terminology.md)。

### Auth Service

负责认证和登录注册。第一阶段支持账号密码注册/登录，密码和认证秘密只归 `auth` 管理；注册时依赖 Account Service 查询账号是否存在，并协作创建账号资料。手机号验证码、微信扫码等能力作为后续扩展，当前不实现。

### Friends Service

负责好友关系维护，包括添加好友、删除好友、查询好友列表和关系状态。第一阶段 public JSON 仍使用 `user_id` / `friend_id`，但它们指向 account id。好友关系不写入 Account Service 的权威资料模型。

### Groups Service

负责群聊和群成员关系维护，包括创建群、加群、退群、查询群成员。第一阶段 `creator_user_id`、`user_id` 等字段是 account id alias。群成员关系不写入 Account Service 的权威资料模型。

### Message Service

负责消息链路契约和实现：`msg-rpc` SendMessage 校验后把 `message.submitted` 事件 publish 到 Kafka `msg.toTransfer.v1`（acks=all，唯一写原语，03 §9 B2/B3b），ACK 带 `server_msg_id` 但 seq=0 占位；seq 分配（Redis Malloc + PG 播种）与批量落库由 msgtransfer 完成，客户端 seq 经自己的 `message_received` push 异步回填。读路径（按 seq 拉取消息、`user_id + conversation_id -> has_read_seq` 已读状态）仍由 msg-rpc 直读 PostgreSQL。这里的 `user_id` 是 V0 account id alias。当前文本、图片、文件消息均通过同一消息链路写入；图片/文件消息必须引用已就绪、归发送者所有的 `media_objects` 记录。设计见 [`docs/design-docs/message-chain-contract.md`](./docs/design-docs/message-chain-contract.md) 和 [`docs/refactor/v1/03-message-pipeline.md`](./docs/refactor/v1/03-message-pipeline.md)，产品规格见 [`docs/product-specs/message-chain.md`](./docs/product-specs/message-chain.md)。

### Media Service

`media-api` + `media-rpc` 负责媒体上传意图、上传完成校验、下载 URL 和头像展示 URL。`media-rpc` 拥有 `media_objects` 写入和对象存储生命周期；`media-api` 提供 `/media/uploads`、`/media/uploads/:media_id/complete`、`/media/:media_id/download-url`、`/media/avatars/:media_id`。Message API/RPC/Gateway 只验证媒体元数据的 owner、purpose、status、content type 和 size，不直接访问对象存储。

### Message Transfer Worker

`service/msgtransfer` 消费 Kafka 链路（03 §9 B1/B3b，唯一消费路径；legacy outbox consumer 已退役）：`msg.toTransfer.v1` → poll-batch barrier（按会话分组）→ Redis seq Malloc（PG max(seq) 播种）+ client_msg_id dedup（7d）→ produce `msg.toPostgres.v1` / `msg.toPush.v1` / `agent.trigger.v1`；persist consumer 批量事务写 PG，push consumer 经 `KafkaPushConsumer` 适配 transfer.Worker + gateway HTTP dispatcher 调 msggateway `/internal/delivery/conversation`（fanout 含 sender 自己的 seq 回填）。投递可靠性通过 `delivery_attempts` 记录 `accepted`、`published`、`delivered`、`offline`、`failed`，其中 `delivered` 不等于已读。设计见 [`docs/design-docs/message-transfer-worker.md`](./docs/design-docs/message-transfer-worker.md)、[`docs/design-docs/transfer-gateway-dispatcher.md`](./docs/design-docs/transfer-gateway-dispatcher.md) 和 [`docs/design-docs/message-delivery-reliability.md`](./docs/design-docs/message-delivery-reliability.md)。

### IM Core Service

负责 IM 核心业务链路，包括用户会话、消息收发、消息状态、会话成员管理等。

### Gateway / WebSocket Service

负责客户端 WebSocket 长连接管理，包括：

- 连接建立与鉴权
- 心跳检测
- ACK 确认
- 在线状态维护
- 消息下发与重试

在线状态和连接元数据通过 Redis presence 层保存为短期运行状态，设计见 [`docs/design-docs/redis-presence.md`](./docs/design-docs/redis-presence.md)。独立入口 `service/msggateway`（原 `service/gateway-ws`，03 §9 A3 改名）通过 `GET /ws` 建立 WebSocket 连接；Handshake 使用与 HTTP API 一致的 JWT 配置，优先支持 `Authorization: Bearer <token>`，`token` query param 仅在 `GatewayWS.AllowQueryToken=true` 时启用；浏览器 `Origin` 由 `GatewayWS.AllowedOrigins` 精确匹配，未配置时只允许 same-origin。连接通过内存 connection manager 按 `user_id`（account id alias）注册多端 `connection_id`，并同步写入 `PresenceStore`。Gateway command router 支持 `heartbeat`、`send_message`、`pull_messages`、`get_conversation_seqs`、`mark_conversation_read`，消息域 command 经 msg-rpc gRPC（`service/msggateway/internal/backend`）转发完成消息写入、拉取、seq 查询和已读推进，gateway 进程不再装配 monolith 业务逻辑/仓储；`send_message` ACK 同步透传 msg-rpc 响应，send 后 gateway 不做本地 fanout——`message_received`（含 sender 自己的 seq 回填）由 msgtransfer 经 `/internal/delivery/conversation` 下发；heartbeat 和 WebSocket pong 会刷新 presence/last-seen，服务端按配置发送 ping 并按连接执行命令限流。Frontend reconnect sync 使用稳定 WebSocket ACK error envelope，并通过 `get_conversation_seqs`、`pull_messages`、`mark_conversation_read` 补偿缺失消息，产品契约见 [`docs/product-specs/frontend-sync-contract.md`](./docs/product-specs/frontend-sync-contract.md)，设计见 [`docs/design-docs/websocket-reconnect-sync.md`](./docs/design-docs/websocket-reconnect-sync.md)。Gateway push delivery 提供 `pkg/gateway/delivery.Dispatcher` 契约（跨服务推送事实源，msgtransfer 同样消费）和本进程内 WebSocket fanout，可向在线连接主动下发 `message_received` / `message_delivered` event；delivery dispatcher 会先查询 presence route metadata，再执行本进程内 fanout，offline/routed/failed recipient 均返回明确状态。Gateway 不拥有消息历史、会话 seq 或已读状态；这些数据由 msg-rpc 与 PostgreSQL 权威维护。msgtransfer 消费消息事件后调用 gateway 内部投递 endpoint，Redis Presence route metadata 用于跨实例路由。设计见 [`docs/design-docs/websocket-gateway.md`](./docs/design-docs/websocket-gateway.md)、[`docs/design-docs/gateway-push-delivery.md`](./docs/design-docs/gateway-push-delivery.md) 和 [`docs/design-docs/gateway-presence-routing.md`](./docs/design-docs/gateway-presence-routing.md)。

### Agent Service

负责 Agent 生命周期、配置组装、运行时能力和工具调用审计。第一版设计见 [`docs/product-specs/agent-system.md`](./docs/product-specs/agent-system.md) 和 [`docs/design-docs/agent-system-architecture.md`](./docs/design-docs/agent-system-architecture.md)。核心能力包括：

- 在账号系统中配合 `user` / `agent` / `admin` 账号类型，让 Agent 账号作为 IM 会话成员参与单聊和群聊。
- 当前 `service/agent/api`（入口 `agent.go`，直接装配 `internal/*`）启动，`service/agent/api/agent.api` 提供 Agent profile 管理基础，配置单独持久化到 `agents` 表；创建 Agent 必须通过 Account Service 资料能力验证绑定账号为 `account_type=agent`，验证不可用时必须失败。当前没有真实 Agent RPC/proto contract，不能创建空 RPC scaffold 冒充服务边界。
- `service/agent`（扁平 main，issue #503 骨架，未部署）以独立 consumer group `agent-trigger` 消费 `agent.trigger.v1`，承载 D15 三步终判（origin 防递归 → D16 账号 ID 类型位判 agent 收信 → conversation hosting）；runtime / IM 写回 / hosting 查询均为显式 mock driver（零副作用），真实实现随 [`docs/refactor/v1/04-agent.md`](./docs/refactor/v1/04-agent.md) §5 落地。过渡期真实 AI 回复仍由 msg-rpc 内 `agent.trigger.v1` 回流 consumer 产生。
- 管理系统提示词、工具、Agent skills 和 Agent 配置，并将元数据持久化在 PostgreSQL。
- 使用系统提示词、工具和 skills 组装 Agent runtime。
- 通过 RustFS (S3-compatible) object storage 保存 Agent skill 文件；Agent 绑定 skill 后默认可读取该 skill 文件，但不能越权读取其他文件。
- 管理 MCP 工具和本地工具。MCP server 和工具元数据入库；本地工具只允许服务端白名单 `handler_key`，不得从数据库执行任意脚本。
- 当前 Agent registry 基线已提供 prompt/tool/skill 元数据与 Agent 白名单绑定的 Go logic/repository 和 PostgreSQL schema；该基线不执行工具、不调用 LLM、不上传或读取对象存储二进制内容。
- 当前 Agent runtime provider 基线已提供 CloudWeGo Eino + DeepSeek ChatModel adapter/config，读取 `DEEPSEEK_API_KEY`、`DEEPSEEK_BASE_URL`、`DEEPSEEK_MODEL`；缺少 API key 时构造模型必须失败，不提供 mock/fake response。
- 当前 AI Hosting LLM observability 通过 `internal/llmobs` 和 Eino callback seam 发出 run/generation 事件；默认 noop/test sink 不联网，Langfuse 是目标后端但 live export 未实现时必须显式失败，设计见 [`docs/design-docs/llm-observability.md`](./docs/design-docs/llm-observability.md)。
- 当前 Agent runtime 工具解析契约位于 `internal/agentruntime/tools`：运行时必须从 `AgentRegistryRepository` 读取 Agent 工具绑定并重新校验工具状态、管理员配置、MCP server 状态和安全 transport；该契约只产出 Eino 可适配的安全 metadata/adapter seam，不执行 MCP 网络调用，也不提供 shell、命令、本地进程、stdio MCP、Python 或文件系统写入工具。
- Agent run、tool call、skill file read、Python exec 审计记录使用 append-only 审计表保存；摘要字段必须脱敏，Python 代码只保存 hash/大小摘要。
- 第一版不提供 shell/命令行脚本执行能力；Python 执行必须通过受限沙箱、限时限资源、默认无网络，并记录审计。
- 当前 Python executor 只提供 `internal/agent/pythonexec` 契约和 disabled 默认实现；未配置真实沙箱时必须返回 `ErrPythonExecutorDisabled`，不得在 Go 主服务进程内直接运行 Python 或 shell。
- 当前 Eino runtime core 只提供 `internal/agentruntime` 本地接口、领域请求/结果类型和 fail-first 归一化校验；不导入 Eino、不调用 LLM、不执行工具、不写回 IM。设计见 [`docs/design-docs/agent-runtime-eino.md`](./docs/design-docs/agent-runtime-eino.md)。
- Agent 响应必须通过 Message Service 写回 IM，不能绕过 IM 消息链路或直接推送 WebSocket。
- Agent-IM 第一阶段 Go 契约位于 `internal/agentim`：定义用户私聊 Agent、群聊 @Agent、管理员手动 run 三类触发；`AgentRunOrchestrator` 通过 `RuntimeRequestBuilder` 调用统一的 `internal/agentruntime.Runtime`，响应 writer 只依赖 `MessageLogic.SendMessage` / Message Service seam，并通过 Agent 消息元数据默认阻止递归触发。Agent 会话托管第一阶段由 `MessageLogic` 的 `MessageCreatedHook` 把已持久化的 `message.created` 快照交给 `ConversationHostingService`，读取 `agent_conversation_hosting` 配置和 `agent_trigger_idempotency`，再通过同一 Message Service 写回 AI 消息。

### Webhook Dispatcher

负责 IM 与 Agent 之间的异步解耦。IM 侧产生事件后，通过 Webhook 或事件投递机制通知 Agent 服务，Agent 服务处理后再将结果写回 IM。

### Message Pipeline

基于 Kafka（生产 Redpanda 单 broker，`deploy/k8s/middleware/redpanda.yaml`；本地 docker-compose redpanda）实现写路径异步解耦与削峰（03 §9 B0-B3）：msg-rpc publish `msg.toTransfer.v1`（acks=all）→ msgtransfer 分配 seq、批量落库并 produce `msg.toPostgres.v1` / `msg.toPush.v1` / `agent.trigger.v1` → push 经 msggateway 下发 `message_received`。同步 ACK 只表示 Kafka 已接受（seq=0 占位），持久化与 seq 由 push 异步回带；at-least-once 重放靠 Redis dedup 收敛。设计见 [`docs/refactor/v1/03-message-pipeline.md`](./docs/refactor/v1/03-message-pipeline.md)。

> 旧 PostgreSQL transactional outbox 链路（`message_outbox` → `internal/outboxpublisher` → transfer outbox consumer）已于 03 §9 B3b 退役删除；`message_outbox` 表保留 90 天观察后另行 DROP。历史设计见 `docs/design-docs/message-outbox.md`（已成历史文档）。

### Storage Layer

- PostgreSQL：持久化账号资料（`accounts` + `profiles`）、会话、消息、Agent 配置、工具调用记录等核心数据。
- Redis：缓存会话状态、在线状态、幂等键、热点数据和短期运行状态。Presence 场景中 Redis 只保存连接 hash、用户连接集合和短期 online marker；丢失后由 Gateway 连接重建，不作为持久业务数据权威。
- RustFS (S3-compatible) object storage：保存用户头像、图片消息和文件消息的二进制对象。PostgreSQL 的 `media_objects` 保存 owner、purpose、status、content type、size、sha256 和 object key；对象 key 由后端生成，客户端只能使用短时预签名 URL 上传/下载。

### Observability Stack

- Prometheus：指标采集
- Loki + Promtail：集群日志采集、聚合与按 `trace_id` 关联查询
- Grafana Tempo：分布式追踪存储与查询（通过 Grafana Explore）
- Grafana：统一查询 UI / dashboard，预置 Prometheus、Loki、Tempo datasource
- `trace_id`：跨服务链路追踪 ID

Backend MVP 的轻量健康检查、readiness、Prometheus text metrics 和 trace/request ID 传播设计见 [`docs/design-docs/observability-mvp.md`](./docs/design-docs/observability-mvp.md)。当前实现不要求本地启动 Prometheus、Loki、Grafana 或 Tempo。

生产分布式追踪使用 OpenTelemetry SDK + OTLP 导出到集群内 OTel Collector，再写入 Grafana Tempo 持久化存储。新请求的 canonical `trace_id` 是 OpenTelemetry trace ID，同时继续写 `X-Trace-Id` / `X-Request-Id` 兼容响应头，并支持 W3C `traceparent` / `tracestate`。REST、WebSocket、gRPC、message transfer/gateway delivery 和 Agent runtime 会创建代表性 spans；`/healthz`、`/readyz`、`/metrics` 不产生高噪声 spans。Grafana 通过 Tempo datasource 查询 trace，Tempo 不暴露公网 ingress。设计和 runbook 见 [`docs/design-docs/distributed-tracing-tempo.md`](./docs/design-docs/distributed-tracing-tempo.md)。

LLM observability is separate from system metrics/tracing. AI Hosting emits run/generation events through `internal/llmobs`, with Langfuse as the intended UI/backend sink and noop/test behavior by default. Design and privacy constraints are documented in [`docs/design-docs/llm-observability.md`](./docs/design-docs/llm-observability.md).

### Deployment / CI-CD

生产发布采用 Drone CI + GHCR + k3s + Docker Compose 的混合单机模型（GitHub Actions 已废弃，唯一 CI/部署链路是 Drone）：

- Drone CI（`.drone.yml`）承担 verify 与 deploy：`verification` pipeline 在 PR 上跑 backend/frontend verify + PostgreSQL integration；`deploy-main` pipeline 在 `main` push 时执行 detect → build → deploy → notify。已取消 `develop` 集成分支，所有变更经任务分支 PR + GitHub Merge Queue 合入 `main` 后才触发部署（见 `AGENTS.md`）。
- deploy pipeline 先执行 `detect changes`（`scripts/ci/drone-detect-deploy.sh`）：业务/镜像相关变更输出受影响后端服务列表和 `web_required`；纯 `deploy/k8s/**`、`etc/<service>.yaml`、`scripts/deploy-k3s.sh` 等变更进入 config-only deploy；文档/Markdown-only 变更不部署。
- 后端每个 Go API/RPC/worker 按动态服务矩阵构建独立镜像，web UI 仅在 web-owned 路径变更时构建独立镜像；镜像推送到 GHCR，并只打不可变 commit SHA tag。
- k3s 运行应用工作负载，包括所有 Go API、RPC、Message Transfer worker、Gateway WebSocket 和 web UI。
- 服务器中间件 PostgreSQL、Redis、RustFS、Redpanda 已迁入 k3s（manifests 在 `deploy/k8s/middleware/`）；本地开发用 Docker Compose 起 PostgreSQL、Redis、RustFS、Redpanda。消息写链路依赖 Kafka（03 §9 B2/B3b），msg-rpc / msgtransfer 缺 brokers 启动失败。
- `scripts/bootstrap-server.sh` 负责首次服务器初始化：写中间件 `.env`、启动 middleware、创建 k3s `agents-im-secrets`，并可创建 `ghcr-pull-secret`。
- `scripts/deploy-k3s.sh` 负责常规发布：启动/确认中间件、从 k3s secret 读取 `DATABASE_URL` 执行 PostgreSQL migration、刷新 GHCR pull secret、应用已渲染且保留不可变镜像 tag 的 `deploy/k8s` manifests 并等待相关 rollout。选择性镜像发布通过 `IMAGE_SERVICES` 只更新已构建服务的镜像 tag；config-only deploy 会跳过镜像更新、middleware 和 migration，只对受影响 deployment 执行 `rollout restart` / `rollout status`。

部署操作手册见 [`deploy/README.md`](./deploy/README.md)。

## 关键链路

### 用户发送消息

1. 客户端经 REST `POST /messages`（web 主路径）或 WebSocket `send_message` command 发送。
2. msg-api / msggateway 校验 JWT 身份，把 token `user_id`（account id alias）注入请求并转发 msg-rpc gRPC。
3. msg-rpc 校验（群成员、媒体引用）后 publish `message.submitted` 到 Kafka `msg.toTransfer.v1`（acks=all），ACK 返回 `server_msg_id`（seq=0 占位）；客户端以 client_msg_id 占位渲染。
4. msgtransfer 消费 toTransfer：Redis seq Malloc 分配会话内递增 seq、client_msg_id dedup，produce toPostgres / toPush / agent.trigger。
5. persist consumer 批量事务写 PostgreSQL（messages / conversation_threads / user_conversation_states）。
6. push consumer 经 gateway HTTP dispatcher 调 msggateway `/internal/delivery/conversation`，fanout `message_received` 给会话成员（含 sender 自己，用于 seq 回填）。
7. 离线收件人由 presence 判定记 offline；二段式离线推送（FCM）按 03 §9 C3 后续补齐。

### Agent 响应消息

1. IM Core 产生会话消息事件。
2. Webhook Dispatcher 或第一阶段 Agent hosting seam 将事件投递给 Agent Service。
3. Agent Service 根据 hosted conversation 配置、私聊 Agent 或显式目标 Agent 构造 trigger，并用 `event_id/server_msg_id + agent_account_id` 幂等。
4. Agent Service 加载 Agent 配置和上下文，通过 runtime seam 推理，必要时调用工具。
5. Agent Service 通过 Message Service / `MessageLogic.SendMessage` 写回 `message_origin=ai` 的普通 IM 消息。
6. IM Core 通过消息链路投递给会话成员；AI 消息默认不再触发 AI，除非策略和消息 metadata 均显式允许递归。

## 设计原则

- IM Core 与 Agent Service 解耦，避免 Agent 运行时阻塞核心消息链路。
- 写路径优先保证可靠性，再优化延迟。
- 长连接层只处理连接、投递和 ACK，不承载复杂业务逻辑。
- Agent 工具调用必须可审计、可追踪、可限制权限；Python 执行必须沙箱化，第一版不提供 shell/命令行能力。
- 所有跨服务请求必须携带 `trace_id`。

## 待细化问题

- 完整 Eino runtime orchestration、工具调用编排和 IM 写回 worker wiring 仍待细化；当前已落地 Eino DeepSeek 模型 adapter、工具解析契约和 Agent-IM runner seam。
- 服务拆分粒度与代码仓库结构。
- 消息事件 schema 见 `pkg/messaging/event.go`（`messaging.MessageEvent`）；早期 Kafka topic 设计为历史参考，broker 已移除。
- PostgreSQL 表结构和迁移方案。
- Agent 工具权限模型第一版见 `docs/design-docs/agent-system-architecture.md`，后续需随 MCP、RustFS skill 和 Python Executor 实现继续细化。

- `docs/deployment-k3s-pitfalls.md` — k3s/Drone deployment pitfalls and runbook.
