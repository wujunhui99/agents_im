#!/usr/bin/env bash
set -euo pipefail

source ./.drone-deploy.env

if [[ "${deploy_required}" != "true" ]]; then
  echo "No deployment required."
  exit 0
fi

: "${GHCR_USERNAME:?GHCR_USERNAME Drone secret is required}"
: "${GHCR_TOKEN:?GHCR_TOKEN Drone secret is required}"
: "${DRONE_COMMIT_SHA:?DRONE_COMMIT_SHA is required}"

registry="${IMAGE_REGISTRY:-ghcr.io/wujunhui99/agents_im}"

if [[ -n "${DRONE_DEPLOY_LOCAL:-}" ]]; then
  echo "Running deployment locally inside the Drone runner host."
  mkdir -p /opt/agents-im/repo
  tar \
    --exclude='.git' \
    --exclude='.dev' \
    --exclude='web/node_modules' \
    --exclude='web/dist' \
    -czf - . | tar -xzf - -C /opt/agents-im/repo

  restart_rollout=false
  if [[ -n "${restart_services}" ]]; then
    restart_rollout=true
  fi

  skip_set_image=true
  if [[ "${build_required}" == "true" ]]; then
    skip_set_image=false
  fi

  cd /opt/agents-im/repo
  IMAGE_REGISTRY="${registry}" \
    IMAGE_TAG="${DRONE_COMMIT_SHA}" \
    GHCR_USERNAME="${GHCR_USERNAME}" \
    GHCR_TOKEN="${GHCR_TOKEN}" \
    DATABASE_URL="${DATABASE_URL:-}" \
    KUBECONFIG="${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}" \
    SKIP_SET_IMAGE="${skip_set_image}" \
    SKIP_MIDDLEWARE="${config_only}" \
    SKIP_MIGRATIONS="${config_only}" \
    IMAGE_SERVICES="${image_services_space}" \
    ROLLOUT_SERVICES="${rollout_services}" \
    RESTART_SERVICES="${restart_services}" \
    RESTART_ROLLOUT="${restart_rollout}" \
    MIDDLEWARE_DIR=/opt/agents-im/middleware \
    ./scripts/deploy-k3s.sh
  exit 0
fi

: "${DEPLOY_SSH_HOST:?DEPLOY_SSH_HOST Drone secret is required}"
: "${DEPLOY_SSH_USER:?DEPLOY_SSH_USER Drone secret is required}"
: "${DEPLOY_SSH_KEY:?DEPLOY_SSH_KEY Drone secret is required}"
port="${DEPLOY_SSH_PORT:-22}"
remote="${DEPLOY_SSH_USER}@${DEPLOY_SSH_HOST}"
key_file="$(mktemp)"
trap 'rm -f "${key_file}"' EXIT

printf '%s\n' "${DEPLOY_SSH_KEY}" > "${key_file}"
chmod 600 "${key_file}"
ssh_opts=(-i "${key_file}" -o StrictHostKeyChecking=accept-new -p "${port}")

ssh "${ssh_opts[@]}" "${remote}" 'mkdir -p /opt/agents-im/repo'
tar \
  --exclude='.git' \
  --exclude='.dev' \
  --exclude='web/node_modules' \
  --exclude='web/dist' \
  -czf - . | ssh "${ssh_opts[@]}" "${remote}" 'tar -xzf - -C /opt/agents-im/repo'

q() {
  printf '%q' "$1"
}

restart_rollout=false
if [[ -n "${restart_services}" ]]; then
  restart_rollout=true
fi

skip_set_image=true
if [[ "${build_required}" == "true" ]]; then
  skip_set_image=false
fi

remote_cmd="cd /opt/agents-im/repo"
remote_cmd+=" && IMAGE_REGISTRY=$(q "${registry}")"
remote_cmd+=" IMAGE_TAG=$(q "${DRONE_COMMIT_SHA}")"
remote_cmd+=" GHCR_USERNAME=$(q "${GHCR_USERNAME}")"
remote_cmd+=" GHCR_TOKEN=$(q "${GHCR_TOKEN}")"
remote_cmd+=" DATABASE_URL=$(q "${DATABASE_URL:-}")"
remote_cmd+=" SKIP_SET_IMAGE=$(q "${skip_set_image}")"
remote_cmd+=" SKIP_MIDDLEWARE=$(q "${config_only}")"
remote_cmd+=" SKIP_MIGRATIONS=$(q "${config_only}")"
remote_cmd+=" IMAGE_SERVICES=$(q "${image_services_space}")"
remote_cmd+=" ROLLOUT_SERVICES=$(q "${rollout_services}")"
remote_cmd+=" RESTART_SERVICES=$(q "${restart_services}")"
remote_cmd+=" RESTART_ROLLOUT=$(q "${restart_rollout}")"
remote_cmd+=" MIDDLEWARE_DIR=/opt/agents-im/middleware ./scripts/deploy-k3s.sh"

ssh "${ssh_opts[@]}" "${remote}" "${remote_cmd}"
