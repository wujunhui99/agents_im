# 03 — 消息转发机制重构（基于 OpenIM 实现版）

> **重写说明（2026-05-29）**：上一版基于"保留 outbox + 推 push 服务"的中间方案。在阅读 `open-im-server/internal/{rpc/msg,msgtransfer,push,msggateway}/` 实际代码后，全部改为对齐 OpenIM 实现。**outbox 弃用**——理由见 §3。
>
> **补充（2026-06-07）**：复核 open-im-server 代码确认链路无误（`message_handler.go`/`send.go`/`online_history_msg_handler.go`/`push_handler.go`）；新增 §0.1（与 `internal/` 退役的关系、当前→目标命名映射、关联 `refactor-domain-to-service` skill）与 §7.2「上行 ACK 同步透传」说明。
>
> 阅读基础：`open-im-server/` 已 clone 在 `/Users/junhui/code/project/agents_im/open-im-server/`。
>
> **路径约定**：本文 `internal/rpc/msg/...`、`internal/msgtransfer/...`、`internal/push/...`、`internal/msggateway/...` 均指 **OpenIM 仓库**源码路径（参考用）；`internal/outboxpublisher/`、`internal/transfer/`、`internal/repository/postgres_outbox.go` 等指 **agents_im 当前**真实位置（§9 B3 退役）。按 00-decisions **D10**，agents_im 重构后剩余的 messaging / presence / idgen / observability 等基础设施位于 `pkg/<name>/`；msg-rpc / transfer / push / gateway 业务代码位于 `service/<svc>/internal/...`。

---

## 0. TL;DR — 关键决策

| 决策                                         | 选择                                              | 来源 |
|----------------------------------------------|---------------------------------------------------|------|
| msg-rpc 是否同步写 PG / 同 tx 写 outbox       | **否**。msg-rpc 只 `producer.SendMessage()` 发 Kafka | OpenIM `internal/rpc/msg/send.go:81` |
| seq 在哪里分配                                | **msgtransfer**，通过 `seqConversation.Malloc(ctx, convID, len(msgs))`（Redis 原子 INCR） | OpenIM `pkg/common/storage/controller/msg_transfer.go:210` |
| 持久化路径                                   | msgtransfer 写 Redis cache → 发 toMongo topic → toMongo consumer 批量入 MongoDB | OpenIM `internal/msgtransfer/online_history_msg_handler.go:318` |
| 是否引入 outbox                              | **不引入**。Kafka 是事实源 + 顺序保证             | 见 §3 |
| push 路由模型                                | **广播**到所有 gateway 实例（service discovery），每个 gateway 查本地连接表 | OpenIM `internal/push/onlinepusher.go:70` |
| 离线 push                                    | 二段式：push 消费 toPush，online 失败的 user 再 produce 到 toOfflinePush；独立 consumer 调 FCM/APNs/Getui/JPush | OpenIM `internal/push/push.go:80~135` |
| Kafka topics                                 | `toRedis`（msg-rpc→transfer）、`toMongo`（transfer→transfer）、`toPush`（transfer→push）、`toOfflinePush`（push→push） | OpenIM `config/kafka.yml` |

---

## 0.1 与 `internal/` 退役的关系（keystone）

本文是**传输链路设计**；消息域同时是顶层 `internal/` 退役的**最后一公里**（见 [`progress/02-microservices.md`](./progress/02-microservices.md) §全局收尾）。链路改造与去 internal 在同一批 PR 内做，遵循 [`refactor-domain-to-service` skill] 的 goctl + BFF 主线：

- **数据层退役**：msg-rpc 走 **goctl model**（`service/msg/rpc/internal/model`，对 `messages` / `conversations` 表），删掉对 `internal/repository` 的依赖；复合主键表先加自增代理 PK 再 `goctl model pg`，事务边界留在 Logic。
- **跨域数据上移 BFF**：msg-rpc 只返回自有字段；发送者昵称/头像等跨域字段由 `service/msg/api`(BFF) 聚合 user-rpc（批量接口防 N+1），**rpc 之间不互调**。
- **keystone 解锁**：`internal/rpcgen/message` 当前 in-process 构造几乎所有域的 `*Logic`（groups/friends/user/...）。这些域的旧 `internal/logic` 代码**必须等 message 迁移完成才能删**——message 是删空 `internal/` 的前置。
- **RPC 面**：msg-rpc 承担消息域全部 RPC（MVP 10 个，见 [`07-message-rpc-redesign.md`](./07-message-rpc-redesign.md)），本文聚焦其中 `SendMessage` 的「只发 Kafka」改造（封装为 `MsgToMQ`）与下行 push 链路。
- **net-new 部署单元**：`push` 是新可部署单元，需按 skill「配套改动清单」把 binary 串进 Dockerfile / drone-build / deploy-k3s / detect-deploy / deployments+services / kustomization / verify 全链。
- **实施顺序：先 07 后 03**——本文链路依赖 msg/rpc 的接口面与 `SendMessage → MsgToMQ` 改造（[`07`](./07-message-rpc-redesign.md) §3.1/§4.2），故先定 msg-rpc 边界再接链路。

**当前 → 目标 命名映射**（现仓库仍是旧名，target 名与本文/用户口径一致）：

| 现状（repo）                       | 目标（本文）                        | 变化 |
|-----------------------------------|-------------------------------------|------|
| `service/gateway-ws`              | `service/msggateway`                | 改名 + 砍业务依赖（纯连接层）|
| `service/message-api` + monolith  | `service/msg/api` + `service/msg/rpc` | api 转纯/聚合 BFF；rpc 从 monolith 抽出 + goctl 数据层 |
| `service/message-transfer`        | `service/msgtransfer`               | 改名 + batcher / Redis seq Malloc |
| （无）                             | `service/push`                      | 新建可部署单元 |
| `internal/outboxpublisher` 等      | 删除                                | §9 B3（灰度稳定后退役）|

