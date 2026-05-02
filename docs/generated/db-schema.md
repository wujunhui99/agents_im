# Database Schema

状态：Manual Snapshot

本文档用于记录数据库 schema。权威来源仍是 [`../../db/migrations/`](../../db/migrations/) 下的迁移文件；未来应由迁移文件或数据库 introspection 自动生成，避免手写文档与真实 schema 漂移。

## 当前状态

第一阶段 PostgreSQL migration 已覆盖账号资料、认证、好友、群聊、消息、outbox、delivery attempt、Agent 管理表、Agent prompt/tool/skill registry 元数据表和 Agent audit 表。`accounts.account_type` 支持 `user`、`agent`、`admin`，默认 1（用户）；账号资料拆分为 `accounts` 与 `profiles`。

## 当前覆盖

- accounts
- profiles
- auth_credentials
- friendships
- groups
- group_members
- conversation_threads
- messages
- user_conversation_states
- message_idempotency_keys
- message_outbox
- delivery_attempts
- agents
- agent_prompts
- mcp_servers
- agent_tools
- agent_skills
- agent_prompt_bindings
- agent_tool_bindings
- agent_skill_bindings
- agent_runs
- agent_tool_calls
- agent_file_reads
- agent_python_execs

## Agent Management

`db/migrations/002_agent_management.sql` 新增 `agents`。

`agents` 字段：

- `agent_id`
- `account_id`
- `name`
- `description`
- `status`
- `created_by`
- `created_at`
- `updated_at`

约束：

- `agent_id` 为无前缀 Snowflake 数字字符串。
- `account_id` 唯一，并引用 `accounts(account_id)`。
- `status` 只能为 `draft`、`active`、`disabled`、`archived`。
- Agent 配置独立于 `profiles` 表；Agent 展示资料和头像来自 `profiles`，类型来源为 `accounts.account_type=2`（Agent）。

## 后续预期覆盖

- agent_conversation_bindings
- tool_invocations
- webhook_deliveries

## Agent Registry 约束摘要

- `agent_prompts` 保存 system prompt 内容、版本、状态、创建人和时间戳。
- `mcp_servers` 只允许 `http`、`sse`、`streamable_http` transport，且必须是管理员配置；第一版不保存 stdio command/args。
- `agent_tools.tool_type` 只能是 `mcp`、`local`、`builtin`。MCP tool 必须引用 `mcp_servers`；local tool 只保存服务端白名单 `handler_key`；builtin tool 只保存 `builtin_key`。
- `agent_skills` 只保存 skill 文件对象元数据：`object_key`、`sha256`、`content_type`、`size_bytes`；PostgreSQL 不保存 skill 文件内容。
- `agent_prompt_bindings`、`agent_tool_bindings`、`agent_skill_bindings` 使用 `(agent_id, *_id)` 主键去重。当前分支不对 `agent_id` 加外键，以便与 Agent profile 分支并行集成。
