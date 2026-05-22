# agents_im deployment

This project uses a hybrid single-server deployment:

- k3s manages application workloads: all Go APIs/RPCs/workers and the web UI.
- Docker Compose manages middleware: PostgreSQL, Redis, Redpanda, and MinIO.
- Drone builds images, pushes them to GHCR, copies deployment files to the server, and runs `scripts/deploy-k3s.sh` remotely.

## Server bootstrap

Run once on the server from the project root:

```bash
DEEPSEEK_API_KEY='...' ./scripts/bootstrap-server.sh
```

The bootstrap script writes middleware config to `/opt/agents-im/middleware/.env`, starts Docker middleware, installs `postgresql-client` if missing, and creates the k3s `agents-im-secrets` Secret. Real secrets are stored only on the server/k3s or in Drone repository secrets, not in Git.

Langfuse LLM observability is additive to the existing admin LLM trace/audit tables. The default host is `https://langfuse.agenticim.xyz`, configured through `LANGFUSE_HOST`; credentials are read only from `LANGFUSE_PUBLIC_KEY` and `LANGFUSE_SECRET_KEY` in environment/k3s secrets. Keep the values in server/k3s secret storage only, never in Git, issue comments, or chat transcripts. Leave `LLM_OBSERVABILITY_ENABLED=false` and `LLM_OBSERVABILITY_BACKEND=noop` until both Langfuse credentials are present.

The Langfuse web UI is deployed by k3s at `https://langfuse.agenticim.xyz/` using `deploy/k8s/langfuse.yaml`. It reads its own runtime secrets from `agents-im-secrets`: `LANGFUSE_DATABASE_URL`, `NEXTAUTH_SECRET`, `SALT`, and `ENCRYPTION_KEY`. `scripts/bootstrap-server.sh` generates those values and creates a separate `langfuse` PostgreSQL database for new servers; existing servers must add them to the k3s secret and create the database before applying the Langfuse deployment. Do not expose the values in logs or chat.

Jaeger UI is exposed at `https://jaeger.agenticim.xyz/` only behind Traefik basic-auth middleware backed by the `agents-im/observability-basic-auth` Secret. `scripts/bootstrap-server.sh` creates that Secret for new servers and stores the generated operator credentials on the server at `/opt/agents-im/observability-basic-auth.env` with mode `0600`.

Prometheus and Grafana are deployed by k3s from `deploy/k8s/prometheus-grafana.yaml`. Prometheus is exposed at `https://prometheus.agenticim.xyz/` behind the same `observability-basic-auth` Traefik middleware used by Jaeger. Grafana is exposed at `https://grafana.agenticim.xyz/` and uses its own login backed by the `agents-im/grafana-admin` Secret. `scripts/bootstrap-server.sh` creates that Secret for new servers and stores the generated operator credentials at `/opt/agents-im/grafana-admin.env` with mode `0600`. Keep both credential files and k8s Secret values off Git, issues, and chat transcripts.

Loki log aggregation is deployed by k3s from `deploy/k8s/loki.yaml`. Loki is intentionally internal-only at `http://loki.agents-im.svc.cluster.local:3100`; operators query logs through the Grafana `Loki` datasource rather than a public Loki domain.

## Drone repository secrets

Drone is deployed at `https://drone.agenticim.xyz` and the `wujunhui99/agents_im` repository must be active in Drone. Secrets are configured at repository scope in Drone, not in Git.

Required repository secrets:

- `ghcr_username`: GitHub username used for GHCR pushes and server-side pull-secret refresh.
- `ghcr_token`: GitHub token with package push/pull permissions for GHCR.
- `deploy_ssh_host`: production deploy SSH host value. Derive from the documented local SSH alias (`server-ssh-tls`) when available; do not paste the raw value in docs or chat.
- `deploy_ssh_user`: production deploy SSH user value. Derive from the documented local SSH alias when available; do not paste the raw value in docs or chat.
- `deploy_ssh_port`: production deploy SSH port value.
- `deploy_ssh_key`: private key used by the deploy pipeline. Use only a key already authorized for deployment; never commit it or print it in logs.

Current operational status: these six secrets have been configured for `wujunhui99/agents_im` in Drone. Future agents should verify names only unless rotating credentials. If a secret must be rotated, update it in Drone and record only the secret name and rotation date, never the value.

