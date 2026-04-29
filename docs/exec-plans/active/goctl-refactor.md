# goctl-refactor

状态：Active（Task 1 REST + RPC goctl generation completed; Task 2/3 deferred）

## 背景

当前仓库已有手写 go-zero 风格服务代码：HTTP 层使用 `net/http` mux，业务集中在 `internal/logic` 和 `internal/auth/logic`，仓储仍为内存实现。任务1要求安装并使用 goctl 对现有 REST 代码做最小重构，保持行为不变；任务2 JWT 鉴权和任务3 PostgreSQL 持久化暂不实现。

## 目标

- 对 `api/*.api` 全部执行 `goctl api validate`，修正为可生成且能表达现有 path/header 输入的 spec。
- 使用 goctl 1.10.1 从 `api/*.api` 生成 REST scaffold，并把安全的 handler/types/routes 结构迁回现有项目。
- 保留 user/auth/friends/groups/message/gateway/read/storage 现有业务逻辑和测试行为。
- 对 `proto/*.proto` 使用 `goctl rpc protoc` 正式生成可维护的 RPC pb 与 zrpc scaffold，保留现有 hand-written RPC contract 兼容层。
- 更新静态校验脚本，确保 zero-skills、goctl-refactor 文档和 API validate 都在校验范围内。

## 非目标

- 不实现任务2 JWT 鉴权；本阶段继续保留现有 `X-User-Id` 本地开发头。
- 不实现任务3 PostgreSQL 持久化；本阶段继续使用内存 repository。
- 不修改 main/develop，不合并其他分支。
- 不输出或提交 secret/token。

## 任务拆分

- [x] Planner：读取 `AGENTS.md`、`ARCHITECTURE.md`、zero-skills references 和 Codex guide。
- [x] Planner：盘点现有 `api/*.api`、`proto/*.proto`、手写 handler/logic/service context 和静态校验脚本。
- [x] Generator：执行 `goctl api validate`，确认初始 API spec 可被 goctl 解析。
- [x] Generator：补齐 REST 生成所需的 header/path request fields，保持现有路由不变。
- [x] Generator：使用 goctl 生成临时 REST scaffold，并迁移 handler/types/routes 结构到现有项目。
- [x] Generator：保留现有业务逻辑，新增 route-level logic adapters 调用原 `internal/logic` 和 `internal/auth/logic`。
- [x] Generator：将 API 命令入口切换到 go-zero `rest.Server`，保留旧 mux 注册函数供现有测试和兼容调用使用。
- [x] Generator：使用 `goctl rpc protoc` 从 `proto/{user,auth,friends,groups,message}.proto` 正式生成 pb 与 zrpc scaffold。
- [x] Generator：保留现有 hand-written RPC contract server 和 `cmd/*-rpc` healthz 行为，避免破坏现有测试。
- [x] Generator：更新静态校验脚本，检查 RPC generated scaffold、goctl marker 和 service server 注册代码。
- [x] Evaluator：运行强制验证并修复问题。
- [x] Evaluator：commit 并 push 到 `feature/goctl-refactor`。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 不直接覆盖现有 `internal/logic` 业务代码 | goctl 生成 skeleton 会覆盖行为风险高，任务要求保留现有业务逻辑。 |
| 2026-04-29 | 先生成到 `.tmp-goctl-gen/*` 后迁移安全文件 | 避免多 API 同时生成到仓库根目录导致多个 root `main` 和已有文件冲突。 |
| 2026-04-29 | `.api` 增加 `X-User-Id` header 与 path request fields | goctl handler 需要通过 typed request 获取现有手写 handler 原本从 header/path 读取的值。 |
| 2026-04-29 | 新增 route-level logic adapter packages | 保持 go-zero Handler -> Logic -> ServiceContext 结构，同时复用原业务逻辑。 |
| 2026-04-29 | 任务2 JWT 暂不做，继续保留 `X-User-Id` | 本阶段只做 goctl 结构重构，JWT 是后续明确边界。 |
| 2026-04-29 | 任务3 PG 暂不做，PG 本地开发后续通过 docker-compose 配置 | 本阶段不新增 PostgreSQL model 或连接；后续中间件和 PG 本地依赖统一进 docker-compose。 |
| 2026-04-29 | RPC pb 生成到 `proto/*pb`，zrpc scaffold 生成到 `internal/rpcgen/<service>` | 使用 `--go_opt=module=github.com/wujunhui99/agents_im` 避免 `github.com/...` 包路径嵌套；独立 scaffold 目录避免覆盖现有 hand-written RPC contract。 |
| 2026-04-29 | 保留 `internal/rpc` 与 `internal/auth/rpc` 兼容层 | 当前 RPC contract 仍承载测试和现有命令入口行为；goctl scaffold 作为正式生成基线，后续可逐步把业务适配接入 generated server logic。 |

