> 注：MinIO 已于 #569 退役为 RustFS（S3 兼容）；下文 MinIO 指当时的对象存储组件，按时点记录保留、不改写。

# 重构完成情况追踪（refactor/v1）

本目录记录 `docs/refactor/v1/` 各分析文档的**落地进度**。分析文档（`00`–`08`）只描述技术债与建议，本目录的追踪文档记录「每条建议是否已实现、由哪个 PR/issue 完成、何时完成」。

## 约定

- 每个分析文档对应一份同名追踪文档（如 `05-observability-cicd.md`）。
- 状态图例：✅ 完成 · 🟡 进行中 · ⬜ 未开始 · ⏭️ 不做/已废弃。
- 每次推进相关 PR 时，**同一 PR 内**更新对应追踪文档（与自我进化规范一致）。

## 追踪文档索引

| 分析文档 | 追踪文档 | 状态 |
|----------|----------|------|
| [`01-project-structure.md`](../01-project-structure.md) / [`02-microservices.md`](../02-microservices.md) | [`02-microservices.md`](./02-microservices.md) | 🟡 groups-rpc 已脱 internal（goctl model + BFF 聚合，PR #415）；其余域待续（见追踪文档 §复刻 Playbook） |
| [`05-observability-cicd.md`](../05-observability-cicd.md) | [`05-observability-cicd.md`](./05-observability-cicd.md) | 🟡 P0/P1 完成；P2 Argo CD 已上线；P3 中间件 Redis+MinIO 已迁，PG/Redpanda 待续（见追踪文档 §交接） |
| 其余 `00`、`03`–`04`、`06`–`08` | 待按需创建 | ⬜ |
