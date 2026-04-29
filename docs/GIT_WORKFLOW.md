# Git Workflow

本文档定义项目的并行开发与合并流程。相比直接从功能分支合并到 `main`，本项目采用 `feature/* -> develop -> main` 的集成模式，并通过 `git worktree` 支持多个 Agent 并行开发。

## 结论

该工作流更适合本项目，原因如下：

- 支持多个 Agent 并行开发，避免所有 Agent 共用同一个工作目录导致互相覆盖。
- `develop` 作为集成分支，可以提前暴露多个 feature 分支之间的冲突。
- `main` 保持稳定，只接收经过集成测试的 `develop`。
- 每个 Agent 的实现、自测、冲突处理和验证结果都可以独立记录。

## 分支模型

- `main`：稳定主分支，只接收已通过集成测试的 `develop`。
- `develop`：集成分支，用于合并多个 feature 分支并解决跨功能冲突。
- `feature/<feature-name>`：v1.0.0 之前的功能分支，功能名使用英文。
- `feature/v1.x.x`：v1.0.0 之后的版本功能分支。

示例：

```text
feature/friend-relationship
feature/websocket-ack
feature/agent-group-chat
feature/v1.1.0
```

## Worktree 并行开发规范

每个 Agent 开发时必须使用独立的 `git worktree` 启动一个新的工作实例，避免多个 Agent 在同一目录修改文件。

推荐目录结构：

```text
/home/ws/project/agents_im/              # 主仓库或协调目录
/home/ws/project/worktrees/
├── friend-relationship/
├── websocket-ack/
└── agent-group-chat/
```

创建 worktree 示例：

```bash
git fetch origin
mkdir -p /home/ws/project/worktrees

git worktree add \
  -b feature/friend-relationship \
  /home/ws/project/worktrees/friend-relationship \
  origin/develop
```

如果远端还没有 `develop`，则先从 `main` 创建：

```bash
git checkout main
git pull origin main
git checkout -b develop
git push -u origin develop
```

## 单个 Agent 的开发流程

1. 从最新 `develop` 创建 feature 分支和 worktree。
2. 阅读 `AGENTS.md`、`ARCHITECTURE.md` 以及相关 `docs/` 文档。
3. 对复杂需求，在 `docs/exec-plans/active/` 创建执行计划。
4. 完成功能实现。
5. 在当前 worktree 内完成自测。
6. 提交代码并推送 feature 分支。
7. 创建 PR/MR：`feature/* -> develop`。
8. CI 通过后，将 feature 分支合并到 `develop`。

## develop 集成流程

`develop` 可能已经包含其他 Agent 合并过的功能，因此 feature 合并前后都需要处理集成风险。

推荐流程：

```bash
git fetch origin
git checkout feature/<feature-name>
git rebase origin/develop
# 或者使用 merge，根据团队偏好决定
```

如果出现冲突，应在 feature 分支或专门的集成 worktree 中解决，并重新运行测试。

合并到 `develop` 后，需要在 `develop` 上执行集成测试：

```bash
git checkout develop
git pull origin develop
# run tests
```

如果多个 feature 合并后才暴露冲突或行为不一致，应在 `develop` 分支上解决冲突并提交修复，然后重新测试。

## develop 合并到 main

只有当 `develop` 满足以下条件时，才允许合并到 `main`：

- 所有目标 feature 已合并到 `develop`。
- 单元测试、集成测试和关键链路测试通过。
- 文档已同步更新。
- 已知高优先级冲突和阻塞问题已解决。
- PR/MR 描述中包含测试结果、风险和回滚方案。

合并路径：

```text
feature/* -> develop -> main
```

禁止普通 feature 分支直接合并到 `main`，除非是紧急 hotfix。

## Hotfix 例外流程

紧急线上修复可从 `main` 拉取 `hotfix/<name>`：

```text
main -> hotfix/<name> -> main
```

修复合并到 `main` 后，必须同步回 `develop`：

```bash
git checkout develop
git pull origin develop
git merge origin/main
# run tests
git push origin develop
```

## Agent 合并前检查清单

- [ ] 当前 worktree 只服务于一个 feature 分支。
- [ ] 已从最新 `develop` 创建或同步分支。
- [ ] 已完成必要的需求文档、设计文档或执行计划更新。
- [ ] 已完成自测并记录测试命令和结果。
- [ ] 已检查与其他已合并 feature 的冲突。
- [ ] PR/MR 目标分支是 `develop`，不是 `main`。

## main 发布前检查清单

- [ ] `develop` 已包含本次发布目标功能。
- [ ] `develop` 已通过完整测试。
- [ ] 已检查数据库迁移、配置变更和兼容性风险。
- [ ] 已更新质量、安全、可靠性相关文档。
- [ ] 已准备回滚方案。
- [ ] `develop -> main` 的 PR/MR 已通过评审和 CI。
