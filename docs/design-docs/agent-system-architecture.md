# Agent System Architecture

状态：Draft

本文档定义 Agent 系统第一版技术架构。产品行为见 [`../product-specs/agent-system.md`](../product-specs/agent-system.md)，IM 与 Agent 事件边界见 [`im-agent-contract.md`](./im-agent-contract.md)。

## 总体结论

Agent 系统应作为独立能力域开发，并通过 IM 后端事件和 Message Service 写回接口与现有 IM 系统解耦。

推荐逻辑架构：

```text
IM Backend
  ├── Account/Auth: user / agent / admin account type
  ├── Message Service: authoritative message writes
  ├── Outbox/Kafka/Webhook: message.created events
  └── Gateway: WebSocket delivery

Agent System
  ├── Agent Management API
  ├── Prompt Management
  ├── Tool Registry
  ├── Skill Registry
  ├── Agent Runtime
  ├── Python Executor
  └── Audit Store

Storage
  ├── PostgreSQL: metadata, bindings, runs, audit records
  ├── RustFS/S3: skill files and large artifacts
  └── Redis: short-lived runtime/cache/locks where needed
```

Agent 会话托管第一阶段设计见 [`agent-conversation-hosting.md`](./agent-conversation-hosting.md)：Message Service 持久化 `message_origin=human|ai|system`，Agent hosting seam 消费已持久化消息事件，通过 `AgentRunOrchestrator` 调用 runtime，并用 Message Service 写回 `message_origin=ai` 的普通 IM 消息。

## 服务边界

### Account/Auth 扩展

- Account Service 提供 `account_type`，支持 `user`、`agent`、`admin`。PostgreSQL 存储使用 `accounts` + `profiles`。
- Account Service 仍只负责账号资料，不保存 Agent prompt、tool 或 skill 配置。
- `auth` 仍只负责认证秘密。Agent 账号默认不提供普通账号密码登录。

### Agent Management

负责：

- Agent profile CRUD；
- Agent 绑定 IM `agent_user_id`；
- prompt/tool/skill 绑定；
- runtime policy 管理；
- 启用、禁用、归档；
- 管理 API 鉴权和权限检查。

当前 Agent profile 管理基础：

- REST 契约：[`../../service/agent/api/agent.api`](../../service/agent/api/agent.api)。
- 入口：`cmd/agent-api`，配置文件：`etc/agent-api.yaml`，启动桥接：`service/agent/api/entry`。
- 业务逻辑：`internal/logic/agentlogic.go`，go-zero adapter：`service/agent/api/internal/logic/agent/`。
- 仓储契约：`internal/repository/agent_repository.go`，默认测试仓储：`internal/repository/agent_memory.go`，PostgreSQL 仓储：`internal/repository/postgres_agent.go`。
- PostgreSQL schema：`db/migrations/002_agent_management.sql`。
- 当前没有真实 Agent RPC/proto contract；不要为了目录形状创建空 RPC scaffold。Agent API 仍沿用现有直接 repository wiring，直到后续 issue 补齐真实 Agent RPC 边界。

创建 Agent 时，业务逻辑先调用窄接口 `UserAccountTypeChecker` 验证绑定账号为 `account_type=agent`，再写入 `agents` 表。当前 `agent-api` wiring 使用真实 Account Service profile repository 校验账号类型；无法验证时必须返回明确错误。此设计避免在 Account profile 中塞入 Agent 配置，也避免用假账号或静默 fallback 冒充账号类型能力。

Issue #129 补齐了 Agent Definition/Assembly 后端基础：

- Agent Definition 读取 `agents`、`agent_prompt_bindings`、`agent_prompts`、`agent_tool_bindings`、`agent_tools`，返回 Agent profile、一个活动 system prompt 和已绑定 tool 列表。
- `PUT /agents/:agent_id/definition` 使用同一套 registry 表更新 system prompt 与 tool list。V1 语义是替换该 Agent 的 prompt bindings 为一个新活动 system prompt；历史 prompt 行保留为元数据，不作为当前定义读取。
- `agent.create` 是受控 local tool，只在服务端代码白名单中有 handler；数据库只保存 `handler_key=agent.create`、schema、权限级别和绑定关系。
- `agent.create` 由业务逻辑组装新 Agent：创建 `account_type=agent` 账号/Profile、`agents` 行、活动 system prompt binding、允许的 tool bindings，并与请求该工具的人类用户创建 accepted friendship。
- 默认数据只把 `agent.create` 绑定到内置 `agent_creator`。由 `agent.create` 创建的新 Agent 默认不能获得 `agent.create`、`python.execute`、MCP/network tool 或 `im.send_agent_message` 等写/执行型工具。

