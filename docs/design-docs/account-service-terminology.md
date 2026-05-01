# Account Service Terminology

状态：Accepted

## 背景

IM 身份主体不只包含人类用户。Agent、admin、未来服务号/公众号都需要登录、持有资料并参与消息链路，因此账号资料领域统一命名为 **Account**。历史 `user` 命名在第一阶段只作为 V0 transport/API/storage compatibility 保留；这也是本文的 V0 compatibility 边界。

## 术语

- Account：身份与资料主体，可代表 human user、agent、admin，未来可扩展 service/official account。
- Account Service：账号资料权威服务，负责 identifier、展示名、性别、年龄、地区、`account_type` 等资料，不负责 credential/password/token。
- Auth Service：认证与凭据边界，负责 password hash、salt、token 签发与校验，不负责账号展示资料。
- `account_type`：账号类型，当前支持 `user`、`agent`、`admin`。`user` 是 account type，不是服务名。
- `user_id`：V0 public compatibility 字段，是 account id alias。friends/groups/message/gateway/read state 中的 `user_id` 均指向 Account Service 管理的 account id。

## V0 Compatibility

- Public REST 继续保留 `/me`、`/users`、`/users/exists`、`/users/:identifier`，避免破坏前端 MVP 和外部调用方。
- Account Service 同时提供 `/accounts`、`/accounts/exists`、`/accounts/:identifier` aliases，语义与对应 `/users` path 相同。
- JSON/RPC 中的 `user_id` 当前不批量改名；它是 account id alias。后续如新增 `account_id` 字段，必须先提供双写/双读兼容层和测试。
- `proto/user.proto`、`cmd/user-api`、`cmd/user-rpc`、`api/user.api` 文件路径和部分 Go generated symbol 仍保留 `user`，作为 V0 transport compatibility。新增业务代码应优先使用 Account 术语或本仓库提供的 account alias seam。
- PostgreSQL `users` 表作为 V0 storage compatibility 保留，但逻辑语义是 account profiles。下一阶段 DB rename 需要独立迁移计划、回滚策略和数据校验。
- 旧 `account_type=normal` 仅作为迁移输入兼容，写入与返回统一归一化为 `user`。

## Service Boundary

Account Service owns:

- account id / V0 `user_id` generation and lookup;
- `identifier` uniqueness and public profile lookup;
- profile fields such as `display_name`、`name`、`gender`、`age`、`region`;
- `account_type=user|agent|admin`;
- `/me` current account profile read/update through JWT identity.

Account Service does not own:

- password、password hash、salt、验证码、OAuth token、JWT signing keys;
- friend relationships;
- group membership;
- message history, seq, delivery state or read progress.

Auth registers or logs in credentials and collaborates with Account Service through existence/profile creation APIs. Friends, groups, message and gateway may keep V0 `user_id` fields but must treat them as account ids.

## Code Naming

The codebase now exposes account-named seams while preserving existing generated names:

- `model.Account` aliases the persisted profile model.
- `repository.AccountRepository` is the profile repository contract; `UserRepository` remains a V0 alias.
- `logic.AccountProfile` and account-named methods on `UserLogic` expose Account terminology for new callers.
- `svc.ServiceContext.AccountLogic` points at the same implementation as `UserLogic`.

New application code should prefer Account naming when it is not constrained by public V0 JSON, goctl-generated symbols, or existing file paths.

## Next Phase

- Decide whether to generate first-class `account.api` / `account.proto` files or continue using V0 transport names.
- Add `account_id` response fields only with compatibility tests proving existing `user_id` clients still work.
- Plan `users` table rename to `accounts` or a view-backed migration after live compatibility requirements are known.
- Update downstream friends/groups/message contracts only when frontend and API clients can consume both names.

## 验证方式

- `goctl api validate -api api/user.api`
- `go test ./...`
- `npm run frontend:test`
- `npm run frontend:build`
- `npm run frontend:lint`
- `bash scripts/verify-static.sh`
