# 04 — Agent 模块重构分析

> 目标：Agent 相关代码当前分散在 4 个不同 `internal/agent*` 包 + 1 个 `service/agent/` 半成品里，本文按"职责切分"重新画边界，并列出技术债与收敛建议。
>
> **路径约定**：本文中 `internal/agent*`、`internal/agentim/`、`internal/agentruntime/`、`internal/agenteval/`、`internal/llmobs/`、`internal/repository/agent_*.go` 等指 **agents_im 当前**真实位置（重构前）。按 00-decisions **D10**：
> - 跨服务 infra（`pythonexec`、`llmobs`）→ `pkg/<name>/`
> - agent 专属（runtime / trigger / orchestrator / hosting / imadapter / audit / eval）→ `service/agent/rpc/internal/<name>/`
> - 顶层 `internal/agent*` 整体退役。

---

## 1. 现状盘点

### 1.1 代码位置

| 包                              | 文件数 | 职责                                                                                  |
|---------------------------------|--------|---------------------------------------------------------------------------------------|
| `service/agent/api/`            | ~15    | go-zero HTTP API：CRUD agent profile、agent definition (prompt/tool/skill 绑定)         |
| `internal/agent/pythonexec/`    | 6      | Python 沙箱执行器（disabled / k8s 两种 backend）                                       |
| `internal/agentim/`             | 18     | **Agent ↔ IM 编排层**：消息触发、trigger 构建、response writer、conversation hosting、读标记、LLM 观测 hook |
| `internal/agentruntime/`        | 12     | **Agent 推理运行时**：Runtime 接口、Eino+DeepSeek 适配、tool resolver、normalize       |
| `internal/agenteval/`           | 2      | Agent 评估/回归测试 harness                                                            |
| `internal/logic/agent*.go`      | 6      | agent_registry、agent_definition、agentauditlogic、aihostinglogic、default_assistant、admin_ai_replay |
| `internal/repository/agent_*.go`、`postgres_agent_*.go` | ~10 | Agent registry / audit / hosting 的 repo                          |
| `internal/llmobs/`              | 6      | LLM 观测 sink（noop / memory / langfuse）+ Eino callback 桥                            |
| `internal/domain/agentaudit/`   | -      | 审计领域类型                                                                            |

### 1.2 入口

- `service/agent/api`（main）：唯一 Agent 相关进程；
- 但是 `service/message-api`（过渡态扁平目录，也即 admin-bootstrap 寄生进程）的 main 通过 `internal/agentim` 处理"消息进来后是否触发 Agent run"；
- `service/gateway-ws`（过渡态扁平目录）的 main 也 import 了 `internal/agentim`、`internal/agent/pythonexec`（连接层这么干非常不合理，见 03 文档 MP-5）。

```
$ grep -rl "agentim\|agent/pythonexec" service/*/  # 各服务 main 包
service/gateway-ws        ← 不应该（迁后为 msggateway）
service/message-api       ← 不应该（迁后为 msg-api，应改为通过 agent-rpc）
internal/rpcgen/message   ← message-rpc 寄生于此
```

### 1.3 与 IM 的耦合

`internal/agentim/hosting.go` 直接持有 `MessageLogic`（IM 消息写入）；`response_writer.go` 直接调 `MessageLogic.SendMessage` 把 Agent 输出写回 IM。这是"in-process tight coupling"，没有任何 RPC 隔离。

---

## 2. 技术债

### AG-1 🚨 没有 `agent-rpc` 服务
- `service/agent/api/` 完整存在，但没有 `service/agent/rpc/`；
- `ARCHITECTURE.md` 明确写"当前没有真实 Agent RPC/proto contract，不能创建空 RPC scaffold 冒充服务边界"；
- 结果是 msg-api / msggateway / msg-rpc 都直接 import `internal/agentim` 来跑 Agent；**Agent 没有独立服务边界**。

> 修复方向：定义 `proto/agent.proto`，至少包含：
> - `RunAgent(RunAgentRequest) returns (RunAgentResponse)`（trigger from message event）
> - `GetConversationHostingConfig`
> - `RecordAgentRunAudit`
>
> 让 msg-rpc 通过 `agentservice.AgentService.RunAgent` 调用，而不是 in-process 跑 runtime。

