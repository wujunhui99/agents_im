# Agent 单聊与群聊

状态：Draft

## 业务目标

用户可以像与普通成员聊天一样与 Agent 对话，也可以在群聊中同时与多个 Agent 协作。

## 核心场景

- 用户与单个 Agent 单聊。
- 多个用户在群聊中 @Agent 获取帮助。
- 多个 Agent 在同一群聊中协作处理任务。
- Agent 调用工具后，将结果以消息形式返回会话。

## 默认助手

系统内置一个默认通用助手 Agent，稳定 identifier 为 `agent_creator`，资料展示名为 `AI 助手`。所有 `account_type=user` 的 human 用户都应拥有与 `agent_creator` 的 accepted 好友/contact 关系；`agent` 和 `admin` 账号不作为 human 用户自动回填。

已有部署通过可重复执行的数据迁移/启动回填创建或修正 `agent_creator` 的 account/profile、Agent 配置、active system prompt 和 prompt binding，并为现有人类用户补齐双向 accepted friendship。历史 `agent_father` 只作为迁移兼容名出现；发现该 identifier 时必须迁移到 `agent_creator`，不能继续作为活跃默认助手命名。

新注册或通过 Account Service 创建的 human 用户在账号创建成功后自动获得 `agent_creator` 好友关系。该能力使用真实 account/profile、friendship、Agent 和 prompt registry 数据路径，不允许前端生产路径硬编码 mock 联系人。

## 验收标准

- Agent 能够接收会话消息并回复。
- 群聊中能够区分用户消息、Agent 消息和工具调用结果。
- Agent 响应失败时，用户能收到可理解的失败说明或降级回复。
- Agent 回复必须通过 Message Service 写回并显示为普通 IM 消息，`message_origin=ai`，前端聊天气泡明显显示 `AI/Agent` 标签。
- 同一 trigger message 不能产生重复 Agent 回复；AI 消息默认不再次触发 AI。
- 默认 `agent_creator` 助手使用通用 AI 助手 system prompt：回答准确、简洁、友好；可以解释概念、比较方案、整理信息、生成文本并提供编程/产品建议；不确定时说明不确定并给出可验证的下一步。

## Agent 群聊 V1

- V1 只支持显式触发：上游事件或后端 seam 传入 `TargetAgentAccountIDs`，或后续 mention metadata 显式给出目标 Agent；普通群消息不会默认触发 Agent。
- 目标 Agent 必须是该群 active 成员，或后续版本定义的显式授权对象；非成员目标不会进入 runtime，并记录失败 trigger 状态。
- 同一条群消息可以显式目标多个 Agent，每个目标 Agent 使用独立幂等 key，避免重复回复。
- Agent 群聊回复必须走 `MessageLogic.SendMessage` / Message Service 写回，作为普通群消息获得新的 conversation `seq`、outbox 事件和 AI origin metadata。
- AI-origin 群消息默认不再触发 Agent；只有会话策略和消息 metadata 同时显式允许递归时才可触发。
- V1 不做文本内容中的自由格式 `@` 解析；如果客户端/API 需要 mention，应由上游构造结构化目标 metadata 后交给 Agent trigger seam。
