# Drone CI 监控指南

本文档说明如何监控 `agents_im` 项目的 Drone CI 构建状态。

## CI 基础信息

- **Drone 控制台**：https://drone.agenticim.xyz/wujunhui99/agents_im
- **触发方式**：push 到 `main` 分支自动触发部署流水线
- **Token 位置**：`secret/drone_token`（本地未追踪文件）

---

## 方案一：Drone CLI `watch`（推荐，**待定**）

> **⚠️ 待定**：尚未在本项目正式安装和验证，流程待实际测试确认。

### 安装 Drone CLI

```bash
# macOS
brew install drone-cli

# 或手动下载
curl -L https://github.com/harness/drone-cli/releases/latest/download/drone_darwin_amd64.tar.gz | tar zx
sudo mv drone /usr/local/bin/
```

### 配置环境变量

```bash
# 从 secret/drone_token 中读取并 export（勿硬编码）
export DRONE_SERVER=https://drone.agenticim.xyz
export DRONE_TOKEN=<secret/drone_token 中的值>
```

### 常用命令

```bash
# 列出最近构建
drone build ls wujunhui99/agents_im

# 实时流式输出指定构建日志（阻塞，直到构建结束）
drone build watch wujunhui99/agents_im <build_number>

# 查看指定构建信息
drone build info wujunhui99/agents_im <build_number>
```

### 典型用法

合并 PR 后，拿到 main 最新 build number，执行：

```bash
drone build watch wujunhui99/agents_im 192
```

日志会实时输出各 step（clone / detect changes / build images / deploy / notify telegram），deploy step 是关键，等它出现 `deployment successfully rolled out` 即为成功。

---

## 方案二：Drone API 轮询（无需额外安装）

适合在 agent 会话内快速检查，不依赖 CLI 工具：

```bash
source secret/drone_token
curl -s -H "Authorization: Bearer $DRONE_TOKEN" \
  "$DRONE_SERVER/api/repos/wujunhui99/agents_im/builds?limit=3" | \
  python3 -c "
import sys, json
for b in json.load(sys.stdin):
    print(f'#{b[\"number\"]} {b[\"status\"]:10} {b[\"message\"][:60]}')
"
```

---

## 方案三：Telegram 通知（已有，零轮询）

流水线最后一步 `notify telegram` 在构建结束后自动推送结果。无需主动查询，结果会推到对应群/频道。

> 当前状态：步骤已存在，效果取决于 Telegram bot token 和 chat_id 是否正确配置。

---

## 部署后健康检查

构建 success 后，还需验证 k8s 层：

```bash
ssh -p 9093 root@207.57.131.50 \
  "kubectl get pods -n agents-im | grep -v Running | grep -v Completed"
```

无输出 = 所有 pod 健康。重点关注 `auth-rpc`（历史上曾出现 CrashLoopBackOff）。
