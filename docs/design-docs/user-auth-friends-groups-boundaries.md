# Account/Auth/Friends/Groups 微服务边界

状态：Draft

## 背景

IM 系统需要将账号资料、认证、好友关系和群聊关系拆分为独立微服务，避免单个 IM Core Service 承担过多职责。产品语义已经从 `user service` 收敛为 **Account Service**：Account 是身份与资料主体，可代表 human user、agent、admin，未来可扩展 service/official accounts。历史 `user` path 和 `user_id` 字段在第一阶段仅作为 V0 compatibility 保留。

## 目标

- 明确 Account、auth、friends、groups 的职责边界。
- 明确 `account_type=user|agent|admin`，其中 `user` 是 account type，不是服务名。
- 明确 friends/groups/message 中的 `user_id` 当前是 account id alias。
- 为后续接口设计、数据库设计和并行开发提供依据。
- 保证密码、验证码、第三方登录凭据等认证秘密只归 `auth` 管理。

## 非目标

- 本文不定义完整数据库表结构。
- 本文不实现手机号验证码、微信扫码等扩展认证方式。
- 本文定义基础好友申请审批；不定义好友通知、备注、黑名单和完整群权限模型。
- 本文不批量把 public JSON 字段从 `user_id` 改成 `account_id`。

## 服务边界

### Account Service

职责：

- 记录账号资料信息。
- 管理账号唯一标识，类似微信号。
- 管理名称、性别、年龄、地区等账号资料字段。
- 管理 `account_type=user|agent|admin`。
- 提供 `/me`：根据 token 查询当前登录账号自己的资料。
- 提供账号公开资料查询能力。
- 提供根据唯一标识符查询账号是否存在的能力。
- 为 `auth` 注册流程提供账号存在性检查和账号资料创建能力。

V0 compatibility：

- 当前 public REST 继续保留 `/users/*`，并提供 `/accounts/*` aliases。
- 当前 JSON/RPC 字段 `user_id` 是 account id alias。
- 当前 PostgreSQL 使用 `accounts` / `profiles` 作为账号资料权威存储。

边界：

- 不保存密码。
- 不验证密码。
- 不保存短信验证码、微信扫码凭据、OAuth token 等认证秘密。
- 不维护好友关系。
- 不维护群成员关系。

### auth service

职责：

- 管理认证身份、密码哈希和认证秘密。
- 第一阶段实现账号密码注册和登录。
- 注册时调用 Account Service 检查账号是否存在。
- 注册成功时调用 Account Service 创建或初始化账号资料。
- 签发客户端后续请求使用的 token。
- 为未来手机号验证码、微信扫码等登录方式预留扩展点。

边界：

- 不维护账号展示资料。
- 不维护好友关系。
- 不维护群成员关系。
- 第一阶段不实现手机号验证码、微信扫码登录。

### friends service

职责：

- 维护好友关系。
- 支持添加好友、删除好友。
- 支持查询好友列表和好友关系状态。
- 添加好友先形成 `pending` 申请，由接收方接受后形成双向 `accepted` 关系。
- 重复添加同一 pending 申请保持幂等成功；添加自己或添加不存在账号必须失败。
- 删除后关系进入非好友状态，列表只返回当前 `accepted` 好友，关系查询可返回 `pending`、`accepted`、`rejected`、`deleted` 或 `none`；历史 `active` 仅作迁移兼容读取。

边界：

- 不保存账号资料副本，除非是为了性能的非权威缓存。
- 不管理登录认证。
- 不维护群成员关系。
- 不写入 Account Service 的权威资料字段。
- V0 `POST /friends {"user_id": ...}`、`DELETE /friends/{user_id}`、`GET /friends/{user_id}` 中的 `user_id` 均是目标 account id alias。

### groups service

职责：

