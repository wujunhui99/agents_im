# REST/Gateway ServiceContext Boundaries

状态：Implemented

## 背景

Issue #52 清理了早期手写 REST 时代的根级 `internal/svc.ServiceContext`。旧结构把 Account/User/Friends/Groups/Message/Media/Agent/Audit/Outbox 等依赖放进同一个聚合上下文，导致 go-zero handler 和 logic 可以跨服务边界直接取任意依赖。

项目方向是 go-zero 风格的服务内依赖注入：每个 REST/API 或 Gateway 进程只持有自己边界内需要的依赖。RPC 生成代码继续使用 goctl 生成的 `internal/rpcgen/<service>/internal/svc.ServiceContext`，不与 REST/Gateway 共享根级聚合上下文。

## 当前结构

REST/Gateway 运行时上下文位于：

| 边界 | Context package | 主要依赖 |
| --- | --- | --- |
| auth-api | `internal/servicecontext/auth` | `AuthLogic`、credential repo、Account adapter |
| user-api | `internal/servicecontext/user` | `UserLogic`、Account repo、头像/media 上传展示依赖 |
| friends-api | `internal/servicecontext/friends` | `FriendsLogic`、Account lookup |
| groups-api | `internal/servicecontext/groups` | `GroupsLogic`、Groups repo、Account existence checker |
| msg-rpc（AI 托管 runtime）| `service/msg/rpc/internal/aihosting`（message-api 已退役 #463，runtime 迁入 msg-rpc；#341/A4 重定位出 `internal/servicecontext/message`）| `MessageLogic`、Media validator、AI hosting/audit |
| agent-api | `service/agent/api/internal/svc` | `AgentLogic`、Agent repo、Account type checker |
| msggateway | 无（03 §9 A3 起不再用 servicecontext） | msg-rpc gRPC backend（`service/msggateway/internal/backend`）、JWT auth |
| shared auth runtime | `internal/servicecontext/common` | JWT config、optional active-session repository |

`internal/handler/**` 和 `internal/logic/<service>/*_logic.go` 只能 import 自己边界的 focused context，不能 import `github.com/wujunhui99/agents_im/internal/svc`。

REST adapter logic 使用 goctl 风格的每 handler 一个 `*_logic.go` 文件；不再维护聚合 `gozero_logic.go` 文件。

## Media 边界说明

当前部署矩阵没有独立 `media-api` 进程，media REST routes 仍挂在 `user-api` 下。`internal/servicecontext/user` 因此持有头像和 media upload/display 需要的 `MediaLogic`、`MediaRepository`、`ObjectStore`，但 friends/groups/message/agent contexts 不携带这些依赖，除非本边界实际需要。

Message Service 只持有 message-send media validation 所需的 media repository/validator，不拥有 object byte upload endpoint。

## 生产初始化规则

- REST/Gateway entrypoint 继续在各自 main（如 `service/msggateway/msggateway.go`）显式构造 repository/object storage/presence，并用 `log.Fatalf(...)` fail fast。
- 不允许为了简化 context wiring 在生产路径静默 fallback 到 memory repository、memory object store 或 nil validator。
- friends-api 不再构造 object storage，因为它不服务 media routes；user-api 仍必须构造 object storage 并 `EnsureBucket`。
- msggateway 不构造任何 message logic/repository：消息域 ws command 经 msg-rpc gRPC backend 转发（03 §9 A3），进程只持有 ws server、presence、session store 与 msg-rpc client。

## 静态校验

`scripts/verify-static.sh` 现在强制：

- 根级 `internal/svc` package 不存在；
- `cmd`、`internal/handler`、`internal/logic`、`tests` 不得 import `github.com/wujunhui99/agents_im/internal/svc`。
- REST adapter logic 不得新增聚合 `gozero_logic.go`；
- auth-api 不得回退到 `internal/auth/svc`，必须使用 `internal/servicecontext/auth`。

goctl RPC 生成目录下的 `internal/rpcgen/*/internal/svc` 是服务本地 context，不属于被清理的根级聚合 context。

`cmd/*-rpc` 保留 `internal/rpcgen/*/entry` 轻量启动桥接，因为 Go `internal` 可见性禁止 `cmd/*-rpc` 直接 import `internal/rpcgen/<service>/internal/{config,server,svc}`。这些 entry 文件只负责调用 goctl 生成的 config/server/svc，不承载业务 wiring。
