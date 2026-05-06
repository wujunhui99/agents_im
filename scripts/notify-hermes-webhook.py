#!/usr/bin/env python3
"""Notify Hermes Agent about GitHub Actions completion.

Sends a compact JSON payload to the Hermes webhook endpoint using the same
HMAC-SHA256 header format as GitHub webhooks:

    X-Hub-Signature-256: sha256=<hmac hex>

Required env:
  HERMES_WEBHOOK_SECRET
Optional env:
  HERMES_WEBHOOK_URL (defaults to https://agenticim.xyz/hermes/webhook)
"""

from __future__ import annotations

import hashlib
import hmac
import json
import os
import sys
import urllib.error
import urllib.request


def env(name: str, default: str = "") -> str:
    return os.environ.get(name, default)


def main() -> int:
    secret = env("HERMES_WEBHOOK_SECRET")
    if not secret:
        print("HERMES_WEBHOOK_SECRET is required", file=sys.stderr)
        return 2

    webhook_url = env("HERMES_WEBHOOK_URL", "https://agenticim.xyz/hermes/webhook")
    payload = {
        "repo": env("GITHUB_REPOSITORY"),
        "branch": env("GITHUB_REF_NAME"),
        "ref": env("GITHUB_REF"),
        "status": env("WORKFLOW_STATUS", "unknown"),
        "workflow": env("GITHUB_WORKFLOW"),
        "job": env("GITHUB_JOB"),
        "run_id": env("GITHUB_RUN_ID"),
        "run_number": env("GITHUB_RUN_NUMBER"),
        "run_attempt": env("GITHUB_RUN_ATTEMPT"),
        "run_url": (
            f"https://github.com/{env('GITHUB_REPOSITORY')}/actions/runs/{env('GITHUB_RUN_ID')}"
            if env("GITHUB_REPOSITORY") and env("GITHUB_RUN_ID")
            else ""
        ),
        "sha": env("GITHUB_SHA"),
        "actor": env("GITHUB_ACTOR"),
        "event_name": env("GITHUB_EVENT_NAME"),
    }
    body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    signature = "sha256=" + hmac.new(secret.encode("utf-8"), body, hashlib.sha256).hexdigest()
    req = urllib.request.Request(
        webhook_url,
        data=body,
        method="POST",
        headers={
            "Content-Type": "application/json",
            "User-Agent": "agents-im-github-actions-hermes-webhook/1.0",
            "X-GitHub-Event": "github-actions",
            "X-GitHub-Delivery": f"{env('GITHUB_RUN_ID')}-{env('GITHUB_RUN_ATTEMPT')}-{env('GITHUB_JOB')}",
            "X-Hub-Signature-256": signature,
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            text = resp.read().decode("utf-8", errors="replace")
            print(f"Hermes webhook response: HTTP {resp.status} {text[:500]}")
            return 0 if 200 <= resp.status < 300 else 1
    except urllib.error.HTTPError as exc:
        text = exc.read().decode("utf-8", errors="replace")
        print(f"Hermes webhook error: HTTP {exc.code} {text[:500]}", file=sys.stderr)
        return 1
    except Exception as exc:
        print(f"Hermes webhook request failed: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
