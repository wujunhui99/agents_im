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
IMAGE_SERVICES="${IMAGE_SERVICES:-}"
ROLLOUT_SERVICES="${ROLLOUT_SERVICES:-}"
RESTART_SERVICES="${RESTART_SERVICES:-}"
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
  mail-rpc
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

known_service() {
  local service="$1"
  local known
  for known in "${SERVICES[@]}"; do
    if [[ "${known}" == "${service}" ]]; then
      return 0
    fi
  done
  return 1
}

array_contains() {
  local needle="$1"
  shift
  local item
  for item in "$@"; do
    if [[ "${item}" == "${needle}" ]]; then
      return 0
    fi
  done
  return 1
}

read_selected_services() {
  local service_list="$1"
  local -n out_ref="$2"
  local service
  out_ref=()
  if [[ -n "${service_list}" ]]; then
    for service in ${service_list}; do
      if ! known_service "${service}"; then
        echo "unknown deployment service: ${service}" >&2
        exit 1
      fi
      out_ref+=("${service}")
    done
  else
    out_ref=("${SERVICES[@]}")
  fi
}

selected_services() {
  local services=()
  read_selected_services "$1" services
  printf '%s\n' "${services[@]}"
}

image_services() {
  selected_services "${IMAGE_SERVICES}"
}

rollout_services() {
  selected_services "${ROLLOUT_SERVICES}"
}

restart_services() {
  if [[ -n "${RESTART_SERVICES}" ]]; then
    selected_services "${RESTART_SERVICES}"
  elif bool_true "${RESTART_ROLLOUT}"; then
    rollout_services
  fi
}

capture_current_images() {
  CURRENT_IMAGES=()
  local service image
  for service in "${SERVICES[@]}"; do
    image="$(${KUBECTL} -n "${NAMESPACE}" get deployment "${service}" -o "jsonpath={.spec.template.spec.containers[?(@.name=='${service}')].image}" 2>/dev/null || true)"
    CURRENT_IMAGES+=("${service}=${image}")
  done
}

current_image_for() {
  local service="$1"
  local pair
  for pair in "${CURRENT_IMAGES[@]:-}"; do
    if [[ "${pair}" == "${service}="* ]]; then
      printf '%s\n' "${pair#*=}"
      return 0
    fi
  done
  return 1
}

restore_unselected_images() {
  local selected=("$@")
  local service image
  for service in "${SERVICES[@]}"; do
    if array_contains "${service}" "${selected[@]}"; then
      continue
    fi
    image="$(current_image_for "${service}")"
    if [[ -n "${image}" ]]; then
      ${KUBECTL} -n "${NAMESPACE}" set image "deployment/${service}" "${service}=${image}" --record=false
      RESTORED_SERVICES+=("${service}")
    fi
  done
}

unique_services() {
  local seen=" "
  local service
  for service in "$@"; do
    [[ -z "${service}" ]] && continue
    if [[ "${seen}" == *" ${service} "* ]]; then
      continue
    fi
    seen+="${service} "
    printf '%s\n' "${service}"
  done
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
  local selected_image_services=()
  local restored_services=()
  RESTORED_SERVICES=()

  capture_current_images
  read_selected_services "${IMAGE_SERVICES}" selected_image_services

  ${KUBECTL} apply -f "${MANIFEST_DIR}/namespace.yaml"
  ensure_secret
  ensure_image_pull_secret
  ${KUBECTL} apply -k "${MANIFEST_DIR}"

  if bool_true "${SKIP_SET_IMAGE}"; then
    echo "Skipping image updates; keeping currently deployed image tags."
    restore_unselected_images
    restored_services=("${RESTORED_SERVICES[@]:-}")
  else
    local service
    for service in "${selected_image_services[@]}"; do
      [[ -z "${service}" ]] && continue
      ${KUBECTL} -n "${NAMESPACE}" set image "deployment/${service}" "${service}=${IMAGE_REGISTRY}/${service}:${IMAGE_TAG}" --record=false
    done
    restore_unselected_images "${selected_image_services[@]}"
    restored_services=("${RESTORED_SERVICES[@]:-}")
  fi

  if [[ -n "${RESTART_SERVICES}" ]] || bool_true "${RESTART_ROLLOUT}"; then
    while IFS= read -r service; do
      [[ -z "${service}" ]] && continue
      ${KUBECTL} -n "${NAMESPACE}" rollout restart "deployment/${service}"
    done < <(restart_services)
  fi

  mapfile -t rollout_wait_services < <(
    unique_services "${selected_image_services[@]}" "${restored_services[@]:-}"
  )
  for service in "${rollout_wait_services[@]}"; do
    [[ -z "${service}" ]] && continue
    ${KUBECTL} -n "${NAMESPACE}" rollout status "deployment/${service}" --timeout=180s
  done

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
