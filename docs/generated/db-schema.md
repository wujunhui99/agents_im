# Database Schema

状态：Manual Snapshot

本文档用于记录数据库 schema。权威来源仍是 [`../../db/migrations/001_init_postgres.sql`](../../db/migrations/001_init_postgres.sql)；未来应由迁移文件或数据库 introspection 自动生成，避免手写文档与真实 schema 漂移。

## 当前状态

第一阶段 PostgreSQL migration 已覆盖用户、认证、好友、群聊、消息、outbox、delivery attempt 和 Agent profile 管理表。

## 当前覆盖

- users
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

## Agent Management

`db/migrations/002_agent_management.sql` 新增：

- `agents_im_agents_id_seq`
- `agents`

`agents` 字段：

- `agent_id`
- `im_user_id`
- `name`
- `description`
- `status`
- `created_by`
- `created_at`
- `updated_at`

约束：

- `im_user_id` 唯一，并引用 `users(user_id)`。
- `status` 只能为 `draft`、`active`、`disabled`、`archived`。
- Agent 配置独立于 `users` 表；`users` 只提供 IM 展示身份和账号类型来源。

## 后续预期覆盖

- agent_conversation_bindings
- tool_invocations
- webhook_deliveries
