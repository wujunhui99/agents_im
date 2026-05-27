# issue-293-message-service-layout

状态：Completed

## 背景

Message API/RPC 仍使用旧的 top-level `api/message.api`、`proto/message.proto`、`internal/rpcgen/message` 与 `cmd/message-api` 直接装配方式。Issue #293 要求迁移到 go-zero service layout，并保持 message/outbox/gateway 语义不变。

## 目标

- 将 Message API canonical source 放到 `service/message/api/message.api`。
- 将 Message RPC canonical source 放到 `service/message/rpc/message.proto`。
- 使用 `goctl api go` 与 `goctl rpc protoc` 生成 scaffold。
- 让 `cmd/message-api` 与 `cmd/message-rpc` 只调用公开 `entry` bridge，不直接 import service internal 包。
- 更新必要 import/config/static-check 路径，保持业务行为不变。

## 非目标

- 不改 message persistence、outbox、conversation sequence、gateway/WebSocket push、message-transfer 行为。
- 不新增 API 直连 DB/repository 能力；如旧 message-api 仍有直连 DB 装配，作为后续 BFF/RPC 拆分债务记录。
- 不引入 `api-gateway` 或 `common/model`。

## 任务拆分

- [x] Task 1：定位旧 message `.api`、`.proto`、API handler/logic/context、RPC scaffold 与 static checks。
- [x] Task 2：复制 canonical specs 到 `service/message/api/message.api` 与 `service/message/rpc/message.proto`，调整 proto `go_package` 到 service-local package。
- [x] Task 3：运行 `goctl api validate` / `goctl api go` 生成 Message API scaffold。
- [x] Task 4：运行带 module flags 的 `goctl rpc protoc` 生成 Message RPC scaffold，确认没有嵌套 `github.com` 目录。
- [x] Task 5：迁移并适配最小业务代码、entry bridge、cmd imports、config and static checks。
- [x] Task 6：运行 required verification：gofmt touched Go files、`git diff --check`、`bash scripts/verify-static.sh`、`go test ./...`。
- [x] Task 7：提交、push、创建/更新 PR 到 `main`，并在 Issue #293 评论 handoff。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-27 | 只做 layout/import/config/static-check 迁移 | Issue 明确要求保持 message 业务语义不变 |
| 2026-05-27 | 保留现有 message-api 行为装配，记录 API 直连 DB 为 deferred | 一次性完整 BFF RPC 化会扩大 message/admin/AI hosting 行为改动面 |

## 验证方式

- `goctl api validate -api service/message/api/message.api`
- `goctl api go -api service/message/api/message.api -dir service/message/api --style go_zero`
- `goctl rpc protoc service/message/rpc/message.proto --go_out=. --go_opt=module=github.com/wujunhui99/agents_im --go-grpc_out=. --go-grpc_opt=module=github.com/wujunhui99/agents_im --zrpc_out=service/message/rpc --style go_zero --verbose`
- `gofmt -w <touched go files>`
- `git diff --check`
- `bash scripts/verify-static.sh`
- `go test ./...`

## 风险与回滚

- 风险：generated path changes can break imports. Mitigation: run focused package tests and full `go test ./...`.
- 风险：static checks may still assert old `internal/rpcgen/message` and top-level spec paths. Mitigation: update path assertions only where they describe migrated Message API/RPC generated layout.
- 回滚：revert this issue branch commit before merge; no DB migration or production state change is planned.

## 结果记录

- `service/message/api` 与 `service/message/rpc` 已由 goctl 生成并接入现有业务逻辑。
- `cmd/message-api` / `cmd/message-rpc` 已改为只调用 service-local `entry` bridge。
- `goctl api validate` 对 `service/message/api/message.api` 通过；原重复 `createFeedback` handler 改为 `createAPIFeedback`，`ClientMeta` 改为 goctl 可解析的 `map[string]interface{}`。
- `goctl rpc protoc` 使用 module flags 生成到 `service/message/rpc/message`，未产生嵌套 `github.com` 目录。
- Verification passed: `gofmt`, `goctl api validate -api service/message/api/message.api`, focused `go test ./cmd/message-api ./cmd/message-rpc ./service/message/...`, `go test ./...`, `bash scripts/verify-static.sh`, `git diff --check`。
- Deferred：message-api 仍保留既有直接 repository/DB 装配以保持 admin、AI hosting、feedback 和 message 行为不变；后续如要纯 BFF 化应单独补 RPC 能力再迁移。
