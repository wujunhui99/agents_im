# Issue 80 Email Code MailRPC Client Fix Plan

**Goal:** Make `POST /auth/register/email-code` receive a real Mail RPC client from list-shaped config and fail safely when the dependency is absent.

**Root Cause Hypothesis:** `auth-api` owns the HTTP route and already attempts to build a Mail RPC client, but `internal/config.LoadAPIConfig` flattens `MailRPC.Endpoints:` YAML lists as an empty scalar. Production list-shaped config therefore becomes an empty `zrpc.RpcClientConf`, `NewOptionalRPCClient` returns nil, and Auth logic reports the missing mail client at request time.

**TDD Steps:**

- [x] Trace whether the HTTP route is served by `auth-api` or proxied to `auth-rpc`.
- [x] Inspect MailRPC config shape in local and k8s configs.
- [x] Add red tests for list-shaped MailRPC config parsing.
- [x] Add a red integration test proving a list-shaped auth-api config creates a Mail RPC sender used by the email-code path.
- [x] Add red tests for missing mail dependency returning a safe typed service-unavailable response and not persisting a code token.
- [x] Fix YAML list parsing and require MailRPC at startup boundaries.
- [x] Add static verification for scalar MailRPC endpoints in auth configs.
- [x] Run required verification.
- [ ] Commit, push, open PR, and comment on Issue #80.
