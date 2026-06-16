#!/usr/bin/env bash
# Deploy/CI/middleware gates: Drone pipeline, migration immutability, dev scripts,
# docker-compose + env, production k8s config, and the observability manifests.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"
cd "$(git rev-parse --show-toplevel)"

# --- Drone pipeline + migration immutability ---
assert_present "-qF" .drone.yml -- \
  "kind: pipeline" "backend-verification" "postgres-integration" "deploy-main" \
  "bash scripts/ci/drone-backend-verify.sh" "bash scripts/ci/drone-postgres-integration.sh" \
  "bash scripts/ci/drone-detect-deploy.sh" "bash scripts/ci/drone-build-images.sh" "bash scripts/ci/drone-deploy.sh" \
  "from_secret: ghcr_token" "postgres:16-alpine" "ghcr.io/wujunhui99/agents_im"

bash scripts/ci/verify-migration-immutability.sh

# --- dev / demo tooling content ---
assert_present "-qF" scripts/dev-up.sh -- \
  "docker compose up -d postgres redis minio" "bash scripts/migrate-postgres.sh" \
  "StorageDriver: postgres" "ObjectStorage:" "msggateway"
assert_present "-qF" scripts/dev-demo-data.sh -- \
  "/auth/register" "/friends" "/groups" "/messages" "/read"

# --- middleware docker-compose + env ---
assert_present "-q" docker-compose.yml -- \
  "^  redis:" "redis:7-alpine" "agents-im-redis" "agents_im_redis_data" "REDIS_PASSWORD"
assert_present "-q" docker-compose.yml deploy/middleware/docker-compose.yml -- \
  "^  minio:" "minio/minio" "agents-im-minio" "MINIO_ROOT_USER" "MINIO_ROOT_PASSWORD" \
  "MINIO_API_PORT" "MINIO_CONSOLE_PORT" "agents_im_minio_data"
assert_present "-q" .env.example -- \
  "REDIS_ADDR" "REDIS_PASSWORD" "REDIS_DB" "PRESENCE_DRIVER" "PRESENCE_TTL_SECONDS" "PRESENCE_KEY_PREFIX"
assert_present "-q" .env.example deploy/middleware/.env.example deploy/k8s/secrets.example.yaml -- \
  "MINIO_ROOT_USER" "MINIO_ROOT_PASSWORD" "MINIO_API_PORT" "MINIO_CONSOLE_PORT" \
  "OBJECT_STORAGE_DRIVER" "OBJECT_STORAGE_ENDPOINT" "OBJECT_STORAGE_EXTERNAL_ENDPOINT" "OBJECT_STORAGE_BUCKET" \
  "OBJECT_STORAGE_REGION" "OBJECT_STORAGE_USE_SSL" "OBJECT_STORAGE_EXTERNAL_USE_SSL" \
  "OBJECT_STORAGE_ACCESS_KEY_ID" "OBJECT_STORAGE_SECRET_ACCESS_KEY"

# --- production msggateway origin / query-token config ---
rg -q "msggateway" service/msggateway/msggateway.go etc/msggateway.yaml
rg -q "AllowQueryToken: true" deploy/k8s/etc/msggateway.yaml
rg -q 'GATEWAY_WS_ALLOW_QUERY_TOKEN: "true"' deploy/k8s/configmap.yaml
rg -q 'GATEWAY_WS_ALLOWED_ORIGINS: "https://agenticim\.xyz"' deploy/k8s/configmap.yaml
if rg -q 'GATEWAY_WS_ALLOWED_ORIGINS:\s*""' deploy/k8s/configmap.yaml; then
  echo "production k8s websocket origins must not be empty" >&2
  exit 1