[`refactor-domain-to-service` skill]: ../../../.claude/skills/refactor-domain-to-service/SKILL.md

---

## 1. OpenIM 实际架构（精读后版本）

```
┌──────────────────┐
│ Client (SDK)     │  WebSocket / REST
└────┬─────────────┘
     │
     ▼
┌─────────────────────────────────────────┐
│ msggateway (cmd/openim-msggateway)      │
│ - WebSocket 长连接 + auth/heartbeat      │
│ - 注册 gRPC server (msggateway.MsgGateway)│
│ - 本地 connection manager（不入 Redis） │
│ - 上行消息 → 调 msg-rpc gRPC SendMsg     │
│ - 下行 push: 由其它服务调本机 gRPC      │
└────┬────────────────────────────────────┘
     │ gRPC SendMsg
     ▼
┌─────────────────────────────────────────┐
│ msg-rpc (openim-rpc-msg)                 │
│ SendMsg = MsgToMQ(convKey, msg)         │
│   │                                      │
│   └─ producer.SendMessage(key=convID,    │
│                          value=proto)    │
│   *** 没有 DB 写；没有 outbox；没有 tx ***│
└────┬────────────────────────────────────┘
     │  Kafka topic: toRedis
     │  partition key = conversationID
     │  保证：同会话内消息严格顺序
     ▼
┌─────────────────────────────────────────────────────────────┐
│ msgtransfer (openim-msgtransfer)                            │
│                                                              │
│ Consumer 1: toRedis → OnlineHistoryRedisConsumerHandler      │
│   batcher: size=500, worker=50, interval=100ms              │
│   sharding key = conversationID hash → worker idx           │
│   per-batch (per-conversation):                              │
│     1. categorize（storage / notStorage、msg / notification）│
│     2. seqConversation.Malloc(convID, len)  ← Redis 原子分配 │
│     3. msgCache.SetMessageBySeqs(convID, msgs) ← Redis cache │
│     4. SetHasReadSeqs(convID, userSeqMap) ← Redis           │
│     5. MsgToMongoMQ(key, convID, msgs, seq)  ← Kafka toMongo │
│     6. MsgToPushMQ(...) per-message  ← Kafka toPush          │
│   commit Kafka offset                                        │
│                                                              │
│ Consumer 2: toMongo → OnlineHistoryMongoConsumerHandler      │
│   批量写 MongoDB（msgs collection，bucket by seq/100）       │
│                                                              │
│ 后台: HandleUserHasReadSeqMessages → 异步把 read seq 写库   │
└──────────────────────────────────────────────────────────────┘
              │                                  │
              │ Kafka: toPush                    │ Kafka: toMongo
              │ key=convID                       │
              ▼                                  ▼
┌─────────────────────────────────────┐  ┌─────────────┐
│ push (openim-push)                   │  │  MongoDB    │
│                                      │  │ msg history │
│ Consumer 1: toPush → HandleMs2PsChat │  │ (archive)   │
│   Push2User / Push2Group:            │  └─────────────┘
│     - onlineCache.GetUsersOnline    
│     - online: 调所有 gateway 实例    
│       gRPC SuperGroupOnlineBatchPush 
│     - online 失败/offline 的 user：  
│       publish to toOfflinePush      
│                                      
│ Consumer 2: toOfflinePush →          
│   HandleMsg2OfflinePush             
│     - FCM / APNs / Getui / JPush    
│                                      
│ + 注册 gRPC server (PushMsgService) 
│   提供 DelUserPushToken 等 admin    
└────────┬─────────────────────────────┘
         │ gRPC (broadcast to all gateways)
         ▼
   msggateway 实例 1..N
   本地 connection 投递
```

### 1.1 关键设计要点

#### (a) msg-rpc 完全无状态
`internal/rpc/msg/send.go:81` `sendMsgGroupChat`：

```go
err = m.MsgDatabase.MsgToMQ(ctx, conversationutil.GenConversationUniqueKeyForGroup(req.MsgData.GroupID), req.MsgData)
```

`MsgToMQ` 实现 `pkg/common/storage/controller/msg.go:125`：

```go
func (db *commonMsgDatabase) MsgToMQ(ctx context.Context, key string, msg2mq *sdkws.MsgData) error {
    data, err := proto.Marshal(msg2mq)
    if err != nil { return err }
    return db.producer.SendMessage(ctx, key, data)
}
```

**就这么简单**。SendMsg ACK 给客户端只代表 Kafka 接受了消息，不代表持久化到 MongoDB、也不代表 seq 已分配。

#### (b) seq 由 msgtransfer 在 batcher 内分配
`pkg/common/storage/controller/msg_transfer.go:201` `BatchInsertChat2Cache`：

```go
currentMaxSeq, err := db.seqConversation.Malloc(ctx, conversationID, int64(len(msgs)))
// Redis 原子 INCRBY，返回旧值
isNew = currentMaxSeq == 0
for _, m := range msgs {
    currentMaxSeq++
    m.Seq = currentMaxSeq
    ...
}
db.msgCache.SetMessageBySeqs(ctx, conversationID, ...)
```

batcher 按 `conversationID hash → worker idx` 分片（`online_history_msg_handler.go:111`）。**同一 conversation 的消息永远落到同一 worker** → seq 严格单调，无锁竞争。

