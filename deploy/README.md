# agents_im Deployment

适用场景：修改或排查 Drone、k3s manifest、生产部署、rollout、运行时观测和部署后验证。

本项目是单节点 k3s 部署：

- k3s 运行所有 Go API/RPC/worker、Web、PostgreSQL、Redis、RustFS、Prometheus/Grafana/Loki/Tempo/Langfuse，
  以及 **Drone CI 本体**（namespace `drone`，见 `deploy/k8s/drone/README.md`）。
- 中间件（PostgreSQL/Redis/RustFS）manifests 在 `deploy/k8s/middleware/`，**不在 kustomization 内**，
  由 bootstrap 应用、人工运维（见 `deploy/k8s/middleware/README.md`）；docker compose 中间件已退役。
- Drone 负责 PR 验证和 `main` 部署；CI/CD 入口是 `.drone.yml`。
- 应用 namespace 是 `agents-im`；镜像发布到 `ghcr.io/wujunhui99/agents_im`。

## Bootstrap

新服务器（已装 k3s + docker）从仓库根目录执行：

```bash
# 先把第三方凭据写入 /opt/agents-im/creds.env（0600），键名见脚本头注释
ADMIN_BOOTSTRAP_PASSWORD='[REDACTED]' ./scripts/bootstrap-server.sh
```

脚本完成：生成 `/opt/agents-im/secrets.env`（全新随机密码）、cert-manager + Let's Encrypt issuer、
`agents-im-secrets` 等 k8s secret、k8s 中间件（PG/Redis/RustFS）、langfuse 库与 media bucket、
数据库迁移、Drone server/runner。收尾的人工步骤（OAuth 登录、仓库激活/trusted/secrets、首次全量
部署）见脚本结尾输出。真实 secret 只能保存在服务器、k3s Secret 或 Drone repository secrets 中，
文档、Issue、PR 和聊天里只写 `[REDACTED]`。

## Drone

当前流水线：

1. `verification`：PR 到 `main` 时运行 branch/Issue 校验、backend verification、按变更触发的 frontend verification、PostgreSQL integration。
2. `devops-lab`：`devops` 分支用于构建/渲染部署计划验证。
3. `deploy-main`：`main` push 触发 detect -> build images -> deploy -> notify -> prune。

`deploy-main` 使用 `DRONE_DEPLOY_LOCAL=1`：Drone runner 容器通过 host volume 访问 `/opt/agents-im` 和 `/etc/rancher/k3s/k3s.yaml`，把仓库同步到 `/opt/agents-im/repo` 后在 runner host 上调用 `scripts/ci/drone-deploy.sh` 与 `scripts/deploy-k3s.sh`。当前主路径不是 SSH 同步部署；`scripts/ci/drone-deploy.sh` 里保留的 SSH 分支只作兼容备用。

必须存在的 Drone secret 名称：

- `ghcr_username`
- `ghcr_token`
- `telegram_bot_token`
- `telegram_chat_id`

只核验 secret 名称是否存在，不打印、不记录 secret 值。Claude Code 后台执行 `scripts/drone-watch.sh`；Codex 前台执行或自行轮询后台日志，必须报告 Drone 结果。

## Deploy Selection

部署选择由 `scripts/ci/drone-detect-deploy.sh` 调用 `scripts/detect-deploy-changes.py` 生成 `.drone-deploy.env`。关键输出包括 `build_required`、`deploy_required`、`migration_required`、`image_services_space`、`rollout_services`、`restart_services`。

当前可构建/部署镜像以 `scripts/detect-deploy-changes.py` 的 `BACKEND_SERVICES` / `ALL_IMAGE_SERVICES` 和 `scripts/deploy-k3s.sh` 的 `IMAGE_DEPLOYMENTS` 为准；不要在文档里复制一份长期服务清单。

选择规则：

