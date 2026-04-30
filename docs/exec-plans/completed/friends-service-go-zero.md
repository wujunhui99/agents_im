# Friends Service Go-Zero

状态：Completed
Completed: 2026-05-01

## 背景

`friends` 服务需要在 `user` 基础接口稳定后落地，负责好友关系维护。当前 worktree 已有 `user-api`/`user-rpc` 的手写 go-zero 风格结构，本任务需要在当前 `feature/friends-service` 分支内完成 friends 第一阶段文档、代码、测试和提交。

## 目标

- 完成 friends 第一阶段产品规格。
- 完成 friends-rpc/friends-api go-zero 实现设计。
- 实现 friends-rpc 逻辑：`AddFriend`、`DeleteFriend`、`ListFriends`、`GetFriendship`。
- 实现 friends-api HTTP 接口：`POST /friends`、`DELETE /friends/{user_id}`、`GET /friends`、`GET /friends/{user_id}`。
- 使用 `X-User-Id` 模拟 gateway 透传当前用户身份。
- 添加好友前通过 user lookup 校验双方用户存在。
- 第一阶段直接双向建立好友关系，重复添加保持幂等。
- 使用内存 repository，并通过接口隔离后续 PostgreSQL 替换。
- 补充单元测试并保持 user 已有测试通过。

## 非目标

- 不实现 auth 登录注册或 token 校验。
- 不保存用户资料权威数据。
- 不实现好友申请审批、备注、分组、黑名单。
- 不实现群成员关系。
- 不实现 PostgreSQL 持久化和迁移脚本。

## 任务拆分

- [x] Task 1：阅读 `AGENTS.md` 和外层必读文档，确认服务边界。
- [x] Task 2：阅读 user service 产品规格、设计和执行计划，确认既有模式。
- [x] Task 3：补充 friends 第一阶段产品规格。
- [x] Task 4：补充 friends go-zero 实现设计。
- [x] Task 5：创建本执行计划并记录 Planner 结果。
- [x] Task 6：新增 friends api/proto、配置和服务入口。
- [x] Task 7：实现好友模型、内存 repository、业务 logic、RPC server 和 HTTP handler。
- [x] Task 8：补充 friends 单元测试。
- [x] Task 9：运行 gofmt、go test ./...、scripts/verify-static.sh，并记录 goctl 状态。
- [x] Task 10：Evaluator 检查代码、测试、文档一致性并修复问题。
- [x] Task 11：历史 feature 分支实现已集成到 `main`；本计划归档。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 第一阶段使用 `X-User-Id` 模拟当前用户身份。 | auth/gateway 尚未接入，但 friends API 需要当前用户上下文。 |
| 2026-04-29 | 第一阶段直接双向建立好友关系，不做申请审批。 | 用户要求第一阶段可以不实现审批，先稳定关系维护能力。 |
| 2026-04-29 | 重复添加好友按幂等成功处理，并返回 `created=false`。 | 客户端重试更容易处理，且满足“幂等或明确结果”的要求。 |
| 2026-04-29 | friends logic 通过 `UserLookup` 窄接口依赖 user-rpc。 | 本地测试可复用 `UserLogic`，后续可替换为真正 user-rpc client。 |
| 2026-04-29 | 第一阶段使用内存 repository。 | 当前目标是稳定契约和业务边界，PostgreSQL 后续替换。 |

## Planner 结果

- 已阅读当前 worktree `AGENTS.md`。
- 已阅读外层 `/home/ws/project/AGENTS.md`。
- 已阅读 `/home/ws/project/ARCHITECTURE.md`。
- 已阅读 `/home/ws/project/docs/PLANS.md`。
- 已阅读 `/home/ws/project/docs/product-specs/account-social-core.md`。
- 已阅读 `/home/ws/project/docs/design-docs/user-auth-friends-groups-boundaries.md`。
- 已阅读 `/home/ws/project/docs/product-specs/user-service.md`。
- 已阅读 `/home/ws/project/docs/design-docs/user-service-go-zero.md`。
- 已阅读 `/home/ws/project/docs/exec-plans/completed/user-service-go-zero.md`。
- 已新增 `docs/product-specs/friends-service.md`。
- 已新增 `docs/design-docs/friends-service-go-zero.md`。

## 验证方式

计划运行：

```bash
goctl version
gofmt -w $(find . -name '*.go' -print)
go test ./...
scripts/verify-static.sh
```

如 `goctl` 不存在，则记录 BLOCKER，并保留手写 go-zero 风格结构。

## 风险与回滚

- 风险：手写 go-zero 骨架可能与未来 `goctl` 生成结构存在细节差异。缓解：保留 `api/friends.api` 和 `proto/friends.proto`，后续以契约和 logic 为准校准。
- 风险：内存 repository 不具备生产持久化能力。缓解：通过 repository 接口隔离，后续替换 PostgreSQL。
- 风险：当前使用本地 `UserLogic` 适配 user lookup，不是真实跨进程 RPC。缓解：接口已按 `GetUserByID` 收敛，后续替换为 user-rpc client。
- 回滚：移除本次新增 friends 文档、契约、入口、模型、逻辑、handler、测试和配置即可。

