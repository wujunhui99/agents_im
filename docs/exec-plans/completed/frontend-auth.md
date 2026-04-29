# Frontend Auth And API Client

状态：Completed

## 背景

当前 `web/` 已有微信风格四 Tab 主框架，但仍使用本地 mock 展示，缺少认证入口、Bearer token 保存与 REST API client 基础能力。

## 目标

- 新增 typed REST API client，统一处理后端 envelope、错误和 `Authorization: Bearer ***` 注入。
- 新增前端认证状态，支持登录、注册、退出，并使用 localStorage 保存 MVP session。
- 未登录时展示微信风格登录/注册页；登录后进入 `消息 / 联系人 / 发现 / 我的` 四 Tab。
- `我的` 页展示当前用户信息。
- 覆盖登录、注册、token 保存、退出和 API client Authorization header 测试。

## 非目标

- 不改后端业务逻辑。
- 不实现完整好友、群聊、消息 API 接入。
- 不引入新的 UI 组件库或状态管理库。

## 任务拆分

- [x] 补充 Vitest + Testing Library 失败测试。
- [x] 实现 REST API client。
- [x] 实现 auth state/hook 与 localStorage session。
- [x] 接入 App 登录/注册入口和已登录四 Tab shell。
- [x] 更新前端文档。
- [x] 运行必需验证命令并记录结果。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 使用轻量 React Context + localStorage 管理认证状态 | 当前前端规模小，避免提前引入外部状态库 |
| 2026-04-29 | API base URL 默认同源，可通过 `VITE_API_BASE_URL` 覆盖 | 符合 Vite 配置和本地开发/部署差异 |

## 验证方式

```bash
npm install --prefix web
npm run frontend:test
npm run frontend:build
npm run frontend:lint
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
docker compose config
```

## 风险与回滚

- 风险：localStorage 中损坏 session 导致 UI 初始化异常。实现会容错并清理坏数据。
- 风险：API envelope 解析不一致导致表单错误提示不准确。测试覆盖错误对象基础行为。
- 回滚：恢复 `web/src/App.tsx`、测试、样式和新增 API/auth 文件，并移除本计划及文档小节。

## 结果记录

- 新增 `web/src/api/client.ts`，支持 envelope 解包、typed `ApiError`、同源默认 base URL、`VITE_API_BASE_URL` 覆盖和 Bearer token 注入。
- 新增 `web/src/auth/AuthContext.tsx` 与 `web/src/auth/session.ts`，支持登录、注册、退出和 localStorage session。
- 更新 `web/src/App.tsx`，未登录显示登录/注册页，登录后进入微信风格四 Tab，`我的` 页显示当前用户信息并支持退出。
- 更新 `docs/FRONTEND.md` 记录认证与 API client 约定。
- 目标测试先按 TDD 失败于缺失 `api/client` 和 `auth/session`，实现后通过。
- 验证命令全部通过：
  - `npm install --prefix web`
  - `npm run frontend:test`
  - `npm run frontend:build`
  - `npm run frontend:lint`
  - `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...`
  - `bash scripts/verify-static.sh`
  - `docker compose config`
