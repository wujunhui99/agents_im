# 07 — msg-rpc 边界对比与改造

> 用户问的核心：OpenIM msg-rpc 是不是只把 gRPC 转 MQ？为什么不能塞 msggateway？答案见 [对话记录]：**msg-rpc 是消息域完整业务边界，40+ RPC，远不止发 MQ**。
>
> 本文：对照 OpenIM `internal/rpc/msg/`，把当前 agents_im **只有 4 个 RPC 的** msg-rpc 重新切分，明确"必做 / 可选 / 不做"三档，并给出 proto 草案 + 改造路径。
>
> 关联：00-decisions D1/D2/D5/D6/D8/**D10**、03 §3.1/§4、04 §4.2。
>
> **实施顺序：07 先于 03**——03 传输链路依赖 msg/rpc 的接口面（§3.1）与 `SendMessage → MsgToMQ` 改造（§4.2）。先定 msg-rpc 边界（本文），再接 gateway→transfer→push 链路（03）。
>
> **路径约定**：本文出现的 `internal/rpc/msg/...`、`internal/push/...`、`internal/msgtransfer/...` 均指 **OpenIM 仓库**的源码路径（参考用）；`internal/handler/message/`、`internal/logic/message/` 等指 **agents_im 当前**的真实路径（重构前）。按 00-decisions D10，agents_im 重构后这些代码全部落到 `service/msg/{api,rpc}/internal/...`（业务）、`pkg/<name>/`（infra，如 `pkg/idgen` 用于 `server_msg_id` 生成）。

---

## 1. 现状对比

### 1.1 agents_im 当前 msg-rpc
来源：`proto/message.proto`（**4 个 RPC**）：

| RPC                       | 输入要点                                                  | 输出要点                                  |
|---------------------------|-----------------------------------------------------------|-------------------------------------------|
| `SendMessage`             | sender_id / receiver_id / group_id / chat_type / client_msg_id / content / origin / agent metadata | `Message`（带 seq）+ `deduplicated` |
| `PullMessages`            | user_id / conversation_id / from_seq / to_seq / limit / order | `[]Message` + is_end + next_seq           |
| `GetConversationSeqs`     | user_id / `[]conversation_id`                             | `[]ConversationSeqState`                  |
| `MarkConversationAsRead`  | user_id / conversation_id / has_read_seq                  | conversation_id + has_read_seq + max_seq + unread_count + updated |

辅助 in-process handler（在 `internal/handler/message/` 里直接对 HTTP，**不在 proto 上**）：
- `get_conversation_a_i_hosting`
- `update_conversation_a_i_hosting`
- `create_feedback`

**严重缺失**：
- ❌ 撤回 / 删除 / 清空 — 微信式 IM 必备；
- ❌ 服务端时间 — 客户端时钟校准基础；
- ❌ 单条已读 / 群已读人数 — 群聊体验差；
- ❌ 流式消息 — AI streaming 没接口；
- ❌ 会话最后一条消息批量查 — 侧边栏每条都 N+1。

### 1.2 OpenIM msg-rpc（参考标杆）
来源：`internal/rpc/msg/*.go`（**40+ RPC**，按文件分组）：

| 文件             | 行数 | RPC（部分）                                                                                  |
|------------------|------|----------------------------------------------------------------------------------------------|
| `send.go`        | 227  | `SendMsg`、`SendSimpleMsg`、`AppendStreamMsg`                                                |
| `sync_msg.go`    | 258  | `PullMessageBySeqs`、`GetSeqMessage`、`GetMaxSeq`、`SearchMessage`、`GetServerTime`、`GetLastMessage` |
| `as_read.go`     | 229  | `MarkConversationAsRead`、`MarkMsgsAsRead`、`SetConversationHasReadSeq`、`GetConversationsHasReadAndMaxSeq` |
| `seq.go`         | 105  | `GetMaxSeqs`、`GetConversationMaxSeq`、`GetHasReadSeqs`、`GetMsgByConversationIDs`、`SetUserConversationsMinSeq` |
| `revoke.go`      | 133  | `RevokeMsg`                                                                                  |
| `delete.go`      | 161  | `DeleteMsgs`、`DeleteMsgPhysical(BySeq)`、`ClearConversationsMsg`、`UserClearAllMsg`         |
| `clear.go`       |  61  | `DestructMsgs`、`GetLastMessageSeqByTime`                                                    |
| `msg_status.go`  |  44  | `SetSendMsgStatus`、`GetSendMsgStatus`                                                       |
| `statistics.go`  | 107  | `GetActiveUser`、`GetActiveGroup`                                                            |
| `verify.go`      | 214  | 内部：`messageVerification`、`encapsulateMsgData`、`modifyMessageByUserMessageReceiveOpt`     |
| `callback.go`    | 219  | 内部：`webhookBeforeSendSingleMsg`/AfterSendGroupMsg/BeforeMsgModify 等 7 个钩子              |
| `notification.go`|  50  | 内部 helper（系统通知消息构造）                                                              |
| `filter.go`      |  91  | `MessageInterceptorChain`（拦截器框架）                                                       |
| `server.go`      | 172  | 服务初始化、依赖注入                                                                          |
| `utils.go`       |  91  | 工具函数                                                                                      |
| 总               | 2162 |                                                                                              |

---

## 2. 全量 RPC 必要性评级

按"我们这套 IM/Agent 产品要不要这个 RPC"分三档：
- **🟢 必做**：MVP 就要，不做就缺基础体验；
- **🟡 可选**：近期 P1，先 stub，按需补；
- **⚪ 不做**：跟我们形态不匹配，或可以由其它服务/admin 单独处理。

### 2.1 写消息

| OpenIM RPC          | 我们的现状     | 评级 | 说明 |
|---------------------|----------------|------|------|
| `SendMsg`           | ✅ `SendMessage` | 🟢 | 改造（见 §4）：ACK 不再返回 seq，只发 Kafka |
| `SendSimpleMsg`     | ❌             | 🟡 | 简化版（系统通知用）。可由 SendMessage 兼任，但 OpenIM 拆出来是为了让 conversation-rpc/group-rpc 调用更轻；P2 再做 |
| `AppendStreamMsg`   | ❌             | 🟡 | **Agent 流式输出必需**。LLM token-by-token 流式回写时使用；agent-rpc → msg-rpc 调用。P1 |

### 2.2 拉历史

| OpenIM RPC                | 我们的现状      | 评级 | 说明 |
|---------------------------|-----------------|------|------|
| `PullMessageBySeqs`       | ✅ `PullMessages` | 🟢 | 改造：用 from_seq/to_seq 区间（已是）+ 加 seq 列表模式 |
| `GetSeqMessage`           | ❌              | 🟡 | 按 `[]seq` 离散点拉；客户端补 gap 用。可由 PullMessages 加一个 `seqs` 字段兼任 |
| `GetMsgByConversationIDs` | ❌              | 🟢 | 给每个 conv 拿最后一条；**侧边栏会话列表必需**，否则 N+1 调用 |
| `GetLastMessage`          | ❌              | 🟢 | 上面那个的同义版（OpenIM 两个共存有历史原因），合并为一个即可 |
| `SearchMessage`           | ❌              | ⚪ | 全文检索，需 ES/MeiliSearch。MVP 不做，做时拆 search-rpc |

### 2.3 seq 查询

| OpenIM RPC                          | 我们的现状           | 评级 | 说明 |
|-------------------------------------|----------------------|------|------|
| `GetMaxSeq`                         | 部分（在 GetConversationSeqs 里） | 🟢 | 单用户全部 conv 的 max_seq 列表；客户端重连/同步必需 |
| `GetMaxSeqs`                        | 部分                 | 🟢 | 给定 conv 列表查 max_seq |
| `GetConversationMaxSeq`             | 部分                 | 🟡 | 单 conv 查 max；用 GetMaxSeqs 兼任 |
| `GetHasReadSeqs`                    | 部分                 | 🟢 | 已读 seq 批查；客户端拉未读时用 |
| `GetConversationsHasReadAndMaxSeq`  | ✅ `GetConversationSeqs` | 🟢 | 一次性返 max + has_read + unread + last_msg，**重连同步主接口** |
| `GetLastMessageSeqByTime`           | ❌                   | ⚪ | 按时间点查 seq，做"漫游消息条数"统计用。不做 |

### 2.4 已读管理

| OpenIM RPC                        | 我们的现状                  | 评级 | 说明 |
|-----------------------------------|-----------------------------|------|------|
| `MarkConversationAsRead`          | ✅                          | 🟢 | 整会话已读到 seq |
| `MarkMsgsAsRead`                  | ❌                          | 🟡 | 按 server_msg_id 列表标已读；OpenIM 用于群聊部分消息已读。我们群聊先按"整会话"标，后期再加 |
| `SetConversationHasReadSeq`       | ❌                          | ⚪ | 内部 RPC，由 transfer 调来更新；我们 transfer 直接写 Redis 即可 |
| `MarkGroupMessageRead`            | ❌                          | ⚪ | 群消息单条已读人数，复杂功能，按需 |
| `GetGroupMessageReadNum`          | ❌                          | ⚪ | 同上 |
| `GetGroupMessageHasRead`          | ❌                          | ⚪ | 同上 |
| `MarkUserMessageRead`             | ❌                          | ⚪ | 单聊单条已读，跟会话已读重复，不做 |
| `MarkConversationRead`            | ❌                          | ⚪ | OpenIM 后期加的简化版，跟 MarkConversationAsRead 重复 |

### 2.5 撤回 / 删除 / 清空

| OpenIM RPC                       | 我们的现状 | 评级 | 说明 |
|----------------------------------|------------|------|------|
| `RevokeMsg`                      | ❌         | 🟢 | **微信式 IM 必备**。2 分钟内可撤回；服务端写撤回标记，推送给会话所有成员 |
| `DeleteMsgs`                     | ❌         | 🟡 | 用户删自己的消息（仅本人可见消失）；MVP 可不做，但前端常用 |
| `ClearConversationsMsg`          | ❌         | 🟡 | 用户清空一个会话的消息；P1 |
| `DeleteMsgPhysical`              | ❌         | ⚪ | 物理删除，admin 操作；放 admin-rpc，不在 msg-rpc |
| `DeleteMsgPhysicalBySeq`         | ❌         | ⚪ | 同上 |
| `DeleteUserAllMessagesInConv`    | ❌         | ⚪ | 单用户删整 conv 消息；admin/合规用 |
| `UserClearAllMsg`                | ❌         | ⚪ | 用户清空所有自己消息；危险，admin only |
| `ClearMsg`                       | ❌         | ⚪ | admin 全局清，放 admin-rpc |
| `DestructMsgs`                   | ❌         | ⚪ | 阅后即焚；产品不需要 |

### 2.6 服务器时间 / 状态 / 统计

| OpenIM RPC                                     | 我们的现状 | 评级 | 说明 |
|-----------------------------------------------|------------|------|------|
| `GetServerTime`                                | ❌         | 🟢 | 客户端时钟校准；调用极轻，必做 |
| `SetSendMsgStatus` / `GetSendMsgStatus`        | ❌         | ⚪ | OpenIM 用于跨进程"是否在批量导入"互斥；我们没此场景 |
| `GetActiveUser` / `GetActiveGroup`             | ❌         | ⚪ | 活跃度统计，admin BI，放 admin-rpc |
| `GetActiveConversation`                        | ❌         | ⚪ | 同上 |

### 2.7 边界管理（admin 侧）

| OpenIM RPC                                                                | 评级 | 说明 |
|---------------------------------------------------------------------------|------|------|
| `SetUserConversationsMinSeq` / `SetUserConversationMinSeq`                | ⚪   | 跳过历史消息；admin only，放 admin-rpc |
| `SetUserConversationMaxSeq`                                               | ⚪   | 同上 |

---

## 3. 我们 MVP 的目标 RPC 列表

按 §2 评级筛选，加入我们 Agent 相关的扩展。

### 3.1 msg-rpc proto 草案

```protobuf
syntax = "proto3";

package message;
option go_package = "github.com/wujunhui99/agents_im/proto/messagepb";

service MessageService {
  // 写
  rpc SendMessage         (SendMessageReq)         returns (SendMessageResp);
  rpc AppendStreamMessage (AppendStreamMessageReq) returns (AppendStreamMessageResp); // Agent 流式回写

  // 拉历史
  rpc PullMessages              (PullMessagesReq)              returns (PullMessagesResp);
  rpc GetLastMessageByConvs     (GetLastMessageByConvsReq)     returns (GetLastMessageByConvsResp); // 侧边栏

  // seq 同步
  rpc GetConversationsSeqState  (GetConversationsSeqStateReq)  returns (GetConversationsSeqStateResp); // = OpenIM GetConversationsHasReadAndMaxSeq
  rpc GetMaxSeqs                (GetMaxSeqsReq)                returns (GetMaxSeqsResp);
  rpc GetHasReadSeqs            (GetHasReadSeqsReq)            returns (GetHasReadSeqsResp);

  // 已读
  rpc MarkConversationAsRead    (MarkConversationAsReadReq)    returns (MarkConversationAsReadResp);

  // 撤回
  rpc RevokeMessage             (RevokeMessageReq)             returns (RevokeMessageResp);

  // 删除 / 清空
  rpc DeleteMessages            (DeleteMessagesReq)            returns (DeleteMessagesResp);            // P1
  rpc ClearConversationMessages (ClearConversationMessagesReq) returns (ClearConversationMessagesResp);// P1

  // 时间
  rpc GetServerTime             (GetServerTimeReq)             returns (GetServerTimeResp);
}
```

合计 **10 个 RPC**（vs OpenIM 40+，vs 现状 4）。

### 3.2 与现状的差异概览

| RPC                          | 状态     | 改动幅度 |
|-----------------------------|----------|----------|
| SendMessage                 | 改造     | 大：去 PG 写、去 seq 分配、改 ACK 语义（00-decisions D1/D2） |
| PullMessages                | 改造     | 中：从 PG 拉改为 Redis cache 优先 + PG 兜底 |
| GetConversationsSeqState    | 改名 + 改实现 | 中：原名 `GetConversationSeqs`；改从 Redis 读 |
| MarkConversationAsRead      | 改实现   | 中：写 Redis seq，produce read event |
| GetMaxSeqs / GetHasReadSeqs | 新增     | 小（拆分 GetConversationsSeqState 子集） |
| GetLastMessageByConvs       | 新增     | 中 |
| AppendStreamMessage         | 新增     | 中（Agent 用） |
| RevokeMessage               | 新增     | **大**（需要 messages 表 revoked 字段、推 revoke 事件） |
| DeleteMessages              | 新增 P1  | 中 |
| ClearConversationMessages   | 新增 P1  | 中 |
| GetServerTime               | 新增     | 极小 |

---

## 4. SendMessage 详细改造

### 4.1 现状（重构前）
```go
// internal/logic/message/send_message_logic.go (简化)
func (l *SendMessageLogic) SendMessage(req) (resp, error) {
    // 1. 鉴权 / 校验 chat_type
    // 2. 生成 server_msg_id (Snowflake)
    // 3. PG transaction:
    //      INSERT messages (...)
    //      UPDATE conversations SET max_seq = max_seq + 1
    //      INSERT message_outbox (event_id, payload, status='pending')
    // 4. trigger AI hosting hook (in-process)
    // 5. return Message{ServerMsgID, Seq, ...}
}
```

### 4.2 目标（重构后）
对齐 OpenIM `internal/rpc/msg/send.go` + 00-decisions D1/D2/D6：

```go
func (m *msgServer) SendMessage(ctx, req) (resp, error) {
    // 1. 鉴权
    if err := m.authVerify(ctx, req.SenderID); err != nil { return nil, err }

    // 2. messageVerification:
    //    - 单聊：检查 receiver 是否好友 / 是否被拉黑
    //    - 群聊：检查 sender 是否群成员 / 是否被禁言
    //    （照搬 OpenIM verify.go:messageVerification）
    if err := m.verify(ctx, req); err != nil { return nil, err }

    // 3. webhookBeforeSendMsg（可选，钩子配置时启用）
    if err := m.webhookBeforeSendMsg(ctx, req); err != nil { return nil, err }

    // 4. interceptor chain（关键词过滤 / 合规改写 / 风控）
    for _, h := range m.handlers {
        if err := h(ctx, req); err != nil { return nil, err }
    }

    // 5. encapsulateMsgData:
    //    - 生成 server_msg_id (Snowflake)
    //    - 设置 send_time = now (ms)
    //    - 设置 created_at
    //    - 注意：**不分配 seq**（seq 是 transfer 的责任，00-decisions D2）
    msgData := m.encapsulate(req)

    // 6. 发 Kafka：唯一写路径，封装成 MsgToMQ（对齐 OpenIM send.go → MsgDatabase.MsgToMQ）
    //    MsgToMQ(ctx, key, msgData) 内部 = proto.Marshal(msgData) + producer.SendMessage(key, data)
    //    key = conversation_id（分区键，保证同会话顺序）；**无 PG 写、无 outbox、无 seq 分配**
    if err := m.msgDatabase.MsgToMQ(ctx, conversationKey(req), msgData); err != nil {
        return nil, err
    }

    // 7. webhookAfterSendMsg（异步）
    go m.webhookAfterSendMsg(ctx, req)

    // 8. ACK：只带 server_msg_id + send_time，**不带 seq**
    return &SendMessageResp{
        ServerMsgID: msgData.ServerMsgID,
        ClientMsgID: req.ClientMsgID,
        SendTime:    msgData.SendTime,
    }, nil
}
```

> **`MsgToMQ` 是 msg-rpc 新模型下的唯一写原语**：旧版在 PG 事务里写 messages + outbox + 推进 max_seq；新版整条写路径坍缩成一次 `MsgToMQ`（Kafka publish）。持久化、seq、push 全部下沉 msgtransfer（03 §1.1/§5）。因此 msg-rpc 数据层（`service/msg/rpc/internal/model`）只在**读路径**碰 DB（PullMessages 的 PG 兜底），写路径无 DB。

### 4.3 ACK 语义变化（重要）

| 字段              | 旧 ACK | 新 ACK | 说明 |
|-------------------|--------|--------|------|
| `server_msg_id`   | ✅     | ✅     | msg-rpc 生成 |
| `client_msg_id`   | ✅     | ✅     | 透传 |
| `send_time`       | ✅     | ✅     | msg-rpc 设置 |
| `seq`             | ✅     | **❌** | 删除（transfer 异步分配） |
| `deduplicated`    | ✅     | ❌     | 改由 transfer 端用 client_msg_id 去重；ACK 不再告诉客户端 |

**前端配合**：
1. 客户端用 `client_msg_id` 占位渲染（OpenIM SDK 标准做法）；
2. 收到 ACK → 用 `server_msg_id + send_time` 替换占位状态为 "sent"；
3. 收到 push event（`message_received`）→ 拿到 `seq`，更新本地存储并按 seq 排序。

### 4.4 校验逻辑保留

OpenIM `verify.go:messageVerification` 做的事我们也要做：

```go
func (m *msgServer) verify(ctx, req) error {
    switch req.ChatType {
    case "single":
        // 1. receiver 是否存在
        // 2. sender 是否被 receiver 拉黑
        // 3. 是否好友（若产品要求好友才能聊）
        return m.verifySingle(ctx, req)
    case "group":
        // 1. group 是否存在
        // 2. sender 是否群成员
        // 3. sender 是否被禁言
        // 4. group 是否被禁言（全员）
        return m.verifyGroup(ctx, req)
    }
}
```

agents_im 当前 SendMessage 缺这些校验（或散落各处）。

---

## 5. 不放在 msg-rpc 的接口（边界澄清）

按 00-decisions D8 和 04 文档 AG-1，以下接口虽然现在散落在 msg-api 里，**不应该在 msg-rpc**：

| 当前位置                                                       | 应当归属      | 原因 |
|----------------------------------------------------------------|---------------|------|
| `internal/handler/message/get_conversation_a_i_hosting_handler.go` | **agent-rpc** | Agent 会话托管配置是 Agent 域 |
| `internal/handler/message/update_conversation_a_i_hosting_handler.go` | **agent-rpc** | 同上 |
| `internal/handler/message/create_feedback_handler.go`          | **feedback-rpc**（独立）或 admin-rpc | 用户反馈不是消息 |
| 任何 `internal/logic/admin_*.go` 涉及消息回放                   | **admin-rpc** | admin 工具 |

msg-rpc 只承担消息域：write/read/seq/read-state/revoke/delete。

---

## 6. 拦截器 / Webhook 框架（OpenIM 关键设计借鉴）

OpenIM `internal/rpc/msg/filter.go` 实现了 `MessageInterceptorChain`：业务方注册 `MessageInterceptorFunc`，在 SendMsg 内 sync 执行。

**建议第一版不做**，因为：
- agents_im 当前没有外部业务方接入需求；
- 拦截器框架引入会让 SendMessage 复杂度变高；
- 等真有需求（如 Agent 系统要在用户发消息时改写消息内容、加签名）再加。

但**预留 hook 点**：在 `verify` 之后、`encapsulate` 之前留一个 `m.runInterceptors(ctx, req)` 占位空函数，未来实现拦截器时不破坏 API。

---

## 7. webhook（业务方钩子）

OpenIM `callback.go` 7 个 webhook 钩子：
- `BeforeSendSingleMsg` / `AfterSendSingleMsg`
- `BeforeSendGroupMsg` / `AfterSendGroupMsg`
- `BeforeMsgModify`
- `AfterGroupMsgRead`
- `AfterSingleMsgRead`
- `AfterRevokeMsg`

我们**第一版不做**，原因同 §6。但如果未来要接合规风控（关键词过滤外包给第三方）会用到。

> 预留：proto 中保留 `before_send_hook_url` / `after_send_hook_url` 配置位（在 msg-rpc 自己的 yaml），第一版不实现。

---

## 8. 改造分阶段（与 03 §9 phase 对齐）

### Phase 0：proto 扩展 + 文档化（1 周）
- 写新 `proto/message.proto`（10 个 RPC，本文 §3.1）；
- 生成 pb.go、pb_grpc.go；
- 写出契约文档（迁移 03 §3.1 + 本文）；
- 暂不实现新接口，老的 4 个保持工作。

### Phase 1：SendMessage 改造（2~3 周，与 03 Phase 1 合并）
- 实现 `verify` / `encapsulate`；
- 移除 PG 写、outbox 写；
- 改为 Kafka publish；
- ACK 去掉 seq；
- 加 feature flag `MSG_DIRECT_KAFKA=true` 灰度。

### Phase 2：读路径走 Redis cache（2 周，与 03 Phase 2 合并）
- `PullMessages` 改为优先 Redis cache（`msg:cache:conv:*`）；
- `GetConversationsSeqState` 改为读 Redis seq；
- 增加 `GetMaxSeqs` / `GetHasReadSeqs` 拆出来；
- 兜底走 PG（消息归档表保留，00-decisions D7）。

### Phase 3：新增 RPC（3~4 周）
- `RevokeMessage`：DB 加 `revoked_at`、`revoked_by` 字段；产生 `msg.toTransfer.v1` 的撤回事件类型；
- `GetLastMessageByConvs`：批量查 Redis 最后一条；
- `GetServerTime`：极简实现；
- `AppendStreamMessage`：Agent 流式 token append 接口（消息 schema 加 stream 状态）；
- `DeleteMessages` / `ClearConversationMessages`：标记删除模式（不物理删）。

### Phase 4：把 ai_hosting / feedback 搬走（1 周）
- `get_conversation_a_i_hosting` → 移到 agent-rpc；
- `update_conversation_a_i_hosting` → 同上；
- `create_feedback` → 移到 feedback-rpc 或 admin-rpc；
- msg-api 改为只调 msg-rpc。

---

## 9. 风险与回滚

| 风险                                       | 应对                                                |
|--------------------------------------------|----------------------------------------------------|
| ACK 不带 seq 前端不兼容                    | feature flag 灰度；前端 SDK 用 client_msg_id 占位（参考 OpenIM SDK） |
| 撤回事件 fanout 顺序与原消息错乱           | 用 conversation_id partition key 保证同 conv 严格顺序 |
| 删除/撤回的消息能从 PG 拉到（cache miss）   | PG 查询自动过滤 `revoked_at != null` / `deleted_for_user_id`  |
| `verify` 引入跨 RPC 调用（user/group/friend）增加 P99 | 用 `internal/rpccache.UserLocalCache` 等本地缓存，参考 OpenIM `server.go:62`-`72` |

---

## 10. 一致性钩子（同步状态）

- **Phase 0 落地（#457 PR1）**：`service/msg/rpc` 已建——proto-first 10-RPC 接口面 + goctl 数据层（脱 `internal/repository`，迁移 019 给 `user_conversation_states` 加代理 PK），4 个 RPC（SendMessage/PullMessages/GetConversationsSeqState/MarkConversationAsRead）行为对齐旧实现（仍 PG 写 + seq 分配 + outbox，**未动 Kafka/§4.2 MsgToMQ**），新增 RPC 返回 Unimplemented。additive 部署（msg-rpc:9098，dormant `internal/rpcgen/message` 暂留）。跨域 inline 鉴权（群/媒体）作 keystone 例外暂依赖 internal。详见 [progress/02-microservices.md](progress/02-microservices.md) msg 行。
- **PR2 落地（#457 PR2）**：`service/msg/api` 已建——.api-first→goctl 纯 BFF，4 消息路由 over gRPC 调 msg-rpc（msgclient + rpcerror.FromStatus，行为对齐旧 message-api 这 4 条）。additive 部署（msg-api:8090，未切流，message-api 保留 8083）。feedback/ai-hosting **不并入 msg-api**（in-process 接线重 + 注定迁 agent-rpc/feedback-rpc）→ 暂留 message-api。**待切流**：ingress 把 /messages + /conversations + /api/feedback 切到 msg-api、退休 message-api/rpcgen/message——需先把 ai-hosting/feedback 归位（ingress `/conversations` 前缀被 ai-hosting 共享，无法部分切）；gateway-ws 仍 in-process。
- **00-decisions D10**：msg-rpc 接口范围以本文为准（10 个 RPC）——已落。
- **03-message-pipeline**：§0.1 已注明 msg-rpc 承担消息域全部 RPC，`SendMessage → MsgToMQ` 只是其中之一——已落。
- **04-agent §3.1**：Agent runtime 写回 IM 走 msg-rpc.SendMessage，stream 模式走 AppendStreamMessage（§2.1）——待 04 确认。
