# 06 — 其他横切技术债

> 目标：把没归到文档 01~05 但仍然影响项目可维护性的横切问题集中整理。
>
> 范围：repository 组织、错误码、context user、Snowflake/ID、object storage、media、test 组织、文档治理、env/config、依赖管理、安全。
>
> **路径约定**：本文中 `internal/apperror/`、`internal/response/`、`internal/ctxuser/`、`internal/idgen/`、`internal/config/`、`internal/objectstorage/`、`internal/repository/` 等是**当前真实位置**的事实描述。按 00-decisions **D10**，重构后：
> - 跨服务基础设施（apperror / response / ctxuser / idgen / config / objectstorage）→ `pkg/<name>/`
> - 按域数据访问 → `service/<domain>/rpc/internal/model/`（goctl model，**无 repository 层**，00-decisions **D13**）
> - 顶层 `internal/` 整体退役。
>
> 本文 §1 / §10 中的修复方向以此 D10 落点为准。

---

## 1. internal/repository/ 50+ 文件扁平化

### XC-1 🚨 平铺一层，无域归属
```
internal/repository/
  admin_repository.go
  agent_audit_memory.go
  agent_audit_repository.go  agent_audit_repository_test.go
  agent_hosting_memory.go
  agent_hosting_repository.go agent_hosting_repository_test.go
  agent_memory.go
  agent_registry_memory.go
  agent_registry_repository.go
  agent_repository.go
  conversation_ai_hosting_memory.go
  conversation_ai_hosting_repository.go conversation_ai_hosting_repository_test.go
  delivery_attempt_repository.go
  feedback_repository.go
  groups_memory.go
  groups_repository.go
  media_memory.go
  media_repository.go
  memory.go
  message_enums.go
  message_memory.go
  message_outbox_repository.go              ⚠️ 00-decisions D1：弃用，Phase 0 删除
  message_repository_contract_test.go
  message_repository.go
  message_storage_contract.go
  message_validation.go
  postgres_account_profiles_test.go
  postgres_agent_audit.go
  postgres_agent_hosting.go postgres_agent_hosting_test.go
  postgres_agent_registry.go
  postgres_agent.go
  postgres_common.go
  postgres_conversation_ai_hosting.go
  postgres_errors.go
  postgres_feedback.go
  postgres_groups.go
  postgres_media.go  postgres_media_test.go
  postgres_message.go  postgres_message_test.go
  postgres_outbox.go                        ⚠️ 00-decisions D1：弃用，Phase 0 删除
  postgres_task_report.go postgres_task_report_test.go
  postgres_user_friends.go
  repository.go
  schema_v2_enums.go
  storage_factory_test.go
  task_report_repository.go
```

观察：
- 50+ 文件全在一层；
- 文件命名两个风格：`<domain>_repository.go` + `<domain>_memory.go`（按 driver）vs `postgres_<domain>.go`（前缀 postgres，更老的风格）；
- `_test.go` 散落其中。

### XC-2 ⚠️ Memory implementation 和 production code 同包
所有 `*_memory.go` in-memory 实现都和 production postgres impl 在同一包，意味着：
- production binary 也会编译 in-memory 实现（虽然 Go linker 能 dead-code eliminate 但符号还在）；
- 单元测试和 production 共用同一个 import path。

> 修复：按域拆 + driver 拆（**outbox 已弃用，不在目标分包内**，见 00-decisions D1）：
> ```
> internal/repository/
>   message/{repo.go, memory.go, postgres.go}
>   agent/{repo.go, memory.go, postgres.go, audit.go, registry.go, hosting.go}
>   group/...
>   friend/...
>   account/...
>   media/...
>   feedback/...
>   taskreport/...
>   deliveryattempt/...
> ```
> 或者更彻底，跟随文档 01 / D13 把每个域的数据访问改为 goctl model 落 `service/<domain>/rpc/internal/model/`（无 repository 层），internal/repository 退役。

### XC-3 ⚠️ `schema_v2_enums.go` + `message_enums.go` 在 repo 层
枚举本应在 domain 层（`internal/domain/<dom>/`），现在塞在 repository 包里，让 repository 又当持久化又当 type system。

