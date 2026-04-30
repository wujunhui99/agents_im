# gateway-presence-routing

状态：Completed

归档日期：2026-05-01

## 背景

WebSocket Gateway 已具备 JWT handshake、连接管理、同步 message command router 和本进程内 push fanout。Redis Presence 已提供 `PresenceStore` 契约、memory 实现和 Redis 实现。当前分支负责把 Gateway 连接生命周期接入 Presence，并为后续跨 Gateway 实例投递补齐路由元数据。

## 目标

- WebSocket 连接建立时写入 `PresenceStore`。
- app `heartbeat` 和 WebSocket pong 刷新 presence TTL。
- WebSocket 断开时 unregister presence。
- presence metadata 包含 `instance_id` / `connection_id` 等未来路由字段。
- delivery dispatcher 在本地 fanout 前查询 presence online/offline 和 route metadata。
- 默认测试使用 memory presence，不依赖 Redis、PostgreSQL、Kafka 或 Docker 服务。
- 更新设计文档、活跃执行计划和静态验证脚本。

## 非目标

- 不实现 Redis-only 默认测试路径。
- 不实现 Kafka consumer、outbox publisher 或 Transfer worker 生产 wiring。
- 不实现 Gateway 之间的跨进程 RPC。
- 不改变现有 WebSocket command request payload 和同步 send/pull/read 行为。

## 任务拆分

- [x] 读取 AGENTS、ARCHITECTURE、go-zero references、message chain、Kafka events、outbox、transfer、Gateway push delivery 文档。
- [x] 梳理 `internal/gateway/ws`、`internal/gateway/delivery` 和 `internal/presence` 现有 seam。
- [x] 在 Gateway Server 中接入 `PresenceStore`、TTL 和 instance id。
- [x] 实现 connect register、heartbeat/pong refresh、disconnect unregister。
- [x] 扩展 presence metadata 和 delivery result route metadata。
- [x] 在 delivery dispatcher 中添加 presence-aware online/offline/routed lookup。
- [x] 新增 WebSocket Gateway presence routing 测试。
- [x] 新增 `docs/design-docs/gateway-presence-routing.md` 并同步索引/架构文档。
- [x] 为 `scripts/verify-static.sh` 增加 presence-routing docs/code 检查。
- [x] 执行强制验证并记录结果。
- [x] 使用 conventional commit 提交并推送 `feature/gateway-presence-routing`。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | Gateway 默认使用 memory presence，配置为 redis 时才接入 Redis | 保证普通 `go test ./...` 和默认本地启动不依赖外部 Redis。 |
| 2026-04-29 | Presence 写入失败不改变 WebSocket command ACK 协议 | Presence 是短期路由状态，不是消息持久化权威；send/pull/read 行为必须保持。 |
| 2026-04-29 | Delivery dispatcher 在 local fanout 前执行 `ListUserConnections` | 后续跨实例投递需要先判断 presence route，本分支仍只做本进程内写 WebSocket。 |
| 2026-04-29 | 新增 `routed` recipient status | 表达 presence 有在线路由但当前 Gateway 没有本地连接，供未来跨实例 transport 使用。 |

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

- 风险：Redis/presence 短暂不可用导致 online route lookup 失败。缓解：delivery result 返回 `failed`，持久消息仍可通过客户端 reconnect 后 pull 补偿。
- 风险：presence stale route 使 dispatcher 返回 `routed` 但当前分支没有跨实例 transport。缓解：文档明确 `routed` 是未来 seam，本分支不做远程 fanout。
- 风险：heartbeat 未及时刷新 TTL。缓解：app heartbeat 和 WebSocket pong 均刷新 presence，测试覆盖 app heartbeat。
- 回滚：移除 Gateway `PresenceStore` option/wiring、route metadata、presence-aware dispatcher 分支和对应测试/文档/静态验证条目。

## 结果记录

2026-05-01 状态对齐：

- 当前 `main` 已包含 Gateway 连接生命周期写入 `PresenceStore`、heartbeat/pong TTL refresh、disconnect unregister、presence-aware delivery routing seam 和 `routed` result。
- 该计划已无 active 剩余任务，因此从 active 归档到 completed。
- 本计划没有实现跨 Gateway 进程 RPC、生产 Message Transfer wiring、offline push、delivery ACK worker 或 read receipt push ACK；`routed` 仍只是未来跨实例 transport 的 seam。
- 本次只做文档状态对齐，未启动真实依赖，也未声称端到端验证。

已完成 Gateway 连接生命周期到 `PresenceStore` 的接入、heartbeat TTL refresh、disconnect unregister、presence-aware delivery routing seam、route metadata、memory-only 默认测试路径、设计文档和静态验证脚本更新。

验证结果：

- `goctl --version`：通过，`goctl version 1.10.1 linux/amd64`。
- `for f in api/*.api; do goctl api validate -api "$f"; done`：通过，5 个 `.api` 文件均 `api format ok`。
- `gofmt -w $(find . -name "*.go" -print)`：通过。
- `go test ./...`：通过；默认测试不需要 PostgreSQL、Redis、Kafka。
- `bash scripts/verify-static.sh`：通过，输出 `static verification passed`。
- `docker compose config`：通过。
- `npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json ...`：通过。
