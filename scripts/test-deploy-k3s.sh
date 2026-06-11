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
        msg-api) echo 'ghcr.io/wujunhui99/agents_im/msg-api:new-message-sha' ;;
        web) echo 'ghcr.io/wujunhui99/agents_im/web:new-web-sha' ;;
        user-rpc) echo 'ghcr.io/wujunhui99/agents_im/user-rpc:stable-backend' ;;
        auth-rpc) echo 'ghcr.io/wujunhui99/agents_im/auth-rpc:stable-backend' ;;
        friends-rpc) echo 'ghcr.io/wujunhui99/agents_im/friends-rpc:stable-backend' ;;
        groups-rpc) echo 'ghcr.io/wujunhui99/agents_im/groups-rpc:stable-backend' ;;
        msg-rpc) echo 'ghcr.io/wujunhui99/agents_im/msg-rpc:stable-backend' ;;
        third-rpc) echo 'ghcr.io/wujunhui99/agents_im/third-rpc:stable-backend' ;;
        web) echo 'ghcr.io/wujunhui99/agents_im/web:old-web' ;;
        *) echo "ghcr.io/wujunhui99/agents_im/${service}:stable-backend" ;;
      esac
      exit 0
    fi
    if [[ "${2:-}" == "pods" && " ${*} " == *" -o json "* ]]; then
      selector=""
      for ((i=1; i <= $#; i++)); do
        arg="${!i}"
        if [[ "${arg}" == "-l" ]]; then
          next=$((i + 1))
          selector="${!next}"
        fi
      done
      service="${selector#app=}"
      image="ghcr.io/wujunhui99/agents_im/${service}:stable-backend"
      case "${service}" in
        web) image='ghcr.io/wujunhui99/agents_im/web:new-web-sha' ;;
        msg-api) image='ghcr.io/wujunhui99/agents_im/msg-api:new-message-sha' ;;
      esac
      cat <<JSON
{"items":[{"metadata":{"name":"${service}-pod"},"status":{"phase":"Running","containerStatuses":[{"name":"${service}","ready":true,"imageID":"ghcr.io/wujunhui99/agents_im/${service}@sha256:testdigest"}]},"spec":{"containers":[{"name":"${service}","image":"${image}"}]}}]}
JSON
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
  name: msg-api
  namespace: agents-im
spec:
  template:
    spec:
      containers:
        - image: ghcr.io/wujunhui99/agents_im/msg-api:__IMAGE_TAG_REQUIRED__
          name: msg-api
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: media-rpc
  namespace: agents-im
spec:
  template:
    spec:
      containers:
        - image: ghcr.io/wujunhui99/agents_im/media-rpc:__IMAGE_TAG_REQUIRED__
          name: media-rpc
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

DETECT_OUTPUT="$(python3 "${ROOT_DIR}/scripts/detect-deploy-changes.py" \
  --event-name push \
  --ref refs/heads/main \
  deploy/k8s/prometheus-grafana.yaml)"
if ! grep -Fq "config_only=true" <<<"${DETECT_OUTPUT}" || \
  ! grep -Fq "rollout_services='prometheus grafana'" <<<"${DETECT_OUTPUT}" || \
  ! grep -Fq "restart_services='prometheus grafana'" <<<"${DETECT_OUTPUT}"; then
  echo "expected prometheus-grafana manifest change to restart prometheus and grafana" >&2
  printf '%s\n' "${DETECT_OUTPUT}" >&2
  exit 1
fi

DETECT_OUTPUT="$(python3 "${ROOT_DIR}/scripts/detect-deploy-changes.py" \
  --event-name push \
  --ref refs/heads/main \
  deploy/k8s/tempo.yaml)"
if ! grep -Fq "rollout_services='grafana tempo'" <<<"${DETECT_OUTPUT}" || \
  ! grep -Fq "restart_services='grafana tempo'" <<<"${DETECT_OUTPUT}"; then
  echo "expected tempo manifest change to restart tempo and grafana" >&2
  printf '%s\n' "${DETECT_OUTPUT}" >&2
  exit 1
