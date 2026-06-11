#!/usr/bin/env bash
# Drone deploy step：在 runner host 本地执行部署（.drone.yml 固定 DRONE_DEPLOY_LOCAL=1）。
# 历史上的 SSH 远程部署分支已删除（#486）：死代码且 SKIP_MIDDLEWARE 误绑迁移开关。
set -euo pipefail

source ./.drone-deploy.env

if [[ "${deploy_required}" != "true" ]]; then
  echo "No deployment required."
  exit 0
fi

: "${GHCR_USERNAME:?GHCR_USERNAME Drone secret is required}"
: "${GHCR_TOKEN:?GHCR_TOKEN Drone secret is required}"
: "${DRONE_COMMIT_SHA:?DRONE_COMMIT_SHA is required}"
: "${DRONE_DEPLOY_LOCAL:?DRONE_DEPLOY_LOCAL=1 is required (remote SSH deploy was retired in #486)}"

registry="${IMAGE_REGISTRY:-ghcr.io/wujunhui99/agents_im}"

echo "Running deployment locally inside the Drone runner host."
mkdir -p /opt/agents-im/repo
tar \
  --exclude='.git' \
  --exclude='.dev' \
  --exclude='web/node_modules' \
  --exclude='web/dist' \
  -czf - . | tar -xzf - -C /opt/agents-im/repo

# Always restart all app services so pods pick up the latest secret values.
restart_rollout=true

skip_set_image=true
if [[ "${build_required}" == "true" ]]; then
  skip_set_image=false
fi

cd /opt/agents-im/repo

skip_migrations=true
if [[ "${migration_required:-false}" == "true" ]]; then
  skip_migrations=false
fi

if [[ "${skip_migrations}" != "true" ]]; then
  migration_port="${DRONE_LOCAL_MIGRATION_PORT:-5432}"
  kubeconfig="${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}"
  database_url="$(kubectl --kubeconfig "${kubeconfig}" -n agents-im get secret agents-im-secrets -o jsonpath='{.data.DATABASE_URL}' | base64 -d)"
  if [[ -z "${database_url}" ]]; then
    echo "DATABASE_URL is missing in agents-im/agents-im-secrets" >&2
    exit 1
  fi
  dsn_prefix="${database_url%%@*}"
  dsn_suffix="${database_url#*@}"
  dsn_path="${dsn_suffix#*/}"
  if [[ "${dsn_prefix}" == "${database_url}" || "${dsn_path}" == "${dsn_suffix}" ]]; then
    echo "DATABASE_URL format is unsupported for local Drone migration rewrite" >&2
    exit 1
  fi
  # PostgreSQL 已迁 k3s（DSN host 是集群内 DNS，docker 解析不了）。改写为 postgres Service 的
  # ClusterIP，迁移容器用 --network host —— node 的 kube-proxy 可路由 ClusterIP。
  migration_host="${DRONE_LOCAL_MIGRATION_HOST:-$(kubectl --kubeconfig "${kubeconfig}" -n agents-im get svc postgres -o jsonpath='{.spec.clusterIP}')}"
  if [[ -z "${migration_host}" ]]; then
    echo "could not resolve postgres ClusterIP in agents-im namespace" >&2
    exit 1
  fi
  echo "Running database migrations against k3s postgres ${migration_host}:${migration_port} (host network)."
  migration_database_url="${dsn_prefix}@${migration_host}:${migration_port}/${dsn_path}"
  docker run --rm \
    --network host \
    -v "${PWD}:/repo" \
    -w /repo \
    -e DATABASE_URL="${migration_database_url}" \
    postgres:16-alpine \
    sh -lc 'apk add --no-cache bash coreutils >/dev/null && bash scripts/migrate-postgres.sh --host-psql'
else
  echo "Skipping database migrations: migration_required=${migration_required:-false}."
fi

IMAGE_REGISTRY="${registry}" \
  IMAGE_TAG="${DRONE_COMMIT_SHA}" \
  GHCR_USERNAME="${GHCR_USERNAME}" \
  GHCR_TOKEN="${GHCR_TOKEN}" \
  KUBECONFIG="${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}" \
  SKIP_SET_IMAGE="${skip_set_image}" \
  SKIP_MIGRATIONS="true" \
  IMAGE_SERVICES="${image_services_space}" \
  ROLLOUT_SERVICES="${rollout_services}" \
  RESTART_SERVICES="${restart_services}" \
  RESTART_ROLLOUT="${restart_rollout}" \
  ./scripts/deploy-k3s.sh