#### (c) Kafka partition key = conversationID
保证同会话内消息按发送顺序进 Kafka，进而按顺序被 msgtransfer 消费。**这是顺序保证的物理基础**。

#### (d) MongoDB 写入异步
msgtransfer 不直接写 Mongo，而是 produce 到 toMongo topic，由 OnlineHistoryMongoConsumerHandler 批量入库。这把"DB 慢"对消息链路的影响隔离掉。

#### (e) push 广播路由
`internal/push/onlinepusher.go:69`：

```go
conns, err := d.disCov.GetConns(ctx, d.config.Discovery.RpcService.MessageGateway)
// 拿到所有 gateway 实例的 grpc.ClientConn
for _, conn := range conns {
    wg.Go(func() error {
        msgClient := msggateway.NewMsgGatewayClient(conn)
        reply, err := msgClient.SuperGroupOnlineBatchPushOneMsg(ctx, input)
        ...
    })
}
```

每个 gateway 自己看本地 connection map，没连接就返回 offline。push 汇总所有 gateway 的 result，得出"哪些 user 真的没在线"，再 produce 到 toOfflinePush。

#### (f) Redis 是消息热缓存事实源
msgtransfer step 3 把 message 写 Redis cache（`msgCache.SetMessageBySeqs`）。客户端拉历史消息 `PullMessageBySeqs` 优先从 Redis 读，miss 才回 MongoDB。**MongoDB 不是热路径**。

---

## 2. 现状 vs OpenIM 差异

### 2.1 现 agents_im 设计假设
- PostgreSQL 是事实源；
- `message_outbox` 表 + `message` 表同 tx 写入保证可靠；
- seq 在 PG 里以 `(conversation_id, seq)` 唯一约束保证单调（`POSTGRES_MESSAGE` 表）；
- outbox-publisher 轮询 outbox → Kafka；
- transfer 消费 Kafka → 调 gateway。

### 2.2 OpenIM 设计假设
- Kafka 是事实源（消息只要进了 Kafka 就视为成功）；
- Redis 是 seq 仲裁者 + 热消息缓存；
- MongoDB 是异步归档；
- 无 outbox、无跨存储事务。

### 2.3 两者优劣对比

| 维度                       | agents_im 现状（outbox + PG）            | OpenIM（无 outbox + Kafka 主）            |
|----------------------------|------------------------------------------|-------------------------------------------|
| 写延迟                     | 高：PG write + outbox write 同 tx        | 低：proto.Marshal + Kafka send            |
| 客户端 ACK 语义            | "持久化成功"                              | "Kafka 接受成功"                          |
| Kafka 不可用时             | SendMsg 仍成功（outbox 兜底）            | SendMsg 直接失败                          |
| PG 不可用时                | SendMsg 直接失败                          | SendMsg 仍成功（异步入 Mongo）            |
| seq 假设                   | 同 tx 分配（每写一条 select max+1）       | Redis Malloc 批量原子                     |
| 大群消息吞吐               | PG 单点写瓶颈                             | Kafka partition + batcher 横扩            |
| 实现复杂度                 | 高（outbox publisher、cleanup、lock 字段） | 低（无 outbox）                           |
| 消息丢失风险               | 几乎零（PG 持久化）                       | Kafka acks=all 时极低，集群故障可能丢一批 |
| 已读 / 撤回 / 历史 query   | PG SQL 自然支持                           | 需要 Redis cache + MongoDB 异步查         |

### 2.4 outbox 弃用决策

**结论：弃用**。理由：

1. **OpenIM 多年生产验证**：百万级在线、千万级日活的实例没用 outbox，依靠 Kafka acks=all + 幂等去重就够。
2. **outbox 的最大价值"避免双写问题"在我们这里不成立**：当前 outbox 模式仍然是 PG → Kafka 双向最终一致，并没有避免双写——只是把双写延迟到 publisher。
3. **outbox 实现负担重**：
   - `message_outbox` 表：locked_by、locked_until、next_attempt_at、attempt_count 等 9 个字段；
   - publisher 进程要做 row lock + retry + cleanup；
   - 监控指标多了一项（outbox_pending）；
   - 文档 06 XC-2 提到 repository 平铺，outbox 占 4 个文件。
4. **agents_im 体量**：当前是 MVP，无需 outbox 这种为了"超低消息丢失率"的复杂度。Kafka acks=all + commit-after-write 已经够。
5. **保留 PG 作为最终持久化**：跟 OpenIM 用 MongoDB 不同，我们仍然写 PostgreSQL（已有 schema），只是改为 transfer 异步写。

> 注：弃用 outbox **不等于** 弃用 PostgreSQL。PG 仍然是消息归档，但写入路径从"同 tx"改为"transfer 消费 toPg topic 异步写"。

---

## 3. 目标架构（agents_im 版）

按 OpenIM 模型 + 适配本项目（PostgreSQL 替代 MongoDB；Redpanda 替代 Kafka，协议兼容）。

