# ARCHITECTURE.md

本文档提供项目的顶层架构地图，帮助人类和 AI Agent 快速理解系统边界、核心模块和关键数据流。

## 系统目标

构建一个高性能、分布式、实时聊天系统，同时提供 Agent 服务能力。系统需要支持：

- 用户单聊与群聊
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

### User Service

负责用户账号资料的权威数据，不管理密码或认证秘密。核心能力包括唯一标识符（类似微信号）、名称、性别、年龄、地区等资料维护，`/me` 查询，公开资料查询，以及供 `auth` 注册流程使用的账号存在性检查。

### Auth Service

负责认证和登录注册。第一阶段支持账号密码注册/登录，密码和认证秘密只归 `auth` 管理；注册时依赖 `user` 查询账号是否存在，并协作创建用户资料。手机号验证码、微信扫码等能力作为后续扩展，当前不实现。

### Friends Service

负责好友关系维护，包括添加好友、删除好友、查询好友列表和关系状态。好友关系不写入 `user` 的权威资料模型。

### Groups Service

负责群聊和群成员关系维护，包括创建群、加群、退群、查询群成员。群成员关系不写入 `user` 的权威资料模型。

### Message Service

负责消息链路第一阶段契约和实现，包括发送消息、生成 `server_msg_id`、维护会话内递增 `seq`、同步存储消息、按 seq 拉取消息、维护 `user_id + conversation_id -> has_read_seq` 已读状态，并通过 PostgreSQL transactional outbox 为后续 Kafka、Message Transfer、Push 服务提供可靠事件源。设计见 [`docs/design-docs/message-chain-contract.md`](./docs/design-docs/message-chain-contract.md) 和 [`docs/design-docs/message-outbox.md`](./docs/design-docs/message-outbox.md)，产品规格见 [`docs/product-specs/message-chain.md`](./docs/product-specs/message-chain.md)。

### Message Transfer Worker

负责消费 Message Outbox 或 Kafka/Redpanda 中的 `message.accepted` 事件，并通过 Delivery Dispatcher 触发在线投递、离线推送或后续 delivery ACK 流程。第一阶段提供独立入口 `cmd/message-transfer`；默认可使用 in-memory consumer/noop dispatcher，不依赖真实 Kafka、Redpanda、PostgreSQL outbox 或 Gateway fanout。Kafka consumer adapter 消费 `message.events.v1`，将 canonical `messaging.MessageEvent` 映射为 worker `Envelope`，成功处理后提交 offset，retry/failed hook 保留给后续 retry topic 或 dead-letter policy。当前已提供 `internal/transfer/gateway` 适配器，将 Transfer worker 的 `DeliveryDispatcher` 接口桥接到 Gateway `delivery.Dispatcher` 契约，用于本进程内 Gateway 投递集成测试和后续共址 wiring；它不实现远程 Gateway 网络调用或 Redis 跨实例路由。MVP 投递可靠性通过 `delivery_attempts` 记录 `accepted`、`published`、`delivered`、`offline`、`failed`，其中 `delivered` 不等于已读；Worker 不拥有消息历史、会话 seq 或已读状态，这些仍由 Message Service 和 PostgreSQL 权威维护。设计见 [`docs/design-docs/message-transfer-worker.md`](./docs/design-docs/message-transfer-worker.md)、[`docs/design-docs/kafka-transfer-consumer.md`](./docs/design-docs/kafka-transfer-consumer.md)、[`docs/design-docs/transfer-gateway-dispatcher.md`](./docs/design-docs/transfer-gateway-dispatcher.md) 和 [`docs/design-docs/message-delivery-reliability.md`](./docs/design-docs/message-delivery-reliability.md)。

### IM Core Service

负责 IM 核心业务链路，包括用户会话、消息收发、消息状态、会话成员管理等。

### Gateway / WebSocket Service

负责客户端 WebSocket 长连接管理，包括：

- 连接建立与鉴权
- 心跳检测
- ACK 确认
- 在线状态维护
- 消息下发与重试

