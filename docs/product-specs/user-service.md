# Account Service 第一阶段产品规格

状态：Draft

## 背景

Account Service 是账号资料的权威服务，先于 `auth`、`friends`、`groups` 开发。Account 可代表 human user、agent、admin，未来可扩展 service/official accounts。第一阶段目标是稳定唯一标识、公开资料、自身资料查询和自身资料更新能力，让 `auth` 注册流程可以依赖 V0 `user-rpc` transport 完成账号存在性检查和资料初始化。

本文件路径保留 `user-service.md`，是 V0 documentation compatibility；服务和领域语义以 Account Service 为准。

## 目标

- 提供账号基础资料创建能力。
- 保证唯一标识符 `identifier` 全局唯一。
- 提供按唯一标识符查询账号是否存在的能力。
- 提供按唯一标识符查询公开资料的能力。
- 提供按 `user_id` 查询账号资料的 RPC 能力，其中 `user_id` 是 account id alias。
- 提供 `/me` 查询当前账号资料的 HTTP 能力。
- 提供当前账号更新自己资料字段的 HTTP 能力。
- 支持 `account_type=user|agent|admin`。

## 非目标

- 不保存 `password`、`password_hash`、验证码、OAuth token、微信扫码凭据等认证秘密。
- 不验证密码、不签发 token、不实现登录注册流程。
- 不维护好友关系、好友申请、黑名单或群成员关系。
- 不在第一阶段实现手机号、邮箱、第三方账号等多 identifier 绑定。
- 不批量把 public JSON 字段从 `user_id` 改为 `account_id`。

## 账号资料字段

第一阶段账号资料包含：

- `account_id`：系统生成的 Snowflake account id，无前缀数字字符串，是内部 source of truth。
- `user_id`：V0 compatibility alias，值等于 `account_id`。
- `identifier`：账号唯一标识，供注册检查和公开查询使用。
- `display_name`：展示名。
- `name`：名称字段，第一阶段与 `display_name` 等价保留，便于客户端兼容。
- `gender`：性别，支持 `unknown`、`male`、`female`、`other`。
- `birth_date`：生日，允许未设置；不落库存储会随时间变化的年龄。
- `region`：地区，允许未设置。
- `account_type`：账号类型，支持 `user`、`agent`、`admin`；公开 HTTP 注册/创建路径默认并固定为 `user`，内部 User RPC/logic 可显式创建 `agent` 或 `admin`。
- `avatar_media_id`：当前头像绑定的 media id，允许为空。头像文件本身由 Media API 上传到 RustFS (S3-compatible) object storage，用户资料只保存 ready media 的引用。
- `created_at` / `updated_at`：资料创建和更新时间。

旧 `account_type=normal` 不再作为有效输入兼容；迁移前必须转换为 `user`，否则按非法 `account_type` 失败。
PostgreSQL 存储拆分为 `accounts` 与 `profiles`：`accounts` 负责 account id、identifier、account_type 和账号时间戳，`profiles` 负责展示资料和头像引用。创建账号必须同时创建 account 与 profile。

## 接口能力

### 创建账号资料

V0 path：`POST /users`

Account alias：`POST /accounts`

请求方通常是 `auth` 注册流程或内部管理流程。请求必须包含 `identifier`，可选 `display_name`、`name`、`gender`、`birth_date`、`region`。HTTP `POST /users` 与 `POST /accounts` 不接受客户端设置 `account_type`，即使请求体包含该字段也按 `user` 创建；需要创建 `agent` 或 `admin` 时必须走内部 User RPC/logic 能力，并通过服务端权限策略保护调用方。

验收标准：

- `identifier` 为空或格式非法时返回明确参数错误。
- 重复 `identifier` 时返回明确冲突错误。
- 非法内部 `account_type` 返回明确参数错误，错误信息包含 `account_type`。
- 成功创建后返回完整账号资料。
- 返回内容不包含任何密码或认证秘密字段。

### 查询账号是否存在

V0 path：`GET /users/exists?identifier=...`

Account alias：`GET /accounts/exists?identifier=...`

验收标准：

- 返回 `exists=true|false`。
- 为空或格式非法的 `identifier` 返回参数错误。
- 该接口面向 `auth` 注册前检查，不暴露认证信息。

### 查询公开资料

V0 path：`GET /users/:identifier`

Account alias：`GET /accounts/:identifier`

验收标准：

- 存在时返回公开资料。
- 不存在时返回明确不存在错误。
- 不返回密码、认证秘密、好友关系或群成员关系。

### 查询自己的资料

`GET /me`

客户端必须携带 `Authorization: Bearer <access_token>`。服务通过统一 JWT 鉴权中间件从 token `user_id` claim 获取当前 account id。

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- 账号不存在时返回明确不存在错误。
- 成功时返回当前账号资料。

### 更新自己的资料

`PATCH /me`

通过 token `user_id` 确认当前账号，只允许更新自己的 `display_name`/`name`、`gender`、`birth_date`、`region`。

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- 不允许更新 `user_id`、`identifier`、创建时间、认证字段。
- 参数非法时返回明确参数错误。
- 成功后 `/me` 返回最新资料。

### 更新自己的头像

`PATCH /me/avatar`

请求：

```json
{
  "mediaId": "med_000001"
}
```

通过 token `user_id` 确认当前用户。服务必须验证 media 对象归当前用户所有、`purpose=avatar`、`status=ready`、MIME 类型为允许的图片类型且大小不超过 5 MiB，然后把 `avatar_media_id` 写入用户资料。

验收标准：

- 缺少、过期或非法 token 时返回未认证错误。
- media 不存在时返回明确不存在错误。
- media 不属于当前用户时返回禁止访问错误。
- media 不是 avatar purpose 或尚未 ready 时返回明确参数错误。
- 成功后 `/me` 返回新的 `avatar_media_id`。

## 依赖关系

- `auth` 注册流程依赖 V0 `user-rpc` 的 `ExistsByIdentifier` 和 `CreateUser`。
- `friends` 和 `groups` 后续可依赖 V0 `user-rpc` 的 `GetUserByID`、`GetUserByIdentifier` 做账号存在性校验和公开资料展示。
- Account Service 不反向依赖 `auth`、`friends`、`groups`。

## 风险与待决

- `identifier` 是否允许修改、是否大小写敏感，后续需要产品确认。第一阶段按小写规范化后唯一处理。
- 所有需要当前账号身份的接口必须使用 JWT Bearer token；`X-User-Id` 只允许作为明确标记的测试绕过断言或历史兼容说明。
- 第一阶段可使用内存 repository 支撑本地开发和测试；共享本地开发使用 PostgreSQL repository。
- PostgreSQL `accounts` / `profiles` 是账号资料权威存储；旧 `users` 仅保留为 transport/path/field 命名兼容概念，不再作为表名。
