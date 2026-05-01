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

## CI Checks

CI 是 feature 分支合入 `develop` 的质量门禁；CD 只从 `main` 发布。GitHub Actions workflow 位于 `.github/workflows/ci.yml`，PR/MR 合入 `develop` 前必须通过默认 backend verification。当前 CI checks 包括：

- `actions/checkout` 拉取仓库代码。
- `actions/setup-go` 按 `go.mod` 配置 Go。
- 安装 `goctl`、`protoc`、`protoc-gen-go`、`protoc-gen-go-grpc`。
- `goctl api validate -api api/*.api` 验证 go-zero API 契约。
- `gofmt` check，发现未格式化 Go 文件即失败。
- `go test ./...`，默认不设置 PostgreSQL DSN，确保普通测试不依赖真实 PG。
- `bash scripts/verify-static.sh`，检查仓库关键文件、接口、文档和 CI workflow 约束。
- `docker compose config`，验证 Compose 配置可解析。
- Markdown link check，排除 `docs/references/` 和 `.ai-context/`，并忽略外部 HTTP/HTTPS 链接波动。

CI 还包含独立 PostgreSQL integration job。该 job 使用 GitHub Actions `postgres:16-alpine` service，设置 `DATABASE_URL`，执行 `bash scripts/migrate-postgres.sh --host-psql` 后运行：

```bash
go test -tags=integration ./tests
```

本地复现默认 backend verification：

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name "*.go" -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json $(find . -name "*.md" -not -path "./.git/*" -not -path "./.ai-context/*" -not -path "./docs/references/*" -print)
```

如需本地复现 PostgreSQL integration job，先启动或准备 PostgreSQL，再运行：

```bash
export DATABASE_URL=postgres://agents_im:***@localhost:5432/agents_im?sslmode=disable
bash scripts/migrate-postgres.sh --host-psql
go test -tags=integration ./tests
```

## CD / Deployment

生产发布由 `.github/workflows/deploy.yml` 负责。该 workflow 在 `main` 分支 push 后自动触发，也可通过 GitHub Actions `workflow_dispatch` 手动触发。所有 build/deploy job 都带有 `github.ref == 'refs/heads/main'` 防线；如果手动触发选择了非 `main` ref，workflow 只会 no-op，不会 SSH 到服务器。发布链路为：

1. `detect-changes` job 先判断本次变更类型，并输出 `build_required`、`deploy_required`、`config_only`、`backend_services`、`web_required`、`image_services` 和 `rollout_services`：
   - `workflow_dispatch` on `main`：完整构建和部署，保持手动发布语义。
   - `workflow_dispatch` on non-`main`：no-op，禁止部署。
   - `deploy/k8s/**`、`etc/<service>.yaml`、`scripts/deploy-k3s.sh`、`.github/workflows/deploy.yml`：config-only deploy，不构建镜像。
   - `docs/**`、`README.md`、其他 Markdown：不部署。
   - `web/**`：只构建和部署 `web`。
   - `cmd/<service>/**`、`api/<domain>.api`：只构建和部署对应服务。
   - `proto/**`、`go.mod`、`go.sum`、`Dockerfile`、`.dockerignore`、`internal/**`、`db/**`、`scripts/migrate-postgres.sh`：构建并部署全部后端服务；只有同时修改 web-owned 路径时才构建 `web`。
   - 其他非文档文件：fail-safe 为全部后端服务，避免漏构建。
2. `build-backend` job 在 `backend_services` 非空时按动态服务矩阵构建并推送后端镜像到 GHCR，服务包括 `user-api`、`auth-api`、`friends-api`、`message-api`、`gateway-ws`、`groups-api`、`agent-api`、`message-transfer` 以及各 RPC 服务。
3. `build-web` job 仅在 `web_required=true` 时构建并推送 web 镜像到 GHCR。
4. `deploy` job 使用 `SERVER_*` secrets 通过 SSH 连接服务器，将仓库部署文件同步到 `/opt/agents-im/repo`，并以当前 commit SHA 作为 `IMAGE_TAG` 执行 `scripts/deploy-k3s.sh`。选择性发布会传入 `IMAGE_SERVICES`，只对已构建服务执行 `kubectl set image`，并只等待受影响服务 rollout。

生产拓扑采用混合单机部署：

- k3s 管理应用工作负载：Go API、RPC、worker 和 web UI。
- Docker Compose 管理中间件：PostgreSQL、Redis、Redpanda。
- `scripts/deploy-k3s.sh` 会启动服务器上的中间件 Compose、从 k3s `agents-im-secrets` 读取 `DATABASE_URL` 执行迁移、刷新 `ghcr-pull-secret`，再 `kubectl apply -k deploy/k8s` 并等待 deployment rollout。选择性镜像发布会向脚本传入 `IMAGE_SERVICES=<services>`，避免未构建服务被设置到不存在的 `${GITHUB_SHA}` tag。config-only deploy 会向脚本传入 `SKIP_SET_IMAGE=true`、`SKIP_MIDDLEWARE=true`、`SKIP_MIGRATIONS=true`、`RESTART_ROLLOUT=true`、`ROLLOUT_SERVICES=<services>` 和 `RESTART_SERVICES=<services>`，用于跳过镜像更新/中间件/迁移，只重启并等待受影响 deployment。
- 首次服务器初始化使用 `scripts/bootstrap-server.sh`，它会写入 `/opt/agents-im/middleware/.env`，启动中间件，并创建 k3s `agents-im-secrets`。真实 secret 只应保存在服务器/k3s，不提交到 Git，也不打印到 Actions 日志。

发布 workflow 需要的 GitHub repository secrets 见 [`../deploy/README.md`](../deploy/README.md)。

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
