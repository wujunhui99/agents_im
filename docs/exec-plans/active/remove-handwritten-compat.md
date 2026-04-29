# remove-handwritten-compat

状态：Active

## 背景

`goctl-refactor` 已把 REST handler/types/routes 和 RPC pb/zrpc scaffold 生成到仓库，但为兼容既有测试和命令入口仍保留了旧 `net/http` mux 注册函数、旧 hand-written RPC wrapper，以及 `cmd/*-rpc` 的 HTTP healthz wrapper。当前任务要求删除这些兼容层，并把可由 goctl/go-zero 承载的 transport/scaffold 切换到生成结构。

## 目标

- 删除旧 REST mux 注册层：`internal/handler/handler.go`、`internal/handler/groups_handler.go`、`internal/handler/message_handler.go`、`internal/auth/handler/handler.go`，以及实际存在的同类旧文件。
- 保留 goctl REST 结构：`internal/handler/gozero_routes.go`、`internal/auth/handler/gozero_routes.go`、`internal/handler/{user,friends,groups,message}`、`internal/auth/handler/auth`、`internal/types/types.go`。
- 删除旧 RPC wrapper：`internal/rpc`、`internal/auth/rpc`。
- 将 `internal/rpcgen/*/internal/logic` 接入现有 business logic/repository/domain，避免 RPC generated logic 为空壳。
- 将 `cmd/{user,auth,friends,groups,message}-rpc` 切换到 go-zero `zrpc` 启动路径，不再导入旧 RPC wrapper。
- 更新测试、静态校验和相关设计/执行文档，确保旧兼容层不会回归。

## 非目标

- 不实现 JWT 鉴权；继续保留当前本地开发用 `X-User-Id` header。
- 不实现 PostgreSQL 持久化；继续使用内存 repository。
- 不修改 main/develop，不合并其他分支。
- 不输出或提交 secret/token。

## 任务拆分

- [x] Planner：读取 `AGENTS.md`、`ARCHITECTURE.md`、zero-skills references、`docs/PLANS.md` 和 `docs/exec-plans/active/goctl-refactor.md`。
- [x] Planner：盘点旧 REST/RPC 兼容层、goctl scaffold、命令入口、测试和文档引用。
- [x] Generator：把 REST 测试从旧 mux 注册函数迁移到 go-zero route registration。
- [x] Generator：接通 `internal/rpcgen/*/internal/logic` 到现有业务 logic/repository/domain。
- [x] Generator：将 `cmd/*-rpc` 切换到 go-zero `zrpc` generated server 入口，并补齐 `cmd/message-rpc`。
- [x] Generator：删除旧 REST mux 文件和旧 RPC wrapper 目录。
- [x] Generator：更新 `scripts/verify-static.sh` 和相关设计/执行文档。
- [x] Evaluator：执行强制验证并记录结果。
- [ ] 收尾：提交并推送 `feature/remove-handwritten-compat`。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | REST 测试通过 go-zero route registration 构造 router | 避免依赖旧 `RegisterHandlers` mux 兼容函数，同时不需要启动真实监听端口。 |
| 2026-04-29 | RPC generated logic 直接调用现有 business logic | 保留业务行为与内存 repository 边界，只替换 transport/scaffold。 |
| 2026-04-29 | `cmd/*-rpc` 通过 `internal/rpcgen/<service>/entry` 启动 zrpc server | Go 的 nested `internal` import 规则不允许 `cmd/*` 直接导入 `internal/rpcgen/<service>/internal/server`，因此在 generated scaffold 父目录下提供薄入口。 |
| 2026-04-29 | `.api` 中 `X-User-Id` 标记为 optional，由 logic 做 401 判定 | go-zero parser 对必填 header 的解析错误早于业务 logic；保持既有缺少身份返回 `UNAUTHENTICATED` 的行为。 |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
protoc --version
protoc-gen-go --version
protoc-gen-go-grpc --version
gofmt -w $(find . -name "*.go" -print)
go test ./...
bash scripts/verify-static.sh
git status --short --branch
```

## 风险与回滚

- 删除旧 mux/RPC wrapper 后，任何仍引用旧函数或旧 package 的测试、命令或文档都会在编译或静态校验中失败。
- RPC 默认仍使用内存 repository，进程重启或跨服务持久化不是本任务范围；后续 PG/docker-compose 任务再处理。
- 如 zrpc 命令入口启动配置不匹配，应修正 `etc/*-rpc.yaml` 与 generated config，而不是恢复旧 HTTP wrapper。

## 结果记录

- 已删除旧 REST mux 文件：`internal/handler/handler.go`、`internal/handler/groups_handler.go`、`internal/handler/message_handler.go`、`internal/auth/handler/handler.go`。
- 已删除旧 RPC wrapper：`internal/rpc/*_server.go`、`internal/auth/rpc/auth_server.go`；空目录已移除。
- 已补齐 `cmd/message-rpc/main.go` 和 `etc/message-rpc.yaml`。
- `cmd/{user,auth,friends,groups,message}-rpc` 均通过 `internal/rpcgen/<service>/entry` 启动 go-zero `zrpc` server。
- `internal/rpcgen/{user,auth,friends,groups,message}/internal/logic` 已调用现有业务 logic，并通过 `internal/rpcgen/rpcerror` 将 `apperror` 映射为 gRPC status。
- REST 测试已改为通过 go-zero route registration 构造 router，不再调用旧 mux 注册函数。
- 静态校验已增加旧兼容层删除检查、RPC command import 检查、generated RPC logic 空壳检查和 REST 测试旧注册函数检查。

已运行 goctl 相关命令：

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
goctl api go -api api/user.api -dir .tmp-goctl-check/user --style go_zero
goctl api go -api api/friends.api -dir .tmp-goctl-check/friends --style go_zero
goctl api go -api api/groups.api -dir .tmp-goctl-check/groups --style go_zero
goctl api go -api api/message.api -dir .tmp-goctl-check/message --style go_zero
rm -rf .tmp-goctl-check
```

验证结果：

- `goctl --version`：`goctl version 1.10.1 linux/amd64`
- `for f in api/*.api; do goctl api validate -api "$f"; done`：5 个 API 均输出 `api format ok`
- `protoc --version`：`libprotoc 3.19.4`
- `protoc-gen-go --version`：`protoc-gen-go v1.36.11`
- `protoc-gen-go-grpc --version`：`protoc-gen-go-grpc 1.6.1`
- `gofmt -w $(find . -name "*.go" -print)`：完成
- `go test ./...`：通过，包含 `tests` 包
- `bash scripts/verify-static.sh`：`static verification passed`
- `git status --short --branch`：位于 `feature/remove-handwritten-compat`，变更待提交
