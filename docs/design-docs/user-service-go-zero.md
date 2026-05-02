# Account Service Go-Zero 实现设计

状态：Draft

## 背景

Account Service 是账号资料的权威边界。Account 可代表 human user、agent、admin，未来可扩展 service/official accounts。当前 REST 与 RPC transport 仍保留 `user-api` / `user-rpc` / `proto/user.proto` V0 compatibility，旧手写 HTTP mux 注册层和 `internal/rpc` wrapper 已移除；业务行为继续由 `internal/logic` 与 repository 承载。

术语规则：

- 领域与服务名使用 Account Service。
- `account_type` 支持 `user`、`agent`、`admin`；旧 `normal` 仅作为迁移输入兼容并归一化为 `user`。
- Public JSON/RPC 字段 `user_id` 是 account id alias，第一阶段不批量改名。
- PostgreSQL source-of-truth 表为 `accounts` 与 `profiles`，不再使用 `users` 表保存资料。
- Account/Agent ID 均由 Snowflake 算法生成，为无前缀数字字符串。

## 服务组成

### user-rpc（V0 Account RPC transport）

职责：

- 管理账号资料权威写入。
- 提供内部服务依赖的 RPC 能力。
- 供 `auth` 注册前检查 `identifier` 是否存在，并创建账号资料。
- 供 `friends`、`groups` 后续查询账号存在性和公开资料。

RPC 方法：

- `CreateUser(CreateUserRequest) returns (UserResponse)`
- `GetUserByIdentifier(GetUserByIdentifierRequest) returns (UserResponse)`
- `ExistsByIdentifier(ExistsByIdentifierRequest) returns (ExistsByIdentifierResponse)`
- `GetUserByID(GetUserByIDRequest) returns (UserResponse)`
- `UpdateUserProfile(UpdateUserProfileRequest) returns (UserResponse)`

### user-api（V0 Account HTTP transport）

职责：

- 对外提供 HTTP 接口。
- 从请求头或后续网关上下文读取当前 account id。
- 调用 Account/User compatibility logic 完成资料读写。

HTTP 接口：

- `GET /me`
- `GET /users/:identifier`
- `GET /users/exists?identifier=...`
- `POST /users`
- `PATCH /me`
- `GET /accounts/:identifier`
- `GET /accounts/exists?identifier=...`
- `POST /accounts`

`/accounts/*` 是 Account Service alias；`/users/*` 是 V0 compatibility path。

## 目录结构

```text
api/user.api
cmd/user-api/main.go
cmd/user-rpc/main.go
etc/user-api.yaml
etc/user-rpc.yaml
internal/config
internal/handler/gozero_routes.go
internal/handler/user
internal/types/types.go
internal/logic
internal/model
internal/repository
internal/response
internal/rpcgen/user
internal/service
internal/svc
proto/user.proto
tests/user_service_test.go
```

## 数据模型

Account identity 字段：

- `AccountID string`：Snowflake account id，无前缀数字字符串。
- `Identifier string`
- `AccountType string`
- `AccountCreatedAt time.Time`
- `AccountUpdatedAt time.Time`

Profile 字段（当前 Go/Proto V0 类型名仍为 `User`）：

- `UserID string`：V0 字段名，语义是 account id alias。
- `DisplayName string`
- `Name string`
- `Gender string`
- `Age int32`
- `Region string`
- `AvatarMediaID string`
- `CreatedAt time.Time`
- `UpdatedAt time.Time`

约束：

- `Identifier` 经 `NormalizeIdentifier` 处理后唯一。
- `Gender` 只允许空值、`unknown`、`male`、`female`、`other`。
- `Age` 为 `0` 表示未设置；设置时范围为 `1..150`。
- `AccountType` 只允许空值、`user`、`agent`、`admin`；空值在 logic 和 repository 层统一归一化为 `user`。
- 旧 `normal` 输入只为 V0 迁移兼容保留，写入/返回统一为 `user`。
- `UserID` / `AccountID` 由 repository 通过 Snowflake 生成；`UserID` 是 V0 account id alias。

禁止字段：

- `password`
- `password_hash`
- 验证码
- OAuth/第三方登录凭据
- 好友关系或群成员关系

账号类型边界：

- HTTP `POST /users` / `POST /accounts` 不把请求体中的 `account_type` 传入业务 logic，公开创建始终得到 `user`。
- `auth` 注册通过 user adapter 创建账号资料，未传 `account_type`，因此始终得到 `user`。
- User RPC `CreateUserRequest.account_type` 是内部能力，可创建 `agent` 或 `admin`；非法值必须映射为 `INVALID_ARGUMENT`/gRPC `InvalidArgument`，不能降级为 `user`。
- 后续若新增 Account RPC transport，必须先提供兼容层，不能破坏当前 User RPC client。

## 错误处理

业务错误使用统一 `apperror.Error` 表达：

- `InvalidArgument`：参数非法，HTTP 400。
- `Unauthenticated`：缺少或非法身份上下文，HTTP 401。
- `NotFound`：账号不存在，HTTP 404。
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

`user-api` 通过 go-zero JWT middleware 获取当前账号身份：

- `GET /me` 必须携带 `Authorization: Bearer <access_token>`。
- `PATCH /me` 必须携带 `Authorization: Bearer <access_token>`。
- `PATCH /me/avatar` 必须携带 `Authorization: Bearer ***`，请求体中的 `media_id` 必须指向当前账号拥有的 `purpose=avatar`、`status=ready` media object。
- go-zero middleware 校验 token 后将 `user_id` claim 注入 context；该字段是 V0 account id alias，再由 logic adapter 调用 `GetUserByID`、`UpdateUserProfile` 或 `UpdateUserAvatar`。

`X-User-Id` 不作为生产鉴权路径；测试仅验证它不能绕过 JWT。

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

- 创建账号成功。
- 重复 `identifier` 返回 `AlreadyExists`。
- `ExistsByIdentifier` 返回存在和不存在两种结果。
- `/me` 缺少 Bearer token 返回未认证。
- `PATCH /me` 只能更新允许字段。
- `PATCH /me/avatar` 拒绝非 owner、非 avatar purpose 或 not-ready media。
- 资料模型和 HTTP/RPC 响应不包含密码或认证秘密。
- `/accounts/*` aliases 与 `/users/*` 使用同一真实 handler。
- `account_type` 默认输出 `user`，内部创建支持 `agent`、`admin`，非法值失败。

## 后续演进

- 评估新增一等 `account.api` / `account.proto`，或继续使用 V0 user transport 名称。
- 评估是否需要只读 `users` compatibility view；默认新代码直接使用 `accounts` / `profiles`。
- 为 `auth` 注册流程增加幂等创建、补偿或 outbox 设计。
- 接入 gateway 长连接鉴权、trace_id 透传和结构化日志。
