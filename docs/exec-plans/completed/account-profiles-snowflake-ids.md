# Account Profiles Snowflake IDs

状态：Completed

## 背景

Account Service 当前仍以 `users` 单表和 `usr_` / `agt_` 前缀序列作为存储兼容实现。新要求允许重置 PostgreSQL 数据，并要求账号资料拆为 `accounts` / `profiles`，用 `account_type` 表示 human user、agent、admin，内部 ID 改为无前缀 Snowflake 数字字符串。

## 目标

- Account/Profile 作为后端模型与 PostgreSQL 存储语义的 source of truth。
- 创建账号时同时写入 account 与 profile。
- accounts 与 agents 使用无前缀 Snowflake 数字字符串 ID。
- Agent 绑定账号时只依赖 `account_type=2`（Agent），不依赖 ID 前缀。
- 保留 V0 `/users`、`user_id` 等兼容别名。

## 非目标

- 不实现前端 UI 改造。
- 不部署、不推送远端分支。
- 不迁移保留旧 PostgreSQL 数据；本次允许 reset/delete。
- 不批量重命名 friends/groups/message/gateway 的 V0 public `user_id` 字段。

## 任务拆分

- [x] Task 1：添加失败优先测试，覆盖 CreateUser/CreateAccount 和 Agent numeric unprefixed ID。
- [x] Task 2：添加 Snowflake ID generator，并接入 memory/PostgreSQL account 与 agent repositories。
- [x] Task 3：拆分 Account/Profile 模型，更新 account creation/query/update 数据流。
- [x] Task 4：将 PostgreSQL `users` 存储替换为 `accounts` + `profiles`，更新 agent/media FK。
- [x] Task 5：同步 API/proto/docs/静态验证术语，保留兼容 alias。
- [x] Task 6：运行聚焦测试、全量测试和要求的验证命令。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-02 | PostgreSQL migration 直接定义新表，不提供旧 `users` 数据迁移 | 用户明确允许本次 refactor 删除/重置 PostgreSQL 数据。 |
| 2026-05-02 | Public `user_id` 字段继续作为 compatibility alias 返回 | 避免破坏现有后端测试与 API contract，内部新增 `account_id` source 字段。 |
| 2026-05-02 | Agent profile 资料存于 `profiles`，`agents` 表仅保存 Agent 管理配置 | 满足 Agent 也需要头像/profile 字段，避免 profile 信息在 agents 表重复。 |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
go test ./tests -run 'User|Account|Agent'
for f in api/*.api; do goctl api validate -api "$f"; done
go test ./internal/logic ./internal/rpcgen/user ./internal/rpcgen/auth ./tests -run 'User|Account|Agent|Auth|Friends'
go test ./...
bash scripts/verify-static.sh
docker compose config
git diff --check
```

## 风险与回滚

- 风险：旧 PostgreSQL 数据不兼容新 schema。缓解：本任务约束中明确允许 reset/delete。
- 风险：V0 `user_id` 调用方仍存在。缓解：继续返回/接受 `user_id` alias，并在 docs 中标明其为 account id alias。
- 回滚：回退本提交并重建旧 migration schema。

## 结果记录

- 已添加失败优先测试；初始聚焦测试失败于 `usr_000001` / `agt_000001` legacy prefix 断言。
- 新增 `internal/idgen` Snowflake generator，memory/PostgreSQL account 和 agent repositories 均生成无前缀数字字符串 ID。
- PostgreSQL schema 改为 `accounts` + `profiles`；`agents.account_id` 引用 `accounts(account_id)`，Agent 展示资料依赖 `profiles` 和 `account_type=2`（Agent）。
- `model.Account` / `model.Profile` 拆分，`model.User` 和 public `user_id` 作为 V0 compatibility aggregate/alias 保留。
- 已同步架构、设计、产品契约、DB schema snapshot 和静态验证规则。

验证结果：

- `go test ./tests -run 'User|Account|Agent'`：先失败于 legacy prefix，修复后通过。
- `for f in api/*.api; do goctl api validate -api "$f"; done`：通过。
- `go test ./internal/logic ./internal/rpcgen/user ./internal/rpcgen/auth ./tests -run 'User|Account|Agent|Auth|Friends'`：通过。
- `go test ./...`：通过。
- `bash scripts/verify-static.sh`：通过。
- `docker compose config`：通过。
- `git diff --check`：通过。
