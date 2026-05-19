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

for service in ${image_services_space:-}; do
  case "${service}" in
    web)
      target="web"
      cache_name="web"
      build_args=()
      ;;
    *)
      target="backend"
      # Backend services share most Dockerfile layers. Use one backend cache so
      # a warm cache can satisfy subsequent service builds instead of exporting
      # and importing one cache image per service.
      cache_name="backend"
      build_args=(--build-arg "SERVICE=${service}")
      ;;
  esac

  cache_ref="${registry}/cache:${cache_scope}-${cache_name}-${cache_tag}"
  echo "Building ${service} image with target ${target}; cache=${cache_ref}."
  start_epoch="$(date +%s)"
  docker buildx build \
    --builder "${builder}" \
    --target "${target}" \
    "${build_args[@]}" \
    --cache-from "type=registry,ref=${cache_ref}" \
    --cache-to "type=registry,ref=${cache_ref},mode=max,image-manifest=true,oci-mediatypes=true" \
    --tag "${registry}/${service}:${commit_sha}" \
    --tag "${registry}/${service}:latest" \
    --push \
    .
  end_epoch="$(date +%s)"
  echo "Built and pushed ${service} in $((end_epoch - start_epoch))s."
done