- `web/**` 只构建部署 `web`。
- `service/<domain>/api/**` -> `<domain>-api`；`service/<domain>/rpc/**` -> `<domain>-rpc`。
- `service/msggateway/**`、`service/msgtransfer/**` 直接映射同名服务。
- `api/<domain>.api` 映射对应 API；`proto/**`、`go.mod`、`go.sum`、`Dockerfile`、`internal/**`、`common/**` 等共享输入 fail safe 到所有后端。
- `deploy/k8s/**`、`.drone.yml`、`scripts/ci/**`、`scripts/deploy-k3s.sh` 是 config-only 部署入口；Markdown-only 不部署。
- `db/migrations/*.sql` 或 `scripts/migrate-postgres.sh` 触发迁移。

## Runtime

生产入口：

- User Web：`https://agenticim.xyz/`
- Management System：`https://ms.agenticim.xyz/`
- Grafana：`https://grafana.agenticim.xyz/`
- Langfuse：`https://langfuse.agenticim.xyz/`
- RustFS Console：`https://minio.agenticim.xyz/`（受 basic-auth 保护；host 暂留 minio.agenticim.xyz）
- Prometheus UI：`https://ms.agenticim.xyz/observability/metrics`（受保护路径）

Ingress 路由要点：

路由事实源是 `deploy/k8s/ingress.yaml`；下列内容只作排查入口概览。

- `/auth` -> `auth-api`
- `/me`、`/users`、`/accounts` -> `user-api`
- `/friends` -> `friends-api`
- `/groups` -> `groups-api`
- `/messages`、`/conversations`、`/api/feedback` -> `msg-api`
- `/ws` -> `msggateway`
- `/media` -> `media-api`
- `/agents-im-media` -> `agents-im-oss`（S3-compatible object API，RustFS）
- `minio.agenticim.xyz/` -> `oss:9001`（RustFS Console）
- `/admin/*`、`/api/admin/*`、`/api/feedback` -> `admin-api`
- `/` -> `web`

Observability：

- Prometheus/Grafana/Loki/Tempo 由 `deploy/k8s/*.yaml` 管理。
- Loki 和 Tempo 不暴露公网域名，通过 Grafana Explore 或 MS redirect 查询。
- 日志/指标 label 禁止加入 account id、conversation id、message id、trace id、prompt、message content 等高基数字段或敏感内容。

## Migrations

迁移只在 detect 判定 `migration_required=true` 时执行。Drone 本地部署路径会先运行迁移（连 k3s
postgres ClusterIP + `--network host`），再调用 `deploy-k3s.sh` 且传入 `SKIP_MIGRATIONS=true`，
避免重复迁移。中间件在 k8s（`deploy/k8s/middleware/`），由 bootstrap/人工管理，部署流水线
不涉及（docker compose 中间件及其 `SKIP_MIDDLEWARE` 开关已退役，#486）。

规则：

- 已发布 migration 不可修改，新增变更写下一号 `db/migrations/*.sql`。
- 迁移脚本读取 k3s `agents-im-secrets` 中的数据库连接信息，但日志不得输出连接串。
- `scripts/migrate-postgres.sh` 通过 `schema_migrations` 跳过已应用版本。

## Operations

优先使用只读检查：

```bash
kubectl -n agents-im get deploy,pod,svc,ingress
kubectl -n agents-im rollout status deploy/web
kubectl -n agents-im logs deploy/msg-api --tail=100
bash scripts/drone-watch.sh
```

CI 绿不等于 runtime 绿。影响线上行为的变更合入后，至少核验相关 rollout、日志和一条真实 API/WS smoke 证据。

## Manual Render

本地只渲染部署清单：

```bash
IMAGE_TAG=test-sha IMAGE_SERVICES="web" ./scripts/deploy-k3s.sh --render-only >/tmp/rendered.yaml
```

渲染输出不应包含 `__IMAGE_TAG_REQUIRED__` 或 `:latest` 应用镜像。
