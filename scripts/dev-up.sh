#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# Shared service registry (package paths) — single source of truth.
source "${ROOT_DIR}/scripts/services.sh"
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
	                   AUTH_API_PORT, FRIENDS_API_PORT, MSG_API_PORT, GATEWAY_WS_PORT,
	                   GROUPS_API_PORT, AGENT_API_PORT, MEDIA_API_PORT, MEDIA_RPC_PORT,
	                   ADMIN_API_PORT, ADMIN_RPC_PORT, and MESSAGE_TRANSFER_OBSERVABILITY_PORT.
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
  export REDIS_ADDR="${REDIS_ADDR:-localhost:6379}"
  export REDIS_DB="${REDIS_DB:-0}"
  export OSS_ROOT_USER="${OSS_ROOT_USER:-agents_im}"
  export OSS_ROOT_PASSWORD="${OSS_ROOT_PASSWORD:-agents_im_dev_password}"
  export OSS_API_PORT="${OSS_API_PORT:-9000}"
  export OSS_CONSOLE_PORT="${OSS_CONSOLE_PORT:-9001}"
  export OBJECT_STORAGE_DRIVER="${OBJECT_STORAGE_DRIVER:-rustfs}"
  export OBJECT_STORAGE_ENDPOINT="${OBJECT_STORAGE_ENDPOINT:-localhost:${OSS_API_PORT}}"
  export OBJECT_STORAGE_EXTERNAL_ENDPOINT="${OBJECT_STORAGE_EXTERNAL_ENDPOINT:-localhost:${OSS_API_PORT}}"
  export OBJECT_STORAGE_BUCKET="${OBJECT_STORAGE_BUCKET:-agents-im-media}"
  export OBJECT_STORAGE_REGION="${OBJECT_STORAGE_REGION:-us-east-1}"
  export OBJECT_STORAGE_USE_SSL="${OBJECT_STORAGE_USE_SSL:-false}"
  export OBJECT_STORAGE_EXTERNAL_USE_SSL="${OBJECT_STORAGE_EXTERNAL_USE_SSL:-${OBJECT_STORAGE_USE_SSL}}"
  export OBJECT_STORAGE_ACCESS_KEY_ID="${OBJECT_STORAGE_ACCESS_KEY_ID:-${OSS_ROOT_USER}}"
  export OBJECT_STORAGE_SECRET_ACCESS_KEY="${OBJECT_STORAGE_SECRET_ACCESS_KEY:-${OSS_ROOT_PASSWORD}}"
  export DATABASE_URL="${DATABASE_URL:-postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable}"
  export JWT_ACCESS_SECRET="${JWT_ACCESS_SECRET:-dev-jwt-secret-change-me}"
  export JWT_ACCESS_EXPIRE="${JWT_ACCESS_EXPIRE:-86400}"
  # 本地追踪上报到 docker-compose 的 tempo，贴近生产（生产由 ConfigMap 注入同名 env）。
  # pkg/observability 系服务读 AGENTS_IM_* env；groups 走 go-zero 原生 Telemetry（见其 yaml）。
  export TEMPO_OTLP_GRPC_PORT="${TEMPO_OTLP_GRPC_PORT:-4317}"
  export AGENTS_IM_TRACING_ENABLED="${AGENTS_IM_TRACING_ENABLED:-true}"
  export AGENTS_IM_OTLP_ENDPOINT="${AGENTS_IM_OTLP_ENDPOINT:-127.0.0.1:${TEMPO_OTLP_GRPC_PORT}}"
  export AGENTS_IM_OTLP_PROTOCOL="${AGENTS_IM_OTLP_PROTOCOL:-grpc}"
  export AGENTS_IM_TRACING_SAMPLER_RATIO="${AGENTS_IM_TRACING_SAMPLER_RATIO:-1.0}"
  export PRESENCE_DRIVER="${PRESENCE_DRIVER:-memory}"
  export FRONTEND_PORT="${FRONTEND_PORT:-5173}"
  export GATEWAY_WS_ALLOWED_ORIGINS="${GATEWAY_WS_ALLOWED_ORIGINS:-http://localhost:${FRONTEND_PORT},http://127.0.0.1:${FRONTEND_PORT}}"
  export GATEWAY_WS_ALLOW_QUERY_TOKEN="${GATEWAY_WS_ALLOW_QUERY_TOKEN:-true}"
  export GATEWAY_WS_PING_INTERVAL_SECONDS="${GATEWAY_WS_PING_INTERVAL_SECONDS:-30}"
  export GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS="${GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS:-75}"
  export GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND="${GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND:-20}"
  export GATEWAY_WS_COMMAND_RATE_LIMIT_BURST="${GATEWAY_WS_COMMAND_RATE_LIMIT_BURST:-40}"
  # Kafka 是唯一写链路（03 §9 B3b）：msg-rpc / msgtransfer 启动必需。
  export REDPANDA_KAFKA_PORT="${REDPANDA_KAFKA_PORT:-19092}"
  export KAFKA_BROKERS="${KAFKA_BROKERS:-localhost:${REDPANDA_KAFKA_PORT}}"
  export MESSAGE_TRANSFER_OBSERVABILITY_ENABLED="${MESSAGE_TRANSFER_OBSERVABILITY_ENABLED:-true}"
  export MESSAGE_TRANSFER_OBSERVABILITY_HOST="${MESSAGE_TRANSFER_OBSERVABILITY_HOST:-127.0.0.1}"
  export MESSAGE_TRANSFER_OBSERVABILITY_PORT="${MESSAGE_TRANSFER_OBSERVABILITY_PORT:-8087}"
  export MESSAGE_TRANSFER_WORKER_ID="${MESSAGE_TRANSFER_WORKER_ID:-msgtransfer-local}"
  export MESSAGE_TRANSFER_POLL_INTERVAL_MILLIS="${MESSAGE_TRANSFER_POLL_INTERVAL_MILLIS:-100}"
  export MESSAGE_TRANSFER_RETRY_BACKOFF_MILLIS="${MESSAGE_TRANSFER_RETRY_BACKOFF_MILLIS:-1000}"
  export MESSAGE_TRANSFER_MAX_ATTEMPTS="${MESSAGE_TRANSFER_MAX_ATTEMPTS:-5}"
  # push（03 §6 / §9 C2-C3）：下行投递调度器。gateway gRPC 推送面 + push 自身。
  export GATEWAY_GRPC_PORT="${GATEWAY_GRPC_PORT:-9101}"
  export PUSH_GATEWAY_TARGET="${PUSH_GATEWAY_TARGET:-127.0.0.1:${GATEWAY_GRPC_PORT}}"
  export PUSH_OBSERVABILITY_PORT="${PUSH_OBSERVABILITY_PORT:-8091}"

  # Service host ports. Exported so the config templates in scripts/dev/etc/ can be
  # rendered with plain ${VAR} expansion (no per-template :- defaults).
  export USER_API_PORT="${USER_API_PORT:-8080}"
  export AUTH_API_PORT="${AUTH_API_PORT:-8081}"
  export FRIENDS_API_PORT="${FRIENDS_API_PORT:-8082}"
  export GATEWAY_WS_PORT="${GATEWAY_WS_PORT:-8084}"
  export GROUPS_API_PORT="${GROUPS_API_PORT:-8085}"
  export AGENT_API_PORT="${AGENT_API_PORT:-8086}"
  export ADMIN_API_PORT="${ADMIN_API_PORT:-8088}"
  export MEDIA_API_PORT="${MEDIA_API_PORT:-8089}"
  export MSG_API_PORT="${MSG_API_PORT:-8090}"
  export USER_RPC_PORT="${USER_RPC_PORT:-9090}"
  export AUTH_RPC_PORT="${AUTH_RPC_PORT:-9091}"
  export FRIENDS_RPC_PORT="${FRIENDS_RPC_PORT:-9092}"
  export GROUPS_RPC_PORT="${GROUPS_RPC_PORT:-9093}"
  export MAIL_RPC_PORT="${MAIL_RPC_PORT:-9095}"
  export MEDIA_RPC_PORT="${MEDIA_RPC_PORT:-9096}"
  export ADMIN_RPC_PORT="${ADMIN_RPC_PORT:-9097}"
  export MSG_RPC_PORT="${MSG_RPC_PORT:-9098}"
  export AGENT_RPC_PORT="${AGENT_RPC_PORT:-9099}"
  export PRESENCE_TTL_SECONDS="${PRESENCE_TTL_SECONDS:-60}"
  export PRESENCE_KEY_PREFIX="${PRESENCE_KEY_PREFIX:-agents_im:presence}"
  export ADMIN_BOOTSTRAP_IDENTIFIER="${ADMIN_BOOTSTRAP_IDENTIFIER:-amin}"
  export ADMIN_BOOTSTRAP_PASSWORD="${ADMIN_BOOTSTRAP_PASSWORD:-}"
  export ADMIN_BOOTSTRAP_DISPLAY_NAME="${ADMIN_BOOTSTRAP_DISPLAY_NAME:-管理后台管理员}"
  export DEEPSEEK_API_KEY="${DEEPSEEK_API_KEY:-}"
  export DEEPSEEK_BASE_URL="${DEEPSEEK_BASE_URL:-}"
  export DEEPSEEK_MODEL="${DEEPSEEK_MODEL:-}"
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

