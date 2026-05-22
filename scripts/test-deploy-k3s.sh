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
    if [[ "${2:-}" == "-f" && "${3:-}" == "-" ]]; then
      stdin="$(cat)"
      printf 'STDIN_START\n%s\nSTDIN_END\n' "${stdin}" >>"${log}"
    fi
    exit 0
    ;;
  kustomize)
    cat <<'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: agents-im
spec:
  template:
    spec:
      containers:
        - image: ghcr.io/wujunhui99/agents_im/web:__IMAGE_TAG_REQUIRED__
          name: web
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: user-rpc
  namespace: agents-im
spec:
  template:
    spec:
      containers:
        - image: ghcr.io/wujunhui99/agents_im/user-rpc:__IMAGE_TAG_REQUIRED__
          name: user-rpc
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agents-im-minio-proxy
  namespace: agents-im
spec:
  template:
    spec:
      containers:
        - name: socat
          image: alpine/socat:1.8.0.3
---
apiVersion: v1
kind: Service
metadata:
  name: web
  namespace: agents-im
YAML
    if [[ "${FAKE_KUSTOMIZE_UNRENDERED_PLACEHOLDER:-}" == "true" ]]; then
      cat <<'YAML'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: bad-placeholder
  namespace: agents-im
data:
  image: ghcr.io/wujunhui99/agents_im/web:__IMAGE_TAG_REQUIRED__
YAML
    fi
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

if grep -Fq "apply -k" "${CALL_LOG}"; then
  echo "deploy must not apply the full kustomization because it contains Deployment image defaults" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "name: agents-im-minio-proxy" "${CALL_LOG}"; then
  echo "expected non-application Deployment manifests to still be applied" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "image: ghcr.io/wujunhui99/agents_im/web:new-web-sha" "${CALL_LOG}"; then
  echo "expected rendered application Deployment to use the immutable selected image tag" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "image: ghcr.io/wujunhui99/agents_im/user-rpc:stable-backend" "${CALL_LOG}"; then
  echo "expected non-selected application Deployment to keep the captured current image" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if grep -Fq "__IMAGE_TAG_REQUIRED__" "${CALL_LOG}"; then
  echo "rendered manifests must not apply placeholder image tags" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if grep -Fq "set image deployment/user-rpc" "${CALL_LOG}"; then
  echo "web-only deploy must not touch non-selected user-rpc image" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if grep -Fq "rollout status deployment/user-rpc --timeout=180s" "${CALL_LOG}"; then
  echo "web-only deploy must not wait on non-selected user-rpc rollout" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

# Config-only deploy must apply non-application resources and restart only selected services.
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
if grep -Fq "apply -k" "${CALL_LOG}"; then
  echo "config-only deploy must not apply the full kustomization because it contains Deployment image defaults" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if grep -Fq "set image deployment/user-rpc" "${CALL_LOG}"; then
  echo "config-only deploy must not touch non-selected user-rpc image" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "rollout restart deployment/groups-rpc" "${CALL_LOG}"; then
  echo "expected config-only deploy to restart the selected groups-rpc deployment" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "rollout status deployment/groups-rpc --timeout=180s" "${CALL_LOG}"; then
  echo "expected config-only deploy to wait for the selected groups-rpc rollout" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if grep -Fq "rollout status deployment/user-rpc --timeout=180s" "${CALL_LOG}"; then
  echo "config-only deploy must not wait for non-selected user-rpc rollout" >&2
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

if grep -Fq "set image deployment/user-rpc" "${CALL_LOG}"; then
  echo "explicit DATABASE_URL deploy must not touch non-selected user-rpc image" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if grep -Fq "user-rpc=new-message-sha" "${CALL_LOG}"; then
  echo "explicit DATABASE_URL deploy must not set user-rpc to the new release tag" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

# Deploys that update images must provide an immutable image tag.
: >"${CALL_LOG}"
if FAKE_KUBECTL_LOG="${CALL_LOG}" \
  NAMESPACE=agents-im \
  KUBECTL="${FAKE_KUBECTL}" \
  SKIP_MIDDLEWARE=true \
  SKIP_MIGRATIONS=true \
  IMAGE_REGISTRY=ghcr.io/wujunhui99/agents_im \
  IMAGE_SERVICES=web \
  ROLLOUT_SERVICES=web \
  "${ROOT_DIR}/scripts/deploy-k3s.sh" >/tmp/deploy-k3s-test-missing-tag.out 2>/tmp/deploy-k3s-test-missing-tag.err; then
  echo "expected deploy without IMAGE_TAG to fail" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "IMAGE_TAG must be a non-empty immutable tag" /tmp/deploy-k3s-test-missing-tag.err; then
  echo "expected missing IMAGE_TAG error" >&2
  cat /tmp/deploy-k3s-test-missing-tag.err >&2
  exit 1
fi

# Mutable latest must be rejected even when the caller supplies it explicitly.
: >"${CALL_LOG}"
if FAKE_KUBECTL_LOG="${CALL_LOG}" \
  NAMESPACE=agents-im \
  KUBECTL="${FAKE_KUBECTL}" \
  SKIP_MIDDLEWARE=true \
  SKIP_MIGRATIONS=true \
  IMAGE_REGISTRY=ghcr.io/wujunhui99/agents_im \
  IMAGE_TAG=latest \
  IMAGE_SERVICES=web \
  ROLLOUT_SERVICES=web \
  "${ROOT_DIR}/scripts/deploy-k3s.sh" >/tmp/deploy-k3s-test-latest-tag.out 2>/tmp/deploy-k3s-test-latest-tag.err; then
  echo "expected deploy with IMAGE_TAG=latest to fail" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if grep -Fq "set image deployment/web" "${CALL_LOG}"; then
  echo "deploy with IMAGE_TAG=latest must not set image" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "mutable latest" /tmp/deploy-k3s-test-latest-tag.err; then
  echo "expected explicit latest IMAGE_TAG error" >&2
  cat /tmp/deploy-k3s-test-latest-tag.err >&2
  exit 1
fi

# Rendering must fail closed if any placeholder survives after image overrides.
: >"${CALL_LOG}"
if FAKE_KUBECTL_LOG="${CALL_LOG}" \
  FAKE_KUSTOMIZE_UNRENDERED_PLACEHOLDER=true \
  NAMESPACE=agents-im \
  KUBECTL="${FAKE_KUBECTL}" \
  SKIP_MIDDLEWARE=true \
  SKIP_MIGRATIONS=true \
  SKIP_SET_IMAGE=true \
  ROLLOUT_SERVICES=groups-rpc \
  RESTART_SERVICES=groups-rpc \
  "${ROOT_DIR}/scripts/deploy-k3s.sh" >/tmp/deploy-k3s-test-placeholder-guard.out 2>/tmp/deploy-k3s-test-placeholder-guard.err; then
  echo "expected deploy with an unrendered placeholder to fail" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "refusing to apply placeholder images" /tmp/deploy-k3s-test-placeholder-guard.err; then
  echo "expected placeholder guard error" >&2
  cat /tmp/deploy-k3s-test-placeholder-guard.err >&2
  exit 1
fi

if grep -Fq "bad-placeholder" "${CALL_LOG}"; then
  echo "placeholder guard must not apply manifests containing placeholders" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi
