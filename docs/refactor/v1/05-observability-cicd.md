# 05 — 可观测性 / Drone CI / 部署 重构分析

> 目标：审计当前观测栈、CI 链路、部署链路的技术债与不一致，给出收敛建议。
>
> 范围：`internal/observability/`、`internal/llmobs/`、`deploy/k8s/`、`deploy/middleware/`、`scripts/ci/`、`.drone.yml`、`docs/deployment-k3s-pitfalls.md`、`docs/RELIABILITY.md`。
>
> **路径约定**：本文中 `internal/observability/`、`internal/llmobs/`、`internal/health/`、`internal/config/` 等是**当前真实位置**的事实描述；按 00-decisions **D10**，重构后这些包将统一搬到 `pkg/observability/`、`pkg/llmobs/`、`pkg/health/`、`pkg/config/`。文档其它章节提到的 metric 注册位置、tracing setup、health check 注册点等，**改造后均对应 `pkg/<name>/`**。

---

## 1. 现状速描

### 1.1 可观测性栈（k8s 内）

| 组件                  | 文件                                | 用途                                |
|-----------------------|-------------------------------------|-------------------------------------|
| Prometheus            | `deploy/k8s/prometheus-grafana.yaml` | 指标采集                            |
| Grafana               | 同上                                | 统一查询 UI                         |
| Loki                  | `deploy/k8s/loki.yaml`              | 日志（filesystem 后端，无外部存储） |
| Promtail              | （应该在 loki.yaml 内）             | 日志 collector                      |
| Tempo                 | `deploy/k8s/tempo.yaml`             | trace 存储（local backend）          |
| OTel Collector        | `deploy/k8s/otel-collector.yaml`    | OTLP → Tempo 转发                    |
| Langfuse              | `deploy/k8s/langfuse.yaml`          | LLM run 观测后端                    |
| node-exporter         | （未直接见到）                       | 节点指标                            |

### 1.2 业务侧观测代码

- `internal/observability/`：tracing.go（OTel SDK setup）、trace.go（context header propagation）、metrics.go（自实现 Prometheus text/metrics）、grpc.go（unary interceptor）。
- `internal/llmobs/`：langfuse sink + noop sink + memory sink + Eino callback handler。

### 1.3 部署架构

- **k3s** 单节点跑业务进程（hostNetwork: true）；
- **Docker Compose** 跑中间件（PostgreSQL / Redis / Redpanda / MinIO）于 `/opt/agents-im/middleware/`，**不进 k8s**；
- 镜像通过 GHCR；
- 配置通过 ConfigMap (`agents-im-config`、`agents-im-etc`) + Secret (`agents-im-secrets`)。

### 1.4 CI

- Drone（`.drone.yml` 227 行）：3 个 pipeline
  - `verification`（PR）：backend verify、postgres integration、telegram notify；
  - `devops-lab`（push to `devops` 分支）：image build benchmark；
  - `deploy-main`（push to `main`）：detect → build → deploy → notify。
- GitHub Actions：**已废弃**，`.github/workflows/` 下已无任何 workflow（只剩 `ISSUE_TEMPLATE/` 与 `markdown-link-check.json`）；`ARCHITECTURE.md` 等文档仍残留 GHA 描述待清理（见 OB-1）。

---

## 2. 主要技术债

### OB-1 🚨 **CI 双轨：Drone + GitHub Actions 并存**

`ARCHITECTURE.md` 描述部署是"GitHub Actions + GHCR + k3s + Docker Compose"，但实际仓库根有 `.drone.yml` 又承担了 verify + deploy。CLAUDE.md（项目级）声明"Drone CI 是部署事实"，AGENTS.md 又说"GitHub Actions deploy.yml 是发布入口"。

**两份文档矛盾**，且：
- `.github/workflows/ci.yml`、`.github/workflows/deploy.yml` 是否还在跑？
- 哪个是真相？哪个是历史遗留？

> 修复：选定一个（建议保留 Drone，因为它跑在自有 k3s 节点上、能直接 kubectl）。删掉另一个；同步 ARCHITECTURE.md 与 AGENTS.md 与 CLAUDE.md。
>
> **决定（2026-05-30）**：只保留 Drone。GHA 已在代码层删除，剩 `ARCHITECTURE.md` / `AGENTS.md` / `RELIABILITY.md` 等文档同步（纯文档活，P0）。

