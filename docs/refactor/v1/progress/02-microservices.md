# 02 — 微服务去 internal 依赖 落地进度

> 追踪 [`../01-project-structure.md`](../01-project-structure.md)（**最终目的：删掉顶层 `internal/`，只留 `service/`**）
> 与 [`../02-microservices.md`](../02-microservices.md)（CP-1 同一逻辑 2~3 份）的落地。状态图例见 [`README.md`](./README.md)。
> **续作者：先读文末 [§复刻 Playbook](#复刻-playbook) 再动手。**

## 目标

顶层 `internal/`（god-package `internal/logic`、`internal/repository`、`internal/rpcgen` 等）整体退役，
每个业务域的真相代码只留在 `service/<domain>/{rpc,api}`：
- **rpc** = 业务真相 + 自有数据层（goctl model），不依赖 `internal/`。
- **api** = BFF 聚合层，调用一个或多个 rpc（**rpc 之间不互调**）。

## 逐域进度

| 域 | rpc 业务逻辑 | rpc 数据层 | 是否仍依赖 internal | 状态 |
|----|------------|-----------|-------------------|------|
| auth | service ✅ | internal/auth | 是 | ⚠️ 结构已迁，数据层未脱 internal |
| user | service ✅ | **internal/repository**（goctl model 已生成于 `user/rpc/internal/model` 但**未接线**，是死代码）| 是 | ⚠️ 同上 |
| **friends** | **`service/friends/rpc/internal/logic` 自包含** | **`service/friends/rpc/internal/model`（goctl）** | **否（rpc 已脱）** | ✅ #426（goctl+BFF，删 `core`）|
| **groups** | **`service/groups/rpc/internal/logic` 自包含** | **`service/groups/rpc/internal/model`（goctl）** | **否（rpc 已脱）** | ✅ #415 |
| **third (mail)** | **`service/third/rpc/internal/logic`（SES 发信，无 DB）** | **provider 库 `service/third/rpc/internal/provider`（mail 无表，无 goctl model）** | **否（已脱）** | ✅ #429（mail 折入新服务 third）|
| **media** | **`service/media/rpc/internal/logic` 自包含（删 `core`）** | **`service/media/rpc/internal/model`（goctl）** | **部分**（仅下载鉴权跨域读 accounts/message，keystone 阻塞）| ✅ #433（goctl，写入脱 internal）|
| message/gateway/transfer/admin | 仍在 internal | internal | 是 | ❌ keystone，最后做 |

> groups 是**第一个真正切断 rpc 数据层对 internal 依赖**的域，作为后续域的参考实现；
> friends 照此复刻（#426）：`friendships` 加自增代理 PK（迁移 `018`）→ goctl model → 状态机搬进 rpc/logic →
> 好友资料移到 friends api(BFF) 聚合 user-rpc（批量 `GetUsersByIDs`），删除 `service/friends/core`。
> friends 的 internal/repository friendship 方法暂留（monolith `default_assistant`/`agent_definition` 仍用 `EnsureAcceptedFriendship`），待 message 迁移后删。

## groups-rpc 本次操作（PR #415，issue #415）

**两种退役路线**——本仓库现存两套，选型见 Playbook：
- 旧路线 `service/<domain>/core` + `internal/<domain>validate` 过渡包（friends 原走此路 #426、media #433 均已切到下一条；`internal/mediavalidate` 作 keystone shim 暂留喂 message monolith + user-rpc 头像校验）；
- **groups/friends/media 用的 goctl model + BFF 聚合**（本文档），rpc 数据层走 goctl，不给 monolith 提供数据层过渡包。

做的事（标准 go-zero 改 config/svc/logic + 数据层换 goctl model）：

1. **goctl model → `service/groups/rpc/internal/model`**（Go internal 可见性 = 只能被 rpc 内部 import）
   - `goctl model pg datasource -url <本地临时PG> -table "groups,group_members" -dir ... --style go_zero`
   - **本地临时 PG 还原 schema**：`docker compose up -d postgres`（端口冲突时 `POSTGRES_PORT=5433`）+ `scripts/migrate-postgres.sh`，不碰生产库。
   - **坑：goctl pg 不支持复合主键**。`group_members` 原主键 `(group_id,account_id)` → 迁移 `db/migrations/017` 引入自增代理主键 `id`，复合键降为 `UNIQUE(group_id,account_id)`（保留唯一性供 `ON CONFLICT` upsert；对 monolith 向后兼容）。`groups` 单主键 `group_id` 无需改。
   - 复合查询/事务建群等 goctl 不生成的，写进 custom 文件 `groups_model.go` / `group_members_model.go`（**不碰 `*_gen.go`，带 DO NOT EDIT**）；DB 整型常量（role/status）放 `vars.go` 单一来源。
   - **事务边界在 Logic 层**：model 只暴露 `Transact` + `WithSession`，不在 model 里编排业务事务。

2. **config/svc**（`internal/{config,svc}`）：删 `business "internal/logic"` 与 `internal/repository` import；
   svc 改 `postgres.New(c.DataSource)` 注入 `model.NewGroupsModel/NewGroupMembersModel`。
   groups-rpc 转 **Postgres-only**（去掉 `StorageDriver`/memory）。

3. **logic**（9 个 `*logic.go`）：把 `internal/logic/groupslogic.go` 业务规则**搬进** rpc/internal/logic，
   经 `svcCtx` 调 model 接口；共享规则集中到 `groups_rules.go`；role/status int↔string 映射在 logic 层。
   logic 依赖 model **接口** → 用 fake model 写单测（`groups_logic_test.go`，无需 PG）。

4. **跨域 user 依赖上移 BFF**（关键架构决策）：groups 需要"用户存在性校验 + 成员资料(昵称/头像)补全"，
   这是 user 域数据。按"rpc 不互调"，**rpc 不读 user 表也不调 user-rpc**；改由 `service/groups/api`(BFF)：
   - rpc 的 `GroupMember` 只返回 `group_id/user_id/role/state/时间`，profile 字段留空；
   - api 加 `UserRPC` client：ListMembers/单成员响应调 `user-rpc.GetUserByID` 补全（`hydrate.go`，列表并发补全）；
     CreateGroup/AddMember 前调 user-rpc 校验存在。

5. **输入处理**：去掉后端规范化（`TrimSpace` 等，由客户端保证）；**保留校验**（required + 长度上限 + 成员数上限 200，
   防脏数据/DoS——DB 无此约束）。函数从 `normalize*` 改名 `validate*`。

6. **tracing 切 go-zero 自带 Telemetry**：去掉 config 的 `Tracing observability.TracingConfig` 字段与 main 里的
   `pkg/observability` tracing 接线（`InitServiceTracing`/Trace 拦截器/中间件）；改由 go-zero 内置 otel（zrpc/rest 默认拦截器
   + `ServiceConf.Telemetry` 启动 trace agent）。生产 endpoint 经 yaml `Telemetry.Endpoint: ${AGENTS_IM_OTLP_ENDPOINT}`
   读 ConfigMap 注入的 env（`deploy/k8s/etc/`）；本地经 docker-compose 的 tempo + dev-up 生成的 `Telemetry` 块上报
   （见 [`DEVELOPMENT.md`](../../../DEVELOPMENT.md) 与 `deploy/local/tempo.yaml`）。**注意：metrics 仍用 `observability.MetricsHandler`，未动**；
   其余 13 个服务仍走 observability tracing，groups 是首个切原生（如需统一是独立迁移）。
   交付遇到的 CD 坑（#418/#420，详见 [`deploy/README.md` §Database migrations during deploy](../../../../deploy/README.md)）：Drone 迁移门控须 grep 文件、
   迁移须连 k3s postgres ClusterIP（`--network host`），均已修。

7. **monolith 保持不动（keystone）**：`internal/logic/groupslogic.go`、`internal/repository/*groups*`、
   `internal/repository/schema_v2_enums.go` **不删**——message monolith（`internal/rpcgen/message`）仍把
   `GroupsLogic` 当 `GroupMemberLister` 喂给 `MessageLogic`。groups 这部分的彻底删除**依赖 message 迁移**。

**etc 区分环境**（三层，本次 UserRPC 按此分别配置）：
`deploy/k8s/etc/`（生产/k8s，服务名 `user-rpc:9090`）· `etc/` + `service/groups/*/etc/`（本地，`127.0.0.1:9090`）。
`scripts/dev-up.sh` 补了 groups-rpc 配置生成 + groups-api 的 GroupsRPC/UserRPC + 启动 groups-rpc。

## 复刻 Playbook（下一个域照做）

完整步骤、坑、goctl 用法、验收清单见 **[`refactor-domain-to-service` skill]**（已从本文档抽出、含本轮 N+1→批量接口与 goctl scaffold 坑）。本文档只保留 groups 的逐域记录与 §剩余/后续。

[`refactor-domain-to-service` skill]: ../../../.claude/skills/refactor-domain-to-service/SKILL.md

## 剩余 / 后续

> 退役欠下的尾巴，按域记一笔；新域重构完在此追加 `### <域>`。
> **全局收尾**：顶层 `internal/` 完全退役以 message/gateway/transfer/admin（07-message-rpc-redesign）为最后一公里；user/auth 把 rpc 数据层从 `internal/repository` 切到各自 `rpc/internal/model`（user 的 goctl model 已生成待接线）。friends/groups rpc 已脱 internal（friends #426、groups #415）；media 写入数据层已脱（#433，下载鉴权跨域读 keystone 暂留）。

### friends（#426）

- **internal/repository friendship 方法暂留**：monolith `internal/logic/default_assistant.go`、`agent_definition.go` 仍调 `EnsureAcceptedFriendship`，故 `postgres_user_friends.go` 的好友方法保留，待 message 迁移后删。
- friends rpc 不再返回 `Friendship.friend` 资料（proto 字段保留留空），由 friends api(BFF) 聚合 user-rpc 补全；proto 未重生成。
- 修正：旧 `core` 把 outgoing 好友请求列表的 `friend` 错填成请求者自己，BFF 改为正确指向对方（friend_id）。

### third（mail，#429）

mail 是**特例**：不是 god-package 退役，它只依赖 `internal/mail`（干净的 Tencent SES provider 库，无 DB、无 `internal/logic`/`internal/repository`）。本次 = **新建第三方接入层服务 `third`，把 `service/mail` 折入、provider 库一并搬进 `service/third/rpc/internal/provider`（包改名 `mailprovider`），断开对顶层 `internal/` 的依赖**；部署单元 `mail-rpc` → `third-rpc`（Dockerfile / k8s / drone build / dev-up / detect-deploy / test-deploy）。

- **wire 契约不变**：proto 仍 `package mail.v1` / `service MailService`，只迁 go 落点；auth 经 `mailadapter` 拨号端点 `MailRPC.Endpoints` 由 `mail-rpc:9095` → `third-rpc:9095`（**auth 配置键仍叫 `MailRPC`**——语义是「mail 能力」，未改键名以缩小爆炸半径）。
- **cosmetic 尾巴**：`service/third/rpc/mail/mail.pb.go` 内嵌 descriptor 的 `go_package`/`source` 字符串仍是 `service/mail/rpc/...`。原因：`mail`→`third` 长度不同，sed 改字符串会破坏 descriptor 的长度前缀（已踩坑→还原）；且本地 `protoc-gen-go` v1.35.2 比仓库 v1.36.11 旧，regen 会把结构体格式降级（183 行不一致）。**功能零影响**（wire 包是 `mail.v1`，测试绿）。待下次用 v1.36.11 对 `service/third/rpc/mail.proto` regen 时一并修正。
- 为何**只折 mail、不折 media**（A 方案）：见下 `### media`。
- **client 包命名 `mailservice`**（goctl 按 proto `service MailService` 生成,与 `authservice` 同族,非异常）：重命名为 `thirdclient` 会破坏 wire 契约,推迟到 v2 → [`../../v2/01-third-service-naming.md`](../../v2/01-third-service-naming.md)。

### media（#433：goctl+BFF 改造完成）

media 已按 goctl+BFF 主线退役 `service/media/core`，**写入数据层**落 `service/media/rpc/internal/model`（goctl），脱 `internal/repository`：

- **删 `service/media/core`**，`MediaLogic` 业务规则折入 `service/media/rpc/internal/logic`（`media_rules.go` 共享校验/int↔string 映射/对象 key 生成/下载鉴权 + 4 个 endpoint logic）；config 转 **Postgres-only**（去 `StorageDriver`，两份 etc 同改）；输入只 validate 不 normalize；logic 依 model **接口** + fake 单测（`media_logic_test.go`，无需 PG）。
- **`media_objects` 数据层 goctl model**：`media_id` 单主键无复合键坑、无需迁移；custom `CreateMediaObject`/`UpdateStatus`（带 `returning`）只写业务列，`conversation_id`/`storage_provider`/`expires_at` 交 DB 默认；`sha256`/`width`/`height` 仍落 `metadata` JSON（与旧 repo 一致，**当前只写不回读**）；purpose/status 整型常量在 `model/vars.go` 单一来源。

**keystone 阻塞暂留（待 message-rpc 落地后删 / BFF 化）**：
- media-rpc **下载鉴权**（管理员判定读 accounts、消息附件可见性读 message）仍经 `svcCtx.Accounts`/`AttachmentAccess` 读 `internal/repository`——**无 message-rpc 可 BFF 化**，故这两笔跨域读没上移；media-api 仍是纯 `mediaclient` 透传 BFF（未加 `UserRPC`）。media-rpc 因此**部分仍依赖 internal**（仅此二者）。
- `internal/mediavalidate` + `internal/repository` 的 media 数据层保留：喂 **message monolith**（发信附件校验）+ **user-rpc 头像校验**。message 迁移后这些连同下载鉴权一并删。
- 删了 core 耦合的 `internal/logic/media_download_access_test.go`（集成 MessageLogic + media core）；media-rpc 下载鉴权改由 `media_logic_test.go` 以 fake accounts/attachment checker 覆盖 owner/admin/附件参与者/forbidden 四路。
- **dev-up.sh 仍未起 media-rpc/media-api**（pre-existing，与本次数据层改造正交，未在本 PR 补）——本地起媒体链路需手动配置，留作后续配套。

> 下方为「为何 media 不并入 third（A 方案保持独立）」的设计权衡，存档备查：
> - **本质不同**：mail 是真·第三方适配器（薄壳包 SES）；media 是**有自己 DB 领域的领域服务**（media 对象、upload intent、附件访问鉴权），只是「用了」对象存储。按「用了外部存储就算第三方」的逻辑每个连 Postgres 的服务都成第三方 → "third" 会退化成按基础设施分类的杂物抽屉。
> - **伸缩/故障域不同**：media 是数据面（上传/下载字节流、头像服务，自带 ingress `media-api` `/media` 4 路由）；mail 在登录/注册验证码关键路径。该独立伸缩、独立挂。
> - **难度不对称**：mail 去依赖 = 搬一个 provider 库（1 PR）；media 去依赖 = 退 `internal/repository` 且用了 3 个 repo（`MediaRepository` + 跨域 `AccountRepository`/`MessageRepository`），属 goctl model 改造（本 #433）。

### groups

- **monolith 清理**（待 message 迁移）：删 `internal/logic/groupslogic*.go` + `internal/repository/*groups*` + `schema_v2_enums.go` 中 groups 部分。
- **user-rpc 批量接口已落（#423）**：BFF `hydrate.go`/`ensureUsersExist` 改用 `user-rpc.GetUsersByIDs`（1 次 gRPC + `WHERE id IN`），N+1 已解。遗留两笔债：
  1. 批量查实现先落在 `internal/logic.GetUsersByIDs` + repo `ListByIDs`（路线一，临时债）；待 user-rpc 脱 internal 时跟着迁回 `service/user/rpc/internal/model`，勿漏。
  2. `service/user/rpc/userclient/user.go`、`internal/server/userserver.go` 的 `GetUsersByIDs` 是**手工补的**（goctl 全量 scaffold 的路径/命名 `user_server.go`/`user_client/` 与本仓库现有 `userserver.go`/`userclient/` 不符，只取了 `.pb.go`）。下次对 `user.proto` 跑 goctl 重生成前：先确认 proto 已含该 rpc，再用 goctl 由 proto 生成校正手工部分。
