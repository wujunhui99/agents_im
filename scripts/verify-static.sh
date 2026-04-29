#!/usr/bin/env bash
set -euo pipefail

required_files=(
  "api/user.api"
  "api/auth.api"
  "api/friends.api"
  "api/groups.api"
  "api/message.api"
  ".ai-context/zero-skills/SKILL.md"
  ".ai-context/zero-skills/references/goctl-commands.md"
  ".ai-context/zero-skills/references/rest-api-patterns.md"
  ".ai-context/zero-skills/references/rpc-patterns.md"
  ".ai-context/zero-skills/references/database-patterns.md"
  "docs/references/go-zero/codex-guide.md"
  "docs/exec-plans/active/goctl-refactor.md"
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
  "etc/message-rpc.yaml"
  "internal/logic/userlogic.go"
  "internal/logic/friendslogic.go"
  "internal/logic/groupslogic.go"
  "internal/logic/messagelogic.go"
  "internal/logic/user/gozero_logic.go"
  "internal/logic/friends/gozero_logic.go"
  "internal/logic/groups/gozero_logic.go"
  "internal/logic/message/gozero_logic.go"
  "internal/model/friendship.go"
  "internal/model/group.go"
  "internal/repository/memory.go"
  "internal/repository/postgres_common.go"
  "internal/repository/postgres_user_friends.go"
  "internal/repository/groups_memory.go"
  "internal/repository/groups_repository.go"
  "internal/repository/postgres_groups.go"
  "internal/repository/message_memory.go"
  "internal/repository/message_repository.go"
  "internal/repository/postgres_message.go"
  "internal/handler/health_handler.go"
  "internal/handler/gozero_routes.go"
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
  "internal/presence/store.go"
  "internal/presence/memory.go"
  "internal/presence/redis.go"
  "internal/presence/memory_test.go"
  "internal/presence/redis_integration_test.go"
  "internal/domain/readreceipt/read_receipt.go"
  "tests/user_service_test.go"
  "tests/auth_service_test.go"
  "tests/friends_service_test.go"
  "tests/groups_service_test.go"
  "tests/message_service_test.go"
  "tests/gateway_contract_test.go"
  "tests/read_receipts_test.go"
  "docs/product-specs/user-service.md"
  "docs/product-specs/auth-service.md"
  "docs/product-specs/friends-service.md"
  "docs/product-specs/groups-service.md"
  "docs/product-specs/message-chain.md"
  "docs/product-specs/message-storage.md"
  "docs/product-specs/gateway-message-contract.md"
  "docs/product-specs/read-receipts.md"
  "docs/design-docs/user-service-go-zero.md"
  "docs/design-docs/auth-service-go-zero.md"
  "docs/design-docs/friends-service-go-zero.md"
  "docs/design-docs/groups-service-go-zero.md"
  "docs/design-docs/message-chain-contract.md"
  "docs/design-docs/message-storage.md"
  "docs/design-docs/jwt-auth-middleware.md"
  "docs/design-docs/postgres-persistence.md"
  "docs/design-docs/gateway-message-contract.md"
  "docs/design-docs/redis-presence.md"
  "docs/design-docs/read-receipts.md"
  "docs/exec-plans/active/user-service-go-zero.md"
  "docs/exec-plans/active/auth-service-go-zero.md"
  "docs/exec-plans/active/friends-service-go-zero.md"
  "docs/exec-plans/active/groups-service-go-zero.md"
  "docs/exec-plans/active/message-storage.md"
  "internal/repository/message_storage_contract.go"
  "docker-compose.yml"
  ".env.example"
  "db/migrations/001_init_postgres.sql"
  "scripts/migrate-postgres.sh"
  "tests/postgres_persistence_integration_test.go"
  "docs/exec-plans/active/gateway-message-contract.md"
  "docs/exec-plans/active/redis-presence.md"
  "docs/exec-plans/active/read-receipts.md"
  "docs/exec-plans/active/remove-handwritten-compat.md"
  "docs/exec-plans/active/jwt-auth-middleware.md"
)

for file in "${required_files[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "missing required file: $file" >&2
    exit 1
  fi
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
  "get /users/exists"
  "get /users/:identifier"
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
  "SendMessage"
  "PullMessages"
  "GetConversationSeqs"
  "MarkConversationAsRead"
)

for pattern in "${gateway_doc_patterns[@]}"; do
  rg -q "$pattern" docs/design-docs/gateway-message-contract.md
  rg -q "$pattern" internal/gateway/contract.go tests/gateway_contract_test.go
done

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
rg -q "ctxuser\\.UserID" internal/logic/user/gozero_logic.go internal/logic/friends/gozero_logic.go internal/logic/groups/gozero_logic.go internal/logic/message/gozero_logic.go
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
pg_persistence_patterns=(
  "users"
  "auth_credentials"
  "friendships"
  "groups"
  "group_members"
  "messages"
  "conversation_threads"
  "user_conversation_states"
  "message_idempotency_keys"
)

for pattern in "${pg_persistence_patterns[@]}"; do
  rg -q "$pattern" db/migrations/001_init_postgres.sql docs/design-docs/postgres-persistence.md
done

rg -q "StorageDriver" internal/config/config.go etc/*.yaml
rg -q "NewPostgresRepository" internal/repository/postgres_user_friends.go internal/auth/repository/postgres.go
rg -q "NewPostgresGroupsRepository" internal/repository/postgres_groups.go
rg -q "NewPostgresMessageRepository" internal/repository/postgres_message.go
rg -q "docker compose" scripts/migrate-postgres.sh docs/design-docs/postgres-persistence.md

if rg -n "password|password_hash|verification_code|oauth_token|credential" \
  api/user.api proto/user.proto cmd/user-api cmd/user-rpc \
  internal/model internal/logic internal/repository internal/handler internal/rpcgen/user internal/svc; then
  echo "forbidden auth secret field found in service source" >&2
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

echo "static verification passed"