---

## 2. apperror / response / 错误码

### XC-4 ✅ 错误码体系基本合理
`internal/apperror/` 定义了 9 个 Code，`internal/response/Envelope{Code,Message,Data,TraceID,RequestID}` 统一 HTTP 响应。这是好的。

### XC-5 ⚠️ RPC 错误传递不一致
`internal/rpcgen/rpcerror/` 与 `internal/apperror/` 双轨。在 RPC 客户端调用结果反序列化时，错误码丢失（gRPC 自己的 status code 与 apperror.Code 不直接映射）。

> 修复：写一个 `apperror.FromGRPCStatus(err) → *apperror.Error` 互转，所有 RPC 客户端 wrap 一次。

### XC-6 ⚠️ 没有错误码字典
9 个 Code 远不够。`apperror.InvalidArgument("x is required")` 一句话错误的 Message 是英文 + 内部信息，前端展示不友好；没有 error_code/error_subcode 双层结构。

> 修复：错误码升级为 `{code: INVALID_ARGUMENT, subcode: MISSING_FIELD, field: "user_id", message_i18n_key: "errors.missing.user_id"}`。这是 P2，不阻塞当前重构。

### XC-7 ⚠️ trace_id 注入只在错误响应里
`response.WriteError` 注入 trace_id 到 envelope，`WriteOK` 不注入：

```go
if status >= http.StatusBadRequest {
    envelope.TraceID = ...
}
```

成功响应也应当带 trace_id，方便客户端记日志。

---

## 3. ID / Snowflake

### XC-8 ⚠️ Snowflake worker_id 来源
`internal/idgen/snowflake.go` 是单文件实现。但 worker_id 怎么分配？多进程同 worker_id 会冲突。

> 修复：从 Redis INCR 取 worker_id，或者从 hostname hash。审计当前实现。

### XC-9 ⚠️ account_id 是 string 还是 int64？
- `accounts.account_id` PG 列是 bigint；
- API/RPC 一律 `string`；
- `db/migrations/013_internal_agent_ids_bigint.sql` 暗示曾经 agent_id 不是 bigint，最近才改的；
- domain model 大量字符串到处转 `strconv.ParseInt`。

> 修复方向：domain layer 用 `int64`，wire format（JSON/proto）用 string（避免 JS number 精度问题）；统一在 convert 层转换。

---

## 4. ctxuser / Auth context

### XC-10 ⚠️ ctxuser 只有一个文件
`internal/ctxuser/user.go` 提供 `WithUserID(ctx, id)` 和 `UserID(ctx)`。当前已被 handler / logic 大量使用。OK。

### XC-11 ⚠️ JWT claims 解析与 ctxuser 没融合
msggateway 的 JWT handshake 解 claims 后调 `ctxuser.WithUserID`；REST 请求也走 middleware → ctxuser。但 claims 里的 `identifier`、`account_type` 等没传到 context，业务代码要用时只能再查一次 user-rpc。

> 修复：扩 `ctxuser.User{ID, Identifier, AccountType, ...}` 并在 middleware 一次塞完。

---

## 5. Object Storage / Media

### XC-12 🚨 Media 寄生在 user-api
按 `ARCHITECTURE.md`："第一阶段不新增独立 media-api 进程；媒体上传意图、上传完成校验、下载 URL 和头像绑定作为受保护 REST 路由挂在 user-api 上"。

后果（历史）：
- user-api 既负责账号又负责媒体（avatar/image/file upload）；
- media routes 曾挂在 `internal/handler/media/`（user-api 的 `addMediaRoutes`）；
- 同时 message 里的图片/文件消息也要访问 `media_objects` 表做 owner 校验，msg-rpc 直接读 media repo。

