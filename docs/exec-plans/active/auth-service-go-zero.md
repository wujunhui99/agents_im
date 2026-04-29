# Auth Service Go-Zero

状态：Active

## 背景

当前 worktree 位于 `feature/auth-service`，目标是在已合入 main 的 user 服务契约基础上开发 auth 第一阶段能力。auth 只负责认证秘密、密码校验和 token，注册时依赖 user 的 `ExistsByIdentifier` 与 `CreateUser` 完成账号存在性检查和资料初始化。

## 目标

- 完成 auth 第一阶段产品规格。
- 完成 auth-rpc/auth-api go-zero 实现设计。
- 实现 auth-rpc 逻辑：`Register`、`Login`、`ValidateToken`。
- 实现 auth-api HTTP 接口：`POST /auth/register`、`POST /auth/login`、`POST /auth/validate`。
- 密码哈希和 salt 只出现在 auth 内部模型与 repository 中。
- 注册流程先调用 user `ExistsByIdentifier`，确认不存在后调用 `CreateUser`，再保存 auth password_hash。
- 当前无真实 RPC 网络时，通过接口 adapter 调用 user logic；文档记录后续切换 go-zero RPC client。
- token 第一阶段使用 HMAC/JWT-like 简化实现，必须有过期时间和测试。
- 保持 user 已有测试通过。
- 完成 gofmt、`go test ./...`、`scripts/verify-static.sh` 验证。
- 完成后提交当前 feature 分支。

## 非目标

- 不实现手机号验证码注册或登录。
- 不实现微信扫码、OAuth 或第三方登录。
- 不实现 PostgreSQL 持久化。
- 不实现真实 go-zero RPC 网络传输。
- 不修改其他 worktree，不切换分支。

## 任务拆分

- [x] Task 1：阅读 `AGENTS.md` 和指定 user 服务文档。
- [x] Task 2：阅读外层架构、计划规范和 user/auth/friends/groups 边界文档。
- [x] Task 3：补充 `auth` 第一阶段产品规格。
- [x] Task 4：补充 `auth` go-zero 实现设计。
- [x] Task 5：创建本执行计划并记录 Planner 结果。
- [x] Task 6：实现 auth api/proto、配置和服务入口。
- [x] Task 7：实现 auth 模型、内存 repository、password hasher、token manager、user adapter 和业务 logic。
- [x] Task 8：实现 auth HTTP handler 和 RPC server 占位。
- [x] Task 9：补充单元测试覆盖注册成功、重复账号、登录成功、密码错误、token 校验。
- [x] Task 10：运行 gofmt、`go test ./...`、`scripts/verify-static.sh` 并记录结果。
- [x] Task 11：Evaluator 检查代码、测试、文档一致性，修复问题。
- [ ] Task 12：提交当前 feature 分支（Blocked：当前沙箱无法写入 worktree 的 git metadata）。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | auth 新代码放在 `internal/auth/...`。 | 避免污染现有 user 服务的 `internal/logic`、`internal/repository` 和 user 模型。 |
| 2026-04-29 | 第一阶段通过 user logic adapter 调用 `ExistsByIdentifier` 和 `CreateUser`。 | 当前无真实 RPC 网络，后续可替换为 go-zero RPC client adapter。 |
| 2026-04-29 | 对外 token 校验接口选择 `POST /auth/validate`。 | 满足 token 验证需求，并避免与 user 的 `/me` 路径冲突。 |
| 2026-04-29 | token 使用 HMAC/JWT-like 本地实现。 | 无外部依赖即可验证签名和过期时间，后续可替换标准 JWT 库。 |

## Planner 结果

- 已阅读当前 worktree `AGENTS.md`。
- 已阅读 `docs/product-specs/user-service.md`。
- 已阅读 `docs/design-docs/user-service-go-zero.md`。
- 已阅读 `docs/exec-plans/active/user-service-go-zero.md`。
- 已阅读外层 `/home/ws/project/AGENTS.md`。
- 已阅读 `/home/ws/project/ARCHITECTURE.md`。
- 已阅读 `/home/ws/project/docs/PLANS.md`。
- 已阅读 `/home/ws/project/docs/product-specs/account-social-core.md`。
- 已阅读 `/home/ws/project/docs/design-docs/user-auth-friends-groups-boundaries.md`。
- 已新增 `docs/product-specs/auth-service.md`。
- 已新增 `docs/design-docs/auth-service-go-zero.md`。

## 验证方式

计划运行：

```bash
go version
goctl version
PATH=/tmp/go/bin:$PATH gofmt -w $(find . -name '*.go' -print)
PATH=/tmp/go/bin:$PATH go test ./...
PATH=/tmp/go/bin:$PATH scripts/verify-static.sh
```

如 `goctl` 不存在，记录 blocker，并以手写 go-zero 风格结构继续实现。

## 风险与回滚