### OB-2 🚨 Loki / Tempo / Langfuse 全部用 local filesystem，无备份
- Loki `storage.filesystem`；
- Tempo `storage.trace.backend: local`；
- Langfuse 用 PostgreSQL（中间件那台）。

**没有任何对象存储后端**。k3s 节点磁盘损坏 → 全部观测数据丢失。

> 修复方向：至少 Loki/Tempo 用 MinIO（已有部署）作为对象存储后端；或者明确接受"观测数据不重要、丢就丢"并在 docs/RELIABILITY.md 写清楚。

### OB-3 🚨 `hostNetwork: true` 让所有业务进程占宿主机端口
`deploy/k8s/deployments.yaml` 全部业务 deployment 都用 `hostNetwork: true`，因为要让进程能用 hostloop 访问 docker-compose 上的 PG/Redis。

后果：
- user-api:8080、auth-api:8081、friends-api:8082、msg-api:8083、msggateway:8084、groups-api:8085、agent-api:8086、msgtransfer:8087... **每个进程都占一个宿主机端口**；
- 任何端口冲突直接 CrashLoopBackOff；
- 横向扩容不可能（同一节点两个 pod 同端口）；
- `docs/deployment-k3s-pitfalls.md` 已经记录过这个坑。

> 修复方向：把中间件搬进 k8s（statefulset + PVC），然后业务 deployment 改回 ClusterIP；这是个大改动但必须做，否则永远是单节点架构。

### OB-4 ⚠️ Prometheus 配置硬编码 8 个 target
`deploy/k8s/prometheus-grafana.yaml`：

```yaml
scrape_configs:
  - job_name: agents-im-http-services
    static_configs:
      - targets:
          - user-api.agents-im.svc.cluster.local:8080
          - auth-api.agents-im.svc.cluster.local:8081
          - friends-api.agents-im.svc.cluster.local:8082
          - ...
```

每加一个服务（push、admin-api、agent-rpc、msgtransfer...）就要改这里。

> 修复：改用 k8s service discovery（kubernetes_sd_configs），按 label 自动发现。

### OB-5 ⚠️ 业务 metrics 自实现而不是用 prometheus client_golang
`internal/observability/metrics.go` 自己维护 sample map、自己暴露 text/plain 格式：

```go
MetricMessageSends       = "agents_im_message_sends_total"
MetricDeliveryAttempts   = "agents_im_delivery_attempts_total"
...
```

- 没有 histogram（只有 counter / gauge）；
- 没有 native exemplar 链 trace_id；
- 不能 push gateway。

> 修复：替换为 `github.com/prometheus/client_golang/prometheus`。

### OB-6 ⚠️ `/metrics` 路径分裂
Prometheus config 显示 prometheus 自身 scrape 路径用 `/observability/metrics/metrics`，而其他业务服务用 `/metrics`：

```yaml
- job_name: prometheus
  metrics_path: /observability/metrics/metrics
- job_name: agents-im-http-services
  metrics_path: /metrics
```

业务 metrics 路径在 `internal/observability/metrics.go` 没看到完整 mount 路径，但配置文件这种不一致说明 mounted path 有歧义。

> 修复：所有服务统一暴露在 `/metrics`，prometheus 自己也用 `/metrics`。

### OB-7 ⚠️ Tracing 配置每个服务 yaml 重复
`etc/<service>.yaml` 每个文件都重复：

```yaml
Tracing:
  Enabled: false
  ServiceName: <service-name>
  Environment: local
  Protocol: grpc
  SamplerRatio: 1.0
  TraceUIBaseURL: http://localhost:3000
  Insecure: true
```

14 个 yaml 几乎一模一样。一处改成 OTLP HTTP，其他 13 处忘了改 → 半服务上 Tempo。

> 修复：tracing 配置改 ConfigMap 注入 env var（`AGENTS_IM_TRACING_ENABLED`、`AGENTS_IM_TRACING_OTLP_ENDPOINT` 等），删服务 yaml 里的 Tracing block；`tracing.go.ResolveTracingConfig` 已经支持环境变量优先，把 yaml block 改可选。

### OB-8 ⚠️ Drone deploy.sh 有过多分支
`scripts/ci/drone-deploy.sh` 处理：
- 中间件启动（detect && middleware required）；
- 数据库迁移（migration required）；
- k8s rollout；
- config-only deploy（只 rollout restart，跳过 image）；
- 受影响服务列表通过 detect-deploy-changes.py 计算。

复杂度集中在一个脚本 + 一个 Python 检测器。这个检测器决定了什么变更触发什么部署，**但没有 dry-run 模式**，调试只能 commit 试。

