# 第一波前端真实 API 联调技术债治理

状态：Completed
Completed: 2026-05-01

## 背景

前端第一阶段已经完成微信风格四 Tab、认证入口、消息/联系人/群聊/我的页骨架和 typed adapters，但仍存在第一波联调技术债：

- 多个 REST 微服务分布在不同端口，Vite 本地开发没有统一 proxy，导致真实注册/登录或页面调用容易打到前端 dev server 并返回非 API envelope。
- `user` / `friends` / `groups` / `messages` adapters 存在多套 request helper/token 注入方式，容易出现某些页面绕过 AuthContext/session。
- 消息页默认依赖本地 mock ACK，Codex 容易只让单测/构建通过而没有证明真实 API 路径可用。
- 文档和 AI skill 未明确“mock 只能显式用于 test/demo，不能替代真实运行”。

## 目标

1. 统一前端 REST adapter 到 `createApiClient`，所有受保护请求共享 envelope 解析和 bearer token 注入。
2. 为 Vite dev server 增加多服务 proxy：`/auth`、`/me`、`/users`、`/friends`、`/messages`、`/conversations`、`/groups`、`/ws`。
3. 消息页增加真实 API 模式，通过 message API 拉取/发送；mock 只用于测试 fixture 或显式 demo/mock 场景。
4. 增加约束测试，防止回退到默认 mock 主流程或绕过统一 client。
5. 更新文档和 Codex frontend skill，说明 mock 产生原因与解决策略。

## 非目标

- 本次不实现完整联系人/群聊页面的远程数据加载 UI 状态机。
- 本次不替换所有视觉占位数据；`发现` 等纯视觉 MVP 占位可继续保留。
- 本次不实现生产 API Gateway；本地真实联调用 Vite proxy 解决。

## 执行步骤

- [x] 检查当前 main、API adapters、Vite 配置和前端测试基线。
- [x] 先写失败测试：统一 client/token、dev proxy、多服务路径、消息页 real mode。
- [x] 实现 adapters 统一、Vite proxy、消息页 real mode。
- [x] 运行 targeted frontend tests/build/lint，修复类型与 lint 问题。
- [x] 更新文档和 AI 规则。
- [x] 运行完整验证并合并到 develop/main；当前 `main` 已包含对应实现。

## Codex mock 倾向原因与约束

Codex 容易写 mock 的主要原因：

1. **验收信号偏局部**：如果只要求组件测试、build、lint，通过本地 mock 最快。
2. **真实环境成本高**：多微服务端口、数据库/Redis/Kafka、鉴权 token 都需要启动和串联；没有脚本/文档时，模型会规避不确定性。
3. **契约入口不唯一**：多套 API helper 和 token 存储方式会让 Codex 倾向复制现有局部写法，而不是统一链路。
4. **mock 没有边界**：如果文档没有写“mock 只能显式 test/demo”，mock 会从测试数据扩散到主流程。

本次治理采用的解决策略：

- 把真实运行入口写进 Vite proxy 和 docs，降低真实联调成本。
- 用测试约束所有 adapter 走统一 `createApiClient`，统一 bearer token 注入。
- 在 docs 和 frontend skill 写明：mock 只能显式用于 test fixture、demo placeholder 或显式 mock mode；业务主流程要证明真实 API/WS 路径。
- 后续 Codex 任务必须在结果里明确哪些调用是真实 API、哪些是 mock/demo，以及真实运行命令输出。

## 验证记录

- 2026-05-01：`npm ci --prefix web` 通过。
- 2026-05-01：`npm run frontend:test` 通过，8 个测试文件 / 18 个测试通过。
- 2026-05-01：`npm run frontend:build` 通过。
- 2026-05-01：`npm run frontend:lint` 通过。
- 2026-05-01：当前 `main`/`HEAD` 为 `e1fdba70ede044879775c13fa31c1025f4a1b371`，已包含统一 `createApiClient`、Vite proxy、多服务 REST adapters、默认真实消息 API 页面和相关前端规则文档。本计划从 `active` 移至 `completed`。
