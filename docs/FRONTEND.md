# FRONTEND.md

本文档记录 `agents_im` Web 前端约定。当前阶段已搭建微信风格主框架，并已把登录后的消息、联系人、加好友、发消息主流程切到真实 REST API。生产代码不再保留 mock/default demo 数据源；mock/fetch stub 仅允许存在于测试 fixture 中。

## 技术栈

- React + TypeScript
- Vite
- Vitest + Testing Library
- lucide-react 图标
- 原生 CSS，先不引入 UI 组件库，避免过早绑定设计系统

## 当前阶段范围

前端第一阶段参考微信主框架，完成四个一级页面：

1. **消息**：会话列表、未读数、最近消息预览；登录后默认通过 `GET /conversations/seqs`、`GET /conversations/:conversation_id/messages` 拉真实后端消息，并通过 `POST /messages` 发送。
2. **联系人**：新的朋友、群聊、标签、公众号入口；支持 identifier 搜索用户、添加好友动作、刷新好友列表，均走真实 `user/friends` REST adapter。
3. **发现**：朋友圈、扫一扫、小程序等发现入口为明确的 `MVP 占位`；不会伪造真实扫码/内容生态能力。
4. **我的**：个人资料卡、用户详情、服务、收藏、朋友圈、设置入口；支持编辑 `display_name`、`gender`、`age`、`region` 等可变资料字段，并支持退出登录。

当前 `web/src/api/{user,contacts,groups,messages}.ts` 均基于统一 `createApiClient` 封装 REST contract，共享 envelope 解析、错误处理和 bearer token 注入。认证页调用真实 `/auth/login` 与 `/auth/register`；我的页通过 typed user API adapter 调用 `PATCH /me` 更新资料。

## 消息页边界

- `web/src/features/messages/MessagesPage.tsx` 默认是真实 API 页面；不再支持 `mode="mock"` 或本地 mock ACK。
- 会话种子来自后端 `getConversationSeqs` 返回的 `states/conversations/seqs`；如果没有会话，页面显示“暂无会话”，不自动插入假会话。
- 发送消息先追加本地 `sending` UI 状态，但最终状态必须来自 `messageApi.sendMessage -> POST /messages` 的真实返回；失败会显示错误，不静默兜底为成功。
- 聊天窗口展示已确认消息时按服务端 `conversationId + seq` 排序，不按 `sendTime`、fetch 数组顺序或 WebSocket 到达顺序排序；重复消息按 `serverMsgId` / `clientMsgId` 去重。
- 本地 optimistic 消息在服务端确认后必须用相同 `clientMsgId` 替换为 canonical server message，并保留 `serverMsgId`、`seq` 等服务端字段；没有 `seq` 的本地 pending 消息排在已确认消息之后。
- 同一会话内发送请求未完成时，composer 显示 `发送中` 并禁用输入/按钮；失败消息保留 `发送失败` 状态，不伪造成功。
- `web/src/models/messages.ts` 定义前端会话与消息模型，发送状态仅用于本地 UI 呈现。
- `web/src/api/messages.ts` 是消息 REST 薄 adapter，函数签名覆盖 `sendMessage`、`pullMessages`、`getConversationSeqs`、`markRead`，字段名保持与前后端合约一致，并基于统一 `createApiClient`。
- `web/src/api/websocketClient.ts` 是 WebSocket client wrapper，提供 `connect`、`send`、`close`，浏览器侧使用 `/ws?token=***` query fallback，并将后端 snake_case ACK 解析为 typed frontend ACK。

## 目录

