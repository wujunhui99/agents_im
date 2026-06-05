#!/usr/bin/env bash
# Secrets, key material, shell-exec, and auth-secret-leak gates.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"
cd "$(git rev-parse --show-toplevel)"

# No real-looking provider API keys / key assignments in tracked files.
if git grep -n -I -E 'sk-[A-Za-z0-9_-]{20,}' -- . ':!docs/references' ':!.ai-context' ':!web/node_modules'; then
  echo "tracked files must not contain real-looking provider API keys (sk-...)" >&2
  exit 1
fi
if git grep -n -I -E "(DEEPSEEK_API_KEY|OPENAI_API_KEY|ANTHROPIC_API_KEY)[[:space:]]*[:=][[:space:]]*['\"]?sk-[A-Za-z0-9_-]{8,}" -- . ':!docs/references' ':!.ai-context' ':!web/node_modules'; then
  echo "tracked files must not contain real provider API key assignments" >&2
  exit 1
fi

# db/change_log SQL must not carry secrets, DSNs, tokens, or private keys.
# (Source of truth for schema is db/migrations/*.sql; change_log is Markdown/audit
# notes per AGENTS.md — no executable-SQL mandate here.)
if find db/change_log -maxdepth 1 -type f -name '*.sql' -print0 | xargs -0 -r rg -n "(postgres://|mysql://|password=|passwd=|token=|secret=|AKIA|BEGIN RSA PRIVATE KEY|BEGIN OPENSSH PRIVATE KEY)"; then
  echo "db/change_log SQL must not contain secrets, DSNs, tokens, or private keys" >&2
  exit 1
fi

# Production Go code must not directly execute shell/python commands.
forbid_match "production Go code must not directly execute shell or python commands" \
  -n '"os/exec"|exec\.Command|CommandContext\(|"(/bin/bash|/bin/sh|bash|sh|python|python3)"' \
  service/gateway-ws service/message-api service/message-transfer internal --glob '*.go' --glob '!*_test.go'

# Header-based current-user auth is forbidden; tests must use Bearer JWT or an explicit reject helper.
forbid_match "production API/code still contains header-based current user auth" \
  -n "X-User-Id|CurrentUserID|currentUserID" api internal service/gateway-ws service/message-api service/message-transfer

legacy_x_user_id_sets="$(rg -n 'Header\.Set\("X-User-Id"' tests internal || true)"
if [[ -n "$legacy_x_user_id_sets" ]]; then
  disallowed_legacy_x_user_id_sets="$(printf '%s\n' "$legacy_x_user_id_sets" | rg -v 'legacy X-User-Id rejection helper' || true)"
  if [[ -n "$disallowed_legacy_x_user_id_sets" ]]; then
    printf '%s\n' "$disallowed_legacy_x_user_id_sets" >&2
    echo "legacy X-User-Id header writes in tests/internal must use Authorization Bearer JWT or an explicit rejection helper/comment" >&2
    exit 1
  fi
fi

# Observability helpers must not log/inspect secrets, auth headers, bodies, or query strings.
forbid_match "observability helpers must not log or inspect secrets, auth headers, bodies, or query strings" \
  -n "RequestURI|RawQuery|DumpRequest|Authorization|password|token" pkg/observability

# Forbidden auth-secret fields must not leak into service / repository / message-contract source.
forbid_match "forbidden auth secret field found in service source" \
  -n "password|password_hash|verification_code|oauth_token|credential" \
  service/user/api/user.api service/user/rpc/user.proto service/user/api/user.go \
  internal/logic internal/handler service/user/rpc internal/servicecontext

forbid_match "forbidden auth secret field found in repository source" \
  -n "password|password_hash|verification_code|oauth_token|credential" \
  internal/repository \
  --glob '!postgres_account_profiles_test.go'

forbid_match "forbidden auth secret field found in message contract source" \
  -n "password|password_hash|verification_code|oauth_token|credential" \
  api/message.api internal/rpcgen/message/message.proto \
  internal/logic/messagelogic.go internal/repository/message_memory.go \
  internal/repository/message_repository.go internal/handler/message
