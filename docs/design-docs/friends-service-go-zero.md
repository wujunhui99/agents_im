# Friends Service Go-Zero 实现设计

状态：Draft

## 背景

`friends` 服务负责好友关系维护，位于 `user` 之后开发。当前 REST 与 RPC transport 已按 goctl/go-zero 生成结构校准，旧手写 RPC wrapper 已移除；业务 logic、repository 和测试继续保留。

## 服务组成

### friends-rpc

职责：

- 提供内部好友关系能力。
- 添加好友前依赖 `user-rpc.GetUserByID` 校验双方用户存在。
- 维护好友关系状态，不保存用户资料权威数据。

RPC 方法：

- `AddFriend(AddFriendRequest) returns (AddFriendResponse)`
- `DeleteFriend(DeleteFriendRequest) returns (DeleteFriendResponse)`
- `ListFriends(ListFriendsRequest) returns (ListFriendsResponse)`
- `GetFriendship(GetFriendshipRequest) returns (GetFriendshipResponse)`

### friends-api

职责：

- 对外提供 HTTP 好友关系接口。
- 从 `X-User-Id` 读取当前用户身份。
- 调用 friends logic 完成添加、删除、列表和关系查询。

HTTP 接口：

- `POST /friends`
- `DELETE /friends/{user_id}`
- `GET /friends`
- `GET /friends/{user_id}`

## 目录结构

```text
api/friends.api
cmd/friends-api/main.go
cmd/friends-rpc/main.go
etc/friends-api.yaml
etc/friends-rpc.yaml
internal/handler/friends routes
internal/logic/friendslogic.go
internal/model/friendship.go
internal/repository/friendship repository
internal/rpcgen/friends
proto/friends.proto
tests/friends_service_test.go
```

当前 worktree 的 `internal/apperror`、`internal/response`、`internal/config`、`internal/svc` 作为服务内共享基础包复用。

## 数据模型

`Friendship` 字段：

- `UserID string`
- `FriendID string`
- `Status string`
- `CreatedAt time.Time`
- `UpdatedAt time.Time`

状态：

- `active`：有效好友关系。
- `deleted`：曾经存在但已删除，当前不是好友。
- `none`：没有关系记录，当前不是好友。

第一阶段 repository 使用内存实现，通过接口隔离：

```go
type FriendshipRepository interface {
    AddFriend(ctx context.Context, userID string, friendID string) (model.Friendship, bool, error)
    DeleteFriend(ctx context.Context, userID string, friendID string) (model.Friendship, bool, error)
    ListFriends(ctx context.Context, userID string) ([]model.Friendship, error)
    GetFriendship(ctx context.Context, userID string, friendID string) (model.Friendship, error)
}
```

后续 PostgreSQL 表建议使用规范化双向关系或有序 pair 唯一索引，并通过事务保证双向状态一致。

## user-rpc 依赖

friends logic 只依赖窄接口：

```go
type UserLookup interface {
    GetUserByID(ctx context.Context, req GetUserByIDRequest) (UserProfile, error)
}
```

第一阶段本地测试使用已有 `UserLogic` 适配该接口；生产化时替换为 `user-rpc` client，调用 `GetUserByID` 校验：

- 当前 `X-User-Id` 对应用户存在。
- `POST /friends` 的目标用户存在。
- `GET /friends/{user_id}` 的目标用户存在。

friends 不保存 `identifier`、`display_name` 等权威资料；如后续列表需要展示公开资料，应由 API 聚合调用 `user-rpc` 或由客户端按需查询。

## 错误处理

业务错误继续使用统一 `apperror.Error`：

- `InvalidArgument`：参数非法，HTTP 400。
- `Unauthenticated`：缺少 `X-User-Id`，HTTP 401。
- `NotFound`：用户或好友关系不存在，HTTP 404。
- `AlreadyExists`：保留给后续非幂等语义；第一阶段重复添加返回 `created=false`。
- `Internal`：未预期错误，HTTP 500。

HTTP 响应格式沿用：

```json
{
  "code": "OK",
  "message": "ok",
  "data": {}
}
```

## 鉴权上下文

第一阶段不实现 auth token 校验。`friends-api` 通过请求头 `X-User-Id` 获取当前用户身份：

- `POST /friends` 必须携带 `X-User-Id`。
- `DELETE /friends/{user_id}` 必须携带 `X-User-Id`。
- `GET /friends` 必须携带 `X-User-Id`。
- `GET /friends/{user_id}` 必须携带 `X-User-Id`。

后续接入 gateway 后，gateway 负责 token 校验，并透传规范化后的用户身份与 `trace_id`。

## 测试方式

计划运行：

```bash
goctl version
gofmt -w $(find . -name '*.go' -print)
go test ./...
scripts/verify-static.sh
```

关键测试用例：

- 添加好友成功并双向可见。
- 重复添加同一好友返回 `created=false`。
- 删除好友后列表不再返回，关系失效。
- 查询好友列表只返回有效好友。
- 不能添加自己。
- 添加好友时当前用户或目标用户不存在返回 `NotFound`。

## 后续演进

- 使用 `goctl api go` 和 `goctl rpc protoc` 重新生成或校准骨架。
- 将内存 repository 替换为 PostgreSQL repository，补充迁移脚本和唯一索引。
- 引入好友申请审批、备注、分组和黑名单。
- 接入正式 gateway 鉴权、trace_id 透传、结构化日志和指标。
