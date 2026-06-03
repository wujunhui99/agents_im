# 05 — 可观测性 / Drone CI / 部署 重构（v2 待办）

> 从 `docs/refactor/v1/05-observability-cicd.md` 迁入的、推迟到 v2 重构才实现的条目。
> 编号沿用 v1 原编号以便追溯。

---

### OB-14 ⚠️ 部署不分环境
当前只有"main → 生产 k3s"一个环境。没有 staging。意味着 main 合入到部署只有 GitHub Merge Queue 这一道闸门。

> 修复：至少加一个 `develop` 分支 → staging 环境（哪怕同 k3s 不同 namespace）。
>
> **决定（2026-05-30，v1 阶段）**：不加 `develop` 分支。采用 Argo CD GitOps：拆独立 gitops 仓库供 Argo CD 监控；Drone CI 按 **PR + label** 改 gitops 仓库；Argo CD 经 **webhook** 接收代码仓库变化自动部署。Argo CD 已接管 prod，但**环境分离（staging/prod）本身推迟到 v2**。
>
> **v2 范围**：在已落地的 Argo CD GitOps 之上引入环境分离——按 namespace（或独立 Argo CD Application）区分 staging / prod，明确各环境的同步触发与晋升（promotion）路径。
