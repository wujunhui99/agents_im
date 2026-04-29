# Frontend Chat Message Page

状态：Completed

## 背景

当前 Web 前端已有微信风格四 Tab 主框架，消息页仍是静态会话列表。消息链路的 REST 与 WebSocket 合约已在 `docs/product-specs/frontend-backend-contract.md` 中定义，前端需要先建立可测试的消息模型、聊天窗口、发送状态、API adapter 和 WebSocket client wrapper。

## 目标

- 拆分消息页组件，支持会话列表到聊天窗口再返回列表的移动端导航。
- 新增 typed message models、会话 mock 数据和文本发送 composer。
- 消息发送后先追加 pending/sending 状态，再基于 mock 结果转为 sent 或 failed。
- 新增 REST 消息 adapter：`sendMessage`、`pullMessages`、`getConversationSeqs`、`markRead`。
- 新增 WebSocket client wrapper：`connect`、`send`、`close`，支持 token query fallback 并解析 ACK/envelope。
- 用 Vitest + Testing Library 和 fake WebSocket 覆盖关键行为。

## 非目标

- 不接入真实后端运行态。
- 不实现 WebSocket 自动重连、离线补偿、去重缓存或已读 UI。
- 不修改无关后端业务逻辑。

## 任务拆分

- [x] 先补失败测试：消息页列表、点击会话、发送 pending、WebSocket command envelope。
- [x] 实现消息 models、mock 数据、拆分后的消息页和聊天窗口。
- [x] 实现 REST adapter 与 WebSocket client wrapper。
- [x] 更新 `docs/FRONTEND.md` 说明 mock 与 adapter 边界。
- [x] 跑完要求的验证命令并记录结果。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 聊天 UI 先以本地 mock 和 async sender 驱动状态 | 当前任务不要求真实后端运行，保持 UI 可测试且边界清晰 |
| 2026-04-29 | REST adapter 仅做薄封装并保留 contract 字段名 | 与 frontend-backend contract 对齐，避免前端提前发明转换规则 |
| 2026-04-29 | WebSocket wrapper 通过构造器注入支持测试 fake WebSocket | 浏览器 `WebSocket` 在测试环境不可控，注入可覆盖 envelope 行为 |

## 验证方式

- `npm install --prefix web`
- `npm run frontend:test`
- `npm run frontend:build`
- `npm run frontend:lint`
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...`
- `bash scripts/verify-static.sh`
- `docker compose config`

## 风险与回滚

风险集中在前端拆分后样式回归，以及 adapter 类型与后端合约漂移。回滚方式是撤销本计划对应前端文件和文档变更，不涉及后端数据结构和服务实现。

## 结果记录

已完成消息页拆分和移动端会话导航，新增 typed message models、mock 会话数据、发送 composer、REST 消息 adapter 与 WebSocket client wrapper。测试先失败于缺少消息页交互和 WebSocket 模块，随后实现通过。

验证结果：

- `npm install --prefix web`：通过
- `npm run frontend:test`：通过，2 个测试文件、6 个测试
- `npm run frontend:build`：通过
- `npm run frontend:lint`：通过
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...`：通过
- `bash scripts/verify-static.sh`：通过
- `docker compose config`：通过
