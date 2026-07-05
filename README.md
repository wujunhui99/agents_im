# agents_im

`agents_im` 是一个把「实时 IM」和「AI Agent」揉在一起的开源聊天系统：既是一个微信风格的即时通讯应用，又是一个可以让 AI 助手像真人一样加入单聊、群聊、@互动的 Agent 平台。后端是一套 Go 微服务（账号、认证、好友、群聊、消息、WebSocket 网关、媒体、Agent runtime），前端是一套 React/Vite 的移动端 IM 界面。

> 🌐 **在线体验**：https://agenticim.xyz — 注册即可和默认 AI 助手聊天。
> 💬 用得不爽 / 想要某个功能？欢迎在 App 内「发现 → 反馈」直接提，或到 GitHub 提 Issue。

## 核心能力

### 💬 即时通讯（IM）

- 账号注册登录、个人资料、头像
- JWT 鉴权 + 活跃会话（按设备）管理
- 好友申请、通过、备注、好友列表
- 建群、群成员管理、群聊
- 单聊 / 群聊消息：持久化、会话 seq 有序、幂等、已读回执与未读数
- WebSocket 长连接：心跳保活、实时推送、断线重连、离线消息补偿
- 图片 / 文件等媒体消息：上传、下载、鉴权（独立 media 服务）
- 消息事件走 Kafka（Redpanda）链路做 fanout 与投递

### 🤖 AI Agent

- Agent 作为一种账号类型，可以像普通成员一样进入单聊 / 群聊
- 私聊 AI 助手：多轮对话、上下文记忆、多条追击消息合并回复
- 群聊 @Agent 触发，带循环预防（Agent 消息默认不再触发 Agent）
- 可组装的 Agent：系统提示词（可版本化）、工具（MCP / 本地 / 内置）、技能包（Skill）、模型配置
- 受限 Python 执行契约、工具调用与文件读取全链路审计（append-only）
- LLM runtime 基线：CloudWeGo Eino + DeepSeek ChatModel adapter

### 🛠️ 产品与体验

- 移动端四 tab：消息、联系人、发现、我的
- App 内用户反馈入口（问题反馈 / 体验建议 / 功能想法，支持截图）
- 管理后台（Admin Console）：管理 prompts、tools、skills、agents

## 技术栈

- 后端：Go、go-zero、gRPC、WebSocket
- 前端：React、Vite、TypeScript、Vitest
- 存储：PostgreSQL、Redis、RustFS（S3 兼容对象存储，存媒体与 skill 文件）
- 消息：Kafka / Redpanda 事件链路
- Agent：CloudWeGo Eino、DeepSeek ChatModel adapter
- 可观测性：Prometheus metrics、OpenTelemetry trace/request id、Langfuse
- CI/CD：Drone、GHCR、k3s、Docker Compose

## 仓库结构

```text
service/             各微服务入口与实现，每个域含 api(BFF) 与 rpc（user/auth/friends/groups/agent/msg/media、msggateway 等），main 在 service/<...>/cmd 或 service 目录
pkg/                 跨服务共享代码（authruntime、中间件等；顶层 internal/ 已于 #618 退役删除）
etc/                 各服务本地和生产配置
db/migrations/       PostgreSQL schema 迁移（append-only，已发布不可变）
scripts/             本地启动、迁移、demo data、静态验证与 CI 脚本
tests/               跨服务契约和 MVP smoke 测试
web/                 React/Vite 前端
docs/                架构、产品规格、设计文档、执行计划和开发文档
deploy/              k3s 应用部署清单、生产中间件 Compose、部署说明
```

## 快速开始

### 1. 准备环境

需要：

- Go toolchain
- Docker Compose
- Node.js / npm
- `goctl`

建议把本项目常用 Go 工具路径加入 `PATH`：

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
```

不要把真实密码、token 或 API key 提交到仓库。真实本地配置放到未跟踪的 `.env`。

### 2. 启动本地后端

启动 PostgreSQL、Redis、RustFS，执行迁移，构建并启动 REST API 与 WebSocket Gateway：

```bash
scripts/dev-up.sh
```

只启动中间件和迁移：

```bash
scripts/dev-up.sh --middleware-only
```

只重启 Go 服务，不启动 Docker 中间件：

```bash
scripts/dev-up.sh --services-only
```

停止脚本启动的本地服务：

```bash
scripts/dev-up.sh --stop
```

### 3. 启动前端

```bash
npm run frontend:dev
```

或使用 Makefile：

```bash
make start
```

常用 Makefile 命令：

```bash
make stop
make restart
make backend-start
make backend-stop
make frontend-start
make frontend-stop
make status
make test
make verify
```

## 端口

| 服务 | 地址 |
| --- | --- |
| User API | `http://127.0.0.1:8080` |
| Auth API | `http://127.0.0.1:8081` |
| Friends API | `http://127.0.0.1:8082` |
| Msg API | `http://127.0.0.1:8090` |
| WebSocket Gateway | `ws://127.0.0.1:8084/ws` |
| Groups API | `http://127.0.0.1:8085` |
| Agent API | `http://127.0.0.1:8086` |
| Frontend dev server | `http://127.0.0.1:5173` |
| PostgreSQL | `localhost:5432` |
| Redis | `localhost:6379` |

