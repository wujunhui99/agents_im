# Database Schema V2 Discussion Notes

Status: Draft

This document records the staged discussion for the next PostgreSQL schema cleanup. It is intentionally design-first: do not treat it as implemented until the migration, repository code, API/RPC contracts, tests, and deployment verification are updated.

## Background

The current schema grew from the first account/profile split, auth/friends/groups MVP, messaging storage, and Agent V0 tables. The next schema pass should optimize for IM-style service ownership, application-level validation, and future sharding/partitioning.

## Goals

- Use application code as the primary place for business validation and enum validation.
- Avoid physical foreign keys in PostgreSQL for service-owned tables; use ordinary indexes and program-level referential checks instead.
- Use `smallint` for enumerable values instead of text enums.
- Clarify account/profile/auth/friend/group-member models before implementation.
- Keep IDs internal and stable; do not expose internal account IDs in frontend display.

## Non-goals

- This document does not implement migrations.
- This document does not finalize message, outbox, delivery, media, Agent, or MCP tables yet. Those will be discussed in the next round and appended here.
- This document does not decide physical partitioning/sharding yet.

## Cross-table principles accepted in this discussion

### 1. No physical foreign keys by default

Decision: do not create PostgreSQL physical foreign key constraints for the next schema version.

Rationale:

- This matches common large-scale internet service practice: services own relationship correctness in application logic, and the database mainly stores indexed facts.
- It reduces write-time FK checks, lock coupling, migration friction, and cross-table cleanup coupling.
- It makes future table splitting, service decomposition, and sharding easier.

Implementation rule:

- Relationship columns still need ordinary btree indexes when they are used for lookup or cleanup.
- Application code must validate referenced records before creating dependent records when the business flow requires it.
- Repository/service tests must cover missing-reference rejection.
- Add periodic integrity inspection scripts later for orphan detection; do not rely on DB FK errors.

Examples:

- `profiles.account_id` references `accounts.account_id` by convention and service code, not by DB FK.
- `auth_credentials.account_id` references `accounts.account_id` by convention and service code.
- `friendships.account_id` and `friendships.friend_account_id` reference accounts by convention and indexed lookup.
- `group_members.group_id` and `group_members.account_id` reference groups/accounts by convention and indexed lookup.

### 2. No DB CHECK for business logic or enum ranges

Decision: business logic validation should live in application code, not DB CHECK constraints.

This includes:

- Enum range validation, such as account type, gender, friend status, member role/status.
- Date business validation, such as birth date not being in the future.
- State transition validation, such as friend delete/block transitions or group member rejoin behavior.

Rationale:

- Application code can return stable API errors and localized/user-friendly messages.
- Validation rules can evolve without DB constraint churn.
- Avoid hidden divergence where DB rejects a value differently from API/RPC validation.
- Keep PostgreSQL constraints focused on storage primitives, not business policy.

Implementation rule:

- API/RPC/domain constructors must validate enum values before repository writes.
- Tests must include invalid enum/date inputs and assert service-level errors.
- PostgreSQL migrations should avoid CHECK constraints except purely physical storage constraints if later explicitly agreed.

### 3. Use `smallint` for all enumerable fields

Decision: enumerable values should use `smallint` in PostgreSQL and int-like values in Go/API/RPC/TypeScript contracts.

Rationale:

- Smaller storage and indexes than text.
- Faster comparisons and less index bloat.
- Display/i18n labels belong in frontend/product mapping, not database string values.
- Consistent with `account_type`: `0 = admin`, `1 = user`, `2 = agent`.

Implementation rule:

- Each enum must have a single authoritative constants file in backend domain/model code.
- API/RPC docs must publish numeric meanings.
- Frontend must map numeric values to labels; it must not infer labels from database strings.

## Proposed enum registry for upper-half tables

### account_type

- `0`: admin
- `1`: user
- `2`: agent

Default: `1`.

### gender

- `0`: unknown
- `1`: male
- `2`: female
- `3`: other

Default: `0`.

### friend_status

- `1`: normal
- `2`: deleted
- `3`: blocked

Default: `1`.

### group_member_role

Initial proposal:

- `1`: member
- `2`: admin
- `3`: owner

Default: `1`.

### group_member_status

Initial proposal:

- `1`: normal
- `2`: removed

