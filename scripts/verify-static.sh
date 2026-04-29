#!/usr/bin/env bash
set -euo pipefail

required_files=(
  "api/user.api"
  "api/friends.api"
  "proto/user.proto"
  "proto/friends.proto"
  "cmd/user-api/main.go"
  "cmd/user-rpc/main.go"
  "cmd/friends-api/main.go"
  "cmd/friends-rpc/main.go"
  "internal/logic/userlogic.go"
  "internal/logic/friendslogic.go"
  "internal/model/friendship.go"
  "internal/repository/memory.go"
  "internal/handler/handler.go"
  "tests/user_service_test.go"
  "tests/friends_service_test.go"
  "docs/product-specs/user-service.md"
  "docs/product-specs/friends-service.md"
  "docs/design-docs/user-service-go-zero.md"
  "docs/design-docs/friends-service-go-zero.md"
  "docs/exec-plans/active/user-service-go-zero.md"
  "docs/exec-plans/active/friends-service-go-zero.md"
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

friends_api_patterns=(
  "post /friends"
  "delete /friends/:user_id"
  "get /friends"
  "get /friends/:user_id"
)

for pattern in "${friends_api_patterns[@]}"; do
  rg -q "$pattern" api/friends.api
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

friends_proto_patterns=(
  "rpc AddFriend"
  "rpc DeleteFriend"
  "rpc ListFriends"
  "rpc GetFriendship"
)

for pattern in "${friends_proto_patterns[@]}"; do
  rg -q "$pattern" proto/friends.proto
done

rg -q "X-User-Id" internal/handler docs

if rg -n "password|password_hash|verification_code|oauth_token|credential" api proto cmd internal; then
  echo "forbidden auth secret field found in service source" >&2
  exit 1
fi

echo "static verification passed"
