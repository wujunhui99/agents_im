# WebSocket 生产坑位与 k8s 发布排查手册

Status: Implemented / Runbook
Last updated: 2026-05-03
Scope: `msggateway`、浏览器 WebSocket、`message-transfer`、Kafka/Redpanda、k3s/Ingress/ConfigMap/Deployment

## 背景

生产问题表现为：

1. 早期浏览器连接 `wss://agenticim.xyz/ws?token=[REDACTED]` 失败或很快断开，常见 close code 为 `1006`。
2. 后续 WebSocket 握手已经成功：HTTP `101 Switching Protocols`，无 `401/403/1006`。
3. A 发消息接口返回 `200`，消息能落库；B 刷新或拉取历史能看到消息。
4. B 不刷新时收不到 WebSocket incoming frame，前端没有 `message_received` 事件。

最终定位为两个阶段的问题：

- 阶段一：浏览器 WebSocket 握手配置问题：query token 与 Origin 白名单。
- 阶段二：实时 fanout 链路未启用：`message-transfer` 生产配置仍是 `Dispatcher.Driver: noop`，不会把 `message.accepted` 事件投递到 `msggateway` 在线连接。

## 一句话结论

浏览器能连上 WebSocket 不等于实时推送已经打通。完整链路必须同时满足：

```text
browser /ws?token=[REDACTED]
  -> msggateway handshake 101
  -> message-api send message 200
  -> message_outbox / Kafka message.accepted
  -> message-transfer consumer
  -> message-transfer gateway dispatcher
  -> msggateway internal delivery endpoint
  -> msggateway in-memory connection manager
  -> browser receives message_received
```

## 生产链路

```text
Browser B
  wss://agenticim.xyz/ws?token=[REDACTED]
        |
        v
Ingress / Traefik
        |
        v
Service msggateway:8084
        |
        v
msggateway /ws
  - validates JWT token
  - checks Origin
  - registers connection in ConnectionManager
  - registers presence metadata

Browser A
  POST /api/messages
        |
        v
message-api / message-rpc
  - persists message
  - writes outbox event
        |
        v
outbox publisher -> Kafka/Redpanda topic message.events.v1
        |
        v
message-transfer
  - consumes message.accepted
  - dispatches via gateway HTTP dispatcher
        |
        v
http://127.0.0.1:8084/internal/delivery/conversation
        |
        v
msggateway PushToConversation / PushToUser
        |
        v
Browser B receives { type: "message_received", data: ... }
```

当前 k3s 生产是单副本 `msggateway`，并且 `msggateway` 使用 `hostNetwork: true`。因此 `message-transfer` 可以通过 `http://127.0.0.1:8084` 调用同节点共址的 gateway 内部投递 endpoint。后续如果 `msggateway` 多副本或跨节点，需要改为 Redis/NATS/Kafka fanout 或按 presence route 投递到对应 gateway instance，不能继续假设 localhost 即目标连接所在进程。

## 坑 1：浏览器 WebSocket 不能设置 Authorization header

### 症状

- 前端浏览器直接连接 WebSocket 时失败。
- 后端日志可能显示 unauthorized。
- 非浏览器脚本如果能设置 `Authorization: Bearer ...`，可能复现不出问题。

### 原因

浏览器原生 `WebSocket` API 不能自定义 `Authorization` header，所以前端只能使用 query token：

```text
wss://agenticim.xyz/ws?token=[REDACTED]
```

### 必须配置

`deploy/k8s/etc/msggateway.yaml`：

```yaml
GatewayWS:
  AllowQueryToken: true
```

`etc/msggateway.yaml` 本地开发也应允许 query token，避免本地与生产行为分叉：

```yaml
GatewayWS:
  AllowQueryToken: true
```

### 防回归

`scripts/verify-static.sh` 必须检查：

```bash
rg -q "AllowQueryToken: true" deploy/k8s/etc/msggateway.yaml
rg -q 'GATEWAY_WS_ALLOW_QUERY_TOKEN: "true"' deploy/k8s/configmap.yaml
rg -q "AllowQueryToken: true" etc/msggateway.yaml
```

## 坑 2：生产 Origin 白名单不能为空

### 症状

- `/ws?token=[REDACTED]` 已经打开 query token 后仍然失败。
- 浏览器可能报 `WebSocket connection failed`，close code 可能是 `1006`。
- 脚本或无 Origin 的客户端可能成功，真实浏览器失败。

### 原因

浏览器 WebSocket 会带 `Origin: https://agenticim.xyz`。生产经过 Ingress/TLS termination 后，gateway 看到的 host/proto 不一定与浏览器 Origin 完全一致。依赖“空白名单 fallback”不可靠。

