# Friends Service 第一阶段产品规格

状态：Draft

## 背景

`friends` 是好友关系的权威服务，依赖 Account Service 的账号存在性能力，但不保存账号资料权威数据，不管理登录认证，也不维护群成员关系。第一阶段目标是在 Account Service 基础接口稳定后，提供直接添加好友、删除好友、查询好友列表和查询好友关系能力，为后续 IM 会话和社交展示提供关系基础。V0 `user_id` / `friend_id` 字段是 account id alias。

## 目标

- 提供添加好友能力。
- 提供删除好友能力。
- 提供当前账号好友列表查询能力。
- 提供当前账号与指定账号的好友关系查询能力。
- 添加好友前校验发起方和目标账号都存在。
- 第一阶段直接双向建立好友关系，不引入好友申请审批流。
- 通过 JWT Bearer token 解析当前用户身份。

## 非目标

- 不保存 `display_name`、`identifier`、头像等账号资料权威字段。
- 不保存密码、验证码、OAuth token 等认证秘密。
- 不验证登录密码、不签发 token、不实现登录注册。
- 不维护群成员关系。
- 不实现好友申请审批、好友备注、好友分组、黑名单。
- 不在第一阶段实现 PostgreSQL 持久化。

## 数据概念

第一阶段好友关系包含：

- `user_id`：当前 account id alias。
- `friend_id`：好友 account id alias。
- `status`：关系状态，第一阶段支持 `active`、`deleted`、`none`。
- `is_friend`：是否为有效好友关系。
- `created_at` / `updated_at`：关系创建和更新时间。

关系语义：

- 添加好友成功后，系统直接写入双向 `active` 关系。
- 删除好友后，双向关系失效，列表不再返回该好友。
- 查询关系时，`active` 表示当前有效好友，`deleted` 或 `none` 表示当前不是好友。

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
- 首次添加后双方好友列表都可见该关系。
- 重复添加同一好友保持幂等，返回明确 `created=false` 结果。

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
- 只返回当前有效 `active` 好友关系。
- 不返回用户资料权威字段的本地副本。

### 查询好友关系

`GET /friends/{user_id}`

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- 当前用户和目标用户不存在时返回明确不存在错误。
- 存在有效好友关系时返回 `status=active` 和 `is_friend=true`。
- 删除或从未添加时返回非好友状态。

## 依赖关系

- `friends` 调用 `user-rpc.GetUserByID` 校验当前用户和目标用户存在。
- `friends` 不反向写入 `user`，也不复制 `user` 资料作为权威数据。
- `friends-api` 通过统一 JWT 鉴权中间件读取 token `user_id`，不接受 `X-User-Id` 作为生产身份来源。

## 后续扩展

- 好友申请审批：`pending`、`accepted`、`rejected` 状态和申请记录。
- 好友备注、标签、分组。
- 黑名单和拉黑后的添加/消息策略。
- PostgreSQL 持久化、唯一索引和迁移脚本。
- 好友资料展示可按需调用 `user-rpc` 聚合公开资料，但不在 friends 中保存权威资料副本。

## 风险与待决

- 所有需要当前用户身份的接口必须使用 JWT Bearer token；`X-User-Id` 只允许作为明确标记的测试绕过断言或历史兼容说明。
- 第一阶段使用内存 repository 支撑本地开发和测试，进程重启后数据会丢失。
- 重复添加当前按幂等成功处理；后续引入申请审批后，需要重新定义重复申请与已拒绝申请的行为。
