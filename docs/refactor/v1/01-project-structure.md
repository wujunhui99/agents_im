# 01 — 项目结构 / 目录结构 重构分析

> 目标：识别仓库当前的目录"双轨制"、迁移残留、命名混乱，给出**一套统一的目标布局**和**逐步收敛路径**。
>
> 范围：根目录 + `service/` + `internal/` + `api/` + `proto/` + `etc/` + `db/` + `deploy/` + `docs/`。
> 不动：`web/`、`tests/`、`secret/`、`.ai-context/`（这些边界清晰、问题不大）。
>
> **入口约定**：无顶层 `cmd/`、无 `entry/` 子包——每个服务的 `package main` 就是 goctl 生成的 `service/<domain>/<api|rpc>/<domain>.go`；启动/构建/服务清单全由根 `Makefile`（`run-<svc>` / `build-<svc>` / `build-backend`，`BACKEND_SERVICES` + `PKG_<svc>` 映射包路径）驱动；非服务 main（如 e2e）放 `test/e2e/<name>/`。

---

## 1. 现状速描

仓库正处在 **从单体 `internal/` 布局往 go-zero `service/<domain>/` 布局迁移的中途**，结果是同一个域同时在两个地方存在：

| 域       | 老位置（internal）                                   | 新位置（service）                | 入口（service main 包）         | 状态 |
|----------|------------------------------------------------------|----------------------------------|---------------------|------|
| auth     | `internal/auth/{logic,repository,token,...}`、`internal/servicecontext/auth/` | `service/auth/{api,rpc}/...`     | `service/auth/{api,rpc}`（goctl main） | ✅ 已迁 |
| user     | `internal/logic/user/`、`internal/handler/user/`、`internal/servicecontext/user/` | `service/user/{api,rpc}/...`     | `service/user/{api,rpc}` | ✅ 已迁 |
| friends  | `internal/logic/friends/`、`internal/handler/friends/`、`internal/servicecontext/friends/`、`internal/rpcgen/friends` | `service/friends/{api,rpc}/...`  | `service/friends/{api,rpc}` | ⚠️ 双轨在跑 |
| groups   | `internal/logic/groups/`、`internal/handler/groups/`、`internal/servicecontext/groups/`、`internal/rpcgen/groups` | `service/groups/{api,rpc}/...`   | `service/groups/{api,rpc}`   | ⚠️ 双轨在跑 |
| third (含 mail) | `service/third/rpc/internal/provider/`（provider + tencent_ses，已脱 internal） | `service/third/rpc/...`           | `service/third/rpc`      | ✅ 已迁（mail 折入新服务 third，#429）|
| agent    | `internal/agent/`（pythonexec）、`internal/agentim/`、`internal/agentruntime/`、`internal/logic/agentlogic*` | `service/agent/api/...`          | `service/agent/api`     | 🟡 只迁了 API，业务逻辑还在 internal |
| message  | `internal/logic/message/`、`internal/handler/message/`、`internal/servicecontext/message/`、`internal/rpcgen/message` | **不存在** `service/msg/`    | 过渡态扁平 `service/message-api`；message-rpc 寄生 `internal/rpcgen/message` | ❌ 未迁 |
| gateway  | `internal/gateway/{ws,delivery}`、`internal/servicecontext/gateway/` | **不存在** `service/msggateway/`    | 过渡态扁平 `service/gateway-ws`    | ❌ 未迁 |
| transfer | `internal/transfer/...`                              | **不存在** `service/msgtransfer/`   | 过渡态扁平 `service/message-transfer` | ❌ 未迁 |
| admin    | `internal/handler/admin/`、`internal/logic/admin*`、`internal/servicecontext/admin/`、`internal/adminbootstrap/` | **不存在** `service/admin/`      | 寄生在 `service/message-api`（其 main 同时引用 `adminsvc`） | 🚨 寄生 |

> 一句话：**单体根的 `internal/handler/` + `internal/logic/` + `internal/servicecontext/` 仍然是 message/gateway/admin 三个服务的事实代码主干**，已迁的 auth/user/friends/groups 还有部分残骸没清。