fi
rg -F -q 'AllowedOrigins: ${GATEWAY_WS_ALLOWED_ORIGINS}' deploy/k8s/etc/msggateway.yaml
rg -q 'AllowedOrigins: http://localhost:5173,http://127\.0\.0\.1:5173' etc/msggateway.yaml
rg -q "AllowQueryToken: true" etc/msggateway.yaml
if ! grep -q 'AllowQueryToken: true' deploy/k8s/etc/msggateway.yaml; then
  echo "production msggateway must allow query token for browser WebSocket" >&2
  exit 1
fi

# --- production msgtransfer dispatcher/consumer config ---
if grep -A2 '^Dispatcher:' deploy/k8s/etc/msgtransfer.yaml | grep -q 'Driver: noop'; then
  echo "production msgtransfer must not use noop dispatcher" >&2
  exit 1
fi
if grep -q '^DryRun: true' deploy/k8s/etc/msgtransfer.yaml; then
  echo "production msgtransfer must not run in dry-run mode" >&2
  exit 1
fi
if grep -q '^Consumer:' deploy/k8s/etc/msgtransfer.yaml; then
  echo "legacy msgtransfer outbox consumer config resurrected (03 §9 B3b retired)" >&2
  exit 1
fi
if ! grep -A3 '^Kafka:' deploy/k8s/etc/msgtransfer.yaml | grep -q 'Enabled: true'; then
  echo "production msgtransfer must run the kafka chain (sole consume path after 03 §9 B3b)" >&2
  exit 1
fi
if ! grep -A3 '^Dispatcher:' deploy/k8s/etc/msgtransfer.yaml | grep -q 'Driver: gateway'; then
  echo "production msgtransfer must dispatch to msggateway" >&2
  exit 1
fi
if ! grep -A3 '^Dispatcher:' deploy/k8s/etc/msgtransfer.yaml | grep -q 'GatewayEndpoint: http://127\.0\.0\.1:8084'; then
  echo "production msgtransfer must target colocated msggateway internal endpoint" >&2
  exit 1
fi

# --- AuthRPC / MailRPC client endpoints in auth configs ---
python3 - <<'PY'
import sys
import yaml

for path in (
    "deploy/k8s/etc/auth-api.yaml",
    "etc/auth-api.yaml",
):
    with open(path, encoding="utf-8") as f:
        data = yaml.safe_load(f) or {}
    auth_rpc = data.get("AuthRPC")
    if not isinstance(auth_rpc, dict):
        print(f"{path}: AuthRPC section is required", file=sys.stderr)
        sys.exit(1)
    endpoints = auth_rpc.get("Endpoints")
    if not isinstance(endpoints, list) or not endpoints:
        print(f"{path}: AuthRPC.Endpoints must be a non-empty YAML list", file=sys.stderr)
        sys.exit(1)
    for index, endpoint in enumerate(endpoints):
        if not isinstance(endpoint, str) or not endpoint.strip():
            print(f"{path}: AuthRPC.Endpoints[{index}] must be a non-empty string", file=sys.stderr)
            sys.exit(1)

for path in (
    "deploy/k8s/etc/auth-rpc.yaml",
    "etc/auth-rpc.yaml",
):
    with open(path, encoding="utf-8") as f:
        data = yaml.safe_load(f) or {}
    mail_rpc = data.get("MailRPC")
    if not isinstance(mail_rpc, dict):
        print(f"{path}: MailRPC section is required", file=sys.stderr)
        sys.exit(1)
    endpoints = mail_rpc.get("Endpoints")
    if not isinstance(endpoints, list) or not endpoints:
        print(f"{path}: MailRPC.Endpoints must be a non-empty YAML list", file=sys.stderr)
        sys.exit(1)
    for index, endpoint in enumerate(endpoints):
        if not isinstance(endpoint, str) or not endpoint.strip():
            print(f"{path}: MailRPC.Endpoints[{index}] must be a non-empty string", file=sys.stderr)
            sys.exit(1)
PY