## 已运行的 goctl 命令

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH goctl --version
PATH=/tmp/go/bin:$HOME/go/bin:$PATH; for f in api/*.api; do goctl api validate -api "$f"; done
PATH=/tmp/go/bin:$HOME/go/bin:$PATH goctl api go -api api/user.api -dir .tmp-goctl-gen/user --style go_zero
PATH=/tmp/go/bin:$HOME/go/bin:$PATH goctl api go -api api/auth.api -dir .tmp-goctl-gen/auth --style go_zero
PATH=/tmp/go/bin:$HOME/go/bin:$PATH goctl api go -api api/friends.api -dir .tmp-goctl-gen/friends --style go_zero
PATH=/tmp/go/bin:$HOME/go/bin:$PATH goctl api go -api api/groups.api -dir .tmp-goctl-gen/groups --style go_zero
PATH=/tmp/go/bin:$HOME/go/bin:$PATH goctl api go -api api/message.api -dir .tmp-goctl-gen/message --style go_zero
PATH=/tmp/go/bin:$HOME/go/bin:$PATH goctl rpc protoc proto/user.proto --go_out=. --go_opt=module=github.com/wujunhui99/agents_im --go-grpc_out=. --go-grpc_opt=module=github.com/wujunhui99/agents_im --zrpc_out=internal/rpcgen/user --style go_zero
PATH=/tmp/go/bin:$HOME/go/bin:$PATH goctl rpc protoc proto/auth.proto --go_out=. --go_opt=module=github.com/wujunhui99/agents_im --go-grpc_out=. --go-grpc_opt=module=github.com/wujunhui99/agents_im --zrpc_out=internal/rpcgen/auth --style go_zero
PATH=/tmp/go/bin:$HOME/go/bin:$PATH goctl rpc protoc proto/friends.proto --go_out=. --go_opt=module=github.com/wujunhui99/agents_im --go-grpc_out=. --go-grpc_opt=module=github.com/wujunhui99/agents_im --zrpc_out=internal/rpcgen/friends --style go_zero
PATH=/tmp/go/bin:$HOME/go/bin:$PATH goctl rpc protoc proto/groups.proto --go_out=. --go_opt=module=github.com/wujunhui99/agents_im --go-grpc_out=. --go-grpc_opt=module=github.com/wujunhui99/agents_im --zrpc_out=internal/rpcgen/groups --style go_zero
PATH=/tmp/go/bin:$HOME/go/bin:$PATH goctl rpc protoc proto/message.proto --go_out=. --go_opt=module=github.com/wujunhui99/agents_im --go-grpc_out=. --go-grpc_opt=module=github.com/wujunhui99/agents_im --zrpc_out=internal/rpcgen/message --style go_zero
```

## 生成结果

- goctl 版本：`goctl version 1.10.1 linux/amd64`。
- `api/*.api` 均通过 `goctl api validate`。
- goctl 临时生成目录：`.tmp-goctl-gen/{user,auth,friends,groups,message}`。
- 已迁移 REST handler packages：
  - `internal/handler/user`
  - `internal/handler/friends`
  - `internal/handler/groups`
  - `internal/handler/message`
  - `internal/auth/handler/auth`
- 已迁移统一 generated types：`internal/types/types.go`。
- 已新增 go-zero routes：
  - `internal/handler/gozero_routes.go`
  - `internal/auth/handler/gozero_routes.go`
- 已新增 route-level logic adapters：
  - `internal/logic/user/gozero_logic.go`
  - `internal/logic/friends/gozero_logic.go`
  - `internal/logic/groups/gozero_logic.go`
  - `internal/logic/message/gozero_logic.go`
  - `internal/auth/logic/auth/gozero_logic.go`
- 已新增正式 RPC generated pb packages：
  - `proto/userpb`
  - `proto/authpb`
  - `proto/friendspb`
  - `proto/groupspb`
  - `proto/messagepb`
- 已新增正式 goctl zrpc scaffold：
  - `internal/rpcgen/user`
  - `internal/rpcgen/auth`
  - `internal/rpcgen/friends`
  - `internal/rpcgen/groups`
  - `internal/rpcgen/message`

## 保留的手写逻辑

- `internal/logic/userlogic.go`：用户资料、唯一 identifier、`/me` 资料更新行为保留。
- `internal/auth/logic/authlogic.go`：注册、登录、token 生成/校验行为保留；未切换 JWT。
- `internal/logic/friendslogic.go`：好友关系幂等添加、删除、列表和关系查询保留。
- `internal/logic/groupslogic.go`：群创建、成员添加/加入/退出/列表行为保留。
- `internal/logic/messagelogic.go`：消息去重、seq、拉取、已读推进行为保留。
- `internal/repository/*`、`internal/domain/readreceipt/*`、`internal/gateway/*` 保留，不做 PG 或 Gateway 行为重写。
- 旧 `net/http` mux 注册函数保留，避免现有测试和兼容调用断裂。

## RPC goctl 正式生成

- 当前 toolchain 已确认可用：

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
protoc --version
protoc-gen-go --version
protoc-gen-go-grpc --version
```

- 版本结果：
  - `goctl version 1.10.1 linux/amd64`
  - `libprotoc 3.19.4`
  - `protoc-gen-go v1.36.11`
  - `protoc-gen-go-grpc 1.6.1`
- 正式输出目录：
  - pb：`proto/{userpb,authpb,friendspb,groupspb,messagepb}`
  - zrpc scaffold：`internal/rpcgen/{user,auth,friends,groups,message}`
- 目录策略：
  - 保留 proto 中现有 `option go_package = "github.com/wujunhui99/agents_im/proto/*pb"`。
  - `--go_out=.` / `--go-grpc_out=.` 配合 `--go_opt=module=github.com/wujunhui99/agents_im` 和 `--go-grpc_opt=module=github.com/wujunhui99/agents_im`，把 generated pb 放到 module 内的 `proto/*pb`，避免生成 `github.com/wujunhui99/agents_im/...` 嵌套目录。
  - `--zrpc_out=internal/rpcgen/<service>` 让每个服务拥有独立 goctl scaffold，不覆盖现有 `cmd/*-rpc`、`internal/rpc`、`internal/auth/rpc`。
- 保留兼容层：
  - `internal/rpc/user_server.go`
  - `internal/rpc/friends_server.go`
  - `internal/rpc/groups_server.go`
  - `internal/auth/rpc/auth_server.go`
  - `cmd/{user,auth,friends,groups}-rpc/main.go`

最终 RPC 生成命令：

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl rpc protoc proto/user.proto --go_out=. --go_opt=module=github.com/wujunhui99/agents_im --go-grpc_out=. --go-grpc_opt=module=github.com/wujunhui99/agents_im --zrpc_out=internal/rpcgen/user --style go_zero
goctl rpc protoc proto/auth.proto --go_out=. --go_opt=module=github.com/wujunhui99/agents_im --go-grpc_out=. --go-grpc_opt=module=github.com/wujunhui99/agents_im --zrpc_out=internal/rpcgen/auth --style go_zero
goctl rpc protoc proto/friends.proto --go_out=. --go_opt=module=github.com/wujunhui99/agents_im --go-grpc_out=. --go-grpc_opt=module=github.com/wujunhui99/agents_im --zrpc_out=internal/rpcgen/friends --style go_zero
goctl rpc protoc proto/groups.proto --go_out=. --go_opt=module=github.com/wujunhui99/agents_im --go-grpc_out=. --go-grpc_opt=module=github.com/wujunhui99/agents_im --zrpc_out=internal/rpcgen/groups --style go_zero
goctl rpc protoc proto/message.proto --go_out=. --go_opt=module=github.com/wujunhui99/agents_im --go-grpc_out=. --go-grpc_opt=module=github.com/wujunhui99/agents_im --zrpc_out=internal/rpcgen/message --style go_zero
```

## 后续任务边界

- 任务2 JWT：本阶段不新增 go-zero JWT middleware，不修改 token 签发为 JWT，不变更受保护路由鉴权机制。后续实现时应通过 `.api` 的 `jwt`/middleware 规格先行，再用 goctl 生成/迁移中间件。
- 任务3 PG/docker-compose：本阶段不生成 PG model、不接入 `sqlx.NewPg`、不变更内存仓储。后续 PG 本地开发、Redis、Kafka、etcd 等中间件均应放入 docker-compose 配置，不要求开发者手动安装本地进程。

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
protoc --version
protoc-gen-go --version
protoc-gen-go-grpc --version
for f in api/*.api; do goctl api validate -api "$f"; done
PATH=/tmp/go/bin:$HOME/go/bin:$PATH gofmt -w $(find . -name "*.go" -print)
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go mod tidy
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
git status --short --branch
```

验证结果：

- `goctl --version`：`goctl version 1.10.1 linux/amd64`
- `protoc --version`：`libprotoc 3.19.4`
- `protoc-gen-go --version`：`protoc-gen-go v1.36.11`
- `protoc-gen-go-grpc --version`：`protoc-gen-go-grpc 1.6.1`
- `for f in api/*.api; do goctl api validate -api "$f"; done`：5 个 API 均输出 `api format ok`
- `gofmt -w $(find . -name "*.go" -print)`：完成
- `go mod tidy`：完成，补齐 goctl zrpc scaffold 编译所需的 `go.sum` 校验和，并将 generated pb 直接导入的 `google.golang.org/grpc` / `google.golang.org/protobuf` 记录为 direct dependencies。
- `go test ./...`：通过，包含 `tests` 包
- `bash scripts/verify-static.sh`：`static verification passed`，包含 RPC generated scaffold 与 service server marker 检查
- `git status --short --branch`：位于 `feature/goctl-refactor`，变更待提交；提交后推送到同名远端分支

## 风险与回滚

- go-zero 1.10.1 依赖要求 Go 1.24+；`go mod tidy` 会把 `go` directive 提升到 `1.24.0`。
- 旧 mux 注册函数和新 go-zero handler 同时存在，短期有重复 HTTP adapter 维护成本；后续可在更多 go-zero HTTP 测试覆盖后移除旧 mux 层。
- 若 go-zero handler 出现回归，可临时将 API 命令入口切回旧 mux 注册函数；业务逻辑未被重写，回滚范围局限在 REST adapter。

## 结果记录

- Task 1 REST goctl refactor 已完成：API spec 可 validate，REST handler/types/routes 已按 goctl 结构迁入，API 命令入口已使用 go-zero `rest.Server`。
- 现有业务逻辑仍由原 hand-written logic/repository 承载，未删除 user/auth/friends/groups/message/gateway/read/storage 行为。
- 新增 `cmd/message-api` 和 `etc/message-api.yaml`，用于启动 message REST 层；仍使用内存仓储，不引入 PG。
- `go.mod` 已引入 `github.com/zeromicro/go-zero v1.10.1`；该版本要求 Go 1.24+，因此 `go` directive 更新为 `1.24.0`。
- RPC/protoc 已从临时探索升级为正式源码生成：pb 生成到 `proto/*pb`，zrpc scaffold 生成到 `internal/rpcgen/<service>`。
- 现有 hand-written RPC contract 兼容层继续保留，避免影响现有 `cmd/*-rpc` healthz 行为和测试。
- 提交信息：`refactor(goctl): align services with generated go-zero structure`。
- RPC 补齐提交信息：`fix(goctl): add generated rpc scaffolds`。