### 必须配置

`deploy/k8s/configmap.yaml`：

```yaml
GATEWAY_WS_ALLOWED_ORIGINS: "https://agenticim.xyz"
```

`deploy/k8s/etc/msggateway.yaml`：

```yaml
GatewayWS:
  AllowedOrigins: ${GATEWAY_WS_ALLOWED_ORIGINS}
```

本地开发：

```yaml
GatewayWS:
  AllowedOrigins: http://localhost:5173,http://127.0.0.1:5173
```

### 防回归

`scripts/verify-static.sh` 必须检查生产 origin 非空且精确包含生产域名：

```bash
rg -q 'GATEWAY_WS_ALLOWED_ORIGINS: "https://agenticim\.xyz"' deploy/k8s/configmap.yaml
if rg -q 'GATEWAY_WS_ALLOWED_ORIGINS:\s*""' deploy/k8s/configmap.yaml; then
  echo "production k8s websocket origins must not be empty" >&2
  exit 1
fi
rg -F -q 'AllowedOrigins: ${GATEWAY_WS_ALLOWED_ORIGINS}' deploy/k8s/etc/msggateway.yaml
```

## 坑 3：WebSocket 握手成功不代表实时推送成功

### 症状

- B 端 WebSocket：HTTP `101 Switching Protocols`。
- 无 `401/403/1006`。
- A 发消息：HTTP `200`。
- B 不刷新：无 incoming WS frame，无 `message_received`。
- B 刷新/拉取：能看到消息。

### 判断

这说明：

- 鉴权、Origin、连接建立已经基本正常。
- 消息落库和历史拉取正常。
- 问题在落库后的实时 fanout：outbox/Kafka/transfer/dispatcher/gateway delivery。

## 坑 4：`message-transfer` 生产不能使用 noop dispatcher

### 旧配置问题

`deploy/k8s/etc/message-transfer.yaml` 之前是：

```yaml
Consumer:
  Driver: kafka

Dispatcher:
  Driver: noop
```

这会导致 worker 即使消费到 `message.accepted`，也只是“假装成功”，不会调用 `msggateway`，所以在线 B 收不到 frame。

### 修复后配置

`deploy/k8s/etc/message-transfer.yaml`：

```yaml
Consumer:
  Driver: kafka

Dispatcher:
  Driver: gateway
  GatewayEndpoint: http://127.0.0.1:8084
```

本地 `etc/message-transfer.yaml` 也同步为 gateway dispatcher，避免本地默认仍走 noop。

### 代码实现

新增 HTTP dispatcher：

- `service/msgtransfer/internal/transfer/gateway_http_dispatcher.go`
- `service/msgtransfer/internal/transfer/gateway_http_dispatcher_test.go`

核心行为：

1. 将 `transfer.MessageEvent` 映射为 `delivery.EventMessageReceived`。
2. POST 到：

```text
/internal/delivery/conversation
```

3. 把 `msggateway` 返回的 `delivery.Result` 映射回 `transfer.DispatchResult`。
4. `5xx` / 网络错误：retryable。
5. `4xx`：failed。
6. offline / routed recipient：不视为整体失败，客户端可通过 seq 补拉。

新增 gateway internal endpoint：

- `internal/gateway/ws/server.go`
- `cmd/msggateway/main.go`

endpoint：

```text
POST /internal/delivery/conversation
```

请求体：

```json
{
  "conversation_id": "single:sender:receiver",
  "recipient_user_ids": ["receiver"],
  "event": {
    "type": "message_received",
    "data": {
      "server_msg_id": "...",
      "conversation_id": "...",
      "seq": 1,
      "sender_id": "sender",
      "receiver_id": "receiver",
      "content_type": "text",
      "content": "hello"
    }
  }
}
```

返回体：`delivery.Result`。

### 防回归

`scripts/verify-static.sh` 必须禁止生产 noop：

```bash
if grep -A2 '^Dispatcher:' deploy/k8s/etc/message-transfer.yaml | grep -q 'Driver: noop'; then
  echo "production message-transfer must not use noop dispatcher" >&2
  exit 1
fi
if ! grep -A3 '^Dispatcher:' deploy/k8s/etc/message-transfer.yaml | grep -q 'Driver: gateway'; then
  echo "production message-transfer must dispatch to msggateway" >&2
  exit 1
fi
if ! grep -A3 '^Dispatcher:' deploy/k8s/etc/message-transfer.yaml | grep -q 'GatewayEndpoint: http://127\.0\.0\.1:8084'; then
  echo "production message-transfer must target colocated msggateway internal endpoint" >&2
  exit 1
fi
```

