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

## Agentic GitHub Project / Issues 工作流

产品功能、复杂 Bug、重构、Research、E2E/Regression 任务必须以 GitHub Issue / Project 作为需求和调度的单一事实源。详细规则见 [`docs/AGENTIC_DEVELOPMENT_WORKFLOW.md`](./AGENTIC_DEVELOPMENT_WORKFLOW.md)。

核心门禁：

- 用户描述产品需求后，默认先进入 Codex Spec Mode / Hermes Spec Writer，创建或更新 Issue，不直接启动 Codex Dev Mode 写代码。
- `Spec Ready` 只表示规格草稿完成；Hermes 检查 Spec Gate 后才能设置 `Ready for Dev`。
- Codex Dev Mode 只能执行 `Ready for Dev` Issue，并必须读取 Issue body、验收标准、测试计划和依赖。
- 普通产品需求默认一个全栈 Issue 完成，不拆成前端/后端/测试微任务；只有 Research、过大需求、低耦合独立能力或系统级 E2E 才拆分。
- PR merge 不等于 Done；需要 Hermes 验收、必要 CI/CD 和生产 smoke/E2E 后才能关闭 Issue。


## Codex commit 前验证门禁

Codex Dev Mode 在提交 commit 前必须按改动范围运行验证，并把命令、结果、branch、commit、PR 和 blocker 写回 Issue/PR。`codex exec` 退出 0 只表示 worker 完成，不等于 Hermes 验收通过。

最低门禁：

```bash
gofmt -w $(find . -name "*.go" -print)
git diff --check
go test ./...
bash scripts/verify-static.sh
```

如果改动 `web/`，还必须运行：

```bash
npm --prefix web test -- --run
npm --prefix web run build
```

如果改动数据库 schema、`db/migrations/*.sql`、repository SQL、或 PostgreSQL integration tests，必须运行 PostgreSQL integration：

```bash
AGENTS_IM_CONFIRM_TRUNCATE=1 scripts/verify-postgres-local.sh
```

脚本读取 `DATABASE_URL` 或 `AGENTS_IM_POSTGRES_DSN`，只允许专用本机/测试 PostgreSQL；integration test 可能 truncate 测试表。无法运行某项验证时，必须报告 blocker，不能假装通过。

## 数据库 change_log 门禁

只有数据库 schema/data migration 行为变化时才需要新增 change log；纯应用代码、前端、普通文档不需要。

要求：

- 新增或更新 `db/change_log/*.sql`，且不能只提交 `template.sql`。
- `.sql` 是实际执行数据库改动的事实源，必须能用 `psql -v ON_ERROR_STOP=1 -f <file>.sql` 执行。
- 建议配对 `.md` 说明目的、影响表/字段、是否破坏性、apply 顺序、rollback/恢复和验证命令。
- SQL 不得包含 secret、DSN、密码、token、server 连接信息。
- `scripts/verify-static.sh` 会在检测到 `db/migrations/`、`db/schema/`、`internal/repository/postgres_*.go` 或 PG integration test 改动时，要求存在非模板 `db/change_log/*.sql`。

## CI Checks

CI 是 feature 分支合入 `develop` 的质量门禁；CD 只从 `main` 发布。当前仓库使用 Drone，pipeline 位于 `.drone.yml`；PR/MR 合入 `develop` 前必须通过默认 `verification` pipeline。为避免同一个 MR 同时触发多轮重复 verification，常规 verification pipeline 只响应目标分支为 `develop` 或 `main` 的 `pull_request` 事件；分支 push 不再触发 verification，且 backend 与 PostgreSQL integration 合并在同一个 Drone pipeline 内顺序执行，因此每个 MR 只有一个 CI task/context。当前 CI checks 包括：

- `verification` pipeline 的 `backend-verification` step 安装固定版本 Go/go-zero/protobuf 工具。
- `goctl api validate -api api/*.api` 验证 go-zero API spec。
- `gofmt` check，发现未格式化 Go 文件即失败。
- `go test ./...` 运行普通 Go 测试；默认不设置 PostgreSQL DSN，确保普通测试不依赖真实 PG。
- `bash scripts/verify-static.sh`，检查仓库关键文件、接口、文档和 Drone workflow 约束。
- `docker compose config`，验证 Compose 配置可解析。
- Markdown link check，排除 `docs/references/` 和 `.ai-context/`，并忽略外部 HTTP/HTTPS 链接波动。

CI 还包含同一 `verification` pipeline 内的 PostgreSQL integration step。该 step 使用 Drone `postgres:16-alpine` service，设置 `DATABASE_URL` 指向该隔离 service，执行 `bash scripts/migrate-postgres.sh --host-psql` 后运行：