> ✅ 已修复（#381，B 方案）：拆出 `service/media/{api,rpc}` 独立服务，ingress `/media` → media-api:8089（4 条路由 live）。Stage 4 Layer 1 已删孤儿 `internal/handler/media/` 与 message-api #380 临时头像路由+objectStore。
> ✅ Stage 4 Layer 2（#389）：已删 usersvc legacy MediaLogic plumbing（`internal/servicecontext/user` 的 `NewServiceContextWithMedia`/`ConfigureMediaAttachmentAccess`/`MediaLogic` 等字段）、legacy `internal/logic/user` 头像逻辑、孤儿 `internal/logic/media/*`，连同 user/auth/friends/groups 仅测试用的 monolith handler/logic/servicecontext 与 gozero 测试一并清理。
> 仍保留：message 媒体 owner 校验（`internal/logic/medialogic.go` `ValidateMessageMedia`）与 media-rpc 复用的 `internal/logic.MediaLogic`。

### XC-13 ⚠️ MinIO 与 PG `media_objects` 一致性
当前流程：客户端预签名上传 MinIO → 客户端调 complete_upload → user-api 校验 → 写 `media_objects` 表。如果客户端上传成功但没回调 complete_upload，MinIO 里会留孤儿对象。

> 修复方向：MinIO event notification → cleanup worker；或者每天 reconcile job 扫 orphan。

### XC-14 ⚠️ objectstorage 抽象 OK
`internal/objectstorage/{factory.go, memory.go, minio.go, store.go}` 抽象合理。Memory implementation 用于测试。继续保留。

---

## 6. 测试组织

### XC-15 ⚠️ tests/ 目录 16+ 个 _test.go 单文件
```
tests/
  agent_service_test.go
  auth_service_test.go
  friends_service_test.go
  gateway_contract_test.go
  groups_service_test.go
  message_service_test.go
  mvp_backend_test.go
  no_shell_execution_test.go
  postgres_persistence_integration_test.go
  read_receipts_test.go
  user_account_type_test.go
  user_service_test.go
  websocket_gateway_internal_delivery_test.go
  websocket_gateway_test.go
  ...
  e2e/
  fixtures/
  ci/
```

平铺，没有 e2e/integration/contract 分层。

> 修复：
> ```
> tests/
>   integration/     (跨 service 集成测试，依赖真实 PG)
>   contract/        (proto/api 契约测试)
>   e2e/             (端到端，启全部服务)
>   smoke/           (部署后烟雾测试)
>   fixtures/        (共享测试数据 builder)
> ```

### XC-16 ⚠️ no_shell_execution_test.go 是仓库级 lint
`tests/no_shell_execution_test.go` 应是禁用 shell 调用的检查测试，但它放在 tests 目录像普通测试。

> 修复：搬进 `scripts/verify-static.sh` 调用的脚本，作为 CI lint。

### XC-17 ⚠️ Memory 实现 + Postgres 实现的契约测试不完整
`internal/repository/message_repository_contract_test.go` 试图保证两种 driver 行为一致，但仅 message_repository 有 contract test，其他 repository 都没。

> 修复：所有 repository 加 contract test，新增 driver 自动覆盖。

---

## 7. 文档治理

### XC-18 🚨 docs/ 体积过大且重叠
```
docs/
  AGENT_GIT_STANDARD.md             10K
  AGENTIC_DEVELOPMENT_WORKFLOW.md   11K
  CODEX_RUN_LOG.md                  7K
  DESIGN.md                         254B（基本是 stub）
  DEVELOPMENT.md                    9K
  FRONTEND.md                       15K
  GIT_WORKFLOW.md                   16K
  PLANS.md                          2K
  PRODUCT_SENSE.md                  1K
  QUALITY_SCORE.md                  858B
  RELIABILITY.md                    4K
  SECURITY.md                       2K
  github-project-init.md            1K
  deployment-k3s-pitfalls.md        7K
  agent-ops/                        协作模型
  design-docs/                      45+ 设计文档
  exec-plans/active/                25 个 active plan
  exec-plans/completed/             34 个 completed plan
  product-specs/                    16 个 spec
  qa/                               
  references/
  superpowers/
  templates/
  workspace-migration/
  generated/
  metrics/
```

观察：
- 文档总数已经超过 100；
- `AGENTS.md` 在仓库根，`AGENT_GIT_STANDARD.md` 在 docs/，`docs/agent-ops/` 又一份协作模型 → 3 处约束 agent 行为；
- design-docs 45 份中，message 链路相关 10 份、gateway 5 份、auth 3 份、agent 8 份，**信息分散**；
- exec-plans/active 25 个有的是 4 个月前的，状态没维护。

