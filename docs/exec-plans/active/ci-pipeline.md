# CI Pipeline

状态：Active

## 背景

Redis presence 和 WebSocket Gateway server 正在其他分支并行推进。当前分支只建立仓库级 CI 护栏，确保后续并行开发在合入 `develop` 前至少通过 go-zero API、Go 格式、单元测试、静态约束和 Compose 配置检查。

## 目标

- 新增 GitHub Actions CI workflow。
- 默认 CI 不依赖真实 PostgreSQL，保持 `go test ./...` 可在无数据库环境运行。
- 独立 PostgreSQL integration job 使用 service postgres，先执行 migration，再运行 `integration` build tag 测试。
- 增加 markdown link check，排除 `docs/references/` 和 `.ai-context/`。
- 将 CI checks 和本地复现命令写入 Git 工作流文档。
- 扩展 `scripts/verify-static.sh`，让静态检查能守住 CI workflow 与文档存在性、关键命令覆盖。

## 非目标

- 不实现 Redis presence 业务代码。
- 不实现 WebSocket Gateway server。
- 不调整 main/develop 分支或合并其他分支。
- 不提交 secret/token；CI 仅使用本地开发 PostgreSQL service 凭据。

## 任务拆分

- [x] Task 1：新增 `.github/workflows/ci.yml`，覆盖 backend 默认验证和 PostgreSQL integration job。
- [x] Task 2：新增 markdown link check 配置，排除大体量参考目录和 `.ai-context/`。
- [x] Task 3：更新 `scripts/verify-static.sh`，检查 CI 文件、计划文档和 workflow 关键命令。
- [x] Task 4：更新 `docs/GIT_WORKFLOW.md`，说明 PR CI checks 与本地复现命令。
- [x] Task 5：执行强制验证并记录结果。
- [x] Task 6：提交并推送 `feature/ci-pipeline`。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 默认 backend job 只运行 `go test ./...`，不设置 `DATABASE_URL` | 保持普通测试不依赖真实 PostgreSQL，符合现有 build tag 设计 |
| 2026-04-29 | PostgreSQL integration 使用独立 job 和 GitHub Actions service postgres | 能验证 migration 与持久化测试，同时不阻塞默认无数据库测试路径 |
| 2026-04-29 | markdown link check 忽略外部 HTTP/HTTPS 链接 | CI 先保证仓库内文档链接，避免外部站点波动导致并行开发被误阻塞 |

## 验证方式

本地复现命令：

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name "*.go" -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json $(find . -name "*.md" -not -path "./.git/*" -not -path "./.ai-context/*" -not -path "./docs/references/*" -print)
git status --short --branch
```

PostgreSQL integration job 复现命令：

```bash
export DATABASE_URL=postgres://agents_im:agents_im_dev_password@localhost:5432/agents_im?sslmode=disable
bash scripts/migrate-postgres.sh --host-psql
go test -tags=integration ./tests
```

## 风险与回滚

- 风险：GitHub hosted runner 的系统包或 npm registry 临时不可用，导致工具安装失败。
- 缓解：关键 Go 工具使用固定版本；markdown link check 只检查仓库内链接，减少外部网络波动。
- 回滚：删除 `.github/workflows/ci.yml` 和 `.github/markdown-link-check.json`，并回退 `docs/GIT_WORKFLOW.md`、`scripts/verify-static.sh` 的 CI 相关条目。

## 结果记录

本地验证结果：

- `goctl --version`：通过，`goctl version 1.10.1 linux/amd64`。
- `for f in api/*.api; do goctl api validate -api "$f"; done`：通过，5 个 API spec 均返回 `api format ok`。
- `gofmt -w $(find . -name "*.go" -print)`：通过，无额外 Go 文件 diff。
- `go test ./...`：通过，普通测试不依赖真实 PostgreSQL。
- `bash scripts/verify-static.sh`：通过，返回 `static verification passed`。
- `docker compose config`：通过，Compose 配置可解析。
- `npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json ...`：通过；排除 `.ai-context/` 和 `docs/references/`，外部 HTTP/HTTPS 链接按配置忽略。
- `git status --short --branch`：提交前位于 `feature/ci-pipeline`，仅包含本任务相关变更。
