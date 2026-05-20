# Python Executor Sandbox

Status: Accepted

## Background

Agents need a `python.execute` tool for bounded calculations and data processing. Running Python inside the Agent Service process, through shell commands, or through host-mounted containers would make prompt/tool input equivalent to production code execution. V1 therefore provides only the Go-side foundation and keeps real execution disabled by default.

## V1 Foundation

V1 exposes Python execution as a local tool handler:

```text
handler_key = python.execute
```

The registry and resolver may represent this handler only through the local-tool whitelist. A runtime that requires callable tools must request adapters explicitly. The safe adapter is `internal/agentruntime/tools.PythonExecuteAdapter`, backed by the injected `internal/agent/pythonexec.Executor` contract.

Default wiring remains fail-closed:

- `pythonexec.NewDefaultExecutor()` returns `DisabledExecutor`.
- `DisabledExecutor` validates request/policy and returns `ErrPythonExecutorDisabled`.
- `NewDefaultLocalAdapterCatalog(nil)` creates the Python adapter with the disabled executor.
- No V1 code starts Python, shell, Docker, network calls, or local processes.

The adapter accepts strict JSON input:

```json
{
  "code": "print(1 + 1)",
  "timeout_seconds": 10,
  "files": []
}
```

`code` is required. `timeout_seconds` defaults to 10 seconds and is capped at 30 seconds. `files` must be empty unless the caller injects an explicit allowlist mapping; the adapter does not read host paths or infer files from the filesystem.

The adapter builds a restrictive `pythonexec.Policy`:

- network disabled;
- timeout from input or default;
- CPU limit no greater than timeout;
- memory limit default 256 MiB;
- max stdout/stderr/result output default 64 KiB;
- explicit read-only file allowlist, empty by default.

Tool output is structured JSON containing `stdout`, `stderr`, `result_json`, `exit_code`, `timed_out`, `output_truncated`, and `error`. Executor failures remain visible errors; they must not be converted into successful tool results.

## Safety Boundary

MCP can be used later as a transport/protocol around a Python executor service, but MCP is not a sandbox and is not the safety boundary. The safety boundary must be an independent executor service or isolated Kubernetes pod outside the Agent Service process.

Production execution must not be implemented by:

- shell invocation;
- `os/exec` in Agent Service;
- in-process Python interpreters;
- mounting the host filesystem into the executor;
- exposing the Docker socket;
- enabling network by default;
- arbitrary database command or script execution.

## Future Executor Service

A real executor should run each request in an isolated sandbox with:

- per-request timeout and cancellation;
- CPU and memory limits enforced by the runtime/container boundary;
- network disabled by default, with any exception controlled by admin policy;
- no host directory mounts;
- read-only copies of explicitly authorized skill files;
- bounded stdout/stderr/result capture and truncation flags;
- structured errors for policy violation, timeout, runtime failure, and output truncation;
- audit records for request id, run id, code hash/size, resource policy, file allowlist, result summaries, and errors.

Executor integration tests must be opt-in and skipped by default unless the sandbox service and required environment are explicitly configured. Default `go test ./...` must stay local, deterministic, and independent of Python, Docker, network, or provider credentials.

## Kubernetes Backend Foundation

The next-stage backend adds an opt-in Kubernetes executor under `internal/agent/pythonexec`.

Default behavior remains fail-closed:

- missing `PythonExecutor` config resolves to `Backend: disabled`;
- local/dev and production example configs set `PythonExecutor.Backend: disabled`;
- `agent-api` constructs a disabled executor unless config explicitly selects `k8s`;
- selecting `k8s` without a Kubernetes client, namespace, or image is a startup/config error, not a fallback.

The Kubernetes backend creates one Job and one ConfigMap per execution. The ConfigMap contains only the submitted code file and is mounted read-only at `/sandbox/input/main.py`; raw code is not placed in object names, labels, or annotations. Non-empty file allowlists are rejected until explicit file materialization exists.

Per-run manifests enforce:

- `automountServiceAccountToken: false`;
- `runAsNonRoot: true`;
- `allowPrivilegeEscalation: false`;
- all Linux capabilities dropped;
- read-only root filesystem plus memory-backed `/tmp`;
- no `hostNetwork`, `hostPID`, `hostIPC`, `hostPath`, privileged containers, shell command, Docker socket, or host filesystem mount;
- `restartPolicy: Never`, `backoffLimit: 0`, active deadline from request timeout, and CPU/memory/output caps from policy/config.

The runner image contract lives in [`../../deploy/python-sandbox/README.md`](../../deploy/python-sandbox/README.md). The image must be prebuilt with any allowed packages; runtime `pip install` or network dependency is not part of the contract.

Cluster requirements before enabling:

- dedicated namespace, for example `agent-python-sandbox`;
- default-deny ingress and egress NetworkPolicy for sandbox Pods;
- tightly scoped Agent Service RBAC that can create/watch/read/delete only the sandbox Jobs, ConfigMaps, and logs it owns;
- a sandbox Pod service account with token automount disabled by manifest;
- Pod Security admission or equivalent policy forbidding privileged pods, host namespaces, hostPath, and privilege escalation;
- image provenance and pinned tags/digests for the runner image;
- observability on Job creation/failure/timeout/cleanup without logging submitted code or secrets.

Live Kubernetes execution is not part of default test gates. Add or run live smoke only when `AGENTS_IM_PYTHON_EXECUTOR_K8S_TEST=1` and the namespace/image/client configuration are explicitly provided.

## Verification

V1 foundation verification:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
go test ./internal/agentruntime/tools ./internal/agent/pythonexec ./internal/domain/agentaudit
go test ./...
bash scripts/verify-static.sh
git diff --check
```