fi


DETECT_OUTPUT="$(python3 "${ROOT_DIR}/scripts/detect-deploy-changes.py" \
  --event-name push \
  --ref refs/heads/main \
  service/msg/api/internal/logic/msg/send_message_logic.go \
  web/src/api/feedback.ts)"
if ! grep -Fq "backend_services='[\"msg-api\"]'" <<<"${DETECT_OUTPUT}" || \
  grep -Fq 'gateway-ws' <<<"${DETECT_OUTPUT}" || \
  grep -Fq 'user-rpc' <<<"${DETECT_OUTPUT}" || \
  ! grep -Fq "image_services_space='msg-api web'" <<<"${DETECT_OUTPUT}"; then
  echo "expected API route/web changes to rebuild API backends plus web, not every backend/RPC/worker" >&2
  printf '%s\n' "${DETECT_OUTPUT}" >&2
  exit 1
fi

DETECT_OUTPUT="$(python3 "${ROOT_DIR}/scripts/detect-deploy-changes.py" \
  --event-name push \
  --ref refs/heads/main \
  internal/servicecontext/message/service_context.go)"
if ! grep -Fq "backend_services='[\"gateway-ws\",\"msg-rpc\"]'" <<<"${DETECT_OUTPUT}" || \
  grep -Fq 'user-api' <<<"${DETECT_OUTPUT}" || \
  ! grep -Fq "migration_required=false" <<<"${DETECT_OUTPUT}"; then
  echo "expected AI hosting runtime changes to rebuild gateway-ws and msg-rpc only without migrations" >&2
  printf '%s\n' "${DETECT_OUTPUT}" >&2
  exit 1
fi

DETECT_OUTPUT="$(python3 "${ROOT_DIR}/scripts/detect-deploy-changes.py" \
  --event-name push \
  --ref refs/heads/main \
  db/migrations/202605260001_example.sql)"
if ! grep -Fq "migration_required=true" <<<"${DETECT_OUTPUT}" || \
  ! grep -Fq "backend_services='[\"user-api\",\"auth-api\",\"friends-api\",\"msg-api\",\"gateway-ws\",\"groups-api\",\"agent-api\",\"admin-api\",\"msgtransfer\",\"user-rpc\",\"auth-rpc\",\"friends-rpc\",\"groups-rpc\",\"msg-rpc\",\"third-rpc\",\"media-api\",\"media-rpc\",\"admin-rpc\"]'" <<<"${DETECT_OUTPUT}"; then
  echo "expected executable migration changes to require migrations and rebuild all backends" >&2
  printf '%s\n' "${DETECT_OUTPUT}" >&2
  exit 1
fi

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
SKIP_MIGRATIONS=true \
IMAGE_REGISTRY=ghcr.io/wujunhui99/agents_im \
IMAGE_TAG=new-web-sha \
IMAGE_SERVICES=web \
ROLLOUT_SERVICES=web \
"${ROOT_DIR}/scripts/deploy-k3s.sh" >/tmp/deploy-k3s-test.out

if grep -Fq "apply -k" "${CALL_LOG}"; then
  echo "deploy must not apply the full kustomization because it contains Deployment image defaults" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "delete ingress agents-im-prometheus --ignore-not-found=true" "${CALL_LOG}"; then
  echo "deploy must delete the retired prometheus.agenticim.xyz ingress" >&2
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

if ! grep -Fq "get pods -l app=web -o json" "${CALL_LOG}"; then
  echo "expected web image deploy to verify running pod image" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "verified image service=web pod=web-pod ready=True image=ghcr.io/wujunhui99/agents_im/web:new-web-sha" /tmp/deploy-k3s-test.out; then
  echo "expected web deploy to print verified running image evidence" >&2
  cat /tmp/deploy-k3s-test.out >&2
  exit 1
