#!/usr/bin/env bash
# go-zero layering boundaries: no legacy compat layers, no cross-layer imports,
# api owns no data access, rpc mains own no business wiring, generated scaffold present.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"
cd "$(git rev-parse --show-toplevel)"

# Old handwritten REST/RPC compatibility layers must be gone.
forbid_paths "old handwritten compatibility layer still exists" \
  internal/handler/handler.go \
  internal/handler/user_handler.go \
  internal/handler/friends_handler.go \
  internal/handler/groups_handler.go \
  internal/handler/message_handler.go \
  internal/auth/handler/handler.go \
  internal/rpc/user_server.go \
  internal/rpc/friends_server.go \
  internal/rpc/groups_server.go \
  internal/auth/rpc/auth_server.go

if [[ -d internal/rpc || -d internal/auth/rpc ]]; then
  echo "old rpc compatibility directory still exists" >&2
  exit 1
fi
if [[ -d internal/svc ]]; then
  echo "legacy root internal/svc package must not exist; use focused internal/servicecontext/<service> packages" >&2
  exit 1
fi
if [[ -d internal/auth/svc ]]; then
  echo "auth-api must use focused internal/servicecontext/auth, not the old internal/auth/svc compatibility context" >&2
  exit 1
fi

aggregate_gozero_logic_files="$(find internal/logic internal/auth/logic -path '*/gozero_logic.go' -type f -print || true)"
if [[ -n "${aggregate_gozero_logic_files}" ]]; then
  echo "go-zero REST adapter logic must use goctl-style per-operation *_logic.go files, not aggregate gozero_logic.go files:" >&2
  echo "${aggregate_gozero_logic_files}" >&2
  exit 1
fi

root_svc_import_files="$(rg -l '"github.com/wujunhui99/agents_im/internal/svc"' service/gateway-ws service/message-api service/message-transfer internal/handler internal/logic internal/gateway tests --glob '*.go' || true)"
if [[ -n "${root_svc_import_files}" ]]; then
  echo "core REST, gateway, and tests must not import legacy root internal/svc:" >&2
  echo "${root_svc_import_files}" >&2
  exit 1
fi

# Generated goctl rpc scaffold must exist for every rpc service.
rpc_generated_dirs=(
  "service/user/rpc"
  "service/auth/rpc"
  "service/friends/rpc"
  "service/groups/rpc"
  "internal/rpcgen/message"
  "service/msg/rpc"
  "service/third/rpc"
)
for dir in "${rpc_generated_dirs[@]}"; do
  if [[ ! -d "$dir/internal/server" || ! -d "$dir/internal/logic" || ! -d "$dir/internal/svc" ]]; then
    echo "missing goctl rpc generated scaffold under: $dir" >&2
    exit 1
  fi
done

# rpc mains must not re-import the old handwritten rpc wrapper.
for main_file in service/user/rpc/user.go service/auth/rpc/auth.go service/friends/rpc/friends.go service/groups/rpc/groups.go service/third/rpc/third.go internal/rpcgen/message/message.go; do
  if rg -n '"github.com/wujunhui99/agents_im/internal/(auth/)?rpc"' "$main_file"; then
    echo "rpc entrypoint imports old handwritten rpc wrapper: $main_file" >&2
    exit 1
  fi
done

# rpc mains must not own business wiring (deps stay behind generated rpc service contexts).
forbid_match "rpc service mains must not own business wiring; keep dependencies behind generated rpc service contexts" \
  -n '"github.com/wujunhui99/agents_im/internal/(logic|repository|auth/logic|auth/repository)"' \
  internal/rpcgen/message/message.go service/user/rpc/user.go service/auth/rpc/auth.go service/friends/rpc/friends.go service/groups/rpc/groups.go service/third/rpc/third.go

# api services own no data access — they call rpc/BFF instead.
for api_spec in \
  "service/user/api:(repository|model|objectstorage|servicecontext/user)" \
  "service/auth/api:(repository|model|objectstorage|auth/repository|servicecontext/auth)" \
  "service/groups/api:(repository|model|objectstorage|servicecontext/groups)" \
  "service/friends/api:(repository|model|objectstorage|servicecontext/friends)"; do
  api_dir="${api_spec%%:*}"
  api_pkgs="${api_spec##*:}"
  forbid_match "${api_dir} must not own data access; use RPC/BFF calls" \
    -n "\"github.com/wujunhui99/agents_im/internal/${api_pkgs}\"|DataSource|StorageDriver|New.*Repository" \
    "${api_dir}" --glob '*.go' --glob '!*_test.go'
done

# Generated api/rpc logic must not still contain empty scaffold behavior.
for logic_dir in \
  service/user/api/internal/logic \
  service/auth/api/internal/logic \
  service/friends/api/internal/logic \
  service/agent/api/internal/logic; do
  forbid_match "generated API logic still contains empty scaffold behavior: ${logic_dir}" \
    -n "todo: add your logic here|return &.*Response\{\}, nil" "${logic_dir}"
done
forbid_match "generated rpc logic still contains empty scaffold behavior" \
  -n "todo: add your logic here|return &.*Response\{\}, nil" \
  internal/rpcgen/*/internal/logic service/user/rpc/internal/logic service/auth/rpc/internal/logic service/third/rpc/internal/logic

# rpc logic must be wired through svcCtx markers (logic talks to model/provider via svcCtx).
rpc_logic_markers=(
  "service/user/rpc/internal/logic:UserLogic"
  "service/auth/rpc/internal/logic:AuthLogic"
  "service/friends/rpc/internal/logic:FriendshipModel"
  "internal/rpcgen/message/internal/logic:MessageLogic"
  "service/msg/rpc/internal/logic:Messages"
  "service/third/rpc/internal/logic:MailProvider"
)
for logic_spec in "${rpc_logic_markers[@]}"; do
  dir="${logic_spec%%:*}"
  marker="${logic_spec##*:}"
  rg -q "svcCtx\.${marker}" "$dir"
done

# Tests must not use old REST mux registration helpers.
forbid_match "tests still use old REST mux registration helpers" \
  -n "RegisterHandlers|RegisterUserHandlers|RegisterFriendsHandlers|RegisterGroupsHandlers|RegisterMessageHandlers|authhandler\.RegisterHandlers" tests
