#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STATE_DIR="${AGENTS_IM_DEV_STATE_DIR:-${ROOT_DIR}/.dev}"
CONFIG_DIR="${STATE_DIR}/etc"
BIN_DIR="${STATE_DIR}/bin"
LOG_DIR="${STATE_DIR}/logs"
PID_DIR="${STATE_DIR}/pids"

WITH_SERVICES=1
RUN_MIGRATIONS=1
WITH_MIDDLEWARE=1
ACTION="start"

usage() {
  cat <<'USAGE'
Usage: scripts/dev-up.sh [--middleware-only] [--with-services] [--no-migrate] [--stop]

Starts local Docker middleware, runs migrations, and by default launches host
backend services with PostgreSQL-backed local config.

Options:
  --middleware-only  Start Docker middleware and migrations, but skip Go services.
  --with-services   Start Go services after middleware. This is the default.
  --services-only   Restart only host Go services; skip Docker middleware and migrations.
                   Service ports can be overridden with USER_API_PORT, AUTH_API_PORT,
                   FRIENDS_API_PORT, MESSAGE_API_PORT, GATEWAY_WS_PORT, GROUPS_API_PORT,
                   and AGENT_API_PORT.
  --no-migrate      Skip PostgreSQL migrations.
  --stop            Stop host Go services started by this script.
  -h, --help        Show this help.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --middleware-only)
      WITH_SERVICES=0
      ;;
    --with-services)
      WITH_SERVICES=1
      ;;
    --services-only)
      WITH_SERVICES=1
      WITH_MIDDLEWARE=0
      RUN_MIGRATIONS=0
      ;;
    --no-migrate)
      RUN_MIGRATIONS=0
      ;;
    --stop)
      ACTION="stop"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

load_env() {
  local env_file=""
  if [[ -f "${ROOT_DIR}/.env" ]]; then
    env_file="${ROOT_DIR}/.env"
  elif [[ -f "${ROOT_DIR}/.env.example" ]]; then
    env_file="${ROOT_DIR}/.env.example"
  fi

  if [[ -n "${env_file}" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "${env_file}"
    set +a
  fi

  export POSTGRES_DB="${POSTGRES_DB:-agents_im}"
  export POSTGRES_USER="${POSTGRES_USER:-agents_im}"
  export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-agents_im_dev_password}"
  export POSTGRES_PORT="${POSTGRES_PORT:-5432}"
  export REDIS_PASSWORD="${REDIS_PASSWORD:-agents_im_redis_dev_password}"
  export MINIO_ROOT_USER="${MINIO_ROOT_USER:-agents_im_minio}"
  export MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD:-agents_im_minio_dev_password}"
  export MINIO_API_PORT="${MINIO_API_PORT:-9000}"
  export MINIO_CONSOLE_PORT="${MINIO_CONSOLE_PORT:-9001}"
  export OBJECT_STORAGE_DRIVER="${OBJECT_STORAGE_DRIVER:-minio}"
  export OBJECT_STORAGE_ENDPOINT="${OBJECT_STORAGE_ENDPOINT:-localhost:${MINIO_API_PORT}}"
  export OBJECT_STORAGE_EXTERNAL_ENDPOINT="${OBJECT_STORAGE_EXTERNAL_ENDPOINT:-localhost:${MINIO_API_PORT}}"
  export OBJECT_STORAGE_BUCKET="${OBJECT_STORAGE_BUCKET:-agents-im-media}"
  export OBJECT_STORAGE_REGION="${OBJECT_STORAGE_REGION:-us-east-1}"
  export OBJECT_STORAGE_USE_SSL="${OBJECT_STORAGE_USE_SSL:-false}"
  export OBJECT_STORAGE_ACCESS_KEY_ID="${OBJECT_STORAGE_ACCESS_KEY_ID:-${MINIO_ROOT_USER}}"
  export OBJECT_STORAGE_SECRET_ACCESS_KEY="${OBJECT_STORAGE_SECRET_ACCESS_KEY:-${MINIO_ROOT_PASSWORD}}"
  export DATABASE_URL="${DATABASE_URL:-postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable}"
  export JWT_ACCESS_SECRET="${JWT_ACCESS_SECRET:-dev-jwt-secret-change-me}"
  export JWT_ACCESS_EXPIRE="${JWT_ACCESS_EXPIRE:-86400}"
  export PRESENCE_DRIVER="${PRESENCE_DRIVER:-memory}"
  export GATEWAY_WS_ALLOWED_ORIGINS="${GATEWAY_WS_ALLOWED_ORIGINS:-http://localhost:5173,http://127.0.0.1:5173}"
  export GATEWAY_WS_ALLOW_QUERY_TOKEN="${GATEWAY_WS_ALLOW_QUERY_TOKEN:-true}"
  export GATEWAY_WS_PING_INTERVAL_SECONDS="${GATEWAY_WS_PING_INTERVAL_SECONDS:-30}"
  export GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS="${GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS:-75}"
  export GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND="${GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND:-20}"
  export GATEWAY_WS_COMMAND_RATE_LIMIT_BURST="${GATEWAY_WS_COMMAND_RATE_LIMIT_BURST:-40}"
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "required command not found: $1" >&2
    exit 1
  fi
}