> 修复：拆 `detect-deploy-changes.py` 为可单独 invoke 的 CLI，加 unit test（已经有一些 sample test 文件如 `test-deploy-k3s.sh`、`test-no-latest-images.sh`，但 detector 本身没 test）。

### OB-9 ⚠️ Drone deploy step 通过 mount /etc/rancher/k3s 给容器
```yaml
volumes:
  - name: host_k3s_config
    host:
      path: /etc/rancher/k3s
```

意味着 **drone runner 能拿 k3s admin kubeconfig**。如果 drone runner 容器被攻破，整个 k3s 被接管。

> 修复方向：drone runner 用专门的 ServiceAccount + RoleBinding（agents-im namespace only），不挂 admin kubeconfig。
>
> **决定（2026-05-30）**：不单独做。随 OB-14/15 迁 Argo CD 自然消解——Drone 改为只 build 镜像 + 按 PR+label 改 gitops 仓库，不再持有 kubeconfig；OB-9 风险随之消失。优先级最后。

### OB-10 ⚠️ Drone telegram_bot_token / GHCR_TOKEN 是 drone secret
没问题，但：
- 没有 secret 轮转记录；
- `scripts/ci/notify-telegram-actions.sh` 与 `scripts/ci/drone-telegram-notify.py` 双轨（一个 GHA 一个 Drone）—— 同 OB-1。

### OB-11 ⚠️ Langfuse 接 hostNetwork PG
`deploy/k8s/langfuse.yaml` 把 Langfuse 直接连到 hostNetwork 的 PG（middleware 那台）。Langfuse 的事件量比业务大一个数量级（每次 LLM call 一条），PG 容量/锁竞争风险高。

> 修复：Langfuse 单独跑一个 PG 实例（k8s statefulset），与业务 PG 隔离。

### OB-12 ⚠️ LLM observability sink 同步 HTTP POST
`internal/llmobs/langfuse.go` 是 inline HTTP（10s timeout），没 queue/batch：

```go
const langfuseExportTimeout = 10 * time.Second
```

每条 generation 都 block 上传 → Agent run 延迟随 Langfuse 健康度漂移。

> 修复：sink 改为 channel + 后台 goroutine 批量发送；前台只入队（drop on backpressure + 计数 metric）。

### OB-13 ⚠️ Drone PR pipeline 不跑前端
`verification` pipeline 只跑 `backend-verification` + `postgres-integration`，没跑前端 lint/test/build。`AGENTS.md` 写"web 改动加前端测试/build"，但 PR CI 没有做。

> 修复：加 `frontend-verification` step，按 `detect-deploy-changes.py` 判断 web 受影响才跑。

### OB-14 ⚠️ 部署不分环境
当前只有"main → 生产 k3s"一个环境。没有 staging。意味着 main 合入到部署只有 GitHub Merge Queue 这一道闸门。

> 修复：至少加一个 `develop` 分支 → staging 环境（哪怕同 k3s 不同 namespace）。但这是 P2，不阻塞当前重构。
>
> **决定（2026-05-30）**：不加 `develop` 分支。采用 Argo CD GitOps：拆独立 gitops 仓库供 Argo CD 监控；Drone CI 按 **PR + label** 改 gitops 仓库；Argo CD 经 **webhook** 接收代码仓库变化自动部署。与 OB-15 同一 epic。

### OB-15 🟡 `__IMAGE_TAG_REQUIRED__` placeholder
所有 `deploy/k8s/deployments.yaml` 用 `:__IMAGE_TAG_REQUIRED__`，部署时 deploy-k3s.sh 用 sed 替换 commit SHA。
- 好处：禁止 `:latest`；
- 坏处：阅读 manifest 不知道当前是哪个版本；GitOps 不友好。

> 修复方向（P3）：迁移到 Argo CD / Flux 配合 image updater；当前 placeholder 方案 OK，加注释即可。

### OB-16 🟡 health/ready check 全部相同
所有 deployment 用 `/healthz` 和 `/readyz`，但 `internal/health/` 的 check 集合是 per-service 自定义的（见 `gozero_routes.go` 里每个 service 注册自己的 health components）。问题在于 `/readyz` 是否真的能检测出 PG/Redis/Kafka 故障 → 需要 audit 每个服务的 ready check 是否实际探测了关键依赖。

> 修复：写一个 ready check audit 表格，每个服务列出"探测了哪些依赖"。

