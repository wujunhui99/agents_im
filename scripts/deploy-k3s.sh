#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-agents-im}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-ghcr.io/wujunhui99/agents_im}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
MIDDLEWARE_DIR="${MIDDLEWARE_DIR:-/opt/agents-im/middleware}"
MANIFEST_DIR="${MANIFEST_DIR:-deploy/k8s}"
KUBECTL="${KUBECTL:-kubectl}"
GHCR_USERNAME="${GHCR_USERNAME:-}"
GHCR_TOKEN="${GHCR_TOKEN:-}"
SKIP_SET_IMAGE="${SKIP_SET_IMAGE:-false}"
SKIP_MIDDLEWARE="${SKIP_MIDDLEWARE:-false}"
SKIP_MIGRATIONS="${SKIP_MIGRATIONS:-false}"
ROLLOUT_SERVICES="${ROLLOUT_SERVICES:-}"
RESTART_ROLLOUT="${RESTART_ROLLOUT:-false}"

SERVICES=(
  user-api
  auth-api
  friends-api
  message-api
  gateway-ws
  groups-api
  agent-api
  message-transfer
  user-rpc
  auth-rpc
  friends-rpc
  groups-rpc
  message-rpc
  web
)

require() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "required command not found: $1" >&2
    exit 1
  fi
}

bool_true() {
  [[ "$1" == "true" || "$1" == "1" || "$1" == "yes" ]]
}

rollout_services() {
  if [[ -n "${ROLLOUT_SERVICES}" ]]; then
    # shellcheck disable=SC2206
    echo "${ROLLOUT_SERVICES}"
  else
    printf '%s\n' "${SERVICES[@]}"
  fi
}

ensure_secret() {
  if ! ${KUBECTL} -n "${NAMESPACE}" get secret agents-im-secrets >/dev/null 2>&1; then
    echo "missing Kubernetes secret ${NAMESPACE}/agents-im-secrets" >&2
    echo "create it on the server before deploying; see deploy/k8s/secrets.example.yaml" >&2
    exit 1
  fi
}

start_middleware() {
  if bool_true "${SKIP_MIDDLEWARE}"; then
    echo "Skipping middleware startup."
    return
  fi
  if [[ ! -f "${MIDDLEWARE_DIR}/docker-compose.yml" ]]; then
    echo "middleware compose file not found: ${MIDDLEWARE_DIR}/docker-compose.yml" >&2
    exit 1
  fi
  docker compose --env-file "${MIDDLEWARE_DIR}/.env" -f "${MIDDLEWARE_DIR}/docker-compose.yml" up -d
}

ensure_image_pull_secret() {
  if [[ -n "${GHCR_USERNAME}" && -n "${GHCR_TOKEN}" ]]; then
    ${KUBECTL} -n "${NAMESPACE}" create secret docker-registry ghcr-pull-secret \
      --docker-server=ghcr.io \
      --docker-username="${GHCR_USERNAME}" \
      --docker-password="${GHCR_TOKEN}" \
      --dry-run=client -o yaml | ${KUBECTL} apply -f -
  fi
}

run_migrations() {
  if bool_true "${SKIP_MIGRATIONS}"; then
    echo "Skipping database migrations."
    return
  fi
  local database_url
  database_url="$(${KUBECTL} -n "${NAMESPACE}" get secret agents-im-secrets -o jsonpath='{.data.DATABASE_URL}' | base64 -d)"
  if [[ -z "${database_url}" ]]; then
    echo "DATABASE_URL is missing in ${NAMESPACE}/agents-im-secrets" >&2
    exit 1
  fi
  DATABASE_URL="${database_url}" bash scripts/migrate-postgres.sh --host-psql
}

apply_manifests() {
  ${KUBECTL} apply -f "${MANIFEST_DIR}/namespace.yaml"
  ensure_secret
  ensure_image_pull_secret
  ${KUBECTL} apply -k "${MANIFEST_DIR}"

  if bool_true "${SKIP_SET_IMAGE}"; then
    echo "Skipping image updates; keeping currently deployed image tags."
  else
    for service in "${SERVICES[@]}"; do
      ${KUBECTL} -n "${NAMESPACE}" set image "deployment/${service}" "${service}=${IMAGE_REGISTRY}/${service}:${IMAGE_TAG}" --record=false
    done
  fi

  if bool_true "${RESTART_ROLLOUT}"; then
    while IFS= read -r service; do
      [[ -z "${service}" ]] && continue
      ${KUBECTL} -n "${NAMESPACE}" rollout restart "deployment/${service}"
    done < <(rollout_services)
  fi

  while IFS= read -r service; do
    [[ -z "${service}" ]] && continue
    ${KUBECTL} -n "${NAMESPACE}" rollout status "deployment/${service}" --timeout=180s
  done < <(rollout_services)

  ${KUBECTL} -n "${NAMESPACE}" get pods -o wide
  ${KUBECTL} -n "${NAMESPACE}" get svc,ingress
}

main() {
  require "${KUBECTL}"
  if ! bool_true "${SKIP_MIDDLEWARE}"; then
    require docker
  fi
  if ! bool_true "${SKIP_MIGRATIONS}"; then
    require psql
  fi
  start_middleware
  run_migrations
  apply_manifests
}

main "$@"