## Generator 结果

- 已新增 `api/friends.api` 和 `proto/friends.proto`，定义 friends-api 与 friends-rpc 第一阶段契约。
- 已新增 `cmd/friends-api/main.go` 与 `cmd/friends-rpc/main.go` 服务入口。
- 已新增 `etc/friends-api.yaml` 与 `etc/friends-rpc.yaml` 配置。
- 已新增 `internal/model.Friendship`，只维护好友关系状态，不包含用户资料权威字段或认证秘密。
- 已扩展 `internal/repository`，新增 `FriendshipRepository` 和内存双向好友关系实现。
- 已新增 `internal/logic.FriendsLogic`：添加好友、删除好友、好友列表、好友关系查询。
- 已迁移为 goctl RPC scaffold：`internal/rpcgen/friends`，旧 `internal/rpc.FriendsServer` wrapper 已移除。
- 已扩展 goctl REST handler 结构，新增 `internal/handler/friends/*` 与 `RegisterFriendsGoZeroHandlers`。
- 已将 `user-api` 入口调整为只注册 user routes，新增 `friends-api` 入口只注册 friends routes；测试通过 go-zero route registration 构造 router。
- 已补充 `tests/friends_service_test.go` 覆盖添加、重复添加、删除、列表、不能添加自己、用户不存在。
- 已更新 `scripts/verify-static.sh` 覆盖 friends 契约、源码、测试和文档。

## Evaluator 结果

验证命令：

```bash
PATH=/tmp/go/bin:$PATH go version
goctl version
PATH=/tmp/go/bin:$PATH gofmt -w $(find . -name '*.go' -print)
GOCACHE=/tmp/go-build-cache PATH=/tmp/go/bin:$PATH go test ./...
scripts/verify-static.sh
```

验证结果：

```text
go version go1.22.12 linux/amd64
/bin/bash: line 1: goctl: command not found
?   	github.com/wujunhui99/agents_im/cmd/friends-api	[no test files]
?   	github.com/wujunhui99/agents_im/cmd/friends-rpc	[no test files]
?   	github.com/wujunhui99/agents_im/cmd/user-api	[no test files]
?   	github.com/wujunhui99/agents_im/cmd/user-rpc	[no test files]
?   	github.com/wujunhui99/agents_im/internal/apperror	[no test files]
?   	github.com/wujunhui99/agents_im/internal/config	[no test files]
?   	github.com/wujunhui99/agents_im/internal/handler	[no test files]
?   	github.com/wujunhui99/agents_im/internal/logic	[no test files]
?   	github.com/wujunhui99/agents_im/internal/model	[no test files]
?   	github.com/wujunhui99/agents_im/internal/repository	[no test files]
?   	github.com/wujunhui99/agents_im/internal/response	[no test files]
?   	github.com/wujunhui99/agents_im/internal/rpcgen/friends	[no test files]
?   	github.com/wujunhui99/agents_im/internal/svc	[no test files]
ok  	github.com/wujunhui99/agents_im/tests	0.002s
static verification passed
```

一致性检查：

- friends 只维护好友关系，不保存用户资料权威字段、不保存认证秘密、不维护群成员关系。
- 添加好友前通过 `UserLookup.GetUserByID` 校验当前用户和目标用户存在；本地实现由 `UserLogic` 适配，后续可替换为 `user-rpc` client。
- `POST /friends` 使用 `X-User-Id` 作为当前用户，目标用户来自 body `user_id`。
- 不能添加自己；重复添加保持幂等并返回 `created=false`。
- 删除好友后双向关系标记为 `deleted`，好友列表只返回 `active` 关系。
- user 已有测试与 friends 新增测试均通过。

## 归档记录

- 2026-05-01：当前 `main`/`HEAD` 为 `e1fdba70ede044879775c13fa31c1025f4a1b371`，已包含 `api/friends.api`、`proto/friends.proto`、`cmd/friends-*`、friends logic/repository/handler、`internal/rpcgen/friends`、产品规格、设计文档和 `tests/friends_service_test.go`。原计划只剩历史“提交 feature 分支”事项，主干已包含对应实现，因此移至 `completed`。
- 本次归档不改业务代码，后续验证以 `bash scripts/verify-static.sh` 和 `git diff --check` 为准。

## 历史 BLOCKER

- 历史执行时 `goctl version` 返回 `/bin/bash: line 1: goctl: command not found`，因此该阶段按要求手写 go-zero 风格结构。
- 历史执行时默认 `go test ./...` 使用 `/home/ws/.cache/go-build` 遇到只读文件系统；已用 `GOCACHE=/tmp/go-build-cache` 完成等价全量测试。
- 历史执行时 `git add` 失败：`fatal: Unable to create '/home/ws/project/agents_im/.git/worktrees/friends/index.lock': Read-only file system`。当前主干已包含对应实现，本项不再阻塞归档。