Drone uses `ghcr_token` to push images to GHCR and to refresh the server-side `ghcr-pull-secret` in k3s.


## Drone CI migration

Drone is deployed at `https://drone.agenticim.xyz` with GitHub OAuth and a Docker runner. The repository must be activated in the Drone UI by an authorized GitHub user before `.drone.yml` pipelines run.

Required Drone repository secrets:

- `ghcr_username`: GitHub username used for GHCR pushes and server-side pull-secret refresh.
- `ghcr_token`: GitHub token with package push/pull permissions for GHCR.
- `deploy_ssh_host`: production deploy SSH host value. Keep the raw value only in Drone secrets.
- `deploy_ssh_user`: production deploy SSH user value. Keep the raw value only in Drone secrets.
- `deploy_ssh_port`: production deploy SSH port value.
- `deploy_ssh_key`: private key used by the deploy pipeline.

The Drone pipelines intentionally preserve the deployment contract previously handled by GitHub Actions:

1. `backend-verification` runs go-zero API validation, gofmt check, Go tests, static verification, Docker Compose config validation, and Markdown link checks.
2. `postgres-integration` uses an isolated `postgres:16-alpine` service and never points at the production app database.
3. `deploy-main` runs only on `main` pushes. It calls `scripts/detect-deploy-changes.py`, builds and pushes affected images to GHCR, then syncs the repo to `/opt/agents-im/repo` and invokes `scripts/deploy-k3s.sh` with the established deployment environment contract.

GitHub Actions workflows have been removed because the account has no Actions quota and failed runs trigger noisy email notifications. Drone is now the CI/CD entrypoint. Rollback is still simple: restore `.github/workflows/ci.yml` and `.github/workflows/deploy.yml` from Git history if Drone is unavailable.

## Deployment workflow

`.drone.yml` runs a single `verification` pipeline on pull requests targeting `develop` or `main`, and runs deployment only on pushes to `main`. Verification intentionally does not run on feature/fix/ci branch push events, because each opened or updated MR already emits a `pull_request` build for the same head SHA. Backend verification and PostgreSQL integration are steps in the same pipeline, so each MR exposes one CI task/context instead of two parallel pipeline tasks.

Production public entrypoints:

- User web app: `https://agenticim.xyz/`
- Management System (MS): `https://ms.agenticim.xyz/`
- Prometheus: `https://prometheus.agenticim.xyz/` (Traefik basic-auth, same operator credentials as Jaeger)
- Grafana: `https://grafana.agenticim.xyz/` (Grafana login from the `grafana-admin` Secret)

Jaeger tracing is deployed as a k3s service and exposed through authenticated Traefik ingress:

- OTLP gRPC/HTTP collector: `jaeger-collector.agents-im.svc.cluster.local:4317` / `:4318`
- Query UI: `https://jaeger.agenticim.xyz/`

The public Jaeger UI route must keep the `agents-im-observability-basic-auth@kubernetescrd` middleware attached. Trace data can reveal topology, route names, internal IDs, latency, and error details, so do not remove that access control boundary. Operators can also use private access when needed:

```bash
kubectl -n agents-im port-forward svc/jaeger-collector 16686:16686
```

Then search `http://127.0.0.1:16686/search?traceID=<trace_id>`.

Prometheus scrapes the in-cluster Prometheus service, node-exporter, and the REST/worker services that expose `/metrics`: `user-api`, `auth-api`, `friends-api`, `message-api`, `gateway-ws`, `groups-api`, `agent-api`, and `message-transfer`. It intentionally avoids high-cardinality labels such as account IDs, conversation IDs, message IDs, trace IDs, prompts, or message content. The default production manifest keeps retention bounded to 7 days / 7 GiB so the single-node k3s host does not grow unbounded.

Grafana provisions Prometheus as the default datasource and Loki as the log datasource through `grafana-provisioning`; it stores dashboard/user state on the `grafana-data` PVC. If Grafana admin credentials need rotation, update the `grafana-admin` Secret and `/opt/agents-im/grafana-admin.env`, then restart the Grafana deployment.

Loki stores recent pod logs on the `loki-data` PVC with 7-day retention. Promtail runs as a DaemonSet and reads `/var/log/pods/agents-im_*/*/*.log`, so Loki streams include `namespace="agents-im"` plus the source `filename`. Do not expose Loki directly to the public Internet; use Grafana Explore with queries such as:

