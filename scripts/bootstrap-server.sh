#!/usr/bin/env bash
# 新服务器引导（2026-06 重装版）：中间件跑 k8s（deploy/k8s/middleware），Drone CI 跑 k3s
# （deploy/k8s/drone），不再使用 docker compose 中间件。
#
# 前置（脚本会检查）：
#   - k3s 已装（kubectl 可用，KUBECONFIG=/etc/rancher/k3s/k3s.yaml）
#   - docker 已装（Drone docker runner 的 step 容器跑在宿主 docker）
#   - 仓库已在本机（从仓库根目录执行）
#   - /opt/agents-im/creds.env 已就位（0600），提供第三方凭据：
#       DRONE_GITHUB_CLIENT_ID / DRONE_GITHUB_CLIENT_SECRET（GitHub OAuth App，
#         callback https://drone.agenticim.xyz/login）
#       GHCR_USERNAME / GHCR_TOKEN（read:packages + write:packages）
#       TELEGRAM_BOT_TOKEN / TELEGRAM_CHAT_ID
#       DEEPSEEK_API_KEY
#       TENCENT_SES_SECRET_ID / TENCENT_SES_SECRET_KEY / TENCENT_SES_REGION /
#         TENCENT_SES_FROM_EMAIL / TENCENT_SES_DEFAULT_TEMPLATE_ID
#   - ADMIN_BOOTSTRAP_PASSWORD 经环境变量传入（管理后台 admin 账号密码）
#
# 应用部署不在本脚本内：bootstrap 完成后由 Drone deploy-main（merge 到 main）部署，
# 或手工 `IMAGE_TAG=<sha> SKIP_MIDDLEWARE=true SKIP_MIGRATIONS=true ./scripts/deploy-k3s.sh`。
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/agents-im}"
ENV_FILE="${APP_DIR}/secrets.env"
CREDS_FILE="${APP_DIR}/creds.env"
NAMESPACE="${NAMESPACE:-agents-im}"
CERT_MANAGER_VERSION="${CERT_MANAGER_VERSION:-v1.18.2}"
export KUBECONFIG="${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}"

require() {
  command -v "$1" >/dev/null 2>&1 || { echo "required command not found: $1" >&2; exit 1; }
}
require kubectl
require docker
require openssl

if [[ ! -f "${CREDS_FILE}" ]]; then
  echo "missing ${CREDS_FILE}; create it (0600) with third-party credentials first" >&2
  exit 1
fi
: "${ADMIN_BOOTSTRAP_PASSWORD:?ADMIN_BOOTSTRAP_PASSWORD env is required}"

install -d -m 0700 "${APP_DIR}"
install -d -m 0755 "${APP_DIR}/backups"

# ---------- 1. 生成/加载本机 secrets ----------
if [[ ! -f "${ENV_FILE}" ]]; then
  cat > "${ENV_FILE}" <<ENV
POSTGRES_USER=agents_im
POSTGRES_DB=agents_im
POSTGRES_PASSWORD=$(openssl rand -hex 24)
REDIS_PASSWORD=$(openssl rand -hex 24)
MINIO_ROOT_USER=agents_im_minio
MINIO_ROOT_PASSWORD=$(openssl rand -hex 24)
JWT_ACCESS_SECRET=$(openssl rand -hex 32)
LANGFUSE_NEXTAUTH_SECRET=$(openssl rand -base64 32 | tr -d '\n')
LANGFUSE_SALT=$(openssl rand -hex 32)
LANGFUSE_ENCRYPTION_KEY=$(openssl rand -hex 32)
OBSERVABILITY_BASIC_AUTH_USER=admin
OBSERVABILITY_BASIC_AUTH_PASSWORD=$(openssl rand -base64 18 | tr -d '\n')
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=$(openssl rand -base64 18 | tr -d '\n')
DRONE_RPC_SECRET=$(openssl rand -hex 16)
DRONE_ADMIN_TOKEN=$(openssl rand -hex 16)
ENV
  chmod 600 "${ENV_FILE}"
  echo "generated ${ENV_FILE}"
fi
set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
# shellcheck disable=SC1090
source "${CREDS_FILE}"
set +a

PG_HOST=postgres.${NAMESPACE}.svc.cluster.local
DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${PG_HOST}:5432/${POSTGRES_DB}?sslmode=disable"
LANGFUSE_DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${PG_HOST}:5432/langfuse?sslmode=disable"

