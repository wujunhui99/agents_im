# Redis Presence

状态：Completed

## 背景

WebSocket Gateway 分支会实现真实长连接服务。当前分支先提供 Redis 本地中间件、presence 契约和可测试实现，使 Gateway 后续可以专注连接鉴权、命令处理、下发和 ACK。

## 目标

- 在 `docker-compose.yml` 中新增 Redis 服务。
- 在 `.env.example` 中新增本地开发 Redis 和 presence 配置，不提交真实密码。
- 在 `internal/config` 中新增 Redis/presence 配置解析，保持 PostgreSQL `StorageDriver` 和 `DataSource` 路径不破坏。
- 在 `internal/presence` 中新增 `PresenceStore`、内存实现和 Redis 实现。
- 明确 Redis key、TTL、数据权威边界：PostgreSQL 是持久数据权威，Redis 只保存在线/短期状态。
- 普通 `go test ./...` 不依赖外部 Redis；Redis 集成测试只有 `REDIS_ADDR` 存在时运行，否则 skip。
- 更新静态校验脚本覆盖 Redis/presence 文档、代码和测试策略。

## 非目标

- 不实现真实 WebSocket server。
- 不实现 GitHub Actions。
- 不改 `main` / `develop`，不合并其他分支。
- 不把消息历史、已读状态、用户资料、好友/群成员关系迁移到 Redis。

## 任务拆分

- [x] Task 1：读取 AGENTS、架构、go-zero、PostgreSQL、Gateway/Message 契约文档。
- [x] Task 2：新增 Redis docker-compose 服务和 `.env.example` 本地配置。
- [x] Task 3：扩展 `internal/config` 的 Redis/presence 配置解析。
- [x] Task 4：实现 `internal/presence` 契约、内存存储、Redis 存储和测试。
- [x] Task 5：新增 `docs/design-docs/redis-presence.md` 并更新架构/索引。
- [x] Task 6：运行强制验证并记录结果。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | Presence 默认使用 memory driver | 保证普通单测和服务启动不依赖外部 Redis |
| 2026-04-29 | Redis 仅存连接 hash、用户连接 set 和短期 online marker | 支持 Gateway 多实例共享在线状态，同时避免 Redis 成为持久数据权威 |
| 2026-04-29 | `IsUserOnline` 以非过期连接 hash 为准 | 避免仅凭 marker 产生 stale online 判断 |
| 2026-04-29 | 用户 set 和 online marker TTL 使用 `2 * heartbeat_ttl` | 给连接 hash 过期后的懒清理留出缓冲 |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name "*.go" -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
git status --short --branch
```

## 风险与回滚

- 风险：Redis 中残留 stale connection id。缓解方式：连接 hash TTL 到期后，`ListUserConnections` 会清理 stale set member。
- 风险：Redis 不可用导致 Gateway 无法查询在线状态。缓解方式：Redis presence 不影响 PG 中的消息、会话和已读权威数据；Gateway 后续应降级为离线补偿/拉取。
- 回滚：移除 Redis compose/env、`internal/presence` 包和文档入口即可恢复到纯内存/PG 阶段。

## 结果记录

- `goctl --version`：通过，版本 `goctl version 1.10.1 linux/amd64`。
- `for f in api/*.api; do goctl api validate -api "$f"; done`：通过，5 个 API 文件均 `api format ok`。
- `gofmt -w $(find . -name "*.go" -print)`：已执行。
- `go test ./...`：通过；Redis integration test 在未设置 `REDIS_ADDR` 时 skip，不依赖外部 Redis。
- `bash scripts/verify-static.sh`：通过，输出 `static verification passed`。
- `docker compose config`：通过，包含 `postgres` 和 `redis` 服务以及 `agents_im_redis_data` volume。
