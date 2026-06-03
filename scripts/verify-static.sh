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
  "api/message.api"
  "api/media.api"
  ".drone.yml"
  ".github/markdown-link-check.json"
  ".ai-context/zero-skills/SKILL.md"
  ".ai-context/zero-skills/references/goctl-commands.md"
  ".ai-context/zero-skills/references/rest-api-patterns.md"
  ".ai-context/zero-skills/references/rpc-patterns.md"
  ".ai-context/zero-skills/references/database-patterns.md"
  "service/user/rpc/user.proto"
  "service/user/api/user.api"
  "service/auth/rpc/auth.proto"
  "service/auth/api/auth.api"
  "service/friends/rpc/friends.proto"
  "service/friends/api/friends.api"
  "service/groups/rpc/groups.proto"
  "service/groups/api/groups.api"
  "service/agent/api/agent.api"
  "internal/rpcgen/message/message.proto"
  "service/mail/rpc/mail.proto"
  "service/user/rpc/user/user.pb.go"
  "service/user/rpc/user/user_grpc.pb.go"
  "service/auth/rpc/auth/auth.pb.go"
  "service/auth/rpc/auth/auth_grpc.pb.go"
  "service/friends/rpc/friends/friends.pb.go"
  "service/friends/rpc/friends/friends_grpc.pb.go"
  "service/groups/rpc/groups/groups.pb.go"
  "service/groups/rpc/groups/groups_grpc.pb.go"
  "internal/rpcgen/message/messagepb/message.pb.go"
  "internal/rpcgen/message/messagepb/message_grpc.pb.go"
  "service/mail/rpc/mail/mail.pb.go"
  "service/mail/rpc/mail/mail_grpc.pb.go"
  "service/user/rpc/user.go"
  "service/user/rpc/internal/server/userserver.go"
  "service/auth/rpc/auth.go"
  "service/auth/rpc/internal/server/auth_service_server.go"
  "service/friends/rpc/friends.go"
  "service/friends/rpc/internal/server/friendsserver.go"
  "service/groups/rpc/groups.go"
  "service/groups/rpc/internal/server/groupsserver.go"
  "service/user/api/user.go"
  "service/auth/api/auth.go"
  "service/friends/api/friends.go"
  "service/groups/api/groups.go"
  "service/agent/api/agent.go"
  "service/user/api/internal/config/config.go"
  "service/user/api/internal/handler/routes.go"
  "service/user/api/internal/svc/service_context.go"
  "service/user/api/internal/types/types.go"
  "service/auth/api/internal/config/config.go"
  "service/auth/api/internal/handler/routes.go"
  "service/auth/api/internal/svc/service_context.go"
  "service/auth/api/internal/types/types.go"
  "service/auth/api/internal/logic/auth/request_registration_email_code_logic.go"
  "service/auth/api/internal/logic/auth/register_logic.go"
  "service/auth/api/internal/logic/auth/login_logic.go"
  "service/auth/api/internal/logic/auth/validate_token_logic.go"
  "service/auth/api/internal/logic/auth/convert.go"
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
  "service/friends/api/internal/config/config.go"
  "service/friends/api/internal/handler/routes.go"
  "service/friends/api/internal/svc/service_context.go"
  "service/friends/api/internal/types/types.go"
  "service/friends/api/internal/logic/friends/add_friend_logic.go"
  "service/friends/api/internal/logic/friends/delete_friend_logic.go"
  "service/friends/api/internal/logic/friends/get_friendship_logic.go"
  "service/friends/api/internal/logic/friends/list_friends_logic.go"
  "service/friends/api/internal/logic/friends/list_friend_requests_logic.go"
  "service/friends/api/internal/logic/friends/accept_friend_request_logic.go"
  "service/friends/api/internal/logic/friends/reject_friend_request_logic.go"
  "service/friends/api/internal/logic/friends/convert.go"
  "service/groups/api/internal/config/config.go"
  "service/groups/api/internal/handler/routes.go"
  "service/groups/api/internal/svc/service_context.go"
  "service/groups/api/internal/types/types.go"
  "service/groups/api/internal/logic/groups/create_group_logic.go"
  "service/groups/api/internal/logic/groups/list_groups_logic.go"
  "service/groups/api/internal/logic/groups/get_group_logic.go"
  "service/groups/api/internal/logic/groups/update_group_logic.go"
  "service/groups/api/internal/logic/groups/add_member_logic.go"
  "service/groups/api/internal/logic/groups/leave_group_logic.go"
  "service/groups/api/internal/logic/groups/kick_member_logic.go"
  "service/groups/api/internal/logic/groups/list_members_logic.go"
  "service/groups/api/internal/logic/groups/convert.go"
  "service/agent/api/internal/config/config.go"
  "service/agent/api/internal/handler/routes.go"
  "service/agent/api/internal/svc/service_context.go"
  "service/agent/api/internal/types/types.go"
  "service/agent/api/internal/logic/agent/create_agent_logic.go"
  "service/agent/api/internal/logic/agent/get_agent_logic.go"
  "service/agent/api/internal/logic/agent/list_agents_logic.go"
  "service/agent/api/internal/logic/agent/update_agent_logic.go"
  "service/agent/api/internal/logic/agent/update_agent_status_logic.go"
  "service/agent/api/internal/logic/agent/delete_agent_logic.go"
  "service/agent/api/internal/logic/agent/get_agent_definition_logic.go"
  "service/agent/api/internal/logic/agent/update_agent_definition_logic.go"
  "service/agent/api/internal/logic/agent/convert.go"
  "internal/rpcgen/message/message.go"
  "internal/rpcgen/message/internal/server/message_service_server.go"
  "service/mail/rpc/mail.go"
  "service/mail/rpc/internal/server/mail_service_server.go"
  "service/gateway-ws/main.go"
  "service/message-api/main.go"
  "service/message-transfer/main.go"
  "etc/gateway-ws.yaml"
  "etc/message-transfer.yaml"
  "etc/message-rpc.yaml"
  "etc/mail-rpc.yaml"
  "internal/mail/provider.go"
  "internal/mail/config.go"
  "internal/mail/tencent_ses.go"
  "internal/logic/userlogic.go"
  "internal/logic/groupslogic.go"
  "internal/logic/messagelogic.go"
  "internal/logic/message_media_test.go"
  "internal/logic/message/send_message_logic.go"
  "internal/logic/message/pull_messages_logic.go"
  "internal/logic/message/get_conversation_seqs_logic.go"
  "internal/logic/message/mark_conversation_as_read_logic.go"
  "internal/logic/message/get_conversation_a_i_hosting_logic.go"
  "internal/logic/message/update_conversation_a_i_hosting_logic.go"
  "internal/logic/message/convert.go"
  "pkg/objectstorage/store.go"
  "pkg/objectstorage/minio.go"
  "pkg/objectstorage/memory.go"
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
  "internal/handler/message/get_conversation_a_i_hosting_handler.go"
  "internal/handler/message/update_conversation_a_i_hosting_handler.go"
  "internal/servicecontext/common/auth.go"
  "internal/servicecontext/message/service_context.go"
  "internal/servicecontext/gateway/service_context.go"
  "pkg/health/health.go"
  "pkg/health/health_test.go"
  "pkg/observability/metrics.go"
  "pkg/observability/metrics_test.go"
  "pkg/observability/trace.go"
  "pkg/observability/trace_test.go"
  "pkg/llmobs/types.go"
  "pkg/llmobs/sink.go"
  "pkg/llmobs/sink_test.go"
  "pkg/llmobs/sanitize.go"
  "pkg/llmobs/eino.go"
  "internal/agenteval/eval.go"
  "internal/agenteval/python_go_test.go"
  "internal/auth/logic/authlogic.go"
  "internal/auth/repository/memory.go"
  "internal/auth/repository/postgres.go"
  "internal/auth/useradapter/user_client.go"
  "pkg/ctxuser/user.go"
  "service/user/rpc/user.go"
  "service/auth/rpc/auth.go"
  "service/groups/rpc/groups.go"
  "internal/rpcgen/message/message.go"
  "internal/gateway/contract.go"
  "internal/transfer/event.go"
  "internal/transfer/interfaces.go"
  "internal/transfer/idempotency.go"
  "internal/transfer/delivery_attempt_recorder.go"
  "internal/transfer/memory.go"
  "internal/transfer/worker.go"
  "internal/transfer/worker_test.go"
  "pkg/presence/store.go"
  "pkg/presence/memory.go"
  "pkg/presence/redis.go"
  "pkg/presence/memory_test.go"
  "pkg/presence/redis_integration_test.go"
  "internal/gateway/ws/connection_manager.go"
  "internal/gateway/ws/server.go"
  "pkg/messaging/event.go"
  "pkg/messaging/event_test.go"
  "internal/gateway/delivery/delivery.go"
  "internal/gateway/ws/delivery.go"
  "internal/transfer/gateway/dispatcher.go"
  "internal/transfer/gateway/dispatcher_test.go"
  "internal/domain/readreceipt/read_receipt.go"
  "pkg/pythonexec/executor.go"
  "pkg/pythonexec/executor_test.go"
  "tests/message_service_test.go"
  "tests/gateway_contract_test.go"
  "tests/websocket_gateway_test.go"
  "tests/read_receipts_test.go"
  "tests/no_shell_execution_test.go"
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