### AG-2 🚨 `internal/agentim/` 命名误导
名字像"agent 的 IM 集成"，实际包含：
- trigger builder（消息→agent run trigger）：纯 agent 侧；
- conversation hosting：agent 侧配置；
- response writer：写回 IM（**反向耦合 message logic**）；
- runner / orchestrator：agent run 执行流程；
- LLM observability adapter；
- read marker：IM 已读语义。

这些拆得不清，且都在 internal/agentim 一层。

> 修复：在 `service/agent/rpc/internal/` 下重新组织：
> ```
> service/agent/rpc/internal/
>   runtime/        (调 agentruntime 跑 LLM)
>   trigger/        (消息触发 → trigger struct)
>   hosting/        (会话托管配置、幂等)
>   audit/          (run audit、tool call audit)
>   orchestrator/   (run lifecycle 编排)
>   imadapter/      (写回 IM 的 client，调用 msg-rpc gRPC)
> ```
> `internal/agentim/` 整个删掉。

### AG-3 ⚠️ `agentruntime` vs `agentim` 边界模糊
两者都在 internal/，都跟 agent run lifecycle 相关；agentruntime 是"纯推理"，agentim 是"运行时编排 + IM 反写"，但接口面没有正交：
- `agentruntime.Runtime.Run(ctx, req)` 输入是 RunRequest；
- `agentim.AgentRunOrchestrator` 也叫"Run"；
- `agentim.RuntimeRequestBuilder` 又把 trigger 转 RunRequest。

阅读路径：trigger → builder → orchestrator → runtime → tools。一个 run 走了 5 个抽象层，且都在 internal/。

> 修复：把 agentruntime 下沉为 `service/agent/rpc/internal/runtime/`（00-decisions D10：顶层 internal 退役；agent 专属逻辑不进 pkg/），保留"纯 LLM runtime adapter"职责；orchestrator/trigger/builder 与之同栋（`service/agent/rpc/internal/{trigger,orchestrator,hosting,imadapter,audit}/`）。

### AG-4 ⚠️ `internal/agent/pythonexec` 命名误导 + 寄生 agent
- 包路径叫 `internal/agent/`（暗示 agent 服务），里面**只**有 `pythonexec/`；
- 实际上 Python 沙箱可以给 message logic、admin、未来任何 service 用，不专属 agent；
- msggateway 都 import 它就更不合理。

> 修复：rename `internal/agent/pythonexec/` → `pkg/pythonexec/`（00-decisions D10：跨服务沙箱执行属于 infra，归 pkg）。

### AG-5 ⚠️ Tool resolver 在 `agentruntime/tools/`，但 forbidden_identifier 黑名单硬编码
`internal/agentruntime/tools/resolver.go` 第 14 行起 hardcode 一长串 forbidden 字符串（"shell", "stdio", "exec", "command", "script", "filesystem.write", 甚至 `"py"+"thon"` 用拼接绕开 grep）：

```go
var forbiddenIdentifierFragments = []string{ ..., "py"+"thon", }
```

`"py"+"thon"` 拼接是为了"防止本仓库的 grep 把这个文件误报为禁词"——是合理 hack，但完全没注释为啥；新人改代码会困惑。

> 修复：黑名单从 hardcode 改为 config-driven；或者最起码加一个注释解释为何字符串拼接。

### AG-6 ⚠️ Agent registry / definition / audit 三套 repo 各自一堆 _test
- `internal/repository/agent_audit_repository.go` + `postgres_agent_audit.go`
- `internal/repository/agent_hosting_repository.go` + `postgres_agent_hosting.go` + memory
- `internal/repository/agent_registry_repository.go` + memory + postgres_agent_registry
- `internal/repository/agent_repository.go` + agent_memory + postgres_agent
- `internal/repository/conversation_ai_hosting_repository.go` + memory + postgres

5 个 agent 相关 repo 全部散落在 `internal/repository/` 平铺一层，每个都有 in-memory + postgres 两种实现。

> 修复（D13）：改为 goctl model 落 `service/agent/rpc/internal/model/`（agent / registry / audit / hosting / convhosting 各表），**无 repository 层**；数据层测试用 sqlmock / 容器 PG，业务规则测试对 model 接口打桩。

