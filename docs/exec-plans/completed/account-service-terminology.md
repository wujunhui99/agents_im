# account-service-terminology

状态：Completed
Completed: 2026-05-01

## 背景

项目历史上使用 `user` 表示账号资料服务，但产品方向要求 human user、agent、admin 和未来服务号都归入 Account 领域。

## 目标

- 将入口文档和契约说明统一到 Account Service。
- 明确 `account_type=0|1|2`（0=管理员，1=用户，2=Agent），并把旧 `normal` 作为临时迁移输入兼容。
- 保留 `/me`、`/users/*` 和 `user_id` V0 compatibility，说明它们是 account id alias。
- 增加 account 命名的代码 seam，避免新业务继续依赖 user domain 命名。
- 更新静态检查，防止 Account 术语和兼容说明回退。

## 非目标

- 不实现 Agent 托管自动回复。
- 不把 credential/password/token 放入 Account Service。
- 不批量重命名所有 public JSON 字段或数据库表。
- 不引入 mock/fake success。

## 任务拆分

- [x] 阅读必读文档和 go-zero 参考。
- [x] 更新 Account terminology 设计文档和入口文档。
- [x] 更新 account_type 代码语义和 `/accounts` V0 alias。
- [x] 更新静态检查。
- [x] 运行必需验证命令。
- [x] 移动计划到 completed 并记录结果。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-01 | 保留 `user_id` 作为 V0 public compatibility。 | 避免破坏前端 MVP、friends/groups/message/gateway 已有契约。 |
| 2026-05-01 | `account_type=normal` 只作为迁移输入兼容，输出统一为 `user`。 | 产品语义要求 `user` 是 account type；兼容旧数据不能吞错或伪造成功。 |
| 2026-05-01 | 保留 `users` 表和 `user-api`/`user-rpc` transport 名称。 | 表/服务二进制重命名需要独立部署迁移；本任务先收敛领域语义。 |

## 验证方式

按任务要求从仓库根目录运行完整命令，并把结果记录到 completed 计划。

## 风险与回滚

- 风险：旧调用方传入 `normal`。缓解：当前 logic/repository 接受并归一化为 `user`，migration 更新旧 rows。
- 风险：新增 `/accounts` aliases 与 `/users` 行为漂移。缓解：同一 handler 实现，静态检查覆盖 API spec 与 route registration。
- 回滚：可回滚本分支 commit；若已执行 DB migration，需将 `users.account_type` 的 `user` rows 回写为旧 `normal` 并恢复旧 constraint。

## 结果记录

已完成：

- 新增 [`docs/design-docs/account-service-terminology.md`](../../design-docs/account-service-terminology.md)，明确 Account Service、`account_type=0|1|2`（0=管理员，1=用户，2=Agent）、`user_id` account id alias 和 V0 compatibility。
- 更新 AGENTS、ARCHITECTURE、Account/social/product/frontend contracts、go-zero 设计、auth/friends/groups/message 相关文档中的 Account 术语。
- 将默认 `account_type` 从旧 `normal` 迁移为 1（用户），保留旧 `normal` 输入归一化兼容。
- 增加 `/accounts`、`/accounts/exists`、`/accounts/:identifier` aliases，并保留 `/users/*` V0 paths。
- 增加 account 命名代码 seam：`model.Account`、`repository.AccountRepository`、`logic.AccountProfile`、`NewAccountLogic`、`ServiceContext.AccountLogic`。
- 更新 `scripts/verify-static.sh`，覆盖 terminology doc、account aliases、account_type values、V0 compatibility 和 storage migration。

验证记录：

| 命令 | 结果 |
| --- | --- |
| `export PATH=/tmp/go/bin:$HOME/go/bin:$PATH; goctl --version` | 通过，`goctl version 1.10.1 linux/amd64` |
| `for f in api/*.api; do goctl api validate -api "$f"; done` | 通过，6 个 `.api` 均 `api format ok` |
| `gofmt -w $(find . -name '*.go' -print)` | 通过 |
| `go test ./...` | 通过 |
| `npm --prefix web ci` | 通过 |
| `npm run frontend:test` | 通过，8 files / 18 tests passed |
| `npm run frontend:build` | 通过 |
| `npm run frontend:lint` | 通过 |
| `bash scripts/verify-static.sh` | 通过 |
| `POSTGRES_PASSWORD=local-postgres-placeholder REDIS_PASSWORD=local-redis-placeholder MINIO_ROOT_USER=local-minio-user MINIO_ROOT_PASSWORD=local-minio-password docker compose -f deploy/middleware/docker-compose.yml config >/tmp/account_service_terminology_compose_config.txt` | 通过 |
| `bash -n scripts/dev-up.sh` | 通过 |
| `bash -n scripts/dev-demo-data.sh` | 通过 |
| `git diff --check` | 通过 |
