# Agent 生命周期管理

状态：Draft

## 业务目标

系统支持 Agent 的创建、销毁、配置更新和持久化，保证 Agent 能够在会话中稳定运行。

## 核心能力

- 创建 Agent
- 更新 Agent 配置
- 禁用或销毁 Agent
- 持久化 Agent 配置与状态
- 查询 Agent 当前状态

## 验收标准

- Agent 创建后可被加入会话。
- Agent 配置变更后对后续会话生效。
- Agent 被禁用后不再响应新消息。