### OB-17 🟡 Drone 镜像构建并行度=3
`DRONE_IMAGE_BUILD_PARALLELISM=3` 写在 yaml 里硬编码。当前服务 14 个，并发 3 = 慢；未来加 push、admin-api 后会更慢。

> 修复：动态调整或挪到 secret/env。
>
> **决定（2026-05-30）**：保持并发=3（runner 4 核，再高 context switch / OOM）。Drone OSS 无 GitHub 式 repo 明文变量（只有 secret），而 `scripts/ci/drone-build-images.sh` 已是 `${DRONE_IMAGE_BUILD_PARALLELISM:-3}`——故**删除 `.drone.yml` 两处硬编码**，并发由脚本默认值 3 控制（非 secret、不在 pipeline 硬编码）；如需调整改脚本默认或注入该 env（见 issue #354 / PR）。

---

## 3. 现状架构图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ Single k3s node (hostNetwork) — agents-im namespace                         │
│                                                                              │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐            │
│  │ user-api   │  │ auth-api   │  │ msg-api│  │ msggateway │  ... 8+    │
│  │ :8080      │  │ :8081      │  │ :8083      │  │ :8084      │            │
│  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘            │
│        │ /metrics      │               │               │                    │
│        └──────────┬────┴───────────────┴───────────────┘                    │
│                   ▼                                                          │
│             ┌──────────┐  ┌──────────┐  ┌──────────┐                        │
│             │Prometheus│  │  Grafana │  │  Loki    │                        │
│             └────┬─────┘  └────┬─────┘  └──────────┘                        │
│                  │             │                                            │
│             ┌────▼───┐   ┌────▼─────────┐   ┌──────────┐  ┌──────────┐    │
│             │ Tempo  │◄──│OTel Collector│◄──│ services │  │ Langfuse │    │
│             └────────┘   └──────────────┘   │ (OTLP)   │  │          │    │
│                                              └──────────┘  └──────────┘    │
└──────────────────────┬──────────────────────────────────────────────────────┘
                       │ host loopback
                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  Docker Compose middleware (/opt/agents-im/middleware/.env)                  │
│  PostgreSQL    Redis    Redpanda    MinIO                                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

问题点已在 §2 各条标注。

---

## 4. 目标架构（重构后）

### 4.1 主要变化

1. 中间件入 k8s：PG、Redis、Redpanda、MinIO 都用 statefulset + PVC，删 docker-compose；
2. 业务 deployment 改 `hostNetwork: false`，用 ClusterIP；
3. Loki/Tempo 后端改 MinIO；
4. Prometheus 用 k8s SD；
5. CI 单轨：**Drone**（GHA 已移除）；部署经 Argo CD GitOps（gitops 仓库 + webhook）；
6. LLM observability 异步；
7. metrics 改 prom client_golang。

### 4.2 目标拓扑

```
┌─────────────────────────────────────────────────────────────┐
│ k3s/k8s namespace agents-im                                  │
│                                                              │
│ ┌──── apps ────┐  ┌─── middleware ───┐  ┌── observability ──┐│
│ │ user-api 1.. │  │ postgres-statful │  │ prometheus(SD)   ││
│ │ auth-api 1.. │  │ redis-sts        │  │ grafana          ││
│ │ msg-api 1..N │  │ redpanda-sts     │  │ loki (MinIO bk)  ││
│ │ msg-rpc      │  │ minio-sts        │  │ tempo (MinIO bk) ││
│ │ msggateway N │  └──────────────────┘  │ otel-collector   ││
│ │ msgtransfer │                        │ langfuse (own PG)││
│ │ push         │                        └──────────────────┘│
│ │ agent-api    │                                            │
│ │ agent-rpc    │                                            │
│ │ admin-api    │                                            │
│ └──────────────┘                                            │
└──────────────────────────────────────────────────────────────┘
```

---

## 5. Trace / Log / Metric 三件套规范

### 5.1 Trace
- 所有 HTTP 入口、所有 gRPC interceptor、所有 Kafka producer/consumer、所有 LLM call、所有 tool call、所有 db query（hot path）必须产生 span；
- span 名称统一前缀：`agents_im.<area>.<action>`（如 `agents_im.message.send`、`agents_im.transfer.dispatch`）；
- trace_id 通过 `X-Trace-Id` + `traceparent` 同时下发；
- WebSocket frame 也要带 trace_id（envelope 加字段）。

