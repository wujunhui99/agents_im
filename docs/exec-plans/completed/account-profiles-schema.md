# Account Profiles Schema Refactor

状态：Completed

## 背景

Account Service 需要把 PostgreSQL 存储从 V0 `users` 表拆为身份主体 `accounts` 和资料主体 `profiles`。本次迁移允许清空现有中间件 PostgreSQL 数据，不要求兼容旧行。

## 目标

- 将账号存储表改为 `accounts` + `profiles`。
- 保持 public JSON/API 中的 `user_id` 兼容字段，但数据库主账号 ID 使用无前缀数字字符串。
- 让 Postgres repository 读写 `accounts` + `profiles`。
- 让 `auth_credentials`、`friendships`、`media_objects`、`agents.im_user_id` 等账号引用指向 account id 语义。
- 用测试覆盖账号创建、资料更新、认证凭据和好友关系。

## 非目标

- 不改前端 UI。
- 不改 public REST/RPC 字段名。
- 不部署、不推送远端分支。

## 任务拆分

- [x] 添加失败优先测试，证明当前实现仍写 `users` 且生成带前缀 ID。
- [x] 引入无前缀数字账号 ID 生成能力。
- [x] 重写 `001_init_postgres.sql` 的账号相关 schema。
- [x] 更新 Postgres user/friends/auth/agent/media repository SQL。
- [x] 更新相关文档和静态校验规则。
- [x] 运行聚焦测试和完整验证。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-02 | 保留代码与 public JSON 的 `user_id` 命名作为 account id alias。 | 用户明确要求现阶段保持 public compatibility。 |
| 2026-05-02 | Fresh schema 使用 `accounts.account_id`，repository 查询时别名为 `user_id`。 | 避免把 V0 transport 命名继续固化到核心表名。 |
| 2026-05-02 | 移除 `account_type=normal` 归一化。 | 本次 refactor 不保留旧 PostgreSQL 行兼容，非法类型应 fail-first。 |

## 验证方式

- `export PATH=/tmp/go/bin:$HOME/go/bin:$PATH`
- `for f in api/*.api; do goctl api validate -api "$f"; done`
- `go test ./internal/repository ./internal/auth/repository ./tests -run 'Postgres|Friends|User|Auth' || true`
- `go test ./...`
- `bash scripts/verify-static.sh`
- `docker compose config`
- `git diff --check`

## 风险与回滚

本次迁移对旧 PostgreSQL 行不兼容。若在共享环境回滚，需要恢复旧 migration 与旧 repository，并清空或重建 middleware PostgreSQL 数据。

## 结果记录

完成 `accounts` + `profiles` schema 拆分；Postgres Account repository 创建时写 `accounts` 和 `profiles`，资料更新写 `profiles`，认证、好友、媒体、群和 Agent 账号引用改为 account id 语义。新增 Snowflake-style 数字字符串 ID generator，Postgres 账号 ID 不再带 `usr_` 前缀。

Fail-first 记录：

- 首次聚焦测试命令 `go test ./internal/repository ./internal/auth/repository -run 'TestPostgres(AccountSchemaUsesAccountsAndProfiles|CreateAccountWritesAccountsAndProfiles|UpdateProfileWritesProfilesTable|CredentialCreateStoresSameAccountID)'` 失败，确认 migration 缺少 `accounts`、`Create` 仍 `insert into users`、`UpdateProfile` 仍 `update users`。
- `docker compose up -d postgres` 因当前用户无 Docker socket 权限失败，无法运行真实 PostgreSQL 容器 E2E；已运行 build-tag integration test 编译/skip 验证。

最终验证：

- `for f in api/*.api; do goctl api validate -api "$f"; done`
- `go test ./internal/repository ./internal/auth/repository ./tests -run 'Postgres|Friends|User|Auth' || true`
- `go test ./...`
- `bash scripts/verify-static.sh`
- `docker compose config`
- `git diff --check`
