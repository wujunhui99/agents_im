#!/usr/bin/env bash
set -euo pipefail

source ./.drone-deploy.env

if [[ "${build_required}" != "true" ]]; then
  echo "No image build required."
  exit 0
fi

: "${GHCR_USERNAME:?GHCR_USERNAME Drone secret is required}"
: "${GHCR_TOKEN:?GHCR_TOKEN Drone secret is required}"
: "${DRONE_COMMIT_SHA:?DRONE_COMMIT_SHA is required}"

registry="${IMAGE_REGISTRY:-ghcr.io/wujunhui99/agents_im}"
commit_sha="${DRONE_COMMIT_SHA}"
cache_tag="${DRONE_BUILD_CACHE_TAG:-buildcache}"
cache_scope="${DRONE_BUILD_CACHE_SCOPE:-shared}"
build_parallelism="${DRONE_IMAGE_BUILD_PARALLELISM:-3}"
force_all_images="${DRONE_IMAGE_BUILD_FORCE_ALL:-false}"
build_only="${DRONE_IMAGE_BUILD_ONLY:-false}"
branch="${DRONE_BRANCH:-}"
export_backend_cache="${DRONE_EXPORT_BACKEND_CACHE:-false}"
export_web_cache="${DRONE_EXPORT_WEB_CACHE:-true}"

# DRONE_IMAGE_BUILD_FORCE_ALL and DRONE_IMAGE_BUILD_ONLY are measurement
# controls for the devops CI/CD lab branch only. Production main deploys must
# use detector-selected images and must continue into the deploy step so a
# green Drone build means rollout really happened.
if [[ "${branch}" != "devops" ]]; then
  if [[ "${force_all_images}" == "true" || "${build_only}" == "true" ]]; then
    echo "Ignoring devops-only image build measurement controls on branch ${branch:-<unset>}."
  fi
  force_all_images="false"
  build_only="false"
fi

if ! [[ "${build_parallelism}" =~ ^[1-9][0-9]*$ ]]; then
  echo "DRONE_IMAGE_BUILD_PARALLELISM must be a positive integer; got ${build_parallelism}." >&2
  exit 1
fi

export DOCKER_BUILDKIT=1

if ! docker buildx version >/dev/null 2>&1; then
  echo "docker buildx is required for cached image builds." >&2
  exit 1
fi

builder="agents-im-drone-builder"
if ! docker buildx inspect "${builder}" >/dev/null 2>&1; then
  docker buildx create --name "${builder}" --driver docker-container --use
else
  docker buildx use "${builder}"
fi
docker buildx inspect --bootstrap >/dev/null

echo "${GHCR_TOKEN}" | docker login ghcr.io -u "${GHCR_USERNAME}" --password-stdin

if [[ "${force_all_images}" == "true" ]]; then
  image_services_space="user-api auth-api friends-api msg-api gateway-ws groups-api agent-api admin-api msgtransfer user-rpc auth-rpc friends-rpc groups-rpc msg-rpc third-rpc media-api media-rpc admin-rpc web"
  echo "DRONE_IMAGE_BUILD_FORCE_ALL=true; overriding selected image services for performance measurement."
fi

