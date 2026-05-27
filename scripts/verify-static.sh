#!/usr/bin/env bash
set -euo pipefail

if git grep -n -I -E 'sk-[A-Za-z0-9_-]{20,}' -- . ':!docs/references' ':!.ai-context' ':!web/node_modules'; then
  echo "tracked files must not contain real-looking provider API keys (sk-...)" >&2
  exit 1
fi

if git grep -n -I -E "(DEEPSEEK_API_KEY|OPENAI_API_KEY|ANTHROPIC_API_KEY)[[:space:]]*[:=][[:space:]]*['\"]?sk-[A-Za-z0-9_-]{8,}" -- . ':!docs/references' ':!.ai-context' ':!web/node_modules'; then
  echo "tracked files must not contain real provider API key assignments" >&2
  exit 1
fi

required_files=(
  "api/user.api"
  "api/auth.api"
  "api/friends.api"
  "api/groups.api"
  "api/message.api"
  "api/media.api"
  ".drone.yml"
  ".github/markdown-link-check.json"
  ".ai-context/zero-skills/SKILL.md"
  ".ai-context/zero-skills/references/goctl-commands.md"
  ".ai-context/zero-skills/references/rest-api-patterns.md"
  ".ai-context/zero-skills/references/rpc-patterns.md"
  ".ai-context/zero-skills/references/database-patterns.md"
  "docs/references/go-zero/codex-guide.md"
  "docs/exec-plans/active/goctl-refactor.md"
  "docs/exec-plans/active/ci-pipeline.md"
  "service/user/rpc/user.proto"
  "service/user/api/user.api"
  "proto/auth.proto"
  "proto/friends.proto"
  "proto/groups.proto"
  "proto/message.proto"
  "proto/mail.proto"
  "proto/userpb/user.pb.go"
  "proto/userpb/user_grpc.pb.go"
  "proto/authpb/auth.pb.go"
  "proto/authpb/auth_grpc.pb.go"
  "proto/friendspb/friends.pb.go"
  "proto/friendspb/friends_grpc.pb.go"
  "proto/groupspb/groups.pb.go"
  "proto/groupspb/groups_grpc.pb.go"
  "proto/messagepb/message.pb.go"
  "proto/messagepb/message_grpc.pb.go"
  "proto/mailpb/mail.pb.go"
  "proto/mailpb/mail_grpc.pb.go"
  "service/user/rpc/user.v1.go"
  "service/user/rpc/internal/server/user_service_server.go"
  "service/user/api/user.go"
  "service/user/api/entry/entry.go"
  "service/user/api/internal/config/config.go"
  "service/user/api/internal/handler/routes.go"
  "service/user/api/internal/svc/service_context.go"
  "service/user/api/internal/types/types.go"
  "service/user/api/internal/logic/user/create_user_logic.go"
  "service/user/api/internal/logic/user/create_account_logic.go"
  "service/user/api/internal/logic/user/exists_user_logic.go"
  "service/user/api/internal/logic/user/exists_account_logic.go"
  "service/user/api/internal/logic/user/get_me_logic.go"
  "service/user/api/internal/logic/user/get_user_by_identifier_logic.go"
  "service/user/api/internal/logic/user/get_account_by_identifier_logic.go"
  "service/user/api/internal/logic/user/update_me_logic.go"
  "service/user/api/internal/logic/user/update_me_avatar_logic.go"
  "service/user/api/internal/logic/user/convert.go"
  "internal/rpcgen/auth/auth.v1.go"
  "internal/rpcgen/auth/internal/server/auth_service_server.go"
  "internal/rpcgen/friends/friends.v1.go"
  "internal/rpcgen/friends/internal/server/friends_service_server.go"
  "internal/rpcgen/groups/groups.v1.go"
  "internal/rpcgen/groups/internal/server/groups_service_server.go"
  "internal/rpcgen/message/message.go"
  "internal/rpcgen/message/internal/server/message_service_server.go"
  "internal/rpcgen/mail/mail.v1.go"
  "internal/rpcgen/mail/internal/server/mail_service_server.go"
  "cmd/user-api/main.go"
  "cmd/user-rpc/main.go"
  "cmd/auth-api/main.go"
  "cmd/auth-rpc/main.go"
  "cmd/friends-api/main.go"
  "cmd/friends-rpc/main.go"
  "cmd/groups-api/main.go"
  "cmd/groups-rpc/main.go"
  "cmd/message-api/main.go"
  "cmd/message-rpc/main.go"
  "cmd/mail-rpc/main.go"
  "cmd/gateway-ws/main.go"
  "cmd/message-transfer/main.go"
  "etc/gateway-ws.yaml"
  "etc/message-transfer.yaml"
  "etc/message-rpc.yaml"
  "etc/mail-rpc.yaml"
  "internal/mail/provider.go"
  "internal/mail/config.go"
  "internal/mail/tencent_ses.go"
  "internal/logic/userlogic.go"
  "internal/logic/friendslogic.go"
  "internal/logic/groupslogic.go"
  "internal/logic/messagelogic.go"
  "internal/logic/medialogic_test.go"
  "internal/logic/message_media_test.go"
  "internal/logic/user/create_user_logic.go"
  "internal/logic/user/create_account_logic.go"
  "internal/logic/user/exists_user_logic.go"
  "internal/logic/user/exists_account_logic.go"
  "internal/logic/user/get_me_logic.go"
  "internal/logic/user/get_user_by_identifier_logic.go"
  "internal/logic/user/get_account_by_identifier_logic.go"
  "internal/logic/user/update_me_logic.go"
  "internal/logic/user/update_me_avatar_logic.go"
  "internal/logic/user/convert.go"
  "internal/logic/friends/add_friend_logic.go"
  "internal/logic/friends/delete_friend_logic.go"
  "internal/logic/friends/get_friendship_logic.go"
  "internal/logic/friends/list_friends_logic.go"
  "internal/logic/friends/list_friend_requests_logic.go"
  "internal/logic/friends/accept_friend_request_logic.go"
  "internal/logic/friends/reject_friend_request_logic.go"
  "internal/logic/friends/convert.go"
  "internal/logic/groups/create_group_logic.go"
  "internal/logic/groups/list_groups_logic.go"
  "internal/logic/groups/get_group_logic.go"
  "internal/logic/groups/update_group_logic.go"
  "internal/logic/groups/add_member_logic.go"
  "internal/logic/groups/leave_group_logic.go"
  "internal/logic/groups/kick_member_logic.go"
  "internal/logic/groups/list_members_logic.go"
  "internal/logic/groups/convert.go"
  "internal/logic/message/send_message_logic.go"
  "internal/logic/message/pull_messages_logic.go"
  "internal/logic/message/get_conversation_seqs_logic.go"
  "internal/logic/message/mark_conversation_as_read_logic.go"
  "internal/logic/message/get_conversation_a_i_hosting_logic.go"
  "internal/logic/message/update_conversation_a_i_hosting_logic.go"
  "internal/logic/message/convert.go"
  "internal/logic/media/create_upload_intent_logic.go"
  "internal/logic/media/complete_upload_logic.go"
  "internal/logic/media/get_download_u_r_l_logic.go"
  "internal/logic/media/get_avatar_logic.go"
  "internal/logic/media/convert.go"
  "internal/logic/agent/create_agent_logic.go"
  "internal/logic/agent/get_agent_logic.go"
  "internal/logic/agent/list_agents_logic.go"
  "internal/logic/agent/update_agent_logic.go"
  "internal/logic/agent/update_agent_status_logic.go"
  "internal/logic/agent/delete_agent_logic.go"
  "internal/logic/agent/convert.go"
  "internal/logic/medialogic.go"
  "internal/model/friendship.go"
  "internal/model/group.go"
  "internal/model/media.go"
  "internal/objectstorage/store.go"
  "internal/objectstorage/minio.go"
  "internal/objectstorage/memory.go"
  "internal/repository/memory.go"
  "internal/repository/postgres_common.go"
  "internal/repository/postgres_user_friends.go"
  "internal/repository/groups_memory.go"
  "internal/repository/groups_repository.go"
  "internal/repository/postgres_groups.go"
  "internal/repository/message_memory.go"
  "internal/repository/message_repository.go"
  "internal/repository/media_repository.go"
  "internal/repository/media_memory.go"
  "internal/repository/postgres_media.go"
  "internal/repository/message_outbox_repository.go"
  "internal/repository/delivery_attempt_repository.go"
  "internal/repository/postgres_message.go"
  "internal/repository/postgres_outbox.go"
  "internal/handler/health_handler.go"
  "internal/handler/gozero_routes.go"
  "internal/handler/media/create_upload_intent_handler.go"
  "internal/handler/media/complete_upload_handler.go"
  "internal/handler/media/get_download_u_r_l_handler.go"
  "internal/handler/media/get_avatar_handler.go"
  "internal/handler/message/get_conversation_a_i_hosting_handler.go"
  "internal/handler/message/update_conversation_a_i_hosting_handler.go"
  "internal/handler/user/create_account_handler.go"
  "internal/handler/user/exists_account_handler.go"
  "internal/handler/user/get_account_by_identifier_handler.go"
 "internal/handler/user/update_me_avatar_handler.go"
  "internal/servicecontext/common/auth.go"
  "internal/servicecontext/user/service_context.go"
  "internal/servicecontext/auth/service_context.go"
  "internal/servicecontext/friends/service_context.go"
  "internal/servicecontext/groups/service_context.go"
  "internal/servicecontext/message/service_context.go"
  "internal/servicecontext/agent/service_context.go"
  "internal/servicecontext/gateway/service_context.go"
  "internal/health/health.go"
  "internal/health/health_test.go"
  "internal/observability/metrics.go"
  "internal/observability/metrics_test.go"
  "internal/observability/trace.go"
  "internal/observability/trace_test.go"
  "internal/llmobs/types.go"
  "internal/llmobs/sink.go"
  "internal/llmobs/sink_test.go"
  "internal/llmobs/sanitize.go"
  "internal/llmobs/eino.go"
  "internal/agenteval/eval.go"
  "internal/agenteval/python_go_test.go"
  "internal/types/types.go"
  "internal/auth/logic/authlogic.go"
  "internal/logic/auth/login_logic.go"
  "internal/logic/auth/register_logic.go"
  "internal/logic/auth/validate_token_logic.go"
  "internal/logic/auth/convert.go"
  "internal/auth/repository/memory.go"
  "internal/auth/repository/postgres.go"
  "internal/handler/auth/login_handler.go"
  "internal/handler/auth/register_handler.go"
  "internal/handler/auth/validate_token_handler.go"
  "internal/auth/token/token.go"
  "internal/auth/useradapter/user_client.go"
  "internal/ctxuser/user.go"
  "service/user/rpc/entry/entry.go"
  "internal/rpcgen/auth/entry/entry.go"
  "internal/rpcgen/friends/entry/entry.go"
  "internal/rpcgen/groups/entry/entry.go"
  "internal/rpcgen/message/entry/entry.go"
  "internal/rpcgen/rpcerror/error.go"
  "internal/gateway/contract.go"
  "internal/transfer/event.go"
  "internal/transfer/interfaces.go"
  "internal/transfer/idempotency.go"
  "internal/transfer/delivery_attempt_recorder.go"
  "internal/transfer/memory.go"
  "internal/transfer/worker.go"
  "internal/transfer/worker_test.go"
  "internal/transfer/kafka_consumer.go"
  "internal/transfer/kafka_consumer_test.go"
  "internal/transfer/kafka_integration_test.go"
  "internal/presence/store.go"
  "internal/presence/memory.go"
  "internal/presence/redis.go"
  "internal/presence/memory_test.go"
  "internal/presence/redis_integration_test.go"
  "internal/gateway/ws/connection_manager.go"
  "internal/gateway/ws/server.go"
  "internal/messaging/event.go"
  "internal/messaging/producer.go"
  "internal/messaging/kafka.go"
  "internal/messaging/event_test.go"
  "internal/messaging/producer_test.go"
  "internal/messaging/kafka_integration_test.go"
  "internal/gateway/delivery/delivery.go"
  "internal/gateway/ws/delivery.go"
  "internal/transfer/gateway/dispatcher.go"
  "internal/transfer/gateway/dispatcher_test.go"
  "internal/domain/readreceipt/read_receipt.go"
  "internal/agent/pythonexec/executor.go"
  "internal/agent/pythonexec/executor_test.go"
  "tests/user_service_test.go"
  "tests/auth_service_test.go"
  "tests/friends_service_test.go"
  "tests/groups_service_test.go"
  "tests/message_service_test.go"
  "tests/gateway_contract_test.go"
  "tests/websocket_gateway_test.go"
  "tests/read_receipts_test.go"
  "tests/no_shell_execution_test.go"
  "docs/product-specs/agent-system.md"
  "docs/design-docs/agent-system-architecture.md"
  "docs/product-specs/user-service.md"
  "docs/product-specs/auth-service.md"
  "docs/product-specs/friends-service.md"
  "docs/product-specs/groups-service.md"
  "docs/product-specs/message-chain.md"
  "docs/product-specs/message-storage.md"
  "docs/product-specs/gateway-message-contract.md"
  "docs/product-specs/frontend-sync-contract.md"
  "docs/product-specs/read-receipts.md"
  "docs/design-docs/user-service-go-zero.md"
  "docs/design-docs/rest-service-context-boundaries.md"
  "docs/design-docs/account-service-terminology.md"
  "docs/design-docs/auth-service-go-zero.md"
  "docs/design-docs/friends-service-go-zero.md"
  "docs/design-docs/groups-service-go-zero.md"
  "docs/design-docs/message-chain-contract.md"
  "docs/design-docs/message-storage.md"
  "docs/design-docs/jwt-auth-middleware.md"
  "docs/design-docs/postgres-persistence.md"
  "docs/design-docs/message-outbox.md"
  "docs/design-docs/gateway-message-contract.md"
  "docs/design-docs/redis-presence.md"
  "docs/design-docs/kafka-message-events.md"
  "docs/design-docs/websocket-gateway.md"
  "docs/design-docs/websocket-reconnect-sync.md"
  "docs/design-docs/message-transfer-worker.md"
  "docs/design-docs/kafka-transfer-consumer.md"
  "docs/design-docs/gateway-push-delivery.md"
  "docs/design-docs/transfer-gateway-dispatcher.md"
  "docs/design-docs/message-delivery-reliability.md"
  "docs/design-docs/gateway-presence-routing.md"
  "docs/exec-plans/active/backend-mvp-completion.md"
  "docs/design-docs/backend-mvp-contract.md"
  "docs/design-docs/observability-mvp.md"
  "docs/design-docs/llm-observability.md"
  "docs/product-specs/backend-mvp.md"
  "docs/product-specs/frontend-backend-contract.md"
  "docs/DEVELOPMENT.md"
  "docs/FRONTEND.md"
  "web/package.json"
  "web/index.html"
  "web/src/App.tsx"
  "web/src/App.test.tsx"
  "web/src/api/user.test.ts"
  "web/src/api/user.ts"
  "web/src/components/ui/ActionRow.tsx"
  "web/src/components/ui/Avatar.tsx"
  "web/src/components/ui/Badge.tsx"
  "web/src/components/ui/Button.tsx"
  "web/src/components/ui/Card.tsx"
  "web/src/components/ui/ListCard.tsx"
  "web/src/components/ui/ListItem.tsx"
  "web/src/components/ui/MessageBubble.tsx"
  "web/src/components/ui/NavigationBar.tsx"
  "web/src/components/ui/SearchBox.tsx"
  "web/src/components/ui/TabBar.tsx"
  "web/src/components/ui/TextField.tsx"
  "web/src/components/ui/TopBar.tsx"
  "web/src/components/ui/TopAppBar.tsx"
  "web/src/main.tsx"
  "web/src/components/ContactsPage.tsx"
  "web/src/pages/DiscoverPage.tsx"
  "web/src/pages/MePage.tsx"
  "web/src/features/messages/MessagesPage.tsx"
  "web/src/styles/tokens.css"
  "web/src/styles.css"
  "web/src/vite-env.d.ts"
  "docs/design-docs/read-receipts.md"
  "docs/exec-plans/completed/user-service-go-zero.md"
  "docs/exec-plans/completed/auth-service-go-zero.md"
  "docs/exec-plans/completed/friends-service-go-zero.md"
  "docs/exec-plans/completed/groups-service-go-zero.md"
  "docs/exec-plans/completed/message-storage.md"
  "docs/exec-plans/active/message-outbox.md"
  "internal/repository/message_storage_contract.go"
  "docker-compose.yml"
  ".env.example"
  "db/migrations/001_init_postgres.sql"
  "scripts/migrate-postgres.sh"
  "scripts/verify-postgres-local.sh"
  "db/change_log/README.md"
  "db/change_log/template.sql"
  "db/change_log/template.md"
  "scripts/dev-up.sh"
  "scripts/dev-demo-data.sh"
  "tests/postgres_persistence_integration_test.go"
  "tests/mvp_backend_test.go"
  "docs/exec-plans/completed/gateway-message-contract.md"
  "docs/exec-plans/active/redis-presence.md"
  "docs/exec-plans/active/read-receipts.md"
  "docs/exec-plans/completed/remove-handwritten-compat.md"
  "docs/exec-plans/active/jwt-auth-middleware.md"
  "docs/exec-plans/completed/websocket-gateway.md"
  "docs/exec-plans/active/kafka-redpanda-compose.md"
  "docs/exec-plans/active/message-transfer-worker.md"
  "docs/exec-plans/active/kafka-transfer-consumer.md"
  "docs/exec-plans/completed/gateway-push-delivery.md"
  "docs/exec-plans/completed/transfer-gateway-dispatcher.md"
  "docs/exec-plans/completed/gateway-presence-routing.md"
  "docs/exec-plans/completed/account-service-terminology.md"
  "docs/exec-plans/active/agent-system-v0.md"
  "docs/exec-plans/active/agent-infrastructure-parallel-baseline.md"
  "docs/design-docs/agent-conversation-hosting.md"
  "docs/exec-plans/completed/agent-conversation-hosting.md"
  "db/migrations/003_agent_conversation_hosting.sql"
  "internal/agentim/hosting.go"
  "internal/agentim/hosting_test.go"
  "internal/agentim/llm_observability.go"
  "internal/agentim/llm_observability_test.go"
  "internal/repository/agent_hosting_repository.go"
  "internal/repository/agent_hosting_memory.go"
  "internal/repository/postgres_agent_hosting.go"
)