### 5.2 Log
- 所有日志带 `trace_id`、`request_id`、`user_id`、`conversation_id`；
- 使用 logx（go-zero）结构化日志；
- Loki label：`{service, namespace, level, trace_id_present}` 不要 high-cardinality；
- 错误日志必含 `err.message` 和（可控的）`err.stack`。

### 5.3 Metric
- 切到 prom client_golang；
- 命名规范：
  - counter：`<area>_total{action,result,...}`；
  - histogram：`<area>_duration_seconds`；
- 关键指标：
  - `message_send_total{result}`
  - `message_delivery_attempts_total{status}`
  - `msg_transfer_batch_size`（histogram，对齐 03 §9 Phase 5）
  - `msg_transfer_seq_malloc_duration_seconds`（histogram，00-decisions D2 验证 Redis Malloc 延迟）
  - `push_online_delivered_total{result}`
  - `push_offline_pushed_total{channel,result}`
  - `agent_run_duration_seconds`（histogram）
  - `agent_tool_call_total{tool,result}`
  - `ws_connections_current{instance}`
  - `kafka_consumer_lag{topic,group}`（覆盖 00-decisions D5 全部 topic）

  > 注：原版列过 `message_outbox_pending`，因 00-decisions D1 弃用 outbox 已删除。
- 不要把高基数 ID（user_id、conversation_id、agent_run_id）作 label。

---

## 6. CI 收敛建议

### 6.1 选定 Drone 还是 GHA？
**建议保留 Drone 作为主部署**，因为：
- Drone runner 在 self-hosted 节点上，能直接 kubectl；
- GHA 要把 kubeconfig 作为 secret，安全面更大；
- 项目所有人已熟悉 Drone。

**GHA 已全部移除**（`.github/workflows/` 无 workflow），PR check 与部署均在 Drone。后续部署经 OB-14/15 迁 Argo CD GitOps（Drone 只 build + 改 gitops 仓库，不再 kubectl）。

### 6.2 Drone pipeline 拆分
```
verification (PR)
  ├ backend-verify (gofmt, go vet, go test ./...)
  ├ frontend-verify (npm run lint, build) ← 新增
  ├ postgres-integration
  └ telegram-notify

deploy-main (main push)
  ├ detect-changes
  ├ build-images (parallelism env-driven)
  ├ migrate-db (only if migration_required)
  ├ deploy-k3s
  ├ smoke-test (新增：访问 /healthz、/readyz、ws ping)
  └ telegram-notify
```

新增 **smoke-test** step：deploy 后用真实流量探测，避免"deploy 绿但服务 CrashLoop"。

### 6.3 检测器测试
`scripts/detect-deploy-changes.py` 加 pytest，覆盖：
- 纯 docs 变更 → no deploy；
- web only 变更 → 只 build web image；
- backend only → 只 build 受影响服务；
- migration 文件变更 → migration_required=true；
- config-only → 跳过 image。

---

## 7. 部署收敛建议

### 7.1 中间件入 k8s（最大一块改动）
- PostgreSQL → bitnami/postgresql chart 或 zalando postgres-operator；
- Redis → bitnami/redis；
- Redpanda → redpanda operator（已有 k8s manifest 支持）；
- MinIO → minio-operator；
- 数据迁移：先在 k8s 内拉起新实例，pg_dump/pg_restore；切流；
- 中间件 secrets 改 k8s secret 而非 `/opt/agents-im/middleware/.env`。

### 7.2 hostNetwork → ClusterIP
- 删 deployment `hostNetwork: true`、`dnsPolicy`；
- 删每个服务的硬编码端口（让 k8s 分配 containerPort）；
- ingress 改走 nginx-ingress（已有部分配置）；
- msggateway 的 WebSocket 路径走 ingress（cert-manager 已配）。

### 7.3 多副本支持
- msggateway：**无需** presence 跨实例路由（00-decisions D4：push 用 service discovery 广播到所有 gateway 实例，每个 gateway 查本地连接表）；msggateway 仍要注册 `GatewayService` gRPC server，供 push 调用；
- msg-rpc：写路径无状态（00-decisions D1/D2：只产生 Kafka），可直接多副本；
- msgtransfer：worker_id 用 hostname；consumer group 按 Kafka partition 自动分区；同 conversation 永远落到同一 partition→同一 transfer 实例的同一 worker（D2 单调 seq 的物理基础）；
- push（新）：消费者组多副本；每个副本独立广播给所有 gateway 实例。