Default: `1`.

Muted/no-disturb should probably be separate per-user setting later, not the base membership status, unless product wants group-level mute enforcement.

## Table decisions discussed so far

## 1. accounts

Purpose: stable internal account identity and login/search identifier ownership.

Proposed columns:

```text
account_id text primary key
identifier text not null unique
account_type smallint not null default 1
created_at timestamptz not null default now()
updated_at timestamptz not null default now()
```

Decisions:

- `account_id` remains the stable internal primary key.
- `identifier` remains unique in `accounts`.
- `account_type` becomes `smallint`, not text.
- No DB CHECK for numeric-only ID, account type enum, or identifier blankness. These move to application validation.
- No role prefixes in `account_id`; continue using Snowflake-style numeric strings.

Indexes:

```text
primary key (account_id)
unique index accounts_identifier_uniq(identifier)
ordinary index accounts_account_type_idx(account_type) if filtering by type becomes common
```

Application responsibilities:

- Generate valid Snowflake numeric string IDs.
- Validate identifier format and uniqueness conflict handling.
- Validate account_type is one of the defined constants.

## 2. profiles

Purpose: user/agent/admin display profile attached to an account.

Proposed columns:

```text
account_id text primary key
display_name text not null
name text not null
gender smallint not null default 0
birth_date date null
region text not null default ''
avatar_media_id text not null default ''
created_at timestamptz not null default now()
updated_at timestamptz not null default now()
```

Decisions:

- No physical FK from `profiles.account_id` to `accounts.account_id`.
- Keep `account_id` as primary key to preserve 1:1 relationship by convention.
- `gender` becomes `smallint`.
- Do not store age. Store `birth_date`; calculate age dynamically if needed.
- Do not use DB CHECK for `birth_date <= current_date`.
- Do not use DB CHECK for gender enum or blank names.

Indexes:

```text
primary key (account_id)
index profiles_display_name_idx(display_name) only if search/listing needs it later
index profiles_avatar_media_id_idx(avatar_media_id) only if media cleanup needs it later
```

Application responsibilities:

- Ensure account exists before profile creation.
- Ensure one profile per account via primary key conflict handling.
- Validate display/name non-empty.
- Validate gender enum.
- Validate birth_date is not in the future.
- Delete or archive profile when account is deleted according to service policy.

Discussion note:

- Removing the physical FK improves decoupling and avoids FK checks. The tradeoff is that orphan profiles are now possible if application code or operational scripts are wrong; this must be covered by tests and future integrity scans.

## 3. auth_credentials

Purpose: password credential record for an account.

Final direction from discussion: store only `account_id` as the account reference; do not duplicate `identifier` in `auth_credentials`.

Reasoning:

- Duplicating `identifier` in both `accounts` and `auth_credentials` creates update coupling. If the user changes identifier, both tables must be updated in one transaction.
- Keeping identifier only in `accounts` makes identifier changes affect one table only.
- Login can first resolve `accounts.identifier -> account_id`, then read `auth_credentials.account_id -> password_hash`; or use a join/query encapsulated in repository code.
- This avoids long-term drift between two unique identifier copies.

Proposed columns:

```text
account_id text primary key
password_hash text not null
password_algo smallint not null default 1
created_at timestamptz not null default now()
updated_at timestamptz not null default now()
```

Password algorithm enum:

- `1`: bcrypt

Decisions:

- Rename old `user_id` to `account_id`.
- Do not store `identifier` in `auth_credentials`.
- Use Go bcrypt implementation: `golang.org/x/crypto/bcrypt`.
- Do not store separate `salt`; bcrypt hash already embeds algorithm/cost/salt/hash material.
- Keep `password_algo` for future migration to another algorithm.
- No physical FK to accounts.
- No DB CHECK for password_algo enum.

Indexes:

```text
primary key (account_id)
```

Login lookup pattern:

1. Query `accounts` by unique `identifier` to get `account_id`, account type, and account state when account state exists.
2. Query `auth_credentials` by `account_id` to get hash and algorithm.
3. Verify password with bcrypt in application code.

Possible optimization later:

- If login read latency becomes important, repository can use a single SQL join:

```sql
select a.account_id, a.account_type, c.password_hash, c.password_algo
from accounts a
join auth_credentials c on c.account_id = a.account_id
where a.identifier = $1
```

