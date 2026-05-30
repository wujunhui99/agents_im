# 05 — 可观测性 / CI / 部署 重构完成情况

> 追踪 [`../05-observability-cicd.md`](../05-observability-cicd.md) 的 OB-1..OB-17 落地进度。
> 决策与分阶段路线见该分析文档 §8。状态图例见 [`README.md`](./README.md)。
> **续作者请直接看文末 [§交接：P2/P3 续作指南](#交接p2p3-续作指南)**。

## 阶段总览

| 阶段 | 范围 | 状态 |
|------|------|------|
| **P0 速赢** | OB-1 · OB-10 · OB-17 | ✅ 完成（2026-05-30） |
| **P1 纯后端重构** | OB-7 · OB-5(+OB-6) · OB-12 · OB-4 · OB-13 · OB-16 · OB-8 | ✅ 完成（2026-05-30） |
| **P2 GitOps/CD** | OB-14/15（+OB-9） | 🟡 Argo CD 已接管 prod；Drone→gitops 流（OB-9）待做 |
| **P3 中间件入 k8s** | OB-3 · OB-11 · OB-2 · hostNetwork→ClusterIP | 🟡 Redis+MinIO+**PostgreSQL 已迁**；**Redpanda 已删（确认无用）**；只读从库/OB-11/OB-2/其余待做 |

## 逐条进度

| 编号 | 摘要 | 阶段 | 状态 | PR / Issue | 完成日期 |
|------|------|------|------|-----------|----------|
| OB-1 | CI 单轨：文档同步为 Drone，清除 GHA 失真 | P0 | ✅ | PR #351 | 2026-05-30 |
| OB-10 | 移除 GHA 时代 Telegram 死脚本 | P0 | ✅ | PR #353 / issue #352 | 2026-05-30 |
| OB-17 | 删 `.drone.yml` 并发硬编码，由脚本默认值 3 控制 | P0 | ✅ | PR #355 / issue #354 | 2026-05-30 |
| OB-7 | tracing 配置改 ConfigMap 注入 env，删 30 份 yaml 的 Tracing block | P1 | ✅ | PR #358 / issue #357 | 2026-05-30 |
| OB-5 | 业务 metrics 切 `prometheus/client_golang` | P1 | ✅ | PR #360 / issue #359 | 2026-05-30 |
| OB-6 | 统一 `/metrics` 路径（业务已统一；prometheus 自身路径由 route-prefix 决定） | P1 | ✅ | PR #360 / issue #359 | 2026-05-30 |
| OB-12 | LLM observability sink 异步化（channel + 后台 worker + drop 计数） | P1 | ✅ | PR #362 / issue #361 | 2026-05-30 |
| OB-4 | Prometheus 改 k8s service discovery（注解+relabel+RBAC，顺带补 admin-api） | P1 | ✅ | PR #364 / issue #363 | 2026-05-30 |
| OB-13 | PR CI 加 `frontend-verification`（when.paths 门控 web 改动） | P1 | ✅ | PR #366 / issue #365 | 2026-05-30 |
| OB-16 | ready check audit 表（[readyz-audit.md](../../observability/readyz-audit.md)；发现 readyz 仅装配检查不探测依赖） | P1 | ✅ | PR #367 | 2026-05-30 |
| OB-8 | `detect-deploy-changes` 单元测试 + 接入 CI（CLI 本已具备） | P1 | ✅ | PR #369 / issue #368 | 2026-05-30 |
| OB-15 | Argo CD 已装并接管 `agents-im`（Application `agents-im` Synced/Healthy） | P2 | ✅ | gitops 引导 | 2026-05-30 |
| OB-14 | gitops 仓库 + Argo Application + auto-sync ✅；**Drone PR+label 改 gitops + webhook 待做** | P2 | 🟡 | repo `agents_im-gitops` | 部分 2026-05-30 |
| OB-9 | Drone 仍挂 admin kubeconfig 直接 kubectl 部署；待 Drone→gitops 后摘除 | P2 | ⬜ | — | — |
| OB-3 | 中间件入 k8s：**Redis ✅ / MinIO ✅ / PostgreSQL ✅**（主库已迁+切端点，docker 主/从已停留作回滚）；只读从库重建待做；**Redpanda ✅ 已删**（确认无用，非迁移，PR #375 + 容器/卷已删） | P3 | 🟡 | gitops manifests | PG/Redpanda 2026-05-30 |
| OB-11 | Langfuse 独立 PG | P3 | ⬜ | — | — |
| OB-2 | Loki/Tempo 后端切 MinIO（k8s MinIO 已就绪，可做） | P3 | ⬜ | — | — |
| OB-— | hostNetwork → ClusterIP | P3 | ⬜ | — | — |

## 备注

- **P0 实现要点**：GHA 已在代码层废弃，OB-1 仅文档同步；Drone OSS 无 GitHub 式 repo 明文变量，OB-17 删硬编码后由 `scripts/ci/drone-build-images.sh` 的 `${DRONE_IMAGE_BUILD_PARALLELISM:-3}` 默认值控制。
- **CI 核验**：Drone 不向 GitHub PR 回报状态检查，需在 Drone UI（`https://drone.agenticim.xyz`）核验；项目 norm 为 PR → 立即 merge → Drone 构建（merge 后）。

---

## 交接：P2/P3 续作指南

> 写给接手的执行者。**先读完本节再动手**。所有 live 集群操作经 owner 授权可经 SSH/Drone 自主执行（含数据迁移，brief downtime 已接受）；但 PostgreSQL 等数据迁移务必**备份优先 + 逐步验证**。不做 staging（内存紧张）。

### 当前总体状态（2026-05-30）
- **Argo CD GitOps 已上线**并接管 `agents-im` 命名空间（Synced/Healthy）。改集群期望态 = 改 gitops 仓库并 push。
- **中间件**：Redis、MinIO、**PostgreSQL** 已入 k8s（GitOps 管理），对应 docker 实例已 `stop`（未 `rm`，可回滚）；**Redpanda 已彻底删除**（确认代码层无用：container `rm` + volume `rm`，无回滚需要）。
- App 健康，pod 无异常重启（28 pods Ready，Argo Synced/Healthy）。节点 7.8G 内存 / 可用约 2.9G，4 核，storageclass `local-path`。

### 访问与工具（执行前必读）
- **Drone token**：`secret/drone_token`，是 **`.env` 格式**（含 `DRONE_SERVER=` + `DRONE_TOKEN=`），不是裸 token。取值：`grep '^DRONE_TOKEN=' secret/drone_token | cut -d= -f2-`。查构建：`curl -H "Authorization: Bearer $T" https://drone.agenticim.xyz/api/repos/wujunhui99/agents_im/builds`。
- **服务器 SSH**：`secret/server_ssh`（单行 `ssh ...` 命令）。**zsh 下需 `${=VAR}` 分词**：`SSH_CMD=$(tr -d '\n' < secret/server_ssh); ${=SSH_CMD} -o BatchMode=yes '<remote cmd>'`。
- **无本地 kubeconfig**（只有 `secret/k8s_access.example`）→ 集群 kubectl 一律经 SSH 在节点跑，先 `export KUBECONFIG=/etc/rancher/k3s/k3s.yaml`。
- 节点已装 `argocd` CLI，但 `argocd app diff --core` 报 `argocd-cm not found`；诊断 diff 改用 `kubectl diff -f <(kubectl kustomize <gitops manifests>)`（最准的 apply 预览）。
- **保密**：日志/PR/Issue 一律不打印 secret 值、服务器 IP（写 `[REDACTED]`）。迁数据用一次性 k8s Job + `secretKeyRef` 引凭据，避免命令行明文。

### Argo CD / GitOps 模型
- **gitops 仓库**：`github.com/wujunhui99/agents_im-gitops`（private）。`manifests/` = kustomize 源（业务 + 已迁中间件的 StatefulSet 都在这），`manifests/kustomization.yaml` 的 `images:` 块 pin 业务镜像 SHA。`apps/agents-im.yaml` 是 Argo Application 记录。
- **Argo Application**：`agents-im`（ns `argocd`），source = gitops `manifests/`，dest = `agents-im`，`syncPolicy.automated{prune:false, selfHeal:false}`。
- **改期望态 = 改 gitops 仓库并 push**；**不要 `kubectl apply` 业务/中间件 manifests**（会与 Argo 漂移）。push 后 Argo ~3min 轮询同步；强制刷新：`kubectl -n argocd annotate application agents-im argocd.argoproj.io/refresh=hard --overwrite`。
- **为何 `selfHeal=false`（重要）**：Drone `deploy-main` 目前仍用 `kubectl apply` 直接部署（P2-5/OB-9 未做）。若 `selfHeal=true`，Argo 会把 Drone 部署的新镜像回滚成 gitops pin 的旧 SHA，两套部署打架。**两套并存期间必须保持 `selfHeal=false`**。等做完 P2-5（Drone 改为推 gitops）后，再开 `selfHeal=true` 并最终开 `prune=true`。
- 私有仓库认证：read-only **deploy key**（私钥在节点 `/root/.argocd_gitops_key`，Argo repo secret `argocd/gitops-repo`，ssh url）。
- `git commit -a` **不会** stage 新文件 —— 新增 manifest 必须 `git add <file>` 再 commit（此坑已踩过，见 PR #371）。

### 配置/密钥位置（切端点时关键）
- configmap `agents-im-config`（**在 gitops** `manifests/configmap.yaml`，envFrom 进所有 pod）：`REDIS_ADDR`、`AGENTS_IM_*` 等非密。改它**经 gitops**。
- secret `agents-im-secrets`（**不在 gitops**）：`DATABASE_URL`、`OBJECT_STORAGE_ENDPOINT`、`REDIS_PASSWORD`、`OBJECT_STORAGE_ACCESS_KEY_ID/SECRET_ACCESS_KEY`、`LANGFUSE_DATABASE_URL` 等。改它用 `kubectl patch`（不在 gitops，selfHeal 不受管）。**改完需 `rollout restart` 相关服务**才生效（env 在启动时读）。
- 业务 pod 是 `hostNetwork: true` + `dnsPolicy: ClusterFirstWithHostNet`，**能解析 ClusterIP / svc DNS**，所以中间件入 k8s 不必先改 hostNetwork（hostNetwork→ClusterIP 留到最后）。

### 中间件迁移模式（每组件照做）
1. 在 gitops `manifests/` 加 `<svc>.yaml`（StatefulSet + Service + PVC，凭据用 `secretKeyRef` 引 `agents-im-secrets`），加进 `kustomization.yaml` 的 `resources:`，**`git add` 新文件**，push。
2. Argo 部署新 k8s 实例（与 docker 并存；单组件内存增量够用）。
3. 迁数据（如需）：一次性 k8s **Job**（如 `minio/mc`、`postgres` client），凭据经 `secretKeyRef`（不打印），从旧（docker，经其暴露途径）→ 新（k8s svc）。**校验数据量/行数一致**。
4. 切端点：configmap（gitops）或 secret（kubectl）改成 `<svc>.agents-im.svc.cluster.local:<port>`。
5. `rollout restart` 用该中间件的服务（不确定就多重启几个；Recreate 策略，短暂停机已接受），验证连到新实例。
6. `docker stop <name>` 停旧实例（**不要 `rm`**，留作回滚）。回滚 = `docker start` + 还原端点。

### 已完成（均可回滚）
- **Redis**：k8s StatefulSet（复用 `agents-im-secrets.REDIS_PASSWORD` 作 requirepass）；`REDIS_ADDR`(configmap)→`redis.agents-im.svc.cluster.local:6379`；docker `agents-im-redis` 已 stop。仅 ~3 个 ephemeral key，未迁数据。
- **MinIO**：k8s StatefulSet（root 凭据来自 secret）；`mc mirror` 34 对象（33MiB）已校验一致。**双切换**：内部 `OBJECT_STORAGE_ENDPOINT`(在 **secret**)→`minio.agents-im.svc.cluster.local:9000`；外部 ingress `/agents-im-media`（presigned URL）经 svc `agents-im-minio`（已 repoint：selector→`app: minio`，targetPort→9000）直达 k8s minio。socat `agents-im-minio-proxy` 已 `replicas: 0` 退役。docker `agents-im-minio` 已 stop。
- **PostgreSQL ✅（2026-05-30）**：k8s StatefulSet `postgres`（`postgres:16-alpine`，gitops `manifests/postgres.yaml`，5Gi local-path PVC，凭据 `agents-im-secrets.POSTGRES_USER/PASSWORD/DB`——新加 3 个 key）。
  - **备份优先**：`pg_dumpall --globals-only` + `pg_dump -Fc agents_im/langfuse` 到 `/opt/agents-im/backups/pg-<ts>/`，已校验 TOC + 30/42 表 + 3 roles。
  - **迁数据**：globals(roles `readonly_user`/`replicator`) + `agents_im`(12MB) + `langfuse`(11MB) 经 `kubectl exec ... pg_restore` 还原；切换前再做一次 fresh dump+restore；**逐表 `count(*)` 校验 30 表与 docker 完全一致**。
  - **切端点**：`DATABASE_URL` + `LANGFUSE_DATABASE_URL`(均在 **secret**) host→`postgres.agents-im.svc.cluster.local:5432`（保留 user/pass/db/sslmode）；`rollout restart` 全部 16 个 DB 消费者。验证 docker PG 0 app 连接、k8s PG 可写主库(`pg_is_in_recovery=f`)、写读 round-trip OK、28 pods Ready。
  - **副作用修复**：langfuse 重启后因 k8s `redis` Service 自动注入 `REDIS_PORT=tcp://…` 致启动校验失败（NaN）；在 gitops `langfuse.yaml` 显式设 `REDIS_HOST/REDIS_PORT/REDIS_AUTH` 覆盖，已恢复 Ready。
  - **停旧**：`docker stop agents-im-postgres agents-im-postgres-replica`（未 rm，可回滚）。
  - **未做（后续）**：k8s 内重建只读从库（streaming replication + 独立 Service，OB-3 长期保留给 owner 查询）——按 owner 决策本次只迁主库，从库后续再做；备份目录与 docker 实例均可回滚。

### 之后（按序）
- **~~Redpanda 入 k8s~~ → ✅ 已删除（2026-05-30）**：确认代码层无用（prod `message-transfer` 用 `Driver: outbox` 读 `message_outbox`，从不走 kafka driver；`outboxpublisher.New` producer 路径仅测试调用；topic `message.events.v1` high-watermark=0、零 consumer group、零连接）。已删：Go kafka 代码(`pkg/messaging/{kafka,producer}.go`、`internal/transfer/kafka_consumer*`、`outboxpublisher.Publisher`、config `KafkaConfig`/`KAFKA_*`/`TransferConsumerKafka`，保留 live 的 `MessageEventFromOutbox`/`MessageEventFromMessagingEvent`) + `segmentio/kafka-go` 依赖 + docker-compose redpanda + `KAFKA_*` configmap（gitops `285f728`）+ docker 容器与卷（`rm`）。代码 PR **#375**（agents_im），文档已同步（ARCHITECTURE/README/DEVELOPMENT/RELIABILITY；3 份 kafka design-doc 加弃用横幅）。注：main 上 `verify-static.sh`/`gofmt` 有与本次无关的预存失败，未在此修复。
- **OB-11 Langfuse 独立 PG**：给 langfuse 单独 k8s PG 实例（`LANGFUSE_DATABASE_URL`），与业务 PG 隔离。
- **OB-2 Loki/Tempo 后端切 MinIO**：k8s MinIO 已就绪；改 loki/tempo 配置 storage 后端为 S3/MinIO + 建对应 bucket。
- **OB-9 / P2-5 Drone→gitops PR+label**：Drone `deploy-main` 改为「构建镜像 → 改 gitops repo `images:` tag（PR+label 自动合）」，删 `kubectl` 部署 + `/etc/rancher/k3s` kubeconfig 挂载；**完成后把 Argo `selfHeal` 开回 `true`，并开 `prune=true`**。
- **hostNetwork→ClusterIP**：所有业务 deployment 删 `hostNetwork`/`dnsPolicy`，走 ClusterIP + ingress；先确认无客户端依赖宿主机 IP。
- **收尾**：全部灰度确认后删 docker-compose 中间件（`/opt/agents-im/middleware`）。

### 核验手段
- Argo：`kubectl -n argocd get application agents-im`（期望 Synced/Healthy）。
- Pod：`kubectl -n agents-im get pods`（restarts 不应异常增长）。
- Drone 构建（merge 后）：用 token 查 builds API（见上）。
- 数据迁移：迁移 Job 输出源/目的数据量对比。
