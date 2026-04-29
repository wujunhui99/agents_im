# Design Docs Index

设计文档用于记录系统级设计、重要技术决策和跨模块约束。每个设计文档都应包含背景、目标、非目标、方案、取舍、风险和验证方式。

## 核心文档

- [core-beliefs.md](./core-beliefs.md)：Agent-first 工程核心理念
- [system-boundaries.md](./system-boundaries.md)：IM 后端、Agent 系统、前端系统职责与边界
- [im-agent-contract.md](./im-agent-contract.md)：IM 与 Agent 第一阶段 API/Event Contract
- [user-auth-friends-groups-boundaries.md](./user-auth-friends-groups-boundaries.md)：User/Auth/Friends/Groups 微服务边界
- [user-service-go-zero.md](./user-service-go-zero.md)：User Service go-zero 实现设计
- [message-chain-contract.md](./message-chain-contract.md)：消息链路接口契约与 OpenIM 借鉴设计
- [im-agent-decoupling.md](./im-agent-decoupling.md)：IM 与 Agent 解耦设计
- [websocket-reliability.md](./websocket-reliability.md)：WebSocket 可靠性设计
- [agent-tooling.md](./agent-tooling.md)：Agent 工具调用体系设计

## 状态说明

- Draft：草案，尚未实现或未评审
- Accepted：已接受，作为实现依据
- Implemented：已实现并通过验证
- Deprecated：已废弃，保留历史背景

新增设计文档时，请同步更新本索引。
