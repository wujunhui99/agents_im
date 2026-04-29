#!/usr/bin/env bash
set -euo pipefail

required_files=(
  "api/user.api"
  "api/auth.api"
  "api/friends.api"
  "proto/user.proto"
  "proto/auth.proto"
  "proto/friends.proto"
  "cmd/user-api/main.go"
  "cmd/user-rpc/main.go"
  "cmd/auth-api/main.go"
  "cmd/auth-rpc/main.go"
  "cmd/friends-api/main.go"
  "cmd/friends-rpc/main.go"
  "internal/logic/userlogic.go"
  "internal/logic/friendslogic.go"
  "internal/model/friendship.go"
  "internal/repository/memory.go"
  "internal/handler/handler.go"
  "internal/auth/logic/authlogic.go"
  "internal/auth/repository/memory.go"
  "internal/auth/handler/handler.go"
  "internal/auth/token/token.go"
  "internal/auth/useradapter/user_client.go"
  "tests/user_service_test.go"
  "tests/auth_service_test.go"
  "tests/friends_service_test.go"
  "docs/product-specs/user-service.md"
  "docs/product-specs/auth-service.md"
  "docs/product-specs/friends-service.md"
  "docs/design-docs/user-service-go-zero.md"
  "docs/design-docs/auth-service-go-zero.md"
  "docs/design-docs/friends-service-go-zero.md"
  "docs/exec-plans/active/user-service-go-zero.md"
  "docs/exec-plans/active/auth-service-go-zero.md"
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

rg -q "X-User-Id" internal/handler docs
rg -q "ExistsByIdentifier" internal/auth docs/design-docs/auth-service-go-zero.md docs/product-specs/auth-service.md
rg -q "CreateUser" internal/auth docs/design-docs/auth-service-go-zero.md docs/product-specs/auth-service.md
rg -q "PasswordHash" internal/auth/model/credential.go
rg -q "Salt" internal/auth/model/credential.go

if rg -n "password|password_hash|verification_code|oauth_token|credential" \
  api/user.api proto/user.proto cmd/user-api cmd/user-rpc \
  internal/model internal/logic internal/repository internal/handler internal/rpc internal/svc; then
  echo "forbidden auth secret field found in service source" >&2
  exit 1
fi

echo "static verification passed"
