# Tech Debt Tracker

本文档集中记录已知技术债，避免技术债只存在于聊天记录或临时代办中。

本次更新仅对齐当前 `main` 的文档与代码静态状态，不声称已经完成真实 Kafka、Message Transfer、LLM/MCP 工具调用或 Python 沙箱端到端验证。

| ID | 标题 | 影响范围 | 优先级 | 状态 | 备注 |
| --- | --- | --- | --- | --- | --- |
| TD-001 | Agent runtime 框架选型未定 | Agent Service | Medium | Closed | 已形成 CloudWeGo Eino + DeepSeek ChatModel adapter/config 基线；完整 runtime orchestration、registry/context loader、tool adapter 和 live provider 集成不因本项关闭而视为完成，剩余工作在 TD-003 等更具体条目中跟踪 |
| TD-002 | Kafka topic 与消息 schema 未定 | Message Pipeline | High | Closed | 已明确 `message.events.v1`、canonical `messaging.MessageEvent` JSON schema、`messaging.Producer` 抽象、PostgreSQL transactional outbox、Outbox Kafka Publisher baseline 和 Kafka Transfer Consumer baseline；生产 retry/DLQ/运维策略剩余债务转入 TD-006 |
| TD-003 | Agent 工具执行与集成策略待完成验证 | Agent Tooling | High | Open | 基础 tool resolver、Agent 绑定白名单、active/admin MCP server 校验、安全 transport 限制、local/builtin 白名单和审计边界已落地；仍缺真实工具执行 adapter、MCP live 调用、Eino tool calling wiring、MinIO skill read/Python tool 集成策略与策略测试完整验证，不能因 resolver metadata 基线已完成而关闭 |
| TD-004 | Python Executor 真实沙箱方案待落地 | Agent Runtime | High | Open | 当前 `internal/agent/pythonexec` 是 Go 侧 contract 加 fail-closed disabled 默认实现；有效请求仍必须返回 `ErrPythonExecutorDisabled`，真实独立沙箱、资源隔离、默认无网络运行时和生产 wiring 未落地 |
| TD-005 | 生产服务初始化依赖 panic 型 helper | API/RPC/Gateway startup | High | Closed | `58cc0a6 Avoid panic-based service initialization` 已将 REST/API 入口、RPC service context 和 Gateway presence 初始化迁移为 `New*` + 显式 `log.Fatalf(...)` 错误上下文，并删除不再使用的 `Must*RepositoryForStorage` / `presence.MustStore` helper；规范见 `docs/RELIABILITY.md` |
| TD-006 | Kafka 生产 retry/DLQ 与运维策略待定 | Message Pipeline | High | Open | Outbox publisher 已记录 retry metadata，Kafka consumer 成功后提交 offset 且 retry/failed 不提交；生产前仍需明确 retry topic 或 outbox-backed retry、dead-letter/poison event 提交条件、durable idempotency、publisher 生产入口/config wiring、指标日志告警和运维恢复策略 |
