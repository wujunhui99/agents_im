# Agent System Product Spec

状态：Draft

本文档定义 Agent 系统第一版的产品范围、账号类型、提示词、工具、skills、Python 执行和 IM 集成行为。技术方案见 [`../design-docs/agent-system-architecture.md`](../design-docs/agent-system-architecture.md)。

## 背景

`agents_im` 的长期目标不只是普通 IM，还要支持 Agent 单聊、Agent 群聊、多 Agent 协作和 Agent 工具调用。当前 IM 后端已经具备账号、认证、好友、群组、消息、WebSocket、PostgreSQL、Redis、Redpanda/Kafka、Outbox 和前端 MVP。下一阶段需要把 Agent 从“架构占位”推进到可管理、可组装、可审计的系统能力。

## 产品目标

1. 在账号系统中区分普通账号、Agent 账号和管理员账号。
2. 管理系统提示词，并将系统提示词持久化在数据库中。
3. 管理工具信息，包括 MCP 工具和本地工具，并将工具元数据持久化在数据库中。
4. 使用系统提示词、工具和 skills 组装 Agent。
5. 管理 Agent 元数据、配置、启停状态和 IM 账号身份。
6. 单独管理 Agent skills；skill 元数据在 PostgreSQL，skill 文件在 MinIO/S3-compatible object storage。
7. Agent 绑定 skill 后默认可以读取该 skill 的文件。
8. 第一版不提供 shell/命令行脚本执行能力。
9. 第一版提供受限 Python 执行能力，并保留审计与资源限制。
10. Agent 响应必须通过 IM Message Service 写回，不能绕过 IM 消息链路。
11. 冻结 Agent-IM 触发契约：用户私聊 Agent、群聊 @Agent、管理员手动 run；当前提供 LLM provider adapter/config 基线和 Agent-IM runner seam，但不实现完整 Eino runtime orchestration。

## 账号类型

账号系统需要支持 `account_type`：

| 类型 | 含义 | 是否可普通登录 | 是否可作为消息成员 | 说明 |
| --- | --- | --- | --- | --- |
| `user` | human user 账号 | 是 | 是 | 当前公开注册账号的默认类型；这里的 `user` 是 account type，不是服务名。 |
| `agent` | Agent 虚拟账号 | 默认否 | 是 | 用于让 Agent 在 IM 中像成员一样参与单聊/群聊。 |
| `admin` | 管理员账号 | 是 | 是 | 可管理 prompts、tools、skills、agents 和运行策略。 |

产品约束：

- Account Service 仍只负责账号资料，不管理密码、Agent 配置或工具配置。
- `auth` 服务继续负责认证秘密；Agent 账号默认不通过账号密码登录，除非后续明确支持内部凭证或 owner 代理操作。
- Agent 的 IM 展示身份来自 Account Service（V0 storage 表名仍为 `users`），详细 Agent 配置来自 `agents`。
- 普通用户是否能搜索、添加或邀请 Agent，需要由 Agent 的发布状态和可见性策略控制。

## 核心对象

### System Prompt

系统提示词是可版本化、可审计的 Agent 行为说明。

用户可感知能力：

- 管理员创建、编辑、启用、归档系统提示词。
- Agent 可以绑定一个当前使用的系统提示词。
- Agent 运行时记录 prompt snapshot，便于复盘历史行为。

第一版不要求复杂 prompt 模板市场，但应支持变量 schema 或元数据字段，为后续模板化做准备。

第一版落地约束：

- 系统提示词元数据包含 `content`、`version`、`status`、`created_by`、`created_at`、`updated_at`。
- `status` 只能是 `draft`、`active`、`archived`。
- Agent prompt 绑定只允许绑定 `active` prompt；重复绑定同一 `agent_id + prompt_id` 必须去重返回，而不是创建多条重复记录。

### Tool

工具是 Agent 可调用的能力。第一版工具分为：

- `mcp`：通过 Model Context Protocol 暴露的外部工具。
- `local`：服务端白名单内的本地工具 handler。
- `builtin`：系统内置工具，例如读取 IM 上下文、读取 skill 文件、写回 Agent 消息、受限 Python 执行。

用户可感知能力：

- 管理员注册工具。
- 管理员启用/禁用工具。
- Agent 只能使用已绑定且策略允许的工具。
- 工具调用必须产生审计记录。

第一版不允许普通用户通过数据库录入任意可执行脚本作为工具。

第一版落地约束：

- 工具类型只允许 `mcp`、`local`、`builtin`；`shell`、`command`、`script` 或其他可执行类型必须拒绝。
- MCP server 由管理员配置，只允许远程 `http`、`sse`、`streamable_http` transport；第一版不开放 stdio/command/args 进程启动配置。
- MCP tool 必须引用已存在的管理员配置 MCP server，并且 Agent 只有在显式工具绑定后才能使用该 tool。
- local tool 只保存服务端白名单 `handler_key`，不保存脚本源码、命令行或任意可执行内容。
- builtin tool 只保存白名单 `builtin_key`。
- Agent 只能使用 `active` 且已绑定的工具；未绑定、disabled 或 archived 工具必须返回不可用，而不能静默降级到 mock 行为。

