# Message Ordering Hardening

状态：Completed

## 背景

当前消息服务已经按会话维护 `seq`，并提供幂等发送与按 seq 拉取能力。前端聊天窗口仍按 `sendTime` 排序，且同一会话内快速连续发送可能并发触发多个 `POST /messages`，在网络波动下导致本地展示顺序偏离服务端权威顺序。

## 目标

- 明确 `conversation_id + seq` 是消息展示和同步的权威顺序。
- 证明内存仓储和 PostgreSQL 仓储在同一会话并发写入时产生唯一、连续的 seq。
- 前端按服务端 seq 排序并按消息身份去重。
- 前端同一会话内发送时避免多个请求并发飞行，失败时可见。
- 文档和静态检查覆盖新的排序与幂等约束。

## 非目标

- 不实现 MinIO、媒体或对象存储。
- 不重构 WebSocket push 协议。
- 不改变 JWT/auth 语义。
- 不引入前端 mock fallback 或后端静默降级。

## 任务拆分

- [x] 后端仓储并发、幂等、last-message-by-seq 测试。
- [x] 前端消息排序、去重、确认替换、同会话发送 in-flight 防护测试。
- [x] 前端实现 seq-first 展示和发送防护。
- [x] 文档更新 message chain、frontend-backend contract、reliability。
- [x] 静态检查增加 message ordering/schema/client 约束。
- [x] 执行验证命令，记录结果。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-01 | 使用 UI 禁用同会话 composer 的方式防止并发发送，而不是后台队列 | MVP 可见、简单、失败优先；不会静默吞掉第二次发送 |
| 2026-05-01 | PostgreSQL 并发测试保持 opt-in，默认 `go test ./...` 不依赖外部中间件 | 符合默认测试不要求 live PostgreSQL 的约束 |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name '*.go' -print)
go test ./...
npm run frontend:test
npm run frontend:build
npm run frontend:lint
bash scripts/verify-static.sh
docker compose -f deploy/middleware/docker-compose.yml config
bash -n scripts/dev-up.sh
bash -n scripts/dev-demo-data.sh
git diff --check
```

## 风险与回滚

- WebSocket push 到达乱序仍需要客户端按 seq 应用和 gap sync；本次只加固当前 REST 拉取/聊天页展示路径和契约文档。
- PostgreSQL 并发证明默认跳过，必须在有 DATABASE_URL 的环境中用 opt-in 变量运行。
- 回滚时恢复前端排序/发送控制和新增测试文档即可，不涉及数据迁移。

## 结果记录

- `internal/repository/message_repository_contract_test.go` 新增内存仓储默认并发/幂等/last-message-by-seq 合约测试；PostgreSQL 合约仍通过 `AGENTS_IM_TEST_POSTGRES_CONTRACT=1` opt-in。
- `tests/postgres_persistence_integration_test.go` 在 `integration` build tag 下新增真实 PostgreSQL 并发顺序测试。
- `web/src/features/messages/MessagesPage.tsx` 改为按 `seq` 展示 confirmed 消息、按身份去重、pending 无 seq 消息稳定排在 confirmed 之后，并在同一会话发送 in-flight 时禁用 composer。
- 文档已明确网络/WebSocket/push 到达顺序不是展示顺序，客户端应按 `conversation_id + seq` 排序、按身份去重，并在 gap 时拉取缺失 seq 范围。
- 验证通过：
  - `goctl --version`
  - `for f in api/*.api; do goctl api validate -api "$f"; done`
  - `gofmt -w $(find . -name '*.go' -print)`
  - `go test ./...`
  - `npm run frontend:test`
  - `npm run frontend:build`
  - `npm run frontend:lint`
  - `bash scripts/verify-static.sh`
  - `bash -n scripts/dev-up.sh`
  - `bash -n scripts/dev-demo-data.sh`
  - `git diff --check`
- `docker compose -f deploy/middleware/docker-compose.yml config` 在未设置环境变量时按设计失败：`POSTGRES_PASSWORD is required`。使用 non-secret placeholder 环境变量 `POSTGRES_PASSWORD=dev-placeholder REDIS_PASSWORD=dev-placeholder` 后配置校验通过。
