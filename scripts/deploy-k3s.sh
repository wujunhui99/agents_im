#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-agents-im}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-ghcr.io/wujunhui99/agents_im}"
IMAGE_TAG="${IMAGE_TAG:-}"
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

build_image_overrides() {
  local selected=("$@")
  local service image
  for service in "${SERVICES[@]}"; do
    if array_contains "${service}" "${selected[@]}"; then
      printf '%s=%s\n' "${service}" "${IMAGE_REGISTRY}/${service}:${IMAGE_TAG}"
      continue
    fi
    image="$(current_image_for "${service}")"
    if [[ -n "${image}" ]]; then
      printf '%s=%s\n' "${service}" "${image}"
    fi
  done
}

apply_rendered_manifests() {
  local image_overrides="$1"
  local app_deployments
  app_deployments="$(printf '%s\n' "${SERVICES[@]}")"
  ${KUBECTL} kustomize "${MANIFEST_DIR}" | APP_DEPLOYMENT_NAMES="${app_deployments}" IMAGE_OVERRIDES="${image_overrides}" python3 -c '
import os
import re
import sys

app_deployments = set(os.environ.get("APP_DEPLOYMENT_NAMES", "").splitlines())
overrides = {}
for line in os.environ.get("IMAGE_OVERRIDES", "").splitlines():
    if "=" in line:
        service, image = line.split("=", 1)
        overrides[service] = image
text = sys.stdin.read()
docs = re.split(r"(?m)^---\s*$", text)
kept = []
for doc in docs:
    if not doc.strip():
        continue
    kind_match = re.search(r"(?m)^kind:\s*Deployment\s*$", doc)
    name_match = re.search(r"(?m)^metadata:\s*$.*?^  name:\s*([^\s#]+)", doc, re.S)
    name = name_match.group(1) if name_match else ""
    if kind_match and name in app_deployments:
        image = overrides.get(name)
        if not image:
            continue
        doc = re.sub(r"(?m)^(\s*-\s*image:\s*|\s*image:\s*)\S+\s*$", r"\g<1>" + image, doc, count=1)
    kept.append(doc.strip() + "\n")
if kept:
    rendered = "---\n" + "---\n".join(kept)
    if "__IMAGE_TAG_REQUIRED__" in rendered:
        print("rendered manifests still contain __IMAGE_TAG_REQUIRED__; refusing to apply placeholder images", file=sys.stderr)
        sys.exit(1)
    sys.stdout.write(rendered)
' | ${KUBECTL} apply -f -
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
  database_url="${DATABASE_URL:-}"
  if [[ -z "${database_url}" ]]; then
    database_url="$(${KUBECTL} -n "${NAMESPACE}" get secret agents-im-secrets -o jsonpath='{.data.DATABASE_URL}' | base64 -d)"
  fi
  if [[ -z "${database_url}" ]]; then
    echo "DATABASE_URL is missing in ${NAMESPACE}/agents-im-secrets" >&2
    exit 1
  fi
  DATABASE_URL="${database_url}" bash scripts/migrate-postgres.sh --host-psql
}

apply_manifests() {
  local selected_image_services=()
  local restart_rollout_services=()
  local image_overrides=""

  if ! bool_true "${SKIP_SET_IMAGE}"; then
    if [[ -z "${IMAGE_TAG}" || "${IMAGE_TAG}" == "latest" ]]; then
      echo "IMAGE_TAG must be a non-empty immutable tag when SKIP_SET_IMAGE is false; refusing to deploy mutable latest tags" >&2
      exit 1
    fi
    read_selected_services "${IMAGE_SERVICES}" selected_image_services
  fi
  capture_current_images
  image_overrides="$(build_image_overrides "${selected_image_services[@]}")"

  ${KUBECTL} apply -f "${MANIFEST_DIR}/namespace.yaml"
  ensure_secret
  ensure_image_pull_secret
  apply_rendered_manifests "${image_overrides}"

  if bool_true "${SKIP_SET_IMAGE}"; then
    echo "Skipping image updates; keeping currently deployed image tags."
  else
    local service
    for service in "${selected_image_services[@]}"; do
      [[ -z "${service}" ]] && continue
      ${KUBECTL} -n "${NAMESPACE}" set image "deployment/${service}" "${service}=${IMAGE_REGISTRY}/${service}:${IMAGE_TAG}" --record=false
    done
  fi

  if [[ -n "${RESTART_SERVICES}" ]] || bool_true "${RESTART_ROLLOUT}"; then
    while IFS= read -r service; do
      [[ -z "${service}" ]] && continue
      restart_rollout_services+=("${service}")
      ${KUBECTL} -n "${NAMESPACE}" rollout restart "deployment/${service}"
    done < <(restart_services)
  fi

  mapfile -t rollout_wait_services < <(
    unique_services "${selected_image_services[@]}" "${restart_rollout_services[@]:-}"
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
  require python3
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