for file in "${required_files[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "missing required file: $file" >&2
    exit 1
  fi
done

shell_scripts=(
  "scripts/migrate-postgres.sh"
  "scripts/verify-postgres-local.sh"
  "scripts/dev-up.sh"
  "scripts/dev-demo-data.sh"
  "scripts/deploy-k3s.sh"
  "scripts/bootstrap-server.sh"
  "scripts/test-deploy-k3s.sh"
  "scripts/test-no-latest-images.sh"
  "scripts/verify-static.sh"
)

for script in "${shell_scripts[@]}"; do
  bash -n "$script"
done

frontend_material_doc_patterns=(
  "Material 3-inspired 轻量设计系统"
  "web/src/styles/tokens.css"
  "Button"
  "Card"
  "TextField"
  "TopAppBar"
  "NavigationBar"
  "ListItem"
  "MessageBubble"
  "消息 / 联系人 / 发现 / 我的"
  '不依赖 `@material/web`、`@mui/*`'
)

for pattern in "${frontend_material_doc_patterns[@]}"; do
  rg -qF "$pattern" docs/FRONTEND.md
done

frontend_material_token_patterns=(
  "--md-sys-color-primary"
  "--md-sys-color-surface-container"
  "--md-shape-corner-small"
  "--md-space-4"
  "--md-elevation-level1"
  "--md-state-hover-opacity"
)

for pattern in "${frontend_material_token_patterns[@]}"; do
  rg -qF -- "$pattern" web/src/styles/tokens.css
done

rg -qF '@import "./styles/tokens.css";' web/src/styles.css
rg -qF "createApiClient" web/src/App.tsx web/src/api/client.ts
rg -qF "POST /messages" docs/FRONTEND.md docs/product-specs/frontend-backend-contract.md
messages_proxy_line="$(rg -n "'/messages':" web/vite.config.ts | head -n1 | cut -d: -f1)"
me_proxy_line="$(rg -n "'/me':" web/vite.config.ts | head -n1 | cut -d: -f1)"
if [[ -z "${messages_proxy_line}" || -z "${me_proxy_line}" || "${messages_proxy_line}" -ge "${me_proxy_line}" ]]; then
  echo "Vite /messages proxy must be declared before /me; Vite proxy matching is prefix-based" >&2
  exit 1
fi

if rg -n "(@material/web|@mui/)" web/package.json web/package-lock.json web/src; then
  echo "frontend must not introduce Material Web or MUI heavy dependencies" >&2
  exit 1
fi

drone_workflow_patterns=(
  "kind: pipeline"
  "backend-verification"
  "postgres-integration"
  "deploy-main"
  "bash scripts/ci/drone-backend-verify.sh"
  "bash scripts/ci/drone-postgres-integration.sh"
  "bash scripts/ci/drone-detect-deploy.sh"
  "bash scripts/ci/drone-build-images.sh"
  "bash scripts/ci/drone-deploy.sh"
  "from_secret: ghcr_token"
  "postgres:16-alpine"
  "ghcr.io/wujunhui99/agents_im"
)

for pattern in "${drone_workflow_patterns[@]}"; do
  rg -qF "$pattern" .drone.yml
done

ci_doc_patterns=(
  "CI Pipeline"
  "go test ./..."
  "bash scripts/verify-static.sh"
  "docker compose config"
  "Drone"
  ".drone.yml"
  "markdown-link-check"
  "Codex commit 前验证门禁"
  "db/change_log/*.sql"
  "scripts/verify-postgres-local.sh"
)

for pattern in "${ci_doc_patterns[@]}"; do
  rg -qF "$pattern" docs/exec-plans/active/ci-pipeline.md docs/GIT_WORKFLOW.md
done


bash scripts/ci/verify-migration-immutability.sh

change_log_required_paths=(
  'db/migrations/'
  'db/schema/'
)

changed_files="$(git diff --name-only --diff-filter=ACMRTUXB HEAD -- || true)"
staged_files="$(git diff --cached --name-only --diff-filter=ACMRTUXB -- || true)"
untracked_files="$(git ls-files --others --exclude-standard || true)"
all_changed_files="${changed_files}"$'\n'"${staged_files}"$'\n'"${untracked_files}"

requires_change_log=0
while IFS= read -r changed_file; do
  [[ -z "${changed_file}" ]] && continue
  for prefix in "${change_log_required_paths[@]}"; do
    if [[ "${changed_file}" == "${prefix}"* ]]; then
      requires_change_log=1
    fi
  done
  if [[ "${changed_file}" =~ ^internal/repository/postgres_.*\.go$ || "${changed_file}" == "tests/postgres_persistence_integration_test.go" ]]; then
    requires_change_log=1
  fi
done <<< "${all_changed_files}"

if [[ "${requires_change_log}" == "1" ]]; then
  non_template_change_log_count=0
  for sql_file in db/change_log/*.sql; do
    [[ -e "${sql_file}" ]] || continue
    [[ "${sql_file}" == "db/change_log/template.sql" ]] && continue
    if git ls-files --error-unmatch "${sql_file}" >/dev/null 2>&1 || git diff --name-only --diff-filter=ACMRTUXB HEAD -- | grep -qxF "${sql_file}" || git diff --cached --name-only --diff-filter=ACMRTUXB -- | grep -qxF "${sql_file}" || git ls-files --others --exclude-standard -- "${sql_file}" | grep -qxF "${sql_file}"; then
      non_template_change_log_count=$((non_template_change_log_count + 1))
    fi
  done
  if [[ "${non_template_change_log_count}" -eq 0 ]]; then
    echo "database/schema changes require a non-template db/change_log/*.sql file" >&2
    exit 1
  fi
fi

if find db/change_log -maxdepth 1 -type f -name '*.sql' -print0 | xargs -0 -r rg -n "(postgres://|mysql://|password=|passwd=|token=|secret=|AKIA|BEGIN RSA PRIVATE KEY|BEGIN OPENSSH PRIVATE KEY)"; then
  echo "db/change_log SQL must not contain secrets, DSNs, tokens, or private keys" >&2
  exit 1
fi

removed_compat_paths=(
  "internal/handler/handler.go"
  "internal/handler/user_handler.go"
  "internal/handler/friends_handler.go"
  "internal/handler/groups_handler.go"
  "internal/handler/message_handler.go"
  "internal/auth/handler/handler.go"
  "internal/rpc/user_server.go"
  "internal/rpc/friends_server.go"
  "internal/rpc/groups_server.go"
  "internal/auth/rpc/auth_server.go"
)

for path in "${removed_compat_paths[@]}"; do
  if [[ -e "$path" ]]; then
    echo "old handwritten compatibility layer still exists: $path" >&2
    exit 1
  fi
done

if [[ -d internal/rpc || -d internal/auth/rpc ]]; then
  echo "old rpc compatibility directory still exists" >&2
  exit 1
fi

if [[ -d internal/svc ]]; then
  echo "legacy root internal/svc package must not exist; use focused internal/servicecontext/<service> packages" >&2
  exit 1
fi

aggregate_gozero_logic_files="$(find internal/logic internal/auth/logic -path '*/gozero_logic.go' -type f -print || true)"
if [[ -n "${aggregate_gozero_logic_files}" ]]; then
  echo "go-zero REST adapter logic must use goctl-style per-operation *_logic.go files, not aggregate gozero_logic.go files:" >&2
  echo "${aggregate_gozero_logic_files}" >&2
  exit 1