# --- production object storage endpoint + env validation ---
if rg -q 'OBJECT_STORAGE_EXTERNAL_ENDPOINT="?((127\.[0-9.]+|localhost|0\.0\.0\.0|\[?::1\]?)(:[0-9]+)?)"?' scripts/bootstrap-server.sh deploy/k8s/secrets.example.yaml; then
  echo "production object storage external endpoint must not be browser-local loopback" >&2
  exit 1
fi
if ! rg -q 'AGENTS_IM_ENV: "production"' deploy/k8s/configmap.yaml; then
  echo "production k8s config must enable production environment validation" >&2
  exit 1
fi

# --- observability manifests (kustomization / loki / prometheus / tempo / otel / ingress / langfuse / secrets) ---
python3 - <<'PY'
import sys
import yaml

with open("deploy/k8s/kustomization.yaml", encoding="utf-8") as f:
    kustomization = yaml.safe_load(f) or {}
resources = set(kustomization.get("resources") or [])
for resource in ("tempo.yaml", "otel-collector.yaml", "prometheus-grafana.yaml", "loki.yaml", "langfuse.yaml"):
    if resource not in resources:
        print(f"deploy/k8s/kustomization.yaml: missing {resource}", file=sys.stderr)
        sys.exit(1)
if "jaeger.yaml" in resources:
    print("deploy/k8s/kustomization.yaml: jaeger.yaml must not be an active resource", file=sys.stderr)
    sys.exit(1)

with open("deploy/k8s/loki.yaml", encoding="utf-8") as f:
    loki_docs = [doc for doc in yaml.safe_load_all(f) if doc]
loki_by_kind_name = {(doc.get("kind"), doc.get("metadata", {}).get("name")): doc for doc in loki_docs}
for kind, name in (("ConfigMap", "loki-config"), ("ConfigMap", "promtail-config"), ("PersistentVolumeClaim", "loki-data"), ("Deployment", "loki"), ("DaemonSet", "promtail"), ("Service", "loki")):
    if (kind, name) not in loki_by_kind_name:
        print(f"deploy/k8s/loki.yaml: missing {kind} {name}", file=sys.stderr)
        sys.exit(1)
loki_config = yaml.safe_load((loki_by_kind_name[("ConfigMap", "loki-config")].get("data") or {}).get("config.yaml") or "") or {}
if loki_config.get("limits_config", {}).get("retention_period") != "168h":
    print("deploy/k8s/loki.yaml: Loki must keep bounded 7-day retention", file=sys.stderr)
    sys.exit(1)
promtail_config = yaml.safe_load((loki_by_kind_name[("ConfigMap", "promtail-config")].get("data") or {}).get("config.yaml") or "") or {}
promtail_clients = promtail_config.get("clients") or []
if not any(client.get("url") == "http://loki.agents-im.svc.cluster.local:3100/loki/api/v1/push" for client in promtail_clients):
    print("deploy/k8s/loki.yaml: Promtail must push to the in-cluster Loki service", file=sys.stderr)
    sys.exit(1)

with open("deploy/k8s/prometheus-grafana.yaml", encoding="utf-8") as f:
    prometheus_grafana_docs = [doc for doc in yaml.safe_load_all(f) if doc]
prometheus_grafana_by_kind_name = {(doc.get("kind"), doc.get("metadata", {}).get("name")): doc for doc in prometheus_grafana_docs}
for kind, name in (("ConfigMap", "grafana-provisioning"), ("PersistentVolumeClaim", "prometheus-data"), ("Deployment", "prometheus"), ("Deployment", "grafana"), ("Service", "prometheus"), ("Service", "grafana")):
    if (kind, name) not in prometheus_grafana_by_kind_name:
        print(f"deploy/k8s/prometheus-grafana.yaml: missing {kind} {name}", file=sys.stderr)
        sys.exit(1)
