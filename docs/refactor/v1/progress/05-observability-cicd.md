# 05 — 可观测性 / CI / 部署 重构完成情况

> 追踪 [`../05-observability-cicd.md`](../05-observability-cicd.md) 的 OB-1..OB-17 落地进度。
> 决策与分阶段路线见该分析文档 §8。状态图例见 [`README.md`](./README.md)。

## 阶段总览

| 阶段 | 范围 | 状态 |
|------|------|------|
| **P0 速赢** | OB-1 · OB-10 · OB-17 | ✅ 完成（2026-05-30） |
| **P1 纯后端重构** | OB-7 · OB-5(+OB-6) · OB-12 · OB-4 · OB-13 · OB-16 · OB-8 | 🟡 进行中 |
| **P2 GitOps/CD** | OB-14/15（+OB-9） | ⬜ 未开始 |
| **P3 中间件入 k8s** | OB-3 · OB-11 · OB-2 · hostNetwork→ClusterIP | ⬜ 未开始 |

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
| OB-16 | ready check audit 表（每服务列探测依赖） | P1 | ⬜ | — | — |
| OB-8 | `detect-deploy-changes` 拆 CLI + 加 test | P1 | ⬜ | — | — |
| OB-14 | Argo CD GitOps：拆 gitops 仓库 + Drone PR+label 改 gitops + webhook | P2 | ⬜ | — | — |
| OB-15 | 迁移到 Argo CD（同 OB-14 epic） | P2 | ⬜ | — | — |
| OB-9 | Drone runner 不挂 admin kubeconfig（随 OB-14/15 消解） | P2 | ⬜ | — | — |
| OB-3 | 中间件入 k8s + 数据迁移 + 长期 PG 只读从库 + 关 docker | P3 | ⬜ | — | — |
| OB-11 | Langfuse 独立 PG | P3 | ⬜ | — | — |
| OB-2 | Loki/Tempo 后端切 MinIO | P3 | ⬜ | — | — |
| OB-— | hostNetwork → ClusterIP | P3 | ⬜ | — | — |

## 备注

- **P0 实现要点**：GHA 已在代码层废弃，OB-1 仅文档同步；Drone OSS 无 GitHub 式 repo 明文变量，OB-17 删硬编码后由 `scripts/ci/drone-build-images.sh` 的 `${DRONE_IMAGE_BUILD_PARALLELISM:-3}` 默认值控制。
- **CI 核验**：Drone 不向 GitHub PR 回报状态检查，需在 Drone UI（`https://drone.agenticim.xyz`）核验；项目norm 为 PR → 立即 merge → Drone 构建（merge 后）。
