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
- [ ] 运行完整验证并修复失败。
- [ ] feature -> develop -> main 合并推送。
- [ ] 本机启动真实服务并执行 E2E。

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