This still keeps identifier stored once.

Application responsibilities:

- Create account and credential in one transaction during registration.
- When identifier changes, only update `accounts.identifier`.
- Validate password policy before hashing.
- Use bcrypt compare and generate functions; never implement custom password hashing.

## 4. friendships

Purpose: per-account friend relationship view.

Decisions:

- Store one directional row for a pending request: `requester -> recipient`; store two directional rows for an accepted friendship: `A -> B` and `B -> A`.
- Rename fields from user terminology to account terminology.
- Use `smallint` friend status.
- Default status is `1 = accepted` for compatibility with existing active rows.
- No physical FKs; program validates account existence.

Proposed columns:

```text
account_id text not null
friend_account_id text not null
status smallint not null default 1
created_at timestamptz not null default now()
updated_at timestamptz not null default now()
primary key (account_id, friend_account_id)
```

Status enum:

- `1`: accepted
- `2`: deleted
- `3`: pending
- `4`: rejected

Semantics:

- `pending`: visible as outgoing for `account_id` and incoming for `friend_account_id`; not visible in the accepted friend list.
- `accepted`: visible in this account's friend list.
- `rejected`: request was rejected; not visible in the accepted friend list.
- `deleted`: this account deleted the friend; not visible in the accepted friend list.

Directional examples:

- If A deletes B:
  - `A -> B = deleted`
  - `B -> A` can remain `normal`.
- If A requests B:
  - `A -> B = pending`
  - no `B -> A` row is required until accept/reject.
- If B accepts A:
  - `A -> B = accepted`
  - `B -> A = accepted`

Indexes:

```text
primary key (account_id, friend_account_id)
index friendships_account_status_idx(account_id, status, friend_account_id)
index friendships_friend_reverse_idx(friend_account_id, account_id) if reverse lookup is needed
```

Application responsibilities:

- Validate both accounts exist before creating friendship rows.
- Write the two directional rows in one transaction for add/accept flows.
- Apply delete/block semantics directionally.
- Validate status enum in service/domain code.

## 5. groups

Purpose: group metadata.

Preliminary proposed columns:

```text
group_id text primary key
name text not null
description text not null default ''
creator_account_id text not null
created_at timestamptz not null default now()
updated_at timestamptz not null default now()
```

Preliminary decisions:

- Rename `creator_user_id` to `creator_account_id`.
- No physical FK to accounts.
- No DB CHECK for non-empty fields; program validates.

Indexes:

```text
primary key (group_id)
index groups_creator_account_idx(creator_account_id, created_at desc)
```

This section is preliminary; full group metadata will be revisited in the next discussion if needed.

## 6. group_members

Purpose: current membership row for account membership in a group.

User decision: only keep join time; remove left time.

Proposed columns:

```text
group_id text not null
account_id text not null
role smallint not null default 1
status smallint not null default 1
join_time timestamptz not null default now()
created_at timestamptz not null default now()
updated_at timestamptz not null default now()
primary key (group_id, account_id)
```

Decisions:

- Rename `user_id` to `account_id`.
- Remove `left_at`.
- Keep only `join_time` for membership join/rejoin time.
- Use `smallint` for role/status.
- No physical FKs to `groups` or `accounts`.
- No DB CHECK for enum ranges or time logic.

Rejoin behavior:

- If no row exists, insert row with `status = normal` and `join_time = now()`.
- If row exists with `status = removed`, update it to `status = normal`, refresh `join_time = now()`, and update `updated_at`.
- If row already exists with `status = normal`, treat join as idempotent success or return already-member depending on API semantics; choose one explicitly during implementation.

Leave behavior:

- Do not delete row by default.
- Set `status = removed` and update `updated_at`.
- Do not store left time in this table.

History note:

- If product later needs full membership history, add a separate append-only `group_member_events` table rather than overloading `group_members`.
- Potential event types: joined, left, kicked, role_changed, muted, unmuted.

Indexes:

```text
primary key (group_id, account_id)
index group_members_account_status_idx(account_id, status, group_id)
index group_members_group_status_idx(group_id, status, account_id)
```

Application responsibilities:

- Validate group exists before join.
- Validate account exists before join.
- Validate role/status enum in service/domain code.
- Enforce owner/admin/member permissions in service code.

