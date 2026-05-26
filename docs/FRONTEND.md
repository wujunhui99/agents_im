# FRONTEND.md

本文档记录 `agents_im` Web 前端约定。当前阶段已搭建微信风格主框架，并已把登录后的消息、联系人、加好友、发消息主流程切到真实 REST API。生产代码不再保留 mock/default demo 数据源；mock/fetch stub 仅允许存在于测试 fixture 中。

## 技术栈

- React + TypeScript
- Vite
- Vitest + Testing Library
- lucide-react 图标
- 原生 CSS + 轻量自研 Material 3-inspired UI layer，不引入官方 Material Web 或 MUI 等重型组件库

## 当前阶段范围

前端第一阶段参考微信主框架，完成四个一级页面：

1. **消息**：会话列表、未读数、最近消息预览；登录和恢复会话后默认通过 `GET /conversations/seqs`、`GET /conversations/:conversation_id/messages` 拉真实后端消息，不依赖先发送新消息，并通过 `POST /messages` 发送。无会话时支持通过 identifier 搜索用户并发起单聊。
2. **联系人**：新的朋友、群聊、标签、公众号入口；进入联系人页后自动调用 `GET /friends` 拉真实好友列表，`刷新好友` 仅作为失败后的手动重试；支持 identifier 搜索用户、添加好友动作，均走真实 `user/friends` REST adapter。群聊入口调用 `GET /groups` 列出本人 active 群聊，并可从已接受好友中选择成员调用 `POST /groups` 创建群聊。`标签 / 公众号` 入口在第一阶段明确标记为 `暂未开放`。
3. **发现**：朋友圈、扫一扫、小程序等发现入口为明确的 `MVP 占位`；不会伪造真实扫码/内容生态能力。
4. **我的**：个人资料卡、用户详情、服务、收藏、设置入口；支持编辑 `display_name`、`gender`、`birth_date`、`region` 等可变资料字段，并支持退出登录。朋友圈入口只保留在 `发现` 页。

当前 `web/src/api/{auth,user,contacts,groups,messages}.ts` 均基于统一 `createApiClient` 封装 REST contract，共享 envelope 解析、错误处理和 bearer token 注入。认证页调用真实 `/auth/login`、`/auth/register/email-code` 与 `/auth/register`；我的页通过 typed user API adapter 调用 `PATCH /me` 更新资料。

## Material 3-inspired 轻量设计系统

当前前端保留微信式四 Tab 产品方向：`消息 / 联系人 / 发现 / 我的`，视觉层重构为轻量自研的 Google Material Design 3-inspired 系统。该系统只使用原生 CSS variables 和本仓库 React 组件，不依赖 `@material/web`、`@mui/*` 或其他重型 UI 框架。

- 登录后的四个一级 Tab 在单次 SPA 会话内采用 lazy keepalive：首次打开后保持组件挂载，切换 Tab 只隐藏非当前面板，避免 `消息`/`联系人` 已加载状态被清空并重复请求；页面刷新或 App 重新挂载仍可重新从服务端加载。
- `web/src/styles/tokens.css` 定义 design tokens：颜色、surface / tonal roles、shape、spacing、typography、state layer、shadow/elevation。
- `web/src/styles.css` 引入 tokens 并按 app shell、components、pages 组织样式，兼容既有页面 class。
- `web/src/components/ui/` 提供轻量组件：`Button`、`Card`、`TextField`、`TopAppBar`、`NavigationBar`、`ListItem`、`Avatar`、`Badge`、`MessageBubble`，以及兼容入口 `TopBar`、`TabBar`、`ListCard`、`SearchBox`、`ActionRow`。
- 未实现入口继续显示明确 helper 或 `MVP 占位`，不能用视觉完成态暗示真实业务已实现。
- 前端 API 错误必须继续显式展示；生产代码不得为了 UI 演示切换到 mock/default demo fallback。

## 消息页边界

