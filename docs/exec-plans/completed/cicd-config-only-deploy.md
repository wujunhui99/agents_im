# CI/CD Config-only Deploy Optimization

状态：Completed

## 背景

2026-05-01 排查 `Deploy to k3s` GitHub Actions 失败时，发现失败发生在远端 k3s 部署阶段，而不是镜像构建阶段。

失败 run：

- <https://github.com/wujunhui99/agents_im/actions/runs/25183364773>

关键现象：

```text
Waiting for deployment "user-api" rollout to finish: 1 old replicas are pending termination...
error: timed out waiting for the condition
```

登录服务器排查后，`user-api` 当前已经恢复为 `Running`，但 `groups-rpc` 持续 `CrashLoopBackOff`：

```text
groups-rpc   0/1   CrashLoopBackOff
```

`groups-rpc` 日志显示：

```text
Starting rpc server at 0.0.0.0:9093...
listen tcp 0.0.0.0:9093: bind: address already in use
panic: listen tcp 0.0.0.0:9093: bind: address already in use
```

服务器上 `9093` 已被 SSH/systemd socket 占用：

```text
0.0.0.0:9093  users:(("sshd"), ("systemd"))
[::]:9093     users:(("sshd"), ("systemd"))
```

而 k8s deployment 中 `groups-rpc` 使用 `hostNetwork: true`，所以容器绑定的是宿主机端口，导致与 SSH 端口冲突。

## 问题分析

原 CI/CD workflow 的行为是：任意 `main` 分支 push 都会触发：

1. 13 个后端服务镜像矩阵构建；
2. web 镜像构建；
3. 部署阶段将所有 deployment 的镜像 tag 更新为 `${{ github.sha }}`；
4. 对所有服务执行 rollout status。

这会导致一个问题：即使只是修改 `deploy/k8s/**` 这类 Kubernetes 配置，也会重新构建所有业务镜像，并强制全服务 rollout。

本次实际只需要修改 k8s 配置中的 `groups-rpc` 端口，不应该重新构建镜像，因为业务代码没有变。

## 目标

- 修复 `groups-rpc` 与宿主机 SSH `9093` 的端口冲突。
- 让纯 k8s 配置变更走 config-only deploy，不重新构建镜像。
- config-only deploy 时不执行数据库迁移、不重启 middleware、不重设所有 deployment 镜像。
- config-only deploy 时只重启并等待受影响服务，本次为 `groups-rpc`。

## 非目标

- 不修改业务代码。
- 不调整 Dockerfile 或根目录开发配置 `etc/groups-rpc.yaml`，避免本次变更被识别为镜像相关代码变更。
- 不改变手动 `workflow_dispatch` 的完整发布语义；手动触发仍按完整构建和部署执行。

## 变更内容

### 1. groups-rpc 端口改为 9103

修改 k8s manifest 中的 `groups-rpc` 端口，避开服务器 SSH 占用的 `9093`。

文件：`deploy/k8s/etc/groups-rpc.yaml`

```diff
- ListenOn: 0.0.0.0:9093
+ ListenOn: 0.0.0.0:9103
```

文件：`deploy/k8s/deployments.yaml`

```diff
- containerPort: 9093
+ containerPort: 9103
```

文件：`deploy/k8s/services.yaml`

```diff
- ports: [{ name: grpc, port: 9093, targetPort: 9093 }]
+ ports: [{ name: grpc, port: 9103, targetPort: 9103 }]
```

### 2. 新增变更检测 job

文件：`.github/workflows/deploy.yml`

新增 `detect-changes` job，输出：

- `build_required`
- `deploy_required`
- `config_only`

判断规则：

- `workflow_dispatch`：完整构建和部署，`build_required=true`。
- `deploy/k8s/**`、`scripts/deploy-k3s.sh`、`.github/workflows/deploy.yml`：只需要部署配置，`build_required=false`，`config_only=true`。
- `docs/**`、`README.md`、其他 Markdown：不部署。
- 其他文件：视为代码或镜像相关变更，完整构建和部署。

### 3. 构建 job 条件化

`build-backend` 和 `build-web` 增加条件：

```yaml
needs: detect-changes
if: needs.detect-changes.outputs.build_required == 'true'
```

纯配置变更时，这两个 job 会被跳过。

### 4. deploy job 支持 config-only 模式

`deploy` job 现在依赖：

- `detect-changes`
- `build-backend`
- `build-web`

并允许两种情况进入部署：

- `build_required=false`：直接部署配置；
- `build_required=true`：等待 backend 和 web 构建成功后部署。

config-only 模式传递以下环境变量到服务器：

```bash
SKIP_SET_IMAGE=true
SKIP_MIDDLEWARE=true
SKIP_MIGRATIONS=true
ROLLOUT_SERVICES=groups-rpc
RESTART_ROLLOUT=true
```

