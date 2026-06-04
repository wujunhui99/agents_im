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
                   Service ports can be overridden with USER_API_PORT, USER_RPC_PORT,
                   AUTH_API_PORT, FRIENDS_API_PORT, MESSAGE_API_PORT, GATEWAY_WS_PORT,
                   GROUPS_API_PORT, AGENT_API_PORT, and MESSAGE_TRANSFER_OBSERVABILITY_PORT.
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
    local line key value
    while IFS= read -r line || [[ -n "${line}" ]]; do
      line="${line#"${line%%[![:space:]]*}"}"
      line="${line%"${line##*[![:space:]]}"}"
      [[ -z "${line}" || "${line}" == \#* || "${line}" != *=* ]] && continue
      key="${line%%=*}"
      value="${line#*=}"
      key="${key%"${key##*[![:space:]]}"}"
      value="${value#"${value%%[![:space:]]*}"}"
      if [[ -n "${key}" && -z "${!key+x}" ]]; then
        export "${key}=${value}"
      fi
    done < "${env_file}"
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
  export OBJECT_STORAGE_EXTERNAL_USE_SSL="${OBJECT_STORAGE_EXTERNAL_USE_SSL:-${OBJECT_STORAGE_USE_SSL}}"
  export OBJECT_STORAGE_ACCESS_KEY_ID="${OBJECT_STORAGE_ACCESS_KEY_ID:-${MINIO_ROOT_USER}}"
  export OBJECT_STORAGE_SECRET_ACCESS_KEY="${OBJECT_STORAGE_SECRET_ACCESS_KEY:-${MINIO_ROOT_PASSWORD}}"
  export DATABASE_URL="${DATABASE_URL:-postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable}"
  export JWT_ACCESS_SECRET="${JWT_ACCESS_SECRET:-dev-jwt-secret-change-me}"
  export JWT_ACCESS_EXPIRE="${JWT_ACCESS_EXPIRE:-86400}"
  export PRESENCE_DRIVER="${PRESENCE_DRIVER:-memory}"
  export FRONTEND_PORT="${FRONTEND_PORT:-5173}"
  export GATEWAY_WS_ALLOWED_ORIGINS="${GATEWAY_WS_ALLOWED_ORIGINS:-http://localhost:${FRONTEND_PORT},http://127.0.0.1:${FRONTEND_PORT}}"
  export GATEWAY_WS_ALLOW_QUERY_TOKEN="${GATEWAY_WS_ALLOW_QUERY_TOKEN:-true}"
  export GATEWAY_WS_PING_INTERVAL_SECONDS="${GATEWAY_WS_PING_INTERVAL_SECONDS:-30}"
  export GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS="${GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS:-75}"
  export GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND="${GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND:-20}"
  export GATEWAY_WS_COMMAND_RATE_LIMIT_BURST="${GATEWAY_WS_COMMAND_RATE_LIMIT_BURST:-40}"
  export MESSAGE_TRANSFER_CONSUMER_DRIVER="${MESSAGE_TRANSFER_CONSUMER_DRIVER:-outbox}"
  export MESSAGE_TRANSFER_DISPATCHER_DRIVER="${MESSAGE_TRANSFER_DISPATCHER_DRIVER:-gateway}"
  export MESSAGE_TRANSFER_GATEWAY_ENDPOINT="${MESSAGE_TRANSFER_GATEWAY_ENDPOINT:-http://127.0.0.1:${GATEWAY_WS_PORT:-8084}}"
  export MESSAGE_TRANSFER_OBSERVABILITY_ENABLED="${MESSAGE_TRANSFER_OBSERVABILITY_ENABLED:-true}"
  export MESSAGE_TRANSFER_OBSERVABILITY_HOST="${MESSAGE_TRANSFER_OBSERVABILITY_HOST:-127.0.0.1}"
  export MESSAGE_TRANSFER_OBSERVABILITY_PORT="${MESSAGE_TRANSFER_OBSERVABILITY_PORT:-8087}"
  export MESSAGE_TRANSFER_WORKER_ID="${MESSAGE_TRANSFER_WORKER_ID:-message-transfer-local}"
  export MESSAGE_TRANSFER_POLL_INTERVAL_MILLIS="${MESSAGE_TRANSFER_POLL_INTERVAL_MILLIS:-100}"
  export MESSAGE_TRANSFER_RETRY_BACKOFF_MILLIS="${MESSAGE_TRANSFER_RETRY_BACKOFF_MILLIS:-1000}"
  export MESSAGE_TRANSFER_MAX_ATTEMPTS="${MESSAGE_TRANSFER_MAX_ATTEMPTS:-5}"
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

write_auth_rpc_config() {
  cat > "${CONFIG_DIR}/auth-rpc.yaml" <<YAML
Name: auth-rpc
ListenOn: 127.0.0.1:${AUTH_RPC_PORT:-9091}
TokenAuth:
  AccessSecret: ${JWT_ACCESS_SECRET}
  AccessExpire: ${JWT_ACCESS_EXPIRE}
AdminBootstrap:
  Identifier: ${ADMIN_BOOTSTRAP_IDENTIFIER:-amin}
  Password: ${ADMIN_BOOTSTRAP_PASSWORD:-}
  DisplayName: ${ADMIN_BOOTSTRAP_DISPLAY_NAME:-管理后台管理员}
StorageDriver: postgres
DataSource: ${DATABASE_URL}
MailRPC:
  Endpoints:
    - 127.0.0.1:${MAIL_RPC_PORT:-9095}
  Timeout: 5000
YAML
}

write_auth_api_config() {
  cat > "${CONFIG_DIR}/auth-api.yaml" <<YAML
Name: auth-api
Host: 127.0.0.1
Port: ${AUTH_API_PORT:-8081}
Auth:
  AccessSecret: ${JWT_ACCESS_SECRET}
  AccessExpire: ${JWT_ACCESS_EXPIRE}
AuthRPC:
  Endpoints:
    - 127.0.0.1:${AUTH_RPC_PORT:-9091}
  Timeout: 5000
YAML
}

write_user_rpc_config() {
  cat > "${CONFIG_DIR}/user-rpc.yaml" <<YAML
Name: user-rpc
ListenOn: 127.0.0.1:${USER_RPC_PORT:-9090}
StorageDriver: postgres
DataSource: ${DATABASE_URL}
YAML
}

write_groups_rpc_config() {
  cat > "${CONFIG_DIR}/groups-rpc.yaml" <<YAML
Name: groups-rpc
ListenOn: 127.0.0.1:${GROUPS_RPC_PORT:-9093}
DataSource: ${DATABASE_URL}
YAML
}

write_message_transfer_config() {
  cat > "${CONFIG_DIR}/message-transfer.yaml" <<YAML
Name: message-transfer
WorkerID: ${MESSAGE_TRANSFER_WORKER_ID}
DryRun: false
StorageDriver: postgres
DataSource: ${DATABASE_URL}

Consumer:
  Driver: ${MESSAGE_TRANSFER_CONSUMER_DRIVER}

Dispatcher:
  Driver: ${MESSAGE_TRANSFER_DISPATCHER_DRIVER}
  GatewayEndpoint: ${MESSAGE_TRANSFER_GATEWAY_ENDPOINT}

Worker:
  PollIntervalMillis: ${MESSAGE_TRANSFER_POLL_INTERVAL_MILLIS}
  RetryBackoffMillis: ${MESSAGE_TRANSFER_RETRY_BACKOFF_MILLIS}
  MaxAttempts: ${MESSAGE_TRANSFER_MAX_ATTEMPTS}

Observability:
  Enabled: ${MESSAGE_TRANSFER_OBSERVABILITY_ENABLED}
  Host: ${MESSAGE_TRANSFER_OBSERVABILITY_HOST}
  Port: ${MESSAGE_TRANSFER_OBSERVABILITY_PORT}
YAML
}

write_configs() {
  mkdir -p "${CONFIG_DIR}"
  write_api_config "user-api" "${USER_API_PORT:-8080}" "UserRPC:
  Endpoints:
    - 127.0.0.1:${USER_RPC_PORT:-9090}
  Timeout: 5000
ObjectStorage:
  Driver: ${OBJECT_STORAGE_DRIVER}
  Endpoint: ${OBJECT_STORAGE_ENDPOINT}
  ExternalEndpoint: ${OBJECT_STORAGE_EXTERNAL_ENDPOINT}
  Bucket: ${OBJECT_STORAGE_BUCKET}
  Region: ${OBJECT_STORAGE_REGION}
  UseSSL: ${OBJECT_STORAGE_USE_SSL}
  ExternalUseSSL: ${OBJECT_STORAGE_EXTERNAL_USE_SSL}
  AccessKeyID: ${OBJECT_STORAGE_ACCESS_KEY_ID}
  SecretAccessKey: ${OBJECT_STORAGE_SECRET_ACCESS_KEY}"
  write_auth_api_config
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
  write_api_config "groups-api" "${GROUPS_API_PORT:-8085}" "GroupsRPC:
  Endpoints:
    - 127.0.0.1:${GROUPS_RPC_PORT:-9093}
  Timeout: 5000
UserRPC:
  Endpoints:
    - 127.0.0.1:${USER_RPC_PORT:-9090}
  Timeout: 5000"
  write_api_config "agent-api" "${AGENT_API_PORT:-8086}"
  write_user_rpc_config
  write_groups_rpc_config
  write_auth_rpc_config
  write_message_transfer_config
}

# Map deployment name -> go main package path. Entrypoints live in their service
# directories (cmd/ was removed).
service_pkg() {
  case "$1" in
    agent-api)        echo "./service/agent/api" ;;
    auth-api)         echo "./service/auth/api" ;;
    auth-rpc)         echo "./service/auth/rpc" ;;
    friends-api)      echo "./service/friends/api" ;;
    friends-rpc)      echo "./service/friends/rpc" ;;
    groups-api)       echo "./service/groups/api" ;;
    groups-rpc)       echo "./service/groups/rpc" ;;
    mail-rpc)         echo "./service/mail/rpc" ;;
    user-api)         echo "./service/user/api" ;;
    user-rpc)         echo "./service/user/rpc" ;;
    message-rpc)      echo "./internal/rpcgen/message" ;;
    gateway-ws)       echo "./service/gateway-ws" ;;
    message-api)      echo "./service/message-api" ;;
    message-transfer) echo "./service/message-transfer" ;;
    *) echo "unknown service: $1" >&2; return 1 ;;
  esac
}

