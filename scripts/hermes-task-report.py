#!/usr/bin/env python3
"""Record Codex task execution reports for Hermes analysis.

This CLI is for Hermes/controller use, not for Codex. It stores one JSONL row per
Codex instance/task so later analysis can find efficiency blockers and update
context, AGENTS.md, or hard constraints.

Default output is intentionally git-ignored: .hermes/task-reports/codex-runs.jsonl
"""

from __future__ import annotations

import argparse
import json
import os
import re
import urllib.error
import urllib.request
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

DEFAULT_OUTPUT = Path(".hermes/task-reports/codex-runs.jsonl")
DEFAULT_REPO = "wujunhui99/agents_im"
ISSUE_RE = re.compile(r"^(?:#)?(?P<num>\d+)$")
URL_RE = re.compile(r"^https?://")

SECRET_KEYWORDS = (
    "password",
    "passwd",
    "token",
    "secret",
    "apikey",
    "api_key",
    "access_key",
    "secret_key",
    "private_key",
    "cookie",
    "jwt",
    "dsn",
    "database_url",
    "object_storage",
    "minio",
    "ssh_host",
    "ssh_user",
    "ssh_port",
    "ssh_key",
)

SAFE_SECRET_WORD_KEYS = {"tokens_used"}
DEFAULT_PUBLISH_URL = "https://ms.agenticim.xyz/api/admin/task-reports"


def utc_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def parse_dt(value: str) -> datetime:
    value = value.strip()
    if value.endswith("Z"):
        value = value[:-1] + "+00:00"
    try:
        dt = datetime.fromisoformat(value)
    except ValueError as exc:
        raise argparse.ArgumentTypeError(f"invalid datetime {value!r}; use ISO-8601, e.g. 2026-05-25T13:29:14Z") from exc
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(timezone.utc)


def parse_duration_seconds(value: str) -> int:
    raw = value.strip().lower()
    if raw.isdigit():
        return int(raw)
    total = 0
    matched = False
    for num, unit in re.findall(r"(\d+)\s*(h|hr|hrs|hour|hours|m|min|mins|minute|minutes|s|sec|secs|second|seconds)", raw):
        matched = True
        n = int(num)
        if unit.startswith("h"):
            total += n * 3600
        elif unit.startswith("m"):
            total += n * 60
        else:
            total += n
    if not matched:
        raise argparse.ArgumentTypeError("duration must be seconds or like '1h20m', '45m', '90s'")
    return total


def split_multi(values: list[str] | None) -> list[str]:
    if not values:
        return []
    out: list[str] = []
    for value in values:
        for item in value.split(";"):
            item = item.strip()
            if item:
                out.append(item)
    return out


def issue_url(issue: str, repo: str) -> str:
    issue = issue.strip()
    if URL_RE.match(issue):
        return issue
    match = ISSUE_RE.match(issue)
    if not match:
        raise argparse.ArgumentTypeError("--issue must be a GitHub issue number like 236, #236, or a URL")
    return f"https://github.com/{repo}/issues/{match.group('num')}"


def issue_number(issue: str) -> int | None:
    issue = issue.strip()
    if URL_RE.match(issue):
        m = re.search(r"/issues/(\d+)(?:$|[/?#])", issue)
        return int(m.group(1)) if m else None
    m = ISSUE_RE.match(issue)
    return int(m.group("num")) if m else None


