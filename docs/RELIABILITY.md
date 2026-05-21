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
- 会话内 `conversation_id + seq` 权威排序：网络到达顺序、WebSocket push 顺序和本地乐观发送顺序不得作为最终展示顺序
- 客户端按 `server_msg_id` / `client_msg_id` 去重；发现 seq gap 时按缺失区间 pull 补偿
- 同一会话本地发送必须串行化、排队或 UI 禁用到上一条发送 accepted/failed，避免本地并发发送造成展示错序
- 事件重试
- 死信队列或失败事件表
- `trace_id` 全链路追踪
- GitHub Actions 发布到 GHCR 后以 commit SHA 镜像 tag 部署，k3s rollout status 必须成功后才算发布完成
- config-only deploy 仅适用于部署配置变更：应跳过镜像更新、middleware 启动和数据库迁移，只重启并等待受影响 deployment；受影响服务列表必须与实际变更一致，不能为了通过发布而跳过异常服务
- 服务器中间件（PostgreSQL / Redis / Redpanda）由 Docker Compose 独立托管，应用发布脚本必须先确认中间件可启动，再执行迁移和 k3s rollout
- 当前 k3s manifests 使用 `hostNetwork: true`，服务会直接占用宿主机端口；新增或调整端口前必须检查宿主机端口冲突，并保持 `ListenOn`、`containerPort`、Service `port/targetPort` 一致
- 生产 tracing 启用时，OTLP endpoint/protocol/sampler 配置错误必须导致服务启动失败；不能静默降级成无 tracing。Jaeger 不可用时应用请求不应伪造成功 trace export，但 shutdown 必须尽量 flush spans。
- 排查跨服务消息链路时优先使用响应头、WebSocket ACK 或日志里的 OpenTelemetry `trace_id`，再在 Jaeger 中查看 REST/WebSocket/message outbox/transfer/gateway delivery/Agent runtime spans。

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
