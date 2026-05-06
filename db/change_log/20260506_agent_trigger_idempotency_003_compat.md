# Agent Trigger Idempotency Migration 003 Compatibility

## Purpose

Allow migration 003 to run on databases where `agent_trigger_idempotency`
already exists with legacy `trigger_message_id` and `response_message_id`
columns.

## Impacted Tables / Fields / Indexes

- Table: `agent_trigger_idempotency`
- Fields: `trigger_server_msg_id`, `response_server_msg_id`
- Index: `agent_trigger_idempotency_trigger_idx`

## Destructive?

No. Legacy columns are renamed to the current contract names when the current
columns do not already exist. Missing current columns are added without deleting
rows.

## Apply Order

This is a human/audit note only. The executable source of truth is
`db/migrations/003_agent_conversation_hosting.sql`, applied by the normal
PostgreSQL migration flow before deploying application code that depends on the
current agent trigger idempotency contract.

## Rollback / Recovery

Rollback is not automatic. If recovery is needed, restore from a database backup
or manually rename the current columns back only after rolling application code
back to a version that expects the legacy names.

## Verification

```bash
git diff --check
go test ./...
bash scripts/verify-static.sh
DATABASE_URL='[REDACTED]' bash scripts/verify-postgres-old-schema-upgrade.sh
```