def git_value(args: list[str]) -> str | None:
    try:
        result = subprocess.run(["git", *args], check=False, text=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    except OSError:
        return None
    value = result.stdout.strip()
    return value or None


def contains_secret_key(data: Any) -> list[str]:
    hits: list[str] = []

    def walk(obj: Any, path: str) -> None:
        if isinstance(obj, dict):
            for key, value in obj.items():
                key_l = str(key).lower()
                child_path = f"{path}.{key}" if path else str(key)
                if key_l not in SAFE_SECRET_WORD_KEYS and any(word in key_l for word in SECRET_KEYWORDS):
                    hits.append(child_path)
                walk(value, child_path)
        elif isinstance(obj, list):
            for idx, value in enumerate(obj):
                walk(value, f"{path}[{idx}]")

    walk(data, "")
    return hits


def build_record(args: argparse.Namespace) -> dict[str, Any]:
    start_dt = parse_dt(args.started_at) if args.started_at else None
    end_dt = parse_dt(args.ended_at) if args.ended_at else None
    duration = parse_duration_seconds(args.duration) if args.duration else None
    if duration is None and start_dt and end_dt:
        duration = max(0, int((end_dt - start_dt).total_seconds()))

    blockers = split_multi(args.blocker)
    permission_candidates = split_multi(args.permission_candidate)
    major_time_sinks = split_multi(args.major_time_sink)
    lessons = split_multi(args.lesson)
    evidence = split_multi(args.evidence)

    would_permissions_help = args.permissions_help
    if would_permissions_help is None and permission_candidates:
        would_permissions_help = "unknown"

    record: dict[str, Any] = {
        "schema_version": 1,
        "recorded_at": utc_now(),
        "task_id": args.task_id,
        "agent": args.agent,
        "codex_session_id": args.codex_session_id,
        "issue": {
            "number": issue_number(args.issue),
            "url": issue_url(args.issue, args.repo),
        },
        "repo": args.repo,
        "branch": args.branch or git_value(["branch", "--show-current"]),
        "worktree": args.worktree or str(Path.cwd()),
        "commit": args.commit or git_value(["rev-parse", "--short=12", "HEAD"]),
        "outcome": args.outcome,
        "started_at": start_dt.isoformat().replace("+00:00", "Z") if start_dt else None,
        "ended_at": end_dt.isoformat().replace("+00:00", "Z") if end_dt else None,
        "duration_seconds": duration,
        "tokens_used": args.tokens_used,
        "pr_url": args.pr_url,
        "evidence": evidence,
        "blockers": blockers,
        "major_time_sinks": major_time_sinks,
        "permission_analysis": {
            "would_more_permission_help": would_permissions_help,
            "candidate_permissions": permission_candidates,
            "reason": args.permission_reason,
        },
        "pitfalls_or_lessons": lessons,
        "notes": args.notes,
    }
    return record


def publish_record(record: dict[str, Any], url: str, token: str | None, timeout: int = 15) -> None:
    body = json.dumps(record, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(url, data=body, method="POST")
    request.add_header("Content-Type", "application/json")
    if token:
        request.add_header("Authorization", f"Bearer {token}")
    try:
        with urllib.request.urlopen(request, timeout=timeout) as resp:
            if resp.status < 200 or resp.status >= 300:
                raise RuntimeError(f"publish failed with HTTP {resp.status}")
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")[:500]
        raise RuntimeError(f"publish failed with HTTP {exc.code}: {detail}") from exc


def cmd_record(args: argparse.Namespace) -> int:
    record = build_record(args)
    secret_hits = contains_secret_key(record)
    if secret_hits:
        print("Refusing to write report with secret-looking keys: " + ", ".join(secret_hits), file=sys.stderr)
        return 2

    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open("a", encoding="utf-8") as fh:
        fh.write(json.dumps(record, ensure_ascii=False, sort_keys=True) + "\n")
    print(f"wrote {output}: {record['task_id']} -> {record['issue']['url']}")

    publish_url = args.publish_url or os.environ.get("HERMES_TASK_REPORT_PUBLISH_URL")
    publish_token = args.publish_token or os.environ.get("HERMES_TASK_REPORT_PUBLISH_TOKEN")
    if publish_url or args.publish:
        url = publish_url or DEFAULT_PUBLISH_URL
        try:
            publish_record(record, url, publish_token, args.publish_timeout)
        except Exception as exc:  # noqa: BLE001 - CLI should surface publish failures clearly.
            print(f"failed to publish task report to management system: {exc}", file=sys.stderr)
            return 3
        print(f"published task report to {url}: {record['task_id']}")
    return 0


def cmd_summary(args: argparse.Namespace) -> int:
    path = Path(args.input)
    if not path.exists():
        print(f"no report file: {path}")
        return 0
    rows: list[dict[str, Any]] = []
    with path.open("r", encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if line:
                rows.append(json.loads(line))
    if args.limit:
        rows = rows[-args.limit :]
    total = len(rows)
    if total == 0:
        print("no reports")
        return 0
    durations: list[int] = []
    for row in rows:
        duration = row.get("duration_seconds")
        if isinstance(duration, int):
            durations.append(duration)
    blockers: dict[str, int] = {}
    permission_yes = 0
    for row in rows:
        for blocker in row.get("blockers") or []:
            blockers[blocker] = blockers.get(blocker, 0) + 1
        if (row.get("permission_analysis") or {}).get("would_more_permission_help") == "yes":
            permission_yes += 1
    avg = int(sum(durations) / len(durations)) if durations else None
    print(f"reports: {total}")
    if avg is not None:
        print(f"avg_duration_seconds: {avg}")
    print(f"permission_help_yes: {permission_yes}")
    if blockers:
        print("top_blockers:")
        for name, count in sorted(blockers.items(), key=lambda item: (-item[1], item[0]))[:10]:
            print(f"- {name}: {count}")
    return 0


def add_recording_args(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--output", default=str(DEFAULT_OUTPUT), help=f"JSONL output path, default {DEFAULT_OUTPUT}")
    parser.add_argument("--repo", default=DEFAULT_REPO)
    parser.add_argument("--task-id", required=True, help="Hermes/controller task id, e.g. codex-issue-236-feedback-ui-admin")
    parser.add_argument("--agent", default="codex")
    parser.add_argument("--codex-session-id")
    parser.add_argument("--issue", required=True, help="GitHub issue number, #number, or issue URL")
    parser.add_argument("--branch")
    parser.add_argument("--worktree")
    parser.add_argument("--commit")
    parser.add_argument("--outcome", required=True, choices=("success", "blocked", "failed", "partial", "cancelled"))
    parser.add_argument("--started-at", help="ISO-8601 UTC timestamp")
    parser.add_argument("--ended-at", help="ISO-8601 UTC timestamp")
    parser.add_argument("--duration", help="seconds or compact duration like 1h20m")
    parser.add_argument("--tokens-used", type=int)
    parser.add_argument("--pr-url")
    parser.add_argument("--evidence", action="append", help="repeatable; command, URL, commit, or verification evidence")
    parser.add_argument("--blocker", action="append", help="repeatable; blocker that delayed the task")
    parser.add_argument("--major-time-sink", action="append", help="repeatable; main time sink even if not a hard blocker")
    parser.add_argument("--permissions-help", choices=("yes", "no", "unknown"), help="would more access/permission likely improve speed?")
    parser.add_argument("--permission-candidate", action="append", help="repeatable; e.g. database, k8s, drone-ci, object-storage")
    parser.add_argument("--permission-reason", help="why the permission would/would not improve speed")
    parser.add_argument("--lesson", action="append", help="repeatable; pitfall or reusable lesson")
    parser.add_argument("--notes")
    parser.add_argument("--publish", action="store_true", help="publish report to Management System after local JSONL write")
    parser.add_argument("--publish-url", help=f"Management System task-report API URL; env HERMES_TASK_REPORT_PUBLISH_URL or default {DEFAULT_PUBLISH_URL}")
    parser.add_argument("--publish-token", help="admin bearer token; env HERMES_TASK_REPORT_PUBLISH_TOKEN")
    parser.add_argument("--publish-timeout", type=int, default=15, help="publish timeout seconds")


def parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(description="Hermes controller CLI for recording Codex task execution reports")
    sub = p.add_subparsers(dest="cmd", required=True)

    rec = sub.add_parser("record", help="append one Codex task execution report")
    add_recording_args(rec)
    rec.set_defaults(func=cmd_record)

    report = sub.add_parser("report", help="alias for record; write a task report and optionally publish it to Management System")
    add_recording_args(report)
    report.set_defaults(func=cmd_record)

    summ = sub.add_parser("summary", help="summarize recorded reports for Hermes retrospectives")
    summ.add_argument("--input", default=str(DEFAULT_OUTPUT), help=f"JSONL input path, default {DEFAULT_OUTPUT}")
    summ.add_argument("--limit", type=int, help="only summarize last N rows")
    summ.set_defaults(func=cmd_summary)
    return p


def main() -> int:
    p = parser()
    args = p.parse_args()
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