### Prompt Management

PostgreSQL 表建议：

```text
agent_prompts
- prompt_id
- name
- description
- content
- variables_schema_json
- version
- status: draft / active / archived
- created_by
- created_at
- updated_at
```

运行时必须记录 prompt snapshot，避免 prompt 后续编辑导致历史 run 不可复现。

当前 registry 实现提供 `agent_prompts` 元数据表和 `agent_prompt_bindings` 绑定表。绑定表以 `(agent_id, prompt_id)` 去重；因 Agent profile 分支并行开发，当前不对 `agent_id` 建外键，集成 `agents` 表后可补充外键或当前 prompt 唯一策略。

### Tool Registry

工具元数据存在 PostgreSQL。工具类型：

```text
mcp
local
builtin
```

建议表：

```text
agent_tools
- tool_id
- name
- description
- tool_type
- handler_key
- mcp_server_id
- mcp_tool_name
- input_schema_json
- output_schema_json
- permission_level
- enabled
- created_at
- updated_at
```

MCP server 配置建议：

```text
mcp_servers
- server_id
- name
- transport: http / sse / streamable_http / stdio_admin_only
- command
- args_json
- url
- headers_secret_ref
- timeout_seconds
- enabled
```

第一版建议只开放管理员配置的 MCP server。stdio MCP 会启动本地进程，默认不对普通用户开放。

当前 registry 实现进一步收紧第一版范围：MCP server 只接受 `http`、`sse`、`streamable_http` transport，不保存 stdio `command` / `args` 元数据，避免把本地进程启动能力引入数据库配置。MCP tool 必须引用管理员配置 server；Agent 必须通过 `agent_tool_bindings` 白名单绑定后才能使用该 tool。

本地工具不得从数据库读取并执行任意脚本。数据库只保存 `handler_key`，服务端代码用白名单映射 handler：

```text
im.get_conversation_context
im.send_agent_message
skill.read_file
python.execute
agent.create
```

当前 registry 只登记工具元数据和绑定关系，不执行 handler、不启动 MCP client、不执行 Python。local tool 只接受服务端白名单 `handler_key`；builtin tool 只接受白名单 `builtin_key`；任何 `shell`、`command`、`script` tool type 或类似 handler key 都必须在 logic 层失败。

当前 Agent runtime 工具解析契约位于 `internal/agentruntime/tools`。Eino runtime 不应直接读取全局工具表或自行组装工具，而应通过该 package 的 `Provider` / `Resolver` 获取当前 Agent 允许的 `ToolSpec` 和可选 `ToolAdapter`：

- `ResolveAgentTools` 默认从 `agent_tool_bindings` 列出该 Agent 已绑定工具；`ResolveTool` 用于解析指定工具并检查绑定是否存在。
- 解析时会重新校验 tool 必须为 `active` 且 `admin_configured=true`；MCP tool 还必须引用 `active` 且管理员配置的 MCP server。
- MCP transport 仅允许 `http`、`sse`、`streamable_http`，并拒绝 stdio / local process / command-like config metadata。
- V0/V1 只提供 metadata contract 与显式安全 adapter；当 runtime 需要可调用 adapter 时必须使用 `RequireAdapters=true`，缺少显式安全 adapter 时返回明确错误。
- `python.execute` 只能通过 `PythonExecuteAdapter` 调用注入的 `pythonexec.Executor`；默认 executor 是 disabled，真实执行必须走独立沙箱。详细边界见 [`python-executor-sandbox.md`](./python-executor-sandbox.md)。
- `agent.create` 只能通过 `AgentCreateAdapter` 调用 Agent assembly 业务逻辑。adapter 必须从 runtime `ToolCall` 获得 `requesting_user_id`；缺失时失败。该工具不直接写库、不启动进程、不调用外部网络工具。
- 该 package 不导入 Eino、不执行 MCP 网络调用、不执行任意本地 handler、不直接调用 Python，也不提供 shell、命令、本地进程或文件系统写入能力。

### Skill Registry 与 RustFS

PostgreSQL 保存 skill 元数据和文件索引，RustFS (S3-compatible) object storage 保存实际文件。

建议表：

```text
agent_skills
- skill_id
- name
- description
- version
- status: draft / active / archived
- owner_user_id
- created_at
- updated_at

agent_skill_files
- file_id
- skill_id
- version
- object_bucket
- object_key
- file_path
- content_type
- sha256
- size_bytes
- created_at
```

