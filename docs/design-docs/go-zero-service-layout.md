# go-zero service layout migration guide

本指南是 agents_im 后续 go-zero 服务渐进重构的执行手册。迁移其他服务时先读本文件，再读 `.claude/skills/refactor-domain-to-service/SKILL.md`（goctl 数据层 + BFF 聚合的复刻 playbook）。

## 目标结构

每个业务域放在 `service/<domain>/` 下，API 与 RPC 分离：

```text
service/
└── <domain>/
    ├── api/
    │   ├── <domain>.api
    │   ├── etc/
    │   ├── internal/
    │   │   ├── config/
    │   │   ├── handler/
    │   │   ├── logic/
    │   │   └── svc/
    │   └── <domain>.go
    └── rpc/
        ├── <domain>.proto
        ├── etc/
        ├── internal/
        │   ├── config/
        │   ├── logic/
        │   ├── model/
        │   ├── server/
        │   └── svc/
        ├── <domain>service/
        └── <domain>.go
```

本仓库采用 BFF 边界：

- `api` 可以调用一个或多个 `rpc`，负责 HTTP/BFF 入参、出参、鉴权、聚合。
- `api` 不允许直接操作数据库；禁止在 API `logic`/`svc` 里直接创建 repository/model/DB connection。
- `rpc` 是业务与数据边界，数据库 model 代码放在对应 `service/<domain>/rpc/internal/model`。
- `rpc` 之间不允许相互调用；跨域组合由 API/BFF 编排，或者后续通过明确的事件/消息机制解耦。
- 不新增 `api-gateway/` 目录；需要 BFF 时使用具体 domain API 或单独的业务 API 服务，而不是一个默认全局 gateway。
- `common/model` 默认不要创建。只有真正跨服务、稳定、非业务表的基础类型才可进入 `common/types`、`common/response`、`common/errorx` 等公共包。

## Git worktree 与分支

从主仓库创建独立 worktree，不在已有脏工作区里直接重构：

```bash
cd /home/ws/project/agents_im
git fetch origin main
mkdir -p /home/ws/project/worktrees

git worktree add \
  -b refactor/hermes/issue-<issue-number>-<domain>-rpc-service-layout \
  /home/ws/project/worktrees/<domain>-rpc-service-layout \
  origin/main

cd /home/ws/project/worktrees/<domain>-rpc-service-layout
git config user.name 'Hermes (AI Agent)'
git config user.email 'hermes@agents.noreply.local'
```

分支名必须满足 `docs/AGENT_GIT_STANDARD.md`：

```text
<type>/<agent-name>/issue-<number>-<task-desc>
```

示例：

```text
refactor/hermes/issue-281-user-rpc-service-layout
refactor/eino/issue-282-friends-rpc-service-layout
```

如果还没有 Issue，必须先创建 Issue，再创建分支和提交；开发 PR 必须只关闭一个 Issue。Codex worker 也必须 issue-first：接到迁移任务后先用 `gh issue create` 创建或复用一个明确匹配的 Issue，得到 Issue number 后再按 `docs/AGENT_GIT_STANDARD.md` 创建仓库认可的 agent 分支；如果当前仓库未允许 `codex` 作为第二路径段，就使用 Controller 指定的可信 agent 分支或明确失败报告 blocker。缺少 GitHub 权限时必须失败并报告 blocker，不能用 `issue-0`、不能假装已有 Issue。

## Commit 规则

提交前设置 Git identity：

```bash
git config user.name 'Hermes (AI Agent)'
git config user.email 'hermes@agents.noreply.local'
```

commit subject 格式：

```text
refactor(user-rpc)[hermes]: move user rpc under service layout
```

commit body 必须带 trailers：

```text
Issue: #<issue-number>
Agent: hermes
Human-Owner: wujunhui99
Coding-Tool: Hermes Agent
```

Codex worker 提交时 `Agent:` 按实际 agent 名称填写，`Coding-Tool:` 可写 `Codex CLI`。

## goctl 生成 RPC 的标准流程

以 `<domain>=user` 为例：

