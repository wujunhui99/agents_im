# Registration Email Service Research for `agenticim.xyz`

Status: Accepted for Issue #10  
Date: 2026-05-05  
Scope: research and implementation input only; no application code or deployment is changed by this document.

## Decision

For the MVP registration verification flow, use a **transactional email provider or SMTP relay** with `no-reply@agenticim.xyz` as the sender identity.

Do **not** self-host a full mailbox stack for the MVP. Do **not** rely on a raw self-hosted outbound SMTP server as the primary production path unless a provider/relay is unavailable and the server IP reputation, outbound ports, PTR/rDNS, SPF, DKIM and DMARC are all confirmed healthy.

Recommended shape:

```text
agents_im auth service
  -> internal Mailer interface
  -> SMTP/transactional relay config from env/secret only
  -> provider authenticates agenticim.xyz with SPF/DKIM/DMARC
  -> no-reply@agenticim.xyz sends registration verification code
```

Rationale:

- Registration codes need reliable outbound delivery, not IMAP/webmail/user mailboxes.
- Transactional providers/relays handle DKIM signing, IP reputation and bounce/abuse controls better than a small self-hosted sender.
- The current production node already runs PostgreSQL, Redis, Redpanda, RustFS and the app services; adding a full mail suite would add disproportionate operational risk.
- SMTP credentials/API keys can be injected through existing secret management without committing secrets.

## Options Compared

| Option | Fit for MVP | Pros | Cons / Risks | Recommendation |
| --- | --- | --- | --- | --- |
| Transactional email provider / SMTP relay | High | Best deliverability; provider-managed IP reputation; supports domain authentication; minimal server footprint; simple app integration | Requires provider account and SMTP/API secret; may have free-tier limits | **Recommended** |
| Lightweight self-hosted sender: maddy / OpenSMTPD / Postfix minimal / Stalwart minimal | Medium | Full control; can run as a small container; no third-party mail API dependency | Deliverability depends on server IP reputation; needs PTR/rDNS and careful DNS/TLS; abuse handling is harder; outbound port 25 may be blocked by host or destination | Use only as fallback or local/dev relay after #10 acceptance |
| Full mailbox stack: mailcow / docker-mailserver / Mailu / iRedMail | Low | Provides mailbox, IMAP, webmail, antispam and admin UI | Overkill for verification-code sending; high memory/storage/ops burden; broad attack surface; more DNS/TLS moving parts | **Not recommended** for MVP |

Provider examples that fit the recommended class include Resend, Mailgun, SendGrid, Postmark, Amazon SES, Brevo, and any equivalent SMTP relay that supports custom-domain DKIM and secret-based SMTP/API auth. The repository must not depend on one hard-coded provider; use a generic SMTP/transactional-mail adapter seam.

## Observed Environment and DNS Baseline

All sensitive server identity, credentials and exact infrastructure origins are redacted.

### DNS check

Tooling note: this WSL image did not have `dig` or `nslookup` installed during the controller check. A fallback `getent` / Python resolver check found:

- `agenticim.xyz` has an A record resolving to `[REDACTED_SERVER_ORIGIN]`.
- `_dmarc.agenticim.xyz` did not resolve through the fallback resolver.
- Common DKIM selectors checked (`default`, `mail`, `smtp`, `selector1`) did not resolve through the fallback resolver.
- MX/TXT could not be fully verified locally without DNS query tooling; provider-side DNS must be checked before #11 is accepted.

Required #11 DNS verification commands should be run from an environment with DNS tooling:

```bash
dig +short A agenticim.xyz
dig +short MX agenticim.xyz
dig +short TXT agenticim.xyz
dig +short TXT _dmarc.agenticim.xyz
dig +short TXT <provider-selector>._domainkey.agenticim.xyz
```

Expected DNS target for the recommended provider/relay path:

- SPF TXT includes the selected provider/relay include or authorized sending host.
- DKIM TXT exists at the exact selector(s) provided by the selected provider.
- DMARC TXT exists at `_dmarc.agenticim.xyz`; start with a monitoring/quarantine policy if needed, then tighten after delivery is stable.
- MX is not required for pure send-only transactional mail unless the provider requires inbound verification or bounce handling via domain mail routing.
- PTR/rDNS is required only if sending directly from the server IP; it is not normally required when using a provider/relay's outbound infrastructure.

### Server/resource check

A best-effort server check through the existing SSH alias succeeded, with all host identity redacted:

- CPU: 4 cores.
- Memory: about 7.8 GiB total; current app stack already running.
- Disk: about 88 GiB root volume, about 27% used at check time.
- Existing middleware observed: PostgreSQL, Redis, Redpanda, RustFS.
- Existing `agents-im` pods were running at check time.

This is enough capacity for a small SMTP relay sidecar/container if needed, but the recommended provider/relay path does not require a new heavy service.

### Port/network check

A local outbound TCP smoke check to public SMTP endpoints reported ports 25, 465 and 587 reachable from the WSL environment. This is not proof of production-node deliverability or provider acceptance. #11 must verify the actual deployment node and selected provider/relay using redacted smoke evidence.

Inbound SMTP ports are not required for provider/relay-based registration code sending.