## 7. IM conversation and media tables

### Cross-cutting ID rule

Decision: unless a table has a strong reason to use a natural/composite key, primary IDs should be Snowflake-style IDs generated by the application.

Implications:

- `conversation_threads.conversation_id` should be a Snowflake-style ID.
- `media_objects.media_id` should be a Snowflake-style ID.
- Append/event tables added later should also use Snowflake-style primary IDs unless the primary key is deliberately a natural idempotency key.
- Existing composite relationship tables can still use composite primary keys when they model a unique relationship, for example `friendships(account_id, friend_account_id)` and `group_members(group_id, account_id)`, unless we later decide every relationship row also needs its own row ID.

### conversation_threads

Purpose: canonical conversation thread for one single chat or one group chat.

Proposed columns:

```text
conversation_id text primary key
conversation_type smallint not null
single_account_a text not null default ''
single_account_b text not null default ''
group_id text not null default ''
max_seq bigint not null default 0
last_message_id text not null default ''
last_message_at timestamptz null
created_at timestamptz not null default now()
updated_at timestamptz not null default now()
```

Conversation type enum:

- `1`: single
- `2`: group

Decisions:

- `conversation_id` uses a Snowflake-style ID.
- Rename `chat_type` to `conversation_type` and store it as `smallint`.
- Rename `single_user_a` / `single_user_b` to `single_account_a` / `single_account_b`.
- Do not create physical FKs to accounts or groups.
- Do not use DB CHECK for single/group shape validation.
- Keep `max_seq`; message creation must atomically increment it, typically with `update ... set max_seq = max_seq + 1 ... returning max_seq`.
- Keep `last_message_id` and `last_message_at` as conversation-list cache fields. They are denormalized and must be updated by the message write flow.

Indexes:

```text
primary key (conversation_id)
unique index conversation_threads_single_pair_uniq(single_account_a, single_account_b) where conversation_type = 1
unique index conversation_threads_group_uniq(group_id) where conversation_type = 2
index conversation_threads_last_message_idx(last_message_at desc) if global/admin listing needs it later
```

Application responsibilities:

- Generate Snowflake-style conversation IDs.
- Validate `conversation_type` enum.
- For single conversations, sort the pair in application code so `single_account_a < single_account_b` by deterministic string ordering before insert/upsert.
- For group conversations, ensure `group_id` is present and account pair fields are empty.
- Ensure referenced accounts/groups exist through service-level checks, not DB FKs.

### user_conversation_states

Purpose: per-account conversation state, such as read position, visibility boundary, mute/archive/pin flags.

Proposed columns:

```text
account_id text not null
conversation_id text not null
last_read_seq bigint not null default 0
visible_start_seq bigint not null default 0
muted boolean not null default false
archived boolean not null default false
pinned_at timestamptz null
created_at timestamptz not null default now()
updated_at timestamptz not null default now()
primary key (account_id, conversation_id)
```

Decisions:

- Rename `user_id` to `account_id`.
- Rename `has_read_seq` to `last_read_seq`; the old name sounds boolean-like and is misleading.
- Rename `last_visible_seq` to `visible_start_seq`.
- Remove the old `has_read_seq <= last_visible_seq` DB CHECK. Under the new semantics it is not the right relationship. A read position can be before or after a visibility reset depending on product flow, and the database should not enforce business logic here.
- Add `pinned_at` for pinned conversations. `pinned_at is not null` means pinned; pinned conversations can be sorted by `pinned_at`.
- Keep `archived`.
- Do not store `unread_count` in V0; compute it from conversation/message state when possible.
- Do not create physical FKs or DB CHECK constraints.

Visibility semantics:

- `visible_start_seq = 0` means the account can see from the beginning, subject to product permissions.
- Clearing a chat can set `visible_start_seq` to the current conversation `max_seq` so older messages are hidden from that account.
- When a member leaves a group and later rejoins, the rejoin should behave like a first join: history is visible only after the rejoin point. On rejoin, set `visible_start_seq` to the current group conversation `max_seq`.
- The exact query should be `seq > visible_start_seq` for history after the boundary, unless product later chooses inclusive semantics and documents it.

Indexes:

```text
primary key (account_id, conversation_id)
index user_conversation_states_account_updated_idx(account_id, updated_at desc)
index user_conversation_states_account_pinned_idx(account_id, pinned_at desc) where pinned_at is not null
```

