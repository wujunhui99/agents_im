# JWT Auth Middleware

状态：Active

## 背景

当前 user、friends、groups、message HTTP 接口通过 `X-User-Id` 模拟当前用户身份。auth 已能签发 HMAC/JWT-like token，但受保护接口尚未接入 go-zero JWT 路由鉴权，也没有统一的 context user id helper。

## 目标

- auth 注册和登录返回标准 JWT 三段 token，payload 至少包含 `user_id`，过期时间来自配置。
- user、friends、groups、message 中需要登录态的 HTTP route 使用 go-zero `jwt: Auth` 语义和统一路由中间件。
- handler/logic adapter 从 request context 获取 `user_id`，不再把 `X-User-Id` 作为生产主路径。
- message send 以 token 用户作为 sender；请求体如带 `senderId` 必须与 token 一致。
- 覆盖无 token、无效 token、Bearer token 访问 `/me`、以及 friends/groups/message 使用 token 用户的测试。
- 更新 JWT 设计、auth 产品规格和静态验证脚本。

## 非目标

- 不实现 refresh token、logout、revoke、手机号登录或微信登录。
- 不实现 PostgreSQL 持久化，不修改 docker-compose，不新增或修改 SQL migrations。
- 不合并 main/develop 或其他 feature 分支。

## 任务拆分

- [x] Task 1：读取仓库入口、架构、go-zero 规范和服务边界文档。
- [x] Task 2：更新 `.api` JWT 声明和本地 JWT 配置。
- [x] Task 3：实现统一 context user id helper，并迁移 go-zero logic adapters。
- [x] Task 4：让 auth API/RPC 使用配置化 JWT secret/expire 签发和校验 token。
- [x] Task 5：更新 HTTP 测试和静态验证。
- [x] Task 6：更新设计/产品文档，记录 message sender 规则。
- [x] Task 7：运行强制验证并记录结果。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 使用 go-zero route-level JWT，而不是自定义生产鉴权头。 | 与 `.api` 的 `jwt: Auth` 声明和 go-zero 生成模式一致。 |
| 2026-04-29 | 受保护 logic adapter 从 context 的 `user_id` claim 取当前用户。 | go-zero JWT middleware 会把非标准 claims 注入 request context。 |
| 2026-04-29 | `X-User-Id` 不再作为生产 HTTP 鉴权主路径。 | 避免未认证 header 绕过受保护接口。 |
| 2026-04-29 | message send 的 sender 以 token `user_id` 为准；请求体 `senderId` 如出现必须一致。 | 防止客户端伪造 sender，同时兼容显式 sender 字段的客户端。 |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name "*.go" -print)
go test ./...
bash scripts/verify-static.sh
git status --short --branch
```

## 风险与回滚

- 风险：测试 helper 直接读取 `server.Routes()` 会绕过 go-zero route options。处理方式：测试改用 `rest.NewServerless` 构建完整 route chain。
- 风险：各服务必须使用同一 JWT secret。处理方式：API/RPC config 均提供 `Auth.AccessSecret` 和 `Auth.AccessExpire`，本地使用 placeholder。
- 回滚：可回退本分支提交，恢复 `X-User-Id` 兼容测试路径；不涉及数据库或外部服务状态。

## 结果记录

- 已在 `api/user.api`、`api/friends.api`、`api/groups.api`、`api/message.api` 为受保护 route 增加 `jwt: Auth`。
- 已新增 `internal/ctxuser.UserID`，并迁移 user/friends/groups/message go-zero logic adapter 从 context claim 获取当前用户。
- 已将 auth API/RPC token manager 改为使用 `Auth.AccessSecret` 和 `Auth.AccessExpire`。
- 已更新测试，覆盖 JWT 形态、无 token 401、无效 token 401、Bearer token 访问 `/me`、token 用户注入 friends/groups/message、`X-User-Id` 不能绕过受保护接口。
- 已新增 `docs/design-docs/jwt-auth-middleware.md`，并更新 auth/user/friends/groups/message 相关规格说明。
- 已运行并通过：`goctl api validate`、`gofmt`、`go test ./...`、`bash scripts/verify-static.sh`。