```logql
{namespace="agents-im"}
{namespace="agents-im", filename=~".*/message-api/.*"} |= "error"
```

Avoid adding user IDs, account IDs, conversation IDs, message IDs, trace IDs, prompts, or message content as Loki labels; those values belong in log fields/text only after normal redaction.

Langfuse LLM observability has two pieces:

- In-app export configuration through `LANGFUSE_HOST`, `LANGFUSE_PUBLIC_KEY`, and `LANGFUSE_SECRET_KEY`.
- The self-hosted Langfuse UI at `https://langfuse.agenticim.xyz/`, deployed by `deploy/k8s/langfuse.yaml` and backed by the k3s `agents-im-secrets` values `LANGFUSE_DATABASE_URL`, `NEXTAUTH_SECRET`, `SALT`, and `ENCRYPTION_KEY`.

The Management System root serves the web SPA and the frontend opens the read-only admin console from that host. On `ms.agenticim.xyz`, Ingress exposes only the admin API prefixes served by `message-api` (`/admin/dashboard`, `/admin/llm-traces`, `/admin/conversations`, `/admin/users`) plus `/` to `web`. `admin.agenticim.xyz` is deprecated and has a Traefik permanent redirect to the same path on `https://ms.agenticim.xyz`; do not use it as the primary management entrypoint.

The deploy pipeline has three steps:

1. `detect changes`: classifies changed files and emits `build_required`, `deploy_required`, `config_only`, `backend_services`, `web_required`, `image_services`, `rollout_services`, and restart service outputs.
2. `build images`: builds only services listed in `image_services`; backend services use the Dockerfile `backend` target and `SERVICE=<name>` build arg, and web uses the Dockerfile `web` target. Each selected image is published to GHCR only with the immutable `${DRONE_COMMIT_SHA}` tag; mutable `latest` tags are not published or deployed.
3. `deploy`: connects to the server over SSH using the `deploy_ssh_*` secrets, syncs the repository files to `/opt/agents-im/repo`, then runs `scripts/deploy-k3s.sh` with `IMAGE_REGISTRY`, `IMAGE_TAG`, GHCR credentials, `IMAGE_SERVICES`, `ROLLOUT_SERVICES`, optional `RESTART_SERVICES`, and `MIDDLEWARE_DIR=/opt/agents-im/middleware`.

Backend images are built for:

- API services: `user-api`, `auth-api`, `friends-api`, `message-api`, `gateway-ws`, `groups-api`, `agent-api`
- Worker: `message-transfer`
- RPC services: `user-rpc`, `auth-rpc`, `friends-rpc`, `groups-rpc`, `message-rpc`, `mail-rpc`

`deploy-k3s.sh` starts middleware Compose, runs PostgreSQL migrations from the server-side k3s secret `DATABASE_URL`, renders `deploy/k8s` with safe immutable application image tags, applies the rendered resources, sets selected deployment images to the current commit SHA tag, and waits only for selected image updates plus explicit config restarts. Deployment manifests intentionally contain placeholder image tags in Git; deploy-time rendering replaces placeholders with either the selected `${IMAGE_TAG}` or the currently deployed image for non-selected services before `kubectl apply`. When `SKIP_SET_IMAGE=false` and `IMAGE_SERVICES` is empty, the script keeps the manual full-deployment behavior by updating every known deployment image to `${IMAGE_TAG}`. Mutable image tags are refused: callers must provide a non-empty immutable `IMAGE_TAG`.

### Selective image builds

`detect-changes` uses these first-version ownership rules:

- Docs-only changes (`docs/**`, `README.md`, other Markdown) do not deploy.
- `web/**`, including web package files and nginx config, builds and deploys only `web`.
- `cmd/<service>/**` builds and deploys only that backend service.
- `api/<domain>.api` builds the matching API service, for example `api/user.api` -> `user-api`.
- `etc/<service>.yaml` and `deploy/k8s/etc/<service>.yaml` are config-only service rollouts and do not build images.
- `deploy/k8s/**` shared manifest changes deploy config and restart affected services; broad manifest files use all services because ownership is not safely inferable.
- `proto/**` changes build all backend services. Generated RPC contracts can affect callers across service boundaries, so the first selective version intentionally fails safe here.
- Shared backend/build inputs such as `go.mod`, `go.sum`, `Dockerfile`, `.dockerignore`, `internal/**`, `db/**`, and `scripts/migrate-postgres.sh` build all backend services. They do not build `web` unless a web-owned path also changed.
- Unknown non-doc paths fail safe by building all backend services.