```text
web/
  index.html
  package.json
  src/
    api/
      client.ts          # typed REST API client，支持 envelope 解析与 Authorization header
      contacts.ts        # friends REST typed adapter
      groups.ts          # groups REST typed adapter
      messages.ts        # message REST typed adapter
      shared.ts          # 兼容 helper，内部委托统一 REST client
      user.ts            # typed /me API adapter，PATCH payload 只允许可变字段
      websocketClient.ts # WebSocket command/ACK wrapper
    auth/
      AuthContext.tsx    # 轻量认证状态和登录/注册/退出动作
      session.ts         # localStorage session 工具
    components/
      ContactsPage.tsx   # 联系人、identifier 搜索和加好友真实 API UI
      ui/                # TabBar、TopBar、ListCard、Avatar、SearchBox、ActionRow 等共享 UI
    features/
      messages/          # 消息页和聊天窗口真实 API UI
    models/
      messages.ts        # frontend message/conversation models
    pages/               # DiscoverPage、MePage
    App.tsx              # 认证入口、四 Tab shell 和页面路由
    App.test.tsx         # 认证、主导航、联系人/加好友、消息页、发现占位和我的页编辑行为测试
    main.tsx
    styles.css
```

## 认证与 API Client

- REST client 入口为 `web/src/api/client.ts`，默认同源请求；本地开发由 Vite proxy 将 `/auth`、`/me`、`/users`、`/friends`、`/messages`、`/conversations`、`/groups`、`/ws` 路由到对应后端微服务端口。
- 后端响应必须使用统一 envelope：`{ "code": "OK", "message": "ok", "data": {} }`。`code !== "OK"` 或 HTTP 非 2xx 时抛出 typed `ApiError`。
- 受保护接口由 client 注入 `Authorization: Bearer ***` token。
- MVP 认证状态使用 React Context 和 localStorage。保存内容限于 access token 与当前用户展示信息；遇到损坏 session 会清理并回到登录页。
- 未登录时显示登录/注册页；登录或注册成功后进入 `消息 / 联系人 / 发现 / 我的` 四 Tab。`我的` 页展示当前用户昵称、账号、地区和用户 ID，并提供退出登录。

## 本地命令

从仓库根目录执行：

```bash
npm install --prefix web
npm run frontend:dev
npm run frontend:test
npm run frontend:build
npm run frontend:lint
```

单机 E2E 前应使用根目录 Makefile 启动真实服务：

```bash
make start
make status
```

## 设计约定

- 主导航固定底部，使用四个 tab：`消息 / 联系人 / 发现 / 我的`。
- 移动端优先；桌面环境使用 phone frame 预览，方便后续做响应式适配。
- 视觉上参考微信：浅灰页面背景、白色列表卡片、绿色激活态、紧凑列表行。
- 联系人页入口固定为：`新的朋友 / 群聊 / 标签 / 公众号`。
- 好友列表来自 `GET /friends`；identifier 搜索来自 `GET /users/:identifier`；加好友来自 `POST /friends`。
- 不在前端生产代码中写入 mock 用户、mock 会话、真实 token、密码或后端 secret。
- 前端用户资料更新必须走 `web/src/api/user.ts`，只向 `PATCH /me` 发送可变字段，不发送 `user_id` 或 `identifier`。

## REST Adapter 约定

- `web/src/api/contacts.ts`：`listFriends` -> `GET /friends`，`addFriend` -> `POST /friends`，`deleteFriend` -> `DELETE /friends/:user_id`。
- `web/src/api/groups.ts`：`getGroup` -> `GET /groups/:group_id`，`createGroup` -> `POST /groups`，`joinGroup` -> `POST /groups/:group_id/members`，`leaveGroup` -> `DELETE /groups/:group_id/members/me`，`listMembers` -> `GET /groups/:group_id/members`。
- `web/src/api/messages.ts`：`sendMessage` -> `POST /messages`，`pullMessages` -> `GET /conversations/:conversation_id/messages`，`getConversationSeqs` -> `GET /conversations/seqs`，`markRead` -> `POST /conversations/:conversation_id/read`。
- Adapter 接受可注入 `fetcher` 和 bearer token；示例 token 只能使用 `***` 或测试 fixture 值。