wait_for_oss() {
  local attempt
  for attempt in $(seq 1 60); do
    # RustFS (S3) returns an HTTP reply (e.g. 403 for anonymous) once up; any
    # response means the endpoint is serving, so don't use --fail.
    if curl --silent --output /dev/null "http://${OBJECT_STORAGE_ENDPOINT}/" 2>/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "object storage (rustfs) did not become ready at ${OBJECT_STORAGE_ENDPOINT}" >&2
  exit 1
}

render_configs() {
  # Render every dev config from its scripts/dev/etc/<name>.yaml.tmpl template,
  # expanding ${VAR} placeholders from the environment exported by load_env.
  # Config lives in the templates (data); this script only orchestrates.
  mkdir -p "${CONFIG_DIR}"
  local tmpl name
  for tmpl in "${ROOT_DIR}"/scripts/dev/etc/*.yaml.tmpl; do
    name="$(basename "${tmpl}" .yaml.tmpl)"
    python3 -c 'import os,sys; sys.stdout.write(os.path.expandvars(open(sys.argv[1]).read()))' \
      "${tmpl}" > "${CONFIG_DIR}/${name}.yaml"
  done
}

build_service() {
  local name="$1"
  local pkg
  # Go main package path comes from the shared registry (scripts/services.json).
  if ! pkg="$(services_package "${name}")"; then
    echo "unknown service: ${name}" >&2
    return 1
  fi
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
    docker compose up -d postgres redis rustfs redpanda tempo
    wait_for_postgres
    require_command curl
    wait_for_oss
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
  require_command python3

  stop_services
  render_configs
  # media-rpc 先起：user-rpc（头像校验）/msg-rpc（附件校验）的 MediaRPC 客户端依赖它（#533）。
  start_service "media-rpc"
  start_service "groups-rpc"
  start_service "msg-rpc"
  start_service "friends-rpc"
  start_service "admin-rpc"
  start_service "auth-rpc"
  # agent-rpc / friends-rpc 必须在 user-rpc 之前就绪：user-rpc 启动即 Backfill 默认助手，
  # 同步调 agent-rpc.EnsureDefaultAssistant + friends-rpc.EnsureFriendship（#606，单向叶子不成环）。
  # agent-rpc 对 msg-rpc/user-rpc 的客户端是 NonBlock，仅运行时调用，故可先于 user-rpc 起。
  start_service "agent-rpc"
  start_service "user-rpc"
  start_service "user-api"
  start_service "auth-api"
  start_service "friends-api"
  start_service "msg-api"
  start_service "msggateway"
  start_service "msgtransfer"
  start_service "push"
  start_service "groups-api"
  start_service "agent-api"
  start_service "media-api"
  start_service "admin-api"

  wait_http "user-api" "http://127.0.0.1:${USER_API_PORT:-8080}/healthz"
  wait_http "auth-api" "http://127.0.0.1:${AUTH_API_PORT:-8081}/healthz"
  wait_http "friends-api" "http://127.0.0.1:${FRIENDS_API_PORT:-8082}/healthz"
  wait_http "msg-api" "http://127.0.0.1:${MSG_API_PORT:-8090}/healthz"
  wait_http "msggateway" "http://127.0.0.1:${GATEWAY_WS_PORT:-8084}/healthz"
  if [[ "${MESSAGE_TRANSFER_OBSERVABILITY_ENABLED}" == "true" ]]; then
    wait_http "msgtransfer" "http://127.0.0.1:${MESSAGE_TRANSFER_OBSERVABILITY_PORT}/healthz"
  fi
  wait_http "push" "http://127.0.0.1:${PUSH_OBSERVABILITY_PORT}/healthz"
  wait_http "groups-api" "http://127.0.0.1:${GROUPS_API_PORT:-8085}/healthz"
  wait_http "agent-api" "http://127.0.0.1:${AGENT_API_PORT:-8086}/healthz"
  wait_http "media-api" "http://127.0.0.1:${MEDIA_API_PORT:-8089}/healthz"
  wait_http "admin-api" "http://127.0.0.1:${ADMIN_API_PORT:-8088}/healthz"

  echo "local backend is ready"
}

main "$@"
