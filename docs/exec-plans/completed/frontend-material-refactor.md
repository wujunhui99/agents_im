# frontend-material-refactor

状态：Completed

## 背景

前端已经完成微信式四 Tab 主框架，并将认证、消息、联系人和个人资料主流程接入真实 REST API。此前视觉层仍以零散全局 CSS 和少量 WeChat-style 共享组件为主，本次重构为轻量自研的 Google Material Design 3-inspired 设计系统，同时不引入 Material Web 或 MUI 等重依赖。

## 目标

- 建立 `web/src/styles/tokens.css`，统一颜色、surface、tonal roles、shape、spacing、typography、state layer 和 elevation tokens。
- 在 `web/src/components/ui/` 下补齐轻量组件层：Button、Card、TextField、TopAppBar、NavigationBar、ListItem、Avatar、Badge、MessageBubble。
- 改造 App shell、登录注册页、消息列表/聊天页、联系人页、发现页、我的页，使视觉统一到 Material 3-inspired surface、shape、spacing 和 state feedback。
- 保留微信式四 Tab：`消息 / 联系人 / 发现 / 我的`。
- 保留真实 REST API 行为、错误暴露、消息排序、发送中禁用和失败状态展示，不引入生产 mock/fake success。
- 更新 `docs/FRONTEND.md` 和 `scripts/verify-static.sh`，记录设计系统和禁止重依赖的静态检查。

## 非目标

- 不修改后端 API、协议或数据库。
- 不实现群聊列表、标签、公众号、朋友圈、扫一扫、小程序等未实现业务；这些入口继续显示明确 `MVP 占位` 或未实现说明。
- 不引入 `@material/web`、`@mui/*` 或其他重型 UI 组件库。
- 不重写消息数据流或用 mock 替代真实 API 错误。

## 任务拆分

- [x] 新增 design tokens 与轻量 UI 组件。
- [x] 重构 App shell、认证页、TopBar/TabBar、消息、联系人、发现、我的页面渲染结构。
- [x] 重写/整理 CSS，使样式通过 tokens 和组件 class 组织。
- [x] 更新前端文档和静态检查脚本。
- [x] 运行必需验证命令，修复失败并记录结果。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-01 | 采用自研 CSS tokens + React 小组件，不引入 Material Web/MUI | 满足轻量化要求，避免组件库绑定和 bundle/主题复杂度 |
| 2026-05-01 | 保留现有 API adapter、AuthContext 和 MessagesPage 数据更新流程 | 当前主流程已接真实 REST API，重构目标是视觉层，不应扩大行为风险 |
| 2026-05-01 | 静态脚本新增 design-system 文件存在性、文档模式、tokens 模式和禁止依赖检查 | 防止后续回退到零散 CSS 或引入 `@material/web` / `@mui/*` 重依赖 |

## 验证方式

从仓库根目录运行：

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
npm --prefix web ci
npm run frontend:test
npm run frontend:build
npm run frontend:lint
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name '*.go' -print)
go test ./...
bash scripts/verify-static.sh
POSTGRES_PASSWORD=local-postgres-placeholder REDIS_PASSWORD=local-redis-placeholder MINIO_ROOT_USER=local-minio-user MINIO_ROOT_PASSWORD=local-minio-password docker compose -f deploy/middleware/docker-compose.yml config >/tmp/frontend_material_refactor_compose_config.txt
bash -n scripts/dev-up.sh
bash -n scripts/dev-demo-data.sh
git diff --check
```

## 风险与回滚

- CSS 重构可能造成布局溢出或 Testing Library 查询误匹配；已通过现有 frontend test/build/lint 和 diff check 控制。
- 共享组件类名调整可能影响页面结构；本次保留现有可访问名称、tab role、form label 和 API 调用路径以降低风险。
- 如后续发现视觉回归，可回滚本计划涉及的 `web/src/styles.css`、`web/src/styles/tokens.css` 和 `web/src/components/ui/*` 变更，不需要修改后端 API。

## 结果记录

- 新增 `web/src/styles/tokens.css`，定义 Material 3-inspired design tokens。
- 新增轻量 UI 组件：`Button`、`Card`、`TextField`、`TopAppBar`、`NavigationBar`、`ListItem`、`Badge`、`MessageBubble`，并让既有 `TopBar`、`TabBar`、`ListCard`、`SearchBox`、`ActionRow` 接入新组件层。
- 重构登录注册页、App shell、消息列表/聊天窗口、联系人页和我的页的渲染结构与样式；发现页继续通过明确 `MVP 占位` 展示未实现入口。
- 消息 API、联系人 API、认证 API 和个人资料 API 调用路径未改为 mock，测试仍验证真实 adapter 请求路径。
- `docs/FRONTEND.md` 已记录 tokens/components、四 Tab 保留和禁止 Material Web/MUI 重依赖。
- `scripts/verify-static.sh` 已增加 design-system 文件、tokens、文档和禁止依赖检查。
- 验证结果：上述命令均已通过；compose config 输出写入 `/tmp/frontend_material_refactor_compose_config.txt`。
