#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-agents-im}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-ghcr.io/wujunhui99/agents_im}"
IMAGE_TAG="${IMAGE_TAG:-}"
MANIFEST_DIR="${MANIFEST_DIR:-deploy/k8s}"
KUBECTL="${KUBECTL:-kubectl}"
GHCR_USERNAME="${GHCR_USERNAME:-}"
GHCR_TOKEN="${GHCR_TOKEN:-}"
SKIP_SET_IMAGE="${SKIP_SET_IMAGE:-false}"
SKIP_MIGRATIONS="${SKIP_MIGRATIONS:-false}"
IMAGE_SERVICES="${IMAGE_SERVICES:-}"
ROLLOUT_SERVICES="${ROLLOUT_SERVICES:-}"
RESTART_SERVICES="${RESTART_SERVICES:-}"
RESTART_ROLLOUT="${RESTART_ROLLOUT:-false}"
RENDER_ONLY="${RENDER_ONLY:-false}"
ROLLOUT_PARALLELISM="${DEPLOY_ROLLOUT_PARALLELISM:-4}"

# Service registry comes from scripts/services.json (single source of truth),
# shared with detect-deploy-changes.py and dev-up.sh.
source "$(dirname "${BASH_SOURCE[0]}")/services.sh"
WEB_DEPLOYMENT="$(services_web_name)"
mapfile -t IMAGE_DEPLOYMENTS < <(services_backend_names; printf '%s\n' "${WEB_DEPLOYMENT}")
mapfile -t RESTARTABLE_DEPLOYMENTS < <(printf '%s\n' "${IMAGE_DEPLOYMENTS[@]}"; services_infra_names)

# Recreate rollouts must respect startup dependencies. user-rpc performs a
# startup backfill through agent-rpc, while APIs/workers depend on RPCs. Keep
# independent provider RPCs parallel, then user-rpc, then all remaining apps.
ROLLOUT_PROVIDER_WAVE=(
  auth-rpc friends-rpc groups-rpc msg-rpc agent-rpc third-rpc media-rpc admin-rpc
)
ROLLOUT_ACCOUNT_WAVE=(user-rpc)

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
  shift
  local known
  for known in "$@"; do
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
  local -n allowed_ref="$2"
  local -n out_ref="$3"
  local service
  out_ref=()
  if [[ -n "${service_list}" ]]; then
    for service in ${service_list}; do
      if ! known_service "${service}" "${allowed_ref[@]}"; then
        echo "unknown deployment service: ${service}" >&2
        exit 1
      fi
      out_ref+=("${service}")
    done
  else
    out_ref=("${allowed_ref[@]}")
  fi
}

selected_services() {
  local service_list="$1"
  shift
  local allowed=("$@")
  local services=()
  read_selected_services "${service_list}" allowed services
  printf '%s\n' "${services[@]}"
}

image_services() {
  selected_services "${IMAGE_SERVICES}" "${IMAGE_DEPLOYMENTS[@]}"
}

rollout_services() {
  selected_services "${ROLLOUT_SERVICES}" "${RESTARTABLE_DEPLOYMENTS[@]}"
}

restart_services() {
  if [[ -n "${RESTART_SERVICES}" ]]; then
    selected_services "${RESTART_SERVICES}" "${RESTARTABLE_DEPLOYMENTS[@]}"
  elif bool_true "${RESTART_ROLLOUT}"; then
    # Restart only app image services (not monitoring infrastructure) so that
    # every deploy picks up the latest secret values even if the image didn't change.
    selected_services "" "${IMAGE_DEPLOYMENTS[@]}"
  fi
}

