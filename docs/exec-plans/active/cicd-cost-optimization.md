# CI cost optimization notes

状态：Draft / Record-only

## 背景

GitHub 提示当前仓库 GitHub Actions 免费分钟消耗过多。近期工作流中，Codex/Agent 常见开发路径会在同一变更上重复触发多次 CI：

1. feature 分支 push 触发一次 CI；
2. feature 分支创建/更新 PR 到 `develop` 再触发一次 PR CI；
3. PR 合并到 `develop` 后，`develop` push 再触发一次集成 CI；
4. `develop` 合并到 `main` 后，`main` push 还会触发 CI 和部署。

其中第 1 步对 Codex 开发过程中的每次中间 push 都会消耗 Actions 分钟，但真正的合并门禁仍然是 PR 到 `develop` 的 CI 以及合并后的 `develop` 集成 CI。

## 当前观察

当前 `.github/workflows/ci.yml` 的触发配置包含：

```yaml
on:
  pull_request:
    branches:
      - develop
      - main
  push:
    branches:
      - develop
      - main
      - "feature/**"
```

这意味着：

- `feature/**` push 会触发 CI；
- `pull_request` 到 `develop` / `main` 会触发 CI；
- `develop` / `main` push 会触发 CI；
- `fix/**`、`refactor/**` 等分支 push 当前不会直接触发 CI，除非开 PR。

## 需求记录：减少 feature 分支 push CI

### 目标

为了减少 GitHub Actions 免费分钟消耗，计划取消 feature 分支 push 触发 CI：

- 保留 `pull_request` 到 `develop` 的 CI；
- 保留 `pull_request` 到 `main` 的 CI；
- 保留 `push` 到 `develop` 的 CI；
- 保留 `push` 到 `main` 的 CI；
- 去掉 `push.branches` 中的 `"feature/**"`。

### 预期变更

将 `.github/workflows/ci.yml` 从：

```yaml
push:
  branches:
    - develop
    - main
    - "feature/**"
```

调整为：

```yaml
push:
  branches:
    - develop
    - main
```

## 预期效果

Codex/Agent 在 `feature/**` 分支开发过程中的中间 push 不再直接消耗 CI 分钟。

典型 feature 开发链路从：

```text
feature push -> CI
PR to develop -> CI
merge to develop -> CI
```

减少为：

```text
PR to develop -> CI
merge to develop -> CI
```

如果 Codex 在 feature 分支上多次 push，节省效果会更明显。

## 风险与约束

- feature 分支 push 后不会自动得到 GitHub Actions 反馈；Codex/Agent 必须继续在本地或 worktree 内运行必要验证。
- PR 创建前的错误会延迟到 PR CI 才暴露。
- Controller/Hermes 仍需在合并到 `develop` 前检查 PR CI，并在合并后监控 `develop` push CI。
- 对 `main` 发布链路不应降低门禁：`main` push CI 和部署仍保留。

## 后续实施前检查

实施前应检查：

- `.github/workflows/ci.yml` 当前 trigger 是否仍与本记录一致；
- 是否存在其他 workflow 也对 `feature/**` push 触发，例如 deploy/notification；
- 分支保护规则是否要求 PR CI，而不是依赖 feature push CI；
- Codex 任务说明是否需要补充“feature push 不触发远端 CI，提交前必须本地跑验证”。

## CI 历史耗时观察

基于近期 30 次 `CI` workflow 历史（调整前基线）：

- 样本：30 个 completed CI workflow、91 个 jobs、1025 个 steps。
- 单次 CI 墙钟时间通常约 `4m42s ~ 5m39s`。
- 按 runner job 计费视角，主要消耗来自：
  - `Backend verification`：平均约 `4m18s`，中位数约 `4m32s`，最大约 `5m14s`；
  - `PostgreSQL integration`：平均约 `1m21s`，中位数约 `1m22s`，最大约 `1m33s`；
  - `Telegram notification`：平均约 `6s`，中位数约 `5s`，最大约 `10s`，可忽略。

### 调整前 workflow/job 耗时明细

近期代表性 completed CI runs：