### 5. deploy-k3s.sh 增加开关

文件：`scripts/deploy-k3s.sh`

新增变量：

```bash
SKIP_SET_IMAGE="${SKIP_SET_IMAGE:-false}"
SKIP_MIDDLEWARE="${SKIP_MIDDLEWARE:-false}"
SKIP_MIGRATIONS="${SKIP_MIGRATIONS:-false}"
ROLLOUT_SERVICES="${ROLLOUT_SERVICES:-}"
RESTART_ROLLOUT="${RESTART_ROLLOUT:-false}"
```

新增行为：

- `SKIP_SET_IMAGE=true`：跳过 `kubectl set image`，保持现有镜像 tag。
- `SKIP_MIDDLEWARE=true`：跳过 `docker compose up -d` middleware。
- `SKIP_MIGRATIONS=true`：跳过数据库迁移。
- `ROLLOUT_SERVICES`：只等待指定服务 rollout；为空时仍等待全部服务。
- `RESTART_ROLLOUT=true`：在 `kubectl apply -k` 后主动 `kubectl rollout restart` 指定服务。

注意：ConfigMap 变更不会可靠触发 Pod 重建，所以 config-only 模式需要主动 rollout restart 目标 deployment。

## 发布记录

提交：

```text
6ddadcb ci: skip image builds for config-only deploys
```

推送到：

```text
origin/main
```

触发的 GitHub Actions run：

- <https://github.com/wujunhui99/agents_im/actions/runs/25193698668>

run 初始状态符合预期：

```text
Detect changed areas: success
Build backend image: skipped
Build web image: skipped
Deploy on server: in_progress
```

说明本次配置变更未重新构建业务镜像。

## 验证

本地验证：

```bash
bash -n scripts/deploy-k3s.sh
git diff --check
python3 - <<'PY'
import yaml
with open('.github/workflows/deploy.yml') as f:
    data = yaml.safe_load(f)
print('workflow yaml parsed')
print('jobs:', ', '.join(data['jobs'].keys()))
PY
```

结果：

```text
workflow yaml parsed
jobs: detect-changes, build-backend, build-web, deploy
```

服务器验证：

```bash
ss -ltnp | grep ':9103' || true
```

结果：`9103` 未被占用。

GitHub Actions 验证：

```bash
gh run view 25193698668 --repo wujunhui99/agents_im --json status,conclusion,jobs
```

结果显示：

- `Detect changed areas` 成功；
- `Build backend image` 跳过；
- `Build web image` 跳过；
- `Deploy on server` 开始执行。

## 风险与注意事项

- 当前 config-only 判断将 `.github/workflows/deploy.yml` 视为部署配置变更，会触发部署但不构建镜像；这适合本次场景，但如果后续 workflow 变更影响镜像构建逻辑，可能需要手动 `workflow_dispatch` 做完整发布。
- 当前 config-only 模式固定 `ROLLOUT_SERVICES=groups-rpc`，适合本次修复。后续如果修改其他服务的 ConfigMap 或 manifest，需要扩展变更检测逻辑，按变更文件推导受影响服务。
- `hostNetwork: true` 会让所有服务直接占用宿主机端口。后续应统一梳理端口规划，避免再次与系统服务或其他 Pod 冲突。
- 如果只是改 Service 端口但应用实际监听端口不同，会导致 readiness/liveness 或服务访问异常；因此 `ListenOn`、`containerPort`、Service `port/targetPort` 要保持一致。

## 回滚方案

### 回滚端口变更

如果 `9103` 引发其他问题，可将以下文件中的 `9103` 改回其他未占用端口：

- `deploy/k8s/etc/groups-rpc.yaml`
- `deploy/k8s/deployments.yaml`
- `deploy/k8s/services.yaml`

不建议改回 `9093`，因为服务器 SSH 正在占用该端口。

### 回滚 config-only deploy 逻辑

回退提交：

```bash
git revert 6ddadcb
```

但这会同时回退 `groups-rpc` 端口修复和 CI/CD 优化。如只想回退 workflow，需要单独 revert `.github/workflows/deploy.yml` 与 `scripts/deploy-k3s.sh` 的相关变更。

## 后续建议

- 将服务端口规划写入部署文档，明确每个 `hostNetwork` 服务占用的宿主机端口。
- 后续考虑移除不必要的 `hostNetwork: true`，优先通过 ClusterIP Service 做集群内访问，通过 Ingress/NodePort 暴露边界服务。
- 扩展 config-only 变更检测：根据变更的 `deploy/k8s/etc/<service>.yaml` 自动设置 `ROLLOUT_SERVICES=<service>`。