fi

if grep -Fq "set image deployment/" "${CALL_LOG}"; then
  echo "render-first deploy must not use post-apply kubectl set image" >&2
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

if ! grep -Fq "delete ingress agents-im-prometheus --ignore-not-found=true" "${CALL_LOG}"; then
  echo "config-only deploy must delete the retired prometheus.agenticim.xyz ingress" >&2
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

if grep -Fq "get pods -l app=groups-rpc -o json" "${CALL_LOG}"; then
  echo "config-only deploy must not verify image tag for restart-only services" >&2
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

# Config-only deploys must be able to restart observability deployments that do
# not use application images. Grafana datasource ConfigMap changes depend on this
# restart so newly provisioned datasources such as Tempo are loaded.
: >"${CALL_LOG}"
FAKE_KUBECTL_LOG="${CALL_LOG}" \
NAMESPACE=agents-im \
KUBECTL="${FAKE_KUBECTL}" \
SKIP_MIGRATIONS=true \
SKIP_SET_IMAGE=true \
ROLLOUT_SERVICES=grafana \
RESTART_SERVICES=grafana \
"${ROOT_DIR}/scripts/deploy-k3s.sh" >/tmp/deploy-k3s-test-grafana-restart.out

if ! grep -Fq "rollout restart deployment/grafana" "${CALL_LOG}"; then
  echo "expected config-only deploy to restart grafana" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "rollout status deployment/grafana --timeout=180s" "${CALL_LOG}"; then
  echo "expected config-only deploy to wait for grafana rollout" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if grep -Fq "grafana=ghcr.io/wujunhui99/agents_im/grafana" "${CALL_LOG}"; then
  echo "observability deploys must not receive application image overrides" >&2
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
DATABASE_URL=postgres://override-dsn \
IMAGE_REGISTRY=ghcr.io/wujunhui99/agents_im \
IMAGE_TAG=new-message-sha \
IMAGE_SERVICES=msg-api \
ROLLOUT_SERVICES=msg-api \
"${ROOT_DIR}/scripts/deploy-k3s.sh" >/tmp/deploy-k3s-test-explicit-dsn.out

if grep -Fq "jsonpath={.data.DATABASE_URL}" "${CALL_LOG}"; then
  echo "expected explicit DATABASE_URL deploy to avoid reading DATABASE_URL from k8s secret" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "image: ghcr.io/wujunhui99/agents_im/msg-api:new-message-sha" "${CALL_LOG}"; then
  echo "expected msg-api image to be rendered to the new release tag when explicit DATABASE_URL is provided" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if grep -Fq "set image deployment/" "${CALL_LOG}"; then
  echo "explicit DATABASE_URL deploy must not use post-apply kubectl set image" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "get pods -l app=msg-api -o json" "${CALL_LOG}"; then
  echo "expected msg-api deploy to verify running pod image" >&2
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

if grep -Fq "set image deployment/" "${CALL_LOG}"; then
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
  SKIP_MIGRATIONS=true \
  SKIP_SET_IMAGE=true \
  ROLLOUT_SERVICES=groups-rpc \
  RESTART_SERVICES=groups-rpc \
  "${ROOT_DIR}/scripts/deploy-k3s.sh" >/tmp/deploy-k3s-test-placeholder-guard.out 2>/tmp/deploy-k3s-test-placeholder-guard.err; then
  echo "expected deploy with an unrendered placeholder to fail" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi

if ! grep -Fq "refusing to apply unsafe images" /tmp/deploy-k3s-test-placeholder-guard.err; then
  echo "expected placeholder guard error" >&2
  cat /tmp/deploy-k3s-test-placeholder-guard.err >&2
  exit 1
fi

if grep -Fq "bad-placeholder" "${CALL_LOG}"; then
  echo "placeholder guard must not apply manifests containing placeholders" >&2
  cat "${CALL_LOG}" >&2
  exit 1
fi