root_svc_import_files="$(rg -l '"github.com/wujunhui99/agents_im/internal/svc"' service/gateway-ws service/message-api service/message-transfer internal/handler internal/logic internal/gateway tests --glob '*.go' || true)"
if [[ -n "${root_svc_import_files}" ]]; then
  echo "core REST, gateway, and tests must not import legacy root internal/svc:" >&2
  echo "${root_svc_import_files}" >&2
  exit 1
fi

if rg -n '"os/exec"|exec\.Command|CommandContext\(|"(/bin/bash|/bin/sh|bash|sh|python|python3)"' service/gateway-ws service/message-api service/message-transfer internal --glob '*.go' --glob '!*_test.go'; then
  echo "production Go code must not directly execute shell or python commands" >&2
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
  rg -q "$pattern" service/user/api/user.api
done

auth_api_patterns=(
  "post /auth/register"
  "post /auth/login"
  "post /auth/validate"
)

for pattern in "${auth_api_patterns[@]}"; do
  rg -q "$pattern" service/auth/api/auth.api
done

friends_api_patterns=(
  "post /friends"
  "delete /friends/:user_id"
  "get /friends"
  "get /friends/:user_id"
)

for pattern in "${friends_api_patterns[@]}"; do
  rg -q "$pattern" service/friends/api/friends.api
