# Friends Service 第一阶段产品规格

状态：Draft

## 背景

`friends` 是好友关系的权威服务，依赖 Account Service 的账号存在性能力，但不保存账号资料权威数据，不管理登录认证，也不维护群成员关系。第一阶段目标是在 Account Service 基础接口稳定后，提供好友申请、审批、删除、查询好友列表和查询好友关系能力，为后续 IM 会话和社交展示提供关系基础。V0 `user_id` / `friend_id` 字段是 account id alias。

## 目标

- 提供发起好友申请能力。
- 提供接受、拒绝好友申请能力。
- 提供删除好友能力。
- 提供当前账号好友列表查询能力。
- 提供当前账号 incoming/outgoing 好友申请查询能力。
- 提供当前账号与指定账号的好友关系查询能力。
- 添加好友前校验发起方和目标账号都存在。
- 通过 JWT Bearer token 解析当前用户身份。

## 非目标

- 不保存 `display_name`、`identifier`、头像等账号资料权威字段。
- 不保存密码、验证码、OAuth token 等认证秘密。
- 不验证登录密码、不签发 token、不实现登录注册。
- 不维护群成员关系。
- 不实现好友通知推送、好友备注、好友分组、黑名单。
- 不在第一阶段实现 PostgreSQL 持久化。

## 数据概念

第一阶段好友关系包含：

- `user_id`：当前 account id alias。
- `friend_id`：好友 account id alias。
- `status`：关系状态，第一阶段支持 `pending`、`accepted`、`rejected`、`deleted`、`none`；历史 `active` 仅作为读取兼容，新的服务响应使用 `accepted`。
- `is_friend`：是否为有效好友关系。
- `created_at` / `updated_at`：关系创建和更新时间。

关系语义：

- 添加好友成功后，系统写入 `pending` 申请，不进入普通好友列表。
- 只有申请接收方可以接受或拒绝 `pending` 申请。
- 接受后系统写入双向 `accepted` 关系。
- 拒绝后关系为 `rejected`，不会进入普通好友列表。
- 删除好友后，双向关系失效，列表不再返回该好友。
- 查询关系时，`accepted` 表示当前有效好友，`pending` 表示待处理，`rejected`、`deleted` 或 `none` 表示当前不是好友。

## 接口能力

### 添加好友

`POST /friends`

请求必须携带 `Authorization: Bearer <access_token>`，请求体包含目标账号的 V0 account id alias：

```json
{
  "user_id": "443081672744960001"
}
```

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- 目标 `user_id` 为空时返回参数错误。
- 不能添加自己为好友。
- 添加前必须通过 `user-rpc` 校验发起方和目标用户都存在。
- 任一用户不存在时返回明确不存在错误。
- 首次添加后返回 `pending` 和 `is_friend=false`，双方普通好友列表都不可见该关系。
- 请求方可在 outgoing 申请中看到该关系，接收方可在 incoming 申请中看到该关系。
- 重复添加同一 pending 申请保持幂等，返回明确 `created=false` 结果。

### 接受好友申请

`POST /friends/{user_id}/accept`

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- 只有 pending 申请的接收方可以接受；请求方或其他用户必须返回 `FORBIDDEN` 或明确失败。
- 接受后双方好友列表都可见该关系。
- 返回 `status=accepted` 和 `is_friend=true`。

### 拒绝好友申请

`POST /friends/{user_id}/reject`

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- 只有 pending 申请的接收方可以拒绝。
- 拒绝后双方普通好友列表都不可见该关系。
- 返回 `status=rejected` 和 `is_friend=false`。

### 删除好友

`DELETE /friends/{user_id}`

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- 目标 `user_id` 为空或非法路径时返回参数错误。
- 删除后双向关系失效。
- 删除后好友列表不再返回该用户。

### 查询好友列表

`GET /friends`

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- 当前用户不存在时返回明确不存在错误。
- 只返回当前有效 `accepted` 好友关系；pending/rejected/deleted 均不返回。
- 不返回用户资料权威字段的本地副本。

### 查询好友申请

`GET /friends/requests`

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- 返回 `incoming` 和 `outgoing` 两组 pending 申请。
- `incoming` 中的 `friend_id` 是申请发起方 account id alias。
- `outgoing` 中的 `friend_id` 是申请接收方 account id alias。

### 查询好友关系

`GET /friends/{user_id}`

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- 当前用户和目标用户不存在时返回明确不存在错误。
- 存在有效好友关系时返回 `status=accepted` 和 `is_friend=true`。
- pending、rejected、deleted 或从未添加时返回对应非好友状态。

## 依赖关系

- `friends` 调用 `user-rpc.GetUserByID` 校验当前用户和目标用户存在。
- `friends` 不反向写入 `user`，也不复制 `user` 资料作为权威数据。
- `friends-api` 通过统一 JWT 鉴权中间件读取 token `user_id`，不接受 `X-User-Id` 作为生产身份来源。

## 后续扩展

- 好友备注、标签、分组。
- 黑名单和拉黑后的添加/消息策略。
- PostgreSQL 持久化、唯一索引和迁移脚本。
- 好友资料展示可按需调用 `user-rpc` 聚合公开资料，但不在 friends 中保存权威资料副本。

## 风险与待决

- 所有需要当前用户身份的接口必须使用 JWT Bearer token；`X-User-Id` 只允许作为明确标记的测试绕过断言或历史兼容说明。
- 第一阶段使用内存 repository 支撑本地开发和测试，进程重启后数据会丢失。
- 重复 pending 添加当前按幂等成功处理；已拒绝申请后再次添加会创建新的 pending 申请。
