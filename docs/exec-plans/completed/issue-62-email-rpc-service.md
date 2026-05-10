# Issue 62 Email RPC Service

状态：Completed

## 背景

GitHub Issue #62 requires a reusable internal email RPC service backed by Tencent Cloud SES. This implementation is limited to the standalone RPC service; Auth integration is intentionally excluded for a later task.

## 目标

- Add `mail-rpc` as an internal go-zero RPC service with no public REST API.
- Define `SendTemplateEmail` for template-based mail with recipient list, template id, template variables, optional from/subject override, and optional idempotency key.
- Isolate Tencent Cloud SES behind a provider interface.
- Fail closed when SES credentials/config are missing or when Tencent returns an error.
- Cover config validation, provider request assembly, provider error normalization, and RPC logic delegation with unit tests.

## 非目标

- No Auth registration or verification-code flow wiring in this issue.
- No public HTTP endpoint.
- No real Tencent Cloud calls in tests.
- No persistent audit table, queue, retry worker, or multi-provider fallback.

## 任务拆分

- [x] Add failing unit tests for mail config validation, Tencent SES request body, Tencent provider error handling, and RPC logic delegation.
- [x] Add `proto/mail.proto` and generate go-zero RPC scaffold into `internal/rpcgen/mail` plus protobuf output under `proto/mailpb`.
- [x] Implement `internal/mail` provider contract and Tencent SES TC3 adapter using standard library HTTP.
- [x] Wire `internal/rpcgen/mail/internal/svc` and `SendTemplateEmailLogic` to validate requests, default template id, call provider, and map provider errors to gRPC status errors.
- [x] Add `cmd/mail-rpc/main.go`, `etc/mail-rpc.yaml`, config placeholders, and static verification entries.
- [x] Run focused tests, goctl API validation, gofmt, `go test ./...`, `scripts/verify-static.sh`, and `git diff --check`.

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-09 | Service name is `mail-rpc`, proto package is `mail.v1`, Go package is `mailpb`. | Matches repo `*-rpc` command/config convention while keeping provider-agnostic mail domain wording. |
| 2026-05-09 | Use standard-library TC3 signing instead of Tencent SDK. | Keeps dependency footprint small and makes request assembly easy to unit test with an `httptest.Server`. |
| 2026-05-09 | Provider failures return gRPC errors instead of successful responses with failed status. | Avoids fake success and keeps callers fail-closed. |
| 2026-05-09 | Auth integration is excluded. | The controller explicitly scoped this issue to the standalone service only. |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
go test ./internal/mail ./internal/rpcgen/mail/internal/logic ./internal/rpcgen/mail/internal/config ./cmd/mail-rpc ./internal/rpcgen/mail
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name '*.go' -print)
go test ./...
bash scripts/verify-static.sh
git diff --check
```

## 风险与回滚

- Tencent SES API compatibility risk is bounded by provider unit tests that assert the JSON action payload and TC3 headers against a local HTTP server.
- If credentials are absent, `mail-rpc` startup fails through config validation instead of running a disabled or fake provider.
- Rollback is removing `mail-rpc` files and static entries; no database migration or external API contract is changed.

## 结果记录

- Added internal-only `mail-rpc` with `SendTemplateEmail`.
- Added Tencent Cloud SES adapter with TC3-signed HTTP requests and provider error normalization.
- Added config placeholders for `TENCENT_SES_SECRET_ID` and `TENCENT_SES_SECRET_KEY`; no real Tencent Cloud secret values are committed.
- Verified with focused tests, API validation, full Go tests, static verification, and `git diff --check`.