done

groups_api_patterns=(
  "post /groups"
  "get /groups/:group_id"
  "post /groups/:group_id/members"
  "delete /groups/:group_id/members/me"
  "get /groups/:group_id/members"
)

for pattern in "${groups_api_patterns[@]}"; do
  rg -q "$pattern" service/groups/api/groups.api
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




dev_script_patterns=(
  "docker compose up -d postgres redis minio"
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
  rg -q "$pattern" service/auth/rpc/auth.proto
done

friends_proto_patterns=(
  "rpc AddFriend"
  "rpc DeleteFriend"
  "rpc ListFriends"
  "rpc GetFriendship"
)

for pattern in "${friends_proto_patterns[@]}"; do
  rg -q "$pattern" service/friends/rpc/friends.proto
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
  rg -q "$pattern" service/groups/rpc/groups.proto
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
  rg -q "$pattern" internal/rpcgen/message/message.proto
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
  rg -q "$pattern" service/mail/rpc/mail.proto
done

agent_conversation_hosting_contract_patterns=(
  "message_origin"
  "agent_account_id"
  "trigger_server_msg_id"
  "agent_run_id"
  "allow_recursive_trigger"
)

for pattern in "${agent_conversation_hosting_contract_patterns[@]}"; do
  rg -q "$pattern" internal/rpcgen/message/message.proto db/migrations/003_agent_conversation_hosting.sql internal/repository/message_repository.go pkg/messaging/event.go
done

agent_conversation_hosting_camel_patterns=(
  "messageOrigin"
  "agentAccountId"
  "triggerServerMsgId"
  "agentRunId"
  "allowRecursiveTrigger"
)

for pattern in "${agent_conversation_hosting_camel_patterns[@]}"; do
  rg -q "$pattern" api/message.api web/src/api/messages.ts web/src/models/messages.ts web/src/features/messages/MessagesPage.tsx
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
  rg -q "$pattern" internal/logic/messagelogic.go internal/agentim internal/repository db/migrations/003_agent_conversation_hosting.sql
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


if rg -n "CreateMessageIdempotent|insert into messages|insertMessage" internal/agentim --glob '*.go'; then
  echo "agentim must write responses through MessageLogic/Message Service, not message repository or direct DB insert" >&2
  exit 1
fi

rpc_generated_dirs=(
  "service/user/rpc"
  "service/auth/rpc"
  "service/friends/rpc"
  "service/groups/rpc"
  "internal/rpcgen/message"
  "service/mail/rpc"
)

for dir in "${rpc_generated_dirs[@]}"; do
  if [[ ! -d "$dir/internal/server" || ! -d "$dir/internal/logic" || ! -d "$dir/internal/svc" ]]; then
    echo "missing goctl rpc generated scaffold under: $dir" >&2
    exit 1
  fi
done

rpc_generated_servers=(
  "service/user/rpc/internal/server/userserver.go:UserServer"
  "service/auth/rpc/internal/server/auth_service_server.go:AuthServiceServer"
  "service/friends/rpc/internal/server/friendsserver.go:FriendsServer"
  "service/groups/rpc/internal/server/groupsserver.go:GroupsServer"
  "internal/rpcgen/message/internal/server/message_service_server.go:MessageServiceServer"
  "service/mail/rpc/internal/server/mail_service_server.go:MailServiceServer"
)

for server_spec in "${rpc_generated_servers[@]}"; do
  file="${server_spec%%:*}"
  server_name="${server_spec##*:}"
  rg -q "Code generated by goctl. DO NOT EDIT." "$file"
  rg -q "type ${server_name} struct" "$file"
  rg -q "Unimplemented${server_name}" "$file"
done

# RPC service mains live in their service dir (cmd/ and the entry bridge were removed)
# and wire the goctl-generated RPC server directly.
rpc_generated_entrypoints=(
  "service/user/rpc/user.go:RegisterUserServer"
  "service/auth/rpc/auth.go:RegisterAuthServiceServer"
  "service/friends/rpc/friends.go:RegisterFriendsServer"
  "service/groups/rpc/groups.go:RegisterGroupsServer"
  "internal/rpcgen/message/message.go:RegisterMessageServiceServer"
  "service/mail/rpc/mail.go:RegisterMailServiceServer"
)

for entrypoint_spec in "${rpc_generated_entrypoints[@]}"; do
  file="${entrypoint_spec%%:*}"
  register_func="${entrypoint_spec##*:}"
  rg -q "$register_func" "$file"
done

for main_file in service/user/rpc/user.go service/auth/rpc/auth.go service/friends/rpc/friends.go service/groups/rpc/groups.go service/mail/rpc/mail.go internal/rpcgen/message/message.go; do
  if rg -n '"github.com/wujunhui99/agents_im/internal/(auth/)?rpc"' "$main_file"; then
    echo "rpc entrypoint imports old handwritten rpc wrapper: $main_file" >&2
    exit 1
  fi
done

# API service mains wire the goctl-generated handlers directly.
api_register_patterns=(
  "service/user/api/user.go:handler.RegisterHandlers"
  "service/auth/api/auth.go:handler.RegisterHandlers"
  "service/friends/api/friends.go:handler.RegisterHandlers"
  "service/groups/api/groups.go:handler.RegisterHandlers"
  "service/agent/api/agent.go:handler.RegisterHandlers"
)

for entry_spec in "${api_register_patterns[@]}"; do
  file="${entry_spec%%:*}"
  pattern="${entry_spec##*:}"
  rg -q "$pattern" "$file"
done

if rg -n '"github.com/wujunhui99/agents_im/internal/(repository|model|objectstorage|servicecontext/user)"|DataSource|StorageDriver|New.*Repository' service/user/api --glob '*.go' --glob '!*_test.go'; then
  echo "service/user/api must not own data access; use RPC/BFF calls" >&2
  exit 1
fi

if rg -n '"github.com/wujunhui99/agents_im/internal/(repository|model|objectstorage|auth/repository|servicecontext/auth)"|DataSource|StorageDriver|New.*Repository' service/auth/api --glob '*.go' --glob '!*_test.go'; then
  echo "service/auth/api must not own data access; use RPC/BFF calls" >&2
  exit 1
fi

if rg -n '"github.com/wujunhui99/agents_im/internal/(repository|model|objectstorage|servicecontext/groups)"|DataSource|StorageDriver|New.*Repository' service/groups/api --glob '*.go' --glob '!*_test.go'; then
  echo "service/groups/api must not own data access; use RPC/BFF calls" >&2
  exit 1
fi

if rg -n '"github.com/wujunhui99/agents_im/internal/(repository|model|objectstorage|servicecontext/friends)"|DataSource|StorageDriver|New.*Repository' service/friends/api --glob '*.go' --glob '!*_test.go'; then
  echo "service/friends/api must not own data access; use RPC/BFF calls" >&2
  exit 1
fi

if rg -n "todo: add your logic here|return &.*Response\\{\\}, nil" service/user/api/internal/logic; then
  echo "generated user API logic still contains empty scaffold behavior" >&2
  exit 1
fi

if rg -n "todo: add your logic here|return &.*Response\\{\\}, nil" service/auth/api/internal/logic; then
  echo "generated auth API logic still contains empty scaffold behavior" >&2
  exit 1
fi

if rg -n "todo: add your logic here|return &.*Response\\{\\}, nil" service/friends/api/internal/logic; then
  echo "generated friends API logic still contains empty scaffold behavior" >&2
  exit 1
fi

if rg -n "todo: add your logic here|return &.*Response\\{\\}, nil" service/agent/api/internal/logic; then
  echo "generated agent API logic still contains empty scaffold behavior" >&2
  exit 1
fi

if rg -n '"github.com/wujunhui99/agents_im/internal/(logic|repository|auth/logic|auth/repository)"' internal/rpcgen/message/message.go service/user/rpc/user.go service/auth/rpc/auth.go service/friends/rpc/friends.go service/groups/rpc/groups.go service/mail/rpc/mail.go; then
  echo "rpc service mains must not own business wiring; keep dependencies behind generated rpc service contexts" >&2
  exit 1
fi

if rg -n "todo: add your logic here|return &.*Response\\{\\}, nil" internal/rpcgen/*/internal/logic service/user/rpc/internal/logic service/auth/rpc/internal/logic service/mail/rpc/internal/logic; then
  echo "generated rpc logic still contains empty scaffold behavior" >&2
  exit 1
fi

rpc_logic_markers=(
  "service/user/rpc/internal/logic:UserLogic"
  "service/auth/rpc/internal/logic:AuthLogic"
  "service/friends/rpc/internal/logic:FriendsLogic"
  "internal/rpcgen/message/internal/logic:MessageLogic"
  "service/mail/rpc/internal/logic:MailProvider"
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
  "service/user/rpc/user/user.pb.go"
  "service/user/rpc/user/user_grpc.pb.go"
  "service/auth/rpc/auth/auth.pb.go"
  "service/auth/rpc/auth/auth_grpc.pb.go"
  "internal/rpcgen/message/messagepb/message.pb.go"
  "internal/rpcgen/message/messagepb/message_grpc.pb.go"
  "service/mail/rpc/mail/mail.pb.go"
  "service/mail/rpc/mail/mail_grpc.pb.go"
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
  rg -q "$pattern" internal/gateway/contract.go tests/gateway_contract_test.go
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

rg -q "gateway-ws" service/gateway-ws/main.go etc/gateway-ws.yaml
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







rg -q "LoadMessageTransferConfig" pkg/config/config.go
rg -q "message-transfer" service/message-transfer/main.go etc/message-transfer.yaml
rg -q "ConsumerGroup|Consumer\\.Group" etc/message-transfer.yaml pkg/config/config.go
rg -q "Topic|Consumer\\.Topic" etc/message-transfer.yaml pkg/config/config.go
rg -q "WorkerID|Worker\\.ID" etc/message-transfer.yaml pkg/config/config.go

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
  rg -q "$pattern" internal/gateway/ws internal/gateway/delivery pkg/presence tests/websocket_gateway_test.go
done

rg -q "Presence:" etc/gateway-ws.yaml


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
  rg -q "$pattern" pkg/config/config.go
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
  rg -q "$pattern" pkg/presence
done

rg -q "REDIS_ADDR is required.*skip|t\\.Skip" pkg/presence/redis_integration_test.go

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
  rg -q "$pattern" pkg/messaging/event.go pkg/messaging/event_test.go
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

if rg -n "X-User-Id|CurrentUserID|currentUserID" api internal service/gateway-ws service/message-api service/message-transfer; then
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
    "etc/auth-api.yaml",
):
    with open(path, encoding="utf-8") as f:
        data = yaml.safe_load(f) or {}
    auth_rpc = data.get("AuthRPC")
    if not isinstance(auth_rpc, dict):
        print(f"{path}: AuthRPC section is required", file=sys.stderr)
        sys.exit(1)
    endpoints = auth_rpc.get("Endpoints")
    if not isinstance(endpoints, list) or not endpoints:
        print(f"{path}: AuthRPC.Endpoints must be a non-empty YAML list", file=sys.stderr)
        sys.exit(1)
    for index, endpoint in enumerate(endpoints):
        if not isinstance(endpoint, str) or not endpoint.strip():
            print(f"{path}: AuthRPC.Endpoints[{index}] must be a non-empty string", file=sys.stderr)
            sys.exit(1)

for path in (
    "deploy/k8s/etc/auth-rpc.yaml",
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
  "service/user/api/user.api"
  "service/friends/api/friends.api"
  "service/groups/api/groups.api"
  "api/message.api"
  "api/media.api"
  "service/agent/api/agent.api"
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

rg -q "type JWTAuthConfig" pkg/config/config.go
rg -q "AccessSecret" pkg/config/config.go service/auth/rpc/internal/config/config.go
rg -q "AccessExpire" pkg/config/config.go service/auth/rpc/internal/config/config.go
rg -q "user_id" pkg/ctxuser/user.go
rg -q "ctxuser\\.UserID" internal/logic/message
rg -q "sender_id must match authenticated user" internal/logic/message/send_message_logic.go

jwt_test_patterns=(
  "bearerTokenForUser"
  "legacy X-User-Id rejection"
  "invalid token status"
  "message sender did not use token user"
)

for pattern in "${jwt_test_patterns[@]}"; do
  rg -q "$pattern" tests
done

rg -q "ExistsByIdentifier" internal/auth
rg -q "CreateUser" internal/auth
rg -q "PasswordHash" internal/auth/model/credential.go
rg -q "Salt" internal/auth/model/credential.go


social_mvp_code_patterns=(
  "CodeForbidden"
  "sender is not a group member"
  "group membership validator is not configured"
  "group owner cannot leave as the only active member"
)

for pattern in "${social_mvp_code_patterns[@]}"; do
  rg -q "$pattern" internal
done

social_mvp_test_patterns=(
  "TestMessageGroupSendRequiresActiveMembership"
  "client-group-outsider"
  "client-group-left"
)

for pattern in "${social_mvp_test_patterns[@]}"; do
  rg -q "$pattern" tests
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

if rg -n 'account_type.*normal|normal.*account_type|`normal`' \
  web/src/api/user.ts; then
  echo "account_type docs/frontend must use user|agent|admin; normal may appear only in explicit migration compatibility docs" >&2
  exit 1
fi

rg -q "NewGroupsRepositoryForStorage" service/message-api/main.go service/gateway-ws/main.go internal/rpcgen/message/internal/svc/service_context.go
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
  rg -q "$pattern" db/migrations/001_init_postgres.sql
done

rg -q "StorageDriver" pkg/config/config.go etc/*.yaml
rg -q "ObjectStorageConfig" pkg/config/config.go
rg -q "NewStore" pkg/objectstorage/factory.go
rg -q "PresignPut" pkg/objectstorage/store.go pkg/objectstorage/minio.go
rg -q "NewMediaRepositoryForStorage" internal/repository/postgres_common.go service/user/api/user.go service/message-api/main.go service/gateway-ws/main.go
rg -q "ValidateMessageMedia" internal/logic/messagelogic.go
rg -q "media_objects" db/migrations/001_init_postgres.sql
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
rg -q "docker compose" scripts/migrate-postgres.sh

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
  rg -q "$pattern" db/migrations/001_init_postgres.sql
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


# Outbox -> messaging.MessageEvent conversion is the live V1 fanout path consumed
# by message-transfer (the former Kafka publisher was removed as unused).
outbox_publisher_code_patterns=(
  "package outboxpublisher"
  "MessageEventFromOutbox"
  "messaging.MessageEvent"
  "EventTypeMessageAccepted"
)

for pattern in "${outbox_publisher_code_patterns[@]}"; do
  rg -q "$pattern" internal/outboxpublisher
done

rg -q "TestMessageEventFromOutboxBuildsAcceptedEvent" internal/outboxpublisher/message_event_test.go
rg -q "TestMessageEventFromOutboxIncludesAISenderInSingleChatReceiverIDs" internal/outboxpublisher/message_event_test.go
rg -q "TestMessageEventFromOutboxRejectsMalformedPayload" internal/outboxpublisher/message_event_test.go

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
  rg -q "$pattern" pkg/health pkg/observability
done

for api_main in service/user/api/user.go service/auth/api/auth.go service/friends/api/friends.go service/groups/api/groups.go service/agent/api/agent.go; do
  rg -q "TraceMiddlewareFunc" "$api_main"
done

observability_wiring_patterns=(
  "/readyz"
  "/metrics"
  "ReadinessHandler"
  "MetricsHandler"
)

for pattern in "${observability_wiring_patterns[@]}"; do
  rg -q "$pattern" internal/handler/gozero_routes.go service/user/api/user.go service/auth/api/auth.go service/friends/api/friends.go service/groups/api/groups.go service/agent/api/agent.go service/gateway-ws/main.go service/message-transfer/main.go
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
rg -q "agents_im_message_sends_total" pkg/observability/metrics.go
rg -q "trace_id" internal/gateway/ws/server.go

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
  rg -q "$pattern" pkg/llmobs internal/agenteval internal/agentim internal/agentruntime/eino pkg/config .env.example
done

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
    "/media": ("media-api", 8089),
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

if rg -n "RequestURI|RawQuery|DumpRequest|Authorization|password|token" pkg/observability; then
  echo "observability helpers must not log or inspect secrets, auth headers, bodies, or query strings" >&2
  exit 1
fi

if rg -n "password|password_hash|verification_code|oauth_token|credential" \
  service/user/api/user.api service/user/rpc/user.proto service/user/api/user.go \
  internal/logic internal/handler service/user/rpc internal/servicecontext; then
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
  api/message.api internal/rpcgen/message/message.proto \
  internal/logic/messagelogic.go internal/repository/message_memory.go \
  internal/repository/message_repository.go internal/handler/message; then
  echo "forbidden auth secret field found in message contract source" >&2
  exit 1
fi



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
