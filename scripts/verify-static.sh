#!/usr/bin/env bash
set -euo pipefail

required_files=(
  "api/user.api"
  "api/groups.api"
  "proto/user.proto"
  "proto/groups.proto"
  "cmd/user-api/main.go"
  "cmd/user-rpc/main.go"
  "cmd/groups-api/main.go"
  "cmd/groups-rpc/main.go"
  "internal/logic/userlogic.go"
  "internal/logic/groupslogic.go"
  "internal/repository/memory.go"
  "internal/repository/groups_memory.go"
  "internal/handler/handler.go"
  "internal/handler/groups_handler.go"
  "tests/user_service_test.go"
  "tests/groups_service_test.go"
  "docs/product-specs/user-service.md"
  "docs/product-specs/groups-service.md"
  "docs/design-docs/user-service-go-zero.md"
  "docs/design-docs/groups-service-go-zero.md"
  "docs/exec-plans/active/user-service-go-zero.md"
  "docs/exec-plans/active/groups-service-go-zero.md"
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

rg -q "X-User-Id" internal/handler docs
rg -q "user-rpc" docs/design-docs/groups-service-go-zero.md docs/product-specs/groups-service.md

if rg -n "password|password_hash|verification_code|oauth_token|credential" api proto cmd internal; then
  echo "forbidden auth secret field found in service source" >&2
  exit 1
fi

echo "static verification passed"
