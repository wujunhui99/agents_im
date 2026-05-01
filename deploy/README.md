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

`.github/workflows/deploy.yml` runs on pushes to `main` and supports manual `workflow_dispatch`.

The workflow has four jobs:

1. `detect-changes`: classifies the push as full build/deploy, config-only deploy, or no deploy.
2. `build-backend`: builds one backend image per service using the Dockerfile `backend` target and `SERVICE=<name>` build arg. It publishes each image to GHCR with both `${GITHUB_SHA}` and `latest` tags.
3. `build-web`: builds the web UI image using the Dockerfile `web` target and publishes `${GITHUB_SHA}` and `latest` tags.
4. `deploy`: connects to the server over SSH using the `SERVER_*` secrets, syncs the repository files to `/opt/agents-im/repo`, then runs `scripts/deploy-k3s.sh` with `IMAGE_REGISTRY`, `IMAGE_TAG`, GHCR credentials, and `MIDDLEWARE_DIR=/opt/agents-im/middleware`.

Backend images are built for:

- API services: `user-api`, `auth-api`, `friends-api`, `message-api`, `gateway-ws`, `groups-api`, `agent-api`
- Worker: `message-transfer`
- RPC services: `user-rpc`, `auth-rpc`, `friends-rpc`, `groups-rpc`, `message-rpc`

`deploy-k3s.sh` starts middleware Compose, runs PostgreSQL migrations from the server-side k3s secret `DATABASE_URL`, applies `deploy/k8s`, sets each deployment image to the current commit SHA tag, and waits for rollout status.

### Config-only deploy

For pushes that only change deployment configuration, `detect-changes` sets `config_only=true`. Current config-only inputs are:

- `deploy/k8s/**`
- `scripts/deploy-k3s.sh`
- `.github/workflows/deploy.yml`

Markdown/doc-only changes do not deploy. Manual `workflow_dispatch` always performs a full build and deploy.

In config-only mode, backend/web image build jobs are skipped and the deploy job runs `scripts/deploy-k3s.sh` with:

```bash
SKIP_SET_IMAGE=true
SKIP_MIDDLEWARE=true
SKIP_MIGRATIONS=true
ROLLOUT_SERVICES=groups-rpc
RESTART_ROLLOUT=true
```

This keeps existing image tags, skips Docker Compose middleware startup and database migrations, applies the k3s manifests, then restarts and waits only for the selected deployment. ConfigMap changes do not reliably recreate Pods by themselves, so config-only deploy must use `RESTART_ROLLOUT=true` for affected services.

Current limitation: `ROLLOUT_SERVICES` is fixed to `groups-rpc` for the first config-only repair. If future config-only changes touch other service ConfigMaps or manifests, extend change detection to infer the affected service instead of reusing this fixed value.

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
