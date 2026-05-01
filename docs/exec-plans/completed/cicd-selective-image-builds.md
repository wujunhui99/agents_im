# CI/CD selective image builds

状态：Completed

## 背景

当前 deploy workflow 只区分是否需要构建镜像；任意业务变更都会构建全部后端服务和 web 镜像，并且服务器部署脚本会把所有 deployment 的镜像 tag 更新为当前 commit SHA。选择性构建时，未构建服务不会存在该 SHA tag，因此必须同时选择性设置镜像和 rollout。

## 目标

- 生产部署仍只允许 `main` 分支执行，包含手动 `workflow_dispatch`。
- `detect-changes` 输出后端服务列表、web 构建开关、镜像更新列表和 rollout 服务列表。
- 后端 build matrix 只包含受影响服务，web 只在 web 代码变更时构建。
- `scripts/deploy-k3s.sh` 支持通过 `IMAGE_SERVICES` 只更新受影响 deployment 镜像。
- Dockerfile 使用 BuildKit cache mount 提高 Go 和 npm 构建缓存命中率。
- 更新部署文档并完成静态验证。

## 非目标

- 不改变 CI workflow 的职责，不让 feature/develop 分支部署。
- 不引入真实 secret。
- 不重构 Kubernetes manifest 结构。

## 任务拆分

- [x] 扩展 deploy workflow 的 change detection 输出。
- [x] 调整后端动态矩阵、web 构建条件和 deploy job 条件。
- [x] 修改 deploy 脚本支持 `IMAGE_SERVICES`。
- [x] 优化 Dockerfile cache mount。
- [x] 同步部署和 Git workflow 文档。
- [x] 运行 YAML、shell、diff 和代表性变更检测验证。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-01 | 非 `main` ref 在 `detect-changes` 输出 no-op，并在 build/deploy job 额外加 `github.ref == 'refs/heads/main'` 防线。 | 手动触发如果选择非 main，workflow 可启动但不能 SSH 或发布。 |
| 2026-05-01 | 不能安全映射的非文档变更按后端全量构建处理；web 只有明显 web 相关路径才构建。 | fail-safe 避免漏部署后端，同时避免无依据构建 web。 |

## 验证方式

```bash
python3 - <<'PY'
import yaml
from pathlib import Path
for p in ['.github/workflows/deploy.yml', '.github/workflows/ci.yml']:
    data = yaml.safe_load(Path(p).read_text())
    print(p, list(data.get('jobs', {}).keys()))
PY
bash -n scripts/deploy-k3s.sh
git diff --check
```

并补充运行代表性文件列表的 detect-changes 逻辑检查。

## 风险与回滚

- 风险：路径映射不完整会导致少构建。缓解：未知非文档路径按后端全量构建。
- 风险：deploy job 对 skipped build job 的条件判断错误会导致不部署。缓解：静态检查 workflow 表达式，并用代表性检测输出确认服务列表。
- 回滚：revert 本次 workflow、Dockerfile、部署脚本和文档变更，恢复全量镜像构建部署。

## 结果记录

- 新增 `scripts/detect-deploy-changes.py`，输出稳定有序的 `backend_services`、`image_services` JSON 数组以及 rollout/restart 服务列表。
- `.github/workflows/deploy.yml` 在非 `main` ref 上 fail-closed/no-op；后端使用动态矩阵，web 只在 `web_required=true` 时构建，deploy job 接受未需要的 build job `skipped` 结果。
- `scripts/deploy-k3s.sh` 新增 `IMAGE_SERVICES` 和 `RESTART_SERVICES`，并校验服务名；`IMAGE_SERVICES` 为空且 `SKIP_SET_IMAGE=false` 时仍保留全量 set image 行为。
- Dockerfile 为 Go module/build cache 和 npm cache 增加 BuildKit cache mount。
- 已更新 `deploy/README.md`、`docs/GIT_WORKFLOW.md`、`ARCHITECTURE.md`、`README.md`。

验证记录：

```text
python3 YAML parse: .github/workflows/deploy.yml jobs detect-changes/build-backend/build-web/deploy; .github/workflows/ci.yml jobs backend/postgres-integration
bash -n scripts/deploy-k3s.sh: pass
python3 -m py_compile scripts/detect-deploy-changes.py: pass
git diff --check: pass
detect-change assertions: docs-only, web-only, cmd/user-api-only, shared internal, config-only service, manual main, manual non-main all pass
combined cmd/user-api + etc/groups-rpc check: image user-api, rollout user-api groups-rpc, restart groups-rpc
```
