# Groups Service Go-Zero 实现设计

状态：Draft

## 背景

`groups` 服务负责群聊基础信息和群成员关系。当前 REST 与 RPC transport 已按 goctl/go-zero 生成结构校准，旧手写 HTTP mux handler 和 RPC contract wrapper 已移除；业务 logic、repository 和单元测试继续保留。

## 服务组成

### groups-rpc

职责：

- 管理群聊基础信息与群成员关系。
- 提供内部服务依赖的 RPC 能力。
- 调用或依赖 user-rpc 语义做成员存在性校验。
- 供 IM Core 后续查询群与成员关系。

RPC 方法：

- `CreateGroup(CreateGroupRequest) returns (GroupResponse)`
- `GetGroup(GetGroupRequest) returns (GroupResponse)`
- `AddMember(AddMemberRequest) returns (MemberResponse)`
- `JoinGroup(JoinGroupRequest) returns (MemberResponse)`
- `LeaveGroup(LeaveGroupRequest) returns (MemberResponse)`
- `ListMembers(ListMembersRequest) returns (ListMembersResponse)`

### groups-api

职责：

- 对外提供 HTTP 接口。
- 从 JWT context `user_id` 读取当前用户身份。
- 调用 groups logic 完成群和成员关系读写。

HTTP 接口：

- `POST /groups`
- `GET /groups/:group_id`
- `POST /groups/:group_id/members`
- `DELETE /groups/:group_id/members/me`
- `GET /groups/:group_id/members`

## 目录结构

```text
api/groups.api
cmd/groups-api/main.go
cmd/groups-rpc/main.go
etc/groups-api.yaml
etc/groups-rpc.yaml
internal/handler/groups
internal/logic/groupslogic.go
internal/model/group.go
internal/repository/groups_repository.go
internal/repository/groups_memory.go
internal/rpcgen/groups
internal/svc/service_context.go
proto/groups.proto
tests/groups_service_test.go
```

## 数据模型

`Group` 字段：

- `GroupID string`
- `Name string`
- `Description string`
- `CreatorUserID string`
- `CreatedAt time.Time`
- `UpdatedAt time.Time`

`GroupMember` 字段：

- `GroupID string`
- `UserID string`
- `State string`
- `JoinedAt time.Time`
- `LeftAt time.Time`

约束：

- `GroupID` 由 repository 生成，内存实现使用递增 ID。
- `Name` 必填，最多 64 个字符。
- `Description` 最多 256 个字符。
- 同一个 `group_id + user_id` 只能有一条成员关系记录。
- 成员 `state=active` 表示有效成员，`state=left` 表示已退出。

禁止字段：

- 用户名称、头像、性别、地区等用户资料权威字段。
- `password`、`password_hash`、验证码、OAuth/第三方登录凭据。
- 好友关系字段。

## user-rpc 依赖

业务逻辑通过窄接口依赖用户存在性：

```go
type UserExistenceChecker interface {
    EnsureUserExists(ctx context.Context, userID string) error
}
```

第一阶段本仓库没有真实 gRPC client 生成代码，因此本地实现使用适配器调用 `UserLogic.GetUserByID`，保持 `user-rpc` 的 `GetUserByID(user_id)` 语义：

- 返回成功：用户存在，可以创建群、加群或添加成员。
- 返回 `NOT_FOUND`：目标用户不存在，groups 返回明确不存在错误。
- 返回其他错误：透传为业务错误或内部错误。

后续接入真实 `user-rpc` 后，只替换该接口实现，不修改 groups 业务逻辑和 repository。

## 错误处理

业务错误继续使用统一 `apperror.Error`：

- `InvalidArgument`：参数非法，HTTP 400。
- `Unauthenticated`：缺少或非法身份上下文，HTTP 401。
- `NotFound`：群或用户或有效成员不存在，HTTP 404。
- `AlreadyExists`：预留给未来非幂等冲突语义，HTTP 409。
- `Internal`：未预期错误，HTTP 500。

重复加群第一阶段不返回错误，而返回：

```json
{
  "already_member": true
}
```

HTTP 响应沿用统一 envelope：

```json
{
  "code": "OK",
  "message": "ok",
  "data": {}
}
```

## 鉴权上下文

`groups-api` 通过 go-zero JWT middleware 获取当前用户身份：

- `POST /groups` 必须携带 `Authorization: Bearer <access_token>`，token 用户作为创建者。
- `POST /groups/:group_id/members` 必须携带 `Authorization: Bearer <access_token>`，请求体不传 `user_id` 时添加 token 用户；添加其他用户时 token 用户必须是群 creator/owner。
- `DELETE /groups/:group_id/members/me` 必须携带 `Authorization: Bearer <access_token>`，表示 token 用户退出。
- `GET /groups/:group_id` 必须携带 `Authorization: Bearer <access_token>`，且 token 用户必须是 active 群成员。
- `GET /groups/:group_id/members` 必须携带 `Authorization: Bearer <access_token>`，且 token 用户必须是 active 群成员。

JWT middleware 校验 token 后将 `user_id` claim 注入 context；logic adapter 使用该 user id 调用 groups 业务逻辑。`X-User-Id` 不作为生产鉴权路径。

## 测试方式

计划运行：

```bash
gofmt -w $(find . -name '*.go' -print)
go test ./...
scripts/verify-static.sh
```

如 `goctl` 不存在，则记录 BLOCKER，并保留手写 go-zero 风格结构。

关键测试用例：

- 创建群成功，并自动添加创建者为成员。
- 加群成功。
- 重复加群返回 `already_member=true` 且成员列表不重复。
- 退群后成员列表不包含该成员。
- 非成员查询群详情或成员列表返回 `FORBIDDEN`。
- 非 owner 添加其他用户返回 `FORBIDDEN`。
- 查询群不存在返回 `NOT_FOUND`。
- 添加不存在用户返回 `NOT_FOUND`。
- user 既有测试继续通过。

## 后续演进

- 使用 `goctl api go` 和 `goctl rpc protoc` 重新生成或校准骨架。
- 将内存 repository 替换为 PostgreSQL repository，增加迁移脚本、唯一索引和成员状态索引。
- 引入群角色、管理员、邀请、审批、禁言、群昵称和群头像。
- 接入 gateway 鉴权、trace_id 透传、结构化日志和 metrics。
