#!/usr/bin/env bash
set -euo pipefail

required_files=(
  "api/user.api"
  "api/auth.api"
  "api/friends.api"
  "api/groups.api"
  "proto/user.proto"
  "proto/auth.proto"
  "proto/friends.proto"
  "proto/groups.proto"
  "cmd/user-api/main.go"
  "cmd/user-rpc/main.go"
  "cmd/auth-api/main.go"
  "cmd/auth-rpc/main.go"
  "cmd/friends-api/main.go"
  "cmd/friends-rpc/main.go"
  "cmd/groups-api/main.go"
  "cmd/groups-rpc/main.go"
  "internal/logic/userlogic.go"
  "internal/logic/friendslogic.go"
  "internal/logic/groupslogic.go"
  "internal/model/friendship.go"
  "internal/model/group.go"
  "internal/repository/memory.go"
  "internal/repository/groups_memory.go"
  "internal/repository/groups_repository.go"
  "internal/handler/handler.go"
  "internal/handler/groups_handler.go"
  "internal/auth/logic/authlogic.go"
  "internal/auth/repository/memory.go"
  "internal/auth/handler/handler.go"
  "internal/auth/token/token.go"
  "internal/auth/useradapter/user_client.go"
  "internal/gateway/contract.go"
  "tests/user_service_test.go"
  "tests/auth_service_test.go"
  "tests/friends_service_test.go"
  "tests/groups_service_test.go"
  "tests/gateway_contract_test.go"
  "docs/product-specs/user-service.md"
  "docs/product-specs/auth-service.md"
  "docs/product-specs/friends-service.md"
  "docs/product-specs/groups-service.md"
  "docs/product-specs/gateway-message-contract.md"
  "docs/design-docs/user-service-go-zero.md"
  "docs/design-docs/auth-service-go-zero.md"
  "docs/design-docs/friends-service-go-zero.md"
  "docs/design-docs/groups-service-go-zero.md"
  "docs/design-docs/gateway-message-contract.md"
  "docs/exec-plans/active/user-service-go-zero.md"
  "docs/exec-plans/active/auth-service-go-zero.md"
  "docs/exec-plans/active/friends-service-go-zero.md"
  "docs/exec-plans/active/groups-service-go-zero.md"
  "docs/exec-plans/active/gateway-message-contract.md"
)

for file in "${required_files[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "missing required file: $file" >&2
    exit 1
  fi
done

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
rg -q "X-User-Id" internal/handler docs
rg -q "ExistsByIdentifier" internal/auth docs/design-docs/auth-service-go-zero.md docs/product-specs/auth-service.md
rg -q "CreateUser" internal/auth docs/design-docs/auth-service-go-zero.md docs/product-specs/auth-service.md
rg -q "PasswordHash" internal/auth/model/credential.go
rg -q "Salt" internal/auth/model/credential.go
rg -q "user-rpc" docs/design-docs/groups-service-go-zero.md docs/product-specs/groups-service.md

if rg -n "password|password_hash|verification_code|oauth_token|credential" \
  api/user.api proto/user.proto cmd/user-api cmd/user-rpc \
  internal/model internal/logic internal/repository internal/handler internal/rpc internal/svc; then
  echo "forbidden auth secret field found in service source" >&2
  exit 1
fi

echo "static verification passed"
