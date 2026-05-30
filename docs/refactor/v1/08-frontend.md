# 08 — 前端重构分析

> 目标：盘点 `web/` 前端代码，识别巨石组件、缺失基础设施、契约不一致；给出收敛建议。
>
> 范围：`web/src/{App,api,auth,components,features,pages,models,utils,styles}.{ts,tsx,css}`、`web/{package.json,vite.config.ts,tsconfig.json,eslint.config.js,nginx/}`。
>
> **路径约定**：本文以 `web/src/` 为前缀的路径都是 **agents_im 当前真实位置**。重构后建议落地的目录方案见 §3 目标布局。
>
> 关联：00-decisions（无直接路径冲突）；与后端契约相关处会引用 00-decisions D6（wire format）。

---

## 1. 现状速描

### 1.1 技术栈

| 项                | 版本               | 备注                                     |
|-------------------|--------------------|------------------------------------------|
| React             | `^19.2.5`          | 最新 stable                              |
| Vite              | `^8.0.10`          | ⚠️ **Vite 还没出 8.0**（当前 5.x 稳，6.x dev）；package.json 写的版本号实际不存在或是 fork |
| TypeScript        | `^6.0.3`           | ⚠️ **TS 6 还没发布**（当前 5.x）；同上    |
| lucide-react      | `^1.14.0`          | ⚠️ lucide-react 仍在 0.x 大版本；1.x 不存在 |
| Vitest            | `^4.1.5`           | 同上版本号疑似 fake/越界                 |
| @vitejs/plugin-react | `^6.0.1`        | 同上                                      |
| Testing Library   | `@testing-library/react@^16.3.2` | OK                       |
| ESLint            | `^10.2.1`          | ⚠️ ESLint 当前 9.x；10.x 不存在           |

> **依赖版本号集体疑似 fake** — 这些都不是当前真实发布版本。package.json 必须先 audit 一次，跑 `npm ls react vite typescript lucide-react` 确认实际锁定版本（看 package-lock.json）。如果是误写的 ^1 应该改回真实的范围。**这是 P0 安全/可重现构建问题**。

### 1.2 代码规模分布

`web/src/` 总计 **8167 行**（非测试），但**极度不均**：

| 文件                                    | 行数  | 性质                                       |
|----------------------------------------|-------|--------------------------------------------|
| `features/messages/MessagesPage.tsx`   | **2552** | 🚨 IM 主页面，含 10 个子组件、51 个 hooks |
| `pages/AdminConsole.tsx`               | **1193** | 🚨 admin 后台单文件，30+ useState         |
| `components/ContactsPage.tsx`          | **809**  | 🚨 联系人页 + 8 个子组件                  |
| `App.tsx`                              | **668**  | App + AuthGate + AuthPage + renderPage 全在一起 |
| `api/websocketClient.ts`               | 396   | 自实现 WS client + reconnect              |
| `pages/AdminConsole.test.tsx` etc.    | ~      | 测试                                      |
| `api/admin.ts`                         | 314   | admin REST 客户端                          |
| `pages/FeedbackPage.tsx`               | 245   |                                            |
| `api/client.ts`                        | 204   | fetch wrapper                              |
| `pages/MePage.tsx`                     | 204   |                                            |
| `utils/avatarUpload.ts`                | 208   |                                            |
| `auth/AuthContext.tsx`                 | 180   | 唯一的全局 context                         |
| `api/messages.ts`                      | 163   |                                            |
| ...                                    | ...   |                                            |

`web/src/styles.css` **2652 行** / **473 个 CSS selector** / **269 个 class** — 单文件全局样式。

### 1.3 目录结构

