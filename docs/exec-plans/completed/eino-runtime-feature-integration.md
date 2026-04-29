# Eino Runtime Feature Integration

状态：Completed

## 背景

`develop` 需要按顺序集成 Eino runtime core、DeepSeek model adapter、安全 tool adapter 和 Agent-IM runner 四个 feature 分支，并消除并行分支间的 runtime contract 分歧。

## 目标

- 按指定顺序合并 `origin/feature/eino-runtime-core`、`origin/feature/eino-deepseek-model`、`origin/feature/eino-tool-adapter`、`origin/feature/eino-im-runner`。
- 保留单一稳定业务 runtime interface：`internal/agentruntime.Runtime`。
- 保留 DeepSeek 默认配置和 fail-first 缺 key 行为。
- 保留 tool whitelist 安全边界。
- 确保 Agent 响应只通过 Message Service response writer 写回。
- 执行并记录集成验证结果。

## 非目标

- 不实现完整 Eino Agent orchestration。
- 不实现真实 MCP 网络调用、tool execution、Python sandbox execution 或 IM worker wiring。
- 不执行 live DeepSeek 网络测试。
- 不合并到 `main`。

## 任务拆分

- [x] 校验 feature branch head SHA。
- [x] 按指定顺序合并 feature 分支。
- [x] 解决 `ARCHITECTURE.md` merge conflict，保留 DeepSeek adapter 和 tool resolver 两侧说明。
- [x] 将 Agent-IM runner 从本地 `AgentRuntime` seam 调整为使用 `internal/agentruntime.Runtime`。
- [x] 为 runner 增加显式 `RuntimeRequestBuilder`，缺少真实 Agent config/context 时 fail-first。
- [x] 补充 runner 单测覆盖 request builder failure 和 recursion policy gating。
- [x] 更新架构、产品和 runtime 边界文档。
- [x] 完成要求的验证命令。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-30 | `internal/agentim.AgentRunOrchestrator` 直接依赖 `internal/agentruntime.Runtime` | 避免并行 feature 分支引入重复且不兼容的 runtime interface。 |
| 2026-04-30 | 引入 `RuntimeRequestBuilder` seam | Agent trigger 本身不包含完整 Agent config、prompt/tool/skill snapshot 或 conversation context；生产 wiring 必须真实加载，不能构造假请求。 |
| 2026-04-30 | `allow_recursive_trigger` 同时要求 runtime policy 和 runtime result metadata opt-in | 保持 Agent 消息默认不递归触发，符合 Agent-IM loop prevention 契约。 |
| 2026-04-30 | Tool resolver 只保留安全 metadata/adapter seam | V0 不执行 MCP、本地 handler、Python 或文件系统写入能力；缺少安全 adapter 时显式失败。 |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -path './web/node_modules' -prune -o -name '*.go' -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
git diff --check
git grep -n 'sk-eae0b9909aa4464aa4f0a5f085a19efc\|DEEPSEEK_API_KEY: sk-' -- . ':!.env' && exit 1 || true
```

## 风险与回滚

- 风险：完整 Eino orchestration 尚未实现，当前只提供 core contract、DeepSeek ChatModel adapter、tool resolver contract 和 Agent-IM runner seam。
- 风险：`RuntimeRequestBuilder` 需要后续生产 wiring 接入真实 registry/context loader；缺失时会显式失败。
- 回滚：可 revert 本集成提交和四个 feature merge commits，或在后续 feature 分支中替换 concrete runtime wiring。

## 结果记录

- 已合并 feature head：
  - `eino-runtime-core`: `659a44c5f9b0aabaa9657e17ec95dd9f1022add9`
  - `eino-deepseek-model`: `9a9df28a49486fb2abb6a5ed98d0f8fdf9a6d8cc`
  - `eino-tool-adapter`: `66059e31b58eafa39a3c8685838557dfe768b778`
  - `eino-im-runner`: `7ce9226c16023d4570164f8f08d7871ce4dc907c`
- 默认测试未执行 live DeepSeek 网络请求；`TestLiveDeepSeekGenerate` 仍需 `RUN_LIVE_DEEPSEEK_TESTS=1` 和 `DEEPSEEK_API_KEY`。
- `.env` 仍由 `.gitignore` 忽略；`.env.example` 只保留 placeholder key。
- 验证结果（2026-04-30）：上述验证命令全部通过。