```
                      ┌──────────────────┐
                      │   Client (web)   │
                      └────────┬─────────┘
                               │ WebSocket
                               ▼
                      ┌──────────────────────┐
                      │  msggateway (cmd/    │
                      │  msggateway)         │
                      │ - ws conn + auth     │
                      │ - 注册 gRPC server   │
                      │   GatewayService     │
                      │ - 上行 → msg-rpc      │
                      └────────┬─────────────┘
                               │ gRPC SendMessage
                               ▼
                      ┌──────────────────────┐
                      │  msg-rpc             │
                      │  SendMessage =       │
                      │  producer.Publish    │
                      │    (key=convID)      │
                      └────────┬─────────────┘
                               │ Kafka topic: msg.toTransfer.v1
                               ▼
              ┌─────────────────────────────────────────┐
              │  msgtransfer                            │
              │                                          │
              │  Consumer toTransfer:                    │
              │    batcher(500/50/100ms,                 │
              │            shard by convID hash)         │
              │    per batch:                            │
              │      Malloc seq (Redis SeqConv)          │
              │      Write Redis MsgCache                │
              │      Write Redis ReadSeq                 │
              │      Publish msg.toPostgres.v1           │
              │      Publish msg.toPush.v1 (per msg)     │
              │    commit offset                         │
              │                                          │
              │  Consumer toPostgres:                    │
              │    批量 insert into messages             │
              │  Consumer (async): HasRead → PG          │
              └──────────────────┬───────────────────────┘
                                 │
            ┌────────────────────┴───────────────┐
            │ Kafka: msg.toPush.v1               │ Kafka: msg.toPostgres.v1
            ▼                                    ▼
   ┌────────────────────┐                ┌────────────────┐
   │  push              │                │  PostgreSQL    │
   │                    │                │  messages      │
   │  Consumer toPush:  │                │  (archive)     │
   │    online cache    │                └────────────────┘
   │    GetConns(GW)    │
   │    broadcast gRPC  │
   │    → all gateways  │
   │    offline → MQ    │
   │      toOfflinePush │
   │                    │
   │  Consumer          │
   │    toOfflinePush:  │
   │    FCM / APNs /    │
   │    Getui ...       │
   │                    │
   │  + gRPC server     │
   │    PushService     │
   └────┬───────────────┘
        │ gRPC broadcast
        ▼
   msggateway 实例 1..N
   各自查 local conn map
```

### 3.1 进程清单

| 进程（Makefile 服务名） | main 包路径               | 主要职责                                                |
|-------------------|---------------------------|--------------------------------------------------------|
| `msggateway`      | `service/msggateway`      | WebSocket 长连接 + 注册 gRPC GatewayService            |
| `msg-api`         | `service/msg/api`         | HTTP 入口（mobile / web）→ 调 msg-rpc gRPC          |
| `msg-rpc`         | `service/msg/rpc`         | 仅 publish 到 `msg.toTransfer.v1`，**不写 DB**          |
| `msgtransfer`     | `service/msgtransfer`     | 消费 toTransfer：Redis seq/cache + produce toPostgres/toPush；消费 toPostgres：批量入 PG |
| `push`            | `service/push`            | 消费 toPush：online 广播 + offline produce；消费 toOfflinePush：第三方推送 |
| ~~outbox-publisher~~ | ~~删除~~              | ~~不再需要~~                                            |

### 3.2 Kafka topics

| Topic                  | Producer         | Consumer        | Partition Key     | 用途              |
|------------------------|------------------|-----------------|-------------------|-------------------|
| `msg.toTransfer.v1`    | msg-rpc          | msgtransfer    | conversation_id   | 入口流，保证会话顺序 |
| `msg.toPostgres.v1`    | msgtransfer     | msgtransfer    | conversation_id   | 批量入 PG 归档     |
| `msg.toPush.v1`        | msgtransfer     | push            | user_id（或 convID） | 推送 fan-out      |
| `msg.toOfflinePush.v1` | push             | push            | user_id           | 二段式离线推送     |

Kafka producer 配置：
- `acks=all`；
- `enable.idempotence=true`；
- `retries=Integer.MAX`；
- `max.in.flight.requests.per.connection=5`。

### 3.3 Redis 数据结构

| Key                                         | 类型 | 用途                          | TTL    |
|---------------------------------------------|------|-------------------------------|--------|
| `msg:seq:conv:{conversation_id}`            | INT  | 会话级递增 seq（Malloc 用 INCRBY） | ∞      |
| `msg:cache:conv:{conversation_id}:{seq/100}` | HASH | 100 条一个 bucket 的消息缓存   | 24h    |
| `msg:hasread:conv:{conversation_id}:user:{user_id}` | INT | 用户的已读 seq                | ∞      |
| `presence:user:{user_id}`                   | HASH | 用户在线状态（gateway_id → conn_id）| 60s（heartbeat 刷新）|
| `presence:gateway:{instance_id}`            | SET  | gateway 实例已知的 user list  | 60s    |

> Redis 是消息系统**主存**之一，必须做 Sentinel 或 Cluster。当前部署是单 Redis，需要补冗余。

### 3.4 PostgreSQL 角色变化

| 当前                                        | 重构后                                  |
|---------------------------------------------|----------------------------------------|
| `messages` 表：写入路径主源                   | `messages` 表：仅作为归档，msgtransfer 异步写入 |
| `message_outbox` 表：outbox 事件             | **删除**                                |
| `conversations` 表：会话 + 当前 max_seq      | 保留，但 max_seq 字段不再权威；Redis 才是 |
| `delivery_attempts` 表                       | 保留：push 服务记录每次 push 结果       |

历史拉取（`pull_messages`）：
- 客户端按 seq 区间拉；
- service 先查 Redis 缓存（最近 24h 的消息）；
- miss 再查 PG。

---

## 4. msg-rpc 的精简

### 4.1 当前职责（重构前）
`internal/logic/message/`、`internal/repository/postgres_message.go` 现在做了：
1. 校验 sender / conversation / chat type；
2. 生成 server_msg_id（Snowflake）；
3. PG tx：写 messages 行 + 写 message_outbox 行 + 推进 conversations.max_seq；
4. 返回 ACK + seq。

