# Groups Service Go-Zero

状态：Completed
Completed: 2026-05-01

## 背景

当前 `feature/groups-service` worktree 已有 user 第一阶段 go-zero 风格骨架。groups 服务需要在此基础上落地群聊基础信息和群成员关系能力，并依赖 user-rpc 语义做成员存在性校验。当前环境可能缺少 `goctl`，因此计划优先保证契约、源码结构、业务逻辑、测试和验证记录可提交。

## 目标

- 完成 groups 第一阶段产品规格。
- 完成 groups-rpc/groups-api go-zero 实现设计。
- 实现 groups-rpc 逻辑：创建群、查询群、加入群或添加成员、退出群、查询群成员。
- 实现 groups-api HTTP 接口：`POST /groups`、`GET /groups/{group_id}`、`POST /groups/{group_id}/members`、`DELETE /groups/{group_id}/members/me`、`GET /groups/{group_id}/members`。
- 使用 `X-User-Id` 模拟网关透传当前用户身份。
- 加群或添加成员前通过 user-rpc 语义校验用户存在。
- 创建群时创建者自动成为成员。
- 使用 repository 接口隔离内存实现和后续 PostgreSQL 实现。
- 补充单元测试并保持 user 既有测试通过。

## 非目标

- 不实现 auth 登录、token 校验或 gateway。
- 不保存用户资料权威数据。
- 不实现好友关系。
- 不实现群管理员、审批、邀请、禁言、群公告、群昵称、群头像。
- 不实现 PostgreSQL 持久化。

## 任务拆分

- [x] Task 1：阅读 `AGENTS.md`、user 规格、user 设计和 user 执行计划。
- [x] Task 2：阅读外层架构、计划规范和 user/auth/friends/groups 边界文档。
- [x] Task 3：补充 groups 第一阶段产品规格。
- [x] Task 4：补充 groups go-zero 实现设计。
- [x] Task 5：创建本执行计划并记录 Planner 结果。
- [x] Task 6：新增 groups api/proto、配置和服务入口。
- [x] Task 7：实现群模型、repository 接口、内存 repository、user 存在性适配器和 groups logic。
- [x] Task 8：实现 groups-rpc goctl/zrpc scaffold 和 groups-api HTTP handler。
- [x] Task 9：补充单元测试覆盖创建群、加群、重复加群、退群、成员列表、群不存在、用户不存在。
- [x] Task 10：运行 `gofmt`、`go test ./...`、`scripts/verify-static.sh`，记录验证结果和 BLOCKER。
- [x] Task 11：Evaluator 检查代码、测试、文档一致性，修复问题。
- [x] Task 12：历史 feature 分支实现已集成到 `main`；本计划归档。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 第一阶段使用 `X-User-Id` 模拟当前用户身份。 | auth/gateway 尚未实现，但创建群、加群和退群需要身份上下文。 |
| 2026-04-29 | groups 通过 user-rpc 的 `GetUserByID` 语义做用户存在性校验。 | groups 不保存用户资料权威数据，只需要确认成员是否存在。 |
| 2026-04-29 | 重复加群按幂等处理并返回 `already_member=true`。 | 符合第一阶段简单客户端重试语义，避免重复成员关系。 |
| 2026-04-29 | 第一阶段使用内存 repository，并保持接口可替换。 | 当前目标是稳定 groups 契约，PostgreSQL 可后续补充。 |

## Planner 结果

- 已阅读当前 worktree `AGENTS.md`。
- 已阅读 `docs/product-specs/user-service.md`。
- 已阅读 `docs/design-docs/user-service-go-zero.md`。
- 已阅读 `docs/exec-plans/completed/user-service-go-zero.md`。
- 已阅读外层 `/home/ws/project/AGENTS.md`。
- 已阅读 `/home/ws/project/ARCHITECTURE.md`。
- 已阅读 `/home/ws/project/docs/PLANS.md`。
- 已阅读 `/home/ws/project/docs/product-specs/account-social-core.md`。
- 已阅读 `/home/ws/project/docs/design-docs/user-auth-friends-groups-boundaries.md`。
- 已新增 `docs/product-specs/groups-service.md`。
- 已新增 `docs/design-docs/groups-service-go-zero.md`。

## 验证方式

计划运行：

```bash
goctl version
gofmt -w $(find . -name '*.go' -print)
go test ./...
scripts/verify-static.sh
```

如 `goctl` 不存在，则记录 BLOCKER，并通过手写 go-zero 风格结构继续推进。

## 风险与回滚

- 风险：手写 go-zero 骨架可能与未来 `goctl` 生成结构存在细节差异。缓解：保留 `api/groups.api` 和 `proto/groups.proto`，后续以契约和 logic 为准校准。
- 风险：内存 repository 不具备生产持久化能力。缓解：通过 repository 接口隔离，后续替换 PostgreSQL 实现。
- 风险：第一阶段未实现群权限，添加指定成员的权限边界需要后续设计确认。缓解：产品规格明确该限制，并预留管理员/邀请/审批扩展。
- 回滚：移除本次新增 groups 服务目录、规格文件、任务文档和测试即可，不影响 user 服务逻辑。

