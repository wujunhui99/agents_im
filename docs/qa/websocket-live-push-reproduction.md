# WebSocket 实时推送生产问题复现与回归检查

最后更新：2026-05-03

## 问题摘要

生产环境中，用户 A 给用户 B 发送单聊消息后，B 页面不会实时出现新消息；B 需要手动刷新/F5 后才能看到该消息。

该问题说明：

- 消息发送 API 基本可用；
- 消息持久化和历史拉取基本可用；
- 实时推送链路存在问题，重点排查 WebSocket 连接/代理/gateway-ws/transfer/outbox 到前端事件处理链路。

生产环境测试地址：

```text
https://agenticim.xyz/
```

> 注意：所有测试账号密码、JWT token、服务器连接信息、数据库连接信息均不得写入本文档；如需记录统一使用 `[REDACTED]`。

## 当前已观察到的生产现象

### 生产最小链路探测结果

测试方式：在生产页面同源上下文中，通过浏览器执行最小 API/WS 探测，不修改代码、不启动本地服务。

已观察结果：

```text
WebSocket:
- B 端尝试连接生产 WebSocket 失败
- 浏览器事件：error
- close code: 1006
- 未收到 message_received frame

发送消息:
- A 调用 POST /messages 给 B 发送消息，HTTP status = 200

刷新/历史拉取:
- B 调用 GET /conversations/seqs?conversationIds=，HTTP status = 200
- 返回数据中能看到 A 刚发送给 B 的 lastMessage
```

结论：

```text
消息落库/历史查询正常；生产实时推送失败至少包含 WebSocket 连接失败或异常关闭问题。
```

目前优先怀疑链路：

```text
browser wss://agenticim.xyz/ws
  -> ingress / reverse proxy WebSocket Upgrade
  -> gateway-ws service / pod
  -> gateway connection registry
  -> message-transfer / outbox dispatcher
  -> frontend message_received handling
```

其中，当前生产最小探测已经显示：

```text
browser -> /ws 连接阶段就失败或异常关闭，close code = 1006
```

因此修复优先级应先确认生产 `/ws` 是否能被浏览器稳定连接，再继续排查是否收到 `message_received`。

## 非生产复现结果，仅作辅助参考

此前一个 Codex agent 在本地环境中也复现到类似现象，但本地环境不是最终验收依据。

本地 Codex 输出摘要：

```text
result: reproduced_message_missing_until_refresh
sendHttpStatus: 200
messageVisibleOnBWithoutRefresh: false
bWsReceivedFrameCountAfterWait: 1
bWsReceivedFramesAfterWait: ["{\"type\":\"connected\"}"]
messageVisibleOnBAfterRefreshList: true
messageVisibleOnBAfterRefreshChat: true
```

本地结果说明：B 端 WebSocket 至少只收到 `connected`，没有收到业务消息；刷新后历史接口可见消息。

但用户已确认：由于生产部署较慢，**修复完成后的优先回归验收先在本地环境执行**。本地通过后，再在需要发布时做生产复验。

本地回归时，将本文档中的生产地址替换为：

```text
http://127.0.0.1:5173/
```

并确认 Vite 代理把：

```text
/ws -> ws://127.0.0.1:8084
/messages -> http://127.0.0.1:8083
/conversations -> http://127.0.0.1:8083
```

转发到本地后端。

## 标准人工复现步骤

### 前置条件

1. 打开生产环境：

   ```text
   https://agenticim.xyz/
   ```

2. 准备两个相互可发送单聊消息的测试用户：

   ```text
   用户 A：发送方
   用户 B：接收方
   密码/token：不得记录，统一 `[REDACTED]`
   ```

3. 使用两个独立浏览器上下文，避免共享登录态：
   - 浏览器窗口/上下文 A：登录用户 A；
   - 浏览器窗口/上下文 B：登录用户 B；
   - 不要使用同一个 localStorage/cookie/session。

### 复现步骤

1. B 打开消息页或与 A 的聊天页。
2. 打开浏览器 DevTools，观察 Console 和 Network / WebSocket。
3. 确认 B 页面尝试连接：

   ```text
   wss://agenticim.xyz/ws?token=[REDACTED]
   ```

   或等价的生产 WebSocket 地址。

4. A 给 B 发送一条唯一消息，例如：

   ```text
   ws-repro-<timestamp>
   ```

5. B 页面不要刷新，等待 5-10 秒。
6. 观察 B 页面：
   - 消息是否自动出现；
   - WebSocket 是否收到 `message_received` 或等价业务事件；
   - Console 是否出现错误；
   - WebSocket 是否异常关闭。
7. B 按 F5/刷新页面。
8. 观察 B 页面刷新后是否能看到刚才 A 发送的消息。

### 预期行为

修复后应满足：

```text
A 发送消息成功后，B 不刷新页面也能在 5 秒内看到新消息。
B 的 WebSocket 连接保持 open。
B 的 WebSocket 收到 message_received 或等价业务事件。
B 刷新后仍然能看到同一条消息，且不会重复展示。
```

### 当前异常行为

当前生产异常表现为：

```text
A 发送消息成功。
B 不刷新页面看不到新消息。
B 的 WebSocket 连接失败或异常关闭，close code = 1006。
B 刷新/F5 后通过历史接口看到该消息。
```

## 标准浏览器自动化复现步骤

可使用 Playwright 或其他真实浏览器自动化工具。必须使用生产 URL，禁止启动本地服务。

### 自动化要求

1. 创建两个独立 browser context：
   - context A：用户 A；
   - context B：用户 B。