---

## 2. 主要技术债（按危害排序）

### TD-1 🚨 admin 寄生在 msg-api，没有自己的服务边界
`service/message-api` 的 main（过渡态扁平目录）同时 import `adminsvc` 和 `messagesvc`，`internal/handler/gozero_routes.go` 把 admin、user、friends、groups、media、message、auth 路由全注册到**一个 go-zero rest.Server**。这违反了 `docs/design-docs/go-zero-service-layout.md` 第 1 节"API 不允许直接操作数据库、跨域不互调"的边界。

- **后果**：msg-api 进程要装载 admin 整条链；admin 改动会触发 msg-api 重启；admin 不能独立部署/扩容。
- **修复**：拆出独立服务 `service/admin/api`（admin-api）；msg-api 只保留 message 路由。

### TD-2 ✅ `internal/handler/gozero_routes.go` 已收敛为 message-only
历史：此文件曾同时为 auth/user/friends/groups/admin/media/message 注册 go-zero 路由。已迁服务的 main（`service/<domain>/api/<domain>.go`）只注册自己的路由，但 msg-api 仍通过这里启动多域路由。

> ✅ 已修复（#389）：user/auth/friends/groups 的 `RegisterXGoZeroHandlers` 与 `addXRoutes` 仅测试用，已连同 `internal/handler/{user,auth,friends,groups}`、`internal/logic/{user,auth,friends,groups}`、`internal/servicecontext/{user,auth,friends,groups}` 一并删除；`gozero_routes.go` 现仅保留 `RegisterMessageGoZeroHandlers`（msg-api live 路径）。message-rpc/message-api/message-transfer 仍骑在 monolith `internal/*` 上，属「干掉两套加载体系」更大议题（见 #336）。

### TD-3 ⚠️ message / gateway / transfer 三大主路径根本没进 `service/`
按照 `ARCHITECTURE.md`，这三个是核心消息链路。但代码主体仍在 `internal/`，意味着：

- `service/message-api`（过渡态扁平目录）的 main 60 行初始化代码，从 `internal/repository`、`internal/logic`、`internal/agentim`、`internal/auth/repository` 一把抓。
- 没有 `service/msg/rpc/message.proto`，但 message-rpc 跑起来了——它的 main 寄生在 `internal/rpcgen/message`，从那里拿生成的 pb，proto 源文件 `proto/message.proto` 还在仓库根的 `proto/`。
- gateway 也一样，`proto/` 里**根本没有** gateway proto。

### TD-4 ⚠️ `proto/` 与 `service/<domain>/rpc/<domain>.proto` 双源
- `proto/friends.proto`、`proto/groups.proto`、`proto/message.proto` 存活；
- 同时 `service/friends/rpc/friends.proto`、`service/groups/rpc/groups.proto`、`service/auth/rpc/auth.proto`、`service/user/rpc/user.proto`、`service/third/rpc/mail.proto`、`service/agent/rpc/agent.proto`（如存在）也存活；
- `internal/rpcgen/{friends,groups,message,rpcerror}` 仍在使用。

go-zero 服务迁移指南要求 proto 跟着 service 走（00-decisions **D10**）。Stage 3 步骤 7：proto 单源到 `service/<domain>/rpc/<domain>.proto`，删除顶层 `proto/`；同步顶层 `api/` 也下沉到 `service/<domain>/api/<domain>.api`（步骤 8）。

### TD-5 ⚠️ `internal/servicecontext/<domain>/` 与 `service/<domain>/api/internal/svc/` 两份 svc
auth、user、friends、groups、message、gateway、admin 在 `internal/servicecontext/` 各有一份；同时 auth/user/friends/groups 在 `service/<domain>/api/internal/svc/service_context.go` 又有一份新的。

- **后果**：调用方分不清要 import 哪个；任何字段加进 svc 都可能要改两处。
- **修复**：迁完一域就**立刻**删 `internal/servicecontext/<domain>/`，禁止两份并存超过 1 个 PR 周期。