The backend image list is stable and ordered:

- API services: `user-api`, `auth-api`, `friends-api`, `message-api`, `gateway-ws`, `groups-api`, `agent-api`
- Worker: `message-transfer`
- RPC services: `user-rpc`, `auth-rpc`, `friends-rpc`, `groups-rpc`, `message-rpc`, `mail-rpc`

### Config-only deploy

For pushes that only change deployment configuration, `detect-changes` sets `config_only=true`. Current config-only inputs are:

- `deploy/k8s/**`
- `scripts/deploy-k3s.sh`
- `.drone.yml`
- `scripts/ci/**`
- `etc/<service>.yaml`

Markdown/doc-only changes do not deploy. There is no GitHub Actions `workflow_dispatch`; manual or replayed Drone runs should still respect the `main` branch deployment gate.

In config-only mode, backend/web image build jobs are skipped and the deploy job runs `scripts/deploy-k3s.sh` with:

```bash
SKIP_SET_IMAGE=true
SKIP_MIDDLEWARE=true
SKIP_MIGRATIONS=true
ROLLOUT_SERVICES='<affected services>'
RESTART_SERVICES='<affected services>'
RESTART_ROLLOUT=true
```

This keeps existing image tags, skips Docker Compose middleware startup and database migrations, applies rendered k3s resources with existing application images preserved, then restarts and waits only for the selected deployment. ConfigMap changes do not reliably recreate Pods by themselves, so config-only deploy must use `RESTART_ROLLOUT=true` for affected services.

## Drone operations runbook

Use read-only checks first and keep raw server/secret values redacted.

```bash
# Runtime status from the server
ssh server-ssh-tls 'kubectl -n drone get pods,svc,ingress'
ssh server-ssh-tls 'kubectl -n drone logs deployment/drone-server --tail=100'
ssh server-ssh-tls 'kubectl -n drone logs deployment/drone-runner-docker --tail=100'

# Public entry check
curl -I https://drone.agenticim.xyz/
```

Expected steady state:

- `drone-server` pod is `Running`.
- `drone-runner-docker` pod is `Running` and logs contain `successfully pinged the remote server` / `polling the remote server`.
- Ingress host is `drone.agenticim.xyz` and TLS is issued by Let's Encrypt.

Repository activation and secret verification should be done in the Drone UI or API. Only verify that the six required secret names exist; never print secret values in logs, issue comments, or handoffs.

### Drone troubleshooting notes

GitHub commit status only reports coarse descriptions such as `Build encountered an error`; when Drone fails, inspect Drone runner/server evidence before assuming the application tests failed.

For the May 2026 migration incident, the useful evidence sources were:

```bash
# Runner/server logs from the documented SSH alias; redact secrets before sharing.
ssh server-ssh-tls 'kubectl -n drone logs deployment/drone-runner-docker --tail=300'
ssh server-ssh-tls 'kubectl -n drone logs deployment/drone-server --tail=300'

# If the Drone UI/API requires login, copy the server SQLite DB and inspect build/stage/step state.
ssh server-ssh-tls '
  pod=$(kubectl -n drone get pod -l app=drone-server -o jsonpath="{.items[0].metadata.name}")
  kubectl -n drone cp "$pod:/data/database.sqlite" /tmp/drone.sqlite >/dev/null
  docker run --rm -v /tmp/drone.sqlite:/db.sqlite:ro alpine:3.20 sh -c '\''apk add --no-cache sqlite >/dev/null; \
    sqlite3 -header -column /db.sqlite "select build_id, build_number, build_event, build_status, build_error, substr(build_after,1,12) as sha, build_started, build_finished from builds order by build_id desc limit 10;"; \
    echo ===stages===; \
    sqlite3 -header -column /db.sqlite "select stage_id, stage_build_id, stage_name, stage_status, stage_error, stage_exit_code, stage_started, stage_stopped from stages order by stage_id desc limit 20;"; \
    echo ===steps===; \
    sqlite3 -header -column /db.sqlite "select step_id, step_stage_id, step_name, step_status, step_error, step_exit_code, step_started, step_stopped from steps order by step_id desc limit 30;"'\''
'
```

Known Drone failure signatures and fixes:

- `linter: untrusted repositories cannot mount host volumes`: Drone Docker runner rejects host-volume mounts unless the repository is trusted. Keep `/var/run/docker.sock` only in jobs that truly need Docker daemon access, such as `deploy-main` image builds on trusted `main` deploys. `backend-verification` must not mount the host Docker socket because it only needs `docker compose config`, which uses the Compose plugin without talking to a daemon.
- `E: Unable to locate package docker-compose-plugin` in `golang:*-bookworm`: install Docker CLI/Compose from Docker's official Debian apt repository, not the default Debian sources.
- `ModuleNotFoundError: No module named 'yaml'`: `scripts/verify-static.sh` imports Python `yaml`; install `python3-yaml` in the backend verification image before running static verification.

If a build remains `pending` while older builds are `running`, check Docker runner capacity and current `drone-*` containers on the server. A single Docker runner with capacity 2 can leave newer push/PR builds queued until the older pair finishes.

## go-zero RPC config naming note

RPC config structs embed `zrpc.RpcServerConf`, which already contains a go-zero transport-level `Auth bool` option. A business field named exactly `Auth` conflicts with that embedded field through go-zero's anonymous-field config loader and can fail startup with `conflict key auth, pay attention to anonymous fields`.

`JWTAuth` does not reproduce that conflict in go-zero v1.10.1, but `auth-rpc` intentionally uses `TokenAuth` for the token-signing configuration because the service owns token issuance/verification rather than go-zero HTTP JWT middleware. This keeps three concepts distinct:

- `zrpc.RpcServerConf.Auth`: go-zero RPC transport auth switch.
- REST API `Auth`: go-zero HTTP JWT middleware config block.
- `auth-rpc` `TokenAuth`: business token/JWT signing settings used by the auth domain.

```yaml
TokenAuth:
  AccessSecret: ${JWT_ACCESS_SECRET}
  AccessExpire: 86400
```

If a rollout fails with a log like `conflict key ... pay attention to anonymous fields`, inspect the affected service's config struct and generated ConfigMap first. In the May 2026 incident, `auth-rpc` entered `CrashLoopBackOff` with `conflict key auth`; the confirmed unsafe pattern is a business config field named `Auth` alongside the embedded `zrpc.RpcServerConf`. Keep the business field distinct (`TokenAuth`) and cover it with a config-load regression test instead of hiding the failure with a remote-only manual patch.

## Ports and host networking

Current k3s manifests use `hostNetwork: true`, so each service binds host ports directly. Keep `ListenOn`, container ports, and Service `port` / `targetPort` aligned.

RPC ports currently include:

- `user-rpc`: `9090`
- `auth-rpc`: `9091`
- `friends-rpc`: `9092`
- `groups-rpc`: `9103` (`9093` is avoided because it is occupied by server SSH/systemd socket)
- `message-rpc`: `9094`
- `mail-rpc`: `9095`

## Object storage

MinIO is bound to localhost by the middleware Compose file:

- API: `127.0.0.1:9000`
- Console: `127.0.0.1:9001`

Required middleware/server secret values:

- `MINIO_ROOT_USER`
- `MINIO_ROOT_PASSWORD`
- `MINIO_API_PORT`
- `MINIO_CONSOLE_PORT`
- `OBJECT_STORAGE_DRIVER=minio`
- `OBJECT_STORAGE_ENDPOINT`
- `OBJECT_STORAGE_EXTERNAL_ENDPOINT`
- `OBJECT_STORAGE_BUCKET`
- `OBJECT_STORAGE_REGION`
- `OBJECT_STORAGE_USE_SSL`
- `OBJECT_STORAGE_EXTERNAL_USE_SSL`
- `OBJECT_STORAGE_ACCESS_KEY_ID`
- `OBJECT_STORAGE_SECRET_ACCESS_KEY`