### Agent

Agent 是由 profile、系统提示词、工具、skills、模型配置和运行策略组装出来的可运行实体。

用户可感知能力：

- 管理员或有权限用户创建 Agent。
- Agent 有名称、描述、头像、状态和对应的 IM `agent` 账号。
- Agent 可以绑定 prompt、tools 和 skills。
- Agent 可以被加入单聊或群聊。
- Agent 接收到 IM 事件后，根据策略决定是否响应。

#### 当前管理和运行基础接口

当前 Agent profile 管理 REST 契约见 [`../../api/agent.api`](../../api/agent.api)，覆盖：

- `POST /agents`：创建 Agent profile，绑定已有 IM account；
- `GET /agents`：按 `status`、`created_by` 可选过滤列表；
- `GET /agents/:agent_id`：查询 Agent profile；
- `PATCH /agents/:agent_id`：更新名称和描述；
- `PATCH /agents/:agent_id/status`：更新 `draft` / `active` / `disabled` / `archived` 状态；
- `DELETE /agents/:agent_id`：归档 Agent，等价于设置 `status=archived`。

约束：

- Agent 配置只写入 `agents` 表，不写入 `users` 表。
- 创建 Agent 必须绑定 `account_type=agent` 的现有 IM account。
- 生产 wiring 必须使用真实 `UserAccountTypeChecker` 验证账号类型；无法验证账号类型时创建必须失败，不能静默创建假用户或假 Agent。

### Model Provider Config

当前 Agent runtime provider 基线使用 CloudWeGo Eino 和 DeepSeek ChatModel adapter。DeepSeek adapter 负责构造真实 ChatModel；IM runner 负责把规范化 runtime result 通过 Message Service 写回 IM；工具执行仍保持 fail-closed adapter seam，缺少显式安全 adapter 时不得执行。

配置来源：

- `DEEPSEEK_API_KEY`：必填；缺失或仍为 `.env.example` 占位值时生产 adapter 必须失败。
- `DEEPSEEK_BASE_URL`：可选，默认 `https://api.deepseek.com`。
- `DEEPSEEK_MODEL`：可选，默认 `deepseek-v4-pro`。

默认测试不得依赖真实 DeepSeek key 或网络；live DeepSeek smoke test 必须同时设置 `RUN_LIVE_DEEPSEEK_TESTS=1` 和 `DEEPSEEK_API_KEY` 才运行。

### Agent Skill

Skill 是可复用的 Agent 能力包，通常由 Markdown、参考资料、模板或结构化文件组成。

产品要求：

- Skill 元数据单独管理。
- Skill 文件保存在 MinIO/S3-compatible object storage。
- Agent 绑定 skill 后，默认允许读取该 skill 下的文件。
- Agent 不能读取未绑定 skill 的文件。
- 第一版不执行 skill 中的 shell 脚本或命令行脚本。
- Skill 文件读取必须审计。

第一版 registry 落地约束：

- PostgreSQL 只保存 skill 元数据：`name`、`description`、`version`、`object_key`、`sha256`、`content_type`、`size_bytes`、`status`、`created_by`、时间戳。
- `sha256`、`object_key`、`content_type`、`size_bytes` 为必填；`sha256` 必须是 64 位十六进制摘要；`size_bytes` 必须大于 0。
- Skill 文件内容不进入 PostgreSQL；MinIO/S3 上传和读取执行链路不在本 registry 任务范围内。
- Agent skill 绑定只允许绑定 `active` skill，重复绑定同一 `agent_id + skill_id` 必须去重。

### Python Execution

第一版支持 Python 执行，但产品语义必须是“受限 Python 执行”，不是任意操作系统命令执行。

用户可感知能力：

- Agent 可调用 `python.execute` 工具完成计算、数据处理或轻量脚本逻辑。
- Python 执行结果可以作为 Agent 推理上下文或工具结果消息的一部分。
- 执行失败必须暴露为明确错误，不能静默返回假成功。

第一版明确不支持：

- shell/命令行执行；
- 访问宿主机任意文件；
- 访问 Docker socket；
- 默认访问外部网络；
- 写入 MinIO skill 文件；
- 无限时长或无限资源执行。

当前 Python sandbox contract 只冻结 Go 侧执行契约，不提供真实 Python runtime。`internal/agent/pythonexec` 定义 `Executor`、`Request`、`Response` 和 `Policy`；`Policy` 必须包含 `run_id`、`audit_id`、timeout、CPU/memory limit、默认禁用网络的策略、显式 file allowlist 和 `max_output_bytes`。默认 executor 是 disabled implementation，请求校验通过后仍必须返回 `ErrPythonExecutorDisabled`，不能伪造成功结果。生产 Go 服务不得直接调用 `os/exec`、shell 或 Python binary；该约束由单元测试和 `scripts/verify-static.sh` 静态检查覆盖。