fi

if [[ -d internal/auth/svc ]]; then
  echo "auth-api must use focused internal/servicecontext/auth, not the old internal/auth/svc compatibility context" >&2
  exit 1
fi

root_svc_import_files="$(rg -l '"github.com/wujunhui99/agents_im/internal/svc"' cmd internal/handler internal/logic internal/gateway tests --glob '*.go' || true)"
if [[ -n "${root_svc_import_files}" ]]; then
  echo "core REST, gateway, and tests must not import legacy root internal/svc:" >&2
  echo "${root_svc_import_files}" >&2
  exit 1
fi

if rg -n '"os/exec"|exec\.Command|CommandContext\(|"(/bin/bash|/bin/sh|bash|sh|python|python3)"' cmd internal --glob '*.go' --glob '!*_test.go'; then
  echo "production Go code must not directly execute shell or python commands" >&2
  exit 1
fi

message_plan_file=""
for candidate in \
  "docs/exec-plans/active/message-service-contract.md" \
  "docs/exec-plans/completed/message-service-contract.md"; do
  if [[ -f "$candidate" ]]; then
    message_plan_file="$candidate"
    break
  fi
done
if [[ -z "$message_plan_file" ]]; then
  echo "missing required file: docs/exec-plans/active/message-service-contract.md or docs/exec-plans/completed/message-service-contract.md" >&2
  exit 1
fi

api_patterns=(
  "get /me"
  "patch /me"
  "post /users"
  "post /accounts"
  "get /users/exists"
  "get /accounts/exists"
  "get /users/:identifier"
  "get /accounts/:identifier"
)

for pattern in "${api_patterns[@]}"; do
  rg -q "$pattern" api/user.api
  rg -q "$pattern" service/user/api/user.api
done

auth_api_patterns=(
  "post /auth/register"
  "post /auth/login"
  "post /auth/validate"
)

for pattern in "${auth_api_patterns[@]}"; do
  rg -q "$pattern" api/auth.api
done

friends_api_patterns=(
  "post /friends"
  "delete /friends/:user_id"
  "get /friends"
  "get /friends/:user_id"
)

for pattern in "${friends_api_patterns[@]}"; do
  rg -q "$pattern" api/friends.api
done

groups_api_patterns=(
  "post /groups"
  "get /groups/:group_id"
  "post /groups/:group_id/members"
  "delete /groups/:group_id/members/me"
  "get /groups/:group_id/members"
)

for pattern in "${groups_api_patterns[@]}"; do
  rg -q "$pattern" api/groups.api
done

message_api_patterns=(
  "post /messages"
  "get /conversations/:conversation_id/messages"
  "get /conversations/seqs"
  "post /conversations/:conversation_id/read"
)

for pattern in "${message_api_patterns[@]}"; do
  rg -q "$pattern" api/message.api
done

message_ordering_schema_patterns=(
  "messages_conversation_seq_uniq"
  "messages_sender_client_msg_uniq"
  "conversation_threads"
)

for pattern in "${message_ordering_schema_patterns[@]}"; do
  rg -q "$pattern" db/migrations/001_init_postgres.sql
done

message_ordering_code_patterns=(
  "for update"
  "existingMessageForIdempotency"
  "orderedChatMessages"
  "conversationHasInFlightSend"
)

for pattern in "${message_ordering_code_patterns[@]}"; do
  rg -q "$pattern" internal/repository/postgres_message.go web/src/features/messages/MessagesPage.tsx
done

if rg -n "sort\\([^\\n]*sendTime|sendTime - .*sendTime|sendTime.* - .*sendTime" web/src/features/messages/MessagesPage.tsx; then
  echo "message UI must not sort confirmed messages by sendTime" >&2
  exit 1
fi

message_ordering_test_patterns=(
  "concurrent same conversation sends allocate contiguous seqs"
  "last message state follows max seq"
  "renders shuffled server messages by authoritative seq"
  "one in-flight send per conversation"
)

for pattern in "${message_ordering_test_patterns[@]}"; do
  rg -q "$pattern" internal/repository/message_repository_contract_test.go web/src/features/messages/MessagesPage.test.tsx
done

message_ordering_doc_patterns=(
  "conversation_id + seq"
  "network arrival order"
  "clientMsgId"
  "gap"
)

for pattern in "${message_ordering_doc_patterns[@]}"; do
  rg -qF "$pattern" docs/product-specs/message-chain.md docs/design-docs/message-chain-contract.md docs/product-specs/frontend-backend-contract.md docs/RELIABILITY.md
done

frontend_contract_patterns=(
  "/auth/register"
  "/auth/login"
  "/me"
  "/users/exists"
  "/accounts/exists"
  "/friends"
  "/groups"
  "/messages"
  "/media/uploads"
  "/me/avatar"
  "/ws"
  "send_message"
  "pull_messages"
  "get_conversation_seqs"
  "mark_conversation_read"
  "message_received"
  "message_delivered"
  "INVALID_ARGUMENT"
)

for pattern in "${frontend_contract_patterns[@]}"; do
  rg -qF "$pattern" docs/product-specs/frontend-backend-contract.md
done

development_doc_patterns=(
  "scripts/dev-up.sh"
  "scripts/dev-demo-data.sh"
  "docker compose up -d postgres redis redpanda minio"
  "bash scripts/migrate-postgres.sh"
  "go test ./..."
)

for pattern in "${development_doc_patterns[@]}"; do
  rg -qF "$pattern" docs/DEVELOPMENT.md
done

dev_script_patterns=(
  "docker compose up -d postgres redis redpanda minio"
  "bash scripts/migrate-postgres.sh"
  "StorageDriver: postgres"
  "ObjectStorage:"
  "gateway-ws"
)

for pattern in "${dev_script_patterns[@]}"; do
  rg -qF "$pattern" scripts/dev-up.sh
done

demo_data_patterns=(
  "/auth/register"
  "/friends"
  "/groups"
  "/messages"
  "/read"
)

for pattern in "${demo_data_patterns[@]}"; do
  rg -qF "$pattern" scripts/dev-demo-data.sh
done

mvp_test_patterns=(
  "TestMVPBackendAuthProfileSmoke"
  "TestMVPBackendFriendGroupMessageSmoke"
  "TestMVPBackendWebSocketSendPullMarkReadSmoke"
)

for pattern in "${mvp_test_patterns[@]}"; do
  rg -q "$pattern" tests/mvp_backend_test.go
done

proto_patterns=(
  "rpc CreateUser"
  "rpc GetUserByIdentifier"
  "rpc ExistsByIdentifier"
  "rpc GetUserByID"
  "rpc UpdateUserProfile"
  "rpc UpdateUserAvatar"
)

for pattern in "${proto_patterns[@]}"; do
  rg -q "$pattern" service/user/rpc/user.proto
done

auth_proto_patterns=(
  "rpc Register"
  "rpc Login"
  "rpc ValidateToken"
  "rpc ParseToken"
)

for pattern in "${auth_proto_patterns[@]}"; do
  rg -q "$pattern" proto/auth.proto
done

friends_proto_patterns=(
  "rpc AddFriend"
  "rpc DeleteFriend"
  "rpc ListFriends"
  "rpc GetFriendship"
)

for pattern in "${friends_proto_patterns[@]}"; do
  rg -q "$pattern" proto/friends.proto
done

groups_proto_patterns=(
  "rpc CreateGroup"
  "rpc GetGroup"
  "rpc AddMember"
  "rpc JoinGroup"
  "rpc LeaveGroup"
  "rpc ListMembers"
)

for pattern in "${groups_proto_patterns[@]}"; do
  rg -q "$pattern" proto/groups.proto
done

message_proto_patterns=(
  "service MessageService"
  "rpc SendMessage"
  "rpc PullMessages"
  "rpc GetConversationSeqs"
  "rpc MarkConversationAsRead"
  "message Message"
  "message ConversationSeqState"
)

for pattern in "${message_proto_patterns[@]}"; do
  rg -q "$pattern" proto/message.proto
done

mail_proto_patterns=(
  "service MailService"
  "rpc SendTemplateEmail"
  "repeated string recipients"
  "map<string, string> template_data"
  "string provider_request_id"
  "string provider_message_id"
)

for pattern in "${mail_proto_patterns[@]}"; do
  rg -q "$pattern" proto/mail.proto
done

agent_conversation_hosting_contract_patterns=(
  "message_origin"
  "agent_account_id"
  "trigger_server_msg_id"
  "agent_run_id"
  "allow_recursive_trigger"
)

for pattern in "${agent_conversation_hosting_contract_patterns[@]}"; do
  rg -q "$pattern" proto/message.proto db/migrations/003_agent_conversation_hosting.sql internal/repository/message_repository.go internal/messaging/event.go docs/design-docs/agent-conversation-hosting.md docs/design-docs/message-chain-contract.md docs/product-specs/message-chain.md
done

agent_conversation_hosting_camel_patterns=(
  "messageOrigin"
  "agentAccountId"
  "triggerServerMsgId"
  "agentRunId"
  "allowRecursiveTrigger"
)

for pattern in "${agent_conversation_hosting_camel_patterns[@]}"; do
  rg -q "$pattern" api/message.api internal/types/types.go web/src/api/messages.ts web/src/models/messages.ts web/src/features/messages/MessagesPage.tsx docs/product-specs/frontend-backend-contract.md
done

agent_conversation_hosting_code_patterns=(
  "MessageCreatedHook"
  "SetMessageCreatedHook"
  "message.created:"
  "NewConversationHostingService"
  "OnMessageCreated"
  "TryStartAgentTrigger"
  "FinishAgentTrigger"
  "agent_conversation_hosting"
  "agent_trigger_idempotency"
  "MessageServiceResponseWriter"
  "SendMessage\\(ctx"
)

for pattern in "${agent_conversation_hosting_code_patterns[@]}"; do
  rg -q "$pattern" internal/logic/messagelogic.go internal/agentim internal/repository db/migrations/003_agent_conversation_hosting.sql docs/design-docs/agent-conversation-hosting.md
done

agent_conversation_hosting_test_patterns=(
  "TestConversationHostingWritesAIResponseThroughMessageServiceAndDeduplicates"
  "SetMessageCreatedHook"
  "AI Agent"
  "messageOrigin: 'ai'"
  "deterministic-test"
)

for pattern in "${agent_conversation_hosting_test_patterns[@]}"; do
  rg -q "$pattern" internal/agentim/hosting_test.go web/src/features/messages/MessagesPage.test.tsx
done

agent_conversation_hosting_doc_patterns=(
  "message_origin=human|ai|system"
  "MessageLogic.SendMessage"
  "MessageCreatedHook"
  "idempotency"
  "allow_recursive_trigger"
  "AI/Agent"
  "fail closed"
)

for pattern in "${agent_conversation_hosting_doc_patterns[@]}"; do
  rg -q "$pattern" docs/design-docs/agent-conversation-hosting.md docs/product-specs/agent-system.md docs/product-specs/agent-chat.md docs/product-specs/frontend-backend-contract.md docs/FRONTEND.md ARCHITECTURE.md docs/exec-plans/completed/agent-conversation-hosting.md
done

if rg -n "CreateMessageIdempotent|insert into messages|insertMessage" internal/agentim --glob '*.go'; then
  echo "agentim must write responses through MessageLogic/Message Service, not message repository or direct DB insert" >&2
  exit 1
fi

rpc_generated_dirs=(
  "service/user/rpc"
  "internal/rpcgen/auth"
  "internal/rpcgen/friends"
  "internal/rpcgen/groups"
  "internal/rpcgen/message"
  "internal/rpcgen/mail"
)

for dir in "${rpc_generated_dirs[@]}"; do
  if [[ ! -d "$dir/internal/server" || ! -d "$dir/internal/logic" || ! -d "$dir/internal/svc" ]]; then
    echo "missing goctl rpc generated scaffold under: $dir" >&2
    exit 1
  fi
done