Application responsibilities:

- Validate referenced account and conversation exist.
- Ensure read advancement is monotonic where the product requires it.
- Set `visible_start_seq` correctly for clear-chat, first group join, leave, and rejoin flows.
- Maintain `updated_at` when mutable state changes.

### media_objects

Purpose: unified object metadata table for uploaded/stored files used by IM and Agent features. File bytes live in OSS/S3/MinIO; PostgreSQL stores metadata and lifecycle state.

Decision: keep one unified `media_objects` table for all object metadata, including IM media and Agent skill files. Agent skill registry tables should still remain separate and link to media object IDs for the actual files belonging to a skill directory.

Proposed columns:

```text
media_id text primary key
owner_account_id text not null default ''
conversation_id text not null default ''
storage_provider smallint not null default 1
bucket text not null
object_key text not null unique
original_filename text not null default ''
content_type text not null
size_bytes bigint not null
purpose smallint not null
status smallint not null default 1
metadata jsonb not null default '{}'::jsonb
expires_at timestamptz null
created_at timestamptz not null default now()
updated_at timestamptz not null default now()
```

Media purpose enum, initial proposal:

- `1`: avatar
- `2`: message_image
- `3`: message_file
- `4`: message_video
- `5`: message_audio
- `6`: agent_skill_file

Media status enum, initial proposal:

- `1`: pending
- `2`: ready
- `3`: rejected
- `4`: deleted
- `5`: expired

Storage provider enum, initial proposal:

- `1`: minio
- `2`: s3
- `3`: aliyun_oss
- `4`: tencent_cos

Decisions:

- `media_id` uses a Snowflake-style ID.
- Rename `owner_user_id` to `owner_account_id`.
- Keep `owner_account_id` and `conversation_id` as non-null text columns with `''` as the not-applicable value, following the existing repository style and avoiding nullable string handling in Go.
- Add `conversation_id` because message media should be organized and authorized primarily by conversation when possible.
- Add `storage_provider` for object storage portability; `bucket + object_key` identify the object within that provider.
- Keep `content_type` as `text`, not `smallint`, because MIME types are open-ended technical values rather than a small business enum.
- Use `purpose` to decide which ownership dimension is required:
  - avatar: `owner_account_id` required, `conversation_id` usually empty.
  - message media: `conversation_id` required, `owner_account_id` should still record uploader/account owner.
  - agent skill file: owner may be the agent/admin/account that uploaded it, while skill tables define directory ownership and references.
- Use `metadata jsonb` for type-specific file information instead of separate nullable columns.
- Remove `sha256` from the proposed required schema for now. SHA-256 can be expensive for large uploads and is not needed for core MVP behavior if OSS/object storage already provides object identity and size/content-type metadata. If future deduplication, tamper verification, malware scanning, or Agent skill integrity requires content hashes, add optional async hash fields later.
- Keep `object_key` unique.
- Do not create physical FKs or DB CHECK constraints.
- Do not use DB CHECK for file size, dimensions, content type, purpose, status, or expiry logic; validate in application code.

`metadata` examples:

```json
{"width": 1280, "height": 720}
```

```json
{"duration_ms": 125000}
```

```json
{"width": 1920, "height": 1080, "duration_ms": 60000}
```

For generic files, `metadata` can remain `{}` or hold safe extracted metadata such as page count later.

OSS object key strategy:

- Prefer conversation-scoped keys for message media.
- Use owner-scoped keys for avatar/profile media.
- Agent skill files can use agent/skill-scoped keys.

Initial key patterns:

```text
agents_im/conversations/{conversation_id}/{safe_filename}
agents_im/accounts/{owner_account_id}/{purpose}/{safe_filename}
agents_im/agents/{agent_id}/skills/{skill_id}/{relative_path_or_safe_filename}
```

Duplicate filename handling:

- If the same logical directory receives the same filename repeatedly, append a numeric suffix before the extension, for example `file.txt`, `file(1).txt`, `file(2).txt`.
- The application must sanitize filenames and generate the final unique object key.
- The database unique index on `object_key` is a final collision guard, not the primary naming algorithm.

Expiry semantics:

