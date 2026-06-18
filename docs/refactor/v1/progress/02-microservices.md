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

一眼概览；路线 / owner 落点 / 数据层 / 尾巴见 [§已迁移域·剩余/后续](#已迁移域--剩余后续)。

| 域 | 仍依赖 internal | 状态 |
|----|----------------|------|
| groups | 否 | ✅ #415 已脱 |
| friends | 否 | ✅ #426 已脱 |
| third (mail) | 否 | ✅ #429 已脱（折入新服务 third）|
| media | 部分 | ✅ #433 写入脱；下载鉴权 keystone 暂留 |
| admin | 部分 | ✅ #448 task_reports 脱；跨域只读 keystone 暂留 |
| auth | 否 | ✅ #563 数据层脱 internal（goctl）+ 去 adapter 直调 user-rpc/mail-rpc；会话 Redis（#435）|
| user | 部分 | ✅ #452 数据层脱；助手开通 keystone 暂留（头像校验 #533 已切 media-rpc）|
| msg-rpc | 部分 | 🚧 #457 PR1 + #463：goctl 数据层；群成员鉴权 + **AI 托管 runtime/触发钩子**（#463 随 message-api 退役迁入）keystone 暂留（媒体附件校验 #533 已切 media-rpc）|
| msg-api | 否（纯 BFF）| ✅ #463 已切流：7 路由 BFF over gRPC（4 消息→msg-rpc、ai-hosting×2→msg-rpc、feedback→admin-rpc）|
| msggateway/transfer | 是 | 🟡 keystone 收敛中；message-api/message-rpc 已退役（#463=A1+A2 合并）；A3 ✅ #492（gateway 切 msg-rpc gRPC 并改名 msggateway，ws/contract 迁出 internal）；后续 A4 删 internal 消息域（注意 AI 托管 runtime 仍消费 servicecontext/message，与 B1 协同）|

## groups-rpc 本次操作（PR #415，issue #415）

**两种退役路线**——本仓库现存两套，选型见 Playbook：
- 旧路线 `service/<domain>/core` + `internal/<domain>validate` 过渡包（friends 原走此路 #426、media #433 均已切到下一条；`internal/mediavalidate` 曾作 keystone shim 喂 message monolith + user-rpc 头像校验，**#533 已删**——校验改经 media-rpc 的 ValidateAvatarMedia/ValidateMessageMedia RPC）；
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
   （见 [`DEVELOPMENT.md`](../../../DEVELOPMENT.md) 与 `deploy/local/tempo.yaml`）。**注意：metrics 仍用 `observability.MetricsHandler`，未动**。
   **统一切换已完成（#443）**：groups 后，凡持有 `Tracing observability.TracingConfig` 字段的 service 均已切原生 Telemetry——
   auth(api+rpc)、user(api+rpc)、admin(api)、third(rpc)、friends(api+rpc)、media(api+rpc)（删字段 + 删旧 tracer/拦截器/中间件 +
   svc client 去 `GRPCUnaryClientInterceptor` + yaml 加 `Telemetry` 块 + dev-up/deploy-k8s/verify-contract-markers 同步）。
   **仍未切**：agent-api 及 message monolith 系（msggateway/msgtransfer）走 **共享 `pkg/config.APIConfig.Tracing`**
   经 `ToRestConf`→`GoZeroTelemetryConfig` 桥接 + 旧 `InitServiceTracing` 重复埋点——它们无独立 config 字段，统一需动共享结构，留后续。
   交付遇到的 CD 坑（#418/#420，详见 [`deploy/README.md` §Database migrations during deploy](../../../../deploy/README.md)）：Drone 迁移门控须 grep 文件、
   迁移须连 k3s postgres ClusterIP（`--network host`），均已修。

7. **monolith 保持不动（keystone）**：`internal/logic/groupslogic.go`、`internal/repository/*groups*`、
   `internal/repository/schema_v2_enums.go` **不删**——message monolith（`internal/rpcgen/message`）仍把
   `GroupsLogic` 当 `GroupMemberLister` 喂给 `MessageLogic`。groups 这部分的彻底删除**依赖 message 迁移**。

**etc 区分环境**（三层，本次 UserRPC 按此分别配置）：
`deploy/k8s/etc/`（生产/k8s，服务名 `user-rpc:9090`）· `etc/` + `service/groups/*/etc/`（本地，`127.0.0.1:9090`）。
`scripts/dev-up.sh` 补了 groups-rpc 配置生成 + groups-api 的 GroupsRPC/UserRPC + 启动 groups-rpc。

## 复刻 Playbook（下一个域照做）

完整步骤、坑、goctl 用法、验收清单见 **[`refactor-domain-to-service` skill]**。本文档只保留 groups 逐域记录与 §已迁移域·剩余/后续 台账。

[`refactor-domain-to-service` skill]: ../../../.claude/skills/refactor-domain-to-service/SKILL.md

## 已迁移域 · 剩余/后续

> 逐域迁移台账，一域一行：路线 / owner 落点 / 数据层 / PR / 退役欠下的尾巴。新域完成后追加一行，只此一处（不在 skill 维护）。
> **全局收尾**：顶层 `internal/` 完全退役以 message/gateway/transfer（[`07-message-rpc-redesign`](../07-message-rpc-redesign.md)）为最后一公里；auth 数据层（credentials/email_verification）从 `internal/repository` 切到 `rpc/internal/model` 待独立 PR；user 数据层已脱（#452），头像校验 #533 已切 media-rpc，仅剩助手开通一处 keystone 跨域例外随 agent 迁移后删。

| 域 | 路线 | owner 落点 | 数据层 | PR | 剩余 / 后续 |
|----|------|-----------|--------|----|-----------|
| groups | goctl+BFF（首个数据层脱 internal，参考实现）| `service/groups/rpc/internal/logic` | `service/groups/rpc/internal/model`（goctl；迁移 017 代理 PK）| #415/#416（批量 #423）| 待 message 迁移删 monolith `groupslogic*.go`+`*groups*` repo+`schema_v2_enums.go` groups 部分。两笔债：批量查暂落 `internal/logic.GetUsersByIDs`（待 user-rpc 脱 internal 迁回 model）、`userclient`/`userserver` 的 `GetUsersByIDs` 手工补（下次 goctl regen 前先确认 proto 含此 rpc）。|
| friends | goctl+BFF（退役 `core`）| `service/friends/rpc/internal/logic` | `service/friends/rpc/internal/model`（goctl；friendships 代理 PK 迁移 018）| #426 | friendship repo 方法暂留喂 monolith（`EnsureAcceptedFriendship`），待 message 迁移删。rpc 不返回 `Friendship.friend`，BFF 聚合 user-rpc 补全。修正 outgoing 列表 friend 错填请求者→指 friend_id。|
| third（mail）| 新建第三方接入服务（折入 `mail`，非 god-package 退役）| `service/third/rpc/internal/logic`（SES 发信，无 DB）| provider 库 `internal/provider`（无表→无 model）| #429 | `mail-rpc`→`third-rpc`；wire 契约不变（proto 仍 `mail.v1`/`MailService`，auth 键仍 `MailRPC`）。cosmetic：`mail.pb.go` descriptor 留旧路径（sed 破长度前缀、本地 protoc-gen-go 旧）→待 v1.36.11 regen 修。client 包 `mailservice`，改名推迟 v2。|
| media | goctl+BFF（删 `core`）| `service/media/rpc/internal/logic` | `service/media/rpc/internal/model`（goctl）| #401→#433 | **部分仍依赖 internal**：下载鉴权（读 accounts/message）无 message-rpc 可 BFF 化，仍读 internal/repository。**#533 已删 `internal/mediavalidate`**：发信附件校验 + user-rpc 头像校验改经 media-rpc 的 ValidateMessageMedia/ValidateAvatarMedia RPC（user-rpc/msg-rpc 加 MediaRPC 客户端）；下载鉴权仍读 internal/repository（accounts/message），message 迁移后删。dev-up 未起 media-rpc/api。〔不并入 third 的 A 方案见表下〕|
| auth | 特性改造（非数据层退役）| `internal/auth/logic`（仍 keystone）| 未迁（credentials/email_verification 仍 internal）；**#466 起 `rpc/internal/model` 已有 goctl `auth_credentials` model**（EnsureTestCredential / EnsureAdminCredential 用，新代码禁 internal）| #435 / #466 | 活跃会话 Postgres→Redis 按 (user,device)；共享 `DeviceAuth` 挂 4 个 `jwt:Auth` api。go-zero 坑：`jwt:Auth` 丢注册声明→token 镜像 `user_id`/`session_id`。5 处 inline 校验迁 Redis。`active_sessions` 表+方法成死代码待清。credentials/email_verification 整体 goctl+BFF（AuthLogic 迁入 rpc、退役 internal/auth）待独立 PR（keystone-blocked）。#466 新增跨域鉴权读例外 `model.AccountsGuard`（accounts.account_type，参照 media #433）+ `logic/password_rules.go` 本地 bcrypt（与 internal bcrypt-v1 契约一致）；本轮新增 `EnsureAdminCredential` 只补 admin 首次凭据、不覆盖已有密码，供 admin-api bootstrap 编排。auth 域整体迁移时合并。**#551**：注册/登录用户资料读改走 user-rpc（`rpc/internal/userrpc` adapter + `UserRPC` client，9090），auth 彻底脱 `internal/logic.UserLogic`、`NormalizeIdentifier` 内联到 useradapter；为不丢邮箱验证态，user-rpc `CreateUserRequest` +`email_verified_at`(RFC3339→落 accounts)。是 gate #550（`profiles.avatar_media_id` text→bigint）「读路径迁出 internal/」的第 1 步（leaf）。**#563（退役完成）**：勘探确认 `internal/auth` 未被 message monolith 消费（叶子域）→ 整体 goctl+BFF 退役：`AuthLogic`（Register/Login/ValidateToken/ParseToken/RequestRegistrationEmailCode + 密码 bcrypt + 邮箱验证码）搬入 `service/auth/rpc/internal/logic`；新增 goctl model `auth_email_verification_tokens` + 扩 `auth_credentials`（`InsertCredential`/`FindAuthByIdentifier` join accounts/`EmailExists`）；**去 adapter 直调下游 client**——删 `rpc/internal/userrpc` 与 `internal/auth/{useradapter,mailadapter,repository,model,logic}`，svc 直接注入 `userclient.User`/`mailclient.Mail`（错误经 `rpcerror.FromStatus` 还原 apperror）；转 Postgres-only（删 `StorageDriver`，3 份 yaml + dev-up 同步）。auth-rpc→user-rpc rpc 互调为既有例外（注册建 user/登录读资料的控制流耦合，不可下沉 BFF）。legacy sha256 校验删除（PG 无 salt 列本就不成立，全 bcrypt）。**勘探修正**：friends-rpc/groups-rpc 早已脱 internal（#426/#415 纯 BFF），不再经 internal 读 profiles；当前仍经 `internal/` 读 profiles（含 avatar）的存活消费者只剩 **agent-api、msg-rpc、admin-rpc**，按此三处依次切。**#553**：agent-api（第 2 步）—— HTTP 面只读账号查询（account_type 校验 + 定义 authorize 的 `GetByID`）改走 `service/agent/api/internal/userrpc` adapter（实现 `repository.AccountRepository` over user-rpc，账号写/好友写不可达→fail-loud），svc 注入、共享 `APIConfig` +`UserRPC`；`internal/logic.AgentLogic`/`AgentAssemblyLogic`（agent 域、agents 表无 avatar）暂留。**待续**：msg-rpc、admin-rpc。|
| admin | 从零建 rpc+goctl+BFF | `service/admin/rpc/internal/logic`（唯一碰 DB；proto `Admin`）；`service/admin/api/internal/bootstrap` 编排 AdminBootstrap | `task_reports` goctl model；跨域只读暂 internal/repository | #448 | 跨域只读（accounts/friendships/messages/agent_audits/feedback）= keystone 例外，待相关 rpc 落地 BFF 化。pb↔行映射在 logic（无第三结构体）。admin 账号闸合进 `DeviceAuth` 经 `GetUserDetail`。AdminBootstrap 已从 auth-rpc/internal 迁到 admin-api：经 user-rpc 建 admin account，经 auth-rpc EnsureAdminCredential 补首次凭据。AI-replay hook 本就 nil（无回归）。新增部署单元 `admin-rpc:9097`。旧 `AdminLogic`/`AdminAIReplayLogic` 随重生成删除。|
| msg | goctl+BFF（keystone 域，07 §3.1 接口重设计；分 2 PR）| `service/msg/rpc/internal/logic`（4 RPC 实现，行为对齐旧；新增 6+ RPC stub Unimplemented）| `messages`/`conversation_threads`/`user_conversation_states`/`message_outbox` goctl model（事务编排在 Logic；迁移 019 给 UCS 加代理 PK）| #457（PR1 rpc / PR2 api）| **PR1 rpc（additive，msg-rpc:9098，不动 dormant rpcgen/message）**：跨域 keystone 例外——SendMessage inline 鉴权（群成员=internal GroupsLogic；媒体附件校验 #533 已切 media-rpc ValidateMessageMedia）群成员暂依赖 internal；outbox payload 与 message-transfer 消费契约逐字节一致。**PR2 msg-api（additive，纯 BFF:8090，未切流）**：4 消息路由 .api-first→goctl，BFF over gRPC 调 msg-rpc（msgclient + rpcerror.FromStatus）；svc 无 UserRPC。**#463（=03 §9 A1+A2 合并）已切流退休 message-api/message-rpc**：勘探确认 web 走 REST POST /messages（WS 只收事件），AI 托管触发钩子在 message-api 进程内 → 触发钩子+hosting runtime 整套迁入 msg-rpc（svc.newConversationAIHostingRuntime 移植 message-api main 接线，SendMessage persist 后 fireMessageCreatedHook，语义对齐含 dedup；config 加 DeepSeek/LLMObservability/PythonExecutor，deployment 加 python-executor-controller SA）；msg.proto +Get/UpdateConversationAIHosting（keystone 例外注明待 B1 迁 msgtransfer/agent 域）；admin-rpc +CreateFeedback（feedback 数据层 owner）；msg-api 7 路由 BFF（+ai-hosting×2→msg-rpc、/api/feedback→admin-rpc，裸 /feedback prod 不可达随退役丢弃）；ingress 三前缀→msg-api:8090；删 service/message-api、internal/rpcgen/message、internal/handler、internal/logic/message；部署/CI/verify 全链清理（Dockerfile/Makefile/drone-build/deploy-k3s/detect-deploy/kustomization/dev-up/test-deploy-k3s/verify-*/vite proxy）；JWT-sender 契约测试迁 msg-api 单测（fake msgclient）。**A3 ✅ #492**（gateway-ws→msggateway，4 ws command 走 msg-rpc gRPC，ws server 迁 service/msggateway/internal/ws、契约迁 common/share/gateway）。**待**：A4 删 internal 消息域剩余（messagelogic/servicecontext/message，与 B1 协同——msg-rpc AI runtime 仍消费）。写路径 Kafka 化进行中：**B0 ✅ #470**（Redpanda 单 broker 入 deploy/k8s/middleware，acks=all 实测 200msg/s p99=4ms；单节点偏差记 03 §9 B0 行；顺带修 Drone PR verification 全跳过 #472）；**B1 ✅ #474**（service/message-transfer 改名 service/msgtransfer + main 文件改 msgtransfer.go；franz-go acks=all 幂等 producer/consumer 封装入 pkg/messaging（topics + message.submitted 事件）；service/msgtransfer/internal/chain 新消费链：toTransfer poll-batch barrier + Redis seq Malloc（PG 播种）/dedup 7d/msgCache bucket-100 → produce toPostgres/toPush/agent.trigger → persist consumer 批量事务写 PG（手写 model，worker 无 CRUD 面故未走 goctl 生成）→ KafkaPushConsumer 适配现有 Worker+gateway dispatcher；prod 配置 Kafka.Enabled=true dormant；CI verification 加 redpanda/redis services 跑链路集成测试；偏差详单见 03 §9 B1 行；合并后需手工 kubectl delete 旧 deployment/service message-transfer）。**B2 ✅ #479 / B3a ✅ #484 / B3b ✅ #494**（B3b：Kafka 单轨——删 msg-rpc 非 Kafka 写分支+outbox model、msgtransfer legacy outbox worker、internal/outboxpublisher 与 repository outbox 残留；dev 补 redpanda；message_outbox 表留 90 天观察）。**待**：拆 push（C）、Redis 读路径/RevokeMessage 等（D，原 07 Phase2-3）。**#555（gate #550 第3步）**：AI 托管 agent-create 工具路径（`AgentAssemblyLogic.CreateAgentFromTool`）的账号读写脱 internal——新增 `service/msg/rpc/internal/userrpc` **Composite**（实现 `repository.Repository`：账号 `Create`/`GetByID`/`ExistsByIdentifier` over user-rpc、其余账号写 fail-loud；好友方法委托内部 postgres repo，`friendships` 表无 avatar 暂留），svc `messageContext.AccountRepo = Composite`，config/etc/k8s/dev-up +`UserRPC`。群成员鉴权本就 avatar-free（`NewGroupsLogic(repo, nil)`，lookup==nil 不 hydrate）。原子性：account 改 user-rpc 后 `TransactRepository` 类型断言失败→落非事务路径（跨服务尽力而为，参照 AdminBootstrap；孤儿 agent 账号风险极低）。无 .proto/.api 变更。|
| user | goctl+BFF（被依赖最多域）| `service/user/rpc/internal/logic` | `accounts`/`profiles` goctl model（事务编排在 Logic，model 仅 `WithSession`+`Transact`+单一原语；regen 顺带修旧 gen 的 MySQL 方言）| #452 | **部分仍依赖 internal**：①默认助手开通（agent 域写，无 agent-rpc 可 BFF）经 `svc.DefaultAssistantProvisioner` 接口注入、实现仍 `internal/logic`；②头像校验 **#533 已切 media-rpc**：`svc.AvatarValidator` 接口由 media-rpc 客户端适配器实现（ValidateAvatarMedia RPC），不再读 `internal/mediavalidate`。①随 agent 迁移删。monolith 消费者（同包 groups·agent logic）仍用 `internal/logic.UserLogic`（**auth/useradapter 已改走 user-rpc，#551**），故 `internal/logic/userlogic.go`+`postgres_user_friends.go` account/profile 部分+`schema_v2_enums.go` 暂保留。logic 依 model 接口 + fake model 单测。user-api 仍纯 BFF（user 是被 hydrate 的源，无跨域聚合）。|

> **存档：media 为何不并入 third（A 方案保持独立）**
> - **本质不同**：mail 是真·第三方适配器（薄壳包 SES）；media 是有自己 DB 领域的领域服务（media 对象、upload intent、附件鉴权），只是「用了」对象存储——否则每个连 Postgres 的服务都成第三方，"third" 退化成杂物抽屉。
> - **伸缩/故障域不同**：media 是数据面（上传/下载字节流、头像，自带 ingress `/media` 4 路由）；mail 在登录/注册验证码关键路径。该独立伸缩、独立挂。
> - **难度不对称**：mail 去依赖=搬一个 provider 库（1 PR）；media 去依赖=退 `internal/repository`（用了 `MediaRepository`+跨域 `AccountRepository`/`MessageRepository`），属 goctl model 改造（#433）。