Object key 建议：

```text
skills/{skill_id}/versions/{version}/{file_path}
```

读取规则：

- Runtime 只能通过 Agent Service 的 `skill_id + file_path` 读取。
- Runtime 不持有 RustFS root credential。
- Agent 绑定 skill 后默认可读取该 skill 下文件。
- 每次读取写 `skill_file_reads` 或通用 audit log。

当前 registry 实现将第一版 skill 文件索引压缩在 `agent_skills` 元数据表中：`object_key`、`sha256`、`content_type`、`size_bytes` 必填，PG 不保存文件内容。`agent_skill_bindings` 表记录 Agent 白名单绑定；真正的 RustFS/S3 上传、下载、读取审计链路留给后续 storage/runtime 任务。

### Agent Runtime

当前 `feature/eino-runtime-core` 冻结了纯 Go 运行时接口边界，位于 `internal/agentruntime`，设计见 [`agent-runtime-eino.md`](./agent-runtime-eino.md)。该边界只定义 `Runtime.Run(ctx, RunRequest) (RunResult, error)`、请求/结果归一化、Agent 配置快照、prompt/tool/skill refs 和模型 usage 元数据；不导入 Eino、不调用 LLM、不执行工具、不写回 IM。

Runtime 每次 run 组装：

```text
system prompt snapshot
+ model config
+ selected tools
+ bound skills file/context
+ conversation context
+ user message
+ runtime policy
```

运行记录建议：

```text
agent_runs
- run_id
- agent_id
- conversation_id
- trigger_message_id
- status
- prompt_snapshot_json
- tools_snapshot_json
- skills_snapshot_json
- input_summary
- output_message_id
- error_code
- error_message
- started_at
- finished_at
```

当前 provider adapter 基线：

- Go package：`internal/agentruntime/llm/deepseek`。
- Eino component：`github.com/cloudwego/eino-ext/components/model/deepseek`。
- 配置来源：`DEEPSEEK_API_KEY`、`DEEPSEEK_BASE_URL`、`DEEPSEEK_MODEL`。
- 默认值：`DEEPSEEK_BASE_URL=https://api.deepseek.com`，`DEEPSEEK_MODEL=deepseek-v4-pro`。
- 缺少 `DEEPSEEK_API_KEY` 时构造 ChatModel 必须返回明确错误，不能降级为 mock/fake response。
- 默认 `go test ./...` 不请求 DeepSeek；live smoke test 只能在 `RUN_LIVE_DEEPSEEK_TESTS=1` 且存在 API key 时运行。

当前 Eino runtime 支持通过 `internal/agentruntime/tools.Provider` 解析 registry-approved tool bindings，并可执行带显式安全 adapter 的本地工具；`python.execute` 通过注入的 `PythonExecuteAdapter` / `pythonexec.Executor` 执行，默认 executor 仍 fail-closed。MCP 网络工具调用、完整 skill 文件读取和长期审计落库仍是后续任务。当前 Agent-IM runner seam 使用 `internal/agentruntime.Runtime`，生产 wiring 必须显式组装 registry-derived `RunRequest` 并注入 tool provider。

工具调用审计：

```text
agent_tool_calls
- tool_call_id
- run_id
- agent_id
- tool_id
- tool_name
- input_summary
- output_summary
- status
- error_code
- error_message
- duration_ms
- trace_id
- request_id
- started_at
- finished_at
- created_at
```

Skill 文件读取审计：

```text
agent_file_reads
- file_read_id
- run_id
- agent_id
- skill_id
- file_id
- object_key
- sha256
- status
- byte_count
- content_summary
- error_code
- error_message
- trace_id
- request_id
- started_at
- finished_at
- created_at
```

Python 执行审计：

```text
agent_python_execs
- python_exec_id
- run_id
- agent_id
- sandbox_request_id
- status
- code_summary
- resource_summary
- stdout_summary
- stderr_summary
- result_summary
- error_code
- error_message
- trace_id
- request_id
- started_at
- finished_at
- created_at
```

审计存储规则（后续正式审计阶段目标）：

开发初期 Agent audit 主要用于调试和运行可观测性，不按合规审计系统验收；允许先保留定位问题所需的触发输入/输出摘要。正式进入审计阶段前，再把下面规则作为收紧目标落地，并补齐数据保留周期、访问控制和脱敏策略。