- `web/src/features/messages/MessagesPage.tsx` 默认是真实 API 页面；不再支持 `mode="mock"` 或本地 mock ACK。
- 会话种子来自后端 `getConversationSeqs` 返回的 `states/conversations/seqs`；如果没有会话，页面显示“暂无会话”和“发起聊天”动作，不自动插入假会话。
- “发起聊天”使用 `UserApi.getPublicProfileByIdentifier -> GET /users/:identifier` 搜索真实公开资料。选择搜索结果后只创建本地 `draft-single:{user_id}` 空会话，直到用户发送第一条消息才调用 `messageApi.sendMessage -> POST /messages` 并用服务端返回的 `conversationId` 替换本地 draft id。
- 发送消息先追加本地 `sending` UI 状态，但最终状态必须来自 `messageApi.sendMessage -> POST /messages` 的真实返回；失败会显示错误，不静默兜底为成功。
- 从发起聊天搜索结果创建的会话标题优先使用 `display_name / name / identifier`，并显示 profile 的 `account_type` 标签。从消息 API 直接加载的历史会话目前只包含内部 account id；前端不会伪造 `/users/id/{user_id}`，因此在后端提供按 user_id 查询公开资料的前端契约前，历史会话标题显示为 `未知联系人`，不能显示内部 ID。
- 群聊会话使用 `GET /groups/:group_id` 和 `GET /groups/:group_id/members` 水合群名称和成员显示名；非本人群消息展示成员 `display_name / name / identifier`，没有可读资料时显示 `群成员`，不展示内部 account id。
- 聊天窗口展示已确认消息时按服务端 `conversationId + seq` 排序，不按 `sendTime`、fetch 数组顺序或 WebSocket 到达顺序排序；重复消息按 `serverMsgId` / `clientMsgId` 去重。
- 本地 optimistic 消息在服务端确认后必须用相同 `clientMsgId` 替换为 canonical server message，并保留 `serverMsgId`、`seq` 等服务端字段；没有 `seq` 的本地 pending 消息排在已确认消息之后。
- 打开有未读的会话并展示到带 `seq` 的消息后，前端调用 `messageApi.markRead -> POST /conversations/:conversation_id/read` 推进 `hasReadSeq`，成功后立即清除已读范围内的本地未读标记；失败必须显示错误状态，不伪造成功。
- 双人单聊 header 展示 `AI 托管` 开关；进入会话时调用 `messageApi.getAIHosting -> GET /conversations/:conversation_id/ai-hosting` 读取持久化状态，开关变更调用 `messageApi.updateAIHosting -> PUT /conversations/:conversation_id/ai-hosting`。对端已开启时必须禁用当前用户开关并展示后端返回的中文原因；加载或更新失败必须显示错误和重试入口。群聊和本地 draft 会话不展示该控制。
- 会话列表合并重新加载结果时保留本地已确认的新消息、最新预览和已读进度，避免旧的 REST reload 把正在查看或刚发送后的会话回退成陈旧未读状态。
- 同一会话内发送请求未完成时，composer 显示 `发送中` 并禁用输入/按钮；失败消息保留 `发送失败` 状态，不伪造成功。
- 发出的已确认消息不再在气泡内显示 `已发送` 文案，而是在右下角显示紧凑对号：`✔` 表示发送成功，`✔✔` 表示已被当前可用的会话已读阈值覆盖。V1 先使用 `Conversation.hasReadSeq` 作为阈值；后端暴露对端 read receipt 后，前端应切换为精确的对端已读状态。
- `web/src/models/messages.ts` 定义前端会话与消息模型，发送状态仅用于本地 UI 呈现。
- `web/src/api/messages.ts` 是消息 REST 薄 adapter，函数签名覆盖 `sendMessage`、`pullMessages`、`getConversationSeqs`、`markRead`、`getAIHosting`、`updateAIHosting`，字段名保持与前后端合约一致，并基于统一 `createApiClient`。
- 消息模型包含 `messageOrigin: human | ai | system` 和 AI metadata；`MessagesPage` 必须用 `AI/Agent` 标签明显标注 `ai` 消息，系统消息使用系统标签。
- `web/src/api/websocketClient.ts` 是 WebSocket client wrapper，提供 `connect`、`send`、`close`，浏览器侧使用 `/ws?token=***` query fallback，并将后端 snake_case ACK 解析为 typed frontend ACK。

## 目录

```text
web/
  index.html
  package.json
  src/
    api/
      auth.ts            # 登录、注册邮箱验证码、注册 REST typed adapter
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
      ui/                # Material 3-inspired Button、Card、TextField、TopAppBar、NavigationBar、ListItem、Avatar、Badge、MessageBubble 等共享 UI
    features/
      messages/          # 消息页和聊天窗口真实 API UI
    models/
      messages.ts        # frontend message/conversation models
    pages/               # DiscoverPage、MePage
    styles/
      tokens.css         # design tokens：颜色、surface、shape、spacing、typography、state layer、elevation
    App.tsx              # 认证入口、四 Tab shell 和页面路由
    App.test.tsx         # 认证、主导航、联系人/加好友、消息页、发现占位和我的页编辑行为测试
    main.tsx
    styles.css
```

## 认证与 API Client

