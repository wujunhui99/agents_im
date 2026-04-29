# FRONTEND.md

本文档记录 `agents_im` Web 前端约定。当前阶段已搭建微信风格主框架，并接入认证入口、typed REST API client 基础、联系人/群聊 UI、消息页/WebSocket client 基础、共享 UI 组件与个人资料编辑；第一波技术债治理已统一 REST adapters 与 Vite 多服务 dev proxy，后续继续逐步接入完整联系人/群聊远端状态、重连补消息和完整已读状态。

## 技术栈

- React + TypeScript
- Vite
- Vitest + Testing Library
- lucide-react 图标
- 原生 CSS，先不引入 UI 组件库，避免过早绑定设计系统

## 当前阶段范围

前端第一阶段参考微信主框架，完成四个一级页面：

1. **消息**：会话列表、未读数、最近消息预览；支持移动端进入聊天窗口、返回列表、文本发送 composer，以及 `sending` / `sent` / `failed` 基础状态。
2. **联系人**：新的朋友、群聊、标签、公众号入口；支持 identifier 搜索、添加好友动作、群聊列表、创建群、加入群和群详情成员列表的 UI 占位。
3. **发现**：朋友圈、扫一扫、小程序等发现入口占位，全部标记为 `MVP 占位`；扫一扫当前不启动真实扫码能力。
4. **我的**：个人资料卡、用户详情、服务、收藏、朋友圈、设置入口；支持编辑 `display_name`、`gender`、`age`、`region` 等可变资料字段，并支持退出登录。

当前 `web/src/api/{user,contacts,groups,messages}.ts` 均基于统一 `createApiClient` 封装 REST contract，共享 envelope 解析、错误处理和 bearer token 注入。认证页调用真实 `/auth/login` 与 `/auth/register`；我的页通过 typed user API adapter 调用 `PATCH /me` 更新资料。消息页支持显式 `mode="real"`：真实模式通过 `pullMessages` 拉取会话消息，并通过 `sendMessage -> POST /messages` 发送；mock ACK 只保留在 `mode="mock"`/测试/demo 数据中。联系人/群聊页面的完整远端状态机仍是后续工作，但 API adapters 已对齐真实后端路径。WebSocket client wrapper 已提供 connect/send/close 与 ACK 解析基础；重连补消息、去重缓存和完整已读状态后续继续按同一契约接入。

## 消息页边界

- `web/src/features/messages/` 持有消息页组件和 demo 会话种子，默认 `mode="mock"` 只用于前端骨架/测试演示；真实联调必须显式使用 `mode="real"` 并注入/创建 `MessageApi`。
- `web/src/models/messages.ts` 定义前端会话与消息模型，发送状态仅用于本地 UI 呈现。
- `web/src/api/messages.ts` 是消息 REST 薄 adapter，函数签名覆盖 `sendMessage`、`pullMessages`、`getConversationSeqs`、`markRead`，字段名保持与前后端合约一致，并基于统一 `createApiClient`。
- `web/src/api/websocketClient.ts` 是 WebSocket client wrapper，提供 `connect`、`send`、`close`，浏览器侧使用 `/ws?token=***` query fallback，并将后端 snake_case ACK 解析为 typed frontend ACK。
- mock sender 会先追加 `sending` 消息，再模拟 ACK 更新为 `sent`；输入 `/fail` 可进入 `failed` 状态用于本地验收。该路径不能作为真实 API 验收证据。

## 目录

```text
web/
  index.html
  package.json
  src/
    api/
      client.ts          # typed REST API client，支持 envelope 解析与 Authorization header
      client.test.ts
      contacts.ts        # friends REST typed adapter
      groups.ts          # groups REST typed adapter
      messages.ts        # message REST typed adapter
      shared.ts          # REST envelope/request helper
      user.ts            # typed /me API adapter，PATCH payload 只允许可变字段
      user.test.ts       # user adapter payload 过滤测试
      social.test.ts     # contacts/groups adapter 测试
      websocketClient.ts # WebSocket command/ACK wrapper
      websocketClient.test.ts
    auth/
      AuthContext.tsx    # 轻量认证状态和登录/注册/退出动作
      session.ts         # localStorage session 工具
    components/
      ContactsPage.tsx   # 联系人、好友搜索、群聊列表和群详情 UI
      ui/                # TabBar、TopBar、ListCard、Avatar、SearchBox、ActionRow 等共享 UI
    data/
      mockData.ts        # MVP mock 会话、联系人、发现入口和当前用户
    features/
      messages/          # 消息页、聊天窗口和 mock conversations
    models/
      messages.ts        # frontend message/conversation models
    pages/               # DiscoverPage、MePage
    App.tsx              # 认证入口、四 Tab shell 和页面路由
    App.test.tsx         # 认证、主导航、联系人/群聊、消息页、发现占位和我的页编辑行为测试
    main.tsx
    styles.css
```

## 认证与 API Client

- REST client 入口为 `web/src/api/client.ts`，默认同源请求；需要跨域联调时使用 Vite env `VITE_API_BASE_URL` 覆盖，例如 `VITE_API_BASE_URL=http://127.0.0.1:8081`。
- 后端响应必须使用统一 envelope：`{ "code": "OK", "message": "ok", "data": {} }`。`code !== "OK"` 或 HTTP 非 2xx 时抛出 typed `ApiError`。
- 受保护接口由 client 注入 `Authorization: Bearer ***`。前...记录真实 token。
- MVP 认证状态使用 React Context 和 localStorage，key 为示例/占位值。保存内容限于 access token 与当前用户展示信息；遇到损坏 session 会清理并回到登录页。
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

或直接进入 `web/`：

```bash
cd web
npm install
npm run dev
npm run test:run
npm run build
npm run lint
```

## 设计约定

- 主导航固定底部，使用四个 tab：`消息 / 联系人 / 发现 / 我的`。
- 移动端优先；桌面环境使用 phone frame 预览，方便后续做响应式适配。
- 视觉上参考微信：浅灰页面背景、白色列表卡片、绿色激活态、紧凑列表行。
- 联系人页入口固定为：`新的朋友 / 群聊 / 标签 / 公众号`。
- 好友列表按首字母分组展示；搜索占位按唯一 `identifier` 精确匹配本地 mock 用户。
- 群聊 UI 当前使用本地 mock 数据，支持创建群、输入群 ID 加入群、查看成员列表。
- 不在前端代码中写入真实 token、密码或后端 secret。
- 前端用户资料更新必须走 `web/src/api/user.ts`，只向 `PATCH /me` 发送可变字段，不发送 `user_id` 或 `identifier`。

## REST Adapter 约定

- `web/src/api/contacts.ts`：`listFriends` -> `GET /friends`，`addFriend` -> `POST /friends`，`deleteFriend` -> `DELETE /friends/:user_id`。
- `web/src/api/groups.ts`：`getGroup` -> `GET /groups/:group_id`，`createGroup` -> `POST /groups`，`joinGroup` -> `POST /groups/:group_id/members`，`leaveGroup` -> `DELETE /groups/:group_id/members/me`，`listMembers` -> `GET /groups/:group_id/members`。
- `web/src/api/messages.ts`：`sendMessage` -> `POST /messages`，`pullMessages` -> `GET /conversations/:conversation_id/messages`，`getConversationSeqs` -> `GET /conversations/seqs`，`markRead` -> `POST /conversations/:conversation_id/read`。
- Adapter 接受可注入 `fetcher` 和 bearer token；示例 token 只能使用 `***` 或 mock 值。
