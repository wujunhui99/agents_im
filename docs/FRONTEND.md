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
3. **发现**：朋友圈、扫一扫、小程序等发现入口占位，全部标记为 `MVP 占位`；扫一扫当前不启动真实扫码能力。
4. **我的**：个人资料卡、用户详情、服务、收藏、朋友圈、设置入口；支持编辑 `display_name`、`gender`、`age`、`region` 等可变资料字段。

当前消息、联系人和发现页使用本地 mock 数据搭信息架构和视觉骨架；我的页通过 typed user API adapter 调用 `PATCH /me` 更新资料。真实登录、WebSocket、重连补消息和已读状态后续按 [`docs/product-specs/frontend-backend-contract.md`](./product-specs/frontend-backend-contract.md) 接入。

## 目录

```text
web/
  index.html
  package.json
  src/
    api/
      user.ts        # typed /me API adapter，PATCH payload 只允许可变字段
      user.test.ts   # user adapter payload 过滤测试
    components/ui/   # TabBar、TopBar、ListCard、Avatar、SearchBox 等共享 UI
    data/
      mockData.ts    # MVP mock 会话、联系人、发现入口和当前用户
    pages/           # MessagesPage、ContactsPage、DiscoverPage、MePage
    App.tsx          # 四 Tab shell 和页面路由
    App.test.tsx     # 主导航、发现占位和我的页编辑行为测试
    main.tsx
    styles.css
```

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
- 不在前端代码中写入真实 token、密码或后端 secret。
- 前端用户资料更新必须走 `web/src/api/user.ts`，只向 `PATCH /me` 发送可变字段，不发送 `user_id` 或 `identifier`。