rpc_generated_servers=(
  "service/user/rpc/internal/server/user_service_server.go:UserServiceServer"
  "internal/rpcgen/auth/internal/server/auth_service_server.go:AuthServiceServer"
  "internal/rpcgen/friends/internal/server/friends_service_server.go:FriendsServiceServer"
  "internal/rpcgen/groups/internal/server/groups_service_server.go:GroupsServiceServer"
  "internal/rpcgen/message/internal/server/message_service_server.go:MessageServiceServer"
  "internal/rpcgen/mail/internal/server/mail_service_server.go:MailServiceServer"
)

for server_spec in "${rpc_generated_servers[@]}"; do
  file="${server_spec%%:*}"
  server_name="${server_spec##*:}"
  rg -q "Code generated by goctl. DO NOT EDIT." "$file"
  rg -q "type ${server_name} struct" "$file"
  rg -q "Unimplemented${server_name}" "$file"
done

rpc_generated_entrypoints=(
  "service/user/rpc/user.v1.go:RegisterUserServiceServer"
  "internal/rpcgen/auth/auth.v1.go:RegisterAuthServiceServer"
  "internal/rpcgen/friends/friends.v1.go:RegisterFriendsServiceServer"
  "internal/rpcgen/groups/groups.v1.go:RegisterGroupsServiceServer"
  "internal/rpcgen/message/message.go:RegisterMessageServiceServer"
  "internal/rpcgen/mail/mail.v1.go:RegisterMailServiceServer"
)

for entrypoint_spec in "${rpc_generated_entrypoints[@]}"; do
  file="${entrypoint_spec%%:*}"
  register_func="${entrypoint_spec##*:}"
  rg -q "$register_func" "$file"
done

for cmd_file in cmd/*-rpc/main.go; do
  if rg -n '"github.com/wujunhui99/agents_im/internal/(auth/)?rpc"' "$cmd_file"; then
    echo "rpc command imports old handwritten rpc wrapper: $cmd_file" >&2
    exit 1
  fi
done

rpc_entry_patterns=(
  "cmd/user-rpc/main.go:service/user/rpc/entry"
  "cmd/auth-rpc/main.go:internal/rpcgen/auth/entry"
  "cmd/friends-rpc/main.go:internal/rpcgen/friends/entry"
  "cmd/groups-rpc/main.go:internal/rpcgen/groups/entry"
  "cmd/message-rpc/main.go:internal/rpcgen/message/entry"
  "cmd/mail-rpc/main.go:internal/rpcgen/mail/entry"
  "service/user/rpc/entry/entry.go:Start bridges cmd/user-rpc"
  "internal/rpcgen/auth/entry/entry.go:Start bridges cmd/auth-rpc"
  "internal/rpcgen/friends/entry/entry.go:Start bridges cmd/friends-rpc"
  "internal/rpcgen/groups/entry/entry.go:Start bridges cmd/groups-rpc"
  "internal/rpcgen/message/entry/entry.go:Start bridges cmd/message-rpc"
  "internal/rpcgen/mail/entry/entry.go:Start bridges cmd/mail-rpc"
)

for entry_spec in "${rpc_entry_patterns[@]}"; do
  file="${entry_spec%%:*}"
  pattern="${entry_spec##*:}"
  rg -q "$pattern" "$file"
done

api_entry_patterns=(
  "cmd/user-api/main.go:service/user/api/entry"
  "service/user/api/entry/entry.go:Start bridges cmd/user-api"
)

for entry_spec in "${api_entry_patterns[@]}"; do
  file="${entry_spec%%:*}"
  pattern="${entry_spec##*:}"
  rg -q "$pattern" "$file"
done

if rg -n '"github.com/wujunhui99/agents_im/internal/(repository|model|objectstorage|servicecontext/user)"|DataSource|StorageDriver|New.*Repository' cmd/user-api service/user/api --glob '*.go' --glob '!*_test.go'; then
  echo "user API entry and service/user/api must not own data access; use RPC/BFF calls" >&2
  exit 1
fi

if rg -n "todo: add your logic here|return &.*Response\\{\\}, nil" service/user/api/internal/logic; then
  echo "generated user API logic still contains empty scaffold behavior" >&2
  exit 1
fi

if rg -n '"github.com/wujunhui99/agents_im/internal/(logic|repository|auth/logic|auth/repository)"' internal/rpcgen/*/entry --glob '*.go'; then
  echo "rpc entry bridges must not own business wiring; keep dependencies behind generated rpc service contexts" >&2
  exit 1
fi

