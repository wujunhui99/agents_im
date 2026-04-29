#!/usr/bin/env bash
set -euo pipefail

required_files=(
  "api/user.api"
  "api/auth.api"
  "api/friends.api"
  "api/groups.api"
  "api/message.api"
  "proto/user.proto"
  "proto/auth.proto"
  "proto/friends.proto"
  "proto/groups.proto"
  "proto/message.proto"
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
  "internal/logic/messagelogic.go"
  "internal/model/friendship.go"
  "internal/model/group.go"
  "internal/repository/memory.go"
  "internal/repository/groups_memory.go"
  "internal/repository/groups_repository.go"
  "internal/repository/message_memory.go"
  "internal/repository/message_repository.go"
  "internal/handler/handler.go"
  "internal/handler/groups_handler.go"
  "internal/handler/message_handler.go"
  "internal/auth/logic/authlogic.go"
  "internal/auth/repository/memory.go"
  "internal/auth/handler/handler.go"
  "internal/auth/token/token.go"
  "internal/auth/useradapter/user_client.go"
  "internal/gateway/contract.go"
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
  "docs/design-docs/gateway-message-contract.md"
  "docs/design-docs/read-receipts.md"
  "docs/exec-plans/active/user-service-go-zero.md"
  "docs/exec-plans/active/auth-service-go-zero.md"
  "docs/exec-plans/active/friends-service-go-zero.md"
  "docs/exec-plans/active/groups-service-go-zero.md"
  "docs/exec-plans/active/message-storage.md"
  "internal/repository/message_storage_contract.go"
  "docs/exec-plans/active/gateway-message-contract.md"
  "docs/exec-plans/active/read-receipts.md"
)

for file in "${required_files[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "missing required file: $file" >&2
    exit 1
  fi
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
rg -q "X-User-Id" internal/handler docs
rg -q "ExistsByIdentifier" internal/auth docs/design-docs/auth-service-go-zero.md docs/product-specs/auth-service.md
rg -q "CreateUser" internal/auth docs/design-docs/auth-service-go-zero.md docs/product-specs/auth-service.md
rg -q "PasswordHash" internal/auth/model/credential.go
rg -q "Salt" internal/auth/model/credential.go
rg -q "user-rpc" docs/design-docs/groups-service-go-zero.md docs/product-specs/groups-service.md
rg -q "client_msg_id" docs/product-specs/message-chain.md docs/design-docs/message-chain-contract.md "$message_plan_file"
rg -q "has_read_seq" docs/product-specs/message-chain.md docs/design-docs/message-chain-contract.md "$message_plan_file"

if rg -n "password|password_hash|verification_code|oauth_token|credential" \
  api/user.api proto/user.proto cmd/user-api cmd/user-rpc \
  internal/model internal/logic internal/repository internal/handler internal/rpc internal/svc; then
  echo "forbidden auth secret field found in service source" >&2
  exit 1
fi

if rg -n "password|password_hash|verification_code|oauth_token|credential" \
  api/message.api proto/message.proto \
  internal/logic/messagelogic.go internal/repository/message_memory.go \
  internal/repository/message_repository.go internal/handler/message_handler.go; then
  echo "forbidden auth secret field found in message contract source" >&2
  exit 1
fi

if rg -ni "message service (owns|stores|manages|persists).*(password|password_hash|verification_code|oauth_token|credential|auth secret)|(password|password_hash|verification_code|oauth_token|credential|auth secret).*(owned by|stored in|managed by|persisted by) message service" \
  docs/product-specs/message-chain.md docs/design-docs/message-chain-contract.md "$message_plan_file"; then
  echo "message docs assign auth secrets to message service" >&2
  exit 1
fi

echo "static verification passed"