- 维护群聊基础信息。
- 维护群成员关系。
- 支持加群、退群。
- 支持查询群与群成员信息。
- MVP 中创建者写入 `creator_user_id`，该字段是 account id alias；创建者视为 owner，并自动成为 active 成员。
- MVP 群默认 open join，账号可直接加入公开群；重复加入保持幂等成功。
- 添加其他账号入群当前仅允许 creator/owner；完整邀请、审批和管理员角色后续补齐。
- 查询群详情和群成员列表要求请求者是 active 群成员。
- 成员可退出；若 owner 是唯一 active 成员，则拒绝退出，不做群解散或 owner 转让。
- 群消息发送前，Message Service 必须依赖 groups 的 active 成员列表校验发送者成员身份。

边界：

- 不保存账号资料权威数据。
- 不管理登录认证。
- 不维护好友关系。
- 不写入 Account Service 的权威资料字段。
- V0 `creator_user_id`、`user_id`、`operator_user_id` 均是 account id alias。

## 依赖关系

```text
client
  ├── auth service
  │     └── Account Service
  ├── Account Service
  ├── friends service
  │     └── Account Service
  └── groups service
        └── Account Service
```

说明：

- `auth` 依赖 Account Service：注册时需要查询账号是否存在，并创建账号资料。
- `friends` 可依赖 Account Service：添加好友时可校验目标账号是否存在，展示好友资料时可查询公开资料。
- `groups` 可依赖 Account Service：加群时可校验账号是否存在，展示群成员时可查询公开资料。
- Account Service 不反向依赖 `auth`、`friends`、`groups`。

## 第一阶段建议接口能力

### Account Service

- `GET /me`：根据 token 查询当前账号资料。
- `GET /users/{identifier}`：V0 兼容路径，按唯一标识符查询公开账号资料。
- `GET /accounts/{identifier}`：Account alias，语义同 `/users/{identifier}`。
- `GET /users/exists?identifier=...`：V0 兼容路径，查询唯一标识符是否存在。
- `GET /accounts/exists?identifier=...`：Account alias，语义同 `/users/exists`。
- `POST /users`：V0 兼容路径，创建账号基础资料，主要供 `auth` 注册流程调用。
- `POST /accounts`：Account alias，语义同 `/users`。
- `PATCH /me`：登录后完善或更新自己的资料，例如名称、性别、年龄、地区。

### auth service

- `POST /auth/register`：账号密码注册。
- `POST /auth/login`：账号密码登录。
- 后续预留但暂不实现：手机号验证码、微信扫码登录。

### friends service

- `POST /friends`：添加好友或发起好友添加流程，请求体 `user_id` 是目标 account id alias。
- `GET /friends/requests`：查询当前账号 incoming/outgoing pending 好友申请。
- `POST /friends/{user_id}/accept`：接受指定账号发起的 pending 好友申请。
- `POST /friends/{user_id}/reject`：拒绝指定账号发起的 pending 好友申请。
- `DELETE /friends/{user_id}`：删除好友，path `user_id` 是目标 account id alias。
- `GET /friends`：查询好友列表。
- `GET /friends/{user_id}`：查询与指定账号的好友关系。

### groups service

- `POST /groups`：创建群聊。
- `POST /groups/{group_id}/members`：加入群聊或添加群成员，请求体 `user_id` 是目标 account id alias。
- `DELETE /groups/{group_id}/members/me`：退出群聊。
- `GET /groups/{group_id}/members`：查询群成员。

> 接口路径为第一阶段设计占位，最终以实际 go-zero `.api` / gRPC proto 为准。

## MVP 业务规则

### 好友关系

1. `AddFriend` 校验发起方和目标账号存在，且二者不同。
2. 校验通过后写入 `requester -> recipient` 的 `pending` 申请，不写入 accepted 双向好友。
3. 已是 `pending` 申请或 `accepted` 好友时重复添加返回幂等成功，不创建重复记录。
4. `AcceptFriend` 只允许 pending 申请接收方调用，成功后写入双向 `accepted` 关系。
5. `RejectFriend` 只允许 pending 申请接收方调用，成功后写入 `rejected` 状态。
6. `DeleteFriend` 使双向 accepted 关系失效。
7. `ListFriends` 只返回 `accepted` 关系。
8. `GetFriendship` 对有效关系返回 `accepted`，对 pending/rejected/deleted/none 返回非好友状态。