- Use one `expires_at` field initially for both pending upload expiry and OSS object expiry.
- For `pending`, `expires_at` means the upload must be completed before this time; after expiry, application/cleanup can mark it `expired` and optionally remove the OSS placeholder/object.
- For `ready`, `expires_at` means the OSS object is expected to expire at or after this time. If OSS has a default seven-day lifecycle, the application should set `expires_at` to the expected expiration timestamp when creating/completing the object.
- If later product needs separate concepts, split into `upload_expires_at` and `object_expires_at`; do not overcomplicate V0 before that need is proven.

Indexes:

```text
primary key (media_id)
unique index media_objects_object_key_uniq(object_key)
index media_objects_owner_status_created_idx(owner_account_id, status, created_at desc)
index media_objects_conversation_status_created_idx(conversation_id, status, created_at desc)
index media_objects_expiry_idx(status, expires_at) where expires_at is not null
```

Application responsibilities:

- Generate Snowflake-style media IDs.
- Validate owner/conversation requirements based on purpose.
- Validate content type, size, and type-specific metadata.
- Generate safe object keys with collision suffixes.
- Set and enforce `expires_at` according to upload/object lifecycle.
- Ensure deleted/expired objects are not usable in messages, avatars, or Agent skill execution.

### Agent skill directory relationship

Decision: keep Agent skill registry separate from `media_objects`, and use a dedicated `agent_skill_files` table to store skill directory structure and media object links.

Proposed table:

```text
skill_file_id text primary key
skill_id text not null
relative_path text not null
media_id text not null
file_role smallint not null
created_at timestamptz not null default now()
updated_at timestamptz not null default now()
```

Initial file role enum:

- `1`: skill_md
- `2`: reference
- `3`: template
- `4`: script
- `5`: asset

Potential direction for later Agent-side discussion:

- `agent_skills` stores skill-level metadata, version, status, name, owner/creator, and high-level directory manifest metadata if needed.
- `agent_skill_files` maps `skill_id + relative_path` to `media_id`.
- One skill can have multiple files, e.g. `SKILL.md`, `references/*.md`, templates, scripts, and assets.
- `media_objects.purpose = agent_skill_file` identifies the object as a skill file, but skill structure belongs to `agent_skill_files` and Agent skill tables.
- Do not use physical FKs; program validates that `skill_id` and `media_id` exist and that the media purpose is `agent_skill_file`.
- Add a unique index on `(skill_id, relative_path)` so one skill version/directory cannot contain duplicate paths.



- Should `groups` include avatar, announcement, visibility, join policy, or owner transfer rules now or later?
- Should `group_members.status` include muted/banned states, or should those be separate moderation tables?
- Should account deletion hard-delete dependent rows, soft-delete accounts, or mark account status? This affects orphan cleanup strategy once physical FKs are removed.
- Should we introduce integrity scan scripts as part of `scripts/verify-static.sh` or as an operational tool?

## 8. Message storage, idempotency, outbox, and delivery

### messages

Purpose: canonical persisted message fact table. A message is durable once it is accepted into this table with a conversation `seq`.

Proposed columns:

```text
message_id text primary key
client_msg_id text not null
sender_account_id text not null
conversation_id text not null
seq bigint not null
conversation_type smallint not null
receiver_account_id text not null default ''
group_id text not null default ''
content_type smallint not null
content jsonb not null
payload_hash text not null
client_send_time timestamptz null
server_received_at timestamptz not null default now()
message_state smallint not null default 1
created_at timestamptz not null default now()
updated_at timestamptz not null default now()
message_origin smallint not null default 1
agent_account_id text not null default ''
trigger_message_id text not null default ''
agent_run_id text not null default ''
allow_recursive_trigger boolean not null default false
```

Message content type enum, initial proposal:

- `1`: text
- `2`: image
- `3`: file
- `4`: audio
- `5`: video
- `6`: system
- `7`: rich

Message origin enum, initial proposal:

- `1`: human
- `2`: agent
- `3`: system

Message state enum, initial proposal:

- `1`: normal
- `2`: recalled
- `3`: deleted
- `4`: hidden_by_moderation

Decisions:

