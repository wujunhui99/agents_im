# WebSocket Gateway Hardening

状态：Completed

## 背景

Gateway WebSocket 第一阶段已经接入真实 JWT、消息逻辑、presence 和本进程投递，但生产边界仍缺少显式跨域策略、query token 开关、服务端 ping 心跳和单连接命令限流。

## 目标

- 增加 Gateway WebSocket 配置面，覆盖 allowed origins、query-token auth、ping/heartbeat 和命令限流。
- 默认行为生产安全：query token 默认关闭，跨域不 allow-all。
- 保持 command ACK envelope 兼容。
- 为 origin、query token、heartbeat/ping 和 rate limit 增加可重复测试。

## 非目标

- 不实现跨实例 Gateway RPC、离线推送、delivery ACK worker。
- 不改变 Message Service 对消息历史、seq 和已读状态的权威职责。
- 不引入 mock/fake 成功路径。

## 任务拆分

- [x] Task 1：扩展 `internal/config` 的 GatewayWS 配置、默认值和 env/file 解析。
- [x] Task 2：将 GatewayWS 配置传入 `cmd/gateway-ws` 和 `internal/gateway/ws.Server`。
- [x] Task 3：实现显式 `CheckOrigin`、query-token auth 开关、ping loop 和 per-connection command rate limiter。
- [x] Task 4：补充/更新测试覆盖生产安全默认值和显式开启场景。
- [x] Task 5：同步设计文档、示例配置和部署配置。
- [x] Task 6：执行格式化、测试、静态验证、提交并推送。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-01 | 复用 `APIConfig` 增加 `GatewayWS` section | `cmd/gateway-ws` 已使用 API config loader，改动面最小。 |
| 2026-05-01 | query-token auth 默认关闭，local/dev 配置显式开启 | 降低 token 经 URL 泄漏风险，同时保留受控开发便利性。 |
| 2026-05-01 | `AllowedOrigins` 为空时只允许 same-origin 浏览器请求 | 空配置不能等价于 allow-all；生产跨域需显式配置 exact origins。 |
| 2026-05-01 | 使用服务端 ping + pong read-deadline 延长 | 避免仅依赖客户端 app heartbeat；死连接通过 read deadline 回收。 |

## 验证方式

- `gofmt -w` touched Go files
- `go test ./internal/gateway/ws ./internal/config ./internal/handler ./internal/logic`
- `git diff --check`
- `bash scripts/verify-static.sh`

## 风险与回滚

- 风险：前端若仍依赖 query token，生产默认关闭会导致握手 401。通过 local config 显式开启、本地文档说明和 header auth 优先降低风险。
- 风险：allowed origins 未配置时，跨域浏览器连接会 403。生产应显式配置 public frontend origins。
- 回滚：还原 GatewayWS 配置和 `Server` 选项后，Gateway 会回到旧握手行为。

## 结果记录

已完成：

- 新增 `GatewayWS` config：allowed origins、query-token auth、ping interval、heartbeat timeout、per-connection command rate limit。
- `cmd/gateway-ws` 将 `cfg.GatewayWS` 注入 WebSocket server。
- `websocket.Upgrader.CheckOrigin` 改为显式策略：无 `Origin` 允许；配置 exact origins 时只允许匹配；未配置 origins 时只允许 same-origin。
- `?token=` 仅在 `GatewayWS.AllowQueryToken=true` 时使用；`Authorization: Bearer` 仍为首选。
- 增加服务端 ping loop，pong 刷新 last-seen/read deadline。
- 增加 per-connection token-bucket command limiter，超限返回兼容 ACK envelope，`error.code=RATE_LIMITED`。

验证结果：

- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH gofmt -w internal/config/config.go internal/config/config_test.go internal/apperror/error.go internal/gateway/ws/server.go internal/gateway/ws/server_test.go cmd/gateway-ws/main.go cmd/single-machine-e2e/main.go tests/websocket_gateway_test.go`
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./internal/gateway/ws ./internal/config ./internal/handler ./internal/logic`：通过
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./tests`：通过
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./cmd/single-machine-e2e`：通过
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...`：通过
- `git diff --check`：通过
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH bash scripts/verify-static.sh`：通过

剩余限制：

- Gateway 仍未实现跨实例 WebSocket fanout RPC；remote presence route 仍只返回 routed 状态。
- 服务端 ping/pong 只判断连接活性，不等价于 delivery ACK 或消息已读。
- 生产浏览器跨域必须显式配置 `GatewayWS.AllowedOrigins` / `GATEWAY_WS_ALLOWED_ORIGINS`，否则只允许 same-origin。
