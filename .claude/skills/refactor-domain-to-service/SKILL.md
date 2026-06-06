---
name: refactor-domain-to-service
description: 把一个业务域从顶层 internal monolith（god-package internal/logic + internal/repository）退役到它的 owner service（service/<domain>/{rpc,api}），rpc 自带 goctl 数据层、BFF 聚合跨域数据。重构/迁移下一个微服务域时照此复刻。
---

# 退役 internal 业务域 → service/\<domain\>

把单个域（user/friends/agent/...）的业务真相从共享 `internal/logic` god-package + `internal/repository`
退役到它的 owner service，让该域真相只留在 `service/<domain>/{rpc,api}`。最终目的：**删掉顶层 `internal/`**。

- **rpc** = 业务真相 + 自有数据层（goctl model），不依赖 `internal/`。
- **api** = BFF 聚合层，调一个或多个 rpc；**rpc 之间不互调**，跨域数据在 api 聚合。

参考实现：**groups**（PR #415/#416，首个 rpc 数据层脱 internal）、**friends**（PR #427，由旧 `core` 退役改造，含删 core + 配套清单）；批量接口优化见 PR #423/#424。

## 先读：关键前提与坑

- **`internal/logic` 是 god-package**：多个域的 `*Logic` 混在同一 `package logic`，共享 DTO（`UserProfile`）、
  helper（`formatTime`）。删一个域文件可能连带影响同包其它域。
- **message monolith 是 keystone**：`internal/rpcgen/message`、`internal/servicecontext/message` 会 in-process
  构造几乎所有域的 `*Logic`。被它消费的域，在 message 迁移前**无法从 internal/logic 彻底删生产代码**——
  保留 internal 旧逻辑给 monolith，新 rpc 走 goctl 自包含，文档里注明“待 message 迁移后删”。
- **Go `internal/` 可见性**：`service/X/rpc/internal/...` 只能被 `service/X/rpc/` 导入。需被多方导入的逻辑
  放 `service/<domain>/core`（与 rpc/api 平级）。
- **import 顺序不是 gofmt-canonical**：本仓库 `pkg/*` 排在 `common/share/*`、`internal/*` 之前。
  **绝不 `gofmt -w` 整个目录**（会重排几十个无关文件污染 diff）；新建/改动文件**照抄同目录邻居的 import 分组**，
  别按 gofmt 字母序。CI（golang:1.24-bookworm）容忍这套顺序。
- **选叶子域先做**：先迁被依赖最少的域。判断：
  ```bash
  D=<domain>
  grep -rln "\b${D^}Logic\b" internal/logic/*.go | grep -v _test.go           # 同包引用数
  grep -rln "\b${D^}Logic\b" internal/ --include=*.go | grep -vE "internal/logic/|_test.go"  # monolith 消费
  ```
  user 被依赖最多（agent/auth/adminbootstrap 都依赖），最后做。

## 路线选择

该域是否被 message monolith in-process 消费？
- **否** → **goctl + BFF**（本 skill 主线，最干净，rpc 完全自包含，不给 monolith 留过渡包）。
- **是且暂不能动 monolith** → 保留 internal 旧逻辑喂 monolith、新 rpc 走 goctl 自包含（groups 选此）；
  或老路 `service/<domain>/core` + `internal/<domain>validate` 过渡包（media 用过；friends 曾用，#426 已改 goctl+BFF）。

## goctl + BFF 主线步骤

### 1. 摸清消费方（务必全量）
分三类：① owner service；② 其它 service（→ BFF 聚合或 owner rpc 提供接口）；③ internal monolith（keystone，留过渡）。

### 2. goctl model → `service/<domain>/rpc/internal/model`
```bash
# 本地临时 PG 还原 schema，不碰生产库：
docker compose up -d postgres            # 端口冲突用 POSTGRES_PORT=5433
bash scripts/migrate-postgres.sh
goctl model pg datasource -url <本地临时PG> -table "<t1>,<t2>" -dir service/<domain>/rpc/internal/model --style go_zero
```
- **坑：goctl pg 不支持复合主键**。复合主键表先加自增代理 `id`（写 `db/migrations/NNN`，复合键降为
  `UNIQUE(...)` 保留 `ON CONFLICT` upsert，向后兼容 monolith），再生成。
- 复合查询/事务等 goctl 不生成的写进 custom 文件（`<table>_model.go`，**不碰 `*_gen.go`，带 DO NOT EDIT**）；
  DB 整型常量（role/status）放 `vars.go` 单一来源。
- **事务边界在 Logic 层**：model 只暴露 `Transact` + `WithSession`，不在 model 里编排业务事务。
  goctl 生成的 custom stub 默认只有 lowercase `withSession`——改成 exported `WithSession(session) XxxModel` 并补
  `Transact(ctx, fn)`（照 groups/friends），logic 才能在事务内复用 session。