### 4.2 重构后职责
1. 校验 sender / conversation / chat type；
2. 生成 server_msg_id（Snowflake）；
3. `producer.Publish(topic=msg.toTransfer.v1, key=conversation_id, value=proto.Marshal(MsgData))`；
4. 返回 ACK（**不带 seq**，seq 由 transfer 分配后通过 push event 回传客户端）。

> ⚠️ **API 契约变化**：当前客户端期望 SendMessage 同步返回 `seq`。重构后 seq 是异步的——客户端通过 push event `message_received`（针对自己也推一份）拿到自己发的消息的 seq。这与 OpenIM SDK 行为一致：本地先以 client_msg_id 占位渲染，收到 server push 后用 server_msg_id + seq 替换。

### 4.3 幂等
client_msg_id + sender_id 是天然幂等键。msgtransfer 在分配 seq 之前要查 Redis `msg:dedup:{client_msg_id}`，存在则跳过分配、复用旧 seq。dedup key TTL 7 天。

---

## 5. msgtransfer 的核心实现

### 5.1 batcher 关键参数（直接抄 OpenIM 默认）
```go
const (
    size              = 500              // 每 worker 一批最多 500 条
    mainDataBuffer    = 500              // 主 channel buffer
    subChanBuffer     = 50               // 每 worker channel buffer
    worker            = 50               // worker 并发
    interval          = 100 * time.Millisecond  // 强制 flush 周期
    hasReadChanBuffer = 1000
)
b.Sharding = func(key string) int {
    return int(stringutil.GetHashCode(key)) % worker
}
b.Key = func(m *ConsumerMessage) string {
    return m.Key  // conversation_id
}
```

> ⚠️ **同 conversation 永远落到同一 worker**——这是 seq 单调和 batch 顺序的物理基础。改 worker 数会破坏分片，需要 stop-the-world 重启。

### 5.2 每个 batch 的 do 函数
```go
func (h *Handler) do(ctx context.Context, channelID int, val *batcher.Msg[ConsumerMessage]) {
    msgs := h.parseConsumerMessages(ctx, val.Val())
    h.doSetReadSeq(ctx, msgs)              // 处理 read receipt 消息
    storageMsgs, notStorageMsgs, _, _ := h.categorize(msgs)

    convID := getConversationID(msgs[0])
    h.handleMsg(ctx, val.Key(), convID, storageMsgs, notStorageMsgs)
    //  ├─ for notStorageMsgs: only MsgToPushMQ
    //  └─ for storageMsgs:
    //       BatchInsertChat2Cache  → Redis seq Malloc + msg cache
    //       SetHasReadSeqs(sender)  → Redis sender's own read marker
    //       MsgToPostgresMQ        → produce toPostgres
    //       MsgToPushMQ            → produce toPush per-msg
}
```

### 5.3 与现 agents_im 的关键调整

| 项                                | OpenIM 实现                           | 本项目调整                                      |
|----------------------------------|---------------------------------------|------------------------------------------------|
| 持久化目标                       | MongoDB                               | PostgreSQL（保留现有 schema）                  |
| `BatchInsertChat2Cache` 后行为   | 写 Redis、produce toMongo、produce toPush | 写 Redis、produce toPostgres、produce toPush  |
| seq cache 实现                   | `seqConversationCacheRedis`           | 同上，复用 `pkg/idgen`（00-decisions D10）或新建 |
| message cache                    | `msgCache.SetMessageBySeqs`，bucket 100 | 同上                                          |
| Agent 触发                       | 无（OpenIM 不带 Agent）               | 在 categorize 后增加分支：判断是否要触发 Agent，produce `agent.trigger.v1` topic（见文档 04） |

---

## 6. push 服务的实现

### 6.1 在线推送（消费 toPush）
```go
func (c *Handler) HandleMs2PsChat(ctx context.Context, msg []byte) {
    pushReq := &PushMsgReq{}; proto.Unmarshal(msg, pushReq)

    switch pushReq.MsgData.SessionType {
    case ChatTypeGroup:
        c.Push2Group(ctx, pushReq.MsgData.GroupID, pushReq.MsgData)
    default:
        c.Push2User(ctx, []string{pushReq.MsgData.RecvID, pushReq.MsgData.SendID}, pushReq.MsgData)
    }
}

func (c *Handler) Push2User(ctx, userIDs, msg) {
    wsResults := c.GetConnsAndOnlinePush(ctx, msg, userIDs)  // 广播 all gateways

    // 检查 wsResults 找出真正没在线的 user
    var offlineUsers []string
    for _, uid := range userIDs {
        if !anyResultDeliveredFor(uid, wsResults) {
            offlineUsers = append(offlineUsers, uid)
        }
    }
    if len(offlineUsers) > 0 {
        c.producerToOfflinePush.Publish(ctx, offlineUsers, msg)  // 二段式
    }
}
```

### 6.2 广播 gateway
```go
conns, _ := disCov.GetConns(ctx, "msggateway")  // service discovery
for _, conn := range conns {
    wg.Go(func() error {
        client := gatewaypb.NewGatewayServiceClient(conn)
        reply, err := client.BatchPushOneMsg(ctx, &BatchPushReq{
            MsgData:       msg,
            PushToUserIDs: userIDs,
        })
        // reply 包含 per-user 投递结果（delivered / no-conn）
        ...
    })
}
```

⚠️ **广播 vs per-instance 路由的取舍**：广播简单但浪费（每条消息 N 次 RPC，N=gateway 实例数）。当 N>20 才考虑改为 per-instance（presence 查路由）。本项目当前 N=1，未来近期 N<=5，**广播足够**。

