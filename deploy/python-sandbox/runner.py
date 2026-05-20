import json
import os
import subprocess
import sys


def positive_int_env(name, default):
    raw = os.environ.get(name, "").strip()
    if not raw:
        return default
    try:
        value = int(raw)
    except ValueError:
        return default
    if value <= 0:
        return default
    return value


def truncate_outputs(stdout, stderr, result_json, max_bytes):
    remaining = max_bytes
    truncated = False

    stdout_bytes = stdout.encode("utf-8", errors="replace")
    if len(stdout_bytes) > remaining:
        stdout = stdout_bytes[:remaining].decode("utf-8", errors="replace")
        remaining = 0
        truncated = True
    else:
        remaining -= len(stdout_bytes)

    stderr_bytes = stderr.encode("utf-8", errors="replace")
    if len(stderr_bytes) > remaining:
        stderr = stderr_bytes[:remaining].decode("utf-8", errors="replace")
        remaining = 0
        truncated = True
    else:
        remaining -= len(stderr_bytes)

    result_bytes = json.dumps(result_json, separators=(",", ":")).encode("utf-8")
    if len(result_bytes) > remaining:
        result_json = None
        truncated = True

    return stdout, stderr, result_json, truncated


def main():
    code_path = os.environ.get("PYTHON_EXECUTOR_CODE_PATH", "/sandbox/input/main.py")
    max_output_bytes = positive_int_env("PYTHON_EXECUTOR_MAX_OUTPUT_BYTES", 65536)
    timeout_seconds = positive_int_env("PYTHON_EXECUTOR_TIMEOUT_SECONDS", 10)

    output = {
        "stdout": "",
        "stderr": "",
        "result_json": None,
        "exit_code": 1,
        "timed_out": False,
        "output_truncated": False,
        "error": None,
    }

    if not os.path.isfile(code_path):
        output["error"] = {"code": "input_missing", "message": "code file is missing"}
        print(json.dumps(output, separators=(",", ":")), flush=True)
        return 1

    try:
        completed = subprocess.run(
            [sys.executable, "-I", code_path],
            check=False,
            capture_output=True,
            text=True,
            timeout=timeout_seconds,
        )
    except subprocess.TimeoutExpired as exc:
        stdout = exc.stdout or ""
        stderr = exc.stderr or ""
        stdout, stderr, result_json, truncated = truncate_outputs(stdout, stderr, None, max_output_bytes)
        output.update(
            {
                "stdout": stdout,
                "stderr": stderr,
                "result_json": result_json,
                "exit_code": -1,
                "timed_out": True,
                "output_truncated": truncated,
                "error": {"code": "timeout", "message": "execution timed out"},
            }
        )
        print(json.dumps(output, separators=(",", ":")), flush=True)
        return 124

    stdout, stderr, result_json, truncated = truncate_outputs(completed.stdout, completed.stderr, None, max_output_bytes)
    output.update(
        {
            "stdout": stdout,
            "stderr": stderr,
            "result_json": result_json,
            "exit_code": completed.returncode,
            "output_truncated": truncated,
        }
    )
    if completed.returncode != 0:
        output["error"] = {"code": "execution_failed", "message": "user code exited non-zero"}

    print(json.dumps(output, separators=(",", ":")), flush=True)
    return 0 if completed.returncode == 0 else completed.returncode


if __name__ == "__main__":
    raise SystemExit(main())