### AG-7 ⚠️ LLM observability 与 system tracing 双链路
- `internal/llmobs/`：写 Langfuse（业务可观测：run/generation/tool_call 摘要）；
- `internal/observability/`：OpenTelemetry → Tempo（系统级 span）。

两个独立，自有 sink 抽象。`ARCHITECTURE.md` 也明确写"LLM observability is separate"。这是合理的，但：
- 现在 langfuse export 是 inline HTTP POST + 单事件，没有 batching/queueing → 高频生成会拖慢 Agent run；
- noop sink 默认开，但 Production 没切到 Langfuse 时**没有任何 alert**。

> 修复：langfuse sink 走异步队列；缺 config 时 fail-first（已有 `ErrLangfuseConfigMissing` 但只在 NewLangfuse 时返回，runtime 用 noop 不会触发）。

### AG-8 ⚠️ Agent 工具体系不完整
当前只有：
- `python_execute_adapter`（pythonexec），但 backend=disabled 默认；
- `agent_create_adapter`（让 agent 创建 agent，很危险）；
- MCP tool 元数据建模了但没执行。

MCP 服务发现、tool 输入输出 schema 校验、tool call audit 都在 contract 层，没有真实远端 MCP server adapter。

> 这是 P1：要不要在重构里推进、还是冻结现状只做 cleanup，是个产品决策。

### AG-9 ⚠️ Eino + DeepSeek 单一 ChatModel 锁死
`internal/agentruntime/eino/deepseek_runtime.go` 整体围绕 DeepSeek 配置。未来要切 Claude、OpenAI、Qwen 等，需要重写 runtime 而不是注入 model。

Eino 本身支持 ChatModel 抽象 — 但适配层把它和 `config.DeepSeekConfig` 焊死。

> 修复：抽 `ProviderConfig` enum + `ChatModelFactory(provider, config)`；删除 `deepseek` 在文件名里。

### AG-10 🟡 agent-api 直接持有 PythonExecutor / AgentLogic 在 svc
看 `service/agent/api/internal/svc/service_context.go`：
- `AgentLogic *logic.AgentLogic`、`AgentRepo`、`PythonExecutor` 都挂在 api 的 ServiceContext；
- api 直接调 logic，没经过 rpc——违反 `docs/design-docs/go-zero-service-layout.md` "API 不允许直接操作数据库"。

> 修复：所有数据库访问交给 future `service/agent/rpc`；api 只做 BFF。

### AG-11 🟡 `internal/agenteval/eval.go` 孤立
评估 harness 与生产代码不解耦，目前 2 个文件，不知道被谁用。

> 修复：搬到 `tests/agent-eval/`，或者明确归入 `service/agent/rpc/internal/eval/`，让它成为 Agent service 的功能而不是孤立模块。

### AG-12 🟡 admin_ai_replay 业务在 internal/logic
`internal/logic/admin_ai_replay.go` 是 admin console 的功能（管理员回放 Agent run），但放在 internal/logic 通用层，没有归属。

> 修复：搬进 `service/admin/api/internal/logic/replay/`（admin 服务独立后，见 01 文档 TD-1）。

### AG-13 🟡 Default Assistant 工具种子在迁移历史里
`db/migrations/009_default_agent_creator_assistant.sql`、`010_default_assistant_python_execute_tool.sql`、`011_default_assistant_agent_create_tool.sql`、`012_agent_create_tool_schema_relaxation.sql` 四个 migration 都在配 default assistant 的 tool 绑定。说明工具绑定是"种子"性数据，但通过 migration 维护非常脆——schema 改动要小心保留这些种子。

> 修复：把 seed 数据移出 migration，改成幂等的一次性 seed 命令（放 `test/e2e/` 同级的工具目录或 admin 服务的子命令）或者 admin-bootstrap 启动时 ensure。`internal/adminbootstrap/` 已经有类似逻辑，可以扩展。

---

## 3. 目标架构