在线状态和连接元数据通过 Redis presence 层保存为短期运行状态，设计见 [`docs/design-docs/redis-presence.md`](./docs/design-docs/redis-presence.md)。第一阶段已提供独立入口 `cmd/gateway-ws`，通过 `GET /ws` 建立 WebSocket 连接；Handshake 使用与 HTTP API 一致的 JWT 配置，支持 `Authorization: Bearer <token>` 和 `token` query param；连接通过内存 connection manager 按 `user_id` 注册多端 `connection_id`，并同步写入 `PresenceStore`。Gateway command router 支持 `heartbeat`、`send_message`、`pull_messages`、`get_conversation_seqs`、`mark_conversation_read`，并调用现有 Message Service 业务逻辑/仓储契约完成消息写入、拉取、seq 查询和已读推进；heartbeat 会刷新 presence TTL。Frontend reconnect sync 使用稳定 WebSocket ACK error envelope，并通过 `get_conversation_seqs`、`pull_messages`、`mark_conversation_read` 补偿缺失消息，产品契约见 [`docs/product-specs/frontend-sync-contract.md`](./docs/product-specs/frontend-sync-contract.md)，设计见 [`docs/design-docs/websocket-reconnect-sync.md`](./docs/design-docs/websocket-reconnect-sync.md)。Gateway push delivery 第一阶段提供 `internal/gateway/delivery.Dispatcher` 契约和本进程内 WebSocket fanout，可向在线连接主动下发 `message_received` / `message_delivered` event；delivery dispatcher 会先查询 presence route metadata，再执行本进程内 fanout，offline/routed/failed recipient 均返回明确状态。Gateway 不拥有消息历史、会话 seq 或已读状态；这些数据仍由 Message Service 和 PostgreSQL 权威维护。后续 Message Transfer worker 将消费 outbox/Kafka 事件并调用 dispatcher，Redis Presence route metadata 用于跨实例路由，delivery ACK 留给后续链路补齐。设计见 [`docs/design-docs/websocket-gateway.md`](./docs/design-docs/websocket-gateway.md)、[`docs/design-docs/gateway-push-delivery.md`](./docs/design-docs/gateway-push-delivery.md) 和 [`docs/design-docs/gateway-presence-routing.md`](./docs/design-docs/gateway-presence-routing.md)。

### Agent Service

负责 Agent 生命周期、配置组装、运行时能力和工具调用审计。第一版设计见 [`docs/product-specs/agent-system.md`](./docs/product-specs/agent-system.md)、[`docs/design-docs/agent-system-architecture.md`](./docs/design-docs/agent-system-architecture.md) 和 [`docs/exec-plans/active/agent-system-v0.md`](./docs/exec-plans/active/agent-system-v0.md)。核心能力包括：

- 在账号系统中配合 `normal` / `agent` / `admin` 账号类型，让 Agent 账号作为 IM 会话成员参与单聊和群聊。
- 当前 `feature/agent-core-management` 提供 `cmd/agent-api` 和 `api/agent.api` 的 Agent profile 管理基础，配置单独持久化到 `agents` 表；由于本分支尚未合入 `users.account_type`，创建 Agent 使用 fail-closed 的账号类型检查接口，不能验证 `account_type=agent` 时必须失败。
- 管理系统提示词、工具、Agent skills 和 Agent 配置，并将元数据持久化在 PostgreSQL。
- 使用系统提示词、工具和 skills 组装 Agent runtime。
- 通过 MinIO/S3-compatible object storage 保存 Agent skill 文件；Agent 绑定 skill 后默认可读取该 skill 文件，但不能越权读取其他文件。
- 管理 MCP 工具和本地工具。MCP server 和工具元数据入库；本地工具只允许服务端白名单 `handler_key`，不得从数据库执行任意脚本。
- 当前 Agent registry 基线已提供 prompt/tool/skill 元数据与 Agent 白名单绑定的 Go logic/repository 和 PostgreSQL schema；该基线不执行工具、不调用 LLM、不上传或读取 MinIO 二进制内容。
- Agent run、tool call、skill file read、Python exec 审计记录使用 append-only 审计表保存；摘要字段必须脱敏，Python 代码只保存 hash/大小摘要。
- 第一版不提供 shell/命令行脚本执行能力；Python 执行必须通过受限沙箱、限时限资源、默认无网络，并记录审计。
- 当前 Python executor 只提供 `internal/agent/pythonexec` 契约和 disabled 默认实现；未配置真实沙箱时必须返回 `ErrPythonExecutorDisabled`，不得在 Go 主服务进程内直接运行 Python 或 shell。
- Agent 响应必须通过 Message Service 写回 IM，不能绕过 IM 消息链路或直接推送 WebSocket。
- Agent-IM 第一阶段 Go 契约位于 `internal/agentim`：定义用户私聊 Agent、群聊 @Agent、管理员手动 run 三类触发，响应 writer 只依赖 `MessageLogic.SendMessage` / Message Service seam，并通过 Agent 消息元数据默认阻止递归触发。