### 6.3 离线推送（消费 toOfflinePush）
```go
func (o *OfflineHandler) HandleMsg2OfflinePush(ctx, data) {
    req := &PushMsgReq{}; proto.Unmarshal(data, req)
    o.offlinePusher.Push(ctx, req.UserIDs, req.MsgData)
    // offlinePusher: FCM / APNs / Getui / JPush adapter
}
```

第一版 agents_im 可以**只实现 FCM 一种 channel**（甚至先 noop / 写 audit 表），后续按需扩。

---

## 7. msggateway 的精简

### 7.1 当前 (`internal/msggateway/server.go`) 的问题
- msggateway 直接 import `internal/logic/message`、`internal/repository`、`internal/agentim`、`internal/auth/repository`；
- WebSocket command 路由（send_message / pull_messages / mark_read / get_seqs）直接调 in-process logic；
- gateway 进程要装载所有 DB / Agent 依赖。

### 7.2 重构后
msggateway 只持有：
- WebSocket 连接管理（complaint to OpenIM `client.go`、`ws_server.go`）；
- 三个 gRPC client：msg-rpc、user-rpc、auth-rpc（JWT 解析也行直接库）；
- 一个 gRPC server：`GatewayService.BatchPushOneMsg` 给 push 服务调用。

command 路由：
- `send_message` → 调 msg-rpc gRPC SendMessage；
- `pull_messages` → 调 msg-rpc gRPC PullMessages；
- `mark_conversation_read` → 调 msg-rpc gRPC MarkRead；
- `get_conversation_seqs` → 调 msg-rpc gRPC GetMaxSeqs；
- `heartbeat` → 本地刷新 last-seen。

**上行 ACK 是同步透传**（用户强调的「收到即回已发送」）：gateway 的 `send_message` handler 同步调 msg-rpc `SendMessage`，把返回的 `SendMsgResp{server_msg_id, client_msg_id, send_time}`（**无 seq**）原样回写 websocket，即客户端看到的「已发送」状态。gateway 本身**不碰 DB、不分配 seq、不做业务校验**（业务校验在 msg-rpc，对齐 OpenIM `msggateway/message_handler.go:GrpcHandler.SendMessage`——它只 `Unmarshal → 调 MsgClient.SendMsg → Marshal 回写`）。该 ACK 只代表「Kafka 已接受」，**不代表持久化、不代表已分配 seq**；seq 与「送达对端」由后续 push event 异步带回（§4.3、§10）。

msg-rpc 拿到读类请求（pull / seqs / mark_read）后，**仍然走 Redis 优先 + PG 兜底**，不需要新走 Kafka。Kafka 只是写路径。

### 7.3 不要 presence Redis 写
OpenIM 的 msggateway 本地维护 `userMap`（`user_map.go`），不依赖 Redis 保存 connection。online status 通过周期心跳推到 user-service 做记录（`internal/msggateway/online.go`）。

本项目当前的 `internal/presence/{memory,redis}.go` 设计成了 Redis 主存——这反而是过度设计。改为：

- 每个 gateway 实例本地 connection manager（已有）；
- online status 周期推到 user-rpc；
- presence Redis **可保留**做"用户是否任意端在线"的快速查询，但 push 不依赖它做路由（路由靠广播）。

---

## 8. 顺序保证与可靠性

### 8.1 消息顺序
1. **Kafka partition key = conversation_id** → 同会话进入 Kafka 时已经按发送顺序排好。
2. **msgtransfer batcher 按 conv hash 分片到固定 worker** → 同会话 batch 内顺序与 Kafka 顺序一致。
3. **seq Malloc 是 Redis 原子 INCRBY** → 严格单调。
4. **push 出去之后到达客户端的顺序**：受网络/客户端处理影响。**客户端必须按 seq 排序**（已在 `docs/RELIABILITY.md` 写过）。

### 8.2 可靠性等级

| 故障                          | 行为                                            |
|-------------------------------|------------------------------------------------|
| msg-rpc 进程挂                | Kubernetes 重启；新请求转移到其它副本           |
| Kafka broker 全挂             | SendMessage 失败（acks=all）；客户端看到错误重试 |
| msgtransfer 进程挂           | Kafka offset 没 commit，重启后从上次位置继续；Redis cache 已写的会重复写（幂等：HSET 覆盖） |
| Redis 主挂                    | 灾难；需要 Sentinel/Cluster 切主                |
| PostgreSQL 挂                 | toPostgres consumer 重试堆积；写 SendMessage 仍 OK；恢复后回放 |
| push 进程挂                   | Kafka 暂存 toPush；恢复后继续；可能少量延迟    |
| msggateway 实例挂             | 该实例上的 conn 断；客户端重连其它实例；presence TTL 60s 自动清理 |

### 8.3 幂等
- client_msg_id 在 Redis dedup key（7d TTL）；
- server_msg_id（Snowflake）在 msg-rpc 生成，保证全局唯一；
- push 重发对客户端是 by-design 可接受（客户端按 server_msg_id 去重）；
- offline push 重发要看第三方通道，FCM 自带 message_id 去重。

---

## 9. 分阶段实施（2026-06-10 重排版，Issue #462）

> 按仓库当前进度重排：msg-rpc（#458 PR1）/ msg-api（#457 PR2）已 additive 上线且**行为对齐旧实现**（SendMessage 仍同步写 PG + outbox，ACK 带 seq）。与原版 Phase 0~6 的两个关键差异：
>
> 1. **gateway 切 gRPC 提前到 Kafka 化之前**（A3）。原版把 gateway 放最后，前提是 msg-rpc 一上来就只发 Kafka；现在 msg-rpc 行为对齐旧实现，提前切对客户端零感知，且提前解锁 keystone（删 `internal/` 消息域，§0.1）。
> 2. **outbox 从「Phase 0 先删」改为「B3 最后删」**。`outbox → outboxpublisher → Kafka → message-transfer` 是当前活路径，先删会断链；必须等新链路灰度稳定后退役。
>
> 每个编号 ≈ 一个 Issue/PR。依赖：A1→A2，A3→A4，B0/B1→B2→B3，C1→C2→C3；阶段 A 与 B0/B1 可并行，C1 可与阶段 B 并行，阶段 D 在 B3 后解锁。