```
                      ┌──────────────────────────────────────┐
                      │       service/agent/api              │
                      │  CRUD profile / definition / list    │  REST
                      └──────────┬───────────────────────────┘
                                 │ grpc
                                 ▼
┌────────────────────────────────────────────────────────────┐
│              service/agent/rpc                              │
│  ┌───────────┐  ┌──────────┐  ┌───────────┐  ┌──────────┐  │
│  │ trigger   │  │ runtime  │  │ orchestr. │  │ hosting  │  │
│  └─────┬─────┘  └─────┬────┘  └─────┬─────┘  └────┬─────┘  │
│        │              │              │            │        │
│  ┌─────▼─────────────▼──────────────▼────────────▼─────┐  │
│  │             repository (agent, registry, audit,     │  │
│  │             hosting, conv_hosting)                  │  │
│  └─────────────────────────────────────────────────────┘  │
└──────┬───────────────────────────────────────────────┬─────┘
       │ chat                                          │ write back
       ▼                                               ▼
┌──────────────┐                              ┌──────────────┐
│  agentruntime│ (internal/agentruntime/)     │ msg-rpc  │
│   Eino +     │                              │ (IM write)   │
│   LLM        │                              └──────────────┘
│   provider   │
└─────┬────────┘
      │ tools
      ▼
┌──────────────────────────────────────┐
│  internal/agentruntime/tools/        │
│  resolver / catalog / adapters       │
└──┬─────────┬─────────┬───────────────┘
   │         │         │
   ▼         ▼         ▼
internal/  agent_   mcp_
pythonexec create   adapter
           adapter  (future)
```

### 3.1 边界

| 包                                 | 边界                                                                  |
|------------------------------------|-----------------------------------------------------------------------|
| `service/agent/api/`               | BFF only。鉴权、入参校验、调 agent-rpc。无 DB                          |
| `service/agent/rpc/`               | 业务真相。Agent CRUD + run orchestration + audit。**数据库主**         |
| `service/agent/rpc/internal/runtime/`       | 纯 LLM runtime adapter。提供 ChatModel/Provider 抽象。**无 DB、无 IM 反写** |
| `service/agent/rpc/internal/runtime/tools/` | tool 解析 + adapter。**无 DB**（catalog 通过参数注入）                  |
| `pkg/pythonexec/`                  | 沙箱执行。**只跑 python**，不关心 agent（00-decisions D10）            |
| `pkg/llmobs/`                      | LLM 观测 sink。批量异步发送（00-decisions D10）                        |
| `service/agent/rpc/internal/imadapter/`     | 写回 IM 的 client。**唯一**调 msg-rpc 的位置                  |

### 3.2 关键 RPC contract（草案）

```protobuf
service AgentService {
  // 由 message event handler 调用
  rpc RunFromMessage(RunFromMessageRequest) returns (RunFromMessageResponse);

  // admin 控制台主动触发
  rpc RunManual(RunManualRequest) returns (RunManualResponse);

  // 会话托管查询
  rpc GetConversationHosting(GetConversationHostingRequest) returns (GetConversationHostingResponse);
  rpc UpdateConversationHosting(UpdateConversationHostingRequest) returns (UpdateConversationHostingResponse);

  // Agent CRUD（被 agent-api 调用）
  rpc CreateAgent(CreateAgentRequest) returns (Agent);
  rpc GetAgent(...) returns (Agent);
  rpc ListAgents(...) returns (ListAgentsResponse);
  rpc UpdateAgent(...) returns (Agent);
  rpc UpdateAgentDefinition(...) returns (AgentDefinition);

  // Run audit 查询
  rpc ListAgentRuns(...) returns (ListAgentRunsResponse);
}
```

---

## 4. 触发路径重构

### 4.1 现在（重构前现状）

```
msg-rpc.SendMessage()
  ├ writes postgres + outbox          ⚠️ 重构后此步骤消失（00-decisions D1/D2）
  └ calls hostingService (in-process) ──> agentim.RunOrchestrator
                                              ├ agentruntime.Run
                                              └ message logic.SendMessage (写回 AI 消息)
```

问题：写回 AI 消息发生在 msg-rpc 的 in-process call 链里，意味着：
- msg-rpc 写消息要等 Agent run（LLM 几秒）；
- Agent run 失败会拖累 msg-rpc；
- 实际通过 hook 异步化了一部分（`MessageCreatedHook`），但仍同一进程内。

### 4.2 目标（重构后）

