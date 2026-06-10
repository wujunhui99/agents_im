# 中间件 manifests（bootstrap 事实源）

PostgreSQL / Redis / MinIO 的 k8s StatefulSet+Service（OB-3 已迁入 k8s，docker compose 中间件退役）。

- **不在 `deploy/k8s/kustomization.yaml` 里**：deploy-main（Drone）不管理中间件，避免应用部署
  误碰有状态服务。变更中间件需人工 `kubectl apply` 并评估数据影响。
- 由 `scripts/bootstrap-server.sh` 在新服务器引导时 apply；凭据经 `agents-im-secrets`
  （`POSTGRES_*`、`REDIS_PASSWORD`、`OBJECT_STORAGE_ACCESS_KEY_ID/SECRET_ACCESS_KEY`）。
- 历史：这些 manifests 源自 gitops 仓库 `agents_im-gitops/manifests/`（Argo CD 旁路已停用，
  2026-06-10 服务器重装后以本目录为事实源）。
