#!/usr/bin/env bash
# Static verification orchestrator.
#
# Preflight (required files + shell syntax) then runs the themed gates under
# scripts/verify/. Each gate is also runnable standalone:
#   scripts/verify/verify-security-static.sh    secrets / key material / auth-leak
#   scripts/verify/verify-gozero-boundaries.sh  go-zero layering boundaries
#   scripts/verify/verify-contract-markers.sh   API/proto/schema/code contract surface
#   scripts/verify/verify-deploy-static.sh      deploy / CI / middleware / k8s config
#   scripts/verify/verify-frontend-static.sh    web/ frontend gates
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/verify/lib.sh"
cd "$(git rev-parse --show-toplevel)"

required_files=(
  # Structural anchors whose silent DELETION no other gate catches.
  # Inclusion criterion — a deleted file here surfaces only at runtime/deploy,
  # never at PR time, because it is NOT covered by:
  #   - go build / go test (any import-reachable *.go is already covered there)
  #   - goctl api validate (runs over a glob, so a missing *.api is just skipped)
  #   - frontend tsc / vitest (covers web/ *.tsx/*.ts)
  #   - verify-deploy-static / migration-immutability / content greps below
  # So we keep ONLY: codegen sources (*.proto, *.api) and runtime configs (etc/*.yaml).
  # Adding a new service? Add its proto/api/etc entries here.
  "api/media.api"
  "service/user/api/user.api"
  "service/auth/api/auth.api"
  "service/friends/api/friends.api"
  "service/groups/api/groups.api"
  "service/agent/api/agent.api"
  "service/msg/api/msg.api"
  "service/user/rpc/user.proto"
  "service/auth/rpc/auth.proto"
  "service/friends/rpc/friends.proto"
  "service/groups/rpc/groups.proto"
  "service/msg/rpc/msg.proto"
  "service/third/rpc/mail.proto"
  "etc/msggateway.yaml"
  "etc/msgtransfer.yaml"
  "etc/msg-rpc.yaml"
  "etc/third-rpc.yaml"
  ".drone.yml"
)
require_files "${required_files[@]}"

syntax_check \
  scripts/migrate-postgres.sh \
  scripts/verify-postgres-local.sh \
  scripts/dev-up.sh \
  scripts/dev-demo-data.sh \
  scripts/deploy-k3s.sh \
  scripts/bootstrap-server.sh \
  scripts/test-deploy-k3s.sh \
  scripts/test-no-latest-images.sh \
  scripts/verify-static.sh \
  scripts/verify/lib.sh \
  scripts/verify/verify-security-static.sh \
  scripts/verify/verify-gozero-boundaries.sh \
  scripts/verify/verify-contract-markers.sh \
  scripts/verify/verify-deploy-static.sh \
  scripts/verify/verify-frontend-static.sh

bash "${SCRIPT_DIR}/verify/verify-security-static.sh"
bash "${SCRIPT_DIR}/verify/verify-gozero-boundaries.sh"
bash "${SCRIPT_DIR}/verify/verify-contract-markers.sh"
bash "${SCRIPT_DIR}/verify/verify-deploy-static.sh"
bash "${SCRIPT_DIR}/verify/verify-frontend-static.sh"

echo "static verification passed"
