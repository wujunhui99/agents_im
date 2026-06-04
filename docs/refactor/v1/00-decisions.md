# 00 — 跨文档决策同步表

> 六份重构文档（01~06）独立写作后，存在一些跨文档冲突点（主要源于 03 在阅读 OpenIM 源码后重写，决策没回灌到其它文档）。本文件**是跨文档冲突的唯一仲裁源**：任何 01~06 中提到与本文件冲突的描述，以本文件为准。
>
> 后续修改要点：**每次改动 D1~D9 任一决策，必须同步更新引用方文档**。引用方按本文末 §4 一致性矩阵核对。

---

## 1. 锁定决策（D1 ~ D9）

### D1 — Outbox 模式弃用 ❌
- **不再使用** `message_outbox` 表 / `outboxpublisher` 包 / `outbox_consumer`。
- msg-rpc 直接 `producer.Publish(msg.toTransfer.v1)`，不写 PG，不写 outbox。
- 来源：03 §0 + §2.4。OpenIM 实测：`internal/rpc/msg/send.go:81` 只调 `MsgToMQ`。
- **影响文档**：01（repository 拆分）、04（agent 触发链路图）、05（监控指标）、06（repository 列表）。

### D2 — seq 生成位置：msgtransfer（不在 msg-rpc）
- seq 由 msgtransfer batcher 用 Redis `seqConversation.Malloc(convID, len)` 原子分配。
- 单调性保证靠 Kafka partition key=conversation_id + batcher 按 convID hash 固定 worker。
- msg-rpc 不再生成 seq，**SendMessage 的 ACK 不返回 seq**——客户端用 client_msg_id 占位，收到 push event 后用 server_msg_id + seq 替换。
- 来源：03 §1.1(b) + §4.2 + §11。
- **影响文档**：02（friends/groups 类似 Kafka event 描述）、04（agent 写回 IM）、06（API 契约变化）。

### D3 — 服务命名规则：msg* 家族（方案 A，对齐 open-im-server）

消息链路所有服务采用 OpenIM 的扁平 `msg*` 前缀命名，**不**设 `service/message/` 父目录、**不**保留 `-ws` 后缀。

| 服务                 | 包路径                | 状态   | 备注                                |
|----------------------|-----------------------|--------|-------------------------------------|
| `msg-api`            | `service/msg/api`     | 重命名 | 原 `message-api`                    |
| `msg-rpc`            | `service/msg/rpc`     | 重命名 | 原 `message-rpc`；D8 已用此名         |
| `msggateway`         | `service/msggateway`  | 重命名 | 原 `gateway-ws`，去掉 `-ws`          |
| `msgtransfer`        | `service/msgtransfer` | 重命名 | 原 `message-transfer`               |
| `push`               | `service/push`        | 新增   | 跨域投递，不带 `msg` 前缀（同 OpenIM） |
| `admin-api`          | `service/admin/api`   | 新增   | 见文档 01 §4 收敛步骤 5             |
| `agent-rpc`          | `service/agent/rpc`   | 新增   | 见文档 04 §3.1                      |
| ~~outbox-publisher~~ | —                     | 不创建 | D1 决策；任何文档提到视为错误         |

> 命名风格：消息域本体走 `<domain>-<role>`（`msg-api` / `msg-rpc`，同 `auth-api`/`auth-rpc`）；无 api/rpc 二分的单体服务走紧凑前缀（`msggateway` / `msgtransfer`，无连字符，同 OpenIM 目录）。

**为什么选扁平 msg* 家族（方案 A），而非 `service/message/{api,rpc,gateway,transfer,push}` 嵌套：**
- **对齐姊妹仓 open-im-server**：本后端基于 OpenIM，其布局即 `msggateway` / `msgtransfer` / `push` + `rpc/msg`，无 `message` 父目录。同名 → porting 修复、对照逻辑、onboarding 成本最低。
- **消除 `message` 与 `msggateway` 命名打架**：父目录 `message/` + `msg*` 前缀是两套互斥分组手段，混用才"分不清"。扁平方案只靠前缀分组，`message` 这个词不出现，歧义消失。
- **保住服务边界（TD-1）**：msggateway / push 是跨域传输/投递设施（gateway 还扛 presence/typing/receipt），嵌进 `message/` 会暗示它们属消息域，重演 admin 寄生 msg-api 的反模式。
- **gateway 名字够具体**：扁平下 `msggateway` 前缀即语境，无需也不应再叠 `message/` 父目录（否则 `message/msggateway` 结巴）。

