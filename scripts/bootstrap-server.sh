#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/agents-im}"
MIDDLEWARE_DIR="${APP_DIR}/middleware"
NAMESPACE="${NAMESPACE:-agents-im}"
POSTGRES_DB="${POSTGRES_DB:-agents_im}"
LANGFUSE_POSTGRES_DB="${LANGFUSE_POSTGRES_DB:-langfuse}"
POSTGRES_USER="${POSTGRES_USER:-agents_im}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"
MINIO_ROOT_USER="${MINIO_ROOT_USER:-agents_im_minio}"
MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD:-}"
JWT_ACCESS_SECRET="${JWT_ACCESS_SECRET:-}"
DEEPSEEK_API_KEY="${DEEPSEEK_API_KEY:-}"
LANGFUSE_PUBLIC_KEY="${LANGFUSE_PUBLIC_KEY:-}"
LANGFUSE_SECRET_KEY="${LANGFUSE_SECRET_KEY:-}"
LANGFUSE_DATABASE_URL="${LANGFUSE_DATABASE_URL:-}"
LANGFUSE_NEXTAUTH_SECRET="${LANGFUSE_NEXTAUTH_SECRET:-}"
LANGFUSE_SALT="${LANGFUSE_SALT:-}"
LANGFUSE_ENCRYPTION_KEY="${LANGFUSE_ENCRYPTION_KEY:-}"
OBSERVABILITY_BASIC_AUTH_USER="${OBSERVABILITY_BASIC_AUTH_USER:-admin}"
OBSERVABILITY_BASIC_AUTH_PASSWORD="${OBSERVABILITY_BASIC_AUTH_PASSWORD:-}"
GRAFANA_ADMIN_USER="${GRAFANA_ADMIN_USER:-admin}"
GRAFANA_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-}"
OBJECT_STORAGE_EXTERNAL_ENDPOINT="${OBJECT_STORAGE_EXTERNAL_ENDPOINT:-}"
OBJECT_STORAGE_EXTERNAL_USE_SSL="${OBJECT_STORAGE_EXTERNAL_USE_SSL:-true}"
GITHUB_ACTOR="${GITHUB_ACTOR:-}"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"

random_hex() {
  openssl rand -hex 32
}

is_browser_local_endpoint() {
  local endpoint="${1#http://}"
  endpoint="${endpoint#https://}"
  endpoint="${endpoint%%/*}"
  if [[ "${endpoint}" == \[*\]* ]]; then
    endpoint="${endpoint#[}"
    endpoint="${endpoint%%]*}"
  elif [[ "${endpoint}" != *:*:* ]]; then
    endpoint="${endpoint%%:*}"
  fi
  case "${endpoint}" in
    localhost|*.localhost|127.*|0.0.0.0|::1|::)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-$(random_hex)}"
REDIS_PASSWORD="${REDIS_PASSWORD:-$(random_hex)}"
MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD:-$(random_hex)}"
JWT_ACCESS_SECRET="${JWT_ACCESS_SECRET:-$(random_hex)}"
LANGFUSE_DATABASE_URL="${LANGFUSE_DATABASE_URL:-postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@127.0.0.1:5432/${LANGFUSE_POSTGRES_DB}?sslmode=disable}"
LANGFUSE_NEXTAUTH_SECRET="${LANGFUSE_NEXTAUTH_SECRET:-$(openssl rand -base64 32)}"
LANGFUSE_SALT="${LANGFUSE_SALT:-$(random_hex)}"
LANGFUSE_ENCRYPTION_KEY="${LANGFUSE_ENCRYPTION_KEY:-$(random_hex)}"
OBSERVABILITY_BASIC_AUTH_PASSWORD="${OBSERVABILITY_BASIC_AUTH_PASSWORD:-$(openssl rand -base64 24)}"
GRAFANA_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-$(openssl rand -base64 24)}"

if [[ -z "${OBJECT_STORAGE_EXTERNAL_ENDPOINT}" ]]; then
  echo "OBJECT_STORAGE_EXTERNAL_ENDPOINT is required for browser uploads in production" >&2
  exit 1
fi
if is_browser_local_endpoint "${OBJECT_STORAGE_EXTERNAL_ENDPOINT}"; then
  echo "OBJECT_STORAGE_EXTERNAL_ENDPOINT must be browser-reachable, not loopback" >&2
  exit 1
fi

install -d -m 0755 "${MIDDLEWARE_DIR}"
cp deploy/middleware/docker-compose.yml "${MIDDLEWARE_DIR}/docker-compose.yml"
cat > "${MIDDLEWARE_DIR}/.env" <<ENV
POSTGRES_DB=${POSTGRES_DB}
POSTGRES_USER=${POSTGRES_USER}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
POSTGRES_PORT=127.0.0.1:5432
REDIS_PASSWORD=${REDIS_PASSWORD}
REDIS_PORT=127.0.0.1:6379
MINIO_ROOT_USER=${MINIO_ROOT_USER}
MINIO_ROOT_PASSWORD=${MINIO_ROOT_PASSWORD}
MINIO_API_PORT=127.0.0.1:9000
MINIO_CONSOLE_PORT=127.0.0.1:9001
OBJECT_STORAGE_DRIVER=minio
OBJECT_STORAGE_ENDPOINT=127.0.0.1:9000
OBJECT_STORAGE_EXTERNAL_ENDPOINT=${OBJECT_STORAGE_EXTERNAL_ENDPOINT}
OBJECT_STORAGE_BUCKET=agents-im-media
OBJECT_STORAGE_REGION=us-east-1
OBJECT_STORAGE_USE_SSL=false
OBJECT_STORAGE_EXTERNAL_USE_SSL=${OBJECT_STORAGE_EXTERNAL_USE_SSL}
OBJECT_STORAGE_ACCESS_KEY_ID=${MINIO_ROOT_USER}
OBJECT_STORAGE_SECRET_ACCESS_KEY=${MINIO_ROOT_PASSWORD}
ENV
chmod 600 "${MIDDLEWARE_DIR}/.env"