### TD-6 ⚠️ `internal/logic/` 是一团扁平大杂烩
35 个文件平铺一层，包含 admin、agent_registry、agent_definition、agentauditlogic、aihostinglogic、default_assistant、feedbacklogic、friendslogic、groupslogic、medialogic、messagelogic、userlogic、admin_ai_replay、user/、auth/、message/ 子目录。命名风格不一致（有的带 logic 后缀，有的不带）。

- **后果**：找不到东西；导入图复杂；测试粒度差。
- **修复**：跟着域迁，文件落到 `service/<domain>/{api,rpc}/internal/logic/`。

### TD-7 ⚠️ `internal/repository/` 50+ 文件扁平化
所有域的 repository（账户、消息、群组、agent、媒体、conversation_ai_hosting、agent_hosting、agent_registry、feedback、task_report、delivery_attempt 等）扔在一层。`memory.go`、`postgres_*.go`、`*_memory.go` 命名风格分裂。`message_outbox_repository.go` / `postgres_outbox.go` 也在其中——根据 00-decisions **D1**，outbox 整体弃用，Stage 1 步骤 3 删除。

- **后果**：repository 边界不清，跨域随便互引；schema 迁移时不知道改谁。
- **修复**：按 00-decisions **D13**，数据访问改为 goctl model 落 `service/<domain>/rpc/internal/model/`（**纯 goctl model，废除手写 repository 层**）；顶层 `internal/repository/` 在 Stage 4 步骤 16 一并删除（**不再保留**）。

### TD-8 🟡 `etc/` 平铺 14 个 yaml，与 `service/<domain>/{api,rpc}/etc/` 重复
迁完的服务还有两份 yaml：仓库根 `etc/auth-api.yaml` 是启动时实际读的（`make run-auth-api` → `go run ./service/auth/api -f etc/auth-api.yaml`）；但 `service/auth/api/etc/auth-api.yaml` 也存在。

- **后果**：改一份生产没生效，改另一份没人读，环境变量是不是被两边都展开了不清楚。
- **修复**：约定**仓库根 `etc/` 是部署事实**（k8s 也是从这渲染），`service/<domain>/{api,rpc}/etc/` 只是 goctl 模板，不下发；或者反过来。**任选其一，文档化**。

### TD-9 🟡 入口装配风格分裂
入口统一在 `service/<domain>/<api|rpc>/<domain>.go`（goctl 生成的 `package main`，无 `cmd/`、无 `entry/`），但装配风格两套：
- auth-api / user-api / friends-api / groups-api / third-rpc / agent-api → goctl 标准 main，只 `conf.MustLoad` + 注册 handler，干净；
- message-api / gateway-ws / message-transfer（过渡态扁平目录）→ 仍在 main 里手动装配 60+ 行依赖（repo、logic、agentim、authrepo、observability、httpx errors…）。

应当全部收敛到 goctl 标准 main，并把过渡态扁平目录迁入 `service/<domain>/<api|rpc>/`（msg-api/msg-rpc/msggateway/msgtransfer）。

### TD-10 🟡 `internal/agent/` 名字误导
里面只有 `pythonexec/`，名字像"agent 服务"实则是"python 沙箱执行器"。同时还有 `internal/agentim/`、`internal/agentruntime/`、`internal/agenteval/`，命名约定不统一。

- **修复**：`internal/agent/pythonexec` → `pkg/pythonexec/`（与 agent 解耦，将来还能给 message 用；00-decisions D10）；其它 agent 相关全部归 `service/agent/rpc/internal/`（见 04 §3）。

### TD-11 🟡 `internal/model/` vs 各服务 model 重复
`internal/model/` 有 `user.go agent.go group.go media.go friendship.go feedback.go agent_registry.go`，但 `service/user/rpc/internal/model/{accounts_model.go,profiles_model.go,...}` 又有一份 goctl 生成的 model。两套并存。

- **修复**：删 `internal/model/`（Stage 1 步骤 4），表对应 model 全部归 `service/<domain>/rpc/internal/model/` 自管；真正跨服务 domain 类型（如果有）由 02 CP-4 提到的 `service/<domain>/rpc/internal/domain/` 承担，**不再设 internal/domain 顶层**。