datasource_config = yaml.safe_load((prometheus_grafana_by_kind_name[("ConfigMap", "grafana-provisioning")].get("data") or {}).get("datasource.yml") or "") or {}
datasources = {item.get("name"): item for item in datasource_config.get("datasources") or []}
expected_datasources = {
    "Prometheus": {"uid": "prometheus", "type": "prometheus", "url": "http://prometheus.agents-im.svc.cluster.local:9090"},
    "Loki": {"uid": "loki", "type": "loki", "url": "http://loki.agents-im.svc.cluster.local:3100"},
    "Tempo": {"uid": "tempo", "type": "tempo", "url": "http://tempo.agents-im.svc.cluster.local:3200"},
}
for name, expected in expected_datasources.items():
    got = datasources.get(name)
    if not got:
        print(f"deploy/k8s/prometheus-grafana.yaml: missing Grafana datasource {name}", file=sys.stderr)
        sys.exit(1)
    for key, want in expected.items():
        if got.get(key) != want:
            print(f"deploy/k8s/prometheus-grafana.yaml: Grafana datasource {name} {key}={got.get(key)!r}, want {want!r}", file=sys.stderr)
            sys.exit(1)
if datasources["Tempo"].get("jsonData", {}).get("tracesToLogsV2", {}).get("datasourceUid") != "loki":
    print("deploy/k8s/prometheus-grafana.yaml: Tempo tracesToLogsV2 must target Loki uid", file=sys.stderr)
    sys.exit(1)

with open("deploy/k8s/tempo.yaml", encoding="utf-8") as f:
    tempo_docs = [doc for doc in yaml.safe_load_all(f) if doc]
if not any(doc.get("kind") == "PersistentVolumeClaim" and doc.get("metadata", {}).get("name") == "tempo-data" for doc in tempo_docs):
    print("deploy/k8s/tempo.yaml: Tempo must use a persistent tempo-data PVC", file=sys.stderr)
    sys.exit(1)
if not any(doc.get("kind") == "Deployment" and doc.get("metadata", {}).get("name") == "tempo" for doc in tempo_docs):
    print("deploy/k8s/tempo.yaml: missing Tempo Deployment", file=sys.stderr)
    sys.exit(1)
if not any(doc.get("kind") == "Service" and doc.get("metadata", {}).get("name") == "tempo" for doc in tempo_docs):
    print("deploy/k8s/tempo.yaml: missing Tempo Service", file=sys.stderr)
    sys.exit(1)

with open("deploy/k8s/otel-collector.yaml", encoding="utf-8") as f:
    otel_text = f.read()
    otel_docs = [doc for doc in yaml.safe_load_all(otel_text) if doc]
if "tempo.agents-im.svc.cluster.local:4317" not in otel_text:
    print("deploy/k8s/otel-collector.yaml: collector must export traces to Tempo", file=sys.stderr)
    sys.exit(1)
if "health_check" not in otel_text or "13133" not in otel_text:
    print("deploy/k8s/otel-collector.yaml: collector must expose health_check extension for probes", file=sys.stderr)
    sys.exit(1)
if not any(doc.get("kind") == "Deployment" and doc.get("metadata", {}).get("name") == "otel-collector" for doc in otel_docs):
    print("deploy/k8s/otel-collector.yaml: missing otel-collector Deployment", file=sys.stderr)
    sys.exit(1)
if not any(doc.get("kind") == "Service" and doc.get("metadata", {}).get("name") == "otel-collector" for doc in otel_docs):
    print("deploy/k8s/otel-collector.yaml: missing otel-collector Service", file=sys.stderr)
    sys.exit(1)

with open("deploy/k8s/ingress.yaml", encoding="utf-8") as f:
    docs = [doc for doc in yaml.safe_load_all(f) if doc]

def ingress_by_host(host):
    for doc in docs:
        if doc.get("kind") != "Ingress":
            continue
        for rule in doc.get("spec", {}).get("rules", []) or []:
            if rule.get("host") == host:
                return doc, rule
    return None, None

def backend_for(rule, path):
    for item in rule.get("http", {}).get("paths", []) or []:
        if item.get("path") == path:
            service = item.get("backend", {}).get("service", {})
            return service.get("name"), service.get("port", {}).get("number")
    return None, None

