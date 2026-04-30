# 生产服务初始化可靠性修复说明

## 背景

在技术债检查中发现，部分生产服务启动路径仍使用 `Must*` 风格初始化函数，例如：

- `repository.Must*RepositoryForStorage(...)`
- `auth/repository.MustRepositoryForStorage(...)`
- `presence.MustStore(...)`

这些 helper 在初始化失败时会直接 `panic(err)`。对服务入口来说，这会带来两个问题：

1. 启动失败原因不够清晰，日志缺少明确的业务上下文。
2. 生产路径依赖 panic 控制流程，不利于运维排障和后续统一错误处理。

因此本次修复目标是：把生产服务初始化改为显式构造、显式错误处理，并删除不再需要的 panic 型 helper。

## 修复范围

### 服务入口

以下入口已从 `Must*` 初始化改为 `New*` + `log.Fatalf(...)`：

- `cmd/user-api/main.go`
- `cmd/friends-api/main.go`
- `cmd/auth-api/main.go`
- `cmd/agent-api/main.go`
- `cmd/groups-api/main.go`
- `cmd/message-api/main.go`
- `cmd/gateway-ws/main.go`

示例变化：

```go
repo, err := repository.NewRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
if err != nil {
    log.Fatalf("build user repository: %v", err)
}
```

这样服务启动失败时会输出类似 `build user repository: ...` 的明确错误上下文，而不是裸 panic。

### RPC service context

以下 go-zero rpcgen service context 也同步修复：

- `internal/rpcgen/auth/internal/svc/service_context.go`
- `internal/rpcgen/friends/internal/svc/service_context.go`
- `internal/rpcgen/groups/internal/svc/service_context.go`
- `internal/rpcgen/message/internal/svc/service_context.go`
- `internal/rpcgen/user/internal/svc/service_context.go`

### Gateway presence 初始化

`cmd/gateway-ws/main.go` 中的：

```go
presence.MustStore(cfg.Presence, cfg.Redis)
```

已改为：

```go
presenceStore, err := presence.NewStore(cfg.Presence, cfg.Redis)
if err != nil {
    log.Fatalf("build presence store: %v", err)
}
```

Redis presence 配置错误或初始化失败时，现在会以清晰启动错误退出。

## 删除的旧 helper

已删除以下不再使用的 panic 型 helper：

- `internal/presence/store.go`
  - `MustStore`
- `internal/repository/postgres_common.go`
  - `MustRepositoryForStorage`
  - `MustGroupsRepositoryForStorage`
  - `MustMessageRepositoryForStorage`
  - `MustOutboxRepositoryForStorage`
  - `MustAgentRepositoryForStorage`
  - `MustAgentAuditRepositoryForStorage`
  - `MustAgentRegistryRepositoryForStorage`
- `internal/auth/repository/postgres.go`
  - `MustRepositoryForStorage`

## 静态校验更新

`scripts/verify-static.sh` 之前仍检查旧的 `MustGroupsRepositoryForStorage` 用法。修复后已更新为检查显式构造路径，例如：

- `NewGroupsRepositoryForStorage`
- `NewMessageRepositoryForStorage`
- `NewMessageLogicWithValidators`

确保静态校验与新的初始化规范一致。

## 相关文档更新

本次顺手修正了 Agent/Eino/DeepSeek 相关陈旧描述：

- `docs/product-specs/agent-system.md`
- `docs/exec-plans/active/agent-infrastructure-parallel-baseline.md`

主要更新点：

- 不再说当前完全“不实现 LLM”。
- 明确当前 Eino/DeepSeek runtime 基线、IM runner 写回路径和工具执行 fail-closed 边界。
- 明确 `DEEPSEEK_API_KEY` 缺失或仍为 `.env.example` 占位值时必须失败。

## 验证结果

修复后已通过以下验证：

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go vet ./...
bash scripts/verify-static.sh
git diff --check
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go run ./cmd/single-machine-e2e
npm run frontend:test
npm run frontend:build
npm run frontend:lint
```

验证结果均通过。

## 提交

本次修复已提交：

```text
58cc0a6 Avoid panic-based service initialization
```

提交后仓库状态：

```text
main...origin/main [ahead 2]
working tree clean
```

## 影响

- 服务初始化失败仍会立即退出，但退出原因更明确。
- 生产路径不再依赖 panic 型 helper 初始化 repository/presence。
- 不改变业务逻辑、API 契约、数据库结构或前端行为。
- 对运行时成功路径无功能性变更；主要提升启动可靠性和可维护性。