### 3. config/svc/logic 三件套切到 model
- config/svc：删 `business "internal/logic"`、`internal/repository` import；svc 改 `postgres.New(c.DataSource)`
  注入 `model.NewXxxModel`。转 Postgres-only（去 memory/StorageDriver）。
- logic：把 `internal/logic/<domain>logic.go` 业务规则**搬进** rpc/internal/logic，经 svcCtx 调 model 接口；
  共享规则集中到 `<domain>_rules.go`；int↔string 映射在 logic 层。logic 依赖 model **接口** → fake model 写单测。

### 4. 跨域数据上移 BFF（rpc 不互调）
该域 rpc **不读别的域的表、不调别的域 rpc**；跨域数据（用户资料/媒体…）在 `service/<domain>/api`(BFF) 聚合：
- rpc 只返回自有字段，跨域字段留空；
- api 加对应 rpc client（如 `UserRPC`），在 BFF 补全 / 校验存在。
- **避免 N+1**：BFF 补全列表资料时，给 owner rpc 用**批量接口**（`GetXxxByIDs` repeated → `WHERE id IN (...)`），
  别每条单发（groups hydrate 的 N+1 → 批量见 PR #423）。批量接口暂落 internal/logic 的，记 §剩余/后续 待迁回 model。
- **proto 跨域字段保留留空免重生成 pb**：rpc 不再填资料类跨域字段，但 proto 字段保留（rpc 留 nil），由 BFF 填进 api types。
- **hydrate 的 peer 语义按端点定**：同一 rpc 在不同端点返回的关系视角不同（如“收到的请求”行是 requester→me），
  BFF 要按端点选对要展示的那一端 id；顺手核对旧 `core` 有没有填错（friends 旧 core 把 outgoing 请求资料误填成自己，#426 修）。
- **存在性校验也上移 BFF**：rpc 不再读 accounts，建关系前校验对端用户存在改用 user-rpc（批量）；缺 profile 的列表项按空资料降级，不阻断整列表。
- **跨域鉴权读暂留 owner rpc（无 peer rpc 可 BFF 时）**：当跨域读是**访问控制**（非展示资料）且对端域还没 rpc 可调，BFF 化无处落——此读可作 keystone-blocked 例外暂留在 owner rpc 直读 `internal/repository`（用接口注入 svcCtx 便于 fake 单测），文档注明“待 peer rpc 落地后 BFF 化”。media #433：下载鉴权（accounts 管理员 + message 附件可见性）即此例外，故 media-rpc **写入脱 internal 但仍部分依赖**，media-api 仍是纯透传 BFF 未加 UserRPC。

### 5. tracing 切 go-zero 原生 Telemetry
去 `pkg/observability` tracing 接线，改 go-zero 内置 otel（zrpc/rest 默认拦截器 + `ServiceConf.Telemetry`）；
endpoint 经 yaml `Telemetry.Endpoint: ${AGENTS_IM_OTLP_ENDPOINT}`。metrics 仍用 `observability.MetricsHandler`。
（原标「可选」导致历次迁移普遍漏切——这是必做步骤，别跳。）

### 6. 输入只 validate 不 normalize（仅当原代码有 normalize）
去掉后端规范化（`TrimSpace` 等，由客户端保证）；**保留校验**（required + 长度上限 + 集合大小上限，防脏数据/DoS）。
函数 `normalize*` → `validate*`。**若该域原本就无 normalize，则无可去除、跳过此步**（别为了对齐硬塞 validate）。

## goctl rpc protoc 改 proto 的坑（重要）

`goctl rpc protoc user.proto --go_out=. --go-grpc_out=. --zrpc_out=. --style go_zero` 的全量 scaffold
**路径/命名可能与本仓库现状不符**：会把 `.pb.go` 生成进嵌套 `github.com/...` 目录、server 出 `user_server.go`
（仓库是 `userserver.go`）、client 出 `user_client/`（仓库是 `userclient/`）。安全做法：
- **只取 `.pb.go` + `_grpc.pb.go`** 覆盖现有 `<svc>/<pkg>/` 下的两个文件（内容正确，仅 `status.Error`↔`Errorf` 等生成器版本漂移）；
- **server/client 手工补**新方法（dispatch + interface + impl），照现有文件命名；
- 新 logic stub 修正 import path（goctl 写的是嵌套 path），改成 `userpb "…/service/<svc>/<pkg>"`；
- 删掉 scaffold 的 `github.com/`、`user_client/`、`user_server.go`、多余 `etc/*.yaml`。
- 手工补的部分在 §剩余/后续 记 TODO，下次跑 goctl 前先确认 proto 已含该 rpc 再由 goctl 校正。

## 配套改动清单（删 core / 转 Postgres-only / BFF 必改，易漏）
代码外的配套散落多处，少改一处就本地/CI 不一致：
- **rpc 配置 3 份**：`etc/<svc>-rpc.yaml`（binary 默认 + verify-static 检查）、`deploy/k8s/etc/<svc>-rpc.yaml`
  （prod configmap，经 `deploy/k8s/kustomization.yaml`）、`service/<svc>/rpc/etc/<svc>.v1.yaml`（残留）。转 Postgres-only 三处都删 `StorageDriver`。
