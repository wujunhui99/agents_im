# Python sandbox runner image

This directory defines the container contract used by the Kubernetes `python.execute` backend.

The Agent Service never starts local interpreters, shells, Docker containers, or host-mounted processes. When explicitly enabled with `PythonExecutor.Backend: k8s`, it creates one Kubernetes Job per request and mounts the submitted code as a read-only ConfigMap file:

```text
/sandbox/input/main.py
```

The runner image must:

- run as a non-root user compatible with UID/GID `65532`;
- need no network access during execution;
- avoid runtime `pip install` or package downloads;
- read `PYTHON_EXECUTOR_CODE_PATH`;
- respect `PYTHON_EXECUTOR_MAX_OUTPUT_BYTES` and `PYTHON_EXECUTOR_TIMEOUT_SECONDS`;
- emit one JSON object to stdout with `stdout`, `stderr`, `result_json`, `exit_code`, `timed_out`, `output_truncated`, and `error`.

The Kubernetes namespace that hosts these Jobs must provide the real isolation controls: default-deny NetworkPolicy, tight RBAC for the Agent Service, Pod Security admission or equivalent, and no Docker socket or hostPath permissions.

Build example:

```bash
docker build -t ghcr.io/wujunhui99/agents_im/python-sandbox:dev deploy/python-sandbox
```

Do not configure this image in production until the sandbox namespace, service account, RBAC, and NetworkPolicy are applied and reviewed.