- 审计表为 append-only；Repository/Logic 不提供 update/delete，PostgreSQL trigger 拒绝直接 update/delete。
- 审计写入是 required path，失败必须返回调用方。
- `*_summary` 字段只存脱敏或摘要后的 JSON，不保存 raw credential、token、secret 或 Python raw code。
- Python code summary 仅包含 `sha256` 和 `size_bytes`，代码执行本身仍由后续 sandbox executor 契约负责。

## Python Executor 设计

第一版支持 Python，但不支持 shell。直接在 Agent Service 进程内执行 Python 风险过高，推荐独立 Python Executor 服务或容器沙箱。

强制约束：

- 每次执行有 timeout。
- 限制 CPU 和 memory。
- 默认禁用网络。
- 不挂载宿主机目录。
- 不暴露 Docker socket。
- 只提供允许的 skill 文件只读副本。
- stdout/stderr/result/error 全部记录。
- 失败必须显式返回，不能伪造成功。

当前仓库实现 Go 侧 sandbox contract、`python.execute` local tool adapter，以及 DeepSeek/Eino runtime 中对 registry-approved local adapter 的工具调用循环；不实现未隔离的真实执行器。契约位于 `internal/agent/pythonexec`：

```text
Executor.Execute(ctx, Request) (*Response, error)

Request
- code
- policy

Policy
- run_id
- audit_id
- timeout
- cpu_time_limit
- memory_limit_bytes
- network: disabled by default
- file_allowlist: explicit read-only relative paths
- max_output_bytes

Response
- run_id / audit_id
- stdout / stderr / result_json
- exit_code / timed_out / output_truncated
- structured error
```

默认实现为 `DisabledExecutor` / `NewDefaultExecutor()`，只校验 request 和 policy，然后返回 `ErrPythonExecutorDisabled`。它不会启动 Python、Docker、shell 或任何本地进程。`internal/agentruntime/tools.PythonExecuteAdapter` 负责把 local tool input 转成受限 policy；缺少真实 executor 时仍显式失败。后续真实 executor 必须是独立沙箱服务或隔离 worker，并在接入前补齐审计落库、资源限制验证和 opt-in 集成测试。沙箱约束见 [`python-executor-sandbox.md`](./python-executor-sandbox.md)。

工具接口草案：

```json
{
  "tool": "python.execute",
  "input": {
    "code": "print(1 + 1)",
    "files": ["SKILL.md"],
    "timeout_seconds": 10
  }
}
```

输出：

```json
{
  "stdout": "2\n",
  "stderr": "",
  "result_json": null,
  "error": null
}
```

仅靠 AST 黑名单不足以提供安全边界；AST 检查可作为辅助，隔离容器才是主边界。

## IM 集成数据流

第一阶段 Go 契约落在 `internal/agentim`，只定义触发、写回接口和循环预防规则，不接入任何 LLM provider/framework。

### 用户消息触发 Agent

```text
Client -> Gateway/Message API
-> Message Service persists message
-> Message Outbox emits message.created
-> Webhook/Event Dispatcher sends event to Agent Service
-> Agent Runtime runs
-> Agent writes response through Message Service
-> Gateway delivers response
```

原则：Agent 不直接写 messages 表，不直接推 WebSocket，不绕过 Message Service。

触发类型：

| 类型 | 来源 | 必需条件 |
| --- | --- | --- |
| `user_private_message` | 用户私聊 Agent | `conversation_type=single`，消息接收方是目标 Agent 用户，发送方不是默认抑制的 Agent 消息。 |
| `group_mention` | 群聊 @Agent | `conversation_type=group`，目标 Agent 在 `at_user_ids` 中。 |
| `admin_manual_run` | 管理员手动触发 | 管理员传入 `request_id`、`admin_user_id`、`agent_user_id`、会话和 prompt 文本；不伪造用户消息。 |

`message.created` 事件必须保留 `event_id`、`operation_id`、`trace_id`、`conversation_id`、`server_msg_id`、`seq`、`sender_id`、`sender_type`、`content_type` 和目标 Agent 列表。第一阶段由 `MessageLogic.MessageCreatedHook` 在持久化后传入 `ConversationHostingService`，event id 使用稳定的 `message.created:<server_msg_id>`；hook 只完成触发选择、幂等接受、托管 owner read seq 推进和后台执行调度，不同步等待 Agent runtime。后续独立异步 consumer 必须保持同一 idempotency 语义。构造出的 Agent trigger 使用 `event_id + agent_user_id` 作为幂等请求基础。

### Agent 响应写回