capture_current_images() {
  CURRENT_IMAGES=()
  local service image
  for service in "${IMAGE_DEPLOYMENTS[@]}"; do
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

image_for_service() {
  local service="$1"
  printf '%s/%s:%s\n' "${IMAGE_REGISTRY}" "${service}" "${IMAGE_TAG}"
}

build_image_overrides() {
  local selected=("$@")
  local service image
  for service in "${IMAGE_DEPLOYMENTS[@]}"; do
    if array_contains "${service}" "${selected[@]}"; then
      printf '%s=%s\n' "${service}" "$(image_for_service "${service}")"
      continue
    fi
    image="$(current_image_for "${service}")"
    if [[ -n "${image}" ]]; then
      printf '%s=%s\n' "${service}" "${image}"
    elif [[ -n "${IMAGE_TAG}" && "${IMAGE_TAG}" != "latest" ]]; then
      # Fresh clusters have no current image to preserve. Use the immutable
      # deployment tag so a full manifest apply can create all Deployments
      # without falling back to placeholders or mutable latest tags.
      printf '%s=%s\n' "${service}" "$(image_for_service "${service}")"
    fi
  done
}

apply_rendered_manifests() {
  local image_overrides="$1"
  local app_deployment_mode="${2:-all}"
  local selected_app_deployments="${3:-}"
  local app_deployments
  app_deployments="$(printf '%s\n' "${IMAGE_DEPLOYMENTS[@]}")"
  ${KUBECTL} kustomize "${MANIFEST_DIR}" | \
    APP_DEPLOYMENT_NAMES="${app_deployments}" \
    APP_DEPLOYMENT_MODE="${app_deployment_mode}" \
    SELECTED_APP_DEPLOYMENTS="${selected_app_deployments}" \
    IMAGE_OVERRIDES="${image_overrides}" \
    python3 -c '
import os
import re
import sys

app_deployments = set(os.environ.get("APP_DEPLOYMENT_NAMES", "").splitlines())
app_deployment_mode = os.environ.get("APP_DEPLOYMENT_MODE", "all")
selected_app_deployments = set(os.environ.get("SELECTED_APP_DEPLOYMENTS", "").splitlines())
if app_deployment_mode not in {"all", "exclude", "only"}:
    print(f"unknown APP_DEPLOYMENT_MODE={app_deployment_mode}", file=sys.stderr)
    sys.exit(1)
overrides = {}
for line in os.environ.get("IMAGE_OVERRIDES", "").splitlines():
    if "=" in line:
        service, image = line.split("=", 1)
        overrides[service] = image
text = sys.stdin.read()
docs = re.split(r"(?m)^---\s*$", text)
kept = []
seen_selected_app_deployments = set()
for doc in docs:
    if not doc.strip():
        continue
    kind_match = re.search(r"(?m)^kind:\s*Deployment\s*$", doc)
    name_match = re.search(r"(?m)^metadata:\s*$.*?^  name:\s*([^\s#]+)", doc, re.S)
    name = name_match.group(1) if name_match else ""
    is_app_deployment = bool(kind_match and name in app_deployments)
    if app_deployment_mode == "exclude" and is_app_deployment:
        continue
    if app_deployment_mode == "only" and (
        not is_app_deployment or name not in selected_app_deployments
    ):
        continue
    if is_app_deployment:
        if name in selected_app_deployments:
            seen_selected_app_deployments.add(name)
        image = overrides.get(name)
        if not image:
            print(f"missing safe image override for Deployment/{name}; refusing to render/apply", file=sys.stderr)
            sys.exit(1)
        doc, replacements = re.subn(r"(?m)^(\s*-\s*image:\s*|\s*image:\s*)\S+\s*$", r"\g<1>" + image, doc, count=1)
        if replacements != 1:
            print(f"Deployment/{name} did not contain exactly one replaceable image field", file=sys.stderr)
            sys.exit(1)
    kept.append(doc.strip() + "\n")
if app_deployment_mode == "only":
    missing = selected_app_deployments - seen_selected_app_deployments
    if missing:
        print(f"selected application Deployments missing from render: {sorted(missing)}", file=sys.stderr)
        sys.exit(1)
if kept:
    rendered = "---\n" + "---\n".join(kept)
    if "__IMAGE_TAG_REQUIRED__" in rendered or re.search(r"(?m)^\s*image:\s*\S+:latest\s*$", rendered):
        print("rendered manifests still contain placeholder or latest images; refusing to apply unsafe images", file=sys.stderr)
        sys.exit(1)
    sys.stdout.write(rendered)
'
}

run_rollout_wave() {
  local wave_name="$1"
  local image_overrides="$2"
  local selected_images_name="$3"
  local restart_services_name="$4"
  shift 4
  local -n selected_images_ref="${selected_images_name}"
  local -n restart_services_ref="${restart_services_name}"
  local services=("$@")
  local service app_selection pid offset=0 batch_number=0

  ((${#services[@]} > 0)) || return 0
  while ((offset < ${#services[@]})); do
    local batch=("${services[@]:offset:ROLLOUT_PARALLELISM}")
    local app_services=()
    local -a rollout_pids=()
    batch_number=$((batch_number + 1))
    echo "Starting rollout wave ${wave_name}.${batch_number}: ${batch[*]}"

    for service in "${batch[@]}"; do
      if known_service "${service}" "${IMAGE_DEPLOYMENTS[@]}"; then
        app_services+=("${service}")
      fi
    done
    if ((${#app_services[@]} > 0)); then
      app_selection="$(printf '%s\n' "${app_services[@]}")"
      apply_rendered_manifests "${image_overrides}" only "${app_selection}" | ${KUBECTL} apply -f -
    fi

    for service in "${batch[@]}"; do
      if array_contains "${service}" "${restart_services_ref[@]}" && \
        ! array_contains "${service}" "${selected_images_ref[@]}"; then
        ${KUBECTL} -n "${NAMESPACE}" rollout restart "deployment/${service}"
      fi
    done

    for service in "${batch[@]}"; do
      ${KUBECTL} -n "${NAMESPACE}" rollout status "deployment/${service}" --timeout=180s &
      rollout_pids+=("$!")
    done
    local rollout_failed=0
    for pid in "${rollout_pids[@]}"; do
      wait "${pid}" || rollout_failed=1
    done
    if ((rollout_failed)); then
      echo "Rollout wave ${wave_name}.${batch_number} failed." >&2
      return 1
    fi
    offset=$((offset + ROLLOUT_PARALLELISM))
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

verify_deployed_images() {
  local service expected deployment_image pods_json

  for service in "$@"; do
    [[ -z "${service}" ]] && continue
    expected="$(image_for_service "${service}")"
    deployment_image="$(${KUBECTL} -n "${NAMESPACE}" get deployment "${service}" -o "jsonpath={.spec.template.spec.containers[?(@.name=='${service}')].image}")"
    if [[ "${deployment_image}" != "${expected}" ]]; then
      echo "deployment/${service} image mismatch: expected=${expected} actual=${deployment_image}" >&2
      exit 1
    fi

    pods_json="$(${KUBECTL} -n "${NAMESPACE}" get pods -l "app=${service}" -o json)"
    SERVICE="${service}" EXPECTED_IMAGE="${expected}" PODS_JSON="${pods_json}" python3 -c '
import json
import os
import sys

service = os.environ["SERVICE"]
expected = os.environ["EXPECTED_IMAGE"]
data = json.loads(os.environ.get("PODS_JSON") or "{}")
items = data.get("items") or []
if not items:
    print(f"deployment/{service} has no pods for app={service}", file=sys.stderr)
    sys.exit(1)

verified = 0
for pod in items:
    name = pod.get("metadata", {}).get("name", "<unknown>")
    metadata = pod.get("metadata", {})
    phase = pod.get("status", {}).get("phase", "")
    # Skip non-live pods: terminal-phase leftovers (k3s graceful-shutdown leaves
    # old replicas in Succeeded/Failed after a node reboot and does not always GC
    # them) and pods already being deleted. They are not serving traffic, so they
    # must not fail verification of a rollout that the controller reported healthy.
    if phase in ("Succeeded", "Failed") or metadata.get("deletionTimestamp"):
        print(f"skipping non-live pod/{name} for {service}: phase={phase} deleting={bool(metadata.get('deletionTimestamp'))}")
        continue
    containers = {c.get("name"): c for c in pod.get("spec", {}).get("containers", [])}
    statuses = {s.get("name"): s for s in pod.get("status", {}).get("containerStatuses", [])}
    container = containers.get(service)
    status = statuses.get(service, {})
    image = (container or {}).get("image", "")
    ready = bool(status.get("ready"))
    image_id = status.get("imageID", "")
    if phase != "Running" or image != expected or not ready:
        print(
            f"pod/{name} image verification failed for {service}: "
            f"phase={phase} ready={ready} expected={expected} actual={image} imageID={image_id}",
            file=sys.stderr,
        )
        sys.exit(1)
    print(f"verified image service={service} pod={name} ready={ready} image={image} imageID={image_id}")
    verified += 1

if verified == 0:
    print(f"deployment/{service} has no verifiable pods", file=sys.stderr)
    sys.exit(1)
'
  done
}

cleanup_obsolete_resources() {
  # kubectl apply does not prune resources that were removed from manifests.
  # Keep explicit tombstones here for renamed/retired public entrypoints.
  ${KUBECTL} -n "${NAMESPACE}" delete ingress agents-im-prometheus --ignore-not-found=true
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
    read_selected_services "${IMAGE_SERVICES}" IMAGE_DEPLOYMENTS selected_image_services
  fi
  capture_current_images
  image_overrides="$(build_image_overrides "${selected_image_services[@]}")"

  if bool_true "${RENDER_ONLY}"; then
    apply_rendered_manifests "${image_overrides}"
    return
  fi

  ${KUBECTL} apply -f "${MANIFEST_DIR}/namespace.yaml"
  ensure_secret
  ensure_image_pull_secret
  cleanup_obsolete_resources
  # Shared and infrastructure resources are safe to apply immediately. App
  # Deployments are applied below in dependency waves so their Recreate
  # strategy cannot trigger a pull storm or startup dependency race.
  apply_rendered_manifests "${image_overrides}" exclude | ${KUBECTL} apply -f -

  if bool_true "${SKIP_SET_IMAGE}"; then
    echo "Skipping image updates; rendered manifest preserves currently deployed image tags."
  fi

  if [[ -n "${RESTART_SERVICES}" ]] || bool_true "${RESTART_ROLLOUT}"; then
    while IFS= read -r service; do
      [[ -z "${service}" ]] && continue
      restart_rollout_services+=("${service}")
    done < <(restart_services)
  fi

  local -a rollout_wait_services=()
  mapfile -t rollout_wait_services < <(
    unique_services "${selected_image_services[@]}" "${restart_rollout_services[@]:-}"
  )
  local -a provider_wave=()
  local -a account_wave=()
  local -a application_wave=()
  local service
  for service in "${ROLLOUT_PROVIDER_WAVE[@]}"; do
    if array_contains "${service}" "${rollout_wait_services[@]}"; then
      provider_wave+=("${service}")
    fi
  done
  for service in "${ROLLOUT_ACCOUNT_WAVE[@]}"; do
    if array_contains "${service}" "${rollout_wait_services[@]}"; then
      account_wave+=("${service}")
    fi
  done
  for service in "${rollout_wait_services[@]}"; do
    if ! array_contains "${service}" "${provider_wave[@]}" && \
      ! array_contains "${service}" "${account_wave[@]}"; then
      application_wave+=("${service}")
    fi
  done

  run_rollout_wave providers "${image_overrides}" selected_image_services restart_rollout_services "${provider_wave[@]}"
  run_rollout_wave account "${image_overrides}" selected_image_services restart_rollout_services "${account_wave[@]}"
  run_rollout_wave applications "${image_overrides}" selected_image_services restart_rollout_services "${application_wave[@]}"

  if ((${#selected_image_services[@]} > 0)); then
    verify_deployed_images "${selected_image_services[@]}"
  fi

  ${KUBECTL} -n "${NAMESPACE}" get pods -o wide
  if [[ "${AGENTS_IM_DEPLOY_LIST_RESOURCES:-false}" == "true" ]]; then
    ${KUBECTL} -n "${NAMESPACE}" get svc,ingress
  fi
}

main() {
  if [[ "${1:-}" == "--render-only" ]]; then
    RENDER_ONLY="true"
    SKIP_MIGRATIONS="true"
    shift
  fi
  require "${KUBECTL}"
  require python3
  if ! [[ "${ROLLOUT_PARALLELISM}" =~ ^[1-9][0-9]*$ ]]; then
    echo "DEPLOY_ROLLOUT_PARALLELISM must be a positive integer; got ${ROLLOUT_PARALLELISM}." >&2
    exit 1
  fi
  if ! bool_true "${SKIP_MIGRATIONS}"; then
    require psql
  fi
  run_migrations
  apply_manifests
}

main "$@"