docker compose --env-file "${MIDDLEWARE_DIR}/.env" -f "${MIDDLEWARE_DIR}/docker-compose.yml" up -d

if ! command -v psql >/dev/null 2>&1; then
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y postgresql-client
fi

PGPASSWORD="${POSTGRES_PASSWORD}" psql -h 127.0.0.1 -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -tc "SELECT 1 FROM pg_database WHERE datname = '${LANGFUSE_POSTGRES_DB}'" | grep -q 1 || \
  PGPASSWORD="${POSTGRES_PASSWORD}" createdb -h 127.0.0.1 -U "${POSTGRES_USER}" "${LANGFUSE_POSTGRES_DB}"

kubectl apply -f deploy/k8s/namespace.yaml
kubectl -n "${NAMESPACE}" create secret generic agents-im-secrets \
  --from-literal=JWT_ACCESS_SECRET="${JWT_ACCESS_SECRET}" \
  --from-literal=DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@127.0.0.1:5432/${POSTGRES_DB}?sslmode=disable" \
  --from-literal=REDIS_PASSWORD="${REDIS_PASSWORD}" \
  --from-literal=OBJECT_STORAGE_DRIVER="minio" \
  --from-literal=OBJECT_STORAGE_ENDPOINT="127.0.0.1:9000" \
  --from-literal=OBJECT_STORAGE_EXTERNAL_ENDPOINT="${OBJECT_STORAGE_EXTERNAL_ENDPOINT}" \
  --from-literal=OBJECT_STORAGE_BUCKET="agents-im-media" \
  --from-literal=OBJECT_STORAGE_REGION="us-east-1" \
  --from-literal=OBJECT_STORAGE_USE_SSL="false" \
  --from-literal=OBJECT_STORAGE_EXTERNAL_USE_SSL="${OBJECT_STORAGE_EXTERNAL_USE_SSL}" \
  --from-literal=OBJECT_STORAGE_ACCESS_KEY_ID="${MINIO_ROOT_USER}" \
  --from-literal=OBJECT_STORAGE_SECRET_ACCESS_KEY="${MINIO_ROOT_PASSWORD}" \
  --from-literal=DEEPSEEK_API_KEY="${DEEPSEEK_API_KEY}" \
  --from-literal=LANGFUSE_PUBLIC_KEY="${LANGFUSE_PUBLIC_KEY}" \
  --from-literal=LANGFUSE_SECRET_KEY="${LANGFUSE_SECRET_KEY}" \
  --from-literal=LANGFUSE_DATABASE_URL="${LANGFUSE_DATABASE_URL}" \
  --from-literal=NEXTAUTH_SECRET="${LANGFUSE_NEXTAUTH_SECRET}" \
  --from-literal=SALT="${LANGFUSE_SALT}" \
  --from-literal=ENCRYPTION_KEY="${LANGFUSE_ENCRYPTION_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

observability_htpasswd="${OBSERVABILITY_BASIC_AUTH_USER}:$(openssl passwd -apr1 "${OBSERVABILITY_BASIC_AUTH_PASSWORD}")"
kubectl -n "${NAMESPACE}" create secret generic observability-basic-auth \
  --from-literal=users="${observability_htpasswd}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${NAMESPACE}" create secret generic grafana-admin \
  --from-literal=admin-user="${GRAFANA_ADMIN_USER}" \
  --from-literal=admin-password="${GRAFANA_ADMIN_PASSWORD}" \
  --dry-run=client -o yaml | kubectl apply -f -

install -d -m 0700 "${APP_DIR}"
{
  printf 'OBSERVABILITY_BASIC_AUTH_USER=%s\n' "${OBSERVABILITY_BASIC_AUTH_USER}"
  printf 'OBSERVABILITY_BASIC_AUTH_PASSWORD=%s\n' "${OBSERVABILITY_BASIC_AUTH_PASSWORD}"
} > "${APP_DIR}/observability-basic-auth.env"
chmod 600 "${APP_DIR}/observability-basic-auth.env"
{
  printf 'GRAFANA_ADMIN_USER=%s\n' "${GRAFANA_ADMIN_USER}"
  printf 'GRAFANA_ADMIN_PASSWORD=%s\n' "${GRAFANA_ADMIN_PASSWORD}"
} > "${APP_DIR}/grafana-admin.env"
chmod 600 "${APP_DIR}/grafana-admin.env"

if [[ -n "${GITHUB_ACTOR}" && -n "${GITHUB_TOKEN}" ]]; then
  kubectl -n "${NAMESPACE}" create secret docker-registry ghcr-pull-secret \
    --docker-server=ghcr.io \
    --docker-username="${GITHUB_ACTOR}" \
    --docker-password="${GITHUB_TOKEN}" \
    --dry-run=client -o yaml | kubectl apply -f -
fi

printf 'server bootstrap complete: %s\n' "${APP_DIR}"
