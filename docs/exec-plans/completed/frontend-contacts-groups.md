# frontend-contacts-groups

状态：Completed

## 背景

前端当前已有微信风格四 Tab 壳和联系人占位页，但联系人、好友搜索和群聊详情还没有可交互的基础结构，也缺少与前后端合同路径对齐的 contacts/groups typed API adapter。

## 目标

- 保持 `消息 / 联系人 / 发现 / 我的` 四 Tab。
- 拆分联系人页组件，提供新的朋友、群聊、标签、公众号入口。
- 好友列表按字母分组展示。
- 提供按唯一 `identifier` 搜索用户的 UI 占位，并展示“添加好友”动作。
- 提供群聊列表、创建群、加入群、群详情成员列表的 UI 占位。
- 新增 contacts/groups typed API adapter，路径与 `docs/product-specs/frontend-backend-contract.md` 对齐。
- 用 Vitest + Testing Library 先覆盖目标行为，再实现。

## 非目标

- 不接入真实登录态、真实 REST 请求生命周期或 WebSocket。
- 不修改后端业务逻辑。
- 不引入 UI 组件库。

## 任务拆分

- [x] 补充联系人和群聊页面行为测试。
- [x] 补充 contacts/groups API adapter 路径测试。
- [x] 确认新增测试失败。
- [x] 实现 typed API adapter。
- [x] 拆分并实现联系人/群聊页面组件与 mock 交互。
- [x] 更新 `docs/FRONTEND.md`。
- [x] 运行全量验证并记录结果。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | UI 交互先使用本地 mock 数据，adapter 独立测试合同路径 | 当前任务要求 UI 占位和 mock 测试，不要求接真实登录态 |

## 验证方式

- `npm install --prefix web`
- `npm run frontend:test`
- `npm run frontend:build`
- `npm run frontend:lint`
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...`
- `bash scripts/verify-static.sh`
- `docker compose config`

## 风险与回滚

- 风险：mock UI 与后续真实接口字段可能出现偏差。
- 控制：typed adapter 的请求路径和 payload 使用合同字段名；文档明确当前为 UI 占位。
- 回滚：回退本分支前端和文档提交即可，不涉及后端数据迁移。

## 结果记录

- 先补测试后执行 `npm --prefix web run test:run -- src/App.test.tsx src/api/social.test.ts`，确认失败：缺少 `./contacts` / `./groups` adapter，联系人入口按钮、identifier 搜索和群详情成员列表不存在。
- 实现后相关测试通过：`2 passed` files，`8 passed` tests。
- 最终验证均通过：
  - `npm install --prefix web`
  - `npm run frontend:test`
  - `npm run frontend:build`
  - `npm run frontend:lint`
  - `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...`
  - `bash scripts/verify-static.sh`
  - `docker compose config`