### 阶段 A：去双轨（行为完全不变，低风险，先做）

| 步骤 | 内容 | 备注 |
|------|------|------|
| ~~A1+A2~~（#463，✅ 已合并执行） | msg-api 承接 message-api **全部** HTTP 职责并切流退休：ai-hosting 加 2 个 RPC 落 **msg-rpc**（非 agent 域——勘探发现 web 走 REST 发消息，AI 托管触发钩子必须随 message-api 退役迁入 msg-rpc SendMessage，hosting runtime 反正已在，注明待 B1 迁 msgtransfer / agent 域 rpc）；feedback 落 admin-rpc CreateFeedback；ingress `/messages` + `/conversations` + `/api/feedback` → msg-api:8090；删 `service/message-api` + dormant `internal/rpcgen/message` + `internal/handler` + `internal/logic/message` | 回滚 = ingress 切回（需先 revert 删除）；AI 触发语义对齐原进程内钩子（含 dedup 触发、错误只记日志） |
| A3 | gateway-ws 4 个 ws command（send_message / pull_messages / mark_conversation_read / get_conversation_seqs）改调 msg-rpc gRPC，删 in-process 接线；随本 PR 改名 `service/msggateway` | ACK 语义不变（msg-rpc 仍同步写 PG 返回 seq），客户端零感知 |
| A4 | 删 `internal/logic` 消息域剩余（messagelogic 等）、`internal/repository` 消息部分、`internal/servicecontext/message` | keystone 解锁；删空顶层 `internal/` 的前置；注意 msg-rpc 的 AI 托管 runtime 也消费 `internal/servicecontext/message`，A4 需与触发点迁移（B1）协同 |

阶段 A 完成后链路：client → msggateway / msg-api → msg-rpc（gRPC）→ PG + outbox → publisher → Kafka → transfer → gateway HTTP。**单轨**，后续改写路径只动 msg-rpc / transfer 两处（internal 消息域残留 = gateway-ws in-process + AI 托管 runtime）。

### 阶段 B：写路径 Kafka 化（原 Phase 0+1+2，核心风险区）

| 步骤 | 内容 | 备注 |
|------|------|------|
| B0（✅ #470 落地，按单节点现实裁剪） | Redis HA（Sentinel / Cluster）+ Redpanda acks=all 实测（p99 < 50ms，§12.1） | **硬前置**：seq 仲裁者迁 Redis 后，Redis 单点挂 = 全链路停摆（§11）。可与阶段 A 并行。**落地偏差**：生产是单物理节点 k3s（4C/8G），Sentinel/3-broker 与应用同故障域、无真实容错价值且内存不允许——Redpanda 单 broker（`deploy/k8s/middleware/redpanda.yaml`，--memory 400M，非 dev 模式保 fsync）、Redis 维持单实例 AOF；seq 抗 Redis 丢失靠 B1 的"Redis miss 时从 PG max(seq) 初始化"+ 唯一约束告警兜底。选型/延迟实测数据见 Issue #470 |
| B1（✅ #474 落地） | msgtransfer 新消费链路（additive、dormant）：batcher（500/50/100ms，conv hash 分片，§5.1）+ Redis seq Malloc + msgCache + toPostgres consumer 批量写 PG，消费 `msg.toTransfer.v1`（此时无人生产）；categorize 内含 Agent trigger（produce `agent.trigger.v1`，§10 / 文档 04 §4.2）；随本 PR 改名 `service/msgtransfer` | 复刻 additive 模式：先建下游，零流量零风险。**落地偏差**：(1) batcher 用 poll-batch barrier（按 conv 分组并行处理一个拉取批、全部完成才 commit offsets，franz-go 手动提交）替代 OpenIM 自由流 batcher——单实例低流量等价，at-least-once 重放靠 Redis dedup 收敛（链路集成测试覆盖）；(2) `agent.trigger.v1` 对每条 storage 消息无条件 produce，hosting/recursion 判定留在消费方（B2 落 msg-rpc runtime，与现 fireMessageCreatedHook 语义对齐），04 落地时再前移判定；(3) **新链路 push fanout 始终含 sender**（ACK 无 seq 后 sender 靠自己的 message_received 回填占位，§4.2）；(4) toPush 经 `KafkaPushConsumer` 适配现有 transfer.Worker + gateway HTTP dispatcher（C2 才拆 push 服务）；(5) seq 计数器 Redis miss 时从 PG max(messages.seq) 播种，(conversation_id,seq) 唯一冲突 = seq 回退告警（persist consumer 日志含恢复指引），不静默吞 |
| B2 | msg-rpc SendMessage 加 feature flag `MSG_DIRECT_KAFKA`：MsgToMQ 只发 Kafka、不写 PG、ACK 去 seq；前端 SDK 以 client_msg_id 占位渲染（§4.2 / §7.2） | **唯一契约破坏点**，集中在一个后端 PR + 一个前端 PR；flag 切回 = 秒级回滚 |
| B3 | 灰度切流 → 观察（consumer lag / seq 连续性 / PG 异步写延迟）→ 退役旧路径：删 `internal/outboxpublisher/`、`internal/transfer/outbox_consumer.go`、`internal/repository/{message_outbox_repository,postgres_outbox}.go`、msg-rpc 内 PG 写与 seq 分配代码；migration drop `message_outbox`（表数据保留 90 天观察）；更新 ARCHITECTURE.md "Message Pipeline" | 原 Phase 0 的清理动作全部移到这里 |