- 风险：手写 go-zero 骨架与未来 `goctl` 生成结构存在细节差异。缓解：保留 `api/auth.api` 和 `proto/auth.proto`，后续用 goctl 校准。
- 风险：注册流程跨 user/auth 两个边界，真实分布式环境中可能出现 user 创建成功但 auth credential 保存失败。缓解：后续增加幂等、补偿或事务外盒机制。
- 风险：第一阶段密码哈希和 token 密钥管理不满足生产安全要求。缓解：文档标记为第一阶段实现，后续替换为生产级 KDF 和密钥管理。
- 回滚：移除本次新增 auth 服务目录、规格文件、任务文档和测试，并恢复静态验证脚本即可。

## Generator 结果

- 已新增 `api/auth.api` 和 `proto/auth.proto`，定义 auth-api 与 auth-rpc 第一阶段契约。
- 已新增 `cmd/auth-api/main.go` 与 `cmd/auth-rpc/main.go` 服务入口。
- 已新增 `etc/auth-api.yaml` 与 `etc/auth-rpc.yaml` 配置占位。
- 已新增 `internal/auth/model.Credential`，仅 auth 内部包含 `PasswordHash`、`Salt`、`HashVersion`。
- 已新增 `internal/auth/repository.CredentialRepository` 和线程安全内存实现。
- 已新增 `internal/auth/useradapter.LogicClient`，通过 user logic 适配 `ExistsByIdentifier` 与 `CreateUser`，后续可替换 go-zero RPC client。
- 已新增 `internal/auth/logic.AuthLogic`：`Register`、`Login`、`ValidateToken`、`ParseToken`。
- 已新增标准库实现的 salted iterative SHA-256 password hasher，响应不暴露 hash 或 salt。
- 已新增 HMAC/JWT-like token manager，包含 `iat`、`exp`、签名校验和过期校验。
- 已新增 `internal/auth/handler`：`POST /auth/register`、`POST /auth/login`、`POST /auth/validate`。
- 已新增 `internal/auth/rpc.AuthServer` 占位，等待 goctl/protoc 生成真实 RPC transport。
- 已新增 `tests/auth_service_test.go`，覆盖注册成功、重复账号、登录成功、密码错误、token 校验和 token 过期。
- 已更新 `scripts/verify-static.sh`，保留 user 边界禁止认证秘密字段，并增加 auth 契约检查。

## Evaluator 结果

验证命令：

```bash
PATH=/tmp/go/bin:$PATH gofmt -w $(find . -name '*.go' -print)
PATH=/tmp/go/bin:$PATH GOCACHE=/tmp/go-build go test ./...
PATH=/tmp/go/bin:$PATH scripts/verify-static.sh
goctl version
```

验证结果：

```text
go test ./...:
?   	github.com/wujunhui99/agents_im/cmd/auth-api	[no test files]
?   	github.com/wujunhui99/agents_im/cmd/auth-rpc	[no test files]
?   	github.com/wujunhui99/agents_im/cmd/user-api	[no test files]
?   	github.com/wujunhui99/agents_im/cmd/user-rpc	[no test files]
?   	github.com/wujunhui99/agents_im/internal/auth/handler	[no test files]
?   	github.com/wujunhui99/agents_im/internal/auth/logic	[no test files]
?   	github.com/wujunhui99/agents_im/internal/auth/model	[no test files]
?   	github.com/wujunhui99/agents_im/internal/auth/repository	[no test files]
?   	github.com/wujunhui99/agents_im/internal/auth/rpc	[no test files]
?   	github.com/wujunhui99/agents_im/internal/auth/svc	[no test files]
?   	github.com/wujunhui99/agents_im/internal/auth/token	[no test files]
?   	github.com/wujunhui99/agents_im/internal/auth/useradapter	[no test files]
ok  	github.com/wujunhui99/agents_im/tests	0.051s

scripts/verify-static.sh:
static verification passed

goctl version:
/bin/bash: line 1: goctl: command not found
```

环境说明：

- `go` 不在默认 `PATH`，本次使用 `/tmp/go/bin/go`。
- 默认 Go build cache 路径 `/home/ws/.cache/go-build` 对当前沙箱只读，因此 `go test` 使用 `GOCACHE=/tmp/go-build` 后通过。

一致性检查：

- `user` 的 `api/user.api`、`proto/user.proto`、`internal/model`、`internal/logic`、`internal/repository`、`internal/handler`、`internal/rpc`、`internal/svc` 未出现 `password`、`password_hash`、`salt` 或 `credential`。
- auth 注册代码顺序为 `ExistsByIdentifier` -> `CreateUser` -> auth credential 保存。
- `POST /auth/validate` 避免与 user 的 `/me` 冲突。
- auth HTTP 响应和测试断言均不泄露明文密码、password hash 或 salt。

## BLOCKER

- `goctl version` 当前返回 `/bin/bash: line 1: goctl: command not found`，因此本阶段手写 go-zero 风格结构，后续工具可用后再用 goctl 校准。
- `git add` 当前返回 `fatal: Unable to create '/home/ws/project/agents_im/.git/worktrees/auth/index.lock': Read-only file system`。代码与文档已完成并验证，但当前沙箱不能写入该 worktree 的 git metadata，因此无法在本会话内完成提交。