```text
run_id       branch                                  event          result   sha      duration
25586551676  main                                    push           success  bb1cf1a  5m11s
25586423787  develop                                 pull_request   success  c15e352  4m42s
25573028464  develop                                 push           success  c15e352  5m39s
25572760705  refactor/issue-55-goctl-scaffold-cleanup pull_request success  81dac61  4m55s
25571261338  develop                                 push           success  0e84b76  5m07s
25571010255  controller/integrate-issue-46           pull_request   success  a23a9e4  4m57s
25570686845  develop                                 push           success  286f85a  5m08s
25570432400  controller/integrate-issues-47-50       pull_request   success  6f088ca  5m08s
25570223519  develop                                 push           success  f2ac451  4m56s
25570207328  develop                                 push           success  c8c8c17  5m00s
25569541465  develop                                 push           success  81dbab3  5m00s
25568448390  feature/issue-50-group-management       pull_request   success  d1406d0  4m59s
25568429592  feature/issue-50-group-management       push           success  d1406d0  5m35s
```

调整前 job 平均耗时：

```text
job                       count  avg    median  max
Backend verification      30     4m18s  4m32s   5m14s
PostgreSQL integration    30     1m21s  1m22s   1m33s
Telegram notification     30     6s     5s      10s
Hermes webhook notification 1    7s     7s      7s
```

调整前最耗时单个 job：

```text
5m14s  Backend verification  run=25568429592  branch=feature/issue-50-group-management  event=push
4m54s  Backend verification  run=25586551676  branch=main                               event=push
4m48s  Backend verification  run=25573028464  branch=develop                            event=push
4m47s  Backend verification  run=25571261338  branch=develop                            event=push
4m47s  Backend verification  run=25567488362  branch=feature/issue-46-ai-hosting-async  event=push
4m46s  Backend verification  run=25567501084  branch=feature/issue-46-ai-hosting-async  event=pull_request
4m45s  Backend verification  run=25570686845  branch=develop                            event=push
4m44s  Backend verification  run=25570432400  branch=controller/integrate-issues-47-50  event=pull_request
```

### 调整前 step 耗时明细

按 step 名称聚合的耗时：

```text
job                     step                                            count  avg    median  max
Backend verification    Run Go tests                                    30     2m34s  2m50s   3m06s
Backend verification    Install goctl                                   30     48s    48s     52s
PostgreSQL integration  Run PostgreSQL integration tests                30     24s    24s     26s
Backend verification    Install system tools                            30     18s    16s     35s
PostgreSQL integration  Install PostgreSQL client                       30     16s    16s     21s
Backend verification    Setup Go                                        30     16s    16s     25s
PostgreSQL integration  Setup Go                                        30     15s    15s     18s
PostgreSQL integration  Initialize containers                           30     14s    14s     20s
Backend verification    Check markdown links                            30     10s    10s     25s
Backend verification    Install protoc generators                       30     4s     4s      10s
Backend verification    Run static verification                         30     2s     2s      3s
PostgreSQL integration  Reset PostgreSQL schema after old-schema fixture 30    2s     2s      4s
PostgreSQL integration  Run PostgreSQL migrations                       30     1s     1s      2s
PostgreSQL integration  Verify old PostgreSQL schema upgrade            30     1s     1s      3s
PostgreSQL integration  Verify PostgreSQL migrations are repeatable     30     1s     1s      1s
Backend verification    Validate go-zero API specs                      30     0s     0s      1s
Backend verification    Check gofmt                                     30     0s     0s      1s
Backend verification    Validate Docker Compose config                  30     0s     0s      1s
```

调整前最耗时的单个 steps：

```text
3m06s  Backend verification :: Run Go tests  run=25568429592  feature/issue-50-group-management push
3m02s  Backend verification :: Run Go tests  run=25571261338  develop push
3m02s  Backend verification :: Run Go tests  run=25570432400  controller/integrate-issues-47-50 pull_request
3m00s  Backend verification :: Run Go tests  run=25567501084  feature/issue-46-ai-hosting-async pull_request
2m59s  Backend verification :: Run Go tests  run=25570686845  develop push
2m58s  Backend verification :: Run Go tests  run=25569541465  develop push
2m56s  Backend verification :: Run Go tests  run=25586551676  main push
2m56s  Backend verification :: Run Go tests  run=25567488362  feature/issue-46-ai-hosting-async push
52s    Backend verification :: Install goctl run=25568429592  feature/issue-50-group-management push
51s    Backend verification :: Install goctl run=25586551676  main push
50s    Backend verification :: Install goctl run=25573028464  develop push
50s    Backend verification :: Install goctl run=25571261338  develop push
```