按 00-decisions D1（无 outbox）+ D2（seq 在 transfer 分配）+ D5（topic 命名）：

```
msg-rpc.SendMessage()
  └ producer.Publish(topic=msg.toTransfer.v1, key=conversation_id, value=proto.Marshal(MsgData))
                                       │ ACK 立刻返回（不带 seq）
                                       ▼
                  msgtransfer batcher ──> categorize 阶段
                                       │   - 判断 message_origin
                                       │   - 判断 conversation 是否 hosted by agent
                                       │   - 判断是否 @ Agent
                                       │   若需要触发 agent ↓
                                       │
                                       ├─ produce msg.toPostgres.v1（归档）
                                       ├─ produce msg.toPush.v1（推送）
                                       └─ produce agent.trigger.v1（触发 agent）
                                                  │
                                                  ▼
                                          agent-rpc worker 消费
                                                  │
                                                  ├ runtime.Run（LLM + tools）
                                                  └ imadapter → msg-rpc.SendMessage（AI msg 再走完整链）
```

好处：完全异步 + 可独立扩容 agent worker + agent 故障不影响消息写入。
代价：trigger 链增加 1 hop kafka 延迟（百毫秒级，可接受）。

> 渐进路径：先在 msg-rpc 内同步走（现状）→ 引入 `service/agent/rpc/internal/orchestrator/` 后台 worker channel → 上 Kafka topic `agent.trigger.v1`（在 03 文档 Phase 2/3 完成后落地；00-decisions D10）。

---

## 5. 收敛 epic

按依赖顺序：

1. **AG-4 rename** `internal/agent/pythonexec` → `pkg/pythonexec`（00-decisions D10），纯 move。
2. **AG-5 forbidden 黑名单加注释 + config-driven**。
3. **AG-1 建 service/agent/rpc**：定义 proto、生成代码、初始化 svc/server。
4. **AG-2/AG-3 拆 internal/agentim**：搬到 `service/agent/rpc/internal/{trigger,orchestrator,hosting,imadapter,audit}/`。
5. **AG-6 数据层改 model**（D13）：agent_* 表全部改 goctl model 落 `service/agent/rpc/internal/model/`，废 repository 层。
6. **AG-9 LLM provider 抽象**：`service/agent/rpc/internal/runtime/llm/{factory.go, deepseek, openai, anthropic}`（00-decisions D10）。
7. **AG-10 agent-api 改 BFF only**：删 PythonExecutor、AgentLogic 在 api svc 的依赖，全部走 agent-rpc。
8. **AG-13 default assistant seed 改 ensure**：从 migration 抽出。
9. **AG-12 admin_ai_replay 搬 admin 服务**：依赖 01 文档拆 admin。
10. **AG-11 agenteval 归位**。

---

## 6. 风险

- Agent run 路径异步化（§4.2）改动较大，要保证：
  - trigger event 含全量 trigger struct（不丢字段）；
  - agent run audit 仍以 trigger event_id 为 idempotency key；
  - 失败重试不能造成重复 Agent message（hosting 表已有幂等记录，要复用）。
- `agent-rpc` 上线意味着 msg-rpc 多一个 RPC client 依赖，部署顺序：先 agent-rpc，再 msg-rpc 升级。
- 黑名单 hardcode → config 切换时，确保旧实例 fail-first 而不是默认放行。

---

## 7. 与文档 03 的交集

文档 03（消息链路）按 OpenIM 模型在 msgtransfer batch handler 里直接产生 `agent.trigger.v1`；本文 §4.2 是同一件事的 Agent 侧描述。约定（00-decisions D5）：

- Kafka topic `agent.trigger.v1` 由 msgtransfer 在 categorize 阶段（即分配 seq、写 Redis cache 之后、produce toPostgres/toPush 同一批次）产生；
- agent-rpc 消费这个 topic，跑 RunOrchestrator；
- 写回 IM 通过 msg-rpc gRPC（imadapter）→ msg-rpc 再 `producer.Publish(msg.toTransfer.v1)`，**AI 消息走与人类消息完全相同的 Kafka 链路**（00-decisions D1：没有 outbox 这种特殊路径）；
- 防递归靠 trigger event 的 `source_agent_run_id` + `hostingService` 的幂等表，不依赖 outbox 字段。