```bash
mkdir -p service/user/api service/user/rpc
cp api/user.api service/user/api/user.api              # 仅迁移 RPC 时先复制，不改 API 入口
# 迁移已有服务时从旧 proto 复制/移动；新服务直接在 rpc 目录创建 proto。
cp proto/user.proto service/user/rpc/user.proto
```

使用 goctl 生成 RPC scaffold；必须带 `--go_opt=module` / `--go-grpc_opt=module`，否则 `go_package` 是完整 module path 时可能生成到 `github.com/wujunhui99/agents_im/...` 嵌套目录：

```bash
goctl rpc protoc service/user/rpc/user.proto \
  --go_out=. --go_opt=module=github.com/wujunhui99/agents_im \
  --go-grpc_out=. --go-grpc_opt=module=github.com/wujunhui99/agents_im \
  --zrpc_out=service/user/rpc \
  --style go_zero \
  --verbose
```

生成后检查：

```bash
test -f service/user/rpc/user.v1.go
test -f service/user/rpc/internal/server/user_service_server.go
test -f service/user/rpc/internal/svc/service_context.go
test -f service/user/rpc/userclient/user.go
test -f service/user/rpc/user/user.pb.go
test -f service/user/rpc/user/user_grpc.pb.go
```

`cmd/<domain>-rpc/main.go` 不直接 import `service/<domain>/rpc/internal/*`，因为 Go `internal` 可见性不允许。需要添加桥接包：

```text
service/<domain>/rpc/entry/entry.go
```

然后 `cmd/<domain>-rpc/main.go` 只调用：

```go
<domain>entry.Start(*configFile)
```

## goctl 生成 API 的标准流程

迁移 API 时先复制 canonical `.api` 到 service layout，再用 goctl 生成 scaffold。必须先生成到临时目录检查输出，再把安全结构生成/迁移到目标目录：

```bash
mkdir -p service/user/api
cp api/user.api service/user/api/user.api

rm -rf /tmp/agents-im-user-api-goctl
mkdir -p /tmp/agents-im-user-api-goctl
goctl api go -api service/user/api/user.api -dir /tmp/agents-im-user-api-goctl --style go_zero

goctl api go -api service/user/api/user.api -dir service/user/api --style go_zero
```

生成后检查：

```bash
test -f service/user/api/internal/handler/routes.go
test -f service/user/api/internal/svc/service_context.go
test -f service/user/api/internal/types/types.go
test -f service/user/api/user.go
```

`cmd/<domain>-api/main.go` 不直接 import `service/<domain>/api/internal/*`，因为 Go `internal` 可见性不允许。需要添加桥接包：

```text
service/<domain>/api/entry/entry.go
```

然后 `cmd/<domain>-api/main.go` 只调用：

```go
<domain>entry.Start(*configFile)
```

API 层是 BFF 边界：可以补 HTTP 鉴权、response envelope、RPC 错误映射、聚合多个 RPC 的逻辑，但不能创建 repository/model/DB/object-storage 依赖。若现有 API 行为需要数据访问，优先补 RPC 能力；如果对应 RPC 不存在且无法在当前 slice 安全补齐，必须 fail closed 并报告 blocker，不能在 API 里保留直接 DB fallback。

## goctl 生成 model 的标准流程

优先使用 PostgreSQL datasource 生成，目标目录必须在 RPC 内部：

```bash
goctl model pg datasource \
  --url "$DATABASE_URL" \
  --schema public \
  --table accounts,profiles \
  --dir service/user/rpc/internal/model \
  --style go_zero
```

注意：

- `DATABASE_URL` 不能提交、不能打印到日志、不能写进文档。输出里统一写 `[REDACTED]`。
- 数据库 schema 事实源是 `db/migrations/*.sql`；新增改动加下一号 migration。
- 如果本地没有可用 PostgreSQL，但任务只是在做结构迁移，可以临时用从 migration 提取的 DDL 生成 scaffold；最终 PR 前必须说明这是 DDL fallback，并尽量用真实 PostgreSQL datasource 重新生成或验证。
- 生成的 model 包默认只属于对应 RPC，API 不得 import `service/<domain>/rpc/internal/model`。

## 补业务代码的边界

必须先让 goctl 生成 scaffold，再补最小必要代码：