## k8s 相关坑位

### 1. ConfigMap 修改后必须确认 Pod 已使用新配置

只看 Drone CI 成功不够，需要确认运行中 Pod 真的拿到了新 config/image。

检查：

```bash
kubectl -n agents-im get deploy,pods,svc -o wide
kubectl -n agents-im describe configmap agents-im-config
kubectl -n agents-im get deploy msggateway message-transfer -o yaml
```

确认点：

- `msggateway` image digest/tag 是否为最新 commit 对应镜像。
- `message-transfer` image digest/tag 是否为最新 commit 对应镜像。
- `GATEWAY_WS_ALLOWED_ORIGINS=https://agenticim.xyz`。
- `GATEWAY_WS_ALLOW_QUERY_TOKEN=true`。
- `/config/message-transfer.yaml` 中 `Dispatcher.Driver=gateway`。
- `/config/message-transfer.yaml` 中 `GatewayEndpoint=http://127.0.0.1:8084`。

### 2. `hostNetwork: true` 与 localhost 假设

当前 `msggateway` 使用 `hostNetwork: true`，生产单副本时 `message-transfer -> http://127.0.0.1:8084` 能命中同节点 gateway。

风险：

- 如果未来 `msggateway` 扩多副本，某个用户连接可能在另一个 gateway 进程。
- 如果 `message-transfer` 与 `msggateway` 不共址，localhost 会打到错误进程或无服务。
- 如果改掉 `hostNetwork`，localhost 语义会变化。

后续扩展方案：

- 使用 Redis/NATS pubsub 按 user/conversation fanout。
- 或使用 presence route 中的 `instance_id/gateway_id` 找到目标 gateway，再通过 service/internal RPC 定向投递。
- 或让每个 gateway 订阅 Kafka/Redis 自己过滤在线用户。

### 3. Service 到 Pod 的负载均衡不适合“本地连接管理器”投递

`msggateway` 的在线连接保存在本进程 `ConnectionManager`。如果通过 Kubernetes Service 随机打到任意 gateway Pod，则可能命中没有该用户连接的 Pod，返回 offline/routed。

生产单副本暂时没问题；多副本必须先实现跨实例路由，不要直接把 dispatcher endpoint 改成普通 service 并以为完成。

### 4. Ingress 路由只负责外部 `/ws`，内部投递不应暴露公网

公网入口：

```text
https://agenticim.xyz/ws
```

内部投递 endpoint：

```text
/internal/delivery/conversation
```

这个 endpoint 仅给集群内 `message-transfer` 调用。当前生产通过 localhost 调用，不经过 Ingress。不要把 internal delivery endpoint 暴露给浏览器或公开 API。

### 5. 发布时必须同时看 Drone CI 和 k3s 状态

Drone CI 成功只说明 pipeline 结束；仍需看：

```bash
gh run list --branch main --limit 5
gh run view <run-id> --json jobs,status,conclusion,url
kubectl -n agents-im rollout status deploy/msggateway
kubectl -n agents-im rollout status deploy/message-transfer
kubectl -n agents-im logs deploy/msggateway --since=20m
kubectl -n agents-im logs deploy/message-transfer --since=20m
```

如果失败，不要只看邮件提醒；必须查看具体 job/step logs 与 k3s events/logs。

## 验证方式

### 单元/集成测试

相关测试：

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./internal/transfer -run 'TestGatewayHTTPDispatcher' -count=1
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./tests -run 'TestWebSocketGatewayInternalConversationDeliveryEndpointPushesToOnlineReceiver' -count=1
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
git diff --check
```

测试覆盖：

- gateway HTTP dispatcher 会 POST 到 `/internal/delivery/conversation`。
- dispatcher 会把 transfer event 映射成 `message_received`。
- gateway internal delivery endpoint 能推送到在线 WebSocket receiver。
- 生产配置禁止 noop dispatcher。

### 生产 E2E 验证

必须用真实浏览器/生产域名验证：

1. B 登录并打开消息页。
2. B WebSocket 连接：`wss://agenticim.xyz/ws?token=[REDACTED]`。
3. DevTools / CDP 看到 HTTP `101`，无 `401/403/1006`。
4. A 给 B 发送一条唯一内容消息，POST 返回 `200`。
5. B 不刷新页面，应收到 incoming WS frame。
6. frame type 应为 `message_received`。
7. B 页面不刷新出现该消息。
8. 刷新/拉取历史也能看到同一条消息，且不重复显示。