### D4 — presence 的角色：仅在线状态查询，不做 push 路由
- presence Redis 保留 `presence:user:{user_id}`（HASH，TTL 60s），表示"用户是否在线"，供 user/group 详情页快查。
- **push 不通过 presence 查路由**。push 通过 service discovery 拿到所有 gateway 实例，并发广播 gRPC；每个 gateway 看本地连接表，命中即推送。
- presence 中**不存** gateway_grpc_addr、不存 connection routing。
- 来源：03 §6.2 + §7.3。OpenIM 实测：`internal/push/onlinepusher.go:69` 是广播。
- **影响文档**：05（msggateway 部署要求）。

### D5 — Kafka topic / consumer group 命名

| Topic                  | Producer       | Consumer      | Key             | Group              |
|------------------------|----------------|---------------|-----------------|--------------------|
| `msg.toTransfer.v1`    | msg-rpc        | msgtransfer  | conversation_id | `msgtransfer`     |
| `msg.toPostgres.v1`    | msgtransfer   | msgtransfer  | conversation_id | `msg-to-postgres`  |
| `msg.toPush.v1`        | msgtransfer   | push          | user_id         | `push-online`      |
| `msg.toOfflinePush.v1` | push           | push          | user_id         | `push-offline`     |
| `agent.trigger.v1`     | msgtransfer   | agent-rpc     | conversation_id | `agent-trigger`    |

- 任何文档新增 topic 必须更新本表。
- 现有 `message.events.v1`（当前 outbox 用）在 D1 完成后即可弃用。

### D6 — Kafka wire format：`proto.Marshal(messagepb.MsgData)`
- 与 OpenIM 一致，Kafka topic value 是 protobuf 二进制（`messagepb.MsgData`），不是 JSON。
- 废弃：`internal/messaging.MessageEvent`（snake_case JSON envelope）+ `internal/transfer.MessageEvent`（camelCase JSON envelope）。它们解决了"两种格式互转"的伪问题，本身就是 D1 outbox 设计的副产品。
- 来源：03 §0 决策表。
- **影响文档**：旧 03 v1 中 MP-1 的"wire format 统一为 messaging.MessageEvent" → 已废弃。

### D7 — PostgreSQL 仍是消息归档（不是事实源）
- `messages` 表保留，但写入路径改为 msgtransfer 异步消费 `msg.toPostgres.v1` 批量入库。
- `messages` 不再是热路径数据源；客户端拉历史 → Redis cache（24h 窗口）→ miss 兜底 PG。
- conversation 的 `max_seq` 字段不再权威；Redis `msg:seq:conv:{id}` 才是。
- 来源：03 §3.4 表格。
- **影响文档**：02（user/group 服务的 conversation 边界）、04（agent imadapter 写回路径）。

### D8 — msggateway 是纯连接层
- msggateway 进程**禁止 import**：`service/<other>/internal/logic/*`、`service/<other>/internal/repository/*`、`service/agent/rpc/internal/*`、`service/auth/rpc/internal/credential/*`。
- msggateway 只能依赖：msg-rpc / user-rpc / auth-rpc gRPC client、`pkg/presence`（D4 限定）、`pkg/observability`、`pkg/idgen`、`pkg/apperror`、`pkg/ctxuser`、`pkg/response`。
- msggateway 注册 gRPC server `GatewayService.BatchPushOneMsg`，由 push 服务广播调用。
- 来源：03 §7.1/§7.2。
- **影响文档**：01（gateway 拆分目标）、04（agent 也不应该在 gateway 内运行）。

