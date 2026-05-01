# agents_im deployment

This project uses a hybrid single-server deployment:

- k3s manages application workloads: all Go APIs/RPCs/workers and the web UI.
- Docker Compose manages middleware: PostgreSQL, Redis, and Redpanda.
- GitHub Actions builds images, pushes them to GHCR, copies deployment files to the server, and runs `scripts/deploy-k3s.sh` remotely.

## Server bootstrap

Run once on the server from the project root:

```bash
DEEPSEEK_API_KEY='...' ./scripts/bootstrap-server.sh
```

The bootstrap script writes middleware config to `/opt/agents-im/middleware/.env`, starts Docker middleware, installs `postgresql-client` if missing, and creates the k3s `agents-im-secrets` Secret. Real secrets are stored only on the server/k3s, not in GitHub.

## GitHub Actions secrets

Required repository secrets:

- `SERVER_HOST`
- `SERVER_USER`
- `SERVER_PORT`
- `SERVER_SSH_KEY`

The workflow uses the built-in `GITHUB_TOKEN` to push images to GHCR and to refresh the server-side `ghcr-pull-secret` in k3s.

## Deployment workflow

`.github/workflows/deploy.yml` runs on pushes to `main` and supports manual `workflow_dispatch`. Deployment jobs are guarded with `github.ref == 'refs/heads/main'`; if a manual dispatch is started from another branch, change detection emits a no-op result and no SSH/deploy step runs.

The workflow has four jobs:

1. `detect-changes`: classifies changed files and emits `build_required`, `deploy_required`, `config_only`, `backend_services`, `web_required`, `image_services`, `rollout_services`, and restart service outputs.
2. `build-backend`: builds only backend services listed in `backend_services` using the Dockerfile `backend` target and `SERVICE=<name>` build arg. It publishes each selected image to GHCR with both `${GITHUB_SHA}` and `latest` tags.
3. `build-web`: builds the web UI image only when `web_required=true` and publishes `${GITHUB_SHA}` and `latest` tags.
4. `deploy`: connects to the server over SSH using the `SERVER_*` secrets, syncs the repository files to `/opt/agents-im/repo`, then runs `scripts/deploy-k3s.sh` with `IMAGE_REGISTRY`, `IMAGE_TAG`, GHCR credentials, `IMAGE_SERVICES`, `ROLLOUT_SERVICES`, optional `RESTART_SERVICES`, and `MIDDLEWARE_DIR=/opt/agents-im/middleware`.

Backend images are built for:

- API services: `user-api`, `auth-api`, `friends-api`, `message-api`, `gateway-ws`, `groups-api`, `agent-api`
- Worker: `message-transfer`
- RPC services: `user-rpc`, `auth-rpc`, `friends-rpc`, `groups-rpc`, `message-rpc`

`deploy-k3s.sh` starts middleware Compose, runs PostgreSQL migrations from the server-side k3s secret `DATABASE_URL`, applies `deploy/k8s`, sets selected deployment images to the current commit SHA tag, and waits for rollout status. When `SKIP_SET_IMAGE=false` and `IMAGE_SERVICES` is empty, the script keeps the legacy full-deploy behavior and sets every service image. Selective deploys pass a space-separated `IMAGE_SERVICES` list so unchanged services are not pointed at a SHA tag that was not built.

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
- RPC services: `user-rpc`, `auth-rpc`, `friends-rpc`, `groups-rpc`, `message-rpc`

### Config-only deploy

For pushes that only change deployment configuration, `detect-changes` sets `config_only=true`. Current config-only inputs are:

- `deploy/k8s/**`
- `scripts/deploy-k3s.sh`
- `.github/workflows/deploy.yml`
- `etc/<service>.yaml`

Markdown/doc-only changes do not deploy. Manual `workflow_dispatch` on `main` performs a full build and deploy; manual dispatch on a non-`main` ref no-ops before any deployment step.

In config-only mode, backend/web image build jobs are skipped and the deploy job runs `scripts/deploy-k3s.sh` with:

```bash
SKIP_SET_IMAGE=true
SKIP_MIDDLEWARE=true
SKIP_MIGRATIONS=true
ROLLOUT_SERVICES='<affected services>'
RESTART_SERVICES='<affected services>'
RESTART_ROLLOUT=true
```

This keeps existing image tags, skips Docker Compose middleware startup and database migrations, applies the k3s manifests, then restarts and waits only for the selected deployment. ConfigMap changes do not reliably recreate Pods by themselves, so config-only deploy must use `RESTART_ROLLOUT=true` for affected services.

## Ports and host networking

Current k3s manifests use `hostNetwork: true`, so each service binds host ports directly. Keep `ListenOn`, container ports, and Service `port` / `targetPort` aligned.

RPC ports currently include:

- `user-rpc`: `9090`
- `auth-rpc`: `9091`
- `friends-rpc`: `9092`
- `groups-rpc`: `9103` (`9093` is avoided because it is occupied by server SSH/systemd socket)
- `message-rpc`: `9094`

## Public entry

The web service is exposed through k3s NodePort `30080`. Traefik Ingress also routes application paths internally.
