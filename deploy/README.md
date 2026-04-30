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

## Public entry

The web service is exposed through k3s NodePort `30080`. Traefik Ingress also routes application paths internally.
