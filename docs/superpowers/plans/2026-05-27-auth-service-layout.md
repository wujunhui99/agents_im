# Auth Service Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move auth API and RPC runtime code into `service/auth/api` and `service/auth/rpc` while preserving existing HTTP and gRPC contracts.

**Architecture:** Keep authentication business and repository wiring in the auth RPC service. Regenerate go-zero API and RPC scaffold under `service/auth`, then make the auth API a BFF that calls `AuthRPC` instead of constructing repositories directly.

**Tech Stack:** Go 1.24, go-zero 1.10.1, `goctl api go`, `goctl rpc protoc`, gRPC, repo-local auth/account business packages.

---

### Task 1: Generate Auth API And RPC Scaffold

**Files:**
- Create: `service/auth/api/auth.api`
- Create: `service/auth/rpc/auth.proto`
- Create/update generated files under `service/auth/api/**` and `service/auth/rpc/**`

- [ ] **Step 1: Copy canonical contracts into service layout**

```bash
mkdir -p service/auth/api service/auth/rpc
cp api/auth.api service/auth/api/auth.api
cp proto/auth.proto service/auth/rpc/auth.proto
```

- [ ] **Step 2: Update service-layout proto package path**

Edit `service/auth/rpc/auth.proto` so `go_package` is:

```proto
option go_package = "github.com/wujunhui99/agents_im/service/auth/rpc/auth";
```

- [ ] **Step 3: Validate and generate API scaffold**

```bash
goctl api validate -api service/auth/api/auth.api
rm -rf /tmp/agents-im-auth-api-goctl
mkdir -p /tmp/agents-im-auth-api-goctl
goctl api go -api service/auth/api/auth.api -dir /tmp/agents-im-auth-api-goctl --style go_zero
goctl api go -api service/auth/api/auth.api -dir service/auth/api --style go_zero
```

- [ ] **Step 4: Generate RPC scaffold with module flags**

```bash
goctl rpc protoc service/auth/rpc/auth.proto \
  --go_out=. --go_opt=module=github.com/wujunhui99/agents_im \
  --go-grpc_out=. --go-grpc_opt=module=github.com/wujunhui99/agents_im \
  --zrpc_out=service/auth/rpc \
  --style go_zero \
  --verbose
```

### Task 2: Wire Auth Runtime

**Files:**
- Modify: `cmd/auth-api/main.go`
- Modify: `cmd/auth-rpc/main.go`
- Create: `service/auth/api/entry/entry.go`
- Modify: `service/auth/api/internal/config/config.go`
- Modify: `service/auth/api/internal/svc/service_context.go`
- Modify: `service/auth/api/internal/logic/auth/*.go`
- Create: `service/auth/api/internal/logic/auth/convert.go`
- Create: `service/auth/rpc/entry/entry.go`
- Modify: `service/auth/rpc/internal/config/config.go`
- Modify: `service/auth/rpc/internal/svc/service_context.go`
- Modify: `service/auth/rpc/internal/logic/*.go`

- [ ] **Step 1: Replace `cmd/auth-api` with an entry bridge call**

`cmd/auth-api/main.go` should import only `service/auth/api/entry` and call `authentry.Start(*configFile)`.

- [ ] **Step 2: Replace `cmd/auth-rpc` with the service-layout entry bridge**

`cmd/auth-rpc/main.go` should import only `service/auth/rpc/entry` and call `authentry.Start(*configFile)`.

- [ ] **Step 3: Keep API BFF-only**

`service/auth/api/internal/svc/service_context.go` should require `AuthRPC` client config, construct the goctl-generated auth client wrapper, and avoid repository/model/data-source imports.

- [ ] **Step 4: Map HTTP requests to AuthRPC requests**

`service/auth/api/internal/logic/auth/*.go` should call `RequestRegistrationEmailCode`, `Register`, `Login`, and `ValidateToken` on `svcCtx.AuthRPC`, map responses into existing JSON envelope types, and convert gRPC status errors into repo `apperror` values.

- [ ] **Step 5: Keep auth business wiring in RPC**

`service/auth/rpc/internal/svc/service_context.go` should build existing auth/user repositories, mail RPC client, token manager, auth logic, default assistant bootstrap, and admin bootstrap using RPC config.

### Task 3: Update Configs And Static Checks

**Files:**
- Modify: `etc/auth-api.yaml`
- Modify: `etc/auth-rpc.yaml`
- Modify: `deploy/k8s/etc/auth-api.yaml`
- Modify: `deploy/k8s/etc/auth-rpc.yaml`
- Modify: `scripts/dev-up.sh`
- Modify: `scripts/verify-static.sh`

- [ ] **Step 1: Add `AuthRPC` to auth API configs**

Local config should point to `127.0.0.1:9091`; k8s config should point to `auth-rpc:9091`.

- [ ] **Step 2: Move admin bootstrap config to auth RPC**

Auth API must no longer need storage/admin bootstrap config. Auth RPC owns credentials and account initialization.

- [ ] **Step 3: Update static service-layout checks**

Require `service/auth/api/**`, `service/auth/rpc/**`, colocated protobuf output, and `cmd/auth-*` entry bridge imports. Add auth API BFF checks equivalent to user/friends/groups.

### Task 4: Verify And Handoff

**Files:**
- Commit all task-scoped changes

- [ ] **Step 1: Format touched Go files**

```bash
gofmt -w cmd/auth-api/main.go cmd/auth-rpc/main.go service/auth/api service/auth/rpc
```

- [ ] **Step 2: Run required checks**

```bash
git diff --check
bash scripts/verify-static.sh
go test ./...
```

- [ ] **Step 3: Commit, push, PR, and Issue comment**

```bash
git config user.name "Hermes (AI Agent)"
git config user.email "hermes@agents.noreply.local"
git add .
git commit -m "refactor(auth)[hermes]: move auth services into service layout"
git push -u origin HEAD
gh pr create --base main --title "refactor(auth)[hermes]: move auth services into service layout" --body-file /tmp/auth-service-layout-pr.md
gh issue comment 291 --body-file /tmp/auth-service-layout-issue-comment.md
```
