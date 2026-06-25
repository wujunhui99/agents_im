#!/usr/bin/env bash
# Frontend (web/) gates: design tokens, API client wiring, Vite proxy order,
# dependency guardrails, account_type union, localized shell, and no mock flows.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"
cd "$(git rev-parse --show-toplevel)"

# Design tokens + API client wiring.
assert_present "-qF" web/src/styles/tokens.css -- \
  "--md-sys-color-primary" "--md-sys-color-surface-container" "--md-shape-corner-small" \
  "--md-space-4" "--md-elevation-level1" "--md-state-hover-opacity"
rg -qF '@import "./styles/tokens.css";' web/src/styles.css
rg -qF "createApiClient" web/src/App.tsx web/src/api/client.ts

# Vite /messages proxy must precede /me (prefix-based matching).
messages_proxy_line="$(rg -n "'/messages':" web/vite.config.ts | head -n1 | cut -d: -f1)"
me_proxy_line="$(rg -n "'/me':" web/vite.config.ts | head -n1 | cut -d: -f1)"
if [[ -z "${messages_proxy_line}" || -z "${me_proxy_line}" || "${messages_proxy_line}" -ge "${me_proxy_line}" ]]; then
  echo "Vite /messages proxy must be declared before /me; Vite proxy matching is prefix-based" >&2
  exit 1
fi

# No heavy Material Web / MUI dependencies.
forbid_match "frontend must not introduce Material Web or MUI heavy dependencies" \
  -n "(@material/web|@mui/)" web/package.json web/package-lock.json web/src

# Message UI must not sort confirmed messages by sendTime (authoritative seq only).
forbid_match "message UI must not sort confirmed messages by sendTime" \
  -n "sort\([^\n]*sendTime|sendTime - .*sendTime|sendTime.* - .*sendTime" web/src/features/messages --glob '*.ts' --glob '*.tsx'

# account_type union must be user|agent|admin (no legacy "normal").
rg -qF "account_type?: 'user' | 'agent' | 'admin'" web/src/api/user.ts
forbid_match "account_type docs/frontend must use user|agent|admin; normal may appear only in explicit migration compatibility docs" \
  -n 'account_type.*normal|normal.*account_type|`normal`' web/src/api/user.ts

# Localized WeChat-style shell strings across the core frontend surfaces.
# styles/ is searched as a directory: #656 split styles.css into 13 modules and
# moved the wechat-green token to styles/base.css, so target the dir (not a single
# file) to survive future module reshuffles.
assert_present "-qF" \
  web/src/App.tsx web/src/components/ui/NavigationBar.tsx web/src/components/ui/TabBar.tsx \
  web/src/components/ContactsPage.tsx web/src/features/messages/MessagesPage.tsx \
  web/src/pages/DiscoverPage.tsx web/src/styles -- \
  "消息" "联系人" "发现" "我的" 'role="tab"' "wechat-green"

# No production mock message flow.
forbid_match "frontend production mock flow found" \
  -q "mockData|mockConversations|mode=\"mock\"|sendMessageWithMock|cloneMockConversations" web/src --glob "*.ts" --glob "*.tsx"
