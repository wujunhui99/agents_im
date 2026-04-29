# Agent Infrastructure Parallel Baseline

状态：Active

本文档是 Agent 基础设施并行开发的共享契约基线。后续多个 Codex Agent 必须从本文件、`docs/product-specs/agent-system.md`、`docs/design-docs/agent-system-architecture.md`、`AGENTS.md` 和 go-zero skill 开始阅读。

## 本阶段目标

建设 Agent 基础设施，不实现 LLM 推理核心，不绑定具体 LLM provider/framework。

必须覆盖：

1. 扩展账号系统 `account_type`: `normal` / `agent` / `admin`。
2. Agent 账号作为 IM 用户参与聊天；Agent 配置另建 `agents` 表，不塞入 `users` 表。
3. 系统提示词、工具、skills 独立管理并持久化。
4. Skill 文件使用 MinIO/OSS 对象存储；PG 只保存元数据、object key、sha256。
5. 第一版禁止 shell/命令行能力。
6. Python 执行只能通过沙箱化 executor 契约，不能在主服务进程内直接运行任意 Python。
7. MCP 工具第一版只允许管理员配置，并通过 Agent 白名单绑定。
8. 每次 Agent run、tool call、file read、python exec 必须审计。
9. Agent 响应必须通过 Message Service 写回 IM，不允许绕过 IM 后端写消息。

## 非目标

- 不接入真实 LLM provider。
- 不实现 LangChain/ADK/OpenAI Agents 等框架运行时。
- 不实现 shell/命令行脚本执行。
- 不提交真实 secret、token、credential、LLM key、MinIO credential。
- 不用 mock/stub/fake success 冒充真实能力。

## 并行分支规划

### 1. `feature/agent-account-types`

职责：账号类型基础能力。

范围：

- `users.account_type` migration，默认 `normal`。
- Go domain/model/repository/API/RPC 类型增加 account_type。
- 创建普通用户默认 `normal`。
- 支持创建 Agent 用户时 account_type=`agent` 的内部能力。
- admin 能力只保留类型和校验，不做完整 RBAC。

不得做：Agent 配置表、prompt/tool/skill CRUD、Python executor。

验收：

- TDD 测试覆盖默认 normal、agent/admin 合法值、非法值拒绝。
- PG 和 memory repository 语义一致。
- 现有注册/登录/E2E 不破坏。

当前分支落地说明：

- `normal`、`agent`、`admin` 作为 user domain 合法枚举写入 Go model、REST response、User RPC contract 和 PostgreSQL migration。
- 内存与 PostgreSQL repository 均把空 `account_type` 归一化为 `normal`，非法值返回明确参数错误。
- HTTP 公开创建与 auth 注册路径不传递请求体中的 `account_type`，避免公开注册为 `admin` 或 `agent`。

### 2. `feature/agent-core-management`

职责：Agent 配置管理，不含 LLM 执行。

依赖：账号类型契约；可以先用接口适配，集成时接 `agent` account type。

范围：

- 新增 agent domain/repository/logic/API 契约。
- `agents` 表：agent_id、im_user_id、name、description、status、created_by、prompt/tool/skill binding 引用字段或关系表。
- Agent 必须绑定一个 account_type=`agent` 的 IM user。
- 提供基础 CRUD/list/get/status。
- 不允许 Agent 配置写入 `users` 表。

不得做：LLM run、Python executor 实现、shell 执行。

验收：

- 创建 Agent 时 user 不存在/不是 agent 类型必须失败。
- 禁止 silent fallback 创建假 user。
- API/logic/repository 单测通过。

### 3. `feature/agent-prompts-tools-skills`

职责：Prompt/Tool/Skill 元数据和绑定。

范围：

- system prompts：内容、版本、状态、created_by。
- tools：类型 `mcp` / `local` / `builtin`，MCP server ref，local handler_key 白名单，不保存可执行脚本。
- skills：元数据、版本、object_key、sha256、content_type、size、status。
- Agent binding：agent_prompt_bindings、agent_tool_bindings、agent_skill_bindings。
- Skill 文件只记录对象元数据；不把文件内容塞 PG。

安全规则：

- shell/command tool 类型必须拒绝。
- MCP 工具必须要求 admin-created + agent whitelist binding。
- local tools 只能保存 `handler_key`，不能保存任意代码。

验收：

- 非法 tool 类型拒绝。
- skill sha256/object key 必填。
- binding 去重/校验。

### 4. `feature/agent-audit-log`

职责：审计基础设施。

范围：

- Agent run audit 表/接口：run_id、agent_id、trigger、conversation_id、requesting_user_id、status、timestamps。
- tool call audit：tool_id、input/output 摘要、status、error、duration。
- file read audit：skill_id/file object key/sha256、status。
- python exec audit：sandbox request id、status、resource summary、error。
- 不记录 secret/token/raw credential。

验收：

- 所有审计写入失败必须暴露；不能吞错后返回成功。
- 提供 append-only repository/logic 测试。
- 审计内容对敏感字段做 redaction 或 summary。

### 5. `feature/agent-python-sandbox-contract`

职责：Python 沙箱执行契约，不在 Go 主服务内执行任意 Python。

范围：

- 定义 Python executor client interface / request / response / policy。
- 支持超时、CPU/memory 限制字段、网络策略、文件 allowlist。
- 提供 `disabled` executor 默认实现：明确返回 `ErrPythonExecutorDisabled`，不得假成功。
- API/logic 层只创建执行请求和审计，不直接 `os/exec` 或 shell。

禁止：

- 禁止 `exec.Command("python"...)` 在主服务中直接执行任意代码。
- 禁止 shell。
- 禁止无审计执行。

验收：

- 默认未配置 executor 时明确失败。
- 测试验证主服务没有 shell/python 直接执行路径。

### 6. `feature/agent-im-integration-contract`

职责：Agent 与 IM 消息链路集成契约。

范围：

- Agent run 触发来源契约：用户私聊 Agent、群聊 mention Agent、管理员手动 run。
- Agent response writer interface：只允许调用 Message Service 发送消息。
- 预留 outbox/worker trigger，不实现 LLM。
- 文档说明 Agent 不能绕过 Message Service 直接写 messages 表。

验收：

- 单元测试确保 response writer 调用 MessageLogic/Message Service，而不是 repository 直写。
- 无 LLM provider 也能编译和测试。

## 共享质量规则

所有 Codex 分支必须：

1. 先写/更新测试，确认失败，再实现。
2. 失败优先：缺依赖/未配置能力必须返回明确错误，不准 fake success。
3. 禁止 mock 进入真实主流程；测试 fixture 可以使用明确命名的 fake/in-memory。
4. 所有真实依赖测试必须 opt-in；默认 `go test ./...` 不依赖外部 PG/Redis/Kafka/MinIO。
5. 使用 go-zero/goctl 时按 spec-first，先 `.api`/`.proto`，再生成/迁移。
6. 不提交任何 secret。
7. 每个分支必须更新相关 product/design/exec-plan 文档。
8. 每个分支必须提交并 push 到 origin feature 分支。

## 每分支必跑验证

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name '*.go' -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
git diff --check
```

涉及前端才需要：

```bash
npm install --prefix web
npm run frontend:test
npm run frontend:build
npm run frontend:lint
```

## 集成顺序

1. `agent-account-types`
2. `agent-core-management`
3. `agent-prompts-tools-skills`
4. `agent-audit-log`
5. `agent-python-sandbox-contract`
6. `agent-im-integration-contract`

集成到 `develop` 后完整验证，再合并到 `main` 并重复验证。
