# Git Workflow

本文档定义项目的并行开发与合并流程。本项目采用 `feature/fix/ci/... -> main` 的 GitHub Merge Queue 模式，并通过 `git worktree` 支持多个 Agent 并行开发。

多 Agent 分支、commit message、Git identity 与 CI/CD 归因的详细规范见 [`docs/AGENT_GIT_STANDARD.md`](./AGENT_GIT_STANDARD.md)。CI 会强制校验任务分支格式：`<type>/<agent-name>/<issue>-<task-desc>`，其中第二段必须是可信 Agent 名。

## 结论

该工作流更适合本项目，原因如下：

- 支持多个 Agent 并行开发，避免所有 Agent 共用同一个工作目录导致互相覆盖。
- 取消日常 `develop` 集成，减少双阶段 promotion 和额外等待。
- `main` 通过 GitHub Merge Queue 保持稳定：PR 先通过常规 CI，再由队列验证并自动合并。
- 每个 Agent 的实现、自测、冲突处理和验证结果都可以独立记录，并可从 feature 分支 / commit metadata 溯源。

## 分支模型

- `main`：唯一长期主分支和发布分支。只允许 GitHub Merge Queue 合并进入；禁止直接 push 或绕过队列 merge。
- `feature/<agent-name>/<issue>-<task-desc>` / `fix/<agent-name>/<issue>-<task-desc>` / `ci/<agent-name>/<issue>-<task-desc>` 等：日常任务分支。完整规则见 [`docs/AGENT_GIT_STANDARD.md`](./AGENT_GIT_STANDARD.md)。
- `feature/v1.x.x`：v1.0.0 之后的版本功能分支。

示例：

```text
feature/eino/issue-131-admin-console-lists
fix/helios/issue-128-drone-postgres-url
ci/hermes/issue-142-drone-notifier-routing
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
  -b feature/eino/issue-131-admin-console-lists \
  /home/ws/project/worktrees/eino-issue-131-admin-console-lists \
  origin/main
```

### 本地开发布局（junhui Mac）

主仓库根在 `/Users/junhui/code/project/agents_im`（VSCode 打开 / 开发工作目录就是这里），独立上游仓库 open-im-server 与之平级。worktree 遵循 **Claude Code 默认**，放在主仓库内的 `.claude/worktrees/`（已在 `.gitignore` 忽略，不会被跟踪）：

```text
/Users/junhui/code/project/
├── agents_im/                    # 主仓库（VSCode 打开 / 工作目录）
│   └── .claude/worktrees/        # worktree 放这里（Claude Code 默认）
└── open-im-server/               # 独立上游仓库
```

约定：worktree 跟随 Claude Code 默认，统一放在 `<仓库>/.claude/worktrees/`，即 `/Users/junhui/code/project/agents_im/.claude/worktrees/`。手动创建示例：

```bash
git -C /Users/junhui/code/project/agents_im worktree add \
  -b fix/claude/issue-N-task-desc \
  /Users/junhui/code/project/agents_im/.claude/worktrees/issue-N-task-desc \
  origin/main
```

> 该路径已在 `.gitignore` 忽略；切勿用 `git add` 把 worktree 目录提交成 gitlink。


## 单个 Agent 的开发流程

1. 从最新 `origin/main` 创建任务分支和 worktree。
2. 阅读 `AGENTS.md`、`ARCHITECTURE.md` 以及相关 `docs/` 文档。
3. 对复杂需求，在 GitHub Issue 上创建/认领任务，以 Issue 作为需求与验收标准的单一事实源（见 [`docs/AGENTIC_DEVELOPMENT_WORKFLOW.md`](./AGENTIC_DEVELOPMENT_WORKFLOW.md)）。
4. 完成功能实现。
5. 在当前 worktree 内完成自测。
6. 提交代码并推送 feature 分支。
7. 创建 PR：`feature/fix/ci/... -> main`。
8. 常规 CI 通过、Hermes/Eino 验收后，将 PR 加入 GitHub Merge Queue。
9. Merge Queue 验证通过后自动合并到 `main`。
10. `main` push 触发 `deploy-main`，Telegram 通知按 feature agent 溯源 @ 对应 bot。

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