> 修复：
> 1. exec-plans 加 `status` frontmatter，CI 检查 active 状态如果 > 90 天 stale 自动 flag；
> 2. 同一主题的 design-docs 合并（message-* 系列 5 份合 1~2 份）；
> 3. 仓库根的 `AGENTS.md`、`CLAUDE.md` 只保留 quick reference + 链接到 docs/。

### XC-19 ⚠️ Markdown 风格不一致
有的文档中文为主、有的全英文、有的中英混排。docs/exec-plans 大部分是英文，product-specs 一半中文。

> 修复：约定主语言；建议中文为主（项目 PRIMARY 文档），英文用于 commit/PR/code comments。

### XC-20 ⚠️ docs/superpowers / docs/templates 内容模糊
不知道这两个目录是什么用途，分别只有 1~2 个文件。

> 修复：归并或删除。

---

## 8. Config / Env

### XC-21 ⚠️ Config 字段在 yaml + env + go struct 三处定义
`internal/config/config.go` 定义 struct；`etc/*.yaml` 提供默认值；`.env.example` 提供环境变量。三处必须同步。

任何新字段：
1. 改 struct；
2. 改 13 个 yaml；
3. 改 env.example；
4. 改 deploy/k8s/configmap.yaml；
5. 改 deploy/k8s/secrets.example.yaml（如果是秘密）。

5 个地方易漏。

> 修复方向：
> - struct 标签里直接声明 yaml + env 双绑定（如 `viper`）；
> - configmap 用模板生成；
> - 写一个 audit 脚本对照 struct 与 yaml 字段。

### XC-22 ⚠️ Env placeholder 展开 bug 史
最近 #301 #302 都是修 env placeholder 展开问题。说明配置加载层有 bug 风险。

> 修复方向：写一份 config loader contract test，覆盖：
> - `${VAR}` 展开；
> - `${VAR:-default}`；
> - 嵌套 yaml 字段；
> - 数字/bool 类型转换。

### XC-23 ⚠️ Secret 通过 envFrom 全量注入
`deploy/k8s/deployments.yaml` 每个 deployment：
```yaml
envFrom:
  - configMapRef:
      name: agents-im-config
  - secretRef:
      name: agents-im-secrets
```

所有服务拿到 **所有 secret**——auth-api 也能看到 `LANGFUSE_SECRET_KEY`，agent-api 也能看到 `KAFKA_BROKERS` 凭证。

> 修复方向：拆 secret 为 per-service，envFrom 改为精确 secretKeyRef。

---

## 9. 依赖管理

### XC-24 🟡 go.mod 单 module
单仓库单 module，OK 简化。但未来要不要拆 service module 独立？

> 决策：暂不拆。单 module 让 cross-service refactor 简单，是优势。

### XC-25 ⚠️ 第三方依赖审计
- `github.com/cloudwego/eino` 是字节开源、相对新；
- `github.com/eino-contrib/jsonschema`；
- `github.com/segmentio/kafka-go`；
- `github.com/redis/go-redis/v9`；
- `github.com/wujunhui99/agents_im` 自身。

> 修复：跑一次 `go mod why` 和 `govulncheck`，列出可去掉的依赖。

---

## 10. 安全

### XC-26 🚨 `dev-jwt-secret-change-me` 是所有 yaml 默认值
`etc/*.yaml` 中：
```yaml
Auth:
  AccessSecret: dev-jwt-secret-change-me
```

如果生产部署忘记覆盖 env，服务用这个 secret 启动。攻击者签发任意 JWT。

> 修复：
> 1. 启动时检查 `AccessSecret == "dev-jwt-secret-change-me"` 在 production environment 必须 fail-first；
> 2. 部署 manifest 强制 secret ref，不接受 yaml 默认。

### XC-27 ⚠️ secret/ 目录约束清晰但易踩
`AGENTS.md` 反复强调 secret/ 是 operator-local，但仓库 root 仍有 `secret/` 目录被 commit（应该只有 .gitignore + .example）。