### D9 — 消息链路重构按文档 03 §9 的 Phase 0~5 顺序推进
- Phase 0：清 outbox 残骸（D1）。
- Phase 1：msg-rpc 改为只发 Kafka（D2 改 ACK 语义）。
- Phase 2：msgtransfer 实现 batcher + Redis Malloc（D2 + D5 + D6）。
- Phase 3：拆 push 进程（D3 + D4）。
- Phase 4：gateway 砍业务依赖（D8）。
- Phase 5：Redis HA + 监控。
- 其它文档（01/02/04/05/06）的 epic 顺序**应作为 Phase 内的子任务**，不应单独推进与 03 phase 冲突的变更（例：05 OB-3 hostNetwork 改 ClusterIP 需要在 Phase 3 之前或之后做都行，但不能跨 phase 同时进行）。

### D10 — 顶层 `internal/` / `api/` / `proto/` 退役，go-zero 风格目录
- **删除顶层 `internal/`**：所有内容收敛到下面两处之一：
  - `service/<domain>/<api|rpc>/internal/`：服务局部，goctl 生成的 config/handler/logic/svc/types/server 都在这里；
  - `pkg/`：跨服务可重用基础设施（apperror、response、ctxuser、config、observability、llmobs、health、idgen、messaging、presence、objectstorage、pythonexec）。