### 7.4 Helm or Kustomize?
当前用 kustomize（`deploy/k8s/kustomization.yaml`）。建议：
- 继续 kustomize，但拆 base + overlays（dev/staging/prod）；
- 镜像 tag 通过 kustomize image override 而不是 sed。

---

## 8. 决策与执行路线（2026-05-30 与 owner 确认）

### 8.1 逐条决策

| 编号 | 决策 |
|------|------|
| OB-1 | 只保留 Drone CI；GitHub Actions 已废弃（代码层 `.github/workflows/` 已删），剩文档同步。|
| OB-2 | Loki/Tempo 后端切 MinIO（排在 MinIO 入 k8s 之后）。|
| OB-3 | 中间件入 k8s + 数据迁移；保留一个**长期** k8s 内 PG 只读从库（streaming replication，独立 Service）供 owner 查询；灰度确认后关闭 docker 中间件（Redis / Redpanda / MinIO + 旧主库）。|
| OB-4 | 改用 k8s service discovery。|
| OB-5 | 替换为 `github.com/prometheus/client_golang/prometheus`。|
| OB-6 | 统一暴露在 `/metrics`。|
| OB-7 | tracing 配置改 ConfigMap 注入 env var。|
| OB-8 | `detect-deploy-changes.py` 拆 CLI + 加 test。|
| OB-9 | **不单独做**；随 OB-14/15 迁 Argo CD 自然解决（Drone 不再持 kubeconfig）。优先级最后。|
| OB-10 | telegram 通知单轨（Drone）。|
| OB-11 | Langfuse 独立 PG（k8s statefulset）。|
| OB-12 | LLM observability sink 异步化（channel + 后台 batch）。|
| OB-13 | PR CI 加 `frontend-verification`。|
| OB-14 | 不加 `develop` 分支；Argo CD GitOps：拆独立 gitops 仓库供 Argo CD 监控，Drone 按 PR+label 改 gitops 仓库，Argo CD 经 webhook 自动部署。|
| OB-15 | 迁移到 Argo CD（与 OB-14 同一 epic）。|
| OB-16 | 写 ready check audit 表（每服务列探测了哪些依赖）。|
| OB-17 | 保持并发=3；删 `.drone.yml` 硬编码，由 `drone-build-images.sh` 的 `DRONE_IMAGE_BUILD_PARALLELISM` 默认值 3 控制（Drone 无 repo 明文变量；非 secret）。|

### 8.2 分阶段路线（按风险 / 依赖排序）

| 阶段 | 内容 | 形态 | 风险 |
|------|------|------|------|
| **P0 速赢** | OB-1 文档同步、OB-10 收尾、OB-17 删并发硬编码 | 纯文档 / 配置 PR | 极低 |
| **P1 纯后端重构** | OB-7 · OB-5（+OB-6）· OB-12 · OB-4 · OB-13 · OB-16 · OB-8 | 每条独立 issue→worktree→PR，CI 兜底 | 低 |
| **P2 GitOps/CD** | OB-14/15 引入 Argo CD（**在中间件入 k8s 之前**，先托管现状 manifests）+ 拆 gitops 仓库 + Drone PR+label 改 gitops + webhook；OB-9 在此解决 | 独立 epic | 中高 |
| **P3 中间件入 k8s** | OB-3（含 k8s 长期只读从库 + 数据迁移 + 关 docker）· OB-11 · OB-2 · hostNetwork→ClusterIP；全部走 P2 的 GitOps | 大 epic，需维护窗口，保留旧 docker 回滚 | 高 |

---

## 9. 风险

- **中间件入 k8s** 是高风险操作，需要在低峰期做，准备好回滚（旧 docker-compose 实例不要立即删）。
- **hostNetwork 改 ClusterIP** 之前要先确认所有客户端用的是 k8s service DNS 而不是宿主机 IP。
- **OB-1 CI 切换** 时要并行跑一段时间确认没遗漏 step。
- **Loki/Tempo 切 MinIO** 历史数据会保留在本地磁盘 → 需要单独 export/import 工具。

---

## 10. 文档治理

可观测性与部署的多份文档当前已有分裂（`ARCHITECTURE.md` vs `CLAUDE.md` vs `AGENTS.md` vs `docs/RELIABILITY.md` vs `docs/deployment-k3s-pitfalls.md` vs `deploy/README.md`）。

> 建议：本次重构后，部署只留一份权威文档 `deploy/README.md`；其它文档引用它，不复述细节。`docs/deployment-k3s-pitfalls.md` 是历史 incident 记录，价值高，保留。