### Webhook Dispatcher

负责 IM 与 Agent 之间的异步解耦。IM 侧产生事件后，通过 Webhook 或事件投递机制通知 Agent 服务，Agent 服务处理后再将结果写回 IM。

### Message Pipeline

基于 Kafka-compatible Redpanda 本地开发环境、`message.events.v1` 事件契约和 PostgreSQL transactional outbox 实现消息异步解耦与削峰，支撑 Message Transfer worker、Gateway fanout 和 Push 链路。Message Service 当前仍同步写 PostgreSQL，并在同一 transaction 内写入 `message_outbox` 的 `message.created` 事件；因此同步 ACK 只表示消息已被 Message Service 接受和持久化，不表示收件端已送达。Outbox Kafka Publisher 将 pending outbox rows 转换为 `message.accepted` 并通过 `messaging.Producer` 发布，采用 at-least-once 语义。事件 topic、schema、producer abstraction 与投递语义见 [`docs/design-docs/kafka-message-events.md`](./docs/design-docs/kafka-message-events.md)，outbox 设计见 [`docs/design-docs/message-outbox.md`](./docs/design-docs/message-outbox.md)，publisher 设计见 [`docs/design-docs/outbox-kafka-publisher.md`](./docs/design-docs/outbox-kafka-publisher.md)。

### Storage Layer

- PostgreSQL：持久化用户、会话、消息、Agent 配置、工具调用记录等核心数据。
- Redis：缓存会话状态、在线状态、幂等键、热点数据和短期运行状态。Presence 场景中 Redis 只保存连接 hash、用户连接集合和短期 online marker；丢失后由 Gateway 连接重建，不作为持久业务数据权威。

### Observability Stack

- Prometheus：指标采集
- Grafana：监控面板
- Jaeger：分布式追踪
- `trace_id`：跨服务链路追踪 ID

Backend MVP 的轻量健康检查、readiness、Prometheus text metrics 和 trace/request ID 传播设计见 [`docs/design-docs/observability-mvp.md`](./docs/design-docs/observability-mvp.md)。当前实现不要求本地启动 Prometheus、Grafana 或 Jaeger。

## 关键链路

### 用户发送消息

1. 客户端通过 WebSocket 发送 `send_message` command。
2. Gateway 校验连接 JWT 身份，并把 token `user_id` 注入消息发送请求。
3. Message Service 写入消息，生成 `server_msg_id` 和会话内递增 `seq`。
4. Message Service 在同一 PostgreSQL transaction 内写入 `message_outbox` 的 `message.created` 事件。
5. Gateway 返回 command ACK。第一阶段 ACK 只表示消息业务命令完成，不表示收件端在线送达。
6. Gateway 当前可通过 in-memory dispatcher 向本实例在线连接主动下发 server push event。
7. Message Transfer Worker 后续从 Message Outbox 或 Kafka/Redpanda 消费消息事件，并调度 Gateway/Push 投递。
8. 跨进程 Gateway fanout、Redis Presence 路由、Push worker 和 delivery ACK 由后续链路继续补齐。

### Agent 响应消息

1. IM Core 产生会话消息事件。
2. Webhook Dispatcher 将事件投递给 Agent Service。
3. Agent Service 加载 Agent 配置和上下文。
4. Agent 根据上下文推理，必要时调用工具。
5. Agent Service 将响应写回 IM Core。
6. IM Core 通过消息链路投递给会话成员。

## 设计原则

- IM Core 与 Agent Service 解耦，避免 Agent 运行时阻塞核心消息链路。
- 写路径优先保证可靠性，再优化延迟。
- 长连接层只处理连接、投递和 ACK，不承载复杂业务逻辑。
- Agent 工具调用必须可审计、可追踪、可限制权限；Python 执行必须沙箱化，第一版不提供 shell/命令行能力。
- 所有跨服务请求必须携带 `trace_id`。

## 待细化问题

- Agent 框架最终选择：LangChain 系列或 Google ADK。
- 服务拆分粒度与代码仓库结构。
- Kafka topic 设计与消息 schema 第一版见 [`docs/design-docs/kafka-message-events.md`](./docs/design-docs/kafka-message-events.md)，后续需随 outbox/transfer/push 实现继续细化。
- PostgreSQL 表结构和迁移方案。
- Agent 工具权限模型第一版见 `docs/design-docs/agent-system-architecture.md`，后续需随 MCP、MinIO skill 和 Python Executor 实现继续细化。