- Rename `server_msg_id` to `message_id`; use a Snowflake-style ID.
- Rename `sender_id` to `sender_account_id`.
- Rename `chat_type` to `conversation_type` and store it as `smallint`.
- Rename `receiver_id` to `receiver_account_id`.
- Store both client and server times:
  - `client_send_time`: timestamp reported by the client, nullable because it is not trusted for ordering.
  - `server_received_at`: server acceptance timestamp, set by the service/database.
- Message ordering is by `(conversation_id, seq)`, not by client time.
- Keep `content jsonb` for type-specific message body.
- Keep `payload_hash`, but use it only with `(sender_account_id, client_msg_id)` idempotency.
- Do not create physical FKs or DB CHECK constraints.

Payload hash semantics:

- `payload_hash` is the canonical hash of the client's intended message payload, not a file hash.
- It should include stable fields that define the intended message target and body, for example:
  - `conversation_id`
  - `conversation_type`
  - `receiver_account_id` or `group_id` as applicable
  - `content_type`
  - canonical JSON form of `content`
  - `message_origin` if clients/internal producers can create non-human messages
- It should exclude server-generated fields such as `message_id`, `seq`, `server_received_at`, `created_at`, `updated_at`, and outbox data.
- It should generally exclude `client_send_time` so that a retry with the same `client_msg_id` but a slightly different client timestamp does not become a false conflict.
- If the same account sends two identical messages intentionally, the client must use two different `client_msg_id` values. They will produce the same `payload_hash` but become two distinct rows because the idempotency key is `(sender_account_id, client_msg_id)`, not `payload_hash` alone.
- If the same `(sender_account_id, client_msg_id)` is retried with the same `payload_hash`, return the existing message.
- If the same `(sender_account_id, client_msg_id)` is reused with a different `payload_hash`, return an idempotency conflict error.

Indexes:

```text
primary key (message_id)
unique index messages_conversation_seq_uniq(conversation_id, seq)
unique index messages_sender_client_msg_uniq(sender_account_id, client_msg_id)
index messages_conversation_seq_idx(conversation_id, seq)
index messages_sender_created_idx(sender_account_id, created_at desc)
index messages_agent_run_idx(agent_run_id) where agent_run_id <> ''
```

Application responsibilities:

- Generate Snowflake-style message IDs.
- Atomically allocate `seq` by incrementing `conversation_threads.max_seq` during message creation.
- Validate sender/conversation/receiver/group shape and content type in service code.
- Canonicalize and hash payload consistently.
- Handle idempotent retry vs idempotency conflict using `messages` directly.

### message_idempotency_keys

Decision: remove this table in schema V2.

Reasoning:

- `messages` already has the necessary unique idempotency key: `(sender_account_id, client_msg_id)`.
- `messages.payload_hash` can distinguish idempotent retry from client ID reuse with different content.
- Removing the separate table reduces write complexity and avoids consistency drift between idempotency records and message facts.

Replacement behavior:

1. Try to insert the message row with `(sender_account_id, client_msg_id)`.
2. If insert succeeds, return the new message.
3. If unique conflict occurs, load the existing message by `(sender_account_id, client_msg_id)`.
4. If existing `payload_hash` matches, return the existing message as idempotent success.
5. If hashes differ, return idempotency conflict.

### message_outbox

Purpose: transactional outbox for reliable message events. It decouples message persistence from asynchronous push/transfer/Kafka publication.

Proposed columns:

```text
event_id text primary key
event_type smallint not null
aggregate_type smallint not null
aggregate_id text not null
conversation_id text not null
message_id text not null
seq bigint not null
payload jsonb not null
status smallint not null default 1
attempt_count integer not null default 0
next_attempt_at timestamptz not null default now()
locked_by text not null default ''
locked_until timestamptz null
last_error text not null default ''
created_at timestamptz not null default now()
updated_at timestamptz not null default now()
published_at timestamptz null
```

Outbox event type enum, initial proposal:

- `1`: message_created
- `2`: message_recalled
- `3`: message_deleted
- `4`: conversation_updated

Outbox aggregate type enum, initial proposal:

- `1`: message
- `2`: conversation

Outbox status enum, initial proposal:

- `1`: pending
- `2`: published
- `3`: failed
- `4`: dead_letter

Event type and aggregate explanation:

- `event_type` describes what happened, for example `message_created`.
- `aggregate_type` describes what kind of entity the event belongs to, for example `message`.
- `aggregate_id` is the ID of that entity, for example `message_id` for a message event.
- The tuple `(event_type, aggregate_type, aggregate_id)` is a natural idempotency key for outbox event creation. It prevents creating duplicate events for the same business fact.
- Example: when message `m1` is accepted, create an outbox row with `event_type=message_created`, `aggregate_type=message`, `aggregate_id=m1`, `message_id=m1`.
- `message_id`, `conversation_id`, and `seq` are repeated as query/routing fields so workers do not need to parse payload for common routing.

Outbox flow:

1. In one DB transaction, create/update the message and insert a pending outbox row.
2. Outbox workers claim pending rows with `next_attempt_at <= now()` by setting `locked_by` and `locked_until`.
3. Worker publishes/processes the event, e.g. transfer to WebSocket gateway, Kafka/Redpanda, or internal dispatcher.
4. On success, set `status=published`, `published_at=now()`, clear lock fields.
5. On transient failure, increment `attempt_count`, set `status=pending`, set `next_attempt_at` with backoff, and record a sanitized `last_error`.
6. On repeated/permanent failure, set `status=dead_letter` and alert/inspect; do not loop forever.

Indexes:

```text
primary key (event_id)
unique index message_outbox_event_aggregate_uniq(event_type, aggregate_type, aggregate_id)
index message_outbox_pending_idx(status, next_attempt_at, created_at, event_id)
index message_outbox_locked_idx(status, locked_until)
index message_outbox_conversation_seq_idx(conversation_id, seq)
index message_outbox_message_idx(message_id)
```

### delivery_attempts

Decision: remove `delivery_attempts` from the core V2 schema unless a later product requirement needs per-recipient durable delivery audit.

Reasoning:

- IM reliability should primarily come from durable message storage, per-conversation `seq`, reconnect sync, and read positions.
- A per-recipient delivery row creates large write amplification, especially for group chats.
- Online push is a real-time optimization; offline users can recover by pulling messages after reconnect using `conversation_id + seq`.
- Per-recipient delivery success/failure can be observed through metrics/logs initially, without storing every attempt in PostgreSQL.

Replacement delivery model:

- `messages` stores the durable message fact.
- `message_outbox` stores reliable asynchronous processing state for each message event.
- `user_conversation_states.last_read_seq` stores read progress.
- WebSocket/transfer workers push online messages best-effort and rely on reconnect sync for missed messages.

Single-chat delivery behavior:

- Message creation inserts `messages` and a `message_created` outbox row in one transaction.
- Outbox worker processes the event immediately.
- If the recipient is online, push the message to the gateway/client.
- If the recipient is offline, mark the outbox event as published/processed; the recipient will fetch by `seq` on reconnect.
- If infrastructure fails, e.g. gateway/dispatcher unavailable, keep/retry the outbox event with backoff.
- Client deduplicates by `message_id` or `(conversation_id, seq)` if push and sync both deliver the same message.

Group-chat delivery behavior:

- Message creation is immediate and durable.
- V1 group push is also real-time: after the `message_created` outbox event is processed, the worker immediately pushes to online group members.
- Do not add a one-second aggregation window in V1; batching can be revisited later if group push pressure becomes a real bottleneck.
- Offline or missed clients recover by syncing messages where `seq > last_read_seq` or `seq > visible_start_seq`, depending on query purpose.
- Do not write one delivery row per group member in V2.

Message state vs delivery state:

- `messages.message_state` should represent message lifecycle, such as normal/recalled/deleted/moderated.
- It should not represent per-recipient delivery state.
- `message_outbox.status` represents asynchronous event processing state, not whether every user has received the message.


- Agent, MCP, and audit tables still need a lower-half schema review.

## Implementation checklist later

Do not implement until discussion is complete and an execution plan is approved.

- Update migrations or create a new migration to remove physical FKs and DB CHECK constraints.
- Convert text enums to `smallint`.
- Rename user-oriented columns to account-oriented columns where needed.
- Move all enum/date/reference validation into domain/service code.
- Add repository/service tests for invalid enum/date/reference inputs.
- Update API/RPC/TypeScript contracts.
- Update generated docs and product/design docs.
- Run full local verification before deploy.
- Because existing online PostgreSQL was previously reset during account/profile work, deployment still needs explicit data-cleanup/migration confirmation before applying destructive schema changes.