if rg -n "todo: add your logic here|return &.*Response\\{\\}, nil" internal/rpcgen/*/internal/logic service/user/rpc/internal/logic; then
  echo "generated rpc logic still contains empty scaffold behavior" >&2
  exit 1
fi

rpc_logic_markers=(
  "service/user/rpc/internal/logic:UserLogic"
  "internal/rpcgen/auth/internal/logic:AuthLogic"
  "internal/rpcgen/friends/internal/logic:FriendsLogic"
  "internal/rpcgen/groups/internal/logic:GroupsLogic"
  "internal/rpcgen/message/internal/logic:MessageLogic"
  "internal/rpcgen/mail/internal/logic:MailProvider"
)

for logic_spec in "${rpc_logic_markers[@]}"; do
  dir="${logic_spec%%:*}"
  marker="${logic_spec##*:}"
  rg -q "svcCtx\\.${marker}" "$dir"
done

if rg -n "RegisterHandlers|RegisterUserHandlers|RegisterFriendsHandlers|RegisterGroupsHandlers|RegisterMessageHandlers|authhandler\\.RegisterHandlers" tests; then
  echo "tests still use old REST mux registration helpers" >&2
  exit 1
fi

rpc_generated_proto_files=(
  "proto/userpb/user.pb.go"
  "proto/userpb/user_grpc.pb.go"
  "proto/authpb/auth.pb.go"
  "proto/authpb/auth_grpc.pb.go"
  "proto/friendspb/friends.pb.go"
  "proto/friendspb/friends_grpc.pb.go"
  "proto/groupspb/groups.pb.go"
  "proto/groupspb/groups_grpc.pb.go"
  "proto/messagepb/message.pb.go"
  "proto/messagepb/message_grpc.pb.go"
  "proto/mailpb/mail.pb.go"
  "proto/mailpb/mail_grpc.pb.go"
)

for file in "${rpc_generated_proto_files[@]}"; do
  rg -q "Code generated by protoc-gen-go" "$file"
done

gateway_doc_patterns=(
  "send_message"
  "pull_messages"
  "get_conversation_seqs"
  "mark_conversation_read"
  "heartbeat"
  "SendMessage"
  "PullMessages"
  "GetConversationSeqs"
  "MarkConversationAsRead"
)

for pattern in "${gateway_doc_patterns[@]}"; do
  rg -q "$pattern" docs/design-docs/gateway-message-contract.md
  rg -q "$pattern" internal/gateway/contract.go tests/gateway_contract_test.go
done

websocket_gateway_patterns=(
  "GET /ws"
  "Authorization: Bearer"
  "query param"
  "ConnectionManager"
  "PresenceReporter"
  "Redis Presence"
  "Kafka fanout"
  "Push worker"
)

for pattern in "${websocket_gateway_patterns[@]}"; do
  rg -q "$pattern" docs/design-docs/websocket-gateway.md docs/exec-plans/completed/websocket-gateway.md
done

websocket_gateway_code_patterns=(
  "HandleWebSocket"
  "Validate\\(rawToken\\)"
  "Register"
  "CommandHeartbeat"
  "CommandSendMessage"
  "CommandPullMessages"
  "CommandGetConversationSeqs"
  "CommandMarkConversationRead"
)

for pattern in "${websocket_gateway_code_patterns[@]}"; do
  rg -q "$pattern" internal/gateway/ws internal/gateway/contract.go tests/websocket_gateway_test.go
done

rg -q "gateway-ws" cmd/gateway-ws/main.go etc/gateway-ws.yaml ARCHITECTURE.md
rg -q "AllowQueryToken: true" deploy/k8s/etc/gateway-ws.yaml
rg -q 'GATEWAY_WS_ALLOW_QUERY_TOKEN: "true"' deploy/k8s/configmap.yaml
rg -q 'GATEWAY_WS_ALLOWED_ORIGINS: "https://agenticim\.xyz"' deploy/k8s/configmap.yaml
if rg -q 'GATEWAY_WS_ALLOWED_ORIGINS:\s*""' deploy/k8s/configmap.yaml; then
  echo "production k8s websocket origins must not be empty" >&2
  exit 1
fi
rg -F -q 'AllowedOrigins: ${GATEWAY_WS_ALLOWED_ORIGINS}' deploy/k8s/etc/gateway-ws.yaml
rg -q 'AllowedOrigins: http://localhost:5173,http://127\.0\.0\.1:5173' etc/gateway-ws.yaml
rg -q "AllowQueryToken: true" etc/gateway-ws.yaml
rg -q "TestWebSocketOriginPolicyUsesConfiguredExactOrigins" internal/gateway/ws/server_test.go
rg -q "websocket-gateway.md" docs/design-docs/index.md ARCHITECTURE.md
rg -q "websocket-gateway" docs/exec-plans/completed/websocket-gateway.md

websocket_reconnect_sync_patterns=(
  "requestId"
  "status"
  "error.code"
  "error.message"
  "get_conversation_seqs"
  "pull_messages"
  "mark_conversation_read"
  "serverMsgId"
  "conversationId"
  "hasReadSeq"
  "unreadCount"
)

for pattern in "${websocket_reconnect_sync_patterns[@]}"; do
  rg -q "$pattern" docs/product-specs/frontend-sync-contract.md docs/design-docs/websocket-reconnect-sync.md
done

websocket_reconnect_sync_code_patterns=(
  "RequestIDCamel"
  "Payload"
  "frontendErrorCode"
  "VALIDATION_ERROR"
  "TestWebSocketGatewayReconnectSyncFlow"
  "TestWebSocketGatewayPullMessagesIsDuplicateSafe"
  "TestWebSocketGatewayPullMessagesFromMissingSeq"
  "TestWebSocketGatewayInvalidCommandReturnsFrontendErrorEnvelope"
)

for pattern in "${websocket_reconnect_sync_code_patterns[@]}"; do
  rg -q "$pattern" internal/gateway/ws/server.go tests/websocket_gateway_test.go
done

rg -q "frontend-sync-contract.md" docs/product-specs/index.md ARCHITECTURE.md docs/design-docs/backend-mvp-contract.md
rg -q "websocket-reconnect-sync.md" docs/design-docs/index.md ARCHITECTURE.md docs/design-docs/backend-mvp-contract.md docs/design-docs/websocket-gateway.md

message_transfer_code_patterns=(
  "type MessageEvent struct"
  "type Envelope struct"
  "type EventConsumer interface"
  "type DeliveryDispatcher interface"
  "type IdempotencyStore interface"
  "type RetryDecision struct"
  "type Worker struct"
  "func NewWorker"
  "func \\(w \\*Worker\\) Start"
  "func \\(w \\*Worker\\) RunOnce"
  "func \\(w \\*Worker\\) Stop"
  "NewInMemoryConsumer"
  "type NoopDispatcher struct"
)

for pattern in "${message_transfer_code_patterns[@]}"; do
  rg -q "$pattern" internal/transfer
done

message_transfer_test_patterns=(
  "TestWorkerConsumesEventAndMarksSuccessful"
  "TestWorkerIdempotencySkipsDuplicateDispatch"
  "TestWorkerRetryableFailureDoesNotMarkSuccessful"
  "TestWorkerContextCancellationStopsLoop"
)

for pattern in "${message_transfer_test_patterns[@]}"; do
  rg -q "$pattern" internal/transfer/worker_test.go
done

message_transfer_doc_patterns=(
  "message.accepted"
  "EventConsumer"
  "DeliveryDispatcher"
  "IdempotencyStore"
  "RetryDecision"
  "memory consumer"
  "noop dispatcher"
)

for pattern in "${message_transfer_doc_patterns[@]}"; do
  rg -q "$pattern" docs/design-docs/message-transfer-worker.md docs/exec-plans/active/message-transfer-worker.md
done

kafka_transfer_consumer_code_patterns=(
  "type KafkaEventConsumerConfig struct"
  "type KafkaEventConsumer struct"
  "func NewKafkaEventConsumer"
  "func EnvelopeFromKafkaMessage"
  "messaging.UnmarshalMessageEvent"
  "DefaultMessageEventsTopic"
  "EventTypeMessageAccepted"
  "FetchMessage"
  "CommitMessages"
  "func \\(c \\*KafkaEventConsumer\\) MarkSuccessful"
  "func \\(c \\*KafkaEventConsumer\\) MarkRetry"
  "func \\(c \\*KafkaEventConsumer\\) MarkFailed"
)

for pattern in "${kafka_transfer_consumer_code_patterns[@]}"; do
  rg -q "$pattern" internal/transfer/kafka_consumer.go internal/transfer/kafka_consumer_test.go
done

kafka_transfer_consumer_test_patterns=(
  "TestEnvelopeFromKafkaMessageMapsAcceptedEvent"
  "TestEnvelopeFromKafkaMessageRejectsInvalidEvents"
  "TestKafkaEventConsumerConstructorDoesNotRequireLiveBroker"
  "TestKafkaEventConsumerReceiveAndAckSemantics"
  "TestKafkaEventConsumerConsumesRedpandaEvent"
  "KAFKA_REDPANDA_INTEGRATION"
  "KAFKA_BROKERS"
  "t\\.Skip"
)

for pattern in "${kafka_transfer_consumer_test_patterns[@]}"; do
  rg -q "$pattern" internal/transfer/kafka_consumer_test.go internal/transfer/kafka_integration_test.go
done

kafka_transfer_config_patterns=(
  "TransferConsumerKafka"
  "MESSAGE_TRANSFER_CONSUMER_DRIVER"
  "Kafka\\s+KafkaConfig"
  "cfg.Kafka = kafkaConfigFromValues"
  "NewKafkaEventConsumer"
  "KAFKA_MESSAGE_EVENTS_TOPIC"
  "KAFKA_CONSUMER_GROUP"
)

for pattern in "${kafka_transfer_config_patterns[@]}"; do
  rg -q "$pattern" internal/config/config.go internal/config/config_test.go cmd/message-transfer/main.go etc/message-transfer.yaml
done

kafka_transfer_doc_patterns=(
  "message.events.v1"
  "message.accepted"
  "messaging.MessageEvent"
  "transfer.Envelope"
  "MarkSuccessful"
  "MarkRetry"
  "MarkFailed"
  "CommitMessages"
  "KAFKA_REDPANDA_INTEGRATION"
  "KAFKA_BROKERS"
)

for pattern in "${kafka_transfer_doc_patterns[@]}"; do
  rg -q "$pattern" docs/design-docs/kafka-transfer-consumer.md docs/exec-plans/active/kafka-transfer-consumer.md
done

rg -q "kafka-transfer-consumer.md" ARCHITECTURE.md docs/design-docs/index.md docs/design-docs/message-transfer-worker.md
rg -q "kafka-transfer-consumer" docs/exec-plans/active/kafka-transfer-consumer.md

rg -q "LoadMessageTransferConfig" internal/config/config.go
rg -q "message-transfer" cmd/message-transfer/main.go etc/message-transfer.yaml ARCHITECTURE.md
rg -q "message-transfer-worker.md" docs/design-docs/index.md ARCHITECTURE.md
rg -q "ConsumerGroup|Consumer\\.Group" etc/message-transfer.yaml internal/config/config.go
rg -q "Topic|Consumer\\.Topic" etc/message-transfer.yaml internal/config/config.go
rg -q "WorkerID|Worker\\.ID" etc/message-transfer.yaml internal/config/config.go

gateway_delivery_code_patterns=(
  "type Dispatcher interface"
  "DeliverToUser"
  "DeliverToConversation"
  "EventMessageReceived"
  "EventMessageDelivered"
  "StatusOffline"
  "NewInMemoryDeliveryDispatcher"
  "PushToUser"
  "PushToConversation"
  "UserConnections"
)

for pattern in "${gateway_delivery_code_patterns[@]}"; do
  rg -q "$pattern" internal/gateway/delivery internal/gateway/ws tests/websocket_gateway_test.go
done

gateway_delivery_doc_patterns=(
  "Message Transfer worker"
  "Redis Presence"
  "message_received"
  "message_delivered"
  "server_msg_id"
  "conversation_id"
  "offline"
  "in-memory"
)

for pattern in "${gateway_delivery_doc_patterns[@]}"; do
  rg -q "$pattern" docs/design-docs/gateway-push-delivery.md docs/exec-plans/completed/gateway-push-delivery.md
done

rg -q "gateway-push-delivery.md" ARCHITECTURE.md docs/design-docs/index.md

transfer_gateway_dispatcher_code_patterns=(
  "type Dispatcher struct"
  "func NewDispatcher"
  "func \\(d \\*Dispatcher\\) Dispatch"
  "EventMessageReceived"
  "DeliverToConversation"
  "StatusOffline"
  "DispatchRetryable"
  "ErrNoRecipients"
)

for pattern in "${transfer_gateway_dispatcher_code_patterns[@]}"; do
  rg -q "$pattern" internal/transfer/gateway
done

transfer_gateway_dispatcher_test_patterns=(
  "TestDispatcherDeliversMessageAcceptedToGateway"
  "TestDispatcherOfflineRecipientsAreCompletedWithoutDeliveredUsers"
  "TestDispatcherNoRecipientsFailsWithoutCallingGateway"
  "TestWorkerIdempotencySkipsDuplicateGatewayDispatch"
  "TestWorkerRetryDecisionForGatewayError"
)

for pattern in "${transfer_gateway_dispatcher_test_patterns[@]}"; do
  rg -q "$pattern" internal/transfer/gateway/dispatcher_test.go
done

transfer_gateway_dispatcher_doc_patterns=(
  "message.accepted"
  "message_received"
  "delivery.Dispatcher"
  "RetryDecision"
  "offline"
  "no recipients"
  "Redis cross-instance"
)

for pattern in "${transfer_gateway_dispatcher_doc_patterns[@]}"; do
  rg -q "$pattern" docs/design-docs/transfer-gateway-dispatcher.md docs/exec-plans/completed/transfer-gateway-dispatcher.md
done

rg -q "transfer-gateway-dispatcher.md" ARCHITECTURE.md docs/design-docs/index.md

message_v2_delivery_code_patterns=(
  "DeliveryRecipientUserIDs"
  "type DeliveryAttemptRecorder interface"
  "MetricsDeliveryAttemptRecorder"
  "RecipientDeliveryResult"
  "message_outbox"
)

for pattern in "${message_v2_delivery_code_patterns[@]}"; do
  rg -q "$pattern" internal/repository internal/transfer db/migrations/001_init_postgres.sql
done

message_v2_removed_table_patterns=(
  "message_idempotency_keys"
)

for pattern in "${message_v2_removed_table_patterns[@]}"; do
  if rg -q "$pattern" db/migrations/001_init_postgres.sql internal/repository --glob '*.go'; then
    echo "removed message V2 table still referenced: $pattern" >&2
    exit 1
  fi
done

delivery_reliability_doc_patterns=(
  "accepted"
  "published"
  "delivered"
  "offline"
  "failed"
  "next_retry_at"
  "has_read_seq"
  "not a read receipt"
)

for pattern in "${delivery_reliability_doc_patterns[@]}"; do
  rg -q "$pattern" docs/design-docs/message-delivery-reliability.md
done

rg -q "message-delivery-reliability.md" ARCHITECTURE.md docs/design-docs/index.md docs/design-docs/message-transfer-worker.md docs/design-docs/transfer-gateway-dispatcher.md docs/exec-plans/active/backend-mvp-completion.md

gateway_presence_routing_code_patterns=(
  "WithPresenceStore"
  "WithPresenceTTL"
  "WithInstanceID"
  "RegisterConnection"
  "Heartbeat"
  "UnregisterConnection"
  "ListUserConnections"
  "InstanceID"
  "StatusRouted"
  "type Route struct"
)

for pattern in "${gateway_presence_routing_code_patterns[@]}"; do
  rg -q "$pattern" internal/gateway/ws internal/gateway/delivery internal/presence tests/websocket_gateway_test.go
done

gateway_presence_routing_doc_patterns=(
  "PresenceStore"
  "connection_id"
  "instance_id"
  "heartbeat"
  "offline"
  "routed"
  "in-process"
  "cross-instance"
)

for pattern in "${gateway_presence_routing_doc_patterns[@]}"; do
  rg -q "$pattern" docs/design-docs/gateway-presence-routing.md docs/exec-plans/completed/gateway-presence-routing.md
done

rg -q "gateway-presence-routing.md" ARCHITECTURE.md docs/design-docs/index.md docs/design-docs/websocket-gateway.md docs/design-docs/gateway-push-delivery.md
rg -q "Presence:" etc/gateway-ws.yaml

gateway_product_patterns=(
  "command ACK"
  "Gateway does not store read progress"
  "does not mean recipients have seen it"
)

for pattern in "${gateway_product_patterns[@]}"; do
  rg -q "$pattern" docs/product-specs/gateway-message-contract.md
done

rg -q "gateway-message-contract.md" docs/design-docs/index.md docs/product-specs/index.md

redis_compose_patterns=(
  "^  redis:"
  "redis:7-alpine"
  "agents-im-redis"
  "agents_im_redis_data"
  "REDIS_PASSWORD"
)

for pattern in "${redis_compose_patterns[@]}"; do
  rg -q "$pattern" docker-compose.yml
done

redpanda_compose_patterns=(
  "^  redpanda:"
  "docker.redpanda.com/redpandadata/redpanda"
  "agents-im-redpanda"
  "kafka-addr"
  "advertise-kafka-addr"
  "REDPANDA_KAFKA_PORT"
  "agents_im_redpanda_data"
)

for pattern in "${redpanda_compose_patterns[@]}"; do
  rg -q "$pattern" docker-compose.yml
done

minio_compose_patterns=(
  "^  minio:"
  "minio/minio"
  "agents-im-minio"
  "MINIO_ROOT_USER"
  "MINIO_ROOT_PASSWORD"
  "MINIO_API_PORT"
  "MINIO_CONSOLE_PORT"
  "agents_im_minio_data"
)

for pattern in "${minio_compose_patterns[@]}"; do
  rg -q "$pattern" docker-compose.yml deploy/middleware/docker-compose.yml
done

redis_env_patterns=(
  "REDIS_ADDR"
  "REDIS_PASSWORD"
  "REDIS_DB"
  "PRESENCE_DRIVER"
  "PRESENCE_TTL_SECONDS"
  "PRESENCE_KEY_PREFIX"
)

for pattern in "${redis_env_patterns[@]}"; do
  rg -q "$pattern" .env.example
done

kafka_env_patterns=(
  "KAFKA_BROKERS"
  "KAFKA_MESSAGE_EVENTS_TOPIC"
  "KAFKA_CONSUMER_GROUP"
  "REDPANDA_KAFKA_PORT"
  "REDPANDA_ADMIN_PORT"
)

for pattern in "${kafka_env_patterns[@]}"; do
  rg -q "$pattern" .env.example
done

object_storage_env_patterns=(
  "MINIO_ROOT_USER"
  "MINIO_ROOT_PASSWORD"
  "MINIO_API_PORT"
  "MINIO_CONSOLE_PORT"
  "OBJECT_STORAGE_DRIVER"
  "OBJECT_STORAGE_ENDPOINT"
  "OBJECT_STORAGE_EXTERNAL_ENDPOINT"
  "OBJECT_STORAGE_BUCKET"
  "OBJECT_STORAGE_REGION"
  "OBJECT_STORAGE_USE_SSL"
  "OBJECT_STORAGE_EXTERNAL_USE_SSL"
  "OBJECT_STORAGE_ACCESS_KEY_ID"
  "OBJECT_STORAGE_SECRET_ACCESS_KEY"
)

for pattern in "${object_storage_env_patterns[@]}"; do
  rg -q "$pattern" .env.example deploy/middleware/.env.example deploy/k8s/secrets.example.yaml
done

presence_config_patterns=(
  "type RedisConfig"
  "type PresenceConfig"
  "ResolveRedisConfig"
  "ResolvePresenceConfig"
  "ResolvePresenceDriver"
)

for pattern in "${presence_config_patterns[@]}"; do
  rg -q "$pattern" internal/config/config.go
done

kafka_config_patterns=(
  "type KafkaConfig"
  "DefaultKafkaConfig"
  "ResolveKafkaConfig"
  "KAFKA_BROKERS"
  "KAFKA_MESSAGE_EVENTS_TOPIC"
  "KAFKA_CONSUMER_GROUP"
)

for pattern in "${kafka_config_patterns[@]}"; do
  rg -q "$pattern" internal/config/config.go internal/config/config_test.go
done

presence_code_patterns=(
  "type PresenceStore interface"
  "RegisterConnection"
  "Heartbeat"
  "UnregisterConnection"
  "ListUserConnections"
  "IsUserOnline"
  "github.com/redis/go-redis/v9"
  ":user:"
  ":conn:"
)

for pattern in "${presence_code_patterns[@]}"; do
  rg -q "$pattern" internal/presence
done

presence_doc_patterns=(
  "PostgreSQL remains the source of truth"
  "Redis presence is non-authoritative"
  "agents_im:presence:user"
  "agents_im:presence:conn"
  "Heartbeat"
  "REDIS_ADDR"
  "go test ./..."
)

for pattern in "${presence_doc_patterns[@]}"; do
  rg -q "$pattern" docs/design-docs/redis-presence.md docs/exec-plans/active/redis-presence.md
done

rg -q "redis-presence.md" ARCHITECTURE.md docs/design-docs/index.md
rg -q "REDIS_ADDR is required.*skip|t\\.Skip" internal/presence/redis_integration_test.go

message_event_schema_patterns=(
  "type MessageEvent struct"
  "event_id"
  "event_type"
  "conversation_id"
  "server_msg_id"
  "sender_id"
  "chat_type"
  "created_at"
  "payload"
  "message.accepted"
  "message.read"
)

for pattern in "${message_event_schema_patterns[@]}"; do
  rg -q "$pattern" internal/messaging/event.go internal/messaging/event_test.go docs/design-docs/kafka-message-events.md
done

producer_contract_patterns=(
  "type Producer interface"
  "NewNoopProducer"
  "NewInMemoryProducer"
  "NewKafkaProducer"
  "ParseBrokerList"
  "segmentio/kafka-go"
  "KAFKA_REDPANDA_INTEGRATION"
  "t\\.Skip"
)

for pattern in "${producer_contract_patterns[@]}"; do
  rg -q "$pattern" internal/messaging go.mod
done

kafka_doc_patterns=(
  "message.events.v1"
  "conversation_id"
  "at-least-once"
  "outbox"
  "Message Transfer"
  "Gateway"
  "Push"
  "KAFKA_BROKERS"
  "Redpanda"
)

for pattern in "${kafka_doc_patterns[@]}"; do
  rg -q "$pattern" docs/design-docs/kafka-message-events.md docs/exec-plans/active/kafka-redpanda-compose.md
done

rg -q "kafka-message-events.md" ARCHITECTURE.md docs/design-docs/index.md docs/design-docs/message-chain-contract.md
rg -q "kafka-redpanda-compose" docs/exec-plans/active/kafka-redpanda-compose.md
read_receipt_patterns=(
  "has_read_seq"
  "unread_count"
  "message.read"
  "requested_has_read_seq > max_seq"
)

for pattern in "${read_receipt_patterns[@]}"; do
  rg -q "$pattern" docs/design-docs/read-receipts.md docs/product-specs/read-receipts.md
done

read_receipt_code_patterns=(
  "NormalizeMarkRead"
  "CanAdvanceReadSeq"
  "UnreadCount"
  "ErrReadSeqExceedsMax"
)

for pattern in "${read_receipt_code_patterns[@]}"; do
  rg -q "$pattern" internal/domain/readreceipt/read_receipt.go tests/read_receipts_test.go
done

rg -q "read-receipts.md" docs/design-docs/index.md docs/product-specs/index.md
if rg -n "X-User-Id|CurrentUserID|currentUserID" api internal cmd; then
  echo "production API/code still contains header-based current user auth" >&2
  exit 1
fi

legacy_x_user_id_sets="$(rg -n 'Header\.Set\("X-User-Id"' tests internal || true)"
if [[ -n "$legacy_x_user_id_sets" ]]; then
  disallowed_legacy_x_user_id_sets="$(printf '%s\n' "$legacy_x_user_id_sets" | rg -v 'legacy X-User-Id rejection helper' || true)"
  if [[ -n "$disallowed_legacy_x_user_id_sets" ]]; then
    printf '%s\n' "$disallowed_legacy_x_user_id_sets" >&2
    echo "legacy X-User-Id header writes in tests/internal must use Authorization Bearer JWT or an explicit rejection helper/comment" >&2
    exit 1
  fi
fi

if ! grep -q 'AllowQueryToken: true' deploy/k8s/etc/gateway-ws.yaml; then
  echo "production gateway-ws must allow query token for browser WebSocket" >&2
  exit 1
fi
if grep -A2 '^Dispatcher:' deploy/k8s/etc/message-transfer.yaml | grep -q 'Driver: noop'; then
  echo "production message-transfer must not use noop dispatcher" >&2
  exit 1
fi
if grep -q '^DryRun: true' deploy/k8s/etc/message-transfer.yaml; then
  echo "production message-transfer must not run in dry-run mode" >&2
  exit 1
fi
if ! grep -A4 '^Consumer:' deploy/k8s/etc/message-transfer.yaml | grep -q 'Driver: outbox'; then
  echo "production message-transfer must consume message_outbox for V1 live push" >&2
  exit 1
fi
if ! grep -A3 '^Dispatcher:' deploy/k8s/etc/message-transfer.yaml | grep -q 'Driver: gateway'; then
  echo "production message-transfer must dispatch to gateway-ws" >&2
  exit 1
fi
if ! grep -A3 '^Dispatcher:' deploy/k8s/etc/message-transfer.yaml | grep -q 'GatewayEndpoint: http://127\.0\.0\.1:8084'; then
  echo "production message-transfer must target colocated gateway-ws internal endpoint" >&2
  exit 1
fi

python3 - <<'PY'
import sys
import yaml

for path in (
    "deploy/k8s/etc/auth-api.yaml",
    "deploy/k8s/etc/auth-rpc.yaml",
    "etc/auth-api.yaml",
    "etc/auth-rpc.yaml",
):
    with open(path, encoding="utf-8") as f:
        data = yaml.safe_load(f) or {}
    mail_rpc = data.get("MailRPC")
    if not isinstance(mail_rpc, dict):
        print(f"{path}: MailRPC section is required", file=sys.stderr)
        sys.exit(1)
    endpoints = mail_rpc.get("Endpoints")
    if not isinstance(endpoints, list) or not endpoints:
        print(f"{path}: MailRPC.Endpoints must be a non-empty YAML list", file=sys.stderr)
        sys.exit(1)
    for index, endpoint in enumerate(endpoints):
        if not isinstance(endpoint, str) or not endpoint.strip():
            print(f"{path}: MailRPC.Endpoints[{index}] must be a non-empty string", file=sys.stderr)
            sys.exit(1)
PY

jwt_api_files=(
  "api/user.api"
  "api/friends.api"
  "api/groups.api"
  "api/message.api"
  "api/media.api"
)

for file in "${jwt_api_files[@]}"; do
  rg -q "jwt:\\s+Auth" "$file"
done

jwt_config_files=(
  "etc/auth-api.yaml"
  "etc/user-api.yaml"
  "etc/friends-api.yaml"
  "etc/groups-api.yaml"
  "etc/message-api.yaml"
  "etc/auth-rpc.yaml"
)

for file in "${jwt_config_files[@]}"; do
  rg -q "AccessSecret" "$file"
  rg -q "AccessExpire" "$file"
done

rg -q "type JWTAuthConfig" internal/config/config.go
rg -q "AccessSecret" internal/config/config.go internal/rpcgen/auth/internal/config/config.go
rg -q "AccessExpire" internal/config/config.go internal/rpcgen/auth/internal/config/config.go
rg -q "user_id" internal/auth/token/token.go internal/ctxuser/user.go
rg -q "ctxuser\\.UserID" internal/logic/user internal/logic/friends internal/logic/groups internal/logic/message internal/logic/media
rg -q "sender_id must match authenticated user" internal/logic/message/send_message_logic.go

jwt_test_patterns=(
  "assertLooksLikeJWT"
  "TestAuthIssuedBearerTokenAccessesMe"
  "bearerTokenForUser"
  "legacy X-User-Id rejection"
  "invalid token status"
  "message sender did not use token user"
)

for pattern in "${jwt_test_patterns[@]}"; do
  rg -q "$pattern" tests
done

rg -q "jwt-auth-middleware.md" docs/design-docs/index.md
rg -q "Auth.AccessSecret" docs/design-docs/jwt-auth-middleware.md docs/design-docs/auth-service-go-zero.md
rg -q "senderId.*must match" docs/design-docs/jwt-auth-middleware.md docs/design-docs/message-chain-contract.md
rg -q "JWT Bearer token" docs/product-specs/auth-service.md docs/product-specs/user-service.md docs/product-specs/friends-service.md docs/product-specs/groups-service.md docs/product-specs/message-chain.md
rg -q "ExistsByIdentifier" internal/auth docs/design-docs/auth-service-go-zero.md docs/product-specs/auth-service.md
rg -q "CreateUser" internal/auth docs/design-docs/auth-service-go-zero.md docs/product-specs/auth-service.md
rg -q "PasswordHash" internal/auth/model/credential.go
rg -q "Salt" internal/auth/model/credential.go
rg -q "user-rpc" docs/design-docs/groups-service-go-zero.md docs/product-specs/groups-service.md
rg -q "client_msg_id" docs/product-specs/message-chain.md docs/design-docs/message-chain-contract.md "$message_plan_file"
rg -q "has_read_seq" docs/product-specs/message-chain.md docs/design-docs/message-chain-contract.md "$message_plan_file"

social_mvp_account_patterns=(
  "AddFriend"
  "重复添加同一有效好友"
  "添加自己为好友"
  "目标账号不存在"
  "MVP 群默认允许公开加入"
  "非成员或已退出成员发送群消息必须失败"
)

for pattern in "${social_mvp_account_patterns[@]}"; do
  rg -q "$pattern" docs/product-specs/account-social-core.md
done

social_mvp_boundary_patterns=(
  "MVP 业务规则"
  "GetFriendship"
  "FORBIDDEN"
  "不能写入消息、推进 seq 或创建 outbox"
  "creator 是 owner/member"
)

for pattern in "${social_mvp_boundary_patterns[@]}"; do
  rg -q "$pattern" docs/design-docs/user-auth-friends-groups-boundaries.md
done

social_mvp_code_patterns=(
  "CodeForbidden"
  "PermissionDenied"
  "sender is not a group member"
  "group membership validator is not configured"
  "group owner cannot leave as the only active member"
)

for pattern in "${social_mvp_code_patterns[@]}"; do
  rg -q "$pattern" internal
done

social_mvp_test_patterns=(
  "TestFriendsLogicNeverAddedStatusIsNone"
  "TestGroupsLogicOwnerCannotLeaveWhenOnlyActiveMember"
  "TestMessageGroupSendRequiresActiveMembership"
  "client-group-outsider"
  "client-group-left"
)

for pattern in "${social_mvp_test_patterns[@]}"; do
  rg -q "$pattern" tests
done

account_terminology_doc_patterns=(
  "Account Service"
  "account_type=user|agent|admin"
  "Snowflake"
  "accounts"
  "profiles"
  "user_id"
  "account id alias"
  "V0 compatibility"
  "Auth Service"
  "credential/password/token"
)

for pattern in "${account_terminology_doc_patterns[@]}"; do
  rg -qF "$pattern" docs/design-docs/account-service-terminology.md
done

account_terminology_entry_patterns=(
  "Account Service"
  "account id alias"
  "account_type=user|agent|admin"
)

for pattern in "${account_terminology_entry_patterns[@]}"; do
  rg -qF "$pattern" AGENTS.md ARCHITECTURE.md docs/product-specs/account-social-core.md docs/product-specs/frontend-backend-contract.md docs/design-docs/user-auth-friends-groups-boundaries.md docs/design-docs/user-service-go-zero.md docs/product-specs/user-service.md
done

account_code_patterns=(
  "AccountTypeUser  AccountType = \"user\""
  "type Account struct"
  "type Profile struct"
  "AccountID"
  "NewAccountProfile"
  "type AccountRepository interface"
  "type UserRepository = AccountRepository"
  "type AccountProfile = UserProfile"
  "func NewAccountLogic"
  "idgen.NewString"
  "AccountLogic"
  "Path:    \"/accounts\""
  "Path:    \"/accounts/exists\""
  "Path:    \"/accounts/:identifier\""
)

for pattern in "${account_code_patterns[@]}"; do
  rg -qF "$pattern" internal/model/user.go internal/repository/repository.go internal/repository/memory.go internal/repository/postgres_user_friends.go internal/logic/userlogic.go internal/servicecontext/user/service_context.go internal/handler/gozero_routes.go
done

account_storage_patterns=(
  "create table if not exists accounts"
  "create table if not exists profiles"
  "account_id text primary key"
  "account_type smallint not null default 1"
)

for pattern in "${account_storage_patterns[@]}"; do
  rg -qF "$pattern" db/migrations/001_init_postgres.sql
done

rg -qF "account_type?: 'user' | 'agent' | 'admin'" web/src/api/user.ts
rg -qF "Legacy server data that still contains \`normal\` is invalid and must be migrated before use" docs/product-specs/frontend-backend-contract.md
rg -qF "旧 \`account_type=normal\` 不再作为有效输入兼容" docs/design-docs/account-service-terminology.md docs/design-docs/user-service-go-zero.md docs/product-specs/user-service.md
rg -qF "Account Service 术语与 V0 compatibility" AGENTS.md docs/design-docs/index.md
rg -qF "Account Service 第一阶段产品规格" docs/product-specs/index.md docs/product-specs/user-service.md
rg -qF "Account Service go-zero 实现设计" docs/design-docs/index.md docs/design-docs/user-service-go-zero.md

if rg -n 'account_type.*normal|normal.*account_type|`normal`' \
  docs/product-specs/agent-system.md \
  docs/design-docs/agent-system-architecture.md \
  docs/exec-plans/active/agent-system-v0.md \
  docs/exec-plans/active/agent-infrastructure-parallel-baseline.md \
  web/src/api/user.ts; then
  echo "account_type docs/frontend must use user|agent|admin; normal may appear only in explicit migration compatibility docs" >&2
  exit 1
fi

rg -q "NewGroupsRepositoryForStorage" cmd/message-api/main.go cmd/gateway-ws/main.go internal/rpcgen/message/internal/svc/service_context.go
rg -q "NewMessageLogicWithMediaValidator" internal/rpcgen/message/internal/svc/service_context.go
rg -q "NewMessageRepositoryForStorage" internal/rpcgen/message/internal/svc/service_context.go
pg_persistence_patterns=(
  "accounts"
  "profiles"
  "auth_credentials"
  "friendships"
  "groups"
  "group_members"
  "media_objects"
  "messages"
  "conversation_threads"
  "user_conversation_states"
  "message_outbox"
)

for pattern in "${pg_persistence_patterns[@]}"; do
  rg -q "$pattern" db/migrations/001_init_postgres.sql docs/design-docs/postgres-persistence.md
done

rg -q "StorageDriver" internal/config/config.go etc/*.yaml
rg -q "ObjectStorageConfig" internal/config/config.go
rg -q "NewStore" internal/objectstorage/factory.go
rg -q "PresignPut" internal/objectstorage/store.go internal/objectstorage/minio.go
rg -q "NewMediaRepositoryForStorage" internal/repository/postgres_common.go cmd/user-api/main.go cmd/message-api/main.go cmd/gateway-ws/main.go
rg -q "ValidateMessageMedia" internal/logic/medialogic.go internal/logic/messagelogic.go
rg -q "media_objects" db/migrations/001_init_postgres.sql docs/product-specs/message-chain.md docs/product-specs/frontend-backend-contract.md
rg -q "PATCH /me/avatar" docs/product-specs/frontend-backend-contract.md
rg -q "POST /media/uploads" docs/product-specs/frontend-backend-contract.md
if rg -q 'OBJECT_STORAGE_EXTERNAL_ENDPOINT="?((127\.[0-9.]+|localhost|0\.0\.0\.0|\[?::1\]?)(:[0-9]+)?)"?' scripts/bootstrap-server.sh deploy/k8s/secrets.example.yaml; then
  echo "production object storage external endpoint must not be browser-local loopback" >&2
  exit 1
fi
if ! rg -q 'AGENTS_IM_ENV: "production"' deploy/k8s/configmap.yaml; then
  echo "production k8s config must enable production environment validation" >&2
  exit 1
fi
rg -q "NewPostgresRepository" internal/repository/postgres_user_friends.go internal/auth/repository/postgres.go
rg -q "NewPostgresGroupsRepository" internal/repository/postgres_groups.go
rg -q "NewPostgresMessageRepository" internal/repository/postgres_message.go
rg -q "docker compose" scripts/migrate-postgres.sh docs/design-docs/postgres-persistence.md

outbox_schema_patterns=(
  "event_id"
  "event_type"
  "aggregate_type"
  "aggregate_id"
  "conversation_id"
  "message_id"
  "payload jsonb"
  "attempt_count"
  "next_attempt_at"
  "locked_by"
  "locked_until"
  "published_at"
)

for pattern in "${outbox_schema_patterns[@]}"; do
  rg -q "$pattern" db/migrations/001_init_postgres.sql docs/design-docs/message-outbox.md
done

outbox_code_patterns=(
  "type OutboxRepository interface"
  "OutboxEventTypeMessageCreated"
  "PollPending"
  "MarkPublished"
  "MarkFailed"
  "messageCreatedOutboxPayload"
  "insertMessageOutboxEvent"
  "SKIP LOCKED"
)

for pattern in "${outbox_code_patterns[@]}"; do
  rg -q "$pattern" internal/repository/message_outbox_repository.go internal/repository/postgres_outbox.go internal/repository/postgres_message.go internal/repository/message_memory.go tests/message_service_test.go tests/postgres_persistence_integration_test.go
done

rg -q "message-outbox.md" ARCHITECTURE.md docs/design-docs/index.md docs/design-docs/postgres-persistence.md
rg -q "message-outbox" docs/exec-plans/active/message-outbox.md

outbox_publisher_code_patterns=(
  "package outboxpublisher"
  "type Publisher struct"
  "repository.OutboxRepository"
  "messaging.Producer"
  "PollPending"
  "MessageEventFromOutbox"
  "EventTypeMessageAccepted"
  "DefaultMessageEventsTopic"
  "MarkPublished"
  "MarkFailed"
)

for pattern in "${outbox_publisher_code_patterns[@]}"; do
  rg -q "$pattern" internal/outboxpublisher
done

outbox_publisher_doc_patterns=(
  "outbox-kafka-publisher.md"
  "message.events.v1"
  "message.accepted"
  "at-least-once"
  "event_id"
  "conversation_id"
  "MarkPublished"
  "MarkFailed"
)

for pattern in "${outbox_publisher_doc_patterns[@]}"; do
  rg -q "$pattern" ARCHITECTURE.md docs/design-docs/index.md docs/design-docs/outbox-kafka-publisher.md docs/exec-plans/active/outbox-kafka-publisher.md
done

rg -q "TestPublisherPublishesMessageCreatedOutboxEvent" internal/outboxpublisher/publisher_test.go
rg -q "TestPublisherMarksPublishErrorRetryable" internal/outboxpublisher/publisher_test.go
rg -q "TestPublisherMarksMalformedPayloadFailedForRetry" internal/outboxpublisher/publisher_test.go
rg -q "TestPublisherStopsOnContextCancellationWithoutMarkingFailed" internal/outboxpublisher/publisher_test.go

observability_code_patterns=(
  "StatusReady"
  "ReadinessHandler"
  "MetricMessageSends"
  "MetricDeliveryAttempts"
  "MetricTransferEvents"
  "MetricWebSocketCurrent"
  "TraceMiddleware"
  "HeaderTraceID"
)

for pattern in "${observability_code_patterns[@]}"; do
  rg -q "$pattern" internal/health internal/observability
done

for api_main in cmd/*-api/main.go; do
  if [[ "$api_main" == "cmd/user-api/main.go" ]]; then
    rg -q "TraceMiddlewareFunc" service/user/api/entry/entry.go
  else
    rg -q "TraceMiddlewareFunc" "$api_main"
  fi
done

observability_wiring_patterns=(
  "/readyz"
  "/metrics"
  "ReadinessHandler"
  "MetricsHandler"
)

for pattern in "${observability_wiring_patterns[@]}"; do
  rg -q "$pattern" internal/handler/gozero_routes.go service/user/api/entry/entry.go cmd/gateway-ws/main.go cmd/message-transfer/main.go
done

observability_metric_hooks=(
  "RecordMessageSend"
  "RecordDeliveryAttempt"
  "RecordTransferEvent"
  "SetWebSocketConnections"
  "RecordWebSocketConnectionEvent"
)

for pattern in "${observability_metric_hooks[@]}"; do
  rg -q "$pattern" internal/logic/messagelogic.go internal/gateway/ws internal/transfer/worker.go
done

rg -q "Observability:" etc/message-transfer.yaml
rg -q "MESSAGE_TRANSFER_OBSERVABILITY_PORT" .env.example
rg -q "observability-mvp.md" ARCHITECTURE.md docs/design-docs/index.md
rg -q "agents_im_message_sends_total" docs/design-docs/observability-mvp.md internal/observability/metrics.go
rg -q "trace_id" docs/design-docs/observability-mvp.md internal/gateway/ws/server.go

llm_observability_patterns=(
  "RuntimeModeAIHostingAutoReply"
  "NewEinoCallbackHandler"
  "ErrLangfuseConfigMissing"
  "langfuseIngestionPath"
  "LLMObservability"
  "LANGFUSE_PUBLIC_KEY"
  "PythonGoPerformanceCaseID"
  "ai_hosting.python_go_performance.v1"
)

for pattern in "${llm_observability_patterns[@]}"; do
  rg -q "$pattern" internal/llmobs internal/agenteval internal/agentim internal/agentruntime/eino internal/config docs/design-docs/llm-observability.md .env.example
done

rg -q "llm-observability.md" ARCHITECTURE.md docs/design-docs/index.md
rg -q "LLM_OBSERVABILITY_BACKEND=noop" .env.example

python3 - <<'PY'
import sys
import yaml

with open("deploy/k8s/kustomization.yaml", encoding="utf-8") as f:
    kustomization = yaml.safe_load(f) or {}
resources = set(kustomization.get("resources") or [])
for resource in ("tempo.yaml", "otel-collector.yaml", "prometheus-grafana.yaml", "loki.yaml", "langfuse.yaml"):
    if resource not in resources:
        print(f"deploy/k8s/kustomization.yaml: missing {resource}", file=sys.stderr)
        sys.exit(1)
if "jaeger.yaml" in resources:
    print("deploy/k8s/kustomization.yaml: jaeger.yaml must not be an active resource", file=sys.stderr)
    sys.exit(1)

with open("deploy/k8s/loki.yaml", encoding="utf-8") as f:
    loki_docs = [doc for doc in yaml.safe_load_all(f) if doc]
loki_by_kind_name = {(doc.get("kind"), doc.get("metadata", {}).get("name")): doc for doc in loki_docs}
for kind, name in (("ConfigMap", "loki-config"), ("ConfigMap", "promtail-config"), ("PersistentVolumeClaim", "loki-data"), ("Deployment", "loki"), ("DaemonSet", "promtail"), ("Service", "loki")):
    if (kind, name) not in loki_by_kind_name:
        print(f"deploy/k8s/loki.yaml: missing {kind} {name}", file=sys.stderr)
        sys.exit(1)
loki_config = yaml.safe_load((loki_by_kind_name[("ConfigMap", "loki-config")].get("data") or {}).get("config.yaml") or "") or {}
if loki_config.get("limits_config", {}).get("retention_period") != "168h":
    print("deploy/k8s/loki.yaml: Loki must keep bounded 7-day retention", file=sys.stderr)
    sys.exit(1)
promtail_config = yaml.safe_load((loki_by_kind_name[("ConfigMap", "promtail-config")].get("data") or {}).get("config.yaml") or "") or {}
promtail_clients = promtail_config.get("clients") or []
if not any(client.get("url") == "http://loki.agents-im.svc.cluster.local:3100/loki/api/v1/push" for client in promtail_clients):
    print("deploy/k8s/loki.yaml: Promtail must push to the in-cluster Loki service", file=sys.stderr)
    sys.exit(1)

with open("deploy/k8s/prometheus-grafana.yaml", encoding="utf-8") as f:
    prometheus_grafana_docs = [doc for doc in yaml.safe_load_all(f) if doc]
prometheus_grafana_by_kind_name = {(doc.get("kind"), doc.get("metadata", {}).get("name")): doc for doc in prometheus_grafana_docs}
for kind, name in (("ConfigMap", "grafana-provisioning"), ("PersistentVolumeClaim", "prometheus-data"), ("Deployment", "prometheus"), ("Deployment", "grafana"), ("Service", "prometheus"), ("Service", "grafana")):
    if (kind, name) not in prometheus_grafana_by_kind_name:
        print(f"deploy/k8s/prometheus-grafana.yaml: missing {kind} {name}", file=sys.stderr)
        sys.exit(1)
datasource_config = yaml.safe_load((prometheus_grafana_by_kind_name[("ConfigMap", "grafana-provisioning")].get("data") or {}).get("datasource.yml") or "") or {}
datasources = {item.get("name"): item for item in datasource_config.get("datasources") or []}
expected_datasources = {
    "Prometheus": {"uid": "prometheus", "type": "prometheus", "url": "http://prometheus.agents-im.svc.cluster.local:9090"},
    "Loki": {"uid": "loki", "type": "loki", "url": "http://loki.agents-im.svc.cluster.local:3100"},
    "Tempo": {"uid": "tempo", "type": "tempo", "url": "http://tempo.agents-im.svc.cluster.local:3200"},
}
for name, expected in expected_datasources.items():
    got = datasources.get(name)
    if not got:
        print(f"deploy/k8s/prometheus-grafana.yaml: missing Grafana datasource {name}", file=sys.stderr)
        sys.exit(1)
    for key, want in expected.items():
        if got.get(key) != want:
            print(f"deploy/k8s/prometheus-grafana.yaml: Grafana datasource {name} {key}={got.get(key)!r}, want {want!r}", file=sys.stderr)
            sys.exit(1)
if datasources["Tempo"].get("jsonData", {}).get("tracesToLogsV2", {}).get("datasourceUid") != "loki":
    print("deploy/k8s/prometheus-grafana.yaml: Tempo tracesToLogsV2 must target Loki uid", file=sys.stderr)
    sys.exit(1)

with open("deploy/k8s/tempo.yaml", encoding="utf-8") as f:
    tempo_docs = [doc for doc in yaml.safe_load_all(f) if doc]
if not any(doc.get("kind") == "PersistentVolumeClaim" and doc.get("metadata", {}).get("name") == "tempo-data" for doc in tempo_docs):
    print("deploy/k8s/tempo.yaml: Tempo must use a persistent tempo-data PVC", file=sys.stderr)
    sys.exit(1)
if not any(doc.get("kind") == "Deployment" and doc.get("metadata", {}).get("name") == "tempo" for doc in tempo_docs):
    print("deploy/k8s/tempo.yaml: missing Tempo Deployment", file=sys.stderr)
    sys.exit(1)
if not any(doc.get("kind") == "Service" and doc.get("metadata", {}).get("name") == "tempo" for doc in tempo_docs):
    print("deploy/k8s/tempo.yaml: missing Tempo Service", file=sys.stderr)
    sys.exit(1)

with open("deploy/k8s/otel-collector.yaml", encoding="utf-8") as f:
    otel_text = f.read()
    otel_docs = [doc for doc in yaml.safe_load_all(otel_text) if doc]
if "tempo.agents-im.svc.cluster.local:4317" not in otel_text:
    print("deploy/k8s/otel-collector.yaml: collector must export traces to Tempo", file=sys.stderr)
    sys.exit(1)
if "health_check" not in otel_text or "13133" not in otel_text:
    print("deploy/k8s/otel-collector.yaml: collector must expose health_check extension for probes", file=sys.stderr)
    sys.exit(1)
if not any(doc.get("kind") == "Deployment" and doc.get("metadata", {}).get("name") == "otel-collector" for doc in otel_docs):
    print("deploy/k8s/otel-collector.yaml: missing otel-collector Deployment", file=sys.stderr)
    sys.exit(1)
if not any(doc.get("kind") == "Service" and doc.get("metadata", {}).get("name") == "otel-collector" for doc in otel_docs):
    print("deploy/k8s/otel-collector.yaml: missing otel-collector Service", file=sys.stderr)
    sys.exit(1)

with open("deploy/k8s/ingress.yaml", encoding="utf-8") as f:
    docs = [doc for doc in yaml.safe_load_all(f) if doc]

def ingress_by_host(host):
    for doc in docs:
        if doc.get("kind") != "Ingress":
            continue
        for rule in doc.get("spec", {}).get("rules", []) or []:
            if rule.get("host") == host:
                return doc, rule
    return None, None

def backend_for(rule, path):
    for item in rule.get("http", {}).get("paths", []) or []:
        if item.get("path") == path:
            service = item.get("backend", {}).get("service", {})
            return service.get("name"), service.get("port", {}).get("number")
    return None, None

expected = {
    "langfuse.agenticim.xyz": ("langfuse", 3000, "langfuse-agenticim-xyz-tls"),
}
for host, (svc, port, tls_secret) in expected.items():
    ingress, rule = ingress_by_host(host)
    if not ingress:
        print(f"deploy/k8s/ingress.yaml: missing ingress rule for {host}", file=sys.stderr)
        sys.exit(1)
    tls_hosts = {
        tls_host: tls.get("secretName")
        for tls in ingress.get("spec", {}).get("tls", []) or []
        for tls_host in tls.get("hosts", []) or []
    }
    if tls_hosts.get(host) != tls_secret:
        print(f"deploy/k8s/ingress.yaml: {host} must use TLS secret {tls_secret}", file=sys.stderr)
        sys.exit(1)
    got_svc, got_port = backend_for(rule, "/")
    if (got_svc, got_port) != (svc, port):
        print(f"deploy/k8s/ingress.yaml: {host}/ routes to {(got_svc, got_port)}, want {(svc, port)}", file=sys.stderr)
        sys.exit(1)

jaeger_ingress, _ = ingress_by_host("jaeger.agenticim.xyz")
if jaeger_ingress:
    print("deploy/k8s/ingress.yaml: jaeger public ingress must be removed; use Grafana Tempo instead", file=sys.stderr)
    sys.exit(1)

prometheus_ingress, _ = ingress_by_host("prometheus.agenticim.xyz")
if prometheus_ingress:
    print("deploy/k8s/ingress.yaml: prometheus.agenticim.xyz public ingress must be removed; use ms.agenticim.xyz/observability/metrics", file=sys.stderr)
    sys.exit(1)

def backend_for_host_path(host, path):
    for doc in docs:
        for rule in doc.get("spec", {}).get("rules", []) or []:
            if rule.get("host") != host:
                continue
            service_name, service_port = backend_for(rule, path)
            if service_name:
                return doc, service_name, service_port
    return None, None, None

metrics_ingress, metrics_svc, metrics_port = backend_for_host_path("ms.agenticim.xyz", "/observability/metrics")
if (metrics_svc, metrics_port) != ("prometheus", 9090):
    print("deploy/k8s/ingress.yaml: ms.agenticim.xyz/observability/metrics must route to prometheus:9090", file=sys.stderr)
    sys.exit(1)
metrics_middlewares = metrics_ingress.get("metadata", {}).get("annotations", {}).get("traefik.ingress.kubernetes.io/router.middlewares", "")
if "agents-im-observability-basic-auth@kubernetescrd" not in metrics_middlewares:
    print("deploy/k8s/ingress.yaml: /observability/metrics must keep observability basic auth", file=sys.stderr)
    sys.exit(1)

ms_media_expectations = {
    "/media": ("user-api", 8080),
    "/agents-im-media": ("agents-im-minio", 9000),
}
for path, expected_backend in ms_media_expectations.items():
    _, service_name, service_port = backend_for_host_path("ms.agenticim.xyz", path)
    if (service_name, service_port) != expected_backend:
        print(
            f"deploy/k8s/ingress.yaml: ms.agenticim.xyz{path} routes to {(service_name, service_port)}, want {expected_backend}",
            file=sys.stderr,
        )
        sys.exit(1)

redirect_expectations = {
    "/observability/logs": "agents-im-observability-logs-redirect@kubernetescrd",
    "/observability/traces": "agents-im-observability-traces-redirect@kubernetescrd",
    "/observability/llm": "agents-im-observability-llm-redirect@kubernetescrd",
}
for path, middleware in redirect_expectations.items():
    redirect_ingress, _, _ = backend_for_host_path("ms.agenticim.xyz", path)
    if not redirect_ingress:
        print(f"deploy/k8s/ingress.yaml: missing ms.agenticim.xyz{path} redirect ingress", file=sys.stderr)
        sys.exit(1)
    middlewares = redirect_ingress.get("metadata", {}).get("annotations", {}).get("traefik.ingress.kubernetes.io/router.middlewares", "")
    if middleware not in middlewares:
        print(f"deploy/k8s/ingress.yaml: {path} must use {middleware}", file=sys.stderr)
        sys.exit(1)

middlewares_by_name = {
    doc.get("metadata", {}).get("name"): doc
    for doc in docs
    if doc.get("kind") == "Middleware"
}
redirect_url_expectations = {
    "observability-logs-redirect": ("schemaVersion=1", "%22type%22%3A%22loki%22", "%22uid%22%3A%22loki%22"),
    "observability-traces-redirect": ("schemaVersion=1", "%22type%22%3A%22tempo%22", "%22uid%22%3A%22tempo%22", "%22queryType%22%3A%22traceql%22"),
}
for name, required_parts in redirect_url_expectations.items():
    replacement = middlewares_by_name.get(name, {}).get("spec", {}).get("redirectRegex", {}).get("replacement", "")
    missing = [part for part in required_parts if part not in replacement]
    if missing:
        print(f"deploy/k8s/ingress.yaml: {name} replacement must use explicit Grafana Explore panes datasource; missing {missing}", file=sys.stderr)
        sys.exit(1)

with open("deploy/k8s/langfuse.yaml", encoding="utf-8") as f:
    langfuse_docs = [doc for doc in yaml.safe_load_all(f) if doc]
if not any(doc.get("kind") == "Deployment" and doc.get("metadata", {}).get("name") == "langfuse" for doc in langfuse_docs):
    print("deploy/k8s/langfuse.yaml: missing langfuse Deployment", file=sys.stderr)
    sys.exit(1)
if not any(doc.get("kind") == "Service" and doc.get("metadata", {}).get("name") == "langfuse" for doc in langfuse_docs):
    print("deploy/k8s/langfuse.yaml: missing langfuse Service", file=sys.stderr)
    sys.exit(1)

with open("deploy/k8s/secrets.example.yaml", encoding="utf-8") as f:
    secrets_text = f.read()
for required in ("LANGFUSE_DATABASE_URL", "NEXTAUTH_SECRET", "SALT", "ENCRYPTION_KEY", "observability-basic-auth"):
    if required not in secrets_text:
        print(f"deploy/k8s/secrets.example.yaml: missing {required}", file=sys.stderr)
        sys.exit(1)
PY

if rg -n "RequestURI|RawQuery|DumpRequest|Authorization|password|token" internal/observability; then
  echo "observability helpers must not log or inspect secrets, auth headers, bodies, or query strings" >&2
  exit 1
fi

if rg -n "password|password_hash|verification_code|oauth_token|credential" \
  api/user.api service/user/api/user.api service/user/rpc/user.proto cmd/user-api cmd/user-rpc \
  internal/model internal/logic internal/handler service/user/rpc internal/servicecontext; then
  echo "forbidden auth secret field found in service source" >&2
  exit 1
fi

if rg -n "password|password_hash|verification_code|oauth_token|credential" \
  internal/repository \
  --glob '!postgres_account_profiles_test.go'; then
  echo "forbidden auth secret field found in repository source" >&2
  exit 1
fi

if rg -n "password|password_hash|verification_code|oauth_token|credential" \
  api/message.api proto/message.proto \
  internal/logic/messagelogic.go internal/repository/message_memory.go \
  internal/repository/message_repository.go internal/handler/message; then
  echo "forbidden auth secret field found in message contract source" >&2
  exit 1
fi

if rg -ni "message service (owns|stores|manages|persists).*(password|password_hash|verification_code|oauth_token|credential|auth secret)|(password|password_hash|verification_code|oauth_token|credential|auth secret).*(owned by|stored in|managed by|persisted by) message service" \
  docs/product-specs/message-chain.md docs/design-docs/message-chain-contract.md "$message_plan_file"; then
  echo "message docs assign auth secrets to message service" >&2
  exit 1
fi

rg -q "Backend MVP" docs/product-specs/backend-mvp.md docs/design-docs/backend-mvp-contract.md
rg -q "message_received" docs/product-specs/backend-mvp.md docs/design-docs/backend-mvp-contract.md
rg -q "get_conversation_seqs" docs/product-specs/backend-mvp.md docs/design-docs/backend-mvp-contract.md
rg -q "healthz" docs/product-specs/backend-mvp.md docs/design-docs/backend-mvp-contract.md
rg -q "readyz" docs/product-specs/backend-mvp.md docs/design-docs/backend-mvp-contract.md

frontend_patterns=(
  "消息"
  "联系人"
  "发现"
  "我的"
  "role=\"tab\""
  "wechat-green"
)

frontend_files=(
  "web/src/App.tsx"
  "web/src/components/ui/NavigationBar.tsx"
  "web/src/components/ui/TabBar.tsx"
  "web/src/components/ContactsPage.tsx"
  "web/src/features/messages/MessagesPage.tsx"
  "web/src/pages/DiscoverPage.tsx"
  "web/src/styles.css"
)

for pattern in "${frontend_patterns[@]}"; do
  rg -qF "$pattern" "${frontend_files[@]}"
done

bash scripts/test-deploy-k3s.sh
bash scripts/test-no-latest-images.sh

if rg -q "mockData|mockConversations|mode=\"mock\"|sendMessageWithMock|cloneMockConversations" web/src --glob "*.ts" --glob "*.tsx"; then
  echo "frontend production mock flow found" >&2
  exit 1
fi

echo "static verification passed"