read -r -a services <<< "${image_services_space:-}"
if ((${#services[@]} == 0)); then
  echo "No image services selected."
  exit 0
fi

work_dir="$(mktemp -d)"
trap 'rm -rf "${work_dir}"' EXIT

declare -a completed_services=()
declare -a service_names=()
declare -a service_durations=()
declare -A service_pid=()
declare -A service_status=()
declare -A service_duration=()

service_target() {
  local service="$1"
  if [[ "${service}" == "web" ]]; then
    echo "web"
  else
    echo "backend"
  fi
}

service_cache_name() {
  local service="$1"
  if [[ "${service}" == "web" ]]; then
    echo "web"
  else
    echo "backend-${service}"
  fi
}

should_export_cache() {
  local service="$1"
  if [[ "${service}" == "web" ]]; then
    [[ "${export_web_cache}" == "true" ]]
  else
    [[ "${export_backend_cache}" == "true" ]]
  fi
}

run_build() {
  local service="$1"
  local export_cache="$2"
  local target cache_name cache_ref start_epoch end_epoch duration
  target="$(service_target "${service}")"
  cache_name="$(service_cache_name "${service}")"
  cache_ref="${registry}/cache:${cache_scope}-${cache_name}-${cache_tag}"

  local -a build_args=()
  if [[ "${target}" == "backend" ]]; then
    build_args=(--build-arg "SERVICE=${service}")
  fi

  local -a cache_to_args=()
  if [[ "${export_cache}" == "true" ]]; then
    cache_to_args=(--cache-to "type=registry,ref=${cache_ref},mode=max,image-manifest=true,oci-mediatypes=true")
  fi

  echo "Building ${service} image with target ${target}; cache=${cache_ref}; export_cache=${export_cache}."
  start_epoch="$(date +%s)"
  docker buildx build \
    --builder "${builder}" \
    --target "${target}" \
    "${build_args[@]}" \
    --cache-from "type=registry,ref=${cache_ref}" \
    "${cache_to_args[@]}" \
    --tag "${registry}/${service}:${commit_sha}" \
    --provenance=false \
    --push \
    .
  end_epoch="$(date +%s)"
  duration="$((end_epoch - start_epoch))"
  echo "${duration}" > "${work_dir}/${service}.duration"
  echo "Built and pushed ${service} in ${duration}s."
}

wait_for_service() {
  local service="$1"
  local pid status duration log_file
  pid="${service_pid[${service}]}"
  log_file="${work_dir}/${service}.log"
  set +e
  wait "${pid}"
  status="$?"
  set -e
  service_status["${service}"]="${status}"
  completed_services+=("${service}")
  echo "----- ${service} build log -----"
  cat "${log_file}"
  echo "----- end ${service} build log -----"
  if [[ -f "${work_dir}/${service}.duration" ]]; then
    duration="$(cat "${work_dir}/${service}.duration")"
    service_duration["${service}"]="${duration}"
  fi
  if [[ "${status}" != "0" ]]; then
    echo "Image build failed for ${service} with exit status ${status}." >&2
    return "${status}"
  fi
}

run_batch() {
  local -a batch_services=("$@")
  local -a active=()
  local service log_file failed=0
  for service in "${batch_services[@]}"; do
    while ((${#active[@]} >= build_parallelism)); do
      wait_for_service "${active[0]}" || failed=1
      active=("${active[@]:1}")
      if ((failed)); then
        return 1
      fi
    done
    log_file="${work_dir}/${service}.log"
    echo "Queueing ${service} image build; active=$(( ${#active[@]} + 1 ))/${build_parallelism}."
    run_build "${service}" "$(if should_export_cache "${service}"; then echo true; else echo false; fi)" >"${log_file}" 2>&1 &
    service_pid["${service}"]="$!"
    active+=("${service}")
  done

  while ((${#active[@]} > 0)); do
    wait_for_service "${active[0]}" || failed=1
    active=("${active[@]:1}")
    if ((failed)); then
      return 1
    fi
  done
}

wall_start_epoch="$(date +%s)"
echo "Image build services: ${services[*]}"
echo "Image build parallelism: ${build_parallelism}"

echo "Backend cache export enabled: ${export_backend_cache}"
echo "Web cache export enabled: ${export_web_cache}"
echo "Building images with parallelism ${build_parallelism}: ${services[*]}"
run_batch "${services[@]}"

wall_end_epoch="$(date +%s)"
wall_duration="$((wall_end_epoch - wall_start_epoch))"

if ((${#completed_services[@]} > 0)); then
  echo "Image build duration summary:"
  total_duration=0
  for service in "${services[@]}"; do
    if [[ -n "${service_duration[${service}]:-}" ]]; then
      service_names+=("${service}")
      service_durations+=("${service_duration[${service}]}")
      echo "  ${service}: ${service_duration[${service}]}s"
      total_duration="$((total_duration + service_duration[${service}]))"
    fi
  done
  echo "Total per-service image build duration: ${total_duration}s"
  echo "Total image build wall-clock duration: ${wall_duration}s"
fi

if [[ "${build_only}" == "true" ]]; then
  echo "DRONE_IMAGE_BUILD_ONLY=true; marking deploy_required=false after image-build measurement."
  tmp_env="${work_dir}/drone-deploy.env"
  while IFS= read -r line || [[ -n "${line}" ]]; do
    if [[ "${line}" == deploy_required=* ]]; then
      echo "deploy_required=false"
    else
      echo "${line}"
    fi
  done < .drone-deploy.env > "${tmp_env}"
  if ! grep -q '^deploy_required=' "${tmp_env}"; then
    echo "deploy_required=false" >> "${tmp_env}"
  fi
  mv "${tmp_env}" .drone-deploy.env
fi
