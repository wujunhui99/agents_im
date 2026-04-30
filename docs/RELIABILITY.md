# RELIABILITY.md

本文档记录可靠性目标和工程约束。

## 可靠性目标

- 消息写入成功后，系统应尽最大可能完成投递或提供可追踪的失败状态。
- WebSocket 断连后支持重连和消息补偿。
- Agent 处理失败不应影响 IM 核心消息链路。
- Kafka、Redis、PostgreSQL 的故障应有降级或恢复策略。

## 关键机制

- WebSocket 心跳
- 消息 ACK
- 幂等写入
- 事件重试
- 死信队列或失败事件表
- `trace_id` 全链路追踪

## 生产服务初始化

生产服务入口和 go-zero RPC `ServiceContext` 初始化必须使用显式构造和显式错误处理，不能依赖 `Must*` helper 的 panic 控制流程。初始化失败时服务仍应立即退出，但日志必须带有可排障的业务上下文，例如：

```go
repo, err := repository.NewRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
if err != nil {
    log.Fatalf("build user repository: %v", err)
}
```

当前基线已将以下生产启动路径从 `Must*` 初始化迁移为 `New*` + `log.Fatalf(...)`：

- REST/API 服务入口：`cmd/user-api/main.go`、`cmd/friends-api/main.go`、`cmd/auth-api/main.go`、`cmd/agent-api/main.go`、`cmd/groups-api/main.go`、`cmd/message-api/main.go`、`cmd/gateway-ws/main.go`
- RPC service context：`internal/rpcgen/auth/internal/svc/service_context.go`、`internal/rpcgen/friends/internal/svc/service_context.go`、`internal/rpcgen/groups/internal/svc/service_context.go`、`internal/rpcgen/message/internal/svc/service_context.go`、`internal/rpcgen/user/internal/svc/service_context.go`
- Gateway presence 初始化：`presence.NewStore(cfg.Presence, cfg.Redis)`，Redis presence 配置错误或初始化失败时必须以清晰启动错误退出。

以下不再使用的 panic 型 helper 已从生产初始化路径删除：

- `presence.MustStore`
- `repository.MustRepositoryForStorage`
- `repository.MustGroupsRepositoryForStorage`
- `repository.MustMessageRepositoryForStorage`
- `repository.MustOutboxRepositoryForStorage`
- `repository.MustAgentRepositoryForStorage`
- `repository.MustAgentAuditRepositoryForStorage`
- `repository.MustAgentRegistryRepositoryForStorage`
- `auth/repository.MustRepositoryForStorage`

`internal/repository`、`internal/auth/repository` 和 `internal/presence` 应继续提供可返回 `error` 的 `New*` 构造函数。新增生产入口或 service context 时，应复用这些构造函数并添加具体错误上下文，不应重新引入 panic 型 repository/presence 初始化 helper。

对应修复提交：`58cc0a6 Avoid panic-based service initialization`。该修复不改变业务逻辑、API 契约、数据库结构或前端行为；主要提升启动失败可观测性和可维护性。

## 待补充

- SLO / SLA
- 告警规则
- 压测目标
- 故障演练方案