- **api 配置**同上 3 份：加 `UserRPC`（及其它 rpc client）。
- **`scripts/dev-up.sh`**：`write_<svc>_rpc_config`（Postgres-only）、给 api 配 rpc client + UserRPC、`start_service "<svc>-rpc"`。
  （friends 之前根本没起 friends-rpc 且 api 无 rpc 配置，是潜在断点——顺手补。）
- **`scripts/verify-static.sh`**：`rpc_logic_markers` 检查每域 `svcCtx.<Marker>`，删 core 后把旧 `<Domain>Logic` 改成新 `<Domain>Model`，否则 CI 红。
- **e2e/test**：原来注入 `core.*Logic` 的（如 `test/e2e/single-machine`）改直接调 `repository`。

## 交付（按 CLAUDE.md 工作流）
issue → worktree（`.claude/worktrees/<branch>`，从 main）→ commit → PR → merge（`--delete-branch`；本地 main 被
worktree 占用导致 gh 报 “main already used by worktree” 属正常，merge 已成功，事后 `git worktree remove`）→
`bash scripts/drone-watch.sh`（后台监控 CI）→ prod 冒烟 → 更新进度文档（逐域表 + §剩余/后续）。

**本地验证坑**：
- 跑迁移/集成测试要 psql：`go test -tags=integration ./tests` 内部调 `migrate-postgres.sh --host-psql`
  （macOS 把 `/opt/homebrew/opt/libpq/bin` 加进 PATH）；goctl 取 schema 用 docker-compose PG（`POSTGRES_PORT=5433` 临时库）。
- `scripts/verify-static.sh`、`test-deploy-k3s.sh` 在 main 上本地就会 fail（缺 psql/ripgrep、依赖 git refs，且原生
  Telemetry 迁移后 `TraceMiddlewareFunc` 等检查会失真）——别追幽灵，只确认你这域相关的检查通过，真正门控是 Drone。
  发现确属失真的检查（如某域已迁原生 Telemetry 仍要求 `TraceMiddlewareFunc`）顺手在同 PR 修正。

## 验收清单
- [ ] 该域 rpc `internal/{config,svc,logic}` 无 `internal/logic`、`internal/repository` import
- [ ] 数据层在 `service/<domain>/rpc/internal/model`（goctl + custom），`*_gen.go` 未手改
- [ ] 跨域数据在 api(BFF) 聚合，rpc 之间不互调；列表补全用批量接口无 N+1
- [ ] tracing 切 go-zero 原生 Telemetry（去 `pkg/observability` 接线，必做）
- [ ] 输入 `validate` 不 `normalize`（仅当原有 normalize）；logic 依 model 接口 + fake 单测
- [ ] monolith 仍消费的部分保留并注明“待 message 迁移后删”
- [ ] 配套改动清单逐项过：配置 3 份去 `StorageDriver`/加 client、`dev-up.sh`、`verify-static.sh` marker、e2e
- [ ] build/vet/test 全绿；diff 无无关 gofmt 噪音；Drone CI 绿 + prod 冒烟
- [ ] 同 PR 更新 `docs/refactor/v1/progress/02-microservices.md`

## 已迁移域（更新此表）
| 域 | 路线 | owner 落点 | 数据层 | PR | 备注 |
|----|------|-----------|--------|----|------|
| media | **goctl + BFF** | `service/media/rpc/internal/logic`（删 `core`）| **`service/media/rpc/internal/model`（goctl）** | #401→#433 | #433 写入脱 internal；下载鉴权（accounts 管理员 + message 附件可见性）**无 message-rpc 可 BFF 化**仍读 internal/repository（部分仍依赖）；`internal/mediavalidate` 留喂 message monolith + user-rpc 头像校验 |
| groups | **goctl + BFF** | `service/groups/rpc/internal/logic` | **`service/groups/rpc/internal/model`（goctl）** | #415/#416 | 首个 rpc 数据层脱 internal；BFF 聚合 user-rpc；批量接口 #423 |
| friends | **goctl + BFF** | `service/friends/rpc/internal/logic` | **`service/friends/rpc/internal/model`（goctl）** | #426 | 由 core 退役改造；`friendships` 加代理 PK（迁移 018）；BFF 聚合 user-rpc 批量 `GetUsersByIDs`；internal/repository 好友方法暂留喂 monolith |
| auth | **特性改造（非数据层退役）** | `internal/auth/logic`（仍 keystone）| 未迁（credentials/email_verification 仍 internal）| #435 | 活跃会话 jti+Redis、共享 `common/middleware.DeviceAuth`（store-only，从 context 读 `user_id`/`session_id`/`device_type`）；**go-zero `jwt:Auth` 丢弃 sub/jti 注册声明** → token 镜像非注册声明；goctl 数据层迁移待独立 PR |