```
web/src/
├── App.tsx                      # ⚠️ 巨石入口（含 routing + auth gate + tab shell）
├── main.tsx                     # 仅 mount
├── styles.css                   # 2652 行单文件
├── styles/tokens.css            # design tokens（OK）
├── api/                         # 8 个 per-domain API 模块
│   ├── client.ts                # fetch wrapper（自实现）
│   ├── websocketClient.ts       # WS 客户端
│   ├── shared.ts                # 21 行公共
│   ├── auth.ts user.ts contacts.ts groups.ts messages.ts media.ts feedback.ts admin.ts
│   └── *.test.ts                # 7 个 api 测试
├── auth/                        # 唯一的全局 context
│   ├── AuthContext.tsx
│   └── session.ts
├── components/
│   ├── ContactsPage.tsx         # 809 行（按文件名应该在 pages/）
│   └── ui/                      # 14 个原子组件（OK，干净）
├── features/messages/
│   └── MessagesPage.tsx         # 2552 行（唯一的 feature 目录）
├── pages/
│   ├── AdminConsole.tsx         # 1193 行
│   ├── FeedbackPage.tsx
│   ├── MePage.tsx
│   └── DiscoverPage.tsx         # 33 行 placeholder
├── models/
│   └── messages.ts              # 类型定义
└── utils/
    ├── avatarUpload.ts
    └── profileDisplay.ts
```

### 1.4 路由 / 状态 / 表单 / 样式四大件

