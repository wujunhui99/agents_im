# WebSocket 可靠性设计

状态：Draft

## 目标

提升 WebSocket 长连接稳定性和消息投递可靠性。

## 核心机制

- 心跳检测：Gateway 服务端按 `GatewayWS.PingIntervalSeconds` 发送 WebSocket ping，pong 会刷新 last-seen 并将 read deadline 延长到 `GatewayWS.HeartbeatTimeoutSeconds`；客户端任意有效业务帧（尤其应用层 `heartbeat` 命令）也会延长同一 read deadline，避免部分移动浏览器/内置 WebView 对 WebSocket control ping/pong 支持不稳定时反复重连。
- ACK 确认：客户端确认消息投递结果。
- 重试机制：未确认消息进入重试或补偿流程。
- 连接状态：在线状态与连接实例分离，避免多端登录时状态混乱。
- 命令限流：每条连接按 `GatewayWS.CommandRateLimitPerSecond` 和 `GatewayWS.CommandRateLimitBurst` 做 token-bucket 限流，超限返回 ACK error `RATE_LIMITED`。

## 待设计

- ACK 超时时间
- 消息重试次数
- 多端同步策略
- 离线消息补偿策略