build_service() {
  local name="$1"
  local pkg
  pkg="$(service_pkg "${name}")"
  mkdir -p "${BIN_DIR}"
  echo "building ${name}"
  go build -o "${BIN_DIR}/${name}" "${pkg}"
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
    docker compose up -d postgres redis minio
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
  start_service "user-rpc"
  start_service "groups-rpc"
  start_service "auth-rpc"
  start_service "user-api"
  start_service "auth-api"
  start_service "friends-api"
  start_service "message-api"
  start_service "gateway-ws"
  start_service "message-transfer"
  start_service "groups-api"
  start_service "agent-api"

  wait_http "user-api" "http://127.0.0.1:${USER_API_PORT:-8080}/healthz"
  wait_http "auth-api" "http://127.0.0.1:${AUTH_API_PORT:-8081}/healthz"
  wait_http "friends-api" "http://127.0.0.1:${FRIENDS_API_PORT:-8082}/healthz"
  wait_http "message-api" "http://127.0.0.1:${MESSAGE_API_PORT:-8083}/healthz"
  wait_http "gateway-ws" "http://127.0.0.1:${GATEWAY_WS_PORT:-8084}/healthz"
  if [[ "${MESSAGE_TRANSFER_OBSERVABILITY_ENABLED}" == "true" ]]; then
    wait_http "message-transfer" "http://127.0.0.1:${MESSAGE_TRANSFER_OBSERVABILITY_PORT}/healthz"
  fi
  wait_http "groups-api" "http://127.0.0.1:${GROUPS_API_PORT:-8085}/healthz"
  wait_http "agent-api" "http://127.0.0.1:${AGENT_API_PORT:-8086}/healthz"

  echo "local backend is ready"
}

main "$@"
