# Auth Service Go-Zero 实现设计

状态：Draft

## 背景

`auth` 服务依赖已稳定的 Account Service 契约完成注册流程。当前 REST 与 RPC transport 已按 goctl/go-zero 生成结构校准，旧手写 `internal/auth/rpc` wrapper 已移除；业务逻辑仍通过接口隔离真实 RPC client 与本地 Account/User compatibility logic adapter。

## 服务组成

### auth-rpc

职责：

- 管理账号认证记录。
- 负责密码哈希、salt 和密码校验。
- 签发和校验 token。
- 通过 Account adapter 调用 `ExistsByIdentifier` 和 `CreateUser`。

RPC 方法：

- `Register(RegisterRequest) returns (AuthResponse)`
- `Login(LoginRequest) returns (AuthResponse)`
- `ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse)`
- `ParseToken(ValidateTokenRequest) returns (ValidateTokenResponse)`

### auth-api

职责：

- 对外提供 HTTP 注册、登录和 token 校验接口。
- 调用 auth logic 完成认证流程。
- 使用统一响应结构，不泄露密码哈希或 salt。

HTTP 接口：

- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/validate`

选择 `POST /auth/validate` 是为了避免与 `user-api` 的 `/me` 冲突；受保护 HTTP API 通过 go-zero JWT middleware 在本服务签发的 access token 中读取 `user_id`。

## 目录结构

```text
api/auth.api
cmd/auth-api/main.go
cmd/auth-rpc/main.go
etc/auth-api.yaml
etc/auth-rpc.yaml
internal/auth/logic
internal/auth/model
internal/auth/repository
internal/auth/token
internal/auth/useradapter
internal/handler/auth
internal/logic/auth
internal/rpcgen/auth
internal/servicecontext/auth
proto/auth.proto
tests/auth_service_test.go
```

Auth 的认证业务、模型、repository 仍放在 `internal/auth/...`。REST transport 与其它 go-zero API 一致：handler 位于 `internal/handler/auth`，REST adapter logic 位于 `internal/logic/auth`，运行时依赖注入位于 `internal/servicecontext/auth`。

## 数据模型

`auth` 内部认证记录：

- `Identifier string`
- `UserID string`
- `PasswordHash string`
- `Salt string`
- `HashVersion string`
- `CreatedAt time.Time`
- `UpdatedAt time.Time`

约束：

- `Identifier` 使用 Account Service 契约的规范化结果。
- `PasswordHash` 和 `Salt` 只存在于 `internal/auth/model` 与 auth repository。
- HTTP/RPC 响应只返回 `user_id`、`identifier`、`token`、`expires_at` 或 token 校验结果。

## 注册流程

1. 校验 `identifier` 和 `password`。
2. 调用 Account adapter 的 `ExistsByIdentifier(identifier)`。
3. 如果已存在，返回 `ALREADY_EXISTS`。
4. 调用 Account adapter 的 `CreateUser(...)` 创建账号资料。
5. 生成 salt 和 password_hash。
6. 保存 auth credential。
7. 签发 token 并返回。

当前 adapter 实现：

- `internal/auth/useradapter.LogicClient` 包装 `internal/logic.UserLogic` / Account compatibility logic。
- 它调用 `ExistsByIdentifier` 和 `CreateUser`，模拟后续 `user-rpc` client。
- 后续替换时只需要新增 go-zero RPC client adapter，auth logic 保持不变。

## RPC entry bridge

`cmd/auth-rpc` 通过 `internal/rpcgen/auth/entry.Start` 启动 goctl 生成的 `internal/rpcgen/auth/internal/{config,server,svc}`。这是 Go `internal` 包可见性限制下的命令入口桥接：`cmd/auth-rpc` 不能直接 import `internal/rpcgen/auth/internal/*`。该 bridge 不承载业务依赖，业务 wiring 仍在 goctl service context seam 内。

## 密码哈希

第一阶段不引入外部依赖，使用标准库实现：

- salt：`crypto/rand` 生成 16 字节随机值，base64url 编码。
- hash：salt + password 经多轮 SHA-256 迭代后 base64url 编码。
- 比较：使用 `hmac.Equal` 做常量时间比较。

生产化前应替换为 Argon2id、bcrypt 或 scrypt，并增加密码强度、登录失败限制和审计日志。

## Token

第一阶段 access token 是 HS256 JWT 三段结构：

```text
base64url(header).base64url(payload).base64url(signature)
```

payload 包含：

- `user_id`
- `identifier`
- `iat`
- `exp`

校验规则：

- 签名必须匹配当前 HMAC secret。
- `exp` 必须晚于当前时间。
- `user_id` 和 `identifier` 必须存在。

配置：

- `Auth.AccessSecret` 配置 JWT HMAC secret，本地 YAML 只使用开发 placeholder。
- `Auth.AccessExpire` 配置 access token 过期秒数，当前本地开发值为 86400。
- 后续应切换为配置中心或密钥管理系统，并支持密钥轮换。

## 错误处理

沿用现有 `internal/apperror`：

- `InvalidArgument`：参数非法，HTTP 400。
- `Unauthenticated`：密码错误、token 非法或 token 过期，HTTP 401。
- `NotFound`：保留给后续需要区分不存在资源的场景。
- `AlreadyExists`：重复注册，HTTP 409。
- `Internal`：未预期错误，HTTP 500。

登录时不存在账号和密码错误均返回 `UNAUTHENTICATED`，避免账号枚举。

## 测试方式

必须覆盖：

- 注册成功。
- 重复账号。
- 登录成功。
- 密码错误。
- token 校验。
- Bearer token 可访问受保护 `/me`。
- 旧 `X-User-Id` header 不能绕过受保护路由。

验证命令：

```bash
PATH=/tmp/go/bin:$PATH gofmt -w $(find . -name '*.go' -print)
PATH=/tmp/go/bin:$PATH go test ./...
PATH=/tmp/go/bin:$PATH scripts/verify-static.sh
```

`goctl` 当前不可用，需在任务文档记录 blocker。

## 后续演进

- 使用 `goctl api go` 和 `goctl rpc protoc` 校准生成骨架。
- 将内存 auth repository 替换为 PostgreSQL repository，并增加唯一索引和迁移脚本。
- 用真实 Account Service RPC client 替换本地 Account/User compatibility logic adapter。
- 增加账号锁定、密码强度策略、审计日志、刷新 token 和 token 吊销。
- 增加手机号验证码、微信扫码等认证方式。
