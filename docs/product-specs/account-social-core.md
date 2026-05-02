# 账号、资料、好友与群聊基础能力

状态：Draft

## 业务目标

系统需要先建立 IM 的账号与社交关系基础能力，为后续消息、会话、Agent 参与聊天等能力提供稳定身份、好友关系和群成员关系。第一阶段按服务依赖顺序推进：先稳定 **Account Service**，再开发依赖 Account Service 的 `auth`，同时 `friends` 与 `groups` 可与 `auth` 并行开发。

Account 是身份与资料主体，可代表 human user、agent、admin，未来可扩展 service/official accounts。历史 `user_id` public 字段在第一阶段是 account id alias。

## 服务职责

### Account Service

Account Service 负责记录账号资料信息，但不负责密码、验证码、第三方登录凭据等认证秘密。

核心职责：

- 维护账号唯一标识，类似微信号，用于查询和对外展示。
- 维护账号名称等基础资料信息。
- 维护 `account_type=user|agent|admin`；这里的 `user` 是 account type，不是服务名。
- 提供 `/me` 接口：登录账号可以根据 token 查询自己的资料。
- 支持查询其他账号公开资料。
- 支持根据唯一标识符查询账号是否存在。
- 支持登录后完善个人资料，例如性别、年龄、地区等。
- 为 `auth` 创建账号流程提供账号存在性检查能力。

V0 compatibility：

- Public REST 继续保留 `/users/*`，并提供 `/accounts/*` aliases。
- Public JSON 的 `user_id` 继续保留，语义是 account id alias。
- 数据表使用 `accounts` + `profiles` 作为账号资料权威存储；`users` 仅是 V0 path/field compatibility 命名。

明确不负责：

- 不保存用户密码。
- 不验证账号密码。
- 不管理短信验证码、微信扫码等认证流程。
- 不维护好友关系和群成员关系。

### auth 微服务

`auth` 微服务负责认证与登录注册流程。第一阶段只实现账号密码登录/注册，后续预留手机号验证码、微信扫码等扩展方式，但现在不实现。

核心职责：

- 负责密码信息与认证秘密的存储和验证。
- 支持账号密码注册。
- 支持账号密码登录。
- 注册创建账号时，必须先调用 Account Service 查询账号是否已存在。
- 注册成功时，与 Account Service 协作创建或初始化账号资料。
- 未来扩展手机号验证码、微信扫码登录等认证方式。

### friends 微服务

`friends` 微服务负责维护好友关系。

核心职责：

- 添加好友。
- 删除好友。
- 查询好友关系和好友列表。
- 后续可扩展好友申请、备注、黑名单等能力。

MVP 语义：

- `AddFriend` 不走审批流，发起后立即创建双向 `active` 好友关系。
- 重复添加同一有效好友是幂等成功，返回已存在关系，不创建重复关系。
- 添加自己为好友必须拒绝。
- 目标账号不存在时必须拒绝；好友关系不写入 Account Service 的账号资料字段。
- `DeleteFriend` 使双向关系失效；`ListFriends` 只返回当前 `active` 好友；关系查询对从未添加返回 `none`，对已删除返回 `deleted`。
- V0 `user_id` / `friend_id` 字段指向 account id。

### groups 微服务

`groups` 微服务负责维护群聊与群成员关系。

核心职责：

- 创建和维护群聊基础信息。
- 加入群聊。
- 退出群聊。
- 查询群成员关系。
- 后续可扩展群角色、邀请、审批、禁言等能力。

MVP 语义：

- 创建群后，创建者记录为 `creator_user_id`，该字段是 account id alias，并自动成为群成员；MVP 将创建者视为群 owner。
- 群默认 open join，存在账号可直接加入公开群，重复加入幂等返回已在群内。
- 添加其他账号入群当前仅允许 creator/owner；完整邀请、审批和管理员角色后续扩展。
- 成员可退出群；若 owner 是唯一 active 成员，MVP 不做解散或转让，直接拒绝退出。
- 查询群详情和 `ListMembers` 要求请求者是 active 成员；`ListMembers` 只返回当前 `active` 成员，已退出成员不再返回。
- 群消息发送必须由 Message Service 校验发送者是当前 `active` 群成员；非成员、已退出成员或缺少群成员校验配置时不得成功写入群消息。
- 群成员关系不写入 Account Service 的账号资料字段。