```bash
go test -tags=integration ./tests
```

本地复现默认 backend verification：

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
gofmt -l $(git ls-files '*.go')
go test ./...
bash scripts/verify-static.sh
docker compose config
npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json $(find . -name "*.md" -not -path "./.git/*" -not -path "./.ai-context/*" -not -path "./docs/references/*" -print)
```

如需本地复现 PostgreSQL integration job，先启动或准备专用本机/测试 PostgreSQL，再运行：

```bash
export DATABASE_URL='postgres://agents_im:[REDACTED]@localhost:5432/agents_im_test?sslmode=disable'
AGENTS_IM_CONFIRM_TRUNCATE=1 scripts/verify-postgres-local.sh
```

`verify-postgres-local.sh` 会先执行 `bash scripts/migrate-postgres.sh --host-psql`，再执行 `go test -tags=integration ./tests -count=1`。

## CD / Deployment

生产发布由 `.drone.yml` 中的 `deploy-main` pipeline 负责。该 pipeline 只在 `main` 分支 push 后自动触发。发布链路为：

1. `detect changes` step 先判断本次变更类型，并输出 `build_required`、`deploy_required`、`config_only`、`backend_services`、`web_required`、`image_services` 和 `rollout_services`：
   - `main` push：按变更范围选择性构建/部署。
   - 非 `main` 分支：不进入 deploy pipeline。
   - `deploy/k8s/**`、`etc/<service>.yaml`、`scripts/deploy-k3s.sh`、`.drone.yml`、`scripts/ci/**`：config-only deploy，不构建镜像。
   - `docs/**`、`README.md`、其他 Markdown：不部署。
   - `web/**`：只构建和部署 `web`。
   - `cmd/<service>/**`、`api/<domain>.api`：只构建和部署对应服务。
   - `proto/**`、`go.mod`、`go.sum`、`Dockerfile`、`.dockerignore`、`internal/**`、`db/**`、`scripts/migrate-postgres.sh`：构建并部署全部后端服务；只有同时修改 web-owned 路径时才构建 `web`。
   - 其他非文档文件：fail-safe 为全部后端服务，避免漏构建。
2. `build images` step 在 `image_services` 非空时构建并推送后端/web 镜像到 GHCR；后端镜像使用 Dockerfile `backend` target 和 `SERVICE=<service>` build arg。
3. `deploy` step 使用 Drone `deploy_ssh_*` secrets 通过 SSH 连接服务器，将仓库部署文件同步到 `/opt/agents-im/repo`，并以当前 commit SHA 作为 `IMAGE_TAG` 执行 `scripts/deploy-k3s.sh`。选择性发布会传入 `IMAGE_SERVICES`，只对已构建服务执行 `kubectl set image`，并只等待受影响服务 rollout。

生产拓扑采用混合单机部署：

- k3s 管理应用工作负载：Go API、RPC、worker 和 web UI。
- Docker Compose 管理中间件：PostgreSQL、Redis、Redpanda、MinIO。
- `scripts/deploy-k3s.sh` 会启动服务器上的中间件 Compose、从 k3s `agents-im-secrets` 读取 `DATABASE_URL` 执行 PostgreSQL migration、刷新 GHCR pull secret，再 `kubectl apply -k deploy/k8s` 并等待 deployment rollout。选择性镜像发布会向脚本传入 `IMAGE_SERVICES=<services>`，脚本会先记录当前 deployment 镜像，apply manifests 后仅把已构建服务切到当前 SHA，并把未选择服务恢复到 apply 前镜像，避免 web-only deploy 把后端/RPC 回退到 manifest 里的 `:latest`。config-only deploy 会向脚本传入 `SKIP_SET_IMAGE=true`、`SKIP_MIDDLEWARE=true`、`SKIP_MIGRATIONS=true`、`RESTART_ROLLOUT=true`、`ROLLOUT_SERVICES=<services>` 和 `RESTART_SERVICES=<services>`，用于跳过镜像更新/中间件/迁移，只重启并等待受影响 deployment。
- 首次服务器初始化使用 `scripts/bootstrap-server.sh`，它会写入 `/opt/agents-im/middleware/.env`，启动中间件，并创建 k3s `agents-im-secrets`。真实 secret 只应保存在服务器/k3s 或 Drone secrets，不提交到 Git，也不打印到 CI 日志。

发布 workflow 需要的 Drone repository secrets 见 [`../deploy/README.md`](../deploy/README.md)。

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
