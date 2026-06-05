# Refactor v2 — 待办 backlog

> v1（`docs/refactor/v1/`）聚焦 monolith 退役、目录结构收敛、观测/CI/部署技术债。
> v2 收纳从 v1 **显式推迟**到下一轮重构才实现的需求——v1 阶段不阻塞、不动工，留待 v2 统一规划落地。

## 文档索引

| 文档 | 内容 |
|------|------|
| [`01-third-service-naming.md`](./01-third-service-naming.md) | third 服务命名对齐：`mailservice` → `thirdclient`（#429 推迟）|
| [`05-observability-cicd.md`](./05-observability-cicd.md) | 部署环境分离（OB-14，从 v1 迁入）|

## 收录规则

- 某条 v1 技术债的修复被判定为「下一轮再做」→ 把该条目从 v1 对应文档移到此处，v1 留一行指针。
- 每条目保留原始编号（如 OB-14）以便追溯。
