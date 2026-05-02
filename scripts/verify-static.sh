#!/usr/bin/env bash
set -euo pipefail

required_files=(
  "api/user.api"
  "api/auth.api"
  "api/friends.api"
  "api/groups.api"
  "api/message.api"
  "api/media.api"
  ".github/workflows/ci.yml"
  ".github/markdown-link-check.json"
  ".ai-context/zero-skills/SKILL.md"
  ".ai-context/zero-skills/references/goctl-commands.md"
  ".ai-context/zero-skills/references/rest-api-patterns.md"
  ".ai-context/zero-skills/references/rpc-patterns.md"
  ".ai-context/zero-skills/references/database-patterns.md"
  "docs/references/go-zero/codex-guide.md"
  "docs/exec-plans/active/goctl-refactor.md"
  "docs/exec-plans/active/ci-pipeline.md"
  "proto/user.proto"
  "proto/auth.proto"
  "proto/friends.proto"
  "proto/groups.proto"
  "proto/message.proto"
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
  "internal/rpcgen/user/user.v1.go"
  "internal/rpcgen/user/internal/server/user_service_server.go"
  "internal/rpcgen/auth/auth.v1.go"
  "internal/rpcgen/auth/internal/server/auth_service_server.go"
  "internal/rpcgen/friends/friends.v1.go"
  "internal/rpcgen/friends/internal/server/friends_service_server.go"
  "internal/rpcgen/groups/groups.v1.go"
  "internal/rpcgen/groups/internal/server/groups_service_server.go"
  "internal/rpcgen/message/message.go"
  "internal/rpcgen/message/internal/server/message_service_server.go"
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
  "cmd/gateway-ws/main.go"
  "cmd/message-transfer/main.go"
  "etc/gateway-ws.yaml"
  "etc/message-transfer.yaml"
  "etc/message-rpc.yaml"
  "internal/logic/userlogic.go"
  "internal/logic/friendslogic.go"
  "internal/logic/groupslogic.go"
  "internal/logic/messagelogic.go"
  "internal/logic/medialogic_test.go"
  "internal/logic/message_media_test.go"
  "internal/logic/user/gozero_logic.go"
  "internal/logic/friends/gozero_logic.go"
  "internal/logic/groups/gozero_logic.go"
  "internal/logic/message/gozero_logic.go"
  "internal/logic/media/gozero_logic.go"
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
  "internal/repository/delivery_attempt_memory.go"
  "internal/repository/postgres_message.go"
  "internal/repository/postgres_outbox.go"
  "internal/repository/postgres_delivery_attempt.go"
  "internal/repository/delivery_attempt_repository_test.go"
  "internal/handler/health_handler.go"
  "internal/handler/gozero_routes.go"
  "internal/handler/media/create_upload_intent_handler.go"
  "internal/handler/media/complete_upload_handler.go"
  "internal/handler/media/get_download_url_handler.go"
  "internal/handler/user/update_me_avatar_handler.go"
  "internal/health/health.go"
  "internal/health/health_test.go"
  "internal/observability/metrics.go"
  "internal/observability/metrics_test.go"
  "internal/observability/trace.go"
  "internal/observability/trace_test.go"
  "internal/types/types.go"
  "internal/auth/logic/authlogic.go"
  "internal/auth/logic/auth/gozero_logic.go"
  "internal/auth/repository/memory.go"
  "internal/auth/repository/postgres.go"
  "internal/auth/handler/health_handler.go"
  "internal/auth/handler/gozero_routes.go"
  "internal/auth/token/token.go"
  "internal/auth/useradapter/user_client.go"
  "internal/ctxuser/user.go"
  "internal/rpcgen/user/entry/entry.go"
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
  "scripts/dev-up.sh"
  "scripts/dev-demo-data.sh"
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

if rg -n "(@material/web|@mui/)" web/package.json web/package-lock.json web/src; then
  echo "frontend must not introduce Material Web or MUI heavy dependencies" >&2
  exit 1
fi

ci_workflow_patterns=(
  "actions/checkout"
  "actions/setup-go"
  "go install github.com/zeromicro/go-zero/tools/goctl"
  "protobuf-compiler"
  "protoc-gen-go"
  "protoc-gen-go-grpc"
  "goctl api validate"
  "go test ./..."
  "bash scripts/verify-static.sh"
  "docker compose config"
  "markdown-link-check"
  "postgres:16-alpine"
  "scripts/migrate-postgres.sh --host-psql"
  "go test -tags=integration ./tests"
)

for pattern in "${ci_workflow_patterns[@]}"; do
  rg -qF "$pattern" .github/workflows/ci.yml
done

ci_doc_patterns=(
  "CI Pipeline"
  "goctl api validate"
  "go test ./..."
  "bash scripts/verify-static.sh"
  "docker compose config"
  "markdown-link-check"
)

for pattern in "${ci_doc_patterns[@]}"; do
  rg -qF "$pattern" docs/exec-plans/active/ci-pipeline.md docs/GIT_WORKFLOW.md
done

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

export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
if ! command -v goctl >/dev/null 2>&1; then
  echo "goctl is required for api validation" >&2
  exit 1
fi
goctl --version >/dev/null
for api_file in api/*.api; do
  goctl api validate -api "$api_file" >/dev/null
done

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
  "message_idempotency_keys"
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
)

for pattern in "${proto_patterns[@]}"; do
  rg -q "$pattern" proto/user.proto
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
  "AI/Agent"
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
  "internal/rpcgen/user"
  "internal/rpcgen/auth"
  "internal/rpcgen/friends"
  "internal/rpcgen/groups"
  "internal/rpcgen/message"
)

for dir in "${rpc_generated_dirs[@]}"; do
  if [[ ! -d "$dir/internal/server" || ! -d "$dir/internal/logic" || ! -d "$dir/internal/svc" ]]; then
    echo "missing goctl rpc generated scaffold under: $dir" >&2
    exit 1
  fi
done

rpc_generated_servers=(
  "internal/rpcgen/user/internal/server/user_service_server.go:UserServiceServer"
  "internal/rpcgen/auth/internal/server/auth_service_server.go:AuthServiceServer"
  "internal/rpcgen/friends/internal/server/friends_service_server.go:FriendsServiceServer"
  "internal/rpcgen/groups/internal/server/groups_service_server.go:GroupsServiceServer"
  "internal/rpcgen/message/internal/server/message_service_server.go:MessageServiceServer"
)

for server_spec in "${rpc_generated_servers[@]}"; do
  file="${server_spec%%:*}"
  server_name="${server_spec##*:}"
  rg -q "Code generated by goctl. DO NOT EDIT." "$file"
  rg -q "type ${server_name} struct" "$file"
  rg -q "Unimplemented${server_name}" "$file"
done

rpc_generated_entrypoints=(
  "internal/rpcgen/user/user.v1.go:RegisterUserServiceServer"
  "internal/rpcgen/auth/auth.v1.go:RegisterAuthServiceServer"
  "internal/rpcgen/friends/friends.v1.go:RegisterFriendsServiceServer"
  "internal/rpcgen/groups/groups.v1.go:RegisterGroupsServiceServer"
  "internal/rpcgen/message/message.go:RegisterMessageServiceServer"
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
  "cmd/user-rpc/main.go:internal/rpcgen/user/entry"
  "cmd/auth-rpc/main.go:internal/rpcgen/auth/entry"
  "cmd/friends-rpc/main.go:internal/rpcgen/friends/entry"
  "cmd/groups-rpc/main.go:internal/rpcgen/groups/entry"
  "cmd/message-rpc/main.go:internal/rpcgen/message/entry"
)

for entry_spec in "${rpc_entry_patterns[@]}"; do
  file="${entry_spec%%:*}"
  pattern="${entry_spec##*:}"
  rg -q "$pattern" "$file"
done

if rg -n "todo: add your logic here|return &.*Response\\{\\}, nil" internal/rpcgen/*/internal/logic; then
  echo "generated rpc logic still contains empty scaffold behavior" >&2
  exit 1
fi

rpc_logic_markers=(
  "internal/rpcgen/user/internal/logic:UserLogic"
  "internal/rpcgen/auth/internal/logic:AuthLogic"
  "internal/rpcgen/friends/internal/logic:FriendsLogic"
  "internal/rpcgen/groups/internal/logic:GroupsLogic"
  "internal/rpcgen/message/internal/logic:MessageLogic"
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

delivery_reliability_code_patterns=(
  "DeliveryStatusAccepted"
  "DeliveryStatusPublished"
  "DeliveryStatusDelivered"
  "DeliveryStatusOffline"
  "DeliveryStatusFailed"
  "type DeliveryAttemptRepository interface"
  "CreateDeliveryAttemptsAccepted"
  "MarkDeliveryAttemptsPublished"
  "RecordDeliveryAttemptResult"
  "ListDeliveryAttemptsByMessage"
  "type DeliveryAttemptRecorder interface"
  "NewRepositoryDeliveryAttemptRecorder"
  "RecipientDeliveryResult"
  "delivery_attempts"
)

for pattern in "${delivery_reliability_code_patterns[@]}"; do
  rg -q "$pattern" internal/repository internal/transfer db/migrations/001_init_postgres.sql
done

delivery_reliability_test_patterns=(
  "TestMemoryMessageRepositoryCreatesAcceptedDeliveryAttempt"
  "TestMemoryMessageRepositoryMarksDeliveryAttemptPublishedWithOutbox"
  "TestMemoryMessageRepositoryRecordsDeliveryAttemptResults"
  "TestWorkerRecordsDeliveredDeliveryAttempt"
  "TestWorkerRecordsOfflineDeliveryAttempt"
  "TestWorkerRecordsRetryableFailedDeliveryAttempt"
  "TestWorkerRecordsTerminalFailedDeliveryAttemptAtMaxAttempts"
)

for pattern in "${delivery_reliability_test_patterns[@]}"; do
  rg -q "$pattern" internal/repository/delivery_attempt_repository_test.go internal/transfer/gateway/dispatcher_test.go
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
rg -q "ctxuser\\.UserID" internal/logic/user/gozero_logic.go internal/logic/friends/gozero_logic.go internal/logic/groups/gozero_logic.go internal/logic/message/gozero_logic.go internal/logic/media/gozero_logic.go
rg -q "sender_id must match authenticated user" internal/logic/message/gozero_logic.go

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
  "account_type=0|1|2"
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
  "account_type=0|1|2"
)

for pattern in "${account_terminology_entry_patterns[@]}"; do
  rg -qF "$pattern" AGENTS.md ARCHITECTURE.md docs/product-specs/account-social-core.md docs/product-specs/frontend-backend-contract.md docs/design-docs/user-auth-friends-groups-boundaries.md docs/design-docs/user-service-go-zero.md docs/product-specs/user-service.md
done

account_code_patterns=(
  "AccountTypeUser  AccountType = 1"
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
  rg -qF "$pattern" internal/model/user.go internal/repository/repository.go internal/repository/memory.go internal/repository/postgres_user_friends.go internal/logic/userlogic.go internal/svc/service_context.go internal/handler/gozero_routes.go
done

account_storage_patterns=(
  "create table if not exists accounts"
  "create table if not exists profiles"
  "account_id text primary key"
  "account_id ~ '^[0-9]+$'"
  "account_type integer not null default 1"
  "account_type in (0, 1, 2)"
)

for pattern in "${account_storage_patterns[@]}"; do
  rg -qF "$pattern" db/migrations/001_init_postgres.sql
done

rg -qF "account_type?: 0 | 1 | 2" web/src/api/user.ts
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
  echo "account_type docs/frontend must use numeric 0(admin)|1(user)|2(agent); normal may appear only in explicit migration compatibility docs" >&2
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
  "message_idempotency_keys"
  "message_outbox"
  "delivery_attempts"
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
  "server_msg_id"
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
  rg -q "TraceMiddlewareFunc" "$api_main"
done

observability_wiring_patterns=(
  "/readyz"
  "/metrics"
  "ReadinessHandler"
  "MetricsHandler"
)

for pattern in "${observability_wiring_patterns[@]}"; do
  rg -q "$pattern" internal/handler/gozero_routes.go internal/auth/handler/gozero_routes.go cmd/gateway-ws/main.go cmd/message-transfer/main.go
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

if rg -n "RequestURI|RawQuery|DumpRequest|Authorization|password|token" internal/observability; then
  echo "observability helpers must not log or inspect secrets, auth headers, bodies, or query strings" >&2
  exit 1
fi

if rg -n "password|password_hash|verification_code|oauth_token|credential" \
  api/user.api proto/user.proto cmd/user-api cmd/user-rpc \
  internal/model internal/logic internal/handler internal/rpcgen/user internal/svc; then
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

if rg -q "mockData|mockConversations|mode=\"mock\"|sendMessageWithMock|cloneMockConversations" web/src --glob "*.ts" --glob "*.tsx"; then
  echo "frontend production mock flow found" >&2
  exit 1
fi

echo "static verification passed"
