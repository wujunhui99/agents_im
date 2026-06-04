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
| friends | `service/friends/core`（[retire-internal-domain] 模式）| internal/repository | 部分 | 🟡 |
| **groups** | **`service/groups/rpc/internal/logic` 自包含** | **`service/groups/rpc/internal/model`（goctl）** | **否（rpc 已脱）** | ✅ 本 PR #415 |
| message/gateway/transfer/admin | 仍在 internal | internal | 是 | ❌ keystone，最后做 |

> groups 是**第一个真正切断 rpc 数据层对 internal 依赖**的域，作为后续域的参考实现。

## groups-rpc 本次操作（PR #415，issue #415）

**两种退役路线**——本仓库现存两套，选型见 Playbook：
- friends/media 用的 `service/<domain>/core` + `internal/<domain>validate` 过渡包（[retire-internal-domain] skill）；
- **groups 用的 goctl model + BFF 聚合**（本文档），rpc 完全自包含，不给 monolith 提供过渡包。

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
   读 ConfigMap 注入的 env（`deploy/k8s/etc/`），本地默认不配=关闭。**注意：metrics 仍用 `observability.MetricsHandler`，未动**；
   其余 13 个服务仍走 observability tracing，groups 是首个切原生（如需统一是独立迁移）。

7. **monolith 保持不动（keystone）**：`internal/logic/groupslogic.go`、`internal/repository/*groups*`、
   `internal/repository/schema_v2_enums.go` **不删**——message monolith（`internal/rpcgen/message`）仍把
   `GroupsLogic` 当 `GroupMemberLister` 喂给 `MessageLogic`。groups 这部分的彻底删除**依赖 message 迁移**。

**etc 区分环境**（三层，本次 UserRPC 按此分别配置）：
`deploy/k8s/etc/`（生产/k8s，服务名 `user-rpc:9090`）· `etc/` + `service/groups/*/etc/`（本地，`127.0.0.1:9090`）。
`scripts/dev-up.sh` 补了 groups-rpc 配置生成 + groups-api 的 GroupsRPC/UserRPC + 启动 groups-rpc。

## 剩余 / 后续

- **groups 收尾**：message 迁移后删 `internal/logic/groupslogic*.go` + `internal/repository/*groups*` + schema_v2_enums 中 groups 部分。
- **user/friends/auth**：把 rpc 数据层从 `internal/repository` 切到各自 `rpc/internal/model`（user 的 goctl model 已生成待接线）；friends 已用 `core` 模式，可继续或改 goctl。
- **user-rpc 批量接口**：BFF 补全成员资料目前是并发 N 次 `GetUserByID`，后续给 user-rpc 加批量获取优化 ListMembers。
- 顶层 `internal/` 完全退役以 message/gateway/transfer/admin（07-message-rpc-redesign）为最后一公里。

## 复刻 Playbook（下一个域照做）

1. **选路线**：该域是否被 message monolith in-process 消费？
   - 否 → 用 **goctl + BFF**（groups 这套，最干净，rpc 完全自包含）。
   - 是且暂不能动 monolith → 用 `core` + 过渡包（[retire-internal-domain] skill），或保留 internal 旧逻辑给 monolith、新 rpc 走 goctl（groups 选了后者）。
2. **goctl model**：本地临时 PG 还原 schema → `goctl model pg datasource`。复合主键先加自增 `id`（迁移 + change_log，向后兼容 monolith）。
3. **custom 文件补领域查询/事务**；事务边界放 Logic（model 只给 `Transact`/`WithSession`）；`*_gen.go` 不动。
4. **config/svc/logic** 三件套切到 model；跨域数据（user/media…）**上移 BFF 聚合**，rpc 不互调。
5. 输入只 `validate` 不 `normalize`；logic 依赖 model 接口 → fake 单测。
6. **改了就改文档**：更新本进度表 + dev-up/部署配置；monolith 仍依赖的部分注明"待 X 迁移后删"。

[retire-internal-domain]: ../../../.claude/skills/retire-internal-domain/SKILL.md
