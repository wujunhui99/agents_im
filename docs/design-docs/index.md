# Design Docs Index

设计文档用于记录系统级设计、重要技术决策和跨模块约束。每个设计文档都应包含背景、目标、非目标、方案、取舍、风险和验证方式。

## 核心文档

- [core-beliefs.md](./core-beliefs.md)：Agent-first 工程核心理念
- [system-boundaries.md](./system-boundaries.md)：IM 后端、Agent 系统、前端系统职责与边界
- [im-agent-contract.md](./im-agent-contract.md)：IM 与 Agent 第一阶段 API/Event Contract
- [account-service-terminology.md](./account-service-terminology.md)：Account Service 术语、account_type 和 V0 compatibility
- [user-auth-friends-groups-boundaries.md](./user-auth-friends-groups-boundaries.md)：Account/Auth/Friends/Groups 微服务边界
- [jwt-auth-middleware.md](./jwt-auth-middleware.md)：统一 JWT 鉴权中间件与 context user 规则
- [user-service-go-zero.md](./user-service-go-zero.md)：Account Service go-zero 实现设计
- [message-chain-contract.md](./message-chain-contract.md)：消息链路接口契约与 OpenIM 借鉴设计
- [kafka-message-events.md](./kafka-message-events.md)：Kafka-compatible Redpanda 消息事件 topic、schema、producer abstraction 与投递语义
- [kafka-transfer-consumer.md](./kafka-transfer-consumer.md)：Message Transfer worker 的 Kafka/Redpanda consumer、event mapping 和 offset commit 语义
- [message-storage.md](./message-storage.md)：消息存储 PostgreSQL/Redis 契约设计
- [postgres-persistence.md](./postgres-persistence.md)：第一阶段 PostgreSQL 持久化 schema、配置和 repository 设计
- [database-schema-v2.md](./database-schema-v2.md)：下一版数据库 schema 讨论稿，记录无物理外键、应用层校验、smallint 枚举、账号/Profile/Auth/Friends/Groups 表改进方向
- [message-outbox.md](./message-outbox.md)：Message Service transactional outbox 事件源与 worker 轮询契约
- [outbox-kafka-publisher.md](./outbox-kafka-publisher.md)：Outbox `message.created` 到 Kafka `message.accepted` 的发布模块与 at-least-once 语义
- [gateway-message-contract.md](./gateway-message-contract.md)：Gateway WebSocket command 到 Message Service RPC 的第一阶段映射契约
- [redis-presence.md](./redis-presence.md)：Redis 在线状态与 Gateway 连接元数据契约
- [websocket-gateway.md](./websocket-gateway.md)：WebSocket Gateway 第一阶段真实入口、JWT handshake、connection manager 和 command router
- [websocket-reconnect-sync.md](./websocket-reconnect-sync.md)：WebSocket 重连、缺失消息同步、稳定 ACK error envelope 和重复拉取契约
- [message-transfer-worker.md](./message-transfer-worker.md)：Message Transfer worker 第一阶段事件消费、投递调度和重试契约
- [gateway-push-delivery.md](./gateway-push-delivery.md)：Gateway push delivery 第一阶段 dispatcher、server push envelope 和 in-memory fanout
- [transfer-gateway-dispatcher.md](./transfer-gateway-dispatcher.md)：Transfer worker 到 Gateway delivery dispatcher 的本进程适配器契约
- [message-delivery-reliability.md](./message-delivery-reliability.md)：MVP 投递尝试模型、状态流转和重试/失败记录规则
- [gateway-presence-routing.md](./gateway-presence-routing.md)：Gateway 连接生命周期接入 PresenceStore，并为未来跨实例投递提供路由 metadata
- [read-receipts.md](./read-receipts.md)：已读回执状态模型、单调推进和扩展设计
- [observability-mvp.md](./observability-mvp.md)：Backend MVP health、readiness、metrics 和 trace/request ID 基础
- [im-agent-decoupling.md](./im-agent-decoupling.md)：IM 与 Agent 解耦设计
- [websocket-reliability.md](./websocket-reliability.md)：WebSocket 可靠性设计
- [agent-tooling.md](./agent-tooling.md)：Agent 工具调用体系设计
- [agent-system-architecture.md](./agent-system-architecture.md)：Agent 账号类型、prompt/tool/skill registry、MinIO skill 文件、MCP 和 Python Executor 第一版架构
- [agent-runtime-eino.md](./agent-runtime-eino.md)：Agent Runtime 本地接口、Eino 适配边界和 fail-first 请求/结果校验
- [agent-conversation-hosting.md](./agent-conversation-hosting.md)：Agent 会话托管、message_origin、AI 写回 Message Service 和防循环幂等设计
- [ai-reply-v1.md](./ai-reply-v1.md)：会话级 AI 回复 V1：默认关闭、手动 suggest-only 草稿、bounded context 与 summary 预留

- [backend-mvp-contract.md](./backend-mvp-contract.md)：前端开工前后端 MVP 接口契约、WebSocket 命令和投递语义
## 状态说明

- Draft：草案，尚未实现或未评审
- Accepted：已接受，作为实现依据
- Implemented：已实现并通过验证
- Deprecated：已废弃，保留历史背景

新增设计文档时，请同步更新本索引。