### TD-12 🟡 `internal/rpcgen/` 还存在
`internal/rpcgen/{friends,groups,message,rpcerror}` 是迁移过渡位置。已迁服务的 pb 客户端走 `service/<domain>/rpc/<domain>service/`，过渡产物 Stage 1 步骤 1 直接删除（00-decisions D10：顶层 internal 整体退役）。

---

## 3. 目标布局（建议收敛终点）

按 go-zero 官方目录指引（[`go-zero.dev`](https://go-zero.dev/docs/tutorials/go-zero/quick-start/dir-structure)）+ 00-decisions D10：

- **删除顶层 `internal/`**：所有内容收敛到 `service/<domain>/<api|rpc>/internal/`（服务局部内部）或 `pkg/`（跨服务基础设施）；
- **删除顶层 `api/`**：`.api` 文件下沉到 `service/<domain>/api/<domain>.api`（单源）；
- **删除顶层 `proto/`**：`.proto` 文件下沉到 `service/<domain>/rpc/<domain>.proto`（单源）；
- **无顶层 `cmd/`、无 `entry/` 子包**：每个服务的 `package main` 就是 goctl 生成的 `service/<domain>/<api|rpc>/<domain>.go`；启动/构建/清单由根 `Makefile`（`run-<svc>` / `build-<svc>` / `build-backend`，`BACKEND_SERVICES` + `PKG_<svc>`）驱动；e2e 等非服务 main 放 `test/e2e/<name>/`。

```text
agents_im/
├── service/                       # 业务域唯一代码主干（main 即 goctl <domain>.go，含 .api、.proto 单源）
│   ├── auth/
│   │   ├── api/
│   │   │   ├── auth.api                   # ← 原 api/auth.api
│   │   │   ├── auth.go                    # goctl 生成的 package main（入口）
│   │   │   ├── etc/auth-api.yaml
│   │   │   └── internal/{config,handler,logic,middleware,svc,types}/
│   │   └── rpc/
│   │       ├── auth.proto                 # ← 原 proto/auth.proto（若有）
│   │       ├── auth.go                    # goctl 生成的 package main（入口）
│   │       ├── authclient/                # 生成的 client（见 §5 命名统一）
│   │       ├── auth/                      # 生成的 pb.go
│   │       ├── etc/
│   │       └── internal/{config,logic,server,svc,model,credential,token,verification}/  # 无 repository、无 adapter（D12/D13）
│   ├── user/{api,rpc}/...
│   ├── friends/{api,rpc}/...
│   ├── groups/{api,rpc}/...
│   ├── mail/rpc/...                       # 只有 RPC
│   ├── agent/{api,rpc}/...                # 见 04 §3；新增 agent-rpc（见 04 AG-1）
│   ├── msg/{api,rpc}/...                  # 新建（见 07）；原 message 域，按 D3 改名 msg
│   ├── admin/api/...                      # 只有 API，从 message-api 拆出（TD-1）
│   ├── msggateway/                        # WebSocket 接入层，扁平单体（无 api/rpc 二分，去掉 -ws 后缀）
│   │   ├── msggateway.go                  # package main（入口）
│   │   ├── etc/
│   │   └── internal/{config,ws,server,svc}/
│   ├── msgtransfer/                       # 消费者 worker（← 原 message-transfer，00-decisions D3）
│   │   ├── msgtransfer.go                 # package main（入口）
│   │   ├── etc/
│   │   └── internal/{config,batcher,handler,svc}/
│   └── push/                              # 消费者 worker + 小 gRPC server（新增，D3）
│       ├── push.go                        # package main（入口）
│       ├── etc/
│       └── internal/{config,handler,offlinepush,svc}/
│
├── pkg/                           # 跨服务可重用基础设施（替代旧 internal/）
│   ├── apperror/                          # ← 原 internal/apperror
│   ├── response/                          # ← 原 internal/response（HTTP envelope）
│   ├── ctxuser/                           # ← 原 internal/ctxuser
│   ├── config/                            # ← 原 internal/config（公共 loader）
│   ├── observability/                     # ← 原 internal/observability（OTel + metrics + tracing）
│   ├── llmobs/                            # ← 原 internal/llmobs（LLM 观测 sink）
│   ├── health/                            # ← 原 internal/health
│   ├── idgen/                             # ← 原 internal/idgen（Snowflake）
│   ├── messaging/                         # ← 原 internal/messaging（Kafka producer/consumer）
│   ├── presence/                          # ← 原 internal/presence（Redis online 状态查询，D4）
│   ├── objectstorage/                     # ← 原 internal/objectstorage（MinIO/S3）
│   ├── pythonexec/                        # ← 原 internal/agent/pythonexec
│   └── jwtauth/                           # 无状态 JWT 验签 + 读共享 Redis `user_active_sessions:{uid}` 比对 jti（D14；非 auth 业务，纯鉴权基础设施）
│
├── etc/                           # 部署 yaml（唯一部署事实，TD-8）
├── Makefile                       # 启动/构建/清单入口（run-<svc> / build-backend，BACKEND_SERVICES + PKG_<svc>）
├── deploy/k8s/                    # 不变
├── db/migrations/                 # 不变
├── docs/                          # 不变
├── test/e2e/<name>/               # 非服务 main（如 single-machine e2e）
├── tests/                         # 不变
├── web/                           # 不变
└── scripts/                       # 不变
```

> **关键约束**（全部对齐 00-decisions D8/D10 + go-zero 指引）：
> - **顶层不存在** `internal/` / `api/` / `proto/` / `rpcgen/` / `cmd/`。
> - **无 `entry/` 子包**：入口即 `service/<domain>/<api|rpc>/<domain>.go`（goctl 生成的 `package main`），由 `Makefile` 驱动启动/构建。
> - `pkg/` 内不允许 import `service/...`（单向依赖：`service → pkg`，禁反向）。
> - `service/<domain>/rpc` 独占该 domain 的 DB model（goctl 生成，**无 repository 层**，D13）与 proto 源文件。
> - `service/<domain>/api` 只做 BFF：参数校验、鉴权、调 RPC 聚合；不允许操作 DB；持有 `.api` 源文件。
> - 跨域 RPC 互调禁止（00-decisions **D12**）：`rpc` 间一律不互调、也不经 `<other>adapter` 间接调；跨域组合只由 API 层 import RPC client 编排。
> - 删除 `internal/types/`、`internal/model/`、`internal/rpcgen/`、`internal/servicecontext/`、`internal/handler/` 整体目录。
> - 删除 `internal/outboxpublisher/`、`internal/repository/{message_outbox,postgres_outbox}.go`（00-decisions D1）。

---

## 4. 收敛路径（按"血压最低 → 最高"排序，每步独立可合并）

> **执行轨（00-decisions D11）**：本节 Stage 1~5 是宏观清理目标；落地采用**长期 refactor 分支 + 逐服务原地重建**——保留 proto/pb 契约，一次只重写一个服务的 `rpc/internal/{config,svc,logic,model}`，其它服务照常编译，顶层 `internal/` 留到最后统一删。**搬运不重写**（域逻辑 `git mv` / 移植，禁止从零重写）。中途不跑 CI，合回 main 前 `go build ./... && go test ./...` 整体绿并跑一次完整 CI + 回归。起点：auth。

### Stage 1 — 删除明显残骸（不动业务逻辑）
1. **删 `internal/rpcgen/{friends,groups}`**：已迁服务走 `service/<domain>/rpc/<domain>service/`，grep 验证后删。
2. **删 `internal/servicecontext/{auth,user,friends,groups}/`**：已被各 service 内 svc 替代。
3. **删 `internal/outboxpublisher/`、`internal/repository/{message_outbox,postgres_outbox}.go`**（00-decisions D1）。
4. **删 `internal/types/`**（基本空）、`internal/model/`（与 service rpc model 重复）。
5. **`etc/` 单源化**（TD-8）：选定仓库根 `etc/` 是部署事实，删 `service/<domain>/{api,rpc}/etc/`。

### Stage 2 — 建立 `pkg/`（顶层 internal 退役第一步）
6. **批量 `git mv internal/<pkg> → pkg/<pkg>`**（详见下表）。仅改 import path，无逻辑改动。一次 PR 完成全部 mv：

| 旧路径                            | 新路径                  | 迁移备注 |
|----------------------------------|-------------------------|---------|
| `internal/apperror/`             | `pkg/apperror/`         | 纯 mv |
| `internal/response/`             | `pkg/response/`         | 纯 mv |
| `internal/ctxuser/`              | `pkg/ctxuser/`          | 纯 mv |
| `internal/config/`               | `pkg/config/`           | 纯 mv |
| `internal/observability/`        | `pkg/observability/`    | 纯 mv |
| `internal/llmobs/`               | `pkg/llmobs/`           | 纯 mv |
| `internal/health/`               | `pkg/health/`           | 纯 mv |
| `internal/idgen/`                | `pkg/idgen/`            | 纯 mv |
| `internal/messaging/`            | `pkg/messaging/`        | 纯 mv |
| `internal/presence/`             | `pkg/presence/`         | 纯 mv（D4：仅作在线状态查询） |
| `internal/objectstorage/`        | `pkg/objectstorage/`    | 纯 mv |
| `internal/agent/pythonexec/`     | `pkg/pythonexec/`       | 顺手扁平化（去掉 agent/ 中间层） |

执行：`git mv` + 全仓 `gofmt -r 'A -> B'` 或 `find . -type f -name '*.go' \| xargs sed -i ''` 改 import；`go build ./...` 验证。

### Stage 3 — proto/api 下沉
7. **proto 下沉到 service**：`proto/<domain>.proto → service/<domain>/rpc/<domain>.proto`；同步改 `option go_package`；删根 `proto/`。
8. **api 下沉到 service**：`api/<domain>.api → service/<domain>/api/<domain>.api`；删根 `api/`（admin.api 等先迁到对应新 service）。
9. **`internal/handler/gozero_routes.go` 退役**：把每个 `RegisterXxxGoZeroHandlers` 拆到对应 `service/<domain>/api/internal/handler/routes.go`；删 `internal/handler/`。

### Stage 4 — 业务 service 落位（最大改动）
10. ✅ **`internal/mail/` → `service/third/rpc/internal/provider/`**：mail 折入新服务 **third**（第三方接入层），provider 实现已搬过去脱离 internal（#429；原计划 02 CP-8 落点 service/mail，实际合并为 third 以减少微服务数量）。
11. **拆 admin-api**（TD-1，收益最大）：
    - 建 `service/admin/api`（goctl main 即入口），Makefile 注册 admin-api（`BACKEND_SERVICES` + `PKG_admin-api`）；
    - 搬：`internal/handler/admin`、`internal/logic/admin*`、`internal/adminbootstrap`、`internal/servicecontext/admin`；
    - msg-api 卸下 admin 依赖。
12. **建 service/msg**（最大块，单独 epic）：搬 `internal/logic/message`、`internal/handler/message`、`internal/servicecontext/message`、`internal/rpcgen/message`；按 07 文档扩展为 10 个 RPC。
13. **建 service/msggateway**：搬 `internal/gateway/`；按 03 §7 砍业务依赖（00-decisions D8）。
14. **建 service/msgtransfer 与 service/push**：搬 `internal/transfer/`；按 03 §3.1 拆出 push（00-decisions D3）。
15. **拆 internal/agent 残部**（04 §3）：`internal/agentim/`、`internal/agentruntime/`、`internal/agenteval/`、`internal/auth/`、`internal/logic/agent*` → 全部进入 `service/agent/rpc/internal/{trigger,orchestrator,hosting,imadapter,audit,runtime,eval}/`。
16. **internal/repository 按域拆为 goctl model**（D13）：每搬一个 service，就用 `goctl model pg` 从 `db/migrations` 生成该域 model 落 `service/<domain>/rpc/internal/model/`（自定义查询写 custom 区），**不保留 repository 抽象**；最终 `internal/repository/` 整目录删除。

### Stage 5 — 收尾验证
17. `internal/` 顶层目录被删除（`ls internal/ 2>/dev/null` 无输出）。
18. `api/`、`proto/`、`rpcgen/` 顶层无残留。
19. CI 加 lint：禁止顶层 `internal/`、`api/`、`proto/` 出现（可用 `scripts/verify-static.sh` 加一段）。

---

## 5. 命名/约定一致性问题（小修）

| 现状                                                          | 问题                              | 建议                                  |
|---------------------------------------------------------------|-----------------------------------|---------------------------------------|
| `xxxlogic.go` vs `xxx_logic.go` 混用                          | 风格不一（02 CP-2）               | 统一 snake_case：`xxx_logic.go`，goctl 加 `--style=goZero` |
| `authservice/` `mailservice/` vs `friendsclient/` `groupsclient/` `userclient/` | proto 里 service 名两种命名 → 生成目录两种命名 | 统一 service 名为简洁形式：`service Auth`/`service Mail`（去 `Service` 后缀），goctl 自动加 `client` 后缀 → 全部产出 `xxxclient/` |
| `friendslogic.go groupslogic.go medialogic.go messagelogic.go userlogic.go` | 单文件巨石                       | 拆到 `service/<domain>/<api\|rpc>/internal/logic/<action>_logic.go` |
| `internal/repository/postgres_*.go` 和 `*_memory.go` 混排    | 手写双实现，goctl 不管理           | 改为 goctl model 落 `service/<domain>/rpc/internal/model/`，废 repository 层（D13） |
| `internal/agent/`、`internal/agentim/`、`internal/agentruntime/`、`internal/agenteval/` | 4 个 agent 前缀彼此关系不清     | 见 04 §3 重新切分                      |
| 顶层 `proto/`、`api/`                                          | 与 `service/<domain>/` 双源       | Stage 3 步骤 7/8：单源下沉到 service  |
| `internal/types/` 几乎空                                      | 没用上                            | Stage 1 步骤 4：删除                  |

---

## 6. 验收信号

收敛完成后，应满足（CI 用一行命令可验证）：

```bash
# A. 顶层目录干净（含无 cmd/）
test ! -d internal && test ! -d api && test ! -d proto && test ! -d rpcgen && test ! -d cmd

# B. 入口在 service 包内、无 entry/ 子包
! find service -type d -name entry | grep -q .
for s in $(make -s services | awk '{print $2}'); do test -n "$(grep -rl '^package main' "$s")" || exit 1; done

# C. 无残留导入
! grep -r '"github.com/wujunhui99/agents_im/internal' --include='*.go' .
! grep -r '"github.com/wujunhui99/agents_im/api' --include='*.go' .
! grep -r '"github.com/wujunhui99/agents_im/proto' --include='*.go' .

# D. pkg → service 单向依赖
! grep -r '"github.com/wujunhui99/agents_im/service' pkg/ --include='*.go'

# E. 跨域 rpc 互调禁止（02 CP-3）
# 例：禁止 service/friends/rpc/internal/logic 调 service/user/rpc/internal/logic
# （API 层 import RPC client 是允许的）
```

并且：
1. 每个域 `service/<domain>/api/<domain>.go`（main）启动后只挂自己的路由前缀；admin 独立部署。
2. `service/<domain>/rpc/<domain>.proto` 作为该域 RPC 契约单源；CI 加 lint 拒绝顶层 proto/ 重新出现。
3. `pkg/` 内不出现任何域名词（如 `pkg/friends/`、`pkg/message/`）——出现即说明边界判断错。

---

## 7. 不动的部分（确认 OK）

- `web/`：前端独立，跟后端解耦，不在这次重构范围。
- `tests/`：可以保留集成测试单独目录，不强制下沉到各 service。
- `secret/`、`scripts/`、`.ai-context/`、`.hermes/`、`.github/`：基础设施类，不动。
- `docs/`：本次只新增 `docs/refactor/`，旧 design-docs 不动；后续 design-docs 应被这次重构的结论替代。