def middleware_by_name(name):
    for doc in docs:
        if doc.get("kind") == "Middleware" and doc.get("metadata", {}).get("name") == name:
            return doc
    return None

expected = {
    "langfuse.agenticim.xyz": ("langfuse", 3000, "langfuse-agenticim-xyz-tls"),
    "minio.agenticim.xyz": ("minio", 9001, "minio-agenticim-xyz-tls"),
}
for host, (svc, port, tls_secret) in expected.items():
    ingress, rule = ingress_by_host(host)
    if not ingress:
        print(f"deploy/k8s/ingress.yaml: missing ingress rule for {host}", file=sys.stderr)
        sys.exit(1)
    tls_hosts = {
        tls_host: tls.get("secretName")
        for tls in ingress.get("spec", {}).get("tls", []) or []
        for tls_host in tls.get("hosts", []) or []
    }
    if tls_hosts.get(host) != tls_secret:
        print(f"deploy/k8s/ingress.yaml: {host} must use TLS secret {tls_secret}", file=sys.stderr)
        sys.exit(1)
    got_svc, got_port = backend_for(rule, "/")
    if (got_svc, got_port) != (svc, port):
        print(f"deploy/k8s/ingress.yaml: {host}/ routes to {(got_svc, got_port)}, want {(svc, port)}", file=sys.stderr)
        sys.exit(1)
    if host == "minio.agenticim.xyz":
        middlewares = ingress.get("metadata", {}).get("annotations", {}).get("traefik.ingress.kubernetes.io/router.middlewares", "")
        if "agents-im-minio-console-basic-auth@kubernetescrd" not in middlewares:
            print("deploy/k8s/ingress.yaml: minio.agenticim.xyz must keep basic auth", file=sys.stderr)
            sys.exit(1)
        middleware = middleware_by_name("minio-console-basic-auth")
        if not middleware:
            print("deploy/k8s/ingress.yaml: missing minio-console-basic-auth middleware", file=sys.stderr)
            sys.exit(1)
        basic_auth = middleware.get("spec", {}).get("basicAuth", {})
        if basic_auth.get("secret") != "observability-basic-auth" or basic_auth.get("removeHeader") is not True:
            print("deploy/k8s/ingress.yaml: minio-console-basic-auth must use observability-basic-auth and remove Authorization header", file=sys.stderr)
            sys.exit(1)

jaeger_ingress, _ = ingress_by_host("jaeger.agenticim.xyz")
if jaeger_ingress:
    print("deploy/k8s/ingress.yaml: jaeger public ingress must be removed; use Grafana Tempo instead", file=sys.stderr)
    sys.exit(1)

prometheus_ingress, _ = ingress_by_host("prometheus.agenticim.xyz")
if prometheus_ingress:
    print("deploy/k8s/ingress.yaml: prometheus.agenticim.xyz public ingress must be removed; use ms.agenticim.xyz/observability/metrics", file=sys.stderr)
    sys.exit(1)

def backend_for_host_path(host, path):
    for doc in docs:
        for rule in doc.get("spec", {}).get("rules", []) or []:
            if rule.get("host") != host:
                continue
            service_name, service_port = backend_for(rule, path)
            if service_name:
                return doc, service_name, service_port
    return None, None, None

metrics_ingress, metrics_svc, metrics_port = backend_for_host_path("ms.agenticim.xyz", "/observability/metrics")
if (metrics_svc, metrics_port) != ("prometheus", 9090):
    print("deploy/k8s/ingress.yaml: ms.agenticim.xyz/observability/metrics must route to prometheus:9090", file=sys.stderr)
    sys.exit(1)
