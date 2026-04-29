# FRONTEND.md

本文档记录 `agents_im` Web 前端约定。当前阶段先搭微信风格主框架，后续再逐步接入真实接口与 WebSocket 消息链路。

## 技术栈

- React + TypeScript
- Vite
- Vitest + Testing Library
- lucide-react 图标
- 原生 CSS，先不引入 UI 组件库，避免过早绑定设计系统

## 当前阶段范围

前端第一阶段参考微信主框架，先完成四个一级页面：

1. **消息**：会话列表、未读数、最近消息预览。
2. **联系人**：新的朋友、群聊、标签、公众号入口，以及好友列表占位。
3. **发现**：朋友圈、扫一扫、小程序等发现入口占位。
4. **我的**：个人资料卡、服务、收藏、朋友圈、设置入口。

当前页面使用本地 mock 数据搭信息架构和视觉骨架。消息页已具备会话列表、移动端进入聊天窗口、返回列表、文本发送 composer，以及 `sending` / `sent` / `failed` 基础状态。真实登录、重连补消息、去重缓存和完整已读状态后续按 [`docs/product-specs/frontend-backend-contract.md`](./product-specs/frontend-backend-contract.md) 接入。

## 消息页边界

- `web/src/features/messages/` 持有消息页组件和 mock 会话数据，当前不直接请求真实后端。
- `web/src/models/messages.ts` 定义前端会话与消息模型，发送状态仅用于本地 UI 呈现。
- `web/src/api/messages.ts` 是消息 REST 薄 adapter，函数签名覆盖 `sendMessage`、`pullMessages`、`getConversationSeqs`、`markRead`，字段名保持与前后端合约一致。
- `web/src/api/websocketClient.ts` 是 WebSocket client wrapper，提供 `connect`、`send`、`close`，浏览器侧使用 `/ws?token=***` query fallback，并将后端 snake_case ACK 解析为 typed frontend ACK。
- 当前 mock sender 会先追加 `sending` 消息，再模拟 ACK 更新为 `sent`；输入 `/fail` 可进入 `failed` 状态用于本地验收。

## 目录

```text
web/
  index.html
  package.json
  src/
    api/           # REST adapter 与 WebSocket wrapper
    features/      # 页面级功能组件
    models/        # typed frontend models
    App.tsx        # 四 Tab 主框架
    App.test.tsx   # 主导航和消息页行为测试
    main.tsx
    styles.css
```

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
