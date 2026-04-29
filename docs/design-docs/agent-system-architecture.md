# Agent System Architecture

状态：Draft

本文档定义 Agent 系统第一版技术架构。产品行为见 [`../product-specs/agent-system.md`](../product-specs/agent-system.md)，IM 与 Agent 事件边界见 [`im-agent-contract.md`](./im-agent-contract.md)。

## 总体结论

Agent 系统应作为独立能力域开发，并通过 IM 后端事件和 Message Service 写回接口与现有 IM 系统解耦。

推荐逻辑架构：

```text
IM Backend
  ├── User/Auth: normal / agent / admin account type
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
  ├── MinIO/S3: skill files and large artifacts
  └── Redis: short-lived runtime/cache/locks where needed
```

## 服务边界

### User/Auth 扩展

- `users` 增加 `account_type`，支持 `normal`、`agent`、`admin`。
- `user` 仍只负责账号资料，不保存 Agent prompt、tool 或 skill 配置。
- `auth` 仍只负责认证秘密。Agent 账号默认不提供普通账号密码登录。

### Agent Management

负责：

- Agent profile CRUD；
- Agent 绑定 IM `agent_user_id`；
- prompt/tool/skill 绑定；
- runtime policy 管理；
- 启用、禁用、归档；
- 管理 API 鉴权和权限检查。

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

本地工具不得从数据库读取并执行任意脚本。数据库只保存 `handler_key`，服务端代码用白名单映射 handler：

```text
im.get_conversation_context
im.send_agent_message
skill.read_file
python.execute
```

### Skill Registry 与 MinIO

PostgreSQL 保存 skill 元数据和文件索引，MinIO/S3-compatible object storage 保存实际文件。

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
- Runtime 不持有 MinIO root credential。
- Agent 绑定 skill 后默认可读取该 skill 下文件。
- 每次读取写 `skill_file_reads` 或通用 audit log。

### Agent Runtime

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

工具调用审计：

```text
tool_invocations
- invocation_id
- run_id
- agent_id
- tool_id
- tool_name
- input_json
- output_json
- status
- error_message
- latency_ms
- created_at
```

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

### 循环控制

- Agent 消息默认不再触发 Agent。
- 群聊默认只在 @Agent 时触发。
- 每个 run 有最大工具调用次数、最大执行时长、最大递归深度。
- 幂等键应包含 `event_id` 或 `trigger_message_id + agent_id`。

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

- `normal` 用户不能管理全局工具和 MCP server。
- `admin` 可以管理 prompt/tool/skill/agent。
- Agent 只能调用已绑定工具。
- Agent 绑定 skill 后可读取 skill 文件。
- Python 执行默认允许但必须沙箱化。
- Shell 执行不提供。
- 网络访问默认关闭或仅管理员显式开启。

## 推荐落地顺序

1. 扩展账号类型和 Agent profile。
2. Prompt CRUD 与 Agent 绑定。
3. Tool registry、Skill registry、MinIO 接入。
4. Agent runtime 最小 run 记录。
5. Python Executor 沙箱。
6. IM event -> Agent -> Message Service 写回闭环。

## 风险

| 风险 | 等级 | 缓解 |
| --- | --- | --- |
| Python 逃逸 | 高 | 独立容器沙箱、禁网络、限资源、不挂载宿主目录。 |
| MCP 工具越权 | 高 | 只允许管理员配置，Agent 绑定白名单，调用审计。 |
| Skill 文件越权 | 高 | `skill_id + file_path` 授权，Runtime 不持有 MinIO 凭证。 |
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

涉及 MinIO/Python Executor 的集成测试应默认跳过，只有显式环境变量存在时才运行，避免默认 `go test ./...` 依赖外部服务。
