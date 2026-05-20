#!/usr/bin/env bash
set -euo pipefail

: "${GHCR_USERNAME:?GHCR_USERNAME is required}"
: "${GHCR_TOKEN:?GHCR_TOKEN is required}"

registry="${IMAGE_REGISTRY:-ghcr.io/wujunhui99/agents_im}"
image="${CI_DEPLOY_IMAGE:-${registry}/ci-deploy}"
tag="${CI_DEPLOY_IMAGE_TAG:-latest}"
commit_tag="${DRONE_COMMIT_SHA:-}"
dockerfile="${CI_DEPLOY_DOCKERFILE:-deploy/ci/Dockerfile.deploy}"

export DOCKER_BUILDKIT=1

echo "${GHCR_TOKEN}" | docker login ghcr.io -u "${GHCR_USERNAME}" --password-stdin

tags=(--tag "${image}:${tag}")
if [[ -n "${commit_tag}" ]]; then
  tags+=(--tag "${image}:${commit_tag}")
fi

docker buildx build \
  --file "${dockerfile}" \
  "${tags[@]}" \
  --provenance=false \
  --push \
  .