| 维度       | 现状                                                                                       |
|-----------|--------------------------------------------------------------------------------------------|
| 路由       | **手工** `isAdminRoute()` + `appRouteFromLocation()` + `popstate` listener + `pushState`；3 个层次：admin host / appRoute (`main`/`feedback`) / tab (`messages`/`contacts`/`discover`/`me`） |
| 状态       | 全部 `useState`；唯一全局是 `AuthProvider`；其它跨组件靠 props drilling（看 `renderPage` 18 个参数） |
| 表单       | 手工 useState + onChange + 手写 validation；AuthPage 一个表单 7+ useState                  |
| 样式       | 单文件 `styles.css` 2652 行；269 个全局 class；命名靠约定（`.admin-feedback-attachment-card`） |

---

## 2. 技术债

### FE-1 🚨 MessagesPage.tsx 2552 行单文件

**内部 10 个组件**全部塞在一个文件：
```
MessagesPage              # 顶层
├── ConversationList      (557~)
├── StartChatPanel        (627~)
├── ChatWindow            (726~)
├── MessageDateSeparator  (831~)
├── GroupManagementPanel  (841~)
├── AIHostingControl      (1039~)
├── FileMessageBubble     (1116~)
├── ImageMessageBubble    (1193~)
├── ImagePreviewDialog    (1352~)
└── SendMessageComposer   (1394~)
```

**51 个 hooks** 散落各组件；MessagesPage 自身就有 7 个 useState + 1 个 useRef。

**后果**：
- 编辑器/IDE 加载慢、navigation 困难；
- 任何小改动都会让 PR review 找不到重点；
- 测试只能写"端到端"，不能针对子组件；
- HMR 慢，开发体验差。

> **修复方向**：
> ```
> features/messages/
> ├── MessagesPage.tsx          # 顶层 ≤ 100 行：组合 + 路由
> ├── components/
> │   ├── ConversationList.tsx
> │   ├── StartChatPanel.tsx
> │   ├── ChatWindow.tsx
> │   ├── GroupManagementPanel.tsx
> │   ├── AIHostingControl.tsx
> │   ├── SendMessageComposer.tsx
> │   └── bubbles/
> │       ├── MessageDateSeparator.tsx
> │       ├── FileMessageBubble.tsx
> │       ├── ImageMessageBubble.tsx
> │       └── ImagePreviewDialog.tsx
> ├── hooks/
> │   ├── useConversations.ts          # items + status + selected
> │   ├── useMessageWebSocket.ts       # WS lifecycle + 重连
> │   ├── useAIHostingState.ts
> │   ├── useReadReceipt.ts
> │   └── useMediaUpload.ts
> ├── state/                            # zustand store（见 FE-3）
> └── types.ts
> ```

### FE-2 🚨 AdminConsole.tsx 1193 行 + 30+ useState

`AdminConsole` 单组件持有 30+ useState：dashboard / traces / conversation / users / feedback / task reports 全部页面级状态。

**后果**：
- 任何 view 切换都要在巨大状态空间里串字段；
- 不能单独测试某个 admin 子页面；
- 状态相互依赖（traceList ↔ selectedTrace ↔ traceLoading）边界模糊。

> 修复：按 admin view 拆 `pages/admin/{Dashboard,LLMTraces,Conversations,Users,Feedback,TaskReports}.tsx`，AdminConsole 只剩 router shell。

### FE-3 🚨 没有全局状态管理 / 服务端状态缓存

跨组件状态全靠 props drilling。App.tsx 里 `renderPage(...)` 参数 18 个：
```
renderPage(
  tab, currentUser, onUpdateProfile, onLogout, userApi, contactsApi,
  groupsApi, messageApi, mediaApi, onUploadAvatar, onOpenFeedback,
  webSocketUrl, webSocketToken, webSocketFactory, onAuthFailure,
  startChatSignal, pendingChatProfile, pendingGroup,
  onPendingChatConsumed, onPendingGroupConsumed,
  onOpenChatFromContact, onOpenGroupFromContact,
)
```

服务端数据（会话列表、好友、群成员、未读 seq）每次组件 mount 都重新 fetch，无缓存层。`useMemo(() => createMessageApi(...))` 只是 memo 一个客户端实例，不缓存请求结果。

> **修复方向**：
> 1. **服务端状态**：引入 **TanStack Query（react-query v5）**。每个 API 方法对应一个 query/mutation，自动缓存 + revalidate + optimistic update + retry。
> 2. **客户端状态**：引入 **Zustand**（轻量）管理跨页面的：当前选中会话、未读计数、WS 连接状态、AI hosting 偏好等。
> 3. 删除 props drilling — 子组件直接 `useQuery(...)` / `useStore(...)`，App.tsx props 表能缩到 3~5 个。

### FE-4 🚨 没有路由库（手工 window.location 派遣）

App.tsx 有 3 个互不协调的"路由"：
- `isAdminRoute()` — 判断 admin host / path；
- `appRouteFromLocation()` — `feedback` vs `main`；
- `activeTab` 切换 — 4 个一级 tab；
- AdminConsole 内部还有 `activeView` 5 个 view。

历史 stack 用 `window.history.pushState` 自己维护，`popstate` listener 处理回退。**4 层路由 4 套实现**。

后果：
- 深链接难（`/admin/users/123/conversations`）；
- 浏览器前进/后退极容易错乱；
- 不能基于路由 code-splitting（`React.lazy` + Suspense）。

> **修复方向**：引入 **React Router v6** 或 **TanStack Router**。建议路由表：
> ```
> /                              → tabs (默认 /messages)
> /messages                      → MessagesPage
> /messages/:conversationId      → ChatWindow（深链接打开）
> /contacts                      → ContactsPage
> /contacts/groups/:groupId      → GroupDetail
> /discover                      → DiscoverPage
> /me                            → MePage
> /feedback                      → FeedbackPage
> /admin                         → AdminConsole
> /admin/dashboard               → Dashboard
> /admin/llm-traces              → LLMTraces
> /admin/users/:userId/...
> /auth/login | /auth/register   → AuthPage
> ```
> 让 App.tsx 退化为 `<RouterProvider>` 包装，~50 行。

### FE-5 🚨 表单状态手工搓

AuthPage 一个登录/注册表单：
- 7 个 useState（identifier / email / code / displayName / password / error / emailCodeFeedback / sendingEmailCode / identifierCheckFeedback / submitting）；
- 一个 useRef 做 race condition guard（`identifierCheckRequest`）；
- canSubmitRegister / isSubmitDisabled 手工算；
- 切换 mode 时手工 reset 一堆 state。

FeedbackPage、MePage profile 编辑、群管理面板都重复这个模式。

> **修复方向**：引入 **react-hook-form** + **zod**（schema 校验）。AuthPage 应能从 7 个 useState 缩到 1 个 useForm。

### FE-6 🚨 styles.css 2652 行单文件 / 269 个全局 class

所有样式平铺一个文件，靠约定避免冲突（`.admin-feedback-attachment-card` / `.auth-frame` / `.message-bubble`）。新增组件要在巨大文件里找位置加 class，rename 极易漏。

引入了 `styles/tokens.css`（design tokens）是好的，但下游 CSS 还是全局。

> **修复方向**（三选一，由轻到重）：
> 1. **CSS Modules**（最轻）：每个组件配一个 `.module.css`，class 名局部化；保留 token 不变。
> 2. **Tailwind CSS v4**（最主流）：原子化 utility class，删 styles.css。
> 3. **vanilla-extract / Panda CSS**（CSS-in-TS）：类型安全 + token 复用，但学习成本高。
>
> 建议 **CSS Modules** —— 最小改动落地，与 tokens.css 配合无缝。

### FE-7 🚨 与后端契约的命名风格不一致

前端 API 类型出现两种风格：

```ts
// api/messages.ts —— camelCase
export type ServerMessage = {
  serverMsgId: string;
  clientMsgId: string;
  conversationId: string;
  hasReadSeq?: number;
  ...
};

// api/user.ts —— snake_case
export type UserProfile = {
  user_id: string;
  display_name: string;
  account_type?: 'user' | 'agent' | 'admin';
  avatar_url?: string;
  ...
};
```

后端 JSON envelope 用 snake_case（`{code, message, data}`），proto 字段也是 snake_case；但 messages.ts 强行用 camelCase 接受值——说明要么后端 msg-api 单独转了 camel，要么前端有隐式适配层。

后果：
- 同一个 user_id 字段，跨 model 时叫法不同（`user_id` vs `userId`）；
- `App.tsx` 里有 `userProfileFromAuth()` / `authUserFromProfile()` 两个手写映射函数，做 camel↔snake 转换；
- 新增字段时双向都要改。

> **修复方向**：
> 1. **统一前端用 snake_case**：直接吃后端 JSON，删 `messages.ts` 里的 camel 重命名；删 App.tsx 里的转换函数。
> 2. 或：**统一前端用 camelCase**：在 `api/client.ts` 层统一做 `snake_case → camelCase` 转换（递归 walker），全前端 camelCase。
> 3. 任选其一，文档化在 `web/README.md` 或 `docs/FRONTEND.md`。
>
> 与 03 文档 §4.3 ACK 语义变化对应（00-decisions D2 SendMessage ACK 去掉 seq）—— 这次重构契约必然要动，**借机做契约清理**最划算。

### FE-8 ⚠️ 自实现 fetch wrapper + WS client，不带 cache / retry

- `api/client.ts` 204 行手写 fetch wrapper（`createApiClient`、`ApiClient.request/get/post/...`）；OK，但没有：
  - 请求级 retry（断网/5xx）；
  - 并发去重（同 url 同时请求合并）；
  - cache stale-while-revalidate。
- `api/websocketClient.ts` 396 行手写 WS client，含 heartbeat + ack tracking + auth failure detection。

> **修复方向**：
> - HTTP：保留 fetch wrapper 但**用 react-query 包一层**（FE-3）；retry / cache / dedup 全由 react-query 解决。
> - WS：保留自实现（IM 场景定制需求多），但 hook 化为 `useMessageWebSocket()`，state 通过 zustand 暴露给所有页面（不再 props drill）。

### FE-9 ⚠️ App.tsx 668 行混合多种职责

```
App (5 行 wrapper)
├── AuthGate (40 行: 鉴权路由 + admin route)
├── AuthenticatedApp (200 行: tab shell + 18 props 转发)
├── AuthPage (200 行: 登录/注册表单)
├── renderPage (60 行 helper: 18 参数 switch tab)
├── isAdminRoute / appRouteFromLocation (2 个 helper)
├── userProfileFromAuth / authUserFromProfile (snake↔camel 转换)
```

> **修复方向**：拆为：
> ```
> App.tsx                  ≤ 50 行：RouterProvider + AuthProvider + QueryProvider
> features/auth/AuthPage.tsx
> features/shell/AppShell.tsx        # tab 切换 + TopBar
> features/shell/AuthGate.tsx        # 路由级鉴权
> ```

### FE-10 ⚠️ tab 切换状态保留 hack

App.tsx 用 `mountedTabs: Set<TabKey>` 跟踪已经 mount 过的 tab，未 mount 的 tab 直接不渲染；已 mount 的 tab 切换时用 `hidden` + `inert` 隐藏但保留 DOM。

```tsx
<section
  hidden={!isActive}
  inert={isActive ? undefined : true}
  aria-hidden={isActive ? undefined : true}
>
```

意图：tab 切换时不重新加载会话列表 / 联系人。

后果：DOM 树永久膨胀（4 个 tab 全部留在 DOM），无 unmount，无析构。

> **修复方向**：把"不要每次 mount 都 refetch"的需求交给 **react-query**（`staleTime: Infinity` / `gcTime: 5min`）。tab 切换可以正常 unmount → react-query 自动从 cache 恢复数据，无 flash。这是引入 react-query 后免费拿到的收益。

### FE-11 ⚠️ vite.config.ts proxy 表手工 8 条

```ts
'/admin/dashboard': { target: httpTarget('MESSAGE_API_PORT', 8083) },
'/admin/llm-traces': { ... },
'/admin/conversations': { ... },
'/auth': { ... AUTH_API_PORT 8081 },
'/messages': { ... MESSAGE_API_PORT 8083 },
'/users': { ... USER_API_PORT 8080 },
'/friends': { ... FRIENDS_API_PORT 8082 },
'/groups': { ... GROUPS_API_PORT 8085 },
'/ws': { ... GATEWAY_WS_PORT 8084 },
```

每次后端加新服务（`push`、`agent-api`、`admin-api`）要改这里。耦合死。

> **修复方向**：根据 01 文档拆出 `service/admin/api`（admin-api）、`service/agent/api`（agent-api）后，proxy 表会更长。建议改用 **单 API gateway**（nginx 或本地一个 Node sidecar），dev 时前端只 proxy `/api/*` → gateway，gateway 内部路由。生产 nginx 已有这层（`nginx/default.conf` 是 SPA serve），可以同份配置加 backend routing。

### FE-12 🟡 lucide-react 图标使用

App.tsx import `MessageCircle, Contact, Compass, UserRound, ShieldCheck`，看似 OK。但 lucide-react 全量打包很大，应该确认 tree-shake 工作。

> 修复：build 后跑 `vite-bundle-visualizer` 看 lucide-react 实际体积；按需可改为 `lucide-react/icons/<name>` 显式 import。

### FE-13 🟡 测试覆盖颗粒度不一

- `api/*.test.ts` 7 个：OK；
- 主要页面都有 `.test.tsx`；
- 但 MessagesPage 2552 行**只有一个测试文件** → 测试只能覆盖宏观路径，子组件没法单独 assert。
- `realIntegration.test.ts` 一份做真实集成（？要确认是否要联真实后端）。

> 修复：FE-1 拆完后，每个子组件配独立测试；加 Playwright / Cypress E2E 覆盖关键路径（登录、发消息、撤回、群管理、admin 操作）。

### FE-14 🟡 ESLint config 简单 + 无 Prettier / Husky

- `eslint.config.js` 只用了 `js.configs.recommended` + `tseslint.configs.recommended`；
- 没有 `eslint-plugin-react-hooks`（漏检 deps）；
- 没有 `eslint-plugin-react-refresh`；
- 没有 Prettier；
- 没有 `husky` + `lint-staged` pre-commit hook；
- `lint --max-warnings=0` 强制无 warning OK。

> 修复：补这几个 plugin + Prettier + husky；CI 加 type-check + lint。

### FE-15 🟡 ContactsPage.tsx 放 `components/` 而非 `pages/`

按文件名应该在 `pages/`。`components/ContactsPage.tsx` 是历史遗留。

> 修复：搬到 `pages/contacts/ContactsPage.tsx`（FE-1 同结构）。

### FE-16 🟡 `models/messages.ts` 只有消息 model，其它域无 model 层

```
src/models/
└── messages.ts    # ChatMessage, Conversation, MessageContentType, MessageStatus
```

friends / groups / users 的领域类型散落在 `api/<domain>.ts`。

> 修复：建立完整 `models/` 层（与 02 CP-4 domain model 思路一致）：
> ```
> models/
> ├── messages.ts
> ├── users.ts            # UserProfile, AuthUser
> ├── friends.ts          # Friendship, FriendRequest
> ├── groups.ts           # Group, GroupMember
> └── media.ts
> ```
> `api/<dom>.ts` 只放请求函数 + DTO（对应后端 JSON），转换为 `models/<dom>.ts` 的 domain 类型时由 api 层做。

### FE-17 🟡 `.deploy-trigger` 文件

仓库根 `web/.deploy-trigger` 是空 trigger，用于触发 CI 检测 web 变更（见 `scripts/detect-deploy-changes.py`）。这是 hack，不优雅。

> 修复：detector 改为按 `web/**` 真实 git diff 判断（已经能做），删 `.deploy-trigger`。

### FE-18 🟡 nginx 配置只有 SPA serve

`nginx/default.conf` 只配了静态资源 + SPA fallback，**没有 API proxy**——意味着前端在生产环境必须依赖**外部** ingress / 公网域名打到后端各端口。

> 现状是否 OK 取决于 k8s ingress 配置（`deploy/k8s/ingress.yaml`）。建议补一份关系图说明清楚 `dev proxy → prod ingress` 的等价路径。

---

## 3. 目标布局

```
web/
├── package.json                  # 修复版本号
├── vite.config.ts                # 简化 proxy → 单 /api 前缀
├── tsconfig.json
├── eslint.config.js              # 补 plugin
├── .prettierrc                   # 新增
├── nginx/default.conf            # 加 API ingress 注释
├── public/
└── src/
    ├── main.tsx                  # ≤ 20 行
    ├── App.tsx                   # ≤ 50 行：Router+QueryClient+Auth
    │
    ├── app/                      # 应用骨架
    │   ├── router.tsx            # React Router 路由表
    │   ├── providers.tsx         # QueryClient / Toast / ErrorBoundary
    │   └── shell/
    │       ├── AppShell.tsx      # Tab + TopBar 框架
    │       ├── AuthGate.tsx
    │       └── AdminGate.tsx
    │
    ├── features/                 # 按业务域分包
    │   ├── auth/
    │   │   ├── LoginForm.tsx
    │   │   ├── RegisterForm.tsx
    │   │   ├── AuthPage.tsx
    │   │   ├── hooks/useLogin.ts useRegister.ts
    │   │   └── schemas/auth.ts   # zod
    │   ├── messages/
    │   │   ├── pages/MessagesPage.tsx   ≤ 100 行
    │   │   ├── components/{ConversationList,ChatWindow,StartChatPanel,SendMessageComposer,GroupManagementPanel,AIHostingControl}.tsx
    │   │   ├── components/bubbles/{FileMessageBubble,ImageMessageBubble,ImagePreviewDialog,MessageDateSeparator}.tsx
    │   │   ├── hooks/{useConversations,useChatWindow,useMessageWebSocket,useReadReceipt,useMediaUpload,useAIHosting}.ts
    │   │   ├── store/messagesStore.ts   # zustand
    │   │   └── *.module.css
    │   ├── contacts/
    │   │   ├── pages/ContactsPage.tsx   ≤ 100 行
    │   │   ├── components/{FriendDirectory,FriendRequestsPanel,GroupChatPanel,IdentifierSearch}.tsx
    │   │   └── hooks/...
    │   ├── discover/
    │   ├── me/
    │   ├── feedback/
    │   └── admin/
    │       ├── AdminConsole.tsx          # ≤ 50 行 router shell
    │       ├── pages/{Dashboard,LLMTraces,Conversations,Users,Feedback,TaskReports}.tsx
    │       └── hooks/...
    │
    ├── api/                      # 只做"HTTP fetch + DTO 类型"
    │   ├── client.ts             # 保留，但 react-query 包外层
    │   ├── websocketClient.ts    # 保留
    │   └── <domain>.ts           # 每个域一份请求函数
    │
    ├── models/                   # domain 类型层（FE-16）
    │   ├── messages.ts users.ts friends.ts groups.ts media.ts
    │
    ├── queries/                  # react-query keys + hooks（可选，也可放 features/<X>/hooks）
    │   ├── useCurrentUser.ts
    │   ├── useConversations.ts
    │   └── ...
    │
    ├── stores/                   # zustand 全局 store
    │   ├── authStore.ts          # 由 AuthProvider 退化而来
    │   ├── messagesStore.ts      # 当前会话、未读、ws 状态
    │   └── adminStore.ts
    │
    ├── components/ui/            # 保留：原子组件库（14 个，干净）
    │   └── ...
    │
    ├── utils/                    # 纯函数工具
    │   ├── avatarUpload.ts profileDisplay.ts caseConvert.ts (FE-7)
    │
    ├── styles/                   # CSS Modules / Tailwind 二选一
    │   ├── tokens.css            # 保留 design tokens
    │   ├── globals.css           # 极简 reset + body
    │   └── （各组件 *.module.css 就近放）
    │
    └── tests/                    # 跨页面 e2e/integration
        ├── playwright/
        └── fixtures/
```

---

## 4. 引入的新依赖

| 库                              | 用途                  | 替代                |
|---------------------------------|----------------------|---------------------|
| `react-router-dom` 或 `@tanstack/react-router` | 路由   | App.tsx 手工 routing |
| `@tanstack/react-query`         | 服务端状态缓存        | useState + useEffect props drilling |
| `zustand`                       | 客户端全局状态        | AuthContext + 各种 props |
| `react-hook-form`               | 表单                  | useState + onChange |
| `zod`                           | schema 验证 + 类型    | 手写 if 检查         |
| `prettier`                      | 格式化               | 无                  |
| `husky` + `lint-staged`         | pre-commit hook      | 无                  |
| `eslint-plugin-react-hooks`     | hook deps lint       | 无                  |
| `vite-bundle-visualizer`        | bundle analyzer (dev) | 无                  |
| `@playwright/test`              | E2E                  | 无                  |

新增 ~10 个 dep。**全部都是前端社区主流选择**，没有花活。

---

## 5. 分阶段实施

### Phase 0 — 依赖与基建（1 周）
- **FE-1 prerequisite**：审计 package.json 版本号（lucide-react/vite/typescript/vitest/eslint），回退到真实存在的版本；跑 `npm ci` 干净安装。
- 加 Prettier + husky + lint-staged + react-hooks plugin；
- 加 vite-bundle-visualizer 看现状 bundle 大小。

### Phase 1 — 全局状态 + 服务端缓存（2~3 周）
- 引入 react-query + zustand；
- 把 AuthContext 改为 zustand store（保持 API 兼容）；
- 拆 `api/<dom>.ts` 配套 `queries/use<Dom>.ts`；
- App.tsx 删 18 个 props drill。

### Phase 2 — 路由（1~2 周）
- 引入 React Router v6；
- 实现 §2 FE-4 路由表；
- 删除 `isAdminRoute()`、`appRouteFromLocation()`、手工 popstate；
- AdminConsole 内 `activeView` 改 nested route。

### Phase 3 — 拆巨石（4~6 周，按页面拆）
- MessagesPage 拆 §3 features/messages 目录；
- AdminConsole 拆 §3 features/admin 目录；
- ContactsPage 搬 features/contacts + 拆子组件；
- App.tsx 砍到 50 行。

### Phase 4 — 表单 + 样式（2~3 周）
- 引入 react-hook-form + zod；改 5 个表单：AuthPage / RegisterForm / FeedbackPage / MePage profile / GroupManagement；
- 引入 CSS Modules；
- 逐文件拆 styles.css（按 §3 目录就近放）；
- 删 styles.css 最终只剩 ≤ 100 行 reset。

### Phase 5 — 后端契约对齐（与后端 03/07 phase 同步）
- 选定 snake_case 或 camelCase 单源（FE-7）；
- 实现 case converter 工具或更新所有 model；
- 适配 03 §4.3 ACK 不带 seq 的语义；
- 适配 07 §3.1 新的 10 个 RPC 接口；
- 客户端用 client_msg_id 占位，收到 push event 后替换 — 这是 SDK 级改造，新建 `features/messages/sdk/optimisticUpdate.ts`。

### Phase 6 — 测试 + CI
- Playwright E2E 覆盖关键链路；
- Drone PR pipeline 加前端 lint / type-check / test / build（与 05 OB-13 对应）；
- bundle size budget gate。

---

## 6. 与后端文档的契约同步点

| 接触面                | 前端动作                                                 | 后端引用 |
|----------------------|---------------------------------------------------------|---------|
| `SendMessage` ACK 不带 seq | client_msg_id 占位渲染，push event 收 seq 后替换；改 `models/messages.ts` + `api/messages.ts` + `MessagesPage` 消息列表 hook | 00-decisions D2、03 §4.3、07 §4.3 |
| msg-rpc 新增 RPC | 新增 `api/messages.ts` 方法：revoke / clear / lastMessageByConvs / appendStream / serverTime；新增 UI（撤回 menu、清空对话按钮） | 07 §3.1 |
| Conversation hosting 接口搬到 agent-rpc | 改 `api/messages.ts` 移除 hosting 相关；新增 `api/agentHosting.ts`；改 MessagesPage 的 AIHostingControl 走新 endpoint | 04 §4.2、07 §5 |
| Push event schema 变化 | 改 `api/websocketClient.ts` event 类型；OptimisticUpdate hook 适配 | 03 §3.3 push topic 命名 |
| Kafka topic 命名暴露给前端 | 一般不暴露；admin LLM traces 页可能展示 topic 名（注意命名一致 D5） | 00-decisions D5 |

---

## 7. 验收信号

```bash
# A. 依赖版本真实存在
npx npm-check-updates -e 2 && npm ls react vite typescript

# B. 巨石文件全部 ≤ 300 行
for f in $(find src -name '*.tsx' -o -name '*.ts' | grep -v test); do
  L=$(wc -l < "$f"); [ "$L" -le 300 ] || { echo "$f $L"; exit 1; }
done

# C. styles.css ≤ 100 行（仅 reset + body）
[ $(wc -l < src/styles.css) -le 100 ]

# D. 无 props drilling > 8 个的组件
# 用 ts-prune / typeforge / 手工 grep 检查

# E. 全局 class name 全部 module 化
# 期望 src/styles.css 不出现 .admin-* / .auth-* / .message-*
! grep -E '^\.(admin|auth|message|conversation|friend|group)' src/styles.css

# F. App.tsx ≤ 50 行
[ $(wc -l < src/App.tsx) -le 50 ]

# G. test coverage 关键路径达标
npx vitest --coverage --reporter=summary
```

---

## 8. 风险与回滚

| 风险                                          | 应对                                      |
|----------------------------------------------|------------------------------------------|
| Phase 1 引入 react-query 与现有 useState 冲突 | 按域逐步迁移；保留旧路径直至每个域切完     |
| Phase 2 路由迁移破坏深链接 / admin host       | 上线前对照路由表跑一遍所有 URL；准备 nginx 兼容 rewrite |
| Phase 3 拆 MessagesPage 期间双轨长 PR         | 拆 epic 为 6~10 个小 PR（每个拆 1~2 个子组件），main 持续可发 |
| Phase 5 ACK 语义变化前端没适配                | feature flag：先 server flag 控制；前端 SDK 同步上线后再开 |
| 版本号纠正引发依赖大改                        | 用 lockfile 锁定，新分支验证，逐项验证组件无 regression |

---

## 9. 待确认

1. 版本号是不是 fork？还是真实写错？（FE Phase 0 阻塞）
2. 用 React Router 还是 TanStack Router？
3. styles 走 CSS Modules 还是 Tailwind 还是两个一起？产品视觉对 design system 的诉求有多严？
4. ContactsPage 里有 `friendshipToUserProfile` 被 MessagesPage import → 耦合点，是否搬到 `models/friends.ts`？
5. realIntegration.test 是否需要真实后端跑？CI 里怎么覆盖？
6. 是否上 Storybook？UI kit 14 组件值得单独可视化文档化吗？
