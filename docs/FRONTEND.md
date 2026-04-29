# FRONTEND.md

本文档记录 `agents_im` Web 前端约定。当前阶段已搭建微信风格主框架，并接入认证入口、typed REST API client 基础、联系人/群聊 UI、共享 UI 组件与个人资料编辑；后续继续逐步接入真实好友/群聊数据、消息和 WebSocket 链路。

## 技术栈

- React + TypeScript
- Vite
- Vitest + Testing Library
- lucide-react 图标
- 原生 CSS，先不引入 UI 组件库，避免过早绑定设计系统

## 当前阶段范围

前端第一阶段参考微信主框架，完成四个一级页面：

1. **消息**：会话列表、未读数、最近消息预览。
2. **联系人**：新的朋友、群聊、标签、公众号入口；支持 identifier 搜索、添加好友动作、群聊列表、创建群、加入群和群详情成员列表的 UI 占位。
3. **发现**：朋友圈、扫一扫、小程序等发现入口占位，全部标记为 `MVP 占位`；扫一扫当前不启动真实扫码能力。
4. **我的**：个人资料卡、用户详情、服务、收藏、朋友圈、设置入口；支持编辑 `display_name`、`gender`、`age`、`region` 等可变资料字段，并支持退出登录。

当前会话、联系人、群聊和发现页仍使用本地 mock 数据搭信息架构和视觉骨架；认证页按 [`docs/product-specs/frontend-backend-contract.md`](./product-specs/frontend-backend-contract.md) 调用 `/auth/login` 与 `/auth/register`。我的页通过 typed user API adapter 调用 `PATCH /me` 更新资料。联系人/群聊 typed API adapter 已按同一契约封装 REST 路径，但页面暂未接真实登录态和远端数据。消息 REST、WebSocket、重连补消息和已读状态后续继续按同一契约接入。

## 目录

```text
web/
  index.html
  package.json
  src/
    api/
      client.ts      # typed REST API client，支持 envelope 解析与 Authorization header
      client.test.ts
      contacts.ts    # friends REST typed adapter
      groups.ts      # groups REST typed adapter
      shared.ts      # REST envelope/request helper
      user.ts        # typed /me API adapter，PATCH payload 只允许可变字段
      user.test.ts   # user adapter payload 过滤测试
      social.test.ts # contacts/groups adapter 测试
    auth/
      AuthContext.tsx  # 轻量认证状态和登录/注册/退出动作
      session.ts       # localStorage session 工具
    components/
      ContactsPage.tsx # 联系人、好友搜索、群聊列表和群详情 UI
      ui/              # TabBar、TopBar、ListCard、Avatar、SearchBox、ActionRow 等共享 UI
    data/
      mockData.ts    # MVP mock 会话、联系人、发现入口和当前用户
    pages/           # MessagesPage、DiscoverPage、MePage
    App.tsx          # 认证入口、四 Tab shell 和页面路由
    App.test.tsx     # 认证、主导航、联系人/群聊、发现占位和我的页编辑行为测试
    main.tsx
    styles.css
```

## 认证与 API Client

- REST client 入口为 `web/src/api/client.ts`，默认同源请求；需要跨域联调时使用 Vite env `VITE_API_BASE_URL` 覆盖，例如 `VITE_API_BASE_URL=http://127.0.0.1:8081`。
- 后端响应必须使用统一 envelope：`{ "code": "OK", "message": "ok", "data": {} }`。`code !== "OK"` 或 HTTP 非 2xx 时抛出 typed `ApiError`。
- 受保护接口由 client 注入 `Authorization: Bearer ***`。前端文档、测试和示例不得记录真实 token。
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
- Adapter 接受可注入 `fetcher` 和 bearer token；示例 token 只能使用 `***` 或 mock 值。