## Generator 结果

- 已新增 `api/groups.api` 和 `proto/groups.proto`，定义 groups-api 与 groups-rpc 第一阶段契约。
- 已新增 `cmd/groups-api/main.go` 与 `cmd/groups-rpc/main.go` 服务入口。
- 已新增 `etc/groups-api.yaml` 与 `etc/groups-rpc.yaml` 配置。
- 已实现 `internal/model.Group` 和 `internal/model.GroupMember`，不包含用户资料权威数据、认证秘密或好友关系字段。
- 已实现 `internal/repository.GroupsRepository` 和线程安全内存实现，后续可替换 PostgreSQL。
- 已实现 `internal/logic.GroupsLogic`：创建群、查询群、加群/添加成员、退群、查询有效成员列表。
- 已实现 `logic.UserExistenceChecker` 窄接口和 `UserLogicExistenceChecker` 适配器，当前按 user-rpc `GetUserByID` 语义校验用户存在。
- 已迁移为 goctl RPC scaffold：`internal/rpcgen/groups`，旧 `internal/rpc.GroupsServer` wrapper 已移除。
- 已迁移为 goctl REST handler 结构：`internal/handler/groups/*` 与 `RegisterGroupsGoZeroHandlers`。
- 已补充 `tests/groups_service_test.go` 覆盖创建群、加群、重复加群、退群、成员列表、群不存在和用户不存在。
- 已更新 `scripts/verify-static.sh` 纳入 groups 契约、源码、测试和文档检查。

## Evaluator 结果

验证命令：

```bash
goctl version
/tmp/go/bin/gofmt -w $(find . -name '*.go' -print)
PATH=/tmp/go/bin:$PATH GOCACHE=/tmp/go-build-cache go test ./...
scripts/verify-static.sh
```

验证结果：

```text
/bin/bash: line 1: goctl: command not found

?   	github.com/wujunhui99/agents_im/cmd/groups-api	[no test files]
?   	github.com/wujunhui99/agents_im/cmd/groups-rpc	[no test files]
?   	github.com/wujunhui99/agents_im/cmd/user-api	[no test files]
?   	github.com/wujunhui99/agents_im/cmd/user-rpc	[no test files]
?   	github.com/wujunhui99/agents_im/internal/apperror	[no test files]
?   	github.com/wujunhui99/agents_im/internal/config	[no test files]
?   	github.com/wujunhui99/agents_im/internal/handler	[no test files]
?   	github.com/wujunhui99/agents_im/internal/logic	[no test files]
?   	github.com/wujunhui99/agents_im/internal/model	[no test files]
?   	github.com/wujunhui99/agents_im/internal/repository	[no test files]
?   	github.com/wujunhui99/agents_im/internal/response	[no test files]
?   	github.com/wujunhui99/agents_im/internal/rpcgen/groups	[no test files]
?   	github.com/wujunhui99/agents_im/internal/svc	[no test files]
ok  	github.com/wujunhui99/agents_im/tests	0.003s

static verification passed
```

一致性检查：

- `groups` 只保存群基础信息和群成员关系，不保存用户资料权威数据。
- `POST /groups` 使用 `X-User-Id` 作为创建者，创建者自动成为有效成员。
- `POST /groups/{group_id}/members` 在写入成员关系前校验群存在、当前用户存在和目标用户存在。
- 重复加群返回 `already_member=true`，不会创建重复有效成员关系。
- `DELETE /groups/{group_id}/members/me` 将成员状态置为 `left`，成员列表只返回 `active` 成员。
- user 既有测试与新增 groups 测试均已通过。
- 初次运行 `PATH=/tmp/go/bin:$PATH go test ./...` 因默认 Go build cache 指向只读目录失败；设置 `GOCACHE=/tmp/go-build-cache` 后通过。

## 归档记录

- 2026-05-01：当前 `main`/`HEAD` 为 `e1fdba70ede044879775c13fa31c1025f4a1b371`，已包含 `api/groups.api`、`proto/groups.proto`、`cmd/groups-*`、groups logic/repository/handler、`internal/rpcgen/groups`、产品规格、设计文档和 `tests/groups_service_test.go`。原计划只剩历史“提交 feature 分支”事项，主干已包含对应实现，因此移至 `completed`。
- 本次归档不改业务代码，后续验证以 `bash scripts/verify-static.sh` 和 `git diff --check` 为准。

## 历史 BLOCKER

- 历史执行时 `goctl version` 返回 `/bin/bash: line 1: goctl: command not found`，因此该阶段按要求手写 go-zero 风格结构。
- 历史执行时默认 PATH 中没有 `go`，使用 `/tmp/go/bin/go` 和 `/tmp/go/bin/gofmt` 完成验证。
- 历史执行时 `git add` 返回 `fatal: Unable to create '/home/ws/project/agents_im/.git/worktrees/groups/index.lock': Read-only file system`。当前主干已包含对应实现，本项不再阻塞归档。