### Audit Log

Agent 审计记录是第一版运行安全边界的一部分。审计基础设施不执行 LLM、不执行工具、不执行 Python，只负责把真实运行链路产生的事实追加写入 PostgreSQL 或测试内存仓储。

第一版审计覆盖：

- `agent_runs`：Agent run 的触发、会话、请求用户、状态、trace/request id、输入/输出摘要、错误和时间戳。
- `agent_tool_calls`：工具调用的工具标识、输入/输出摘要、状态、耗时、错误、trace/request id 和时间戳。
- `agent_file_reads`：skill 文件读取的 skill/file/object key/sha256、读取字节数、状态、错误、trace/request id 和时间戳。
- `agent_python_execs`：Python 沙箱请求标识、代码摘要、资源/stdout/stderr/result 摘要、状态、错误、trace/request id 和时间戳。

产品约束：

- 审计记录是 append-only；业务代码只提供 create/list/get 能力，PostgreSQL 层拒绝 update/delete。
- 审计写入失败必须返回给调用方，不能吞错、静默降级或返回假成功。
- 摘要字段必须递归脱敏 `password`、`token`、`secret`、`credential`、`authorization`、`api_key` 等敏感键。
- Python 代码只保存 `sha256` 与 `size_bytes` 摘要，不保存 raw code。
- 错误信息和普通文本摘要会做内联 token/password/secret 模式脱敏；长文本只保存 hash/长度摘要。

## 典型场景

### 创建 Agent

1. 管理员创建 `account_type=agent` 的 IM 账号。
2. 管理员创建 Agent profile，并绑定 `agent_user_id`。
3. 管理员选择系统提示词。
4. 管理员绑定允许的 tools 和 skills。
5. 管理员启用 Agent。
6. 普通用户可以在允许范围内把 Agent 加入会话。

### Agent 响应单聊消息

1. 用户向 Agent 账号发送消息。
2. Message Service 持久化消息并写 outbox/event。
3. Agent Service 接收 IM 事件。
4. Agent Service 判断接收方是 active Agent。
5. Runtime 加载 Agent prompt、tools、skills 和会话上下文。
6. Agent 调用 LLM，必要时调用工具或读取 skill 文件。
7. Agent 响应通过 Message Service 写回 IM。
8. WebSocket/Gateway 将消息投递给用户。

### Agent 群聊响应

第一版建议默认只在以下情况触发 Agent：

- 用户 @Agent；或
- Agent 策略明确配置为监听群内所有消息。

防止无限循环：

- Agent 消息默认不再次触发 Agent。
- Agent 响应消息必须带 `agent_run_id`、`trigger_message_id` 和 `allow_recursive_trigger=false` 的语义化元数据；只有消息元数据和运行策略都显式允许时，Agent 消息才能再次触发 Agent。
- 多 Agent 协作需要显式策略和最大轮次限制。

### 管理员手动 Run

管理员可以在指定会话中手动触发 Agent run，用于调试、补偿或运营动作。手动 run 必须包含 `request_id`、`admin_user_id`、`agent_user_id`、`conversation_id`、`conversation_type` 和本次 prompt 文本；它不伪造用户消息，也不直接写 IM 消息表。若产生响应，仍必须通过 Message Service 写回。

### Agent 写回 IM

第一阶段 Go 契约位于 `internal/agentim`：

- `BuildMessageCreatedTriggers` 只从 IM `message.created` 事件构造触发请求，不执行 LLM。
- `AgentRunOrchestrator` 使用统一的 `internal/agentruntime.Runtime` 接口；生产 wiring 必须通过显式 `RuntimeRequestBuilder` 从 Agent registry、prompt/tool/skill 绑定和会话上下文组装 `RunRequest`，缺失配置必须失败，不能构造假 runtime request。
- `MessageServiceResponseWriter` 只依赖兼容 `MessageLogic.SendMessage` 的 `MessageSender` 接口。
- 写回失败、未配置 Message Service、或 Message Service 返回空 `server_msg_id` / `conversation_id` / 非法 `seq` 时必须返回明确错误，不能返回假成功。
- Agent 系统不得直接调用 message repository、不得直接写 `messages` 表、不得直接推 WebSocket。

## 验收标准

第一版文档和实现进入开发前，应冻结以下产品契约：

- `account_type` 取值和可见行为。
- Agent 创建、启用、禁用、绑定 prompt/tool/skill 的 API 行为。
- Skill 文件读取权限边界。
- Python 执行能力边界。
- Agent 消息写回 IM 的路径。
- IM 事件到 Agent run 的触发类型和循环预防规则。
- 审计记录至少覆盖 Agent run、tool invocation、skill file read、python execution。

## 非目标

第一版不实现：

- shell/命令行脚本执行；
- 用户自定义 stdio MCP 进程启动；
- 复杂 Agent marketplace；
- 长期 Agent memory；
- 多 Agent 自动协商协议；
- 前端复杂 Agent 编排画布；
- 跨租户公开 skill 共享市场。