> 修复：CI lint 强制 `secret/` 只允许 `.gitignore`、`README.md`、`*.example`。已经有部分规则，加强 enforcement。

### XC-28 ⚠️ Python sandbox 默认 disabled
`PythonExecutor.Backend = disabled` 是 fail-safe 默认，OK。但生产部署 k8s backend 时，namespace、image、ServiceAccount、RuntimeClass 都来自配置——这些配置错误会让 agent 静默不能跑 python，而不是 fail-first。

> 修复：Agent run 试图 invoke python 但 executor disabled 时必须显式失败（已有 `ErrPythonExecutorDisabled`），但要在 agent run audit 里清晰区分"工具不可用"和"工具调用失败"。

### XC-29 ⚠️ Forbidden identifier list 防漏洞
`internal/agentruntime/tools/resolver.go` forbidden list 是黑名单防御（black-list），永远漏。

> 修复：改为白名单——只允许已注册的 builtin handler_key + 已认证的 MCP server。当前一定程度是这样，但黑名单这层还在多余地存在。

---

## 11. Frontend / 前后端契约（简短）

### XC-30 ⚠️ frontend-backend-contract.md 与 frontend-sync-contract.md 双份
两份契约文档分别在 product-specs/ 和 design-docs/，内容应合并。

### XC-31 ⚠️ WebSocket envelope 与 HTTP envelope 不一致
- HTTP 响应：`{code, message, data, trace_id?}`；
- WebSocket ACK：`{requestId, command, status, error?, payload?}`；

两套字段命名风格不同（code vs status, message vs error.message）。客户端要处理两种 envelope。

> 修复：长期统一；短期至少在 contract 文档里对照清楚。

---

## 12. 优先级总览

按"投入产出比"排序：

| 优先级 | 条目                                                                  | 工作量 |
|--------|-----------------------------------------------------------------------|--------|
| P0     | XC-26 prod 启动检查 dev secret                                         | XS     |
| P0     | XC-23 secret per-service envFrom                                       | S      |
| P0     | XC-22 config loader contract test                                      | S      |
| P0     | XC-2 repository 按域 + driver 拆                                       | M      |
| P1     | XC-15 tests/ 分层                                                      | S      |
| P1     | XC-7 WriteOK 加 trace_id                                               | XS     |
| P1     | XC-21 config 字段同步审计脚本                                          | S      |
| P1     | XC-13 MinIO 孤儿清理                                                   | M      |
| P1     | XC-5 RPC error 互转                                                    | S      |
| P2     | XC-18 docs 治理（exec-plans staleness、design-docs 合并）              | M      |
| P2     | XC-12 拆 service/media                                                 | L      |
| P2     | XC-6 errcode subcode                                                  | L      |
| P2     | XC-9 account_id int64 一致化                                          | L      |
| P3     | XC-20 docs/superpowers 归并                                            | XS     |
| P3     | XC-24 go module 拆分                                                   | XL（暂不做） |

---

## 13. 重构总收尾建议

把 01~06 六份文档作为重构 epic 的设计输入：

- **Epic 1**：项目结构收敛（文档 01 §4 收敛路径）；
- **Epic 2**：四服务双轨清理 + domain model 引入（文档 02）；
- **Epic 3**：消息链路三件套拆分（文档 03 Phase 0~4）；
- **Epic 4**：Agent 模块归位 + agent-rpc（文档 04）；
- **Epic 5**：可观测性 + CI + 部署（文档 05）；
- **Epic 6**：横切清理（本文档 §12 中 P0/P1 项）。

每个 epic 拆为 GitHub Issue，按 `docs/AGENT_GIT_STANDARD.md` 创建分支。

总体节奏建议：
- 第 1~2 周：Epic 1 + Epic 6 P0；
- 第 3~5 周：Epic 2 + Epic 5（OB-1 单轨化先做）；
- 第 6~10 周：Epic 3 Phase 0/1（push 拆分）；
- 第 11~14 周：Epic 4（agent-rpc）；
- 第 15+：Epic 3 Phase 2/3、Epic 5 中间件入 k8s（高风险，留到最后）。