# ---------- 2. namespace + cert-manager ----------
kubectl apply -f deploy/k8s/namespace.yaml
if ! kubectl get ns cert-manager >/dev/null 2>&1; then
  kubectl apply -f "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml"
fi
kubectl -n cert-manager rollout status deploy/cert-manager-webhook --timeout=300s
kubectl apply -f deploy/k8s/cert-manager-issuers.yaml

# ---------- 3. 应用 secrets ----------
kubectl -n "${NAMESPACE}" create secret generic agents-im-secrets \
  --from-literal=JWT_ACCESS_SECRET="${JWT_ACCESS_SECRET}" \
  --from-literal=ADMIN_BOOTSTRAP_IDENTIFIER="${ADMIN_BOOTSTRAP_IDENTIFIER:-admin}" \
  --from-literal=ADMIN_BOOTSTRAP_PASSWORD="${ADMIN_BOOTSTRAP_PASSWORD}" \
  --from-literal=ADMIN_BOOTSTRAP_DISPLAY_NAME="${ADMIN_BOOTSTRAP_DISPLAY_NAME:-管理后台管理员}" \
  --from-literal=DATABASE_URL="${DATABASE_URL}" \
  --from-literal=POSTGRES_USER="${POSTGRES_USER}" \
  --from-literal=POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
  --from-literal=POSTGRES_DB="${POSTGRES_DB}" \
  --from-literal=REDIS_PASSWORD="${REDIS_PASSWORD}" \
  --from-literal=OBJECT_STORAGE_DRIVER="minio" \
  --from-literal=OBJECT_STORAGE_ENDPOINT="minio.${NAMESPACE}.svc.cluster.local:9000" \
  --from-literal=OBJECT_STORAGE_EXTERNAL_ENDPOINT="${OBJECT_STORAGE_EXTERNAL_ENDPOINT:-agenticim.xyz}" \
  --from-literal=OBJECT_STORAGE_BUCKET="agents-im-media" \
  --from-literal=OBJECT_STORAGE_REGION="us-east-1" \
  --from-literal=OBJECT_STORAGE_USE_SSL="false" \
  --from-literal=OBJECT_STORAGE_EXTERNAL_USE_SSL="true" \
  --from-literal=OBJECT_STORAGE_ACCESS_KEY_ID="${MINIO_ROOT_USER}" \
  --from-literal=OBJECT_STORAGE_SECRET_ACCESS_KEY="${MINIO_ROOT_PASSWORD}" \
  --from-literal=DEEPSEEK_API_KEY="${DEEPSEEK_API_KEY:?missing in creds.env}" \
  --from-literal=LANGFUSE_PUBLIC_KEY="${LANGFUSE_PUBLIC_KEY:-}" \
  --from-literal=LANGFUSE_SECRET_KEY="${LANGFUSE_SECRET_KEY:-}" \
  --from-literal=LANGFUSE_DATABASE_URL="${LANGFUSE_DATABASE_URL}" \
  --from-literal=NEXTAUTH_SECRET="${LANGFUSE_NEXTAUTH_SECRET}" \
  --from-literal=SALT="${LANGFUSE_SALT}" \
  --from-literal=ENCRYPTION_KEY="${LANGFUSE_ENCRYPTION_KEY}" \
  --from-literal=TENCENT_SES_SECRET_ID="${TENCENT_SES_SECRET_ID:?missing in creds.env}" \
  --from-literal=TENCENT_SES_SECRET_KEY="${TENCENT_SES_SECRET_KEY:?missing in creds.env}" \
  --from-literal=TENCENT_SES_REGION="${TENCENT_SES_REGION:-ap-hongkong}" \
  --from-literal=TENCENT_SES_FROM_EMAIL="${TENCENT_SES_FROM_EMAIL:?missing in creds.env}" \
  --from-literal=TENCENT_SES_DEFAULT_TEMPLATE_ID="${TENCENT_SES_DEFAULT_TEMPLATE_ID:?missing in creds.env}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${NAMESPACE}" create secret generic observability-basic-auth \
  --from-literal=users="${OBSERVABILITY_BASIC_AUTH_USER}:$(openssl passwd -apr1 "${OBSERVABILITY_BASIC_AUTH_PASSWORD}")" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${NAMESPACE}" create secret generic grafana-admin \
  --from-literal=admin-user="${GRAFANA_ADMIN_USER}" \
  --from-literal=admin-password="${GRAFANA_ADMIN_PASSWORD}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${NAMESPACE}" create secret docker-registry ghcr-pull-secret \
  --docker-server=ghcr.io \
  --docker-username="${GHCR_USERNAME:?missing in creds.env}" \
  --docker-password="${GHCR_TOKEN:?missing in creds.env}" \
  --dry-run=client -o yaml | kubectl apply -f -

