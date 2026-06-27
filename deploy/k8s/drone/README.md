# Drone CI on k3s（bootstrap 事实源）

Drone server + docker runner 跑在 k3s `drone` namespace（2026-06-10 服务器重装起）：

- **drone-server**：`drone/drone:2`，sqlite 落 local-path PVC，ingress `drone.agenticim.xyz`
  （cert-manager `letsencrypt-prod` 签证书）。
- **drone-runner-docker**：挂宿主机 `/var/run/docker.sock` —— 流水线 step 容器跑在宿主 docker，
  所以 `.drone.yml` 里的 host volume（`/opt/agents-im`、`/etc/rancher/k3s`）按宿主机路径解析，
  deploy 步骤的 `host.docker.internal:host-gateway` 能打到节点上的 k3s API。
- **不在 kustomization 里**，由 `scripts/bootstrap-server.sh` 引导时 apply。
- 凭据在 k8s secret `drone/drone-secrets`：`DRONE_GITHUB_CLIENT_ID/SECRET`（GitHub OAuth App，
  callback `https://drone.agenticim.xyz/login`）、`DRONE_RPC_SECRET`、`DRONE_USER_CREATE`
  （预置 admin 用户与 API token）。值来自服务器 `/opt/agents-im/{secrets,creds}.env`，仓库不存。
- 坑：Service 名含 `drone-server` 会让 k8s 注入 `DRONE_SERVER_PORT=tcp://...` 环境变量，Drone
  会当成监听端口配置而启动失败 —— deployment 已设 `enableServiceLinks: false`。
- 仓库激活后必须设 `trusted=true`（host volume 需要），repo secrets：`ghcr_username`、
  `ghcr_token`、`telegram_bot_token`、`telegram_chat_id`。
- backend/web 构建共用宿主持久的 `agents-im-drone-builder`。新建 builder 时必须传
  `scripts/ci/buildkitd.toml`，将 BuildKit 总占用限制在约 20GB、目标空闲空间设为 30GB，
  同时优先保留 7 天内使用过的 Go/npm cache mount。修改该配置后需在无构建运行时重建
  builder 容器（保留同名 state volume），否则运行中的 buildkitd 不会热加载配置。