结论：调整前的最大消耗来自 `Backend verification :: Run Go tests`，其次是 `Install goctl`。`PostgreSQL integration` 整体耗时稳定但短于 backend；Telegram notification 可忽略。

## 优化优先级记录

### P0：去掉 feature push CI

同意作为第一优先级实施。取消 `feature/**` 分支 push 触发 CI，只保留 PR 到 `develop` / `main`、push 到 `develop` / `main` 的 CI。

原因：每次 feature 中间 push 都会消耗约一个完整 CI 的 runner 分钟，而真正的合并门禁仍由 PR CI 和 `develop` 集成 CI 提供。

### P1：使用 path filter 减少无关检查

后续考虑引入路径过滤，让不同类型变更只触发必要检查：

- docs-only：跳过 Go tests、PostgreSQL integration、frontend build/test，只做 Markdown/link/diff 等轻量检查；
- web-only：优先只跑 frontend test/build 和必要静态检查，不跑 backend/postgres；
- backend/API/proto/internal/cmd/config 变更：跑 backend verification；
- DB/migration/repository 相关变更：跑 PostgreSQL integration；
- workflow/deploy/script 变更：跑对应 YAML/shell/static 检查，并按影响范围决定是否跑 backend/PG。

当前决策：`Backend verification` 和 `PostgreSQL integration` 的触发逻辑应保持一致：**没有后端相关代码/配置/数据库/schema/脚本变更时，两者都不执行**。例如仅文档变化、仅前端代码变化，不应执行 backend 或 postgres-integration。

风险：path filter 必须 fail-safe。无法明确分类的非文档变更应默认跑完整 backend/PG 门禁，避免漏测。

### P2：优化 `go test ./...`

后续重点优化 `Backend verification :: Run Go tests`，因为这是当前最大单步耗时。

可评估方向：

- 避免 `go test ./...` 扫到非项目 Go 包，例如当前历史中出现过 `web/node_modules/flatted/golang/pkg/flatted`；
- 改成基于 `go list` 的包列表过滤，例如排除 `web/node_modules`；
- 仅在 path filter 判定 backend 相关时运行全量 Go tests；
- 谨慎评估 test sharding：它能缩短墙钟时间，但可能增加并行 job 数和总 runner 分钟，不一定适合免费额度优化。

### P3：缓存或替代 goctl 安装

后续优化 `Install goctl`，当前每次平均约 `48s`。

可评估方向：

- 缓存 `$HOME/go/bin/goctl` / Go module cache；
- 使用固定版本预构建二进制下载；
- 或使用自维护工具镜像/预装环境。

要求：必须保持 goctl 版本固定、可复现，不因缓存命中导致版本漂移。

### 暂不合并 Backend verification 和 PostgreSQL integration

已决定暂不把 `Backend verification` 与 `PostgreSQL integration` 合并成一个 job。

原因：两者当前并行运行；合并后只能节省少量重复 setup 时间（估计约 `25s ~ 35s`/次 CI），但会让 PostgreSQL 检查串行排在 backend 后面，增加 PR 等待墙钟时间，并降低失败定位清晰度。

### Go dependency cache 现状

当前 `Backend verification` 和 `PostgreSQL integration` 均使用 `actions/setup-go@v5` 且 `cache: true`，已经会基于 `go.sum` 等信息缓存 Go module/build cache。

这类缓存有助于减少 `go test` 和 `go install` 的依赖下载时间，但不等同于缓存已安装的工具二进制，例如 `$HOME/go/bin/goctl`。`goctl` 目前仍每次通过 `go install github.com/zeromicro/go-zero/tools/goctl@v1.10.1` 安装，历史平均约 `48s`。

### goctl 校验取舍

当前 `goctl api validate -api api/*.api` 本身很快，主要耗时来自安装 `goctl`。完全移除 goctl 安装和 API 校验预计可减少接近 `48s`/次 backend CI。

决策：**完全删除 CI 中的 goctl 安装和 `goctl api validate`。**

理由：goctl 在本项目主要检查 `.api` 文件能否被 goctl 正确解析。若 `.api` 不正确，开发阶段通常无法基于 `.api` 生成代码，问题应在 Codex/开发者本地生成代码时暴露；CI 不必为每次运行重复安装 goctl 只做该校验。

后续实施要点：