- 修改已有 `db/migrations/*.sql`：禁止；已发布 migration 不可变，必须新增下一号 migration。`scripts/ci/verify-migration-immutability.sh` 会阻止 PR 修改、删除、重命名或 type-change 历史 migration。
- 新增 `db/migrations/*.sql`：允许；仍必须通过 PostgreSQL integration，从空库执行全量 migration，并在 deploy 时只应用生产库尚未记录的新 migration。
- 修改 `db/schema/`、repository SQL 或 PG integration test：需要配套新增 migration 或明确说明不涉及生产 schema。
- SQL 不得包含 secret、DSN、密码、token、server 连接信息。
- `scripts/verify-static.sh` 会在检测到 `db/migrations/`、`db/schema/`、`internal/repository/postgres_*.go` 或 PG integration test 改动时，要求存在非模板 `db/change_log/*.sql`；同时禁止修改/删除/重命名已有 `db/migrations/*.sql`。

## CI Checks

CI 是 PR 进入 GitHub Merge Queue 和合并到 `main` 的质量门禁；CD 只从 `main` / `devops` 等发布分支运行。当前仓库主要使用 Drone：

- Drone 地址：<https://drone.agenticim.xyz>
- Drone 仓库：`wujunhui99/agents_im`
- Pipeline 定义：`.drone.yml`

不要只看 GitHub 上的红叉；失败时必须点进 Drone build，再打开失败的 pipeline / step，看日志尾部和具体错误。

### PR 阶段怎么看 CI

每次 PR 或 push 后，Drone 会生成 build。普通 PR 重点看这两个 pipeline：

- `backend-verification`
  - 后端基础验证。
  - 包括 Go 格式检查、go-zero API 校验、Go 测试、静态验证、Compose/Markdown 等基础检查。
- `postgres-integration`
  - PostgreSQL 集成验证。
  - 会启动 PostgreSQL service，执行 migration，再跑 integration tests。

普通 PR 主要关注 `backend-verification` 和 `postgres-integration` 是否都是 `success`。如果这两个都绿，说明 PR CI 基本通过；进入 Merge Queue 后仍必须以 Merge Queue 的合并组验证结果为最终合并门禁。

### 当前 CI 内容

`backend-verification` 主要覆盖：

- `bash scripts/ci/verify-agent-branch-name.sh`，硬性校验 PR source branch 为 `<type>/<agent-name>/issue-<number>-<task-desc>`，且第二段是可信 Agent 名。
- `bash scripts/ci/verify-pr-issue-link.sh`，硬性校验 PR body 包含且只包含一个 closing issue keyword，例如 `Closes #152`。
- 安装固定版本 Go/go-zero/protobuf 工具。
- `goctl api validate -api api/*.api` 验证 go-zero API spec。
- `gofmt` check，发现未格式化 Go 文件即失败。
- `go test ./...` 运行普通 Go 测试；默认不设置 PostgreSQL DSN，确保普通测试不依赖真实 PG。
- `bash scripts/verify-static.sh`，检查仓库关键文件、接口、文档、Drone workflow 约束，并调用 `scripts/ci/verify-migration-immutability.sh` 禁止 PR 修改历史 migration。
- `scripts/ci/drone-telegram-notify.py` 在 success / failure 都发送 Telegram 通知；开发 PR 和 `main` push 从分支第二段、main merge source branch、subject `[agent]`、`Agent:` trailer、author email 解析负责 Agent，在群里 @ 对应 bot；归因冲突会在通知中展示 warning。`devops` push 是 CI/CD lane，固定 @ Eino。
- `docker compose config`，验证 Compose 配置可解析。
- Markdown link check，排除 `docs/references/` 和 `.ai-context/`，并忽略外部 HTTP/HTTPS 链接波动。

`postgres-integration` 使用 Drone `postgres:16-alpine` service，设置 `DATABASE_URL` 指向该隔离 service，执行 `bash scripts/migrate-postgres.sh --host-psql` 后运行：

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

部署只在 `main` / `devops` 等发布分支 push 时运行，普通 PR 不跑部署。部署 pipeline 是 `deploy-main`，它会：

`main` push 的 Drone 通知会根据 Merge Queue 合入的 feature/fix/ci 分支和 commit metadata 溯源到功能 Agent，并直接 @ 对应 bot；`devops` push 仍固定归 Eino（`@eino_hermes_bot`），因为它是 CI/CD lane。

1. 检测是否需要构建镜像。
2. 构建并推送镜像。
3. 部署到 k3s 服务器。
4. 等待 rollout。

因此日常判断顺序是：

- PR 阶段：看 `backend-verification` + `postgres-integration`。
- Merge Queue 阶段：看 merge group / queued PR 对应验证。
- 合并到 `main` 后：再看 `deploy-main`。

失败时按失败 pipeline/step 定位：

