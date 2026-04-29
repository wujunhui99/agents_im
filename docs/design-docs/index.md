# Design Docs Index

设计文档用于记录系统级设计、重要技术决策和跨模块约束。每个设计文档都应包含背景、目标、非目标、方案、取舍、风险和验证方式。

## 核心文档

- [core-beliefs.md](./core-beliefs.md)：Agent-first 工程核心理念
- [system-boundaries.md](./system-boundaries.md)：IM 后端、Agent 系统、前端系统职责与边界
- [im-agent-contract.md](./im-agent-contract.md)：IM 与 Agent 第一阶段 API/Event Contract
- [user-auth-friends-groups-boundaries.md](./user-auth-friends-groups-boundaries.md)：User/Auth/Friends/Groups 微服务边界
- [jwt-auth-middleware.md](./jwt-auth-middleware.md)：统一 JWT 鉴权中间件与 context user 规则
- [user-service-go-zero.md](./user-service-go-zero.md)：User Service go-zero 实现设计
- [message-chain-contract.md](./message-chain-contract.md)：消息链路接口契约与 OpenIM 借鉴设计
- [kafka-message-events.md](./kafka-message-events.md)：Kafka-compatible Redpanda 消息事件 topic、schema、producer abstraction 与投递语义
- [message-storage.md](./message-storage.md)：消息存储 PostgreSQL/Redis 契约设计
- [postgres-persistence.md](./postgres-persistence.md)：第一阶段 PostgreSQL 持久化 schema、配置和 repository 设计
- [message-outbox.md](./message-outbox.md)：Message Service transactional outbox 事件源与 worker 轮询契约
- [gateway-message-contract.md](./gateway-message-contract.md)：Gateway WebSocket command 到 Message Service RPC 的第一阶段映射契约
- [redis-presence.md](./redis-presence.md)：Redis 在线状态与 Gateway 连接元数据契约
- [websocket-gateway.md](./websocket-gateway.md)：WebSocket Gateway 第一阶段真实入口、JWT handshake、connection manager 和 command router
- [read-receipts.md](./read-receipts.md)：已读回执状态模型、单调推进和扩展设计
- [im-agent-decoupling.md](./im-agent-decoupling.md)：IM 与 Agent 解耦设计
- [websocket-reliability.md](./websocket-reliability.md)：WebSocket 可靠性设计
- [agent-tooling.md](./agent-tooling.md)：Agent 工具调用体系设计

## 状态说明

- Draft：草案，尚未实现或未评审
- Accepted：已接受，作为实现依据
- Implemented：已实现并通过验证
- Deprecated：已废弃，保留历史背景

新增设计文档时，请同步更新本索引。