### 阶段 C：拆 push 进程（原 Phase 3 + Phase 4 后半）

| 步骤 | 内容 | 备注 |
|------|------|------|
| C1 | msggateway 注册 gRPC `GatewayService.BatchPushOneMsg`（additive，暂无人调） | 可与阶段 B 并行 |
| C2 | 新建 `service/push`：消费 `msg.toPush.v1`，广播所有 gateway gRPC（§6）；msgtransfer 改 produce toPush；删 `internal/transfer/gateway_http_dispatcher.go`、`internal/transfer/gateway/` | net-new 部署单元，按 [`refactor-domain-to-service` skill] 配套清单串 Dockerfile / drone-build / deploy-k3s / detect-deploy / deployments+services / kustomization / verify 全链 |
| C3 | 二段式离线推送：`msg.toOfflinePush.v1` producer/consumer + FCM adapter（第一版可 noop / 写 audit 表，§6.3） | 独立小 PR |

### 阶段 D：读路径 + 新增 RPC（07 §8 Phase 2+3，B3 后解锁）

- D1：PullMessages / GetConversationsSeqState 改 Redis cache 优先 + PG 兜底；拆出 GetMaxSeqs / GetHasReadSeqs；
- D2+：RevokeMessage、AppendStreamMessage（Agent 流式依赖）、GetLastMessageByConvs、DeleteMessages / ClearConversationMessages，按需逐个 PR（见 07 §8 Phase 3）。

### 阶段 E（贯穿并行）：监控与可观测

- 指标随 B/C 各自 PR 带上：
  - `kafka_consumer_lag{topic,group}`
  - `msg_transfer_batch_size`
  - `msg_transfer_seq_malloc_duration`
  - `push_online_delivered_total{result}`
  - `push_offline_pushed_total{channel,result}`
- Loki / Tempo 链 trace_id（trace_id 通过 Kafka headers 传递）。

### 远期（原 Phase 6，可选）：MongoDB 归档

若 PG 在大消息量下扛不住，可以增加 toMongo topic 写 MongoDB 做归档；这是 P3，不阻塞。

---

## 10. 对 04-agent.md 的反向影响

Agent 触发链路（文档 04 §4.2）改为：

```
msgtransfer 在 categorize 阶段判断是否需要触发 Agent：
  - conversation 是否配置 hosting；
  - sender 是否为 user（非 ai）；
  - 消息是否 @ Agent。
若需要 → produce `agent.trigger.v1` topic。

agent-rpc 消费 agent.trigger.v1 → 跑 RunOrchestrator → 写回 IM（调 msg-rpc SendMessage）。
AI 写回的消息再次走完整链路（msg-rpc → toTransfer → transfer → toPush → push），
不会形成无限循环，因为：
  1. transfer categorize 阶段会用 hostingService 检查 recursion policy；
  2. trigger event 带 source_agent_run_id，幂等去重。
```

这与原文档 04 §4.2 一致，唯一变更是触发点从"msg-rpc in-process hook"前移到"transfer batch handler"。

---

## 11. 风险与回滚

| 风险                                        | 应对                                             |
|---------------------------------------------|--------------------------------------------------|
| B2 ACK 语义变化，前端报错                   | feature flag 灰度；前端在 SDK 用 client_msg_id 占位渲染（OpenIM SDK 已有模式可抄）|
| Kafka 不可用导致 SendMessage 失败           | 生产前必须 Redpanda 3-broker 集群 + acks=all；监控 producer error rate |
| Redis 单点导致 seq 不可用                   | B0 上 Sentinel 是硬前置                           |
| 消息漂移到错误 partition（key 错误）         | 部署前 e2e 测试覆盖：单聊、群聊、self-send、转发等 |
| msgtransfer batcher 重启丢内存中未 flush 批 | Kafka offset 在 OnComplete 才 commit；崩了从上次 offset 重放；幂等保证不重 |
| 现有 outbox 数据如何处理                    | B3 不立即 DROP 表；保留 90 天观察                |

---

## 12. 待确认问题

1. **Redpanda producer acks=all 是否会显著拖慢 SendMessage？** 需要在 §9 B0 实测，目标 p99 < 50ms。
2. **client_msg_id 来源**：当前由客户端生成？还是 sdk wrapper？要 audit `web/src/api/`、native sdk。
3. **大群（>1000 人）push 广播性能**：OpenIM 文档建议群消息走 `Push2Group` 拆分 → 多个 push 任务；本项目第一版可以接受全员 fan-out，超过 500 人 group 再说。
4. **PostgreSQL 异步写延迟**：transfer 批量写 PG 的 lag 多久可接受？OpenIM 通常允许 Mongo lag 几秒，本项目同口径。
5. **Agent trigger 是否也用 transfer 内 inline 判断**：见文档 04 §4.2，建议引入 `agent.trigger.v1` topic 而不是 in-process。

---

## 13. 与原版 03 的差异（备忘）

原版（outbox + "msg-rpc → outbox → transfer → push"）已被本文件整体覆盖；关键差异（outbox 弃用、msg-rpc 不写 PG、seq 在 transfer Redis Malloc、push 广播 + 二段式离线、gateway 纯连接层）见头部「重写说明」与 §0/§2.4，不在此重复。
