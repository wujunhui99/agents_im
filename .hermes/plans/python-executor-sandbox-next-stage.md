# Sandboxed Python Executor Service Implementation Plan

> **For Hermes:** Use Codex/subagent-driven-development to implement this plan task-by-task.

**Goal:** Implement a real opt-in sandboxed Python executor backend for `python.execute` while preserving fail-closed defaults and avoiding in-process Python execution inside Agent Service.

**Architecture:** Add a Kubernetes sandbox executor under `internal/agent/pythonexec` that converts validated `pythonexec.Request` policies into per-request Kubernetes Job/Pod manifests and collects bounded stdout/stderr. Runtime wiring stays disabled by default; production enablement requires explicit config and admin tool binding. Unit tests verify manifest/security/policy behavior without a live cluster; live execution tests are opt-in only.

**Tech Stack:** Go, Kubernetes client-go or typed manifest builders, existing `pythonexec.Executor` contract, existing tools adapter foundation from #104, k3s deployment docs.

---

## Task 1: Add executor configuration types

**Objective:** Represent disabled vs k8s sandbox backend explicitly, with disabled as the default.

**Files:**
- Modify: `internal/config/config.go` or the existing config package holding service config types.
- Test: config package tests if present, or new unit test near config parsing.

**Behavior:**
- Add `PythonExecutorConfig` with fields similar to:
  - `Backend string` (`disabled`, `k8s`)
  - `K8S Namespace string`
  - `K8S Image string`
  - `K8S ServiceAccountName string` optional; sandbox pod should not mount its token.
  - `K8S RuntimeClassName string` optional.
  - `K8S DefaultTimeoutSeconds`, `MaxTimeoutSeconds`, `DefaultMemoryMiB`, `MaxOutputBytes` if useful.
- Missing/empty config means disabled.
- Invalid backend should fail config validation if the service has validation hooks.

**TDD:**
1. Write a test asserting zero-value config resolves to disabled.
2. Write a test asserting `Backend=k8s` requires namespace and image.
3. Run targeted config tests and watch them fail before implementation.

## Task 2: Add Kubernetes manifest builder for sandbox jobs/pods

**Objective:** Convert `pythonexec.Request` into a secure Kubernetes Job/Pod spec without contacting a cluster.

**Files:**
- Create: `internal/agent/pythonexec/k8s_executor.go` or `k8s_manifest.go`
- Create: `internal/agent/pythonexec/k8s_executor_test.go`

**Behavior:**
- Build a per-run Job or Pod name derived from run/audit id with safe DNS label truncation.
- Container image comes from config.
- Runner receives code via ConfigMap/Secret/projected volume or stdin contract; do not put raw code in pod name/labels/log labels.
- Security requirements:
  - `runAsNonRoot: true`
  - `allowPrivilegeEscalation: false`
  - drop all Linux capabilities
  - `automountServiceAccountToken: false`
  - no `hostPath`
  - no privileged containers
  - resource limits for CPU/memory based on policy
  - restartPolicy never
- Network remains default-deny by namespace NetworkPolicy docs; manifest must not request hostNetwork.
- File allowlist is represented as read-only mounted inputs only; if actual file materialization is not ready, reject non-empty allowlist visibly rather than faking support.

**TDD:**
- Test manifest includes non-root/no-token/no-hostNetwork/no-hostPath/drop capabilities.
- Test memory/output/timeout policy validation.
- Test non-empty file allowlist fails until materialization exists.

## Task 3: Implement k8s executor lifecycle with injectable client interface

**Objective:** Implement `pythonexec.Executor` using an interface around Kubernetes create/watch/log/delete so unit tests do not need a cluster.

**Files:**
- Modify/create: `internal/agent/pythonexec/k8s_executor.go`
- Test: `internal/agent/pythonexec/k8s_executor_test.go`

**Behavior:**
- Constructor: `NewKubernetesExecutor(config, client)` or equivalent.
- `Execute(ctx, req)`:
  1. validate request/policy;
  2. reject network-enabled policies;
  3. create sandbox workload;
  4. wait until success/failure or timeout/context cancellation;
  5. collect stdout/stderr bounded by `MaxOutputBytes`;
  6. delete/cleanup workload best-effort;
  7. return structured `pythonexec.Response`.
- Distinguish timeout, execution failure, infrastructure failure.
- No shell/os exec in Agent Service.

**TDD:**
- Fake client success returns stdout and exit code 0.
- Fake timeout returns TimedOut true and cleanup called.
- Fake log larger than MaxOutputBytes truncates and sets OutputTruncated.
- Create failure returns visible error.

## Task 4: Add runner contract / image scaffold

**Objective:** Define how the sandbox container receives code and emits stdout/stderr/result.

**Files:**
- Create: `deploy/python-sandbox/README.md`
- Optionally create: `deploy/python-sandbox/Dockerfile`
- Optionally create: `deploy/python-sandbox/runner.py`

**Behavior:**
- Runner image should be non-root compatible.
- No network dependency.
- No pip install at runtime.
- Code input path and output contract documented.
- If adding `runner.py`, keep it minimal and tested only as a local artifact, not used by Agent Service process.

## Task 5: Wire disabled/k8s executor construction into service context behind config

**Objective:** Make future runtime wiring able to inject the executor, while default remains disabled.

**Files:**
- Modify relevant service context package if agent runtime local adapter catalog is already constructed there.
- Otherwise add a small factory in `internal/agent/pythonexec` and document usage.

**Behavior:**
- Default config returns `DisabledExecutor`.
- `Backend=k8s` returns k8s executor only with valid namespace/image and Kubernetes client availability.
- Do not enable `python.execute` for all agents automatically.

## Task 6: Deployment docs and opt-in integration test

**Objective:** Document production requirements and add optional smoke path without making CI require a cluster.

**Files:**
- Modify: `docs/design-docs/python-executor-sandbox.md`
- Modify: `deploy/README.md` or add `deploy/python-executor/README.md`
- Optional: `internal/agent/pythonexec/k8s_integration_test.go`

**Behavior:**
- Document namespace, RBAC, NetworkPolicy deny-all, image, service account, cleanup, observability.
- Integration test must skip unless `AGENTS_IM_PYTHON_EXECUTOR_K8S_TEST=1` and required env vars are set.

## Final verification

Run:
```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
gofmt -w <changed-go-files>
go test ./internal/agent/pythonexec ./internal/agentruntime/tools ./internal/servicecontext/...
go test ./...
bash scripts/verify-static.sh
git diff --check
```

If Kubernetes/client-go dependency is added, verify `go mod tidy` and commit `go.mod`/`go.sum`. Do not claim live k8s execution passed unless the opt-in integration test actually ran against a configured cluster.
