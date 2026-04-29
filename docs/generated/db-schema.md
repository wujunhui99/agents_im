# Database Schema

状态：Manual Snapshot

本文档用于记录数据库 schema。权威来源仍是 [`../../db/migrations/001_init_postgres.sql`](../../db/migrations/001_init_postgres.sql)；未来应由迁移文件或数据库 introspection 自动生成，避免手写文档与真实 schema 漂移。

## 当前状态

第一阶段 PostgreSQL migration 已覆盖用户、认证、好友、群聊、消息和 outbox 表。

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

## 后续预期覆盖

- agents
- agent_conversation_bindings
- tool_invocations
- webhook_deliveries
