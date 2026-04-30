# agents_im

`agents_im` 是一个面向 IM + Agent 场景的实时聊天系统。项目以 Go 微服务为主体，提供用户、认证、好友、群聊、消息、WebSocket Gateway、Message Transfer、Agent 管理与 Agent runtime 基础能力，并配套 React/Vite 前端。

## 核心能力

- 用户资料与账号注册登录
- JWT 鉴权
- 好友关系管理
- 群聊与群成员管理
- 单聊 / 群聊消息存储、会话 seq、已读状态
- WebSocket 长连接、心跳、消息发送、离线补偿和在线推送
- PostgreSQL transactional outbox 与 Kafka/Redpanda 消息事件基础
- Redis presence 在线状态基础
- Agent profile 管理、Agent-IM 写回契约、Eino/DeepSeek runtime adapter 基线
- React/Vite 前端四 tab 方向：消息、联系人、发现、我的

## 技术栈

- 后端：Go、go-zero、gRPC、WebSocket
- 前端：React、Vite、TypeScript、Vitest
- 存储：PostgreSQL、Redis
- 消息：Kafka-compatible Redpanda
- Agent：CloudWeGo Eino、DeepSeek ChatModel adapter
- 可观测性：Prometheus text metrics、trace/request id 基础

## 仓库结构

```text
api/                 go-zero REST API 定义
cmd/                 服务入口，包括 user/auth/friends/groups/message/agent API、RPC、gateway-ws 等
config / etc/        本地和服务配置
internal/            核心业务逻辑、仓储、网关、Agent runtime、transfer worker 等
db/migrations/       PostgreSQL schema 迁移
proto/               gRPC proto 和生成代码
scripts/             本地启动、迁移、demo data、静态验证脚本
tests/               跨服务契约和 MVP smoke 测试
web/                 React/Vite 前端
docs/                架构、产品规格、设计文档、执行计划和开发文档
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

启动 PostgreSQL、Redis、Redpanda，执行迁移，构建并启动 REST API 与 WebSocket Gateway：

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
| Message API | `http://127.0.0.1:8083` |
| WebSocket Gateway | `ws://127.0.0.1:8084/ws` |
| Groups API | `http://127.0.0.1:8085` |
| Agent API | `http://127.0.0.1:8086` |
| Frontend dev server | `http://127.0.0.1:5173` |
| PostgreSQL | `localhost:5432` |
| Redis | `localhost:6379` |
| Redpanda Kafka | `localhost:19092` |

如果默认端口被占用，可用环境变量覆盖端口并指定独立状态目录：

```bash
USER_API_PORT=18080 \
AUTH_API_PORT=18081 \
FRIENDS_API_PORT=18082 \
MESSAGE_API_PORT=18083 \
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

## 单机 Smoke E2E

当 Docker、本地端口或外部服务环境不可用时，可以运行快速单进程 smoke：

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go run ./cmd/single-machine-e2e
```

该命令会在一个进程内验证：

1. 注册 Alice；
2. 注册 Bob；
3. Alice 添加 Bob 为好友；
4. 通过业务逻辑发送单聊消息；
5. Bob 拉取消息；
6. Alice/Bob 连接 WebSocket；
7. Alice 通过 WebSocket 发送消息；
8. Alice 收到 ACK，Bob 收到在线 `message_received` 推送。

它是快速 smoke，不替代完整 Docker + REST + WebSocket 的本地 E2E。

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

本地端到端 smoke：

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go run ./cmd/single-machine-e2e
```

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
RUN_LIVE_DEEPSEEK_TESTS=1 PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./internal/agentruntime/llm/deepseek -run TestLiveDeepSeekGenerate -count=1 -v
```

不要在日志、文档或提交中打印真实 API key。

## 开发规则

- 禁止用假实现、假成功、静默 fallback 冒充真实能力。
- 失败必须可见；能明确失败就不要隐藏错误。
- 复杂变更先写计划，完成后记录验证结果。
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
- 技术债追踪：[`docs/exec-plans/tech-debt-tracker.md`](./docs/exec-plans/tech-debt-tracker.md)
- Git 工作流：[`docs/GIT_WORKFLOW.md`](./docs/GIT_WORKFLOW.md)
