#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

CALL_LOG="${TMP_DIR}/kubectl-calls.log"
FAKE_KUBECTL="${TMP_DIR}/kubectl"

cat >"${FAKE_KUBECTL}" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
log="${FAKE_KUBECTL_LOG:?}"
printf '%q ' "$@" >>"${log}"
printf '\n' >>"${log}"

ns_args=()
if [[ "${1:-}" == "-n" ]]; then
  ns_args=("$1" "$2")
  shift 2
fi

case "${1:-}" in
  get)
    if [[ "${2:-}" == "secret" ]]; then
      if [[ "${*: -1}" == *DATABASE_URL* || " ${*} " == *" jsonpath={.data.DATABASE_URL} "* ]]; then
        printf 'cG9zdGdyZXM6Ly9mcm9tLXNlY3JldA=='
      fi
      exit 0
    fi
    if [[ "${2:-}" == "deployment" || "${2:-}" == "deploy" ]]; then
      service="${3:-}"
      case "${service}" in
        user-rpc) echo 'ghcr.io/wujunhui99/agents_im/user-rpc:stable-backend' ;;
        auth-rpc) echo 'ghcr.io/wujunhui99/agents_im/auth-rpc:stable-backend' ;;
        friends-rpc) echo 'ghcr.io/wujunhui99/agents_im/friends-rpc:stable-backend' ;;
        groups-rpc) echo 'ghcr.io/wujunhui99/agents_im/groups-rpc:stable-backend' ;;
        message-rpc) echo 'ghcr.io/wujunhui99/agents_im/message-rpc:stable-backend' ;;
        mail-rpc) echo 'ghcr.io/wujunhui99/agents_im/mail-rpc:stable-backend' ;;
        web) echo 'ghcr.io/wujunhui99/agents_im/web:old-web' ;;
        *) echo "ghcr.io/wujunhui99/agents_im/${service}:stable-backend" ;;
      esac
      exit 0
    fi
    exit 0
    ;;
  apply)
    exit 0
    ;;
  set)
    if [[ "${2:-}" == "image" ]]; then
      exit 0
    fi
    ;;
  rollout)
    exit 0
    ;;
  create)
    exit 0
    ;;
esac
exit 0
FAKE
chmod +x "${FAKE_KUBECTL}"

cat >"${TMP_DIR}/psql" <<'FAKEPSQL'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "postgres://override-dsn" ]]; then
  shift
fi
case " ${*} " in
  *"select checksum from schema_migrations"*) exit 0 ;;
  *) exit 0 ;;
esac
FAKEPSQL
chmod +x "${TMP_DIR}/psql"
export PATH="${TMP_DIR}:${PATH}"

FAKE_KUBECTL_LOG="${CALL_LOG}" \
NAMESPACE=agents-im \
KUBECTL="${FAKE_KUBECTL}" \
SKIP_MIDDLEWARE=true \
SKIP_MIGRATIONS=true \
IMAGE_REGISTRY=ghcr.io/wujunhui99/agents_im \
IMAGE_TAG=new-web-sha \
IMAGE_SERVICES=web \
ROLLOUT_SERVICES=web \
"${ROOT_DIR}/scripts/deploy-k3s.sh" >/tmp/deploy-k3s-test.out

if ! grep -Fq "set image deployment/web web=ghcr.io/wujunhui99/agents_im/web:new-web-sha" "${CALL_LOG}"; then
  echo "expected web image to be updated to the new release tag" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "set image deployment/user-rpc user-rpc=ghcr.io/wujunhui99/agents_im/user-rpc:stable-backend" "${CALL_LOG}"; then
  echo "expected non-selected user-rpc image to be restored after kubectl apply" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "rollout status deployment/user-rpc --timeout=180s" "${CALL_LOG}"; then
  echo "expected restored non-selected user-rpc rollout to be waited on" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

# Config-only deploy must also preserve all existing service images after apply.
: >"${CALL_LOG}"
FAKE_KUBECTL_LOG="${CALL_LOG}" \
NAMESPACE=agents-im \
KUBECTL="${FAKE_KUBECTL}" \
SKIP_MIDDLEWARE=true \
SKIP_MIGRATIONS=true \
SKIP_SET_IMAGE=true \
ROLLOUT_SERVICES=groups-rpc \
RESTART_SERVICES=groups-rpc \
"${ROOT_DIR}/scripts/deploy-k3s.sh" >/tmp/deploy-k3s-test-config-only.out

# Config-only deploy checks must run before the next scenario resets CALL_LOG.
if ! grep -Fq "set image deployment/user-rpc user-rpc=ghcr.io/wujunhui99/agents_im/user-rpc:stable-backend" "${CALL_LOG}"; then
  echo "expected config-only deploy to restore non-selected user-rpc image after kubectl apply" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if grep -Fq "user-rpc=new-web-sha" "${CALL_LOG}"; then
  echo "config-only deploy must not set user-rpc to the new release tag" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

# Non-config-only deploys should use an explicit DATABASE_URL when supplied by
# the caller. This covers local Drone deploys where 127.0.0.1 inside the deploy
# container is not the middleware host, while the k3s secret still contains the
# application in-cluster DSN.
: >"${CALL_LOG}"
FAKE_KUBECTL_LOG="${CALL_LOG}" \
NAMESPACE=agents-im \
KUBECTL="${FAKE_KUBECTL}" \
SKIP_MIDDLEWARE=true \
DATABASE_URL=postgres://override-dsn \
IMAGE_REGISTRY=ghcr.io/wujunhui99/agents_im \
IMAGE_TAG=new-message-sha \
IMAGE_SERVICES=message-api \
ROLLOUT_SERVICES=message-api \
"${ROOT_DIR}/scripts/deploy-k3s.sh" >/tmp/deploy-k3s-test-explicit-dsn.out

if grep -Fq "jsonpath={.data.DATABASE_URL}" "${CALL_LOG}"; then
  echo "expected explicit DATABASE_URL deploy to avoid reading DATABASE_URL from k8s secret" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "set image deployment/message-api message-api=ghcr.io/wujunhui99/agents_im/message-api:new-message-sha" "${CALL_LOG}"; then
  echo "expected message-api image to be updated when explicit DATABASE_URL is provided" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "set image deployment/user-rpc user-rpc=ghcr.io/wujunhui99/agents_im/user-rpc:stable-backend" "${CALL_LOG}"; then
  echo "expected explicit DATABASE_URL deploy to preserve non-selected images" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if grep -Fq "user-rpc=new-message-sha" "${CALL_LOG}"; then
  echo "explicit DATABASE_URL deploy must not set user-rpc to the new release tag" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi
