-- Change log: move account email ownership out of auth credentials.
--
-- Email is account identity/profile data, not credential data. Auth still owns
-- password/session state and temporary verification tokens. Runtime login by
-- email resolves accounts.email_normalized -> auth_credentials.account_id.

alter table if exists accounts
  add column if not exists email_normalized text not null default '';

alter table if exists accounts
  add column if not exists email_verified_at timestamptz;

update accounts a
set email_normalized = c.email_normalized,
    email_verified_at = c.email_verified_at,
    updated_at = now()
from auth_credentials c
where c.account_id = a.account_id
  and coalesce(a.email_normalized, '') = ''
  and coalesce(c.email_normalized, '') <> '';

create unique index if not exists accounts_email_normalized_uniq
  on accounts (email_normalized)
  where email_normalized <> '';

drop index if exists auth_credentials_email_normalized_uniq;

alter table if exists auth_credentials
  drop column if exists email_verified_at;

alter table if exists auth_credentials
  drop column if exists email_normalized;
