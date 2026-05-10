alter table if exists auth_credentials
  add column if not exists email_normalized text not null default '';

alter table if exists auth_credentials
  add column if not exists email_verified_at timestamptz;

create unique index if not exists auth_credentials_email_normalized_uniq
  on auth_credentials (email_normalized)
  where email_normalized <> '';

create table if not exists auth_email_verification_tokens (
  id text primary key,
  purpose smallint not null,
  email_normalized text not null,
  code_hash text not null,
  code_hash_algo smallint not null default 1,
  expires_at timestamptz not null,
  consumed_at timestamptz,
  attempt_count integer not null default 0,
  last_sent_at timestamptz not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists auth_email_verification_tokens_email_purpose_created_idx
  on auth_email_verification_tokens (email_normalized, purpose, created_at desc);

create index if not exists auth_email_verification_tokens_unconsumed_expiry_idx
  on auth_email_verification_tokens (expires_at)
  where consumed_at is null;
