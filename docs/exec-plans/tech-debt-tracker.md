# Tech Debt Tracker

本文档集中记录已知技术债，避免技术债只存在于聊天记录或临时代办中。

| ID | 标题 | 影响范围 | 优先级 | 状态 | 备注 |
| --- | --- | --- | --- | --- | --- |
| TD-001 | Agent runtime 框架选型未定 | Agent Service | Medium | Superseded | 已形成 Eino/DeepSeek runtime 基线；后续技术债转为完善 production orchestration、registry/context loader 与 live provider 验证 |
| TD-002 | Kafka topic 与消息 schema 未定 | Message Pipeline | High | Open | 需要在首个消息链路实现前确定 |
| TD-003 | Agent 工具权限模型需实现验证 | Agent Tooling | High | Open | 第一版边界已定义：MCP/本地工具白名单、skill 文件授权、Python 沙箱、无 shell；实现时需补审计和策略测试 |
| TD-004 | Python Executor 沙箱方案待落地 | Agent Runtime | High | Open | 第一版允许受限 Python，但必须独立沙箱、限时限资源、默认无网络，不能在主服务进程内直接执行任意代码 |
| TD-005 | 生产服务初始化依赖 panic 型 helper | API/RPC/Gateway startup | High | Closed | `58cc0a6 Avoid panic-based service initialization` 已将 REST/API 入口、RPC service context 和 Gateway presence 初始化迁移为 `New*` + 显式 `log.Fatalf(...)` 错误上下文，并删除不再使用的 `Must*RepositoryForStorage` / `presence.MustStore` helper；规范见 `docs/RELIABILITY.md` |
