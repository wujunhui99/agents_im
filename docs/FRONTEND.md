# FRONTEND.md

本文档记录 `agents_im` Web 前端约定。当前阶段已搭建微信风格主框架，并接入认证入口与 typed REST API client 基础；后续再逐步接入好友、群聊、消息和 WebSocket 链路。

## 技术栈

- React + TypeScript
- Vite
- Vitest + Testing Library
- lucide-react 图标
- 原生 CSS，先不引入 UI 组件库，避免过早绑定设计系统

## 当前阶段范围

前端第一阶段参考微信主框架，完成四个一级页面：

1. **消息**：会话列表、未读数、最近消息预览。
2. **联系人**：新的朋友、群聊、标签、公众号入口，以及好友列表占位。
3. **发现**：朋友圈、扫一扫、小程序等发现入口占位。
4. **我的**：个人资料卡、服务、收藏、朋友圈、设置入口。

当前会话、联系人和发现页仍使用本地 mock 数据；认证页按 [`docs/product-specs/frontend-backend-contract.md`](./product-specs/frontend-backend-contract.md) 调用 `/auth/login` 与 `/auth/register`。好友、群聊、消息 REST、WebSocket、重连补消息和已读状态后续按同一契约继续接入。

## 目录

```text
web/
  index.html
  package.json
  src/
    api/
      client.ts        # typed REST API client
      client.test.ts
    auth/
      AuthContext.tsx  # 轻量认证状态和登录/注册/退出动作
      session.ts       # localStorage session 工具
    App.tsx            # 认证入口和四 Tab 主框架
    App.test.tsx       # 认证与主导航行为测试
    main.tsx
    styles.css
```

## 认证与 API Client

- REST client 入口为 `web/src/api/client.ts`，默认同源请求；需要跨域联调时使用 Vite env `VITE_API_BASE_URL` 覆盖，例如 `VITE_API_BASE_URL=http://127.0.0.1:8081`。
- 后端响应必须使用统一 envelope：`{ "code": "OK", "message": "ok", "data": {} }`。`code !== "OK"` 或 HTTP 非 2xx 时抛出 typed `ApiError`。
- 受保护接口由 client 注入 `Authorization: Bearer ***`。前端文档和示例不得记录真实 token。
- MVP 认证状态使用 React Context 和 localStorage，key 为 `agents_im.auth.v1`。保存内容限于 access token 与当前用户展示信息；遇到损坏 session 会清理并回到登录页。
- 未登录时显示登录/注册页；登录或注册成功后进入 `消息 / 联系人 / 发现 / 我的` 四 Tab。`我的` 页展示当前用户昵称、账号、地区和用户 ID，并提供退出登录。

## 本地命令

从仓库根目录执行：

```bash
npm install
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
- 不在前端代码中写入真实 token、密码或后端 secret。
