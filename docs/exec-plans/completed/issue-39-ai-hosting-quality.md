# Issue 39 AI Hosting Quality

状态：Completed

## 背景

生产 AI 托管收到明确任务“你能帮我对比一下 Python 和 Go 语言的性能吗？”后回复“可以，你简单说说吧。”。审计记录显示 `agent_runs.input_summary.prompt_text` 已包含触发文本，`output_summary.final_text` 是模型输出；生产 `agent_prompts` 和 `agent_prompt_bindings` 为空，因此问题集中在默认 AI hosting runtime prompt/context construction。

## 目标

- 让默认 AI hosting prompt 明确要求：对方提出清晰问题或任务时直接回答/完成，不只做泛泛确认或要求用户重新说明。
- 确保 runtime/provider 消息中显式使用当前 trigger `prompt_text`，即使 bounded recent conversation context 非空。
- 保持 AI 回复仍通过 Message Service writeback，保留 `message_origin=ai` 和 Agent metadata。
- 用 deterministic/fake runtime/provider boundary 补充回归测试，不引入生产 fake success 或 Python/Go 特例。

## 非目标

- 不实现 custom prompt binding。
- 不调用 live DeepSeek 做默认测试。
- 不变更 DB schema。

## 任务拆分

- [x] 读设计文档和相关代码，定位 trigger -> builder -> runtime -> writeback 数据流。
- [x] 写失败测试覆盖 builder 最近消息包含 trigger/current prompt、默认 prompt 禁止 vague follow-up、DeepSeek message construction 使用 current prompt。
- [x] 修改默认 hosting prompt 和 DeepSeek runtime message construction。
- [x] 运行 focused tests、全量 Go tests、static verification、diff check。
- [x] 记录实现和验证结果。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-07 | 在 DeepSeek runtime messages 中把 `prompt_text` 作为最终当前任务消息显式传入 | 当前代码只在 conversation 为空时使用 `prompt_text`；生产 evidence 说明 prompt_text 到达 audit，但模型主要看到普通历史行，容易把“能帮我吗”当成只需确认。 |
| 2026-05-07 | 在默认 AI hosting system prompt 中加入“清晰任务直接完成，必要信息缺失才澄清，禁止泛泛确认”的约束 | 修复真实 prompt/context 根因，不硬编码 Python/Go 问题。 |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
go test ./internal/agentim ./internal/agentruntime ./internal/agentaudit ./internal/repository ./tests
go test ./...
bash scripts/verify-static.sh
git diff --check
```

## 风险与回滚

- 风险：显式当前任务消息会让 provider 输入多一条 instruction message；回归测试需确认 bounded context 仍保留，trigger 文本仍是最后任务。
- 回滚：revert 本 issue commit 即可恢复旧 prompt/context construction。

## 结果记录

- Root cause：`ConversationAIHostingRuntimeRequestBuilder` 已把生产触发文本放入 `PromptText` 和 bounded recent context，但 `internal/agentruntime/eino.runtimeMessages` 在 `Conversation` 非空时不会把 `PromptText` 作为当前任务显式传给 provider；默认 AI hosting prompt 也只要求“自然、简洁”回复，没有约束明确任务必须直接完成，导致“你能帮我...”被模型当作可泛泛确认的问题。
- Fix：默认 AI hosting prompt 增加“明确问题/请求/任务直接回答或完成，缺少必要信息才澄清，不要只回复可以/好的/你说说”的约束；DeepSeek runtime message construction 跳过 bounded context 中的当前 trigger 复制，并把 `PromptText` 包装为最终 current-task user message。
- Verification：
  - `go test ./internal/agentim ./internal/agentruntime/eino`
  - `go test ./internal/agentim ./internal/agentruntime ./internal/agentaudit ./internal/repository ./tests` failed only because `./internal/agentaudit` directory does not exist.
  - `go test ./internal/agentim ./internal/agentruntime ./internal/agentruntime/eino ./internal/domain/agentaudit ./internal/repository ./tests`
  - `go test ./...`
  - `bash scripts/verify-static.sh`
  - `git diff --check`