- 删除 `Backend verification` 中的 `Install goctl` step；
- 删除 `Validate go-zero API specs` step；
- 保留 Go 编译/测试、静态检查和 PR/develop/main 门禁；
- Codex/Agent 任务说明仍应要求：如果修改 `.api`，必须在开发分支本地运行相应 goctl 生成/验证，并提交生成后能通过编译测试的代码。

### protoc generators 作用与后续决策

当前 `Install protoc generators` 安装：

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1
```

作用：为 `protoc` 提供 Go / gRPC 代码生成插件。它们通常用于根据 `proto/**/*.proto` 生成 `.pb.go` / `_grpc.pb.go`。

已检查 `.github/workflows/*` 与 `scripts/*`：当前 CI 没有运行 `protoc`、`goctl rpc protoc` 或 `go generate`。`scripts/verify-static.sh` 只是用 `rg` 检查已提交生成文件是否包含 `Code generated by protoc-gen-go` 标记，不需要安装 `protoc-gen-go` / `protoc-gen-go-grpc`。

决策：**删除 CI 中的 `Install protoc generators` step。**

理由：当前 CI 不重新生成 protobuf 代码，安装 generators 只做版本输出，没有参与实际验证。删除预计节省约 `4s`，最大约 `10s`/次 backend CI，并简化工具安装。

后续实施要点：

- 删除 `.github/workflows/ci.yml` 中 `Install protoc generators` step；
- 同步更新 `scripts/verify-static.sh` 中对 CI workflow 必须包含 `protoc-gen-go` / `protoc-gen-go-grpc` 的 pattern 要求；
- 保留 `scripts/verify-static.sh` 对已提交 `.pb.go` / `_grpc.pb.go` 生成文件标记的检查；
- 如果未来要在 CI 中验证 proto 生成一致性，再重新引入 protoc/generator 安装，并仅在 `proto/**` 变更时运行。

### CI workflow 自身变更的门禁

对于本轮 CI workflow 优化本身（例如删除 `feature/**` push 触发、删除 goctl/protoc 安装、引入 path filter），虽然没有业务代码修改，但它会改变 CI 门禁逻辑，属于高影响基础设施变更。

决策：**修改 CI workflow 的 PR 应触发并通过 backend 和 postgres-integration 一次。**

理由：

- 需要验证新的 workflow 条件没有把关键门禁误跳过；
- 需要验证 `scripts/verify-static.sh` 与 workflow 内容同步，不会因删除 goctl/protoc pattern 后产生静态检查漂移；
- 这是一次 CI 机制变更，不应按普通 docs-only 或 web-only 处理。

后续 path filter 规则中，`.github/workflows/**`、`scripts/verify-static.sh`、`scripts/verify-postgres-*.sh`、`scripts/migrate-postgres.sh` 等 CI/验证脚本变化应默认触发 backend 和 postgres-integration，除非未来拆出单独的 workflow-validation job 并明确覆盖等效风险。

### CI telemetry additions

为继续定位 `Run Go tests` 的实际耗时来源，CI workflow 增加以下观测项：

- 在 backend job 中新增独立 `Download Go modules` step，显式运行 `go mod download` 并打印耗时，用于区分依赖下载/cache miss 与测试/编译耗时；
- `Run Go tests` 改为 `go test -json`，同时 `tee` 到 `/tmp/go-test.json`，并在 step 末尾汇总最慢 package、测试 package 数、失败 package；
- 为分析 GitHub Actions 上本地 `go test` 约 6s、CI `Run Go tests` 约 2m+ 的差异，曾临时拆分 `go test -run '^$'` compile/empty-run 与后续 `go test -json` 真实执行计时；诊断确认瓶颈主要在 Go 编译/build cache 后，临时双跑已移除，避免常态 CI 额外变慢；
- `actions/setup-go` 内置 cache 改为关闭，改用显式 `actions/cache/restore` / `actions/cache/save` 分开管理 `~/go/pkg/mod` 与 `~/.cache/go-build`。backend job 负责保存完整 Go module/build cache，postgres-integration 只 restore 不 save，避免并行 job 用较小/不完整 cache 抢占同一个 key；
- Telegram notification 增加 workflow job timing 汇总，通知中包含各 job 的 conclusion/status 与 duration，方便在 Telegram 里直接看到本次 CI 耗时。

### 暂不合并 Backend verification 和 PostgreSQL integration

本文件只记录需求和预期，不修改 workflow。
