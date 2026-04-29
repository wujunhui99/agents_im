# User Service Go-Zero

状态：Active

## 背景

`user` 服务需要先于 `auth`、`friends`、`groups` 落地，提供账号资料权威能力。当前 worktree 初始代码为空，环境中没有 `go`/`goctl`，因此本计划要求同时产出产品规格、实现设计、go-zero 风格源码、proto/api 定义和可执行测试设计。

## 目标

- 完成 user 第一阶段产品规格。
- 完成 user-rpc/user-api go-zero 实现设计。
- 实现 user-rpc 逻辑：创建用户、按唯一标识查询用户、查询账号存在性、按 user_id 查询用户、更新自己的资料字段。
- 实现 user-api HTTP 接口：`GET /me`、`GET /users/:identifier`、`GET /users/exists`、`POST /users`、`PATCH /me`。
- 不引入密码、验证码、第三方凭据、好友关系或群成员关系字段。
- 提供单元测试或可运行验证脚本。
- 记录实际验证结果和 BLOCKER。

## 非目标

- 不实现 auth 登录注册。
- 不实现 PostgreSQL 持久化。
- 不实现 gateway/token 鉴权。
- 不实现 friends/groups 关系维护。

## 任务拆分

- [x] Task 1：阅读 `AGENTS.md` 和外层必读文档，确认边界。
- [x] Task 2：补充 `user` 第一阶段产品规格。
- [x] Task 3：补充 `user` go-zero 实现设计。
- [x] Task 4：创建本执行计划并记录 Planner 结果。
- [x] Task 5：实现 go-zero 风格目录、api/proto、配置和服务入口。
- [x] Task 6：实现用户模型、内存 repository、业务 logic 和 HTTP handler。
- [x] Task 7：补充单元测试或验证脚本。
- [x] Task 8：运行所有可运行验证命令，记录工具缺失 BLOCKER。
- [x] Task 9：Evaluator 检查代码、测试、文档一致性，修复问题。
- [ ] Task 10：提交当前 feature 分支。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 第一阶段使用 `X-User-Id` 模拟当前用户身份。 | `auth` 和 gateway 尚未实现，但 `/me` 与 `PATCH /me` 需要明确身份上下文。 |
| 2026-04-29 | 第一阶段使用内存 repository，并保持 repository 接口可替换。 | 当前目标是稳定 user 契约，PostgreSQL 迁移可在基础接口稳定后补充。 |
| 2026-04-29 | 环境缺少 `go`/`goctl` 时手写 go-zero 风格源码。 | 任务要求不能因工具缺失停止，需优先产出可维护源码和契约文件。 |
| 2026-04-29 | 文档写入 worktree 内 `docs/...` 副本。 | 沙箱拒绝写入外层 `/home/ws/project/docs/...`，当前无法请求提升权限。 |

## Planner 结果

- 已阅读当前 worktree `AGENTS.md`。
- 已阅读外层 `/home/ws/project/AGENTS.md`。
- 已阅读 `/home/ws/project/ARCHITECTURE.md`。
- 已阅读 `/home/ws/project/docs/PLANS.md`。
- 已阅读 `/home/ws/project/docs/product-specs/account-social-core.md`。
- 已阅读 `/home/ws/project/docs/design-docs/user-auth-friends-groups-boundaries.md`。
- 已在 worktree 内新增 `docs/product-specs/user-service.md`。
- 已在 worktree 内新增 `docs/design-docs/user-service-go-zero.md`。

## 验证方式

计划运行：

```bash
go version
goctl version
go test ./...
```

如 `go` 不存在，则记录 BLOCKER，并通过源码静态检查、文档对照和 `git status` 核验完成度。

## 风险与回滚

- 风险：手写 go-zero 骨架可能与未来 `goctl` 生成结构存在细节差异。缓解：保留 `api/user.api` 和 `proto/user.proto`，后续用 goctl 生成时以契约和 logic 为准迁移。
- 风险：内存 repository 不具备生产持久化能力。缓解：通过 repository 接口隔离，后续替换 PostgreSQL 实现。
- 回滚：移除本次新增服务目录、规格文件和任务文档即可，不影响其他服务。

## Generator 结果

- 已新增 `api/user.api` 和 `proto/user.proto`，定义 user-api 与 user-rpc 第一阶段契约。
- 已新增 `cmd/user-api/main.go` 与 `cmd/user-rpc/main.go` 服务入口。
- 已新增 `etc/user-api.yaml` 与 `etc/user-rpc.yaml` 配置占位。
- 已实现 `internal/model.User`，不包含密码、验证码、OAuth、credential、好友关系或群成员关系字段。
- 已实现 `internal/repository.UserRepository` 和线程安全内存实现，后续可替换 PostgreSQL。
- 已实现 `internal/logic.UserLogic`：创建用户、按 identifier 查询、存在性查询、按 user_id 查询、更新资料。
- 已实现 `internal/handler`：`GET /me`、`PATCH /me`、`POST /users`、`GET /users/exists`、`GET /users/{identifier}`。
- 已实现统一错误与 HTTP 响应封装。
- 已补充 `tests/user_service_test.go` 覆盖核心逻辑和 HTTP handler。

## Evaluator 结果

验证命令：

```bash
PATH=/tmp/go/bin:$PATH gofmt -w $(find . -name '*.go' -print)
PATH=/tmp/go/bin:$PATH go test ./...
```

验证结果：

```text
?   	github.com/wujunhui99/agents_im/cmd/user-api	[no test files]
?   	github.com/wujunhui99/agents_im/cmd/user-rpc	[no test files]
?   	github.com/wujunhui99/agents_im/internal/apperror	[no test files]
?   	github.com/wujunhui99/agents_im/internal/config	[no test files]
?   	github.com/wujunhui99/agents_im/internal/handler	[no test files]
?   	github.com/wujunhui99/agents_im/internal/logic	[no test files]
?   	github.com/wujunhui99/agents_im/internal/model	[no test files]
?   	github.com/wujunhui99/agents_im/internal/repository	[no test files]
?   	github.com/wujunhui99/agents_im/internal/response	[no test files]
?   	github.com/wujunhui99/agents_im/internal/rpc	[no test files]
?   	github.com/wujunhui99/agents_im/internal/svc	[no test files]
ok  	github.com/wujunhui99/agents_im/tests	0.002s
```

一致性检查：

- user 模型和响应未包含密码或认证秘密字段。
- `/me` 和 `PATCH /me` 使用 `X-User-Id` 作为第一阶段网关透传身份占位。
- `identifier` 创建时规范化并保证唯一，重复创建返回 `ALREADY_EXISTS`/HTTP 409。
- `PATCH /me` 通过严格 JSON 解码拒绝 `identifier`、`user_id` 等不可变字段。
- 文档中已记录 goctl 不可用和外层文档写入受限。

## BLOCKER

- `goctl version` 当前返回 `/bin/bash: line 1: goctl: command not found`。
- Codex 沙箱拒绝写入外层 `/home/ws/project/docs/...`，因此 Codex 先提交 worktree 内 `docs/...` 副本；控制会话可在完成后同步到外层文档。