如果第 1-3 步失败，优先查 query token / Origin / Ingress。
如果第 4 步失败，优先查 message-api / auth / message persistence。
如果第 5-7 步失败但第 8 步成功，优先查 outbox/Kafka/message-transfer/dispatcher/gateway internal delivery。

## 排查命令清单

### WebSocket handshake

```bash
kubectl -n agents-im logs deploy/msggateway --since=20m | grep -Ei 'websocket|handshake|origin|unauthorized|connected|disconnected'
```

重点看：

- `websocket_handshake_failed status=unauthorized`
- Origin 相关拒绝
- `websocket_connected`
- `websocket_read_closed`，其中 `error` 可区分 read timeout、client close、abnormal EOF 等断线原因；只记录 trace/request/connection/user 与 close code，不记录 token。
- `websocket_disconnected`

### message-transfer

```bash
kubectl -n agents-im logs deploy/message-transfer --since=20m | grep -Ei 'message-transfer|dispatcher|gateway|retry|failed|consumed'
```

重点看：

- worker 是否启动。
- consumer 是否为 kafka。
- dispatcher 是否为 gateway，不能是 noop。
- 是否有 gateway dispatcher 5xx/网络错误。

### 配置确认

```bash
kubectl -n agents-im get configmap -o yaml | grep -E 'GATEWAY_WS|KAFKA|MESSAGE_TRANSFER'
kubectl -n agents-im get deploy msggateway message-transfer -o wide
```

### 生产行为判定

- `101 + no frame + refresh sees message`：fanout 问题。
- 周期性 `websocket_read_closed ... i/o timeout` 且前端持续重连：优先查客户端是否定期发送应用层 `heartbeat`、gateway 是否已部署“收到有效客户端帧也续 read deadline”的版本，以及移动浏览器/WebView 是否吞掉 WebSocket control pong。
- `401`：token/query token/JWT 问题。
- `403` 或浏览器 close `1006`：Origin/Ingress/upgrade 问题。
- `POST /messages 200 + DB 有消息 + Kafka 无事件`：outbox publisher 问题。
- Kafka 有事件但 transfer 无日志：consumer/group/broker/config 问题。
- transfer 消费成功但 B 无 frame：dispatcher/gateway internal delivery/连接路由问题。

## 当前实现边界

已实现：

- 浏览器 query-token WebSocket handshake。
- 生产精确 Origin 白名单。
- `message-transfer` gateway HTTP dispatcher。
- `msggateway` internal conversation delivery endpoint。
- 生产配置从 noop dispatcher 改为 gateway dispatcher。
- 静态检查防止生产退回 noop。

未实现 / 后续注意：

- 多 gateway 副本跨实例 fanout。
- internal delivery endpoint 的服务间鉴权；当前依赖 localhost/集群内调用边界。后续如果改为 Service/RPC，应加入 internal auth 或网络策略。
- 离线推送/APNs/FCM。
- 每用户 delivery ACK 持久化；当前 V1 依赖 `conversation_id + seq` 补拉。
- 复杂群聊大 fanout 优化；V1 单聊/群聊都实时推送，不做 1 秒聚合。

## 相关文件

- `cmd/msggateway/main.go`
- `internal/gateway/ws/server.go`
- `internal/gateway/ws/delivery.go`
- `internal/gateway/delivery/delivery.go`
- `cmd/message-transfer/main.go`
- `service/msgtransfer/internal/transfer/gateway_http_dispatcher.go`
- `service/msgtransfer/internal/transfer/gateway_http_dispatcher_test.go`
- `tests/websocket_gateway_internal_delivery_test.go`
- `deploy/k8s/etc/msggateway.yaml`
- `deploy/k8s/etc/message-transfer.yaml`
- `deploy/k8s/configmap.yaml`
- `deploy/k8s/deployments.yaml`
- `deploy/k8s/ingress.yaml`
- `scripts/verify-static.sh`

## 复盘结论

这次最容易误判的点是：

1. 看到 WebSocket 报错时，只看 token，不看 Origin。
2. 看到 WebSocket 已经 `101` 后，以为实时推送已修好。
3. 忽略了 `message-transfer` 的 `Dispatcher.Driver: noop`，导致消费链路存在但没有真正 fanout。
4. k8s 上只看 CI/CD 成功，不确认 Pod 配置和运行时日志。

以后遇到“刷新能看到，不刷新看不到”的消息问题，优先按以下顺序分层排查：

```text
WS handshake -> message persistence -> outbox/Kafka -> transfer consumer -> dispatcher -> gateway local connection -> frontend upsert
```