- **删除顶层 `api/`**：`.api` 文件下沉到 `service/<domain>/api/<domain>.api`（单源）。
- **删除顶层 `proto/`**：`.proto` 文件下沉到 `service/<domain>/rpc/<domain>.proto`（单源）。
- **删除顶层 `rpcgen/`**：原过渡产物，已被 service 内 pb 替代。
- **依赖方向**：`service → pkg` 单向；`pkg` 内禁止 import `service/...`；`pkg` 内禁止出现域名词（如 `pkg/friends/`）。
- 参考：[go-zero 官方目录结构](https://go-zero.dev/docs/tutorials/go-zero/quick-start/dir-structure)（`service/`、`pkg/`、`deploy/`），OpenIM 同样使用 `pkg/` 承载跨服务基础设施。**agents_im 不设顶层 `cmd/`**：每个服务的 `package main` 就是 goctl 生成的 `service/<domain>/<api|rpc>/<domain>.go`，启动/构建由根 `Makefile` 驱动（见 01 §3 入口约定）。
- 来源：01 §3 + §4（Stage 1~5）。
- **影响文档**：01 全文、02（CP-4 引用 internal/domain 改为 service domain）、03（pkg/messaging）、04（pkg/pythonexec、pkg/llmobs）、05（pkg/observability、metric 路径）、06（XC-1 repository 平铺最终归宿）、07（pkg/idgen 用于 server_msg_id 生成）。

### D11 — 重构执行策略：长期分支 / 逐服务原地重建 / 搬运不重写

- **长期重构分支**：所有重构改动进一条专用 refactor 分支，**中途不跑 Drone CI、不逐步回归**，允许过程中其它服务暂时编译断。**合回 `main` 前**必须本地 `go build ./... && go test ./...` 整体绿，并跑一次完整 CI + 回归——这是"搬运中没丢东西"的唯一机械验证，不可省。
- **逐服务原地重建（≠ 清空 `service/`）**：重建某服务时只重写它的 `rpc/internal/{config,svc,logic,model}`（model 由 goctl 生成，见 D10 / 01 TD-11），其它服务不动、照常编译。**不要一次性清空整个 `service/`**——那会删掉服务间 RPC 契约（pb stub），导致所有 RPC client 编译不过、全盘瘫痪、无法逐个验证。
- **proto 契约随服务下沉，不另存顶层 `proto/`**（对齐 D10）：迁某服务时把 `proto/<domain>.proto` **逐个 `git mv` 到 `service/<domain>/rpc/<domain>.proto`** 并就地 `goctl rpc protoc` 重生成 pb；`proto/` 搬空后删除整个目录。契约**不丢，只换位置**——下沉后的 proto/pb 仍是其它服务 RPC client 依赖的单源。
- **搬运不重写**：域逻辑从旧 `internal/logic` 等**移动 / 移植**（git 全程可回溯），**禁止从零重写**。问题是代码"放错位置"不是"写错了"；重写会丢失已验证的校验 / ACL / 边界与历史 bug 修复，成本与风险都远高于 `git mv` + 改 import。
- **RPC 契约解耦 → 任意顺序可重建**：跨域只依赖对方 proto 契约（svc 注入对方 rpc client，见 D12），故各服务可独立、任意顺序重建；半成品服务通过 RPC 调用尚未重建的旧服务（旧服务仍在运行）。这正是"改一个域不连锁改其它域逻辑"的根因——服务间只认 proto，不认代码。
- **顶层 `internal/` 最后删**：每个服务脱钩后清自己用到的 internal 子树；顶层 `internal/` 留到最后一个服务脱钩后统一删除（对齐 01 §4 Stage 5）。
- **起点**：auth（自包含，仅跨到 user）。
- **来源**：本轮 Claude × 用户讨论（2026-06-04）。
- **影响文档**：01（§4 收敛路径作为宏观清理目标，本 D11 给出"逐服务原地重建"的执行轨）。

### D12 — 跨域调用边界收紧：RPC 间一律不互调（含不经 adapter）

- **锁定 CP-3 为最严口径**：`rpc` 之间**一律不相互调用**，**也不经** `service/<domain>/rpc/internal/<other>adapter/` **间接调**；`rpc/internal` 不持有任何他域 rpc client。
- **跨域组合全部上移 API / BFF 层**：`service/<domain>/api` 自由 import 任意 rpc client 做聚合 / 编排。
- **取代（旧表述作废）**：02 CP-3 的"需要时显式建立 `<other>adapter`"逃生口、AU-3 的 `adapter/user.go` / `adapter/mail.go`、01 §3 约束里的"或新增 `service/<domain>/rpc/internal/<other>adapter/`"。
- **直接后果（建 auth 时落实）**：① 注册流程 auth 不再在 rpc 内调 user-rpc 建 account → 由 **auth-api 编排**（先 user-rpc 建 account，再 auth-rpc 建 credential）；② 发验证码邮件不在 auth-rpc 内调 mail-rpc → 由 **auth-api 编排**或走异步事件。
- **来源**：本轮 Claude × 用户讨论（2026-06-04），收紧 02 CP-3 的"建议"为锁定决策。
- **影响文档**：02（CP-3 修复、AU-3；FR-6 / §group list 聚合本就与此一致）、01（§3 约束、§6 验收 E）。

### D13 — 数据访问层：纯 goctl model，废除 repository 抽象层

- **数据访问 = goctl model**：每个域的数据访问由 `goctl model pg` 从 `db/migrations`（PostgreSQL）生成，落 `service/<domain>/rpc/internal/model/`（对齐 D10 / 01 TD-11）；自定义查询写在 model 的非生成文件（`<table>model.go` custom 区）。
- **不再设 repository 抽象层**：废除 `internal/repository` 风格的手写 repository（含 `{memory,postgres}` 双实现）。`svc.ServiceContext` 持有 model（+ DB conn / redis cache），`logic` 经 `svcCtx` 直接调 model。
- **测试**：放弃 memory-repo 单测 seam；数据层测试用 goctl model + sqlmock 或容器 / 嵌入 PostgreSQL；纯业务规则（校验 / ACL）测试不碰 DB，对 model 接口打桩。
- **取代（旧表述作废）**：01 TD-7 / §5 / §4 step 16 中"repository 下沉到 `service/<domain>/rpc/internal/repository/{memory,postgres}/`"、D10 映射"`internal/repository → service/<domain>/rpc/internal/repository/`"、06 XC-1 repository 归宿——全部改为 `service/<domain>/rpc/internal/model/`，**无 repository 层**。
- **来源**：本轮 Claude × 用户讨论（2026-06-04）。
- **影响文档**：01（TD-7、§3 约束、§4 step 16、§5）、02（list 聚合描述、AU-2）、04（AG-3 / AG-6 数据层）、06（XC-1）。

---

## 2. 已被本文件覆盖 / 失效的旧表述

| 文档 | 段落            | 旧表述（已失效）                                    | 取代它的 D 编号 |
|------|-----------------|----------------------------------------------------|-----------------|
| 01   | TD-7 行 72/75  | `repository/{...,outbox,...}` 拆分                  | D1              |
| 01   | §3 target 布局  | `cmd/message-transfer`（保持现状/不重命名）         | D3（改名 `service/msgtransfer`，无 cmd/） |
| 02   | §4.3 行 201    | "发 outbox 事件 `group.member.added/removed`"        | D1（改为 Kafka topic 事件） |
| 04   | §4.1 行 256     | "writes postgres + outbox"                          | D1 + D2         |
| 04   | §4.2 行 271~273 | "outbox publisher → Kafka `message.accepted.v1`"     | D1（直接发 Kafka） |
| 04   | §7 行 326       | "AI 消息是新的 outbox event"                         | D1（新的 Kafka 事件） |
| 05   | §5.3            | metric `message_outbox_pending`                     | D1（移除该指标） |
| 05   | 行 373          | "msggateway 必须配 presence 跨实例路由（见文档 03 D-2）" | D4（push 用广播，不查 presence） |
| 06   | XC-1            | repo 列表含 `message_outbox_repository.go` / `postgres_outbox.go` | D1（标"弃用待删"） |
| 06   | XC-2 行 79     | 目标分包含 `outbox/`                                | D1（移除）       |
| 01   | TD-7 §5 矩阵   | "拆到 `internal/repository/<domain>/{memory,postgres}/`" | D10（顶层 internal 退役，改为 `service/<domain>/rpc/internal/repository/`） |
| 01   | §3 旧 target   | "`internal/` 收敛为'跨服务横切基础设施'"             | D10（改为 `pkg/`，顶层 internal 删除） |
| 01   | TD-4           | "proto/ 与 `service/<domain>/rpc/<domain>.proto` 双源" | D10（单源到 service，删顶层 proto/） |
| 01   | TD-11 / TD-12  | 把 model/rpcgen 收敛到 `internal/`                  | D10（顶层 internal 整体退役）        |
| D8 来源描述    | 引用 `internal/presence` `internal/observability`   | D10（改为 `pkg/...`）                |
| 02   | CP-3 行 74      | "需要时显式建立 `service/<domain>/rpc/internal/<other>adapter/`" | D12（一律禁止，含不经 adapter）       |
| 02   | AU-3 / §144~145 | `adapter/user.go`、`adapter/mail.go`（rpc 内持他域 client） | D12（跨域编排上移 API 层）             |
| 01   | §3 约束 行 182  | "或新增 `service/<domain>/rpc/internal/<other>adapter/`" | D12（删除该逃生口）                   |
| 01   | TD-7 / §4 step16 / §5 | repository 下沉到 `…/rpc/internal/repository/{memory,postgres}/` | D13（改为 `…/rpc/internal/model/`，无 repository 层） |
| 01   | §3 约束         | "`service/<domain>/rpc` 独占该 domain 的 DB model、repository、proto" | D13（去掉 repository，留 model + proto） |
| 06   | XC-1            | repository 平铺最终归宿                              | D13（归 `…/rpc/internal/model/`） |

下面 §3 给出每条具体修复 PR 的可执行 diff 指引。

---

## 3. 修复指引（已应用在本仓库各文档中）

### 3.1 01-project-structure.md
- TD-7 描述：repository 拆分目标中 `outbox` 项删除；改为：被 D1 弃用，无需保留。
- §3 target 布局：消息域服务按 D3 `msg*` 家族命名（`msg-api` / `msg-rpc` / `msggateway` / `msgtransfer` / `push`），去掉 `service/message/` 父目录与 `-ws` 后缀。

### 3.2 02-microservices.md
- §4.3 group event hook：改为发 Kafka `group.member.added/removed.v1` topic（直接用 D6 wire format）。

### 3.3 04-agent.md
- §4.1（现状描述）：保留"现 msg-rpc 同 tx 写 outbox"作为**重构前**事实陈述，但加一行"⚠️ 重构后见 D1/D2"。
- §4.2（重构后）：流程图改为 msg-rpc → Kafka toTransfer → msgtransfer（在 categorize 阶段判断 Agent 触发）→ Kafka agent.trigger.v1 → agent-rpc。删 outbox publisher 节点。
- §7 行 326：把"AI 消息是新的 outbox event"改为"AI 消息走相同 Kafka 链路"。

### 3.4 05-observability-cicd.md
- §5.3 关键指标：移除 `message_outbox_pending`。
- §7.3 多副本支持的 msggateway 一节：删除"必须配 presence 跨实例路由"，改为"广播路由不需要 presence routing；presence 仅用于在线状态查询（D4）"。

### 3.5 06-cross-cutting.md
- XC-1：repo 列表中 `message_outbox_repository.go`、`postgres_outbox.go` 后加标注"⚠️ D1 弃用，Phase 0 删除"。
- XC-2：目标分包列表移除 `outbox/`。
- XC-9 (account_id int64) 与 D2 无冲突，保留。

### 3.6 D10 配套修改（全文档）
- 01-project-structure.md：§3 目标布局重写为"删 internal/api/proto，加 pkg/"；§4 Stage 1~5 收敛路径；§5/§6 路径前缀全替换。
- 02-microservices.md：CP-4 引用的 `internal/domain/<domain>/` 改为 `service/<domain>/rpc/internal/domain/`；CP-3 中 RPC 间 adapter 路径同步。
- 03/04/05/06/07 文档中的 `internal/<pkg>/` 引用按映射表批量替换：
  - `internal/messaging` → `pkg/messaging`
  - `internal/presence` → `pkg/presence`
  - `internal/observability` → `pkg/observability`
  - `internal/llmobs` → `pkg/llmobs`
  - `internal/apperror` → `pkg/apperror`
  - `internal/response` → `pkg/response`
  - `internal/ctxuser` → `pkg/ctxuser`
  - `internal/idgen` → `pkg/idgen`
  - `internal/objectstorage` → `pkg/objectstorage`
  - `internal/config` → `pkg/config`
  - `internal/health` → `pkg/health`
  - `internal/agent/pythonexec` → `pkg/pythonexec`
  - `internal/agentruntime` → `service/agent/rpc/internal/runtime`
  - `internal/agentim` → `service/agent/rpc/internal/{trigger,orchestrator,hosting,imadapter,audit}`
  - `internal/agenteval` → `service/agent/rpc/internal/eval`
  - `internal/auth/*` → `service/auth/rpc/internal/*`
  - `internal/mail` → `service/mail/rpc/internal/provider`
  - `internal/gateway` → `service/msggateway/internal`
  - `internal/transfer` → `service/msgtransfer/internal`
  - `internal/outboxpublisher` → 删除（D1）
  - `internal/repository` → 按域分散到 `service/<domain>/rpc/internal/model/`（**纯 goctl model，无 repository 层**，D13）
  - `internal/logic/<domain>` → `service/<domain>/{api,rpc}/internal/logic/`
  - `internal/handler/<domain>` → `service/<domain>/api/internal/handler/`
  - `internal/servicecontext/<domain>` → `service/<domain>/<api|rpc>/internal/svc/`
  - `internal/rpcgen` → 删除（已被 `service/<domain>/rpc/<domain>service/` 替代）
  - `internal/model` → 删除（让 `service/<domain>/rpc/internal/model/` 自管）
  - `internal/types` → 删除（基本空）
  - `internal/domain/<X>` → `service/<X>/rpc/internal/domain/`
  - `internal/adminbootstrap` → `service/admin/api/internal/bootstrap`
- 文档间引用如"见 D8 ... `internal/...`"统一改为 `pkg/...`；当前 D8 描述已就地更新（见 §1）。

### 3.7 D11 / D12 配套修改

- **01-project-structure.md**：§3 约束"跨域 RPC 互调禁止…或新增 `<other>adapter/`"删去 adapter 逃生口，改为"跨域只由 API 层 import RPC client 编排（D12）"；§4 收敛路径开头加一行指明执行采用 D11"逐服务原地重建"轨。
- **02-microservices.md**：CP-3 的"建议"升级为引用 D12（一律禁止，含不经 adapter）；AU-3 收敛清单删 `adapter/user.go`、`adapter/mail.go`，注册 / 发信编排上移 auth-api。
- D11 为纯过程决策，不改 01~07 的目标布局，仅约束"怎么落地"。

### 3.8 D13 配套修改

- **01-project-structure.md**：TD-7 修复目标、§4 step 16、§5 命名矩阵中所有"repository → `…/rpc/internal/repository/{memory,postgres}/`"改为"`…/rpc/internal/model/`（goctl 生成，无 repository 层）"；§3 约束"rpc 独占 DB model、repository、proto"删去 repository。
- **06-cross-cutting.md**：XC-1 repository 归宿改为 model；memory/postgres 双实现项移除。
- 数据层测试策略改为 goctl model + sqlmock / 容器 PG（见 D13）。

---

## 4. 一致性矩阵

读者快速核对用——任何一条 ❌ 都是 bug，应立即修复或回到本文件改决策。

| 决策 | 01 | 02 | 03 | 04 | 05 | 06 | 07 |
|------|----|----|----|----|----|----|----|
| D1 outbox 弃用                | ✅ 修复（§3.1） | ✅ 修复（§3.2） | ✅ 原始决策   | ✅ 修复（§3.3） | ✅ 修复（§3.4） | ✅ 修复（§3.5） | ✅ 引用 |
| D2 seq 在 transfer 分配       | n/a            | n/a            | ✅ 原始决策   | ✅ 引用       | n/a           | ✅ 引用       | ✅ 引用 |
| D3 cmd 命名（不 rename）      | ✅ 修复（§3.1） | n/a            | ✅            | ✅            | n/a           | n/a           | n/a    |
| D4 presence 仅在线查询        | ✅            | n/a            | ✅ 原始决策   | n/a           | ✅ 修复（§3.4） | n/a           | n/a    |
| D5 Kafka topic 命名           | n/a            | n/a            | ✅ 原始决策   | ✅ 引用       | ✅ 引用       | n/a           | n/a    |
| D6 wire format = proto.Marshal | n/a            | n/a            | ✅ 原始决策   | n/a           | n/a           | n/a           | n/a    |
| D7 PG 角色：归档              | n/a            | n/a            | ✅ 原始决策   | ✅ 引用       | n/a           | n/a           | n/a    |
| D8 msggateway 纯连接层        | ✅            | n/a            | ✅ 原始决策   | ✅ 引用       | ✅            | n/a           | n/a    |
| D9 phase 顺序                 | ✅            | ✅            | ✅ 原始决策   | ✅            | ✅            | ✅            | ✅     |
| D10 顶层 internal/api/proto 退役 | ✅ 原始决策 | ✅ 修复（§3.6 + 路径约定） | ✅ 修复（§3.6 + 路径约定） | ✅ 修复（§3.6 + 路径约定） | ✅ 引用（路径约定） | ✅ 引用（路径约定） | ✅ 引用（路径约定） |
| D11 重构执行策略（分支/原地重建/不重写） | ✅ 引用（§4 执行轨） | n/a | n/a | n/a | n/a | n/a | n/a |
| D12 RPC 间一律不互调（含不经 adapter） | ✅ 引用（§3 约束/§6 验收 E） | ✅ 原始（收紧 CP-3） | n/a | n/a | n/a | n/a | n/a |
| D13 纯 goctl model，废 repository 层 | ✅ 修复（TD-7/§4/§5/§3） | ✅ 引用（聚合描述） | n/a | ✅ 修复（AG-3/AG-6） | n/a | ✅ 修复（XC-1） | n/a |

`✅ 原始决策`：该决策的来源文档。
`✅ 修复`：本次同步通过修改对齐。
`✅ 引用`：该文档引用但未冲突。
`n/a`：该文档与此决策无关。

---

## 5. 引用规则

- 任何文档涉及 D1~D9 的内容，必须在该段附近写"（见 00-decisions D1）"等显式标记。
- 不要在文档间复制 D1~D9 的全文 —— 引用本文件就够。
- 若要修改某条 D，先改本文件，再扫描 §4 矩阵中标 ✅ 的引用方文档对齐。
