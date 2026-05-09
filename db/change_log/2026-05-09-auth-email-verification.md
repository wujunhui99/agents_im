# 2026-05-09 Auth Email Verification

## Purpose

Require registration email verification in Auth while keeping verification secrets in Auth-owned storage.

## Impacted Tables / Fields / Indexes

- `auth_credentials`: adds `email_normalized`, `email_verified_at`, and a partial unique index for non-empty verified emails.
- `auth_email_verification_tokens`: new table for hashed registration codes, purpose, normalized email, expiry, consumed state, attempt count, and last send time.

## Destructive?

No. The migration is additive and idempotent.

## Apply Order

1. Apply `db/migrations/008_auth_email_verification.sql`.
2. Deploy Auth changes that require `POST /auth/register/email-code` before `POST /auth/register`.

## Rollback / Recovery

Rollback is not automatic because removing verification tables/columns would discard audit/state for issued registration codes. If rollback is required before any new registrations, stop Auth, restore the previous application version, and drop the added table/columns only after confirming no in-flight registration codes are needed.

## Verification

```bash
go test ./internal/auth/logic ./internal/auth/repository ./tests
bash scripts/verify-static.sh
AGENTS_IM_CONFIRM_TRUNCATE=1 scripts/verify-postgres-local.sh
```