metrics_middlewares = metrics_ingress.get("metadata", {}).get("annotations", {}).get("traefik.ingress.kubernetes.io/router.middlewares", "")
if "agents-im-observability-basic-auth@kubernetescrd" not in metrics_middlewares:
    print("deploy/k8s/ingress.yaml: /observability/metrics must keep observability basic auth", file=sys.stderr)
    sys.exit(1)

ms_media_expectations = {
    "/media": ("media-api", 8089),
    "/agents-im-media": ("agents-im-minio", 9000),
}
for path, expected_backend in ms_media_expectations.items():
    _, service_name, service_port = backend_for_host_path("ms.agenticim.xyz", path)
    if (service_name, service_port) != expected_backend:
        print(
            f"deploy/k8s/ingress.yaml: ms.agenticim.xyz{path} routes to {(service_name, service_port)}, want {expected_backend}",
            file=sys.stderr,
        )
        sys.exit(1)

redirect_expectations = {
    "/observability/logs": "agents-im-observability-logs-redirect@kubernetescrd",
    "/observability/traces": "agents-im-observability-traces-redirect@kubernetescrd",
    "/observability/llm": "agents-im-observability-llm-redirect@kubernetescrd",
}
for path, middleware in redirect_expectations.items():
    redirect_ingress, _, _ = backend_for_host_path("ms.agenticim.xyz", path)
    if not redirect_ingress:
        print(f"deploy/k8s/ingress.yaml: missing ms.agenticim.xyz{path} redirect ingress", file=sys.stderr)
        sys.exit(1)
    middlewares = redirect_ingress.get("metadata", {}).get("annotations", {}).get("traefik.ingress.kubernetes.io/router.middlewares", "")
    if middleware not in middlewares:
        print(f"deploy/k8s/ingress.yaml: {path} must use {middleware}", file=sys.stderr)
        sys.exit(1)

middlewares_by_name = {
    doc.get("metadata", {}).get("name"): doc
    for doc in docs
    if doc.get("kind") == "Middleware"
}
redirect_url_expectations = {
    "observability-logs-redirect": ("schemaVersion=1", "%22type%22%3A%22loki%22", "%22uid%22%3A%22loki%22"),
    "observability-traces-redirect": ("schemaVersion=1", "%22type%22%3A%22tempo%22", "%22uid%22%3A%22tempo%22", "%22queryType%22%3A%22traceql%22"),
}
for name, required_parts in redirect_url_expectations.items():
    replacement = middlewares_by_name.get(name, {}).get("spec", {}).get("redirectRegex", {}).get("replacement", "")
    missing = [part for part in required_parts if part not in replacement]
    if missing:
        print(f"deploy/k8s/ingress.yaml: {name} replacement must use explicit Grafana Explore panes datasource; missing {missing}", file=sys.stderr)
        sys.exit(1)

with open("deploy/k8s/langfuse.yaml", encoding="utf-8") as f:
    langfuse_docs = [doc for doc in yaml.safe_load_all(f) if doc]
if not any(doc.get("kind") == "Deployment" and doc.get("metadata", {}).get("name") == "langfuse" for doc in langfuse_docs):
    print("deploy/k8s/langfuse.yaml: missing langfuse Deployment", file=sys.stderr)
    sys.exit(1)
if not any(doc.get("kind") == "Service" and doc.get("metadata", {}).get("name") == "langfuse" for doc in langfuse_docs):
    print("deploy/k8s/langfuse.yaml: missing langfuse Service", file=sys.stderr)
    sys.exit(1)

with open("deploy/k8s/secrets.example.yaml", encoding="utf-8") as f:
    secrets_text = f.read()
for required in ("LANGFUSE_DATABASE_URL", "NEXTAUTH_SECRET", "SALT", "ENCRYPTION_KEY", "observability-basic-auth"):
    if required not in secrets_text:
        print(f"deploy/k8s/secrets.example.yaml: missing {required}", file=sys.stderr)
        sys.exit(1)
PY

# --- deploy script smoke gates ---
bash scripts/test-deploy-k3s.sh
bash scripts/test-no-latest-images.sh