- REST client 入口为 `web/src/api/client.ts`，默认同源请求；本地开发由 Vite proxy 将 `/auth`、`/me`、`/users`、`/friends`、`/messages`、`/conversations`、`/groups`、`/ws` 路由到对应后端微服务端口。
- 后端响应必须使用统一 envelope：`{ "code": "OK", "message": "ok", "data": {} }`。`code !== "OK"` 或 HTTP 非 2xx 时抛出 typed `ApiError`。
- 受保护接口由 client 注入 `Authorization: Bearer *** token。
- MVP 认证状态使用 React Context 和 localStorage。保存内容限于 access token 与当前用户展示信息；遇到损坏 session 会清理并回到登录页。
- 未登录时显示登录/注册页；注册页要求填写邮箱、发送验证码、填写验证码后再提交 `/auth/register`。登录或注册成功后进入 `消息 / 联系人 / 发现 / 我的` 四 Tab。`我的` 页展示当前用户昵称、identifier、账号类型和地区，不展示内部 user/account ID，并提供退出登录。

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
- 移动端优先；桌面宽屏从 `900px` 起展开为全视口 Web shell，不再保持窄手机画幅，内容区域在宽屏下使用最大阅读宽度约束。
- 视觉上采用 Material 3-inspired surface 层级、tonal container、state feedback、elevation 和圆角节奏，同时保留微信式四 Tab 信息架构。
- 列表、卡片、按钮、输入框、导航、消息气泡等基础 UI 必须优先复用 `web/src/components/ui/` 的轻量组件和 `web/src/styles/tokens.css` tokens。
- 联系人页入口固定为：`新的朋友 / 群聊 / 标签 / 公众号`。
- 好友列表在联系人页挂载时自动来自 `GET /friends`；identifier 搜索来自 `GET /users/:identifier`；加好友来自 `POST /friends`。
- 不在前端生产代码中写入 mock 用户、mock 会话、真实 token、密码或后端 secret。
- 前端用户资料更新必须走 `web/src/api/user.ts`，只向 `PATCH /me` 发送可变字段，不发送 `user_id` 或 `identifier`。
- 默认禁止新增 `@material/web`、`@mui/*` 等重依赖；如未来必须引入，需要先在执行计划和本文档中记录原因、替代方案与验证结果。

## Management System 约定

- `ms.agenticim.xyz` 是 **AgenticIM Management** 入口，不再使用 `Admin Console` 作为面向用户的主品牌。
- Loki、Tempo 不新增独立公网域名；MS 提供实际路由跳转入口：
  - `/observability/logs` -> 302 到 Grafana Explore / Loki datasource（使用 `schemaVersion=1&panes=...` 明确 `uid=loki`，不要用旧 `left={datasource:...}`）
  - `/observability/traces` -> 302 到 Grafana Explore / Tempo datasource（使用 `schemaVersion=1&panes=...` 明确 `uid=tempo` + `queryType=traceql`，避免 Grafana fallback 到上次/默认 Loki）
  - `/observability/metrics` -> 直接承接受保护的 Prometheus UI，不再暴露 `prometheus.agenticim.xyz`
  - `/observability/llm` -> 302 到 `langfuse.agenticim.xyz`
- `langfuse.agenticim.xyz` 保留独立域名，MS 只提供跳转入口。
- Management System 的数据列表必须有表头或字段 label，不能只展示一组无字段名的值。
- Management System 的 SPA 页面路由可以使用 `/admin/*`；新增/易冲突的数据接口应使用明确 JSON API 前缀，当前反馈管理数据使用 `/api/admin/feedback`，任务报告使用 `/api/admin/task-reports`，避免 `/admin/*` 页面路由被误当成 API。

## REST Adapter 约定

- `web/src/api/contacts.ts`：`listFriends` -> `GET /friends`，`addFriend` -> `POST /friends`，`deleteFriend` -> `DELETE /friends/:user_id`。
- `web/src/api/groups.ts`：`listGroups` -> `GET /groups`，`getGroup` -> `GET /groups/:group_id`，`createGroup` -> `POST /groups`（支持 `member_user_ids`），`joinGroup` -> `POST /groups/:group_id/members`，`leaveGroup` -> `DELETE /groups/:group_id/members/me`，`listMembers` -> `GET /groups/:group_id/members`。
- `web/src/api/messages.ts`：`sendMessage` -> `POST /messages`，`pullMessages` -> `GET /conversations/:conversation_id/messages`，`getConversationSeqs` -> `GET /conversations/seqs`，`markRead` -> `POST /conversations/:conversation_id/read`，`getAIHosting` -> `GET /conversations/:conversation_id/ai-hosting`，`updateAIHosting` -> `PUT /conversations/:conversation_id/ai-hosting`。
- `web/src/api/admin.ts`：反馈管理使用 `GET /api/admin/feedback`、`GET /api/admin/feedback/:feedback_id`、`PATCH /api/admin/feedback/:feedback_id`，任务报告使用 `GET /api/admin/task-reports`；不要用 `/admin/feedback` 或 `/admin/task-reports` 作为 JSON fetch 目标。
- Adapter 接受可注入 `fetcher` 和 bearer token；示例 token 只能使用 `***` 或测试 fixture 值。
