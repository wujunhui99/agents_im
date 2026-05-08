# Groups Service 第一阶段产品规格

状态：Draft

## 背景

`groups` 是群聊基础信息与群成员关系的权威服务。第一阶段目标是提供创建群、查询群、加入或添加群成员、退出群聊、查询群成员列表等基础能力，为后续 IM Core 的群聊消息投递和 Agent 参与群聊提供稳定的群与成员关系。

`groups` 依赖 Account Service 的账号存在性能力，但不保存账号资料权威数据，不管理登录认证，不维护好友关系。V0 `creator_user_id`、`operator_user_id`、成员 `user_id` 字段是 account id alias。

## 目标

- 提供群聊基础信息创建能力。
- 创建群时创建者自动成为群成员。
- 提供按 `group_id` 查询群基础信息的能力。
- 提供加入群或添加群成员能力。
- 加群或添加成员前校验目标账号存在。
- 重复加群返回明确结果且不创建重复有效成员关系。
- 提供退出群聊能力，退出后成员关系失效。
- 提供查询群成员列表能力。
- 提供群管理 V1：active 成员可查看群名称、头像占位、公告和成员；群主/管理员可编辑群名称/公告并按角色约束踢人。
- HTTP 当前用户身份通过 JWT Bearer token 解析。

## 非目标

- 不保存账号资料权威数据，例如名称、头像、性别、地区等。
- 不管理登录认证、密码、token 签发或 token 校验。
- 不维护好友关系或好友申请。
- 第一阶段不实现群主转让、邀请、审批、禁言、群昵称和群头像上传。
- 第一阶段不实现 PostgreSQL 持久化，先使用内存 repository 并通过接口隔离。

## 群聊数据

第一阶段群基础信息包含：

- `group_id`：系统生成的群 ID。
- `name`：群名称。
- `description`：群描述，允许为空。
- `announcement`：前端展示字段，V1 由 `description` 持久化。
- `creator_user_id`：创建者 account id alias。
- `created_at` / `updated_at`：创建和更新时间。

第一阶段群成员关系包含：

- `group_id`：群 ID。
- `user_id`：成员 account id alias。
- `role`：成员角色，使用 `owner`、`admin`、`member`。
- `state`：成员状态，第一阶段使用 `active` 和 `left`。
- `joined_at`：最近一次加入时间。
- `left_at`：最近一次退出时间，未退出时为空。

## 接口能力

### 创建群

`POST /groups`

请求必须携带 `Authorization: Bearer <access_token>`，请求体必须包含 `name`，可选 `description`。

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- 创建者不存在时返回明确不存在错误。
- `name` 为空或格式非法时返回明确参数错误。
- 成功创建后返回群基础信息。
- 创建者自动成为 `active` 成员。

### 查询群

`GET /groups/{group_id}`

请求必须携带 `Authorization: Bearer <access_token>`，且 token 用户必须是该群 active 成员。

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- 当前用户不是 active 群成员时返回禁止错误。
- 群存在时返回群基础信息。
- 群不存在时返回明确不存在错误。
- 返回内容不包含用户资料权威字段、密码或认证秘密。

### 更新群资料

`PATCH /groups/{group_id}`

请求必须携带 `Authorization: Bearer <access_token>`。仅群主或管理员可更新 `name` 和 `announcement`。

验收标准：

- 普通成员更新返回禁止错误。
- 非成员更新返回禁止错误。
- 群主或管理员更新成功后返回最新群资料。
- `announcement` 写入现有 `description` 存储字段。

### 加入群或添加成员

`POST /groups/{group_id}/members`

请求必须携带 `Authorization: Bearer <access_token>`。请求体可选 `user_id`：为空或等于当前 token 用户时表示当前用户加入群；不为空且不同于当前 token 用户时表示添加指定用户。第一阶段尚未实现完整管理员权限模型，添加其他用户暂时仅允许群创建者/owner 操作。

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- 群不存在时返回明确不存在错误。
- 非 owner 添加其他用户时返回禁止错误。
- 目标用户不存在时返回明确不存在错误。
- 首次加入成功后成员状态为 `active`。
- 重复加入保持幂等，返回 `already_member=true`，不创建重复有效成员关系。
- 退出后再次加入会重新激活成员关系并刷新加入时间。

### 退出群

`DELETE /groups/{group_id}/members/me`

请求必须携带 `Authorization: Bearer <access_token>`。

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- 群不存在时返回明确不存在错误。
- 当前用户不是有效成员时返回明确不存在错误。
- 成功退出后该成员关系变为失效，后续成员列表不再返回该成员。

### 查询群成员

`GET /groups/{group_id}/members`

请求必须携带 `Authorization: Bearer <access_token>`，且 token 用户必须是该群 active 成员。

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- 当前用户不是 active 群成员时返回禁止错误。
- 群不存在时返回明确不存在错误。
- 返回当前 `active` 成员列表。
- 已退出成员不出现在列表中。
- 返回成员关系信息、角色和可展示资料快照，不返回密码或认证秘密。

### 踢出成员

`DELETE /groups/{group_id}/members/{user_id}`

请求必须携带 `Authorization: Bearer <access_token>`。群主或管理员可踢出符合角色约束的 active 成员。

验收标准：

- 普通成员踢人返回禁止错误。
- 非成员踢人返回禁止错误。
- 群主不能被踢出。
- 管理员不能踢出群主或其他管理员。
- 被踢出的成员变为 inactive，后续成员列表不再返回。

## 依赖关系

- `groups` 依赖 `user-rpc` 的用户存在性能力。第一阶段按 `GetUserByID(user_id)` 语义校验成员是否存在。
- `groups` 不反向修改 `user` 资料，也不缓存用户资料为权威数据。
- `IM Core` 后续可依赖 `groups-rpc` 判断群成员关系并做群消息投递。

## 风险与待决

- 所有需要当前用户身份的接口必须使用 JWT Bearer token；`X-User-Id` 只允许作为明确标记的测试绕过断言或历史兼容说明。
- 第一阶段未实现管理员任命或审批，`POST /groups/{group_id}/members` 的添加指定成员能力当前只允许 creator/owner，后续需要补角色管理和邀请审批模型。
- 第一阶段使用内存 repository 支撑本地开发和测试；生产化需要替换 PostgreSQL，并补充唯一约束和成员状态索引。
- 重复加群第一阶段按幂等处理并显式返回 `already_member`，如客户端需要冲突语义，可在后续版本调整。
