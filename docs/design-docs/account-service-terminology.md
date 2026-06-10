# Account Service Terminology

状态：Accepted

## 背景

IM 身份主体不只包含人类用户。Agent、admin、未来服务号/公众号都需要登录、持有资料并参与消息链路，因此账号资料领域统一命名为 **Account**。历史 `user` 命名在第一阶段只作为 V0 transport/API compatibility 保留；这也是本文的 V0 compatibility 边界。

## 术语

- Account：身份与资料主体，可代表 human user、agent、admin，未来可扩展 service/official account。
- Account Service：账号资料权威服务，负责 identifier、展示名、性别、生日、地区、`account_type` 等资料，不负责 credential/password/token。
- Auth Service：认证与凭据边界，负责 password hash、salt、token 签发与校验，不负责账号展示资料。
- `account_type`：账号类型，当前支持 `user`、`agent`、`admin`、`test`。`user` 是 account type，不是服务名。
- `account_id`：Account Service 内部 source-of-truth ID，由 Snowflake 算法生成，保存为无前缀数字字符串。
- `user_id`：V0 public compatibility 字段，是 account id alias。friends/groups/message/gateway/read state 中的 `user_id` 均指向 Account Service 管理的 account id。

## 账户类型（account_type）

`accounts.account_type` 为 smallint，整型取值的单一来源是 `service/user/rpc/internal/model/vars.go`：

| 字符串 | DB 值 | 说明 | 创建途径 | 邮箱 | 登录凭据 |
|--------|------|------|---------|------|---------|
| `admin` | 0 | 管理后台管理员，过 admin-api 的 admin 闸 | auth-rpc 启动时由 `ADMIN_BOOTSTRAP_*` secret 引导 | 不绑定 | bootstrap 时写入 |
| `user` | 1 | 普通用户（默认） | 注册流程（强制邮箱验证码绑定） | 必须绑定且验证 | 注册时写入 |
| `agent` | 2 | AI Agent 账号（如默认助手） | 系统开通（default assistant provisioner 等） | 不绑定 | 无（不可密码登录） |
| `test` | 3 | 测试账户，行为与 `user` 一致（含默认助手开通） | admin 管理后台 `POST /admin/test-accounts`（BFF 编排 user-rpc 建号 + auth-rpc 设凭据） | 不绑定 | 创建时写入；同名重复创建 = 重置密码 |

测试账户要点：

- 仅 admin 可创建（admin-api DeviceAuth + admin 闸）；密码可指定，缺省自动生成并在响应中一次性返回。
- user-rpc `CreateTestAccount` 幂等：identifier 已存在且为 `test` 时返回该账户；为其它类型时报 AlreadyExists。
- auth-rpc `EnsureTestCredential` 仅对 `test` 账户放行（防止被误用于覆盖普通用户/管理员密码）。

## V0 Compatibility

- Public REST 继续保留 `/me`、`/users`、`/users/exists`、`/users/:identifier`，避免破坏前端 MVP 和外部调用方。
- Account Service 同时提供 `/accounts`、`/accounts/exists`、`/accounts/:identifier` aliases，语义与对应 `/users` path 相同。
- JSON/RPC 中的 `user_id` 当前不批量改名；它是 account id alias。新增 public `account_id` 字段时必须保留 `user_id` alias 并提供兼容测试。
- `service/user/rpc/user.proto`、`service/user/api/user.api`、`cmd/user-api`、`cmd/user-rpc` 和部分 Go generated symbol 仍保留 `user`，作为 V0 transport compatibility。`service/user/rpc/user` 是 Go protobuf 输出路径，`service/user/rpc/userclient` 是 goctl RPC client。新增业务代码应优先使用 Account 术语或本仓库提供的 account alias seam。
- PostgreSQL source-of-truth 表为 `accounts` 与 `profiles`。`accounts` 保存 `account_id`、`identifier`、`email_normalized`、`email_verified_at`、`account_type` 和账号时间戳；`profiles` 保存展示名、名称、性别、生日、地区、头像 media 和资料时间戳；不持久化年龄。
- 旧 `account_type=normal` 不再作为有效输入兼容；迁移前必须转换为 `user`，否则按非法账号类型失败。

## Service Boundary

Account Service owns:

- Snowflake account id / V0 `user_id` alias generation and lookup;
- `identifier` uniqueness and public profile lookup;
- `email_normalized` / `email_verified_at` as account-owned contact/login identity fields;
- profile fields such as `display_name`、`name`、`gender`、`birth_date`、`region`;
- `account_type=user|agent|admin|test`;
- `/me` current account profile read/update through JWT identity.

Account Service does not own:

- password、password hash、salt、验证码、OAuth token、JWT signing keys;
- friend relationships;
- group membership;
- message history, seq, delivery state or read progress.

Auth registers or logs in credentials and collaborates with Account Service through existence/profile creation APIs. Friends, groups, message and gateway may keep V0 `user_id` fields but must treat them as account ids.

## Code Naming

The codebase now exposes account-named seams while preserving existing generated names:

- `model.Account` owns account identity fields and `model.Profile` owns profile/avatar fields; `model.User` remains a flattened V0 aggregate alias for existing callers.
- `repository.AccountRepository` is the account/profile repository contract; `UserRepository` remains a V0 alias.
- `logic.AccountProfile` and account-named methods on `UserLogic` expose Account terminology for new callers.
- `internal/servicecontext/user.ServiceContext.AccountLogic` points at the same implementation as `UserLogic`.

New application code should prefer Account naming when it is not constrained by public V0 JSON, goctl-generated symbols, or existing file paths.

## Next Phase

- Decide whether to generate first-class `account.api` / `account.proto` files or continue using V0 transport names.
- Expand first-class `account_id` response fields only with compatibility tests proving existing `user_id` clients still work.
- Decide whether to keep `user_id` as a long-term public alias after frontend API models move to account terminology.
- Update downstream friends/groups/message contracts only when frontend and API clients can consume both names.

## 验证方式

- `goctl api validate -api api/user.api`
- `go test ./...`
- `npm run frontend:test`
- `npm run frontend:build`
- `npm run frontend:lint`
- `bash scripts/verify-static.sh`