# ---------- 4. 中间件（k8s StatefulSet）----------
kubectl apply -f deploy/k8s/middleware/postgres.yaml
kubectl apply -f deploy/k8s/middleware/redis.yaml
kubectl apply -f deploy/k8s/middleware/minio.yaml
kubectl apply -f deploy/k8s/middleware/redpanda.yaml
kubectl -n "${NAMESPACE}" rollout status statefulset/postgres --timeout=300s
kubectl -n "${NAMESPACE}" rollout status statefulset/redis --timeout=300s
kubectl -n "${NAMESPACE}" rollout status statefulset/minio --timeout=300s
kubectl -n "${NAMESPACE}" rollout status statefulset/redpanda --timeout=300s

# ---------- 5. langfuse 库 + media bucket ----------
kubectl -n "${NAMESPACE}" exec statefulset/postgres -- \
  sh -c "psql -U ${POSTGRES_USER} -d ${POSTGRES_DB} -tc \"SELECT 1 FROM pg_database WHERE datname='langfuse'\" | grep -q 1 || createdb -U ${POSTGRES_USER} langfuse"
kubectl -n "${NAMESPACE}" exec statefulset/minio -- \
  sh -c "mc alias set local http://127.0.0.1:9000 \"\$MINIO_ROOT_USER\" \"\$MINIO_ROOT_PASSWORD\" >/dev/null && mc mb --ignore-existing local/agents-im-media"

# ---------- 6. 数据库迁移（经 ClusterIP，宿主 psql 或 docker）----------
PG_IP="$(kubectl -n "${NAMESPACE}" get svc postgres -o jsonpath='{.spec.clusterIP}')"
MIGRATION_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${PG_IP}:5432/${POSTGRES_DB}?sslmode=disable"
if command -v psql >/dev/null 2>&1; then
  DATABASE_URL="${MIGRATION_URL}" bash scripts/migrate-postgres.sh --host-psql
else
  docker run --rm --network host -v "${PWD}:/repo" -w /repo \
    -e DATABASE_URL="${MIGRATION_URL}" postgres:16-alpine \
    sh -lc 'apk add --no-cache bash coreutils >/dev/null && bash scripts/migrate-postgres.sh --host-psql'
fi

# ---------- 7. Drone CI on k3s ----------
kubectl create namespace drone --dry-run=client -o yaml | kubectl apply -f -
kubectl -n drone create secret generic drone-secrets \
  --from-literal=DRONE_GITHUB_CLIENT_ID="${DRONE_GITHUB_CLIENT_ID:?missing in creds.env}" \
  --from-literal=DRONE_GITHUB_CLIENT_SECRET="${DRONE_GITHUB_CLIENT_SECRET:?missing in creds.env}" \
  --from-literal=DRONE_RPC_SECRET="${DRONE_RPC_SECRET}" \
  --from-literal=DRONE_USER_CREATE="username:wujunhui99,admin:true,token:${DRONE_ADMIN_TOKEN}" \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f deploy/k8s/drone/drone.yaml
kubectl -n drone rollout status deploy/drone-server --timeout=300s
kubectl -n drone rollout status deploy/drone-runner-docker --timeout=300s

cat <<'NEXT'
bootstrap complete. 接下来人工完成：
  1. 浏览器打开 https://drone.agenticim.xyz 用 GitHub 登录授权 OAuth（同步仓库列表必需）。
  2. 用 DRONE_ADMIN_TOKEN（secrets.env）经 API 激活仓库并配置：
       POST /api/user/repos?async=false
       POST /api/repos/wujunhui99/agents_im            # activate（自动建 webhook）
       PATCH /api/repos/wujunhui99/agents_im {"trusted": true}   # .drone.yml host volume 必需
       POST /api/repos/wujunhui99/agents_im/secrets    # ghcr_username/ghcr_token/telegram_bot_token/telegram_chat_id
  3. 首次全量部署：push devops 分支触发全量镜像构建，然后
       IMAGE_TAG=<sha> SKIP_MIDDLEWARE=true SKIP_MIGRATIONS=true ./scripts/deploy-k3s.sh
NEXT
