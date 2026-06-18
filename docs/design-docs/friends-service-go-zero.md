# Friends Service Go-Zero 实现设计

状态：Draft

## 背景

`friends` 服务负责好友关系维护，位于 Account Service 之后开发。当前 REST 与 RPC transport 已按 goctl/go-zero 生成结构校准，旧手写 RPC wrapper 已移除；业务 logic、repository 和测试继续保留。V0 `user_id` / `friend_id` 字段是 account id alias。

## 服务组成

### friends-rpc

职责：

- 提供内部好友关系能力。
- 添加好友前依赖 V0 `user-rpc.GetUserByID` 校验双方账号存在。
- 维护好友关系状态，不保存账号资料权威数据。

RPC 方法：

- `AddFriend(AddFriendRequest) returns (AddFriendResponse)`
- `DeleteFriend(DeleteFriendRequest) returns (DeleteFriendResponse)`
- `ListFriends(ListFriendsRequest) returns (ListFriendsResponse)`
- `GetFriendship(GetFriendshipRequest) returns (GetFriendshipResponse)`

### friends-api

职责：

- 对外提供 HTTP 好友关系接口。
- 从 JWT context `user_id` 读取当前 account id。
- 调用 friends logic 完成添加、删除、列表和关系查询。

HTTP 接口：

- `POST /friends`
- `DELETE /friends/{user_id}`
- `GET /friends`
- `GET /friends/{user_id}`

## 目录结构

```text
service/friends/api/friends.go                  # friends-api 入口（package main）
service/friends/api/friends.api
service/friends/api/etc/friends-api.yaml
service/friends/api/internal/{config,handler,logic,svc,types}
service/friends/rpc/friends.go                  # friends-rpc 入口（package main）
service/friends/rpc/friends.proto
service/friends/rpc/etc/friends-rpc.yaml
service/friends/rpc/internal/{config,logic,model,server,svc}  # 业务逻辑 + goctl 数据层
service/friends/rpc/{friends,friendsclient}     # goctl 生成的 pb / client
pkg/model/friendship.go                # 共享数据模型（迁出 internal/model，#397）
internal/repository/postgres_user_friends.go    # 旧好友数据层（暂留喂 monolith，#426 待删）
```

业务逻辑与好友状态机集中在 `service/friends/rpc/internal/logic`，经 `internal/svc` 注入 goctl 数据层
`service/friends/rpc/internal/model`（`friendships` 表，#426 退役 `core`）；friends-rpc 不再依赖
`internal/repository`。跨域好友资料由 friends-api(BFF) 聚合 user-rpc 补全。REST/RPC 的共享基础包
（错误映射、auth/token 等）落在 `pkg/*`。

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

friends-rpc 数据层用 goctl model（`service/friends/rpc/internal/model`，#426）落 PostgreSQL `friendships` 表：
双向单向行 + `(account_id, friend_account_id)` 唯一约束（迁移 `018` 加自增代理 PK 以适配 goctl），
事务边界在 Logic 层（model 暴露 `Transact` + `WithSession`）保证双向状态一致。

## user-rpc 依赖（BFF 聚合）

friends-rpc 只维护好友关系本身，不读账号表、不调其它 rpc；跨域用户资料在 friends-api(BFF) 聚合：

- friends-api 持 `UserRPC` client，列表用批量 `GetUsersByIDs` 补全好友资料（无 N+1），单条用 `GetUserByID`。
- `Friendship.friend`（proto 字段保留）由 BFF 填充：好友列表 / accept / reject 展示 `friend_id` 的资料，
  收到的请求（incoming）展示发起方 `user_id` 的资料。
- 账号已注销（profile 缺失）按空资料降级，不阻断整列表。

friends 不保存 `identifier`、`display_name` 等权威资料。

## 错误处理

业务错误继续使用统一 `apperror.Error`：

- `InvalidArgument`：参数非法，HTTP 400。
- `Unauthenticated`：缺少、过期或非法 Bearer token，HTTP 401。
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

`friends-api` 通过 go-zero JWT middleware 获取当前用户身份：

- `POST /friends` 必须携带 `Authorization: Bearer <access_token>`。
- `DELETE /friends/{user_id}` 必须携带 `Authorization: Bearer <access_token>`。
- `GET /friends` 必须携带 `Authorization: Bearer <access_token>`。
- `GET /friends/{user_id}` 必须携带 `Authorization: Bearer <access_token>`。

JWT middleware 校验 token 后将 `user_id` claim 注入 context；logic adapter 使用该 user id 调用 friends 业务逻辑。`X-User-Id` 不作为生产鉴权路径。

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