Do not commit real MinIO credentials. The example files contain placeholders only.
`OBJECT_STORAGE_ENDPOINT` is the server-local MinIO API endpoint used by `user-api`.
`OBJECT_STORAGE_EXTERNAL_ENDPOINT` is embedded into presigned browser upload/download URLs and must be reachable from end-user browsers; do not set it to `localhost`, `127.0.0.1`, or another loopback/unspecified address in production.
For the current single-server k3s + Docker Compose topology, use the application origin as the browser-facing endpoint (`agenticim.xyz`) and route the bucket path `/agents-im-media` through Traefik to the server-local MinIO API. The internal endpoint remains `127.0.0.1:9000`; only the browser-facing presigned URL host changes.
When `OBJECT_STORAGE_EXTERNAL_ENDPOINT` differs from the internal `OBJECT_STORAGE_ENDPOINT`, presigned browser URLs default to HTTPS. Set `OBJECT_STORAGE_EXTERNAL_USE_SSL=false` only for an explicitly HTTP external object-storage endpoint.
`scripts/bootstrap-server.sh` requires `OBJECT_STORAGE_EXTERNAL_ENDPOINT` and rejects browser-local loopback values before writing the Kubernetes secret.

## Sandboxed Python executor

`agent-api` and `message-api` include a fail-closed Python executor. The default `agent_creator` / `AI 助手` is the only built-in Agent with an automatic `python.execute` registry binding; other Agents still require explicit tool binding. Production now enables the Kubernetes backend for the shared runtime used by `agent-api` and `message-api`:

```yaml
PYTHON_EXECUTOR_BACKEND: "k8s"
PYTHON_EXECUTOR_K8S_NAMESPACE: "agent-python-sandbox"
PYTHON_EXECUTOR_K8S_IMAGE: "ghcr.io/wujunhui99/agents_im/python-sandbox:<pinned-tag-or-digest>"
PYTHON_EXECUTOR_K8S_SERVICE_ACCOUNT_NAME: "python-executor-controller"
```

Keep non-production/local configs disabled unless a reviewed sandbox cluster is available. Enabling `k8s` without namespace/image/client configuration is a startup error, not a silent fallback. The production ConfigMap values must include resource limits as well as the pinned image and sandbox service account:

```yaml
PYTHON_EXECUTOR_BACKEND: "k8s"
PYTHON_EXECUTOR_K8S_NAMESPACE: "agent-python-sandbox"
PYTHON_EXECUTOR_K8S_IMAGE: "ghcr.io/wujunhui99/agents_im/python-sandbox:<pinned-tag-or-digest>"
PYTHON_EXECUTOR_K8S_SERVICE_ACCOUNT_NAME: "python-executor-controller"
PYTHON_EXECUTOR_MAX_TIMEOUT_SECONDS: "30"
PYTHON_EXECUTOR_MAX_MEMORY_MIB: "256"
PYTHON_EXECUTOR_MAX_OUTPUT_BYTES: "65536"
```

The sandbox Job manifest intentionally disables service account token automount, host networking, privileged mode, privilege escalation, hostPath volumes, and Linux capabilities. Do not add Docker socket mounts, host filesystem mounts, shell access, default egress, or runtime package installation to the sandbox path.

Runner image contract and scaffold: [`python-sandbox/README.md`](./python-sandbox/README.md).

Production enablement checklist:

1. Build and publish a pinned sandbox runner image from the reviewed `deploy/python-sandbox` contract.
2. Create the dedicated sandbox namespace, for example `agent-python-sandbox`.
3. Apply default-deny ingress and egress NetworkPolicy in that namespace.
4. Apply Pod Security controls that reject privileged pods, host namespaces, hostPath, and privilege escalation.
5. Grant only scoped RBAC for creating/watching/reading/deleting owned sandbox Jobs, ConfigMaps, and logs; keep the sandbox Pod service account token automount disabled.
6. Ensure the sandbox namespace has access to the GHCR pull secret referenced by the Job Pod (`ghcr-pull-secret`).
7. Set `PYTHON_EXECUTOR_BACKEND=k8s` plus namespace/image/service-account/resource limits for both `agent-api` and `message-api`.
8. Smoke test by chatting with `AI 助手` and asking for a calculation that should use Python; verify the final reply and tool-run evidence. A WebSocket open event alone does not prove Python execution works.

## Public entry

The web service is exposed through k3s NodePort `30080`. Traefik Ingress also routes application paths internally.

### 2026-05-19 Drone runtime note

- Docker Hub authenticated pulls are configured on the deployment host Docker daemon to avoid anonymous pull limits during Drone Docker-runner deploy stages.
- If changing this setup, verify `docker:29-cli`, `golang:1.25-alpine`, `node:22-alpine`, `nginx:1.29-alpine`, `alpine:3.22`, and `alpine/git:2.45.2` can be pulled without anonymous rate-limit errors.