- 可以补：`internal/logic/*` 的实际业务调用、`internal/svc/service_context.go` 的依赖注入、`entry/entry.go` 的启动桥接、必要的 convert/helper。
- 不要手写替代：server registration、client interface、pb/grpc generated 文件、handler/logic scaffold 的空壳。
- 不要把旧 `internal/rpcgen/<domain>` 留作运行入口；新入口应是 `service/<domain>/rpc`。
- 迁移一个服务时只改该服务的 RPC/API 入口与必要静态检查，不顺手重构其他 domain。

## 验证命令

RPC 迁移最小验证：

```bash
gofmt -w cmd/<domain>-rpc/main.go service/<domain>/rpc proto/<domain>pb/*.go

go test ./cmd/<domain>-rpc ./service/<domain>/rpc/...
```

API 迁移最小验证：

```bash
goctl api validate -api service/<domain>/api/<domain>.api

gofmt -w cmd/<domain>-api/main.go service/<domain>/api

go test ./cmd/<domain>-api ./service/<domain>/api/...
go test ./cmd/<domain>-rpc ./service/<domain>/rpc/...
```

提交前公共验证：

```bash
go test ./...

bash scripts/verify-static.sh

git diff --check
```

如果改了 DB model/schema 或 repository 行为，加跑 PostgreSQL integration；如果本地 PostgreSQL/Docker 不可用，必须在交付说明里写明 blocker，不得伪造验证。

## PR、Merge Queue 与 Drone 检查

推送并开 PR：

```bash
git push -u origin HEAD

gh pr create \
  --base main \
  --title 'refactor(user-rpc)[hermes]: move user rpc under service layout' \
  --body-file /tmp/pr-body.md
```

PR body 必须包含：

```text
Closes #<issue-number>

## Summary
- ...

## Verification
- `go test ...`
- `bash scripts/verify-static.sh`

## Risk
- ...
```

本仓库使用 GitHub PR + Merge Queue，不能直接 merge 到 `main`。CI 通过后加入 Merge Queue；Merge Queue 合并到 `main` 后再查 Drone。

Drone 地址：

```text
https://drone.agenticim.xyz
```

合并到 `main` 后检查方式：

1. 打开 Drone UI：`https://drone.agenticim.xyz/wujunhui99/agents_im`，确认最新 `main` build 状态。
2. 如果 CLI/API token 可用，只能从本地 secret 文件读取，不得打印 token：

```bash
# 示例：不要 echo token 值
export DRONE_SERVER=https://drone.agenticim.xyz
export DRONE_TOKEN=$(cat secret/drone_token)
drone build ls wujunhui99/agents_im --branch main --limit 10
```

3. 对 deploy 相关变更，Drone 绿不等于生产完成；还要验证 k3s rollout、pod imageID、health/API/WS smoke。没有权限时写清楚需要的权限和原因。

## 当前 user service 迁移示例

当前 user RPC 的 canonical source 位于：

```text
service/user/rpc/user.proto
service/user/rpc/internal/config/config.go
service/user/rpc/internal/logic/*.go
service/user/rpc/internal/model/*.go
service/user/rpc/internal/server/user_service_server.go
service/user/rpc/internal/svc/service_context.go
service/user/rpc/userclient/user.go
service/user/rpc/entry/entry.go
```

同时生成 `service/user/rpc/user/*` 作为 RPC-local protobuf Go package 输出，供 `userclient` 与 user API import：

```go
github.com/wujunhui99/agents_im/service/user/rpc/user
```

`cmd/user-rpc/main.go` 只导入 `service/user/rpc/entry`，不再导入旧 `internal/rpcgen/user/entry`。

当前 user API 的 canonical source 位于：

```text
service/user/api/user.api
service/user/api/internal/config/config.go
service/user/api/internal/handler/routes.go
service/user/api/internal/logic/user/*.go
service/user/api/internal/svc/service_context.go
service/user/api/internal/types/types.go
service/user/api/entry/entry.go
```

`cmd/user-api/main.go` 只导入 `service/user/api/entry`。`service/user/api/internal/svc.ServiceContext` 只持有 API config 和 `service/user/rpc/userclient.User` RPC client；用户 API 逻辑不得 import `internal/repository`、`internal/model`、`internal/objectstorage` 或旧 `internal/servicecontext/user`。