## Recommended #11 Configuration Checklist

### DNS

Use the selected provider's exact DNS instructions. At minimum record:

```text
SPF:   TXT agenticim.xyz = provider-authorized SPF value
DKIM:  TXT <selector>._domainkey.agenticim.xyz = provider DKIM public key
DMARC: TXT _dmarc.agenticim.xyz = v=DMARC1; p=none/quarantine; rua=... (optional)
```

Do not commit DNS provider credentials or DKIM private keys. DKIM private material must stay provider-side or in server secrets only.

### App/server secrets

Use environment/secret injection only:

```text
MAILER_ENABLED=true
MAILER_PROVIDER=smtp
SMTP_HOST=<provider smtp host>
SMTP_PORT=587
SMTP_USERNAME=<secret>
SMTP_PASSWORD=<secret>
SMTP_FROM_ADDRESS=no-reply@agenticim.xyz
SMTP_FROM_NAME=Agentic IM
SMTP_TLS_MODE=starttls
SMTP_TIMEOUT=10s
```

Repository examples must use placeholders only. Logs and GitHub comments must write secrets as `[REDACTED]`.

### Container/deploy direction

Provider/relay path:

- No new middleware is required.
- Add only application config/secret documentation and k3s secret references when the app feature consumes mailer config.
- `docker-compose.yml` does not need a production mail container.

Self-host fallback path:

- Add a lightweight service only if the provider path is rejected.
- Prefer maddy or OpenSMTPD/Postfix minimal, not a full mailbox stack.
- Use resource limits, restart policy, TLS cert management, outbound-only posture when possible, and explicit log retention.
- Verify PTR/rDNS and deliverability before enabling production registration.

### Smoke verification for #11

Use a controlled mailbox or provider sandbox. Evidence must redact recipient details if sensitive and must never include verification codes or SMTP credentials.

```bash
# DNS
for q in \
  "TXT agenticim.xyz" \
  "TXT _dmarc.agenticim.xyz" \
  "TXT <selector>._domainkey.agenticim.xyz"; do
  dig +short $q
 done

# SMTP handshake/auth without printing credentials
openssl s_client -starttls smtp -connect "$SMTP_HOST:587" -servername "$SMTP_HOST" </dev/null

# App/provider smoke should print only success/failure and provider message id redacted if needed.
```

Failure cases that #11 must surface:

- DNS not propagated or incorrect SPF/DKIM/DMARC.
- SMTP auth failure.
- Provider domain not verified.
- Provider rate limit/rejection.
- Outbound network blocked.
- TLS handshake failure.

## Recommended #12 App Contract and Safety Constraints

### Backend contract

Add a mailer seam in Auth, not Account. Account Service must continue to avoid password/credential/verification secret ownership.

Suggested endpoints:

```text
POST /auth/email-verifications
  request:  { email, purpose: "register", identifier? }
  response: generic success if accepted for delivery, no account/email enumeration

POST /auth/register
  request:  { identifier, password, email, email_verification_code, display_name?, ... }
  response: existing AuthResp on success
```

Alternative acceptable shape: issue a short-lived verification ticket after code verification and require that ticket on register. If used, the ticket must be opaque, short-lived, one-time, and stored/validated server-side.

### Data model

Use a dedicated auth-owned table/repository, for example:

```text
email_verification_tokens
  id
  purpose
  email_normalized
  identifier_normalized nullable
  code_hash
  salt or keyed-hash metadata
  expires_at
  consumed_at nullable
  attempt_count
  send_count or last_sent_at
  requested_ip_hash nullable
  created_at
  updated_at
```

Rules:

- Store only hashes of verification codes; never store plaintext codes.
- TTL: 10-15 minutes.
- Limit verification attempts per code.
- Consume atomically during registration.
- Keep uniqueness/enumeration behavior generic.
- Use `db/migrations/*.sql` for schema changes.

### Rate limiting

Minimum dimensions:

- Per normalized email.
- Per IP or IP hash.
- Per identifier/account candidate.
- Per purpose.

Redis is suitable for short-window rate limits if configured; if Redis is unavailable, fail closed for sending new codes instead of silently bypassing rate limits. PostgreSQL can store durable token/attempt state.

### Mailer behavior

- Missing provider/model SMTP config must fail visibly.
- No mock/fake sender in production path.
- Fake sender is allowed only in tests and explicit local/dev fixtures.
- Do not log verification codes, SMTP credentials, Authorization headers, cookies, DSNs, provider tokens, DKIM private keys, or presigned signatures.

### Frontend UX

- Registration form requires email and verification code.
- Provide Chinese messages for sending, retry countdown, wrong code, expired code, too many attempts, and mailer unavailable.
- Do not persist passwords or verification codes across refresh.

## Issue Dependency Outcome

Issue #10 can be closed after this research branch is merged into `develop` and controller-side verification passes.

Issue #11 remains blocked until a provider/relay is selected and credentials/DNS access are available outside Git. If no provider/DNS credentials are available to the unattended agent, #11 should be treated as blocked rather than faking deployment.

Issue #12 remains blocked until #11 has a verified SMTP/relay endpoint and secret contract.