如果默认端口被占用，可用环境变量覆盖端口并指定独立状态目录：

```bash
USER_API_PORT=18080 \
AUTH_API_PORT=18081 \
FRIENDS_API_PORT=18082 \
MSG_API_PORT=18090 \
GATEWAY_WS_PORT=18084 \
GROUPS_API_PORT=18085 \
AGENT_API_PORT=18086 \
AGENTS_IM_DEV_STATE_DIR=/tmp/agents-im-dev-e2e \
PATH=/tmp/go/bin:$HOME/go/bin:$PATH \
scripts/dev-up.sh --services-only
```

## Demo 数据

后端启动成功后，可写入两个用户、好友关系、群聊和一条单聊消息：

```bash
scripts/dev-demo-data.sh
```

脚本会打印 demo ID 和 conversation ID，不打印 token 或密码。

## 验证命令

提交前建议至少运行：

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go vet ./...
bash scripts/verify-static.sh
git diff --check
```

前端相关变更还需要运行：

```bash
npm run frontend:test
npm run frontend:build
npm run frontend:lint
```

## CI/CD 与部署

仓库使用 Drone 作为 CI/CD 入口：

- `.drone.yml` `backend-verification`：PR/MR 合入前的 backend verification，包括 go-zero API validate、Go 格式、`go test ./...`、静态约束、Compose 配置和 Markdown link check。
- `.drone.yml` `postgres-integration`：使用独立 `postgres:16-alpine` service 跑 migration 与 integration tests，避免触碰生产数据库。
- `.drone.yml` `deploy-main`：`main` 分支 push 后先由 `scripts/ci/drone-detect-deploy.sh` / `scripts/detect-deploy-changes.py` 判断变更类型和受影响镜像；业务/镜像相关变更只构建受影响后端服务或 web 镜像，推送到 GHCR，再通过 SSH 把部署文件同步到服务器并运行 `scripts/deploy-k3s.sh`。纯 `deploy/k8s/**`、`etc/<service>.yaml`、`scripts/deploy-k3s.sh`、`.drone.yml` 或 `scripts/ci/**` 配置变更走 config-only deploy：跳过镜像构建、middleware 启动、数据库迁移和 `kubectl set image`，仅应用 manifests 并重启/等待受影响服务。

当前生产部署采用混合单机模型：k3s 承载 Go API/RPC/worker、web UI 及中间件 PostgreSQL、Redis、RustFS（GitOps 管理）。首次部署服务器需先执行 `scripts/bootstrap-server.sh` 创建 `/opt/agents-im/middleware/.env`、启动中间件并创建 k3s `agents-im-secrets`；真实 secret 只保存在服务器/k3s 或 Drone secrets，不提交到仓库，也不放进 CI 日志。

部署入口与必需 Drone Secrets 见 [`deploy/README.md`](./deploy/README.md)。

## Agent / DeepSeek 配置

Agent runtime provider 基线使用 Eino + DeepSeek ChatModel adapter。真实本地运行需要在未跟踪的 `.env` 中配置：

```text
DEEPSEEK_API_KEY=...
DEEPSEEK_BASE_URL=https://api.deepseek.com
DEEPSEEK_MODEL=deepseek-v4-pro
```

`DEEPSEEK_API_KEY` 缺失或仍是 `.env.example` 占位值时必须 fail-fast，不允许返回 mock/fake response。

Live DeepSeek smoke test 需要显式 opt-in：

```bash
set -a; . ./.env; set +a
RUN_LIVE_DEEPSEEK_TESTS=1 PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./service/agent/rpc/internal/runtime/llm/deepseek -run TestLiveDeepSeekGenerate -count=1 -v
```

不要在日志、文档或提交中打印真实 API key。

## 开发规则

- 禁止用假实现、假成功、静默 fallback 冒充真实能力。
- 失败必须可见；能明确失败就不要隐藏错误。
- 复杂变更先写清 GitHub Issue；需要长期上下文时更新对应产品规格、设计文档或运行手册。
- 新增或修改重要行为时，同步更新相关文档。
- 后端行为变更至少跑 Go 测试和静态验证。
- 前端联调变更同时检查前后端契约、dev 启动脚本和 MVP smoke 测试。

更详细的 Agent 工作入口见 [`AGENTS.md`](./AGENTS.md)。

## 文档入口

- 架构总览：[`ARCHITECTURE.md`](./ARCHITECTURE.md)
- 本地开发：[`docs/DEVELOPMENT.md`](./docs/DEVELOPMENT.md)
- 前端约定：[`docs/FRONTEND.md`](./docs/FRONTEND.md)
- 产品规格索引：[`docs/product-specs/index.md`](./docs/product-specs/index.md)
- 设计文档索引：[`docs/design-docs/index.md`](./docs/design-docs/index.md)
- Git 工作流：[`docs/GIT_WORKFLOW.md`](./docs/GIT_WORKFLOW.md)
- 部署说明：[`deploy/README.md`](./deploy/README.md)