2. 在 B context 中监听 WebSocket：
   - 记录 WS URL；
   - 记录 open/error/close；
   - 记录收到的 frame；
   - token 必须脱敏。
3. A 发送唯一消息。
4. B 不刷新等待 5-10 秒。
5. 断言/记录：
   - B 页面是否出现该消息；
   - B 是否收到 `message_received`；
   - B WS 是否关闭；
   - close code/reason。
6. B 刷新。
7. 断言/记录：
   - B 刷新后是否出现该消息。
8. 输出证据 JSON 和截图。

### 证据文件建议

```text
docs/qa/evidence/ws-live-push-<timestamp>/
  b-before-send.png
  b-after-wait-no-refresh.png
  b-after-refresh.png
  playwright-observations.json
  console-a.log
  console-b.log
```

如截图或日志包含 token、密码、服务器敏感信息，提交前必须脱敏。

## 生产最小探测脚本思路

该脚本只用于说明检查点；执行时不得把 token 输出到日志。

```js
// 在 https://agenticim.xyz/ 页面同源上下文执行。
// 账号密码/token 必须脱敏，不得写入文档。

const ws = new WebSocket(`wss://${location.host}/ws?token=${encodeURIComponent(B_TOKEN)}`);

ws.onopen = () => console.log('B ws open');
ws.onerror = () => console.log('B ws error');
ws.onclose = (event) => console.log('B ws close', event.code, event.reason);
ws.onmessage = (event) => console.log('B ws frame', event.data);

await fetch('/messages', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${A_TOKEN}`,
  },
  body: JSON.stringify({
    receiverId: B_USER_ID,
    chatType: 'single',
    clientMsgId: `qa-${Date.now()}`,
    contentType: 'text',
    content: `ws-repro-${Date.now()}`,
  }),
});

// 等待 5-10 秒后检查 B 是否收到 frame。
// 再用 B token 拉历史，验证刷新/历史能看到。
await fetch('/conversations/seqs?conversationIds=', {
  headers: { Authorization: `Bearer ${B_TOKEN}` },
});
```

## 修复后的回归检查清单

每次修复 WebSocket/Ingress/gateway/transfer/outbox/frontend 后，必须按以下清单检查：

### A. WebSocket 连接

- [ ] 生产页面从 HTTPS 下连接的是 `wss://agenticim.xyz/ws?...`，不是 `ws://`。
- [ ] 浏览器 Network 中 `/ws` 状态为 101 Switching Protocols 或等价成功 WS 连接。
- [ ] B 端 WebSocket 不再出现 close code `1006`。
- [ ] B 端连接后能收到 `connected` 或等价握手事件。

### B. 实时消息事件

- [ ] A 发送消息后，B 的 WebSocket 收到 `message_received` 或等价业务事件。
- [ ] 事件 payload 包含可映射的消息字段：
  - `serverMsgId` 或 `server_msg_id` 或 `message_id`
  - `conversationId` 或 `conversation_id`
  - `senderId` 或 `sender_id`
  - `receiverId` 或 `receiver_id`
  - `content`
  - `seq`
- [ ] B 页面不刷新也能显示唯一消息内容。
- [ ] B 会话列表 last message 同步更新。
- [ ] 当前聊天窗口消息列表同步更新。

### C. 刷新后一致性

- [ ] B 刷新后仍能看到同一条消息。
- [ ] 不出现重复消息。
- [ ] `conversation_id + seq` 顺序正确。

### D. 后端链路

- [ ] `gateway-ws` pod 正常运行，无 crash/restart。
- [ ] Ingress/reverse proxy 正确透传 WebSocket Upgrade headers。
- [ ] `message-transfer` 正常运行，并能从 outbox/消息事件分发到 gateway。
- [ ] `message_outbox` 中对应事件不会长期卡 pending/retry/dead。
- [ ] gateway 日志能看到 B 用户连接与投递结果。

### E. 失败判定

以下任一情况均视为未修复：

- [ ] B 仍需 F5 才能看到消息。
- [ ] B WebSocket 仍 close `1006`。
- [ ] B WebSocket 只收到 `connected`，收不到 `message_received`。
- [ ] HTTP 历史能看到消息，但实时 UI 不更新。
- [ ] 修复只在本地可用，生产 `https://agenticim.xyz/` 不可用。

## 排查优先级

如果问题仍复现，按顺序排查：

1. **生产 WebSocket 连接是否成功**
   - 浏览器是否使用 `wss://`；
   - Ingress/Nginx 是否启用 WebSocket Upgrade；
   - `/ws` path 是否转发到 `gateway-ws`。

2. **gateway-ws 是否接受连接**
   - pod 是否 Running；
   - 日志中是否有连接建立/鉴权失败/异常关闭；
   - token 是否被正确解析。

3. **message-transfer / outbox 是否发事件**
   - A 发消息后是否创建 outbox 事件；
   - transfer worker 是否消费并调用 gateway；
   - 投递目标是否是 B 的 account/user id。

4. **前端是否处理事件**
   - 是否订阅 `message_received`；
   - payload 字段是否与前端解析兼容；
   - React state 是否 upsert 到会话列表和当前消息列表。

## 当前结论

截至 2026-05-03 的生产探测结论：

```text
生产环境 WebSocket 实时推送未正常工作。
A -> B 消息发送和历史查询可用。
B 端生产 WebSocket 连接失败/异常关闭，close code = 1006。
因此 B 需要刷新页面才能通过历史接口看到新消息。
```
