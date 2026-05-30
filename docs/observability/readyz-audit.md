# `/readyz` Ready-Check Audit（OB-16）

> 审计每个服务 `/readyz` 实际探测了哪些依赖，回答 §05 OB-16：`/readyz` 是否真能检测出 PG / Redis / Kafka 故障。
> 代码位置：`pkg/health/health.go`（`ReadinessHandler`/`ComponentCheck`）、`internal/handler/gozero_routes.go`、各 `service/<svc>/{api,main}`。
> 审计日期：2026-05-30。

## 结论（重要）

**当前几乎所有 `/readyz` 只做"装配检查"（in-process 组件 non-nil / config 非空），并不对 PG / Redis / Kafka 做存活探测。** 唯一的真实探测是 `gateway-ws` 的 `websocket_server`（`wsServer.Ready()`）。

后果：当 PostgreSQL / Redis / Redpanda 宕机但进程仍在运行时，`/readyz` 仍返回 `ready` → k8s **不会**把故障 pod 摘出流量。`/readyz` 目前等价于一个加强版 `/healthz`，不具备依赖级就绪语义。

> 探测类型图例：🟢 真实存活探测（连接/ping） · 🟡 装配检查（non-nil/配置非空） · ⚪ 无 `/readyz`

## 审计表

| 服务 | 暴露 `/readyz` | ready checks | 探测类型 | 关键依赖（PG/Redis/Kafka） | 依赖被探测？ |
|------|:---:|------|:---:|------|:---:|
| auth-api | ✅ | auth_logic, auth_repository, user_client, mail_rpc_client | 🟡 | 经下游 RPC 间接用 PG | ❌ |
| user-api | ✅ | auth_config, account_logic, user_logic, repository, media_logic | 🟡 | PG（user-rpc/repo）、对象存储 | ❌ |
| friends-api | ✅ | auth_config, friends_logic, repository | 🟡 | PG | ❌ |
| groups-api | ✅ | auth_config, groups_logic, groups_repository | 🟡 | PG | ❌ |
| message-api | ✅ | auth_config, message_logic, ai_hosting_logic, message_repository, feedback_logic, feedback_repository, ai_hosting_repository, outbox_repository | 🟡 | PG、Kafka（事件发布） | ❌ |
| agent-api | ✅ | auth_config, agent_logic, agent_definition_logic, agent_repository, agent_registry_repository | 🟡 | PG、DeepSeek、LLM obs | ❌ |
| admin-api | ✅ | auth_config, admin_logic, accounts | 🟡 | PG | ❌ |
| gateway-ws | ✅ | **websocket_server（Ready()）** 🟢, message_logic 🟡, presence_store 🟡 | 🟢🟡 | Redis（presence）、message-rpc | ❌（Redis 未 ping） |
| message-transfer | ✅ | event_consumer, delivery_dispatcher | 🟡 | **Kafka（消费）**、PG、gateway dispatch | ❌ |
| auth-rpc | ⚪ | 无 HTTP `/readyz` | ⚪ | PG | ❌ |
| user-rpc | ⚪ | 无 HTTP `/readyz` | ⚪ | PG | ❌ |
| friends-rpc | ⚪ | 无 HTTP `/readyz` | ⚪ | PG | ❌ |
| groups-rpc | ⚪ | 无 HTTP `/readyz` | ⚪ | PG | ❌ |
| message-rpc | ⚪ | 无 HTTP `/readyz` | ⚪ | PG、Kafka | ❌ |
| mail-rpc | ⚪ | 无 HTTP `/readyz` | ⚪ | SMTP/外部 | ❌ |

## 缺口与建议（后续 issue）

1. **给"拥有"依赖的服务加真实就绪探测**（带超时、缓存几秒避免抖动）：
   - PG：`db.PingContext`（auth/user/friends/groups/message/agent/admin 及对应 rpc）
   - Redis：`client.Ping`（gateway-ws presence、其他用 Redis 者）
   - Kafka/Redpanda：broker metadata / consumer group 状态（message-transfer、message-rpc）
2. **RPC 服务暴露就绪**：用 go-zero/gRPC health 或附带一个轻量 HTTP `/readyz`，至少 ping PG。
3. **就绪与存活分离**：`/healthz`（liveness）保持纯进程存活；`/readyz`（readiness）反映依赖可用，避免依赖抖动导致 liveness 重启。
4. 探测失败应让 `/readyz` 返回 503（`health.Readiness` 已支持：任一 check 非 `ready` 即 `not_ready` → handler 返回 503）。

> 实现这些探测会改动每个服务的就绪构造（需把 db/redis/kafka 句柄引入就绪路径），属独立改动，不在本审计范围内。本表为后续实现的依据。
