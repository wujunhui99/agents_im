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

echo "${GHCR_TOKEN}" | docker login ghcr.io -u "${GHCR_USERNAME}" --password-stdin

for service in ${image_services_space:-}; do
  case "${service}" in
    web)
      target="web"
      build_args=()
      ;;
    *)
      target="backend"
      build_args=(--build-arg "SERVICE=${service}")
      ;;
  esac

  echo "Building ${service} image with target ${target}."
  docker build \
    --target "${target}" \
    "${build_args[@]}" \
    -t "${registry}/${service}:${commit_sha}" \
    -t "${registry}/${service}:latest" \
    .
  docker push "${registry}/${service}:${commit_sha}"
  docker push "${registry}/${service}:latest"
done
