# User Service Go-Zero 实现设计

状态：Draft

## 背景

`user` 服务是账号资料的权威边界。当前环境没有 `go` 和 `goctl`，因此第一阶段代码以 go-zero 目录风格和接口契约为目标，优先保证源码结构、业务逻辑、proto/api/spec/docs/tests 可维护；当工具可用时再用 `goctl` 生成或校准骨架。

## 服务组成

### user-rpc

职责：

- 管理用户资料权威写入。
- 提供内部服务依赖的 RPC 能力。
- 供 `auth` 注册前检查 `identifier` 是否存在，并创建用户资料。
- 供 `friends`、`groups` 后续查询用户存在性和公开资料。

RPC 方法：

- `CreateUser(CreateUserRequest) returns (UserResponse)`
- `GetUserByIdentifier(GetUserByIdentifierRequest) returns (UserResponse)`
- `ExistsByIdentifier(ExistsByIdentifierRequest) returns (ExistsByIdentifierResponse)`
- `GetUserByID(GetUserByIDRequest) returns (UserResponse)`
- `UpdateUserProfile(UpdateUserProfileRequest) returns (UserResponse)`

### user-api

职责：

- 对外提供 HTTP 接口。
- 从请求头或后续网关上下文读取当前用户身份。
- 调用 user logic 完成资料读写。

HTTP 接口：

- `GET /me`
- `GET /users/:identifier`
- `GET /users/exists?identifier=...`
- `POST /users`
- `PATCH /me`

## 目录结构

```text
api/user.api
cmd/user-api/main.go
cmd/user-rpc/main.go
etc/user-api.yaml
etc/user-rpc.yaml
internal/config
internal/handler
internal/logic
internal/model
internal/repository
internal/response
internal/rpc
internal/service
internal/svc
proto/user.proto
tests/user_service_test.go
```

## 数据模型

`User` 字段：

- `UserID string`
- `Identifier string`
- `DisplayName string`
- `Name string`
- `Gender string`
- `Age int32`
- `Region string`
- `CreatedAt time.Time`
- `UpdatedAt time.Time`

约束：

- `Identifier` 经 `NormalizeIdentifier` 处理后唯一。
- `Gender` 只允许空值、`unknown`、`male`、`female`、`other`。
- `Age` 为 `0` 表示未设置；设置时范围为 `1..150`。
- `UserID` 由 repository 生成，内存实现使用递增 ID。

禁止字段：

- `password`
- `password_hash`
- 验证码
- OAuth/第三方登录凭据
- 好友关系或群成员关系

## 错误处理

业务错误使用统一 `apperror.Error` 表达：

- `InvalidArgument`：参数非法，HTTP 400。
- `Unauthenticated`：缺少或非法身份上下文，HTTP 401。
- `NotFound`：用户不存在，HTTP 404。
- `AlreadyExists`：唯一标识符重复，HTTP 409。
- `Internal`：未预期错误，HTTP 500。

HTTP 响应格式：

```json
{
  "code": "OK",
  "message": "ok",
  "data": {}
}
```

错误响应使用同一结构，`data` 为 `null`。

## 鉴权上下文

第一阶段不实现 auth token 校验。`user-api` 通过请求头 `X-User-Id` 获取当前用户身份：

- `GET /me` 必须携带 `X-User-Id`。
- `PATCH /me` 必须携带 `X-User-Id`。
- handler 将该值传入 `GetUserByID` 或 `UpdateUserProfile`。

后续接入 gateway 后，网关负责 token 校验，并透传规范化后的用户身份和 `trace_id`。

## 测试方式

当前控制会话已临时安装 Go 到 `/tmp/go` 并完成验证；goctl 仍不可用：

- `/tmp/go/bin/go version`：`go version go1.22.12 linux/amd64`。
- `goctl version` 不可用。
- 已运行 `PATH=/tmp/go/bin:$PATH go test ./...`。

工具可用后运行：

```bash
go test ./...
go run ./cmd/user-api -f etc/user-api.yaml
go run ./cmd/user-rpc -f etc/user-rpc.yaml
```

关键测试用例：

- 创建用户成功。
- 重复 `identifier` 返回 `AlreadyExists`。
- `ExistsByIdentifier` 返回存在和不存在两种结果。
- `/me` 缺少 `X-User-Id` 返回未认证。
- `PATCH /me` 只能更新允许字段。
- 资料模型和 HTTP/RPC 响应不包含密码或认证秘密。

## 后续演进

- 使用 `goctl api go` 和 `goctl rpc protoc` 重新生成骨架并对齐手写逻辑。
- 将内存 repository 替换为 PostgreSQL repository，增加迁移脚本和唯一索引。
- 为 `auth` 注册流程增加幂等创建、补偿或 outbox 设计。
- 接入 gateway 鉴权、trace_id 透传和结构化日志。
- 当前执行环境无法写入外层 `/home/ws/project/docs/design-docs/user-service-go-zero.md`，本文件为 worktree 内可提交副本。

