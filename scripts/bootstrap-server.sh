#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/agents-im}"
MIDDLEWARE_DIR="${APP_DIR}/middleware"
NAMESPACE="${NAMESPACE:-agents-im}"
POSTGRES_DB="${POSTGRES_DB:-agents_im}"
POSTGRES_USER="${POSTGRES_USER:-agents_im}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"
JWT_ACCESS_SECRET="${JWT_ACCESS_SECRET:-}"
DEEPSEEK_API_KEY="${DEEPSEEK_API_KEY:-}"
GITHUB_ACTOR="${GITHUB_ACTOR:-}"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"

random_hex() {
  openssl rand -hex 32
}

POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-$(random_hex)}"
REDIS_PASSWORD="${REDIS_PASSWORD:-$(random_hex)}"
JWT_ACCESS_SECRET="${JWT_ACCESS_SECRET:-$(random_hex)}"

install -d -m 0755 "${MIDDLEWARE_DIR}"
cp deploy/middleware/docker-compose.yml "${MIDDLEWARE_DIR}/docker-compose.yml"
cat > "${MIDDLEWARE_DIR}/.env" <<ENV
POSTGRES_DB=${POSTGRES_DB}
POSTGRES_USER=${POSTGRES_USER}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
POSTGRES_PORT=127.0.0.1:5432
REDIS_PASSWORD=${REDIS_PASSWORD}
REDIS_PORT=127.0.0.1:6379
REDPANDA_KAFKA_PORT=127.0.0.1:19092
REDPANDA_ADMIN_PORT=127.0.0.1:9644
ENV
chmod 600 "${MIDDLEWARE_DIR}/.env"

docker compose --env-file "${MIDDLEWARE_DIR}/.env" -f "${MIDDLEWARE_DIR}/docker-compose.yml" up -d

if ! command -v psql >/dev/null 2>&1; then
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y postgresql-client
fi

kubectl apply -f deploy/k8s/namespace.yaml
kubectl -n "${NAMESPACE}" create secret generic agents-im-secrets \
  --from-literal=JWT_ACCESS_SECRET="${JWT_ACCESS_SECRET}" \
  --from-literal=DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@127.0.0.1:5432/${POSTGRES_DB}?sslmode=disable" \
  --from-literal=REDIS_PASSWORD="${REDIS_PASSWORD}" \
  --from-literal=DEEPSEEK_API_KEY="${DEEPSEEK_API_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

if [[ -n "${GITHUB_ACTOR}" && -n "${GITHUB_TOKEN}" ]]; then
  kubectl -n "${NAMESPACE}" create secret docker-registry ghcr-pull-secret \
    --docker-server=ghcr.io \
    --docker-username="${GITHUB_ACTOR}" \
    --docker-password="${GITHUB_TOKEN}" \
    --dry-run=client -o yaml | kubectl apply -f -
fi

printf 'server bootstrap complete: %s\n' "${APP_DIR}"