- Go test 失败：看具体 package 和 test name。
- migration / DB 失败：看 `postgres-integration`。
- 镜像构建失败：看 build images 相关 step。
- 生产部署失败：看 deploy step。

PR CI 已做并行优化，正常情况下验证应在 2 分 40 秒以内；当前实测通常是几十秒级。明显超过该时间需要报告并排查。

生产发布由 `.drone.yml` 中的 `deploy-main` pipeline 负责。发布链路为：

1. `detect changes` step 先判断本次变更类型，并输出 `build_required`、`deploy_required`、`config_only`、`backend_services`、`web_required`、`image_services` 和 `rollout_services`：
   - `main` push：按变更范围选择性构建/部署。
   - 非 `main` 分支：不进入 deploy pipeline。
   - `deploy/k8s/**`、`etc/<service>.yaml`、`scripts/deploy-k3s.sh`、`.drone.yml`、`scripts/ci/**`：config-only deploy，不构建镜像。
   - `docs/**`、`README.md`、其他 Markdown：不部署。
   - `web/**`：只构建和部署 `web`。
   - `service/<domain>/api/**`、`service/<domain>/rpc/**`、`service/<name>/**`（gateway-ws/message-api/message-transfer）、`internal/rpcgen/message/**`、`api/<domain>.api`：只构建和部署对应服务。
   - `proto/**`、`go.mod`、`go.sum`、`Dockerfile`、`.dockerignore`、`internal/**`、`db/**`、`scripts/migrate-postgres.sh`：构建并部署全部后端服务；只有同时修改 web-owned 路径时才构建 `web`。
   - 其他非文档文件：fail-safe 为全部后端服务，避免漏构建。
2. `build images` step 在 `image_services` 非空时构建并推送后端/web 镜像到 GHCR；后端镜像使用 Dockerfile `backend` target 和 `SERVICE=<service>` build arg。
3. `deploy` step 使用 Drone `deploy_ssh_*` secrets 通过 SSH 连接服务器，将仓库部署文件同步到 `/opt/agents-im/repo`，并以当前 commit SHA 作为 `IMAGE_TAG` 执行 `scripts/deploy-k3s.sh`。选择性发布会传入 `IMAGE_SERVICES`，只对已构建服务执行 `kubectl set image`，并只等待受影响服务 rollout。

生产拓扑采用混合单机部署：

- k3s 管理应用工作负载：Go API、RPC、worker 和 web UI。
- Docker Compose 管理中间件：PostgreSQL、Redis、Redpanda、MinIO。
- `scripts/deploy-k3s.sh` 会启动服务器上的中间件 Compose、从 k3s `agents-im-secrets` 读取 `DATABASE_URL` 执行 PostgreSQL migration、刷新 GHCR pull secret，再应用已渲染且保留不可变镜像 tag 的 `deploy/k8s` manifests。选择性镜像发布会向脚本传入 `IMAGE_SERVICES=<services>`，脚本只对已构建服务使用当前 SHA；未选择服务会保留当前已部署镜像 tag，避免被 manifest 占位 tag 或历史 mutable tag 回退。config-only deploy 会向脚本传入 `SKIP_SET_IMAGE=true`、`SKIP_MIDDLEWARE=true`、`SKIP_MIGRATIONS=true`、`RESTART_ROLLOUT=true`、`ROLLOUT_SERVICES=<services>` 和 `RESTART_SERVICES=<services>`，用于跳过镜像更新/中间件/迁移，只重启并等待受影响 deployment。
- 首次服务器初始化使用 `scripts/bootstrap-server.sh`，它会写入 `/opt/agents-im/middleware/.env`，启动中间件，并创建 k3s `agents-im-secrets`。真实 secret 只应保存在服务器/k3s 或 Drone secrets，不提交到 Git，也不打印到 CI 日志。

发布 workflow 需要的 Drone repository secrets 见 [`../deploy/README.md`](../deploy/README.md)。

- 代码合入 `develop` 后，该 PR 对应的 GitHub Issue 视为 completed；Controller / CI bot 应评论完成摘要并关闭 Issue。
- `main` 发布失败不重新打开已完成开发 Issue；应新建 release/deploy issue 追踪。

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

- [ ] 分支名符合 `docs/AGENT_GIT_STANDARD.md` 中的 `<type>/<agent-name>/<issue>-<task-desc>`，且第二段 `<agent-name>` 是可信 Agent 名。
- [ ] commit 使用当前 Agent 专用 Git identity，subject 包含 `[agent-name]`，并包含 `Issue` / `Agent` / `Human-Owner` trailers。
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
