# Eino DeepSeek Model Adapter

状态：Completed

## 背景

Agent 系统已经具备管理、审计、Python executor 契约和 IM 写回契约，但还没有 Go 侧 LLM provider adapter。本任务添加 CloudWeGo Eino 与 DeepSeek ChatModel 的最小配置和构造边界。

## 目标

- 添加 CloudWeGo Eino 和 Eino DeepSeek component 依赖。
- 添加 DeepSeek 配置解析：`DEEPSEEK_API_KEY`、`DEEPSEEK_BASE_URL`、`DEEPSEEK_MODEL`。
- 默认 `DEEPSEEK_BASE_URL=https://api.deepseek.com`，`DEEPSEEK_MODEL=deepseek-v4-pro`。
- 生产 adapter 缺少 API key 必须明确失败。
- 默认测试不依赖真实 key 或网络；live DeepSeek 测试必须显式 opt-in。

## 非目标

- 不实现 Agent runtime orchestration。
- 不实现工具/MCP 执行。
- 不实现 Agent 响应写回 IM。

## 任务拆分

- [x] Task 1：添加配置解析和验证测试。
- [x] Task 2：添加 DeepSeek ChatModel adapter 和 fail-first 测试。
- [x] Task 3：添加 `.env.example` placeholder 和相关文档。
- [x] Task 4：运行验证、记录结果、提交并推送分支。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-30 | 将 Eino/DeepSeek 放在 `internal/agentruntime/llm/deepseek` | 保持 provider 作为内部 runtime 实现细节，不进入公共业务契约。 |
| 2026-04-30 | 配置解析允许 API key 为空，但 adapter 构造必须校验失败 | 默认 `go test ./...` 和普通服务加载不能依赖真实 secret；实际生产使用模型时 fail-first。 |
| 2026-04-30 | 将 Go directive 提升到 `go 1.24.1` | Eino 依赖 `github.com/bytedance/sonic`；Sonic README 明确 Go 1.24.0 存在 linker issue，1.24.1 可让默认 `go test ./...` 不需要额外 `-ldflags`。 |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
go get github.com/cloudwego/eino@latest
go get github.com/cloudwego/eino-ext/components/model/deepseek@latest
go mod tidy
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -path './web/node_modules' -prune -o -name '*.go' -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
git diff --check
```

## 风险与回滚

- 风险：依赖体积和间接依赖增加。回滚方式是移除 adapter package、配置字段和 `go.mod`/`go.sum` 依赖。
- 风险：DeepSeek API 变更。处理方式是在 `go get` 后检查本地 module docs/source，并按实际 API 适配。

## 结果记录

- 添加 `internal/agentruntime/llm/deepseek`，使用 Eino DeepSeek component 构造真实 `ToolCallingChatModel`。
- 添加 `internal/config.DeepSeekConfig` 解析和验证；缺失 `DEEPSEEK_API_KEY` 时 adapter 构造返回 `ErrDeepSeekAPIKeyMissing`。
- 添加 `.env.example` 和 `etc/agent-api.yaml` placeholder，不提交真实 key。
- 添加默认跳过的 live DeepSeek smoke test：必须设置 `RUN_LIVE_DEEPSEEK_TESTS=1` 和 `DEEPSEEK_API_KEY`。
- 验证结果（2026-04-30）：上述验证命令全部通过；未执行 live DeepSeek 网络请求。