## 核心场景

### 查询自己的资料

1. 账号携带 token 请求 Account Service 的 `/me`。
2. 系统根据 token 识别当前账号。
3. Account Service 返回当前账号资料信息。

验收标准：

- 登录账号能查询自己的唯一标识、名称、`account_type` 和已完善资料。
- 未登录或 token 无效时不能查询 `/me`。

### 查询其他账号资料

1. 用户输入或选择另一个账号的唯一标识。
2. 客户端请求 Account Service 查询目标账号。
3. Account Service 返回允许公开展示的账号资料。

验收标准：

- 可以根据唯一标识符查到存在的账号。
- 不存在的账号返回明确的不存在结果。
- 不泄露密码、认证秘密等 auth 数据。

### 注册账号

1. 客户端提交账号、密码等注册信息到 `auth`。
2. `auth` 调用 Account Service 查询唯一标识符是否已存在。
3. 如果账号已存在，注册失败。
4. 如果账号不存在，`auth` 保存密码认证信息，并与 Account Service 协作创建账号基础资料。
5. 注册成功后返回登录态或注册成功结果。

验收标准：

- 重复唯一标识符不能注册。
- 密码只由 `auth` 管理，不进入 Account Service 数据模型。
- 注册成功后，`/me` 可以查询到新账号资料。
- 公开注册默认创建 `account_type=user`，不能自选 `agent` 或 `admin`。

### 登录后完善资料

1. 用户登录成功后进入资料完善流程。
2. 用户更新名称、性别、年龄、地区等资料。
3. Account Service 保存资料更新。

验收标准：

- 用户只能修改自己的资料。
- 性别、年龄、地区等字段可逐步完善。
- 资料更新后 `/me` 返回最新信息。

### 好友关系维护

1. 用户发起添加好友。
2. MVP 中 `friends` 立即写入双向 `active` 好友关系，不创建待审批申请。
3. 用户可以删除好友。

验收标准：

- 添加好友后能查询到好友关系。
- 重复添加同一有效好友保持幂等成功。
- 添加自己或添加不存在账号必须失败。
- 删除好友后关系失效。
- 好友列表只返回当前 `active` 关系；关系查询能区分 `active`、`deleted` 与 `none`。
- 好友关系不写入 Account Service。

### 群聊成员关系维护

1. 用户创建或加入群聊。
2. `groups` 维护群基础信息与成员关系。
3. 用户可以退出群聊。
4. 用户发送群消息前，`message` 必须校验发送者仍是群 active 成员。

验收标准：

- 创建群后创建者是 owner 标识账号，也是 active 成员。
- MVP 群默认允许公开加入，重复加入不创建重复 active 成员。
- 加群后能查询到群成员关系。
- 非成员不能查询群详情或成员列表。
- 非 owner 不能添加其他账号入群。
- 退群后成员关系失效。
- owner 是唯一 active 成员时不能退出；MVP 不实现群解散或群主转让。
- 非成员或已退出成员发送群消息必须失败，active 成员发送群消息可以成功。
- 群成员关系不写入 Account Service。

## 开发顺序

第一阶段开发顺序：

```text
Account Service -> auth
                -> friends
                -> groups
```

约束：

- 必须先开发 Account Service，因为 `auth` 注册流程依赖账号存在性查询和账号资料创建能力。
- `auth`、`friends`、`groups` 在 Account Service 的基础接口稳定后可以并行开发。
- `friends` 和 `groups` 可以依赖 Account Service 的账号存在性与账号公开资料查询，但不应直接管理账号资料。
- 第一阶段不批量修改 `user_id` public 字段；文档和实现必须说明它是 account id alias。

## 当前不实现

- 手机号验证码登录/注册。
- 微信扫码登录。
- 好友申请审批细节、备注、黑名单。
- 群权限、邀请审批、群管理员、群主转让、群解散、禁言。
- 密码找回、多因素认证等高级 auth 能力。
- 全量 `account_id` public 字段迁移。
