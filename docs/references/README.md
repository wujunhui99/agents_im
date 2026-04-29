# References

本目录存放外部系统、框架、工具和长文本参考资料，尤其是便于 LLM/Agent 读取的 `.txt` 或 `.md` 文件。

## 当前参考仓库

### IM 系统参考

- 仓库：`openimsdk/open-im-server`
- URL：https://github.com/openimsdk/open-im-server.git
- 本地目录：`docs/references/open-im-server/`
- 用途：作为本项目 IM 核心系统的主要参考，包括服务拆分、消息链路、用户/会话/群组模型、部署方式、配置结构、可观测性和工程组织方式。

### Agent 系统参考

目前没有完全匹配本项目目标的 Agent 系统参考，因此先参考以下两个项目的局部设计：

#### DeerFlow

- 仓库：`bytedance/deer-flow`
- URL：https://github.com/bytedance/deer-flow.git
- 本地目录：`docs/references/deer-flow/`
- 用途：参考多 Agent/工作流编排、任务规划、工具调用、执行链路和 Agent 应用工程组织方式。

#### NanoBot

- 仓库：`HKUDS/nanobot`
- URL：https://github.com/HKUDS/nanobot.git
- 本地目录：`docs/references/nanobot/`
- 用途：参考轻量 Agent 系统、工具调用、Agent 配置与运行时抽象。

## 使用原则

- 参考仓库只作为设计输入，不直接决定本项目实现。
- 需要先理解参考项目的边界，再提炼适合本项目的架构和工程实践。
- 如果从参考仓库吸收具体设计，需要在 `docs/design-docs/` 中记录取舍原因。
- 如果参考仓库内容发生变化，需要记录更新时间和关键变化。
- 不应直接复制大段代码，除非明确确认许可证兼容性并保留必要声明。

## 参考资料更新记录

| 日期 | 资料 | 动作 | 备注 |
| --- | --- | --- | --- |
| 2026-04-28 | openimsdk/open-im-server | 初始引入 | IM 系统主要参考 |
| 2026-04-28 | bytedance/deer-flow | 初始引入 | Agent 系统参考之一 |
| 2026-04-28 | HKUDS/nanobot | 初始引入 | Agent 系统参考之一 |