stop_services() {
  if [[ ! -d "${PID_DIR}" ]]; then
    echo "no local backend PID directory found"
    return 0
  fi

  local pid_file pid name
  for pid_file in "${PID_DIR}"/*.pid; do
    [[ -e "${pid_file}" ]] || continue
    name="$(basename "${pid_file}" .pid)"
    pid="$(cat "${pid_file}")"
    if [[ -n "${pid}" ]] && kill -0 "${pid}" >/dev/null 2>&1; then
      echo "stopping ${name} pid=${pid}"
      kill "${pid}" >/dev/null 2>&1 || true
    fi
    rm -f "${pid_file}"
  done
}

wait_for_postgres() {
  local attempt
  for attempt in $(seq 1 60); do
    if docker compose exec -T postgres pg_isready -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "postgres did not become ready" >&2
  exit 1
}

wait_for_minio() {
  local attempt
  for attempt in $(seq 1 60); do
    if curl --silent --fail "http://${OBJECT_STORAGE_ENDPOINT}/minio/health/live" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "minio did not become ready at ${OBJECT_STORAGE_ENDPOINT}" >&2
  exit 1
}

write_api_config() {
  local name="$1"
  local port="$2"
  local extra="${3:-}"

  cat > "${CONFIG_DIR}/${name}.yaml" <<YAML
Name: ${name}
Host: 127.0.0.1
Port: ${port}
Auth:
  AccessSecret: ${JWT_ACCESS_SECRET}
  AccessExpire: ${JWT_ACCESS_EXPIRE}
StorageDriver: postgres
DataSource: ${DATABASE_URL}
${extra}
YAML
}

write_configs() {
  mkdir -p "${CONFIG_DIR}"
  write_api_config "user-api" "${USER_API_PORT:-8080}" "ObjectStorage:
  Driver: ${OBJECT_STORAGE_DRIVER}
  Endpoint: ${OBJECT_STORAGE_ENDPOINT}
  ExternalEndpoint: ${OBJECT_STORAGE_EXTERNAL_ENDPOINT}
  Bucket: ${OBJECT_STORAGE_BUCKET}
  Region: ${OBJECT_STORAGE_REGION}
  UseSSL: ${OBJECT_STORAGE_USE_SSL}
  AccessKeyID: ${OBJECT_STORAGE_ACCESS_KEY_ID}
  SecretAccessKey: ${OBJECT_STORAGE_SECRET_ACCESS_KEY}"
  write_api_config "auth-api" "${AUTH_API_PORT:-8081}"
  write_api_config "friends-api" "${FRIENDS_API_PORT:-8082}"
  write_api_config "message-api" "${MESSAGE_API_PORT:-8083}"
  write_api_config "gateway-ws" "${GATEWAY_WS_PORT:-8084}" "Presence:
  Driver: ${PRESENCE_DRIVER}
  HeartbeatTTLSeconds: ${PRESENCE_TTL_SECONDS:-60}
  KeyPrefix: ${PRESENCE_KEY_PREFIX:-agents_im:presence}
GatewayWS:
  AllowedOrigins: ${GATEWAY_WS_ALLOWED_ORIGINS}
  AllowQueryToken: ${GATEWAY_WS_ALLOW_QUERY_TOKEN}
  PingIntervalSeconds: ${GATEWAY_WS_PING_INTERVAL_SECONDS}
  HeartbeatTimeoutSeconds: ${GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS}
  CommandRateLimitPerSecond: ${GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND}
  CommandRateLimitBurst: ${GATEWAY_WS_COMMAND_RATE_LIMIT_BURST}"
  write_api_config "groups-api" "${GROUPS_API_PORT:-8085}"
  write_api_config "agent-api" "${AGENT_API_PORT:-8086}"
}

build_service() {
  local name="$1"
  mkdir -p "${BIN_DIR}"
  echo "building ${name}"
  go build -o "${BIN_DIR}/${name}" "./cmd/${name}"
}

start_service() {
  local name="$1"
  local config_file="${CONFIG_DIR}/${name}.yaml"
  local pid_file="${PID_DIR}/${name}.pid"
  local log_file="${LOG_DIR}/${name}.log"

  mkdir -p "${LOG_DIR}" "${PID_DIR}"
  if [[ -f "${pid_file}" ]]; then
    local existing_pid
    existing_pid="$(cat "${pid_file}")"
    if [[ -n "${existing_pid}" ]] && kill -0 "${existing_pid}" >/dev/null 2>&1; then
      echo "${name} already running pid=${existing_pid}"
      return 0
    fi
  fi

  build_service "${name}"
  echo "starting ${name}; log=${log_file}"
  nohup "${BIN_DIR}/${name}" -f "${config_file}" > "${log_file}" 2>&1 &
  echo "$!" > "${pid_file}"
}

wait_http() {
  local name="$1"
  local url="$2"
  local attempt
  for attempt in $(seq 1 60); do
    if curl --silent --fail "${url}" >/dev/null 2>&1; then
      echo "${name} ready: ${url}"
      return 0
    fi
    sleep 1
  done
  echo "${name} did not become ready at ${url}" >&2
  exit 1
}

main() {
  cd "${ROOT_DIR}"
  load_env

  if [[ "${ACTION}" == "stop" ]]; then
    stop_services
    return 0
  fi

  if [[ "${WITH_MIDDLEWARE}" -eq 1 ]]; then
    require_command docker
    docker compose up -d postgres redis redpanda minio
    wait_for_postgres
    require_command curl
    wait_for_minio
  fi

  if [[ "${RUN_MIGRATIONS}" -eq 1 ]]; then
    bash scripts/migrate-postgres.sh
  fi

  if [[ "${WITH_SERVICES}" -eq 0 ]]; then
    echo "middleware is ready"
    return 0
  fi

  require_command go
  require_command curl

  stop_services
  write_configs
  start_service "user-api"
  start_service "auth-api"
  start_service "friends-api"
  start_service "message-api"
  start_service "gateway-ws"
  start_service "groups-api"
  start_service "agent-api"

  wait_http "user-api" "http://127.0.0.1:${USER_API_PORT:-8080}/healthz"
  wait_http "auth-api" "http://127.0.0.1:${AUTH_API_PORT:-8081}/healthz"
  wait_http "friends-api" "http://127.0.0.1:${FRIENDS_API_PORT:-8082}/healthz"
  wait_http "message-api" "http://127.0.0.1:${MESSAGE_API_PORT:-8083}/healthz"
  wait_http "gateway-ws" "http://127.0.0.1:${GATEWAY_WS_PORT:-8084}/healthz"
  wait_http "groups-api" "http://127.0.0.1:${GROUPS_API_PORT:-8085}/healthz"
  wait_http "agent-api" "http://127.0.0.1:${AGENT_API_PORT:-8086}/healthz"

  echo "local backend is ready"
}

main "$@"