### 群成员与群消息

1. `CreateGroup` 校验创建者账号存在，写入群基础信息，并将创建者作为 active 成员。
2. `JoinGroup` 允许存在账号直接加入公开群；`AddMember` 添加其他账号时要求操作者是 creator/owner。
3. 重复加入 active 群成员返回幂等成功，不创建重复 active 成员。
4. `LeaveGroup` 使成员状态变为 `left`；owner 是唯一 active 成员时拒绝退出。
5. `GetGroup` / `ListMembers` 对外请求要求请求者是 active 成员；`ListMembers` 只返回 active 成员。
6. Message Service 发送 group 消息前必须查询 active 成员；发送者不在 active 成员列表时返回 `FORBIDDEN`，不能写入消息、推进 seq 或创建 outbox。

## 关键数据流

### 注册

1. 客户端请求 `auth` 注册。
2. `auth` 调用 Account Service 查询唯一标识符是否已存在。
3. 若存在，`auth` 返回注册失败。
4. 若不存在，`auth` 写入密码哈希等认证数据。
5. `auth` 调用 Account Service 创建账号基础资料。
6. `auth` 返回注册成功或签发 token。

### 查询自己资料

1. 客户端携带 token 请求 Account Service 的 `/me`。
2. Account Service 根据 token 中的 account id 查询资料。
3. Account Service 返回当前账号资料，不包含密码或认证秘密。

### 完善资料

1. 客户端携带 token 请求 `PATCH /me`。
2. Account Service 校验请求身份只能修改自己。
3. Account Service 更新名称、性别、年龄、地区等资料字段。

### 群消息发送成员校验

1. 客户端携带 token 通过 Gateway 或 Message API 发起 group 消息发送。
2. Message Service 根据 `group_id` 查询 groups active 成员列表。
3. 若发送者不是 active 成员，Message Service 返回 `FORBIDDEN`，不写入消息。
4. 若发送者是 active 成员，Message Service 写入群会话消息，并仅向 active 成员建立可见会话状态。

## 开发计划约束

推荐顺序：

1. Account Service：先实现唯一标识、资料模型、存在性查询、`/me`、公开资料查询、资料更新。
2. `auth`：在 Account Service 接口稳定后实现账号密码注册/登录，并调用 Account Service 完成存在性检查和资料初始化。
3. `friends`：可与 `auth` 并行，在需要时调用 Account Service 校验账号存在性。
4. `groups`：可与 `auth` 和 `friends` 并行，在需要时调用 Account Service 校验成员存在性。

## 风险与待决问题

- token 是由 `auth` 签发后由各服务本地校验，还是通过网关统一鉴权后透传 account id，需要后续设计确认。
- Account Service 创建资料与 `auth` 写入密码哈希之间需要处理一致性问题，例如补偿、事务外盒或幂等创建。
- 唯一标识符的命名规则、可修改性、大小写敏感性需要产品确认。
- 公开资料与仅本人可见资料的字段边界需要补充隐私策略。
- 未来若新增 `account_id` public 字段，必须先建立双字段兼容契约和测试，再迁移前端。

## 验证方式

- 注册重复唯一标识符时必须失败。
- Account Service 数据库中不应出现密码或认证秘密字段。
- `/me` 只能返回当前登录账号的信息。
- 查询公开账号资料不泄露认证数据。
- 好友和群成员关系分别由 `friends` 与 `groups` 管理，不能写入 Account Service 的权威数据模型。
- 添加好友先 pending，只有接收方可接受/拒绝；接受后双方 accepted，重复 pending 添加幂等，self-add 和 missing target 均失败。
- 创建群后 creator 是 owner/member；open join 成功；非成员或已退出成员发送 group message 失败，active member 发送成功。
