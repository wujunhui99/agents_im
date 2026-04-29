# 单机 E2E 前端真实主流程整改

## 背景

用户要求在本机启动并验证真实单机 E2E：注册两个用户、登录、加好友、聊天发消息。整改前，前端登录后的主流程仍存在 mock/default demo 数据源：

- 消息 Tab 默认进入 mock 消息页，本地 ACK 可伪造发送成功。
- 联系人页存在硬编码好友、搜索用户和群聊数据。
- `web/src/data/mockData.ts` 与 `web/src/features/messages/mockConversations.ts` 可被生产主流程引用。

这些会导致 UI 看起来成功，但没有证明真实后端 API 可用，违反 fail-first 和禁止假实现规则。

## 目标

1. 登录后消息页默认走真实 `MessageApi`，不再支持 mock mode。
2. 联系人页通过真实 `UserApi` / `ContactsApi` 搜索用户、添加好友、刷新好友列表。
3. 删除生产 mock/demo 数据源文件与旧页面兼容层。
4. 增加/更新测试，证明主流程请求真实 contract path，并注入 bearer token。
5. 完整验证后合并到 `develop`，再合并到 `main`。
6. 在本机启动服务执行真实注册/登录/加好友/聊天 E2E；若失败，明确失败原因，不假成功。

## 非目标

- 本轮不声称多 Gateway 远程投递、Kafka retry/DLQ 等非单机可靠性技术债完成。
- `发现` 页仍可保留明确标注的 MVP 占位入口，但不能伪造真实能力。

## 执行步骤

- [x] 确认 E2E 阻塞点：消息页 mock 默认路径、联系人页硬编码 demo 数据。
- [x] 先写/更新失败测试：App 主流程必须调用 `/conversations/seqs`、`/users/:identifier`、`/friends`、`/messages`。
- [x] 删除生产 mock 数据源，改造消息页和联系人页走真实 API。
- [x] 运行完整验证并修复失败。
- [x] feature -> develop -> main 合并推送。
- [x] 本机启动真实服务并执行 E2E。

## 验收标准

```bash
npm install --prefix web
npm run frontend:test
npm run frontend:build
npm run frontend:lint
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
docker compose config
git diff --check
```

真实 E2E 另需：

```bash
make start
make status
```

并通过真实后端路径完成：注册 A、注册 B、登录、A 添加 B、聊天发送消息、B 可收到或拉取消息。


## 已完成补充修复

另一个 Agent 在单机 E2E 排查中发现并修复了两个合理问题，已评审通过并并入长期文档：

1. **WebSocket live delivery**：`send_message` 通过 WebSocket 持久化并 ACK 后，单聊在线接收方现在会收到同实例 `message_received` push。失败只记录日志，不回滚已持久化消息；跨实例投递仍由 Message Transfer / Delivery pipeline 负责。
2. **本地 E2E 启动韧性**：`scripts/dev-up.sh --services-only` 可以在 Docker middleware 已运行时只重启 Go host services，并支持 `USER_API_PORT`、`AUTH_API_PORT`、`FRIENDS_API_PORT`、`MESSAGE_API_PORT`、`GATEWAY_WS_PORT`、`GROUPS_API_PORT` 覆盖，方便默认端口被 stale/root-owned 进程占用时使用备用端口。
3. **单进程 smoke**：`go run ./cmd/single-machine-e2e` 可在不绑定外部 HTTP 端口的情况下验证注册、加好友、消息逻辑、WebSocket ACK 和在线 push。它是 smoke check，不替代完整 Docker/REST/WebSocket runtime E2E。

对应长期文档位置：

- `docs/DEVELOPMENT.md`
- `docs/design-docs/websocket-gateway.md`