Agent 响应写回使用 `MessageServiceResponseWriter`：

```text
AgentTrigger
-> AgentRunOrchestrator
-> RuntimeRequestBuilder loads Agent config/context
-> agentruntime.Runtime
-> agentim.ResponseWriter
-> MessageSender interface
-> existing MessageLogic.SendMessage / Message Service
-> Message Service storage/outbox path
```

强制约束：

- `internal/agentim` 不依赖 message repository，也不拥有 `messages` 表写入能力。
- `AgentRunOrchestrator` 只依赖统一的 `internal/agentruntime.Runtime`，不得重新定义 provider-specific 或 Eino-specific runtime interface。
- `RuntimeRequestBuilder` 必须从真实 Agent 配置、prompt/tool/skill 绑定和会话上下文构造 `agentruntime.RunRequest`；缺失配置、请求不匹配或 runtime result 无 `run_id` / `final_text` 必须记录失败 audit 并返回明确错误。
- 生产实现必须注入兼容 `MessageLogic.SendMessage(ctx, logic.SendMessageRequest)` 的 Message Service seam。
- Writer 只生成 `sender_id=agent_user_id` 的标准 `SendMessageRequest`，并复用 Message Service 的幂等、seq、outbox、投递链路。
- Message Service 返回错误必须原样暴露；返回空 `server_msg_id`、空 `conversation_id` 或非法 `seq` 必须视为内部错误，禁止把空返回当成功。

### 循环控制

- Agent 消息默认不再触发 Agent。
- 群聊默认只在 @Agent 时触发。
- 每个 run 有最大工具调用次数、最大执行时长、最大递归深度。
- 幂等键应包含 `event_id` 或 `trigger_message_id + agent_id`。
- Agent 响应元数据包含 `agent_run_id`、`trigger_message_id` 和 `allow_recursive_trigger`；只有全局/会话策略 `AllowAgentMessageRecursion=true` 且消息元数据 `allow_recursive_trigger=true` 时，Agent 发送的消息才能再次构造 Agent trigger。

当前第一阶段实现新增 `agent_conversation_hosting` 与 `agent_trigger_idempotency` 仓储契约。托管配置以 `conversation_id -> agent_account_id` 表示是否 enabled；trigger idempotency key 使用 `event_id/server_msg_id + agent_account_id`，running/succeeded 重复事件不再执行，失败会记录为 `failed` 并允许后续显式重试。AI hosting V1 的后台执行仍在 message-api 进程内完成；幂等状态是持久的，但进程重启后的自动恢复 worker 属于后续可靠性增强。

## 权限模型

权限决策至少包含：

```text
actor account_type
agent owner/admin relationship
agent status
agent tool bindings
agent skill bindings
runtime policy
conversation membership
requested tool permission level
```

第一版默认策略：

- `user` 类型账号不能管理全局工具和 MCP server。
- `admin` 可以管理 prompt/tool/skill/agent。
- Agent 只能调用已绑定工具。
- Agent 绑定 skill 后可读取 skill 文件。
- Python 执行默认允许但必须沙箱化。
- Shell 执行不提供。
- 网络访问默认关闭或仅管理员显式开启。

## 推荐落地顺序

1. 扩展账号类型和 Agent profile。
2. Prompt CRUD 与 Agent 绑定。
3. Tool registry、Skill registry、RustFS 接入。
4. Agent runtime 最小 run 记录。
5. Python Executor 沙箱。
6. IM event -> Agent -> Message Service 写回闭环。

## 风险

| 风险 | 等级 | 缓解 |
| --- | --- | --- |
| Python 逃逸 | 高 | 独立容器沙箱、禁网络、限资源、不挂载宿主目录。 |
| MCP 工具越权 | 高 | 只允许管理员配置，Agent 绑定白名单，调用审计。 |
| Skill 文件越权 | 高 | `skill_id + file_path` 授权，Runtime 不持有 RustFS 凭证。 |
| Prompt 注入 | 高 | 权限由服务端策略强制，不能靠 prompt 自律。 |
| Agent 无限循环 | 高 | Agent 消息默认不触发 Agent，max depth/max run。 |
| 成本失控 | 中高 | max tokens、max tool calls、预算与超时。 |
| 审计不足 | 高 | run、tool、file read、python exec 全部入库。 |

## 验证计划

实现阶段至少验证：

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
docker compose config
```

涉及 RustFS/Python Executor 的集成测试应默认跳过，只有显式环境变量存在时才运行，避免默认 `go test ./...` 依赖外部服务。
