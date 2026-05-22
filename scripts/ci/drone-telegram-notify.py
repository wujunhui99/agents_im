#!/usr/bin/env python3
"""Send Drone CI/CD result notifications to Telegram with agent attribution.

Attribution is resolved from trusted project metadata, not free-form chat:
1. PR source branch path: <type>/<agent>/issue-<number>-<task>
2. commit subject marker: <type>(<scope>)[<agent>]: <title>
3. commit trailer: Agent: <agent>
4. commit author email: <agent>@agents.noreply.local

Development PR notifications are attributed to the responsible feature agent.
Integration/release push notifications on ``main`` and ``devops`` are owned by
Eino because they may include changes from multiple agents and need CI/CD
triage before routing to a feature owner.
"""
from __future__ import annotations

import json
import os
import re
import subprocess
import sys
import time
import urllib.parse
import urllib.request
from dataclasses import dataclass
from typing import Optional

TRUSTED_AGENTS = {
    "eino": {
        "mention": "@eino_hermes_bot",
        "email": "eino@agents.noreply.local",
    },
    "helios": {
        "mention": "@ws_ubuntu_claw_bot",
        "email": "helios@agents.noreply.local",
    },
    "hermes": {
        "mention": "@ws_ubuntu_hermes_bot",
        "email": "hermes@agents.noreply.local",
    },
    "achilles": {
        "mention": "@achilles_hermes_bot",
        "email": "achilles@agents.noreply.local",
    },
    "furies": {
        "mention": "@furies_hermes_bot",
        "email": "furies@agents.noreply.local",
    },
    "gaia": {
        "mention": "@gaia_hermes_bot",
        "email": "gaia@agents.noreply.local",
    },
}

BRANCH_RE = re.compile(
    r"^(feature|fix|refactor|docs|test|chore|ci|perf|style|hotfix)/"
    r"(?P<agent>eino|helios|hermes|achilles|furies|gaia)/"
    r"issue-[0-9]+-[a-z0-9][a-z0-9-]*$"
)
TRAILER_RE = re.compile(r"^Agent:\s*([a-z0-9_-]+)\s*$", re.MULTILINE)
SUBJECT_AGENT_RE = re.compile(r"(?:^|\s)([a-z]+)(?:\([a-z0-9_.-]+\))?\[(eino|helios|hermes|achilles|furies|gaia)\]:")


@dataclass
class Attribution:
    agent: Optional[str]
    method: str
    mention: str
    warnings: list[str]


def env(name: str, default: str = "") -> str:
    return os.environ.get(name, default).strip()


def git_output(*args: str) -> str:
    try:
        return subprocess.check_output(["git", *args], text=True, stderr=subprocess.DEVNULL).strip()
    except Exception:
        return ""


def commit_message() -> str:
    return env("DRONE_COMMIT_MESSAGE") or git_output("log", "-1", "--format=%B")


def commit_author_email() -> str:
    return env("DRONE_COMMIT_AUTHOR_EMAIL") or git_output("log", "-1", "--format=%ae")


def commit_author_name() -> str:
    return env("DRONE_COMMIT_AUTHOR") or git_output("log", "-1", "--format=%an")


def agent_from_branch(branch: str) -> Optional[str]:
    branch = branch or ""
    match = BRANCH_RE.match(branch)
    if match:
        return match.group("agent")

    # Legacy compatibility: older open PRs may predate the hard branch gate and
    # use names such as `ci/telegram-drone-notify`. When no stronger signal is
    # present, infer from known task slugs so success/failure notifications still
    # mention the responsible agent instead of silently omitting @mentions.
    legacy_slug_map = {
        "telegram-drone-notify": "eino",
    }
    tail = branch.split("/", 1)[1] if "/" in branch else branch
    return legacy_slug_map.get(tail)


def agent_from_trailer(message: str) -> Optional[str]:
    match = TRAILER_RE.search(message or "")
    if not match:
        return None
    candidate = match.group(1).strip().lower()
    return candidate if candidate in TRUSTED_AGENTS else None


def agent_from_subject(message: str) -> Optional[str]:
    first_line = (message or "").splitlines()[0] if message else ""
    match = SUBJECT_AGENT_RE.search(first_line.strip())
    if not match:
        return None
    candidate = match.group(2).strip().lower()
    return candidate if candidate in TRUSTED_AGENTS else None


def agent_from_email(email: str) -> Optional[str]:
    email = (email or "").strip().lower()
    for agent, meta in TRUSTED_AGENTS.items():
        if email == meta["email"]:
            return agent
    return None


def integration_push_owner(branch: str, event: str) -> Optional[Attribution]:
    if (event or "").lower() != "push":
        return None
    if branch not in {"main", "devops"}:
        return None

    method = "main release owner" if branch == "main" else "devops owner"
    return Attribution(
        agent="eino",
        method=method,
        mention=TRUSTED_AGENTS["eino"]["mention"],
        warnings=[],
    )


def resolve_attribution() -> Attribution:
    event = env("DRONE_BUILD_EVENT") or env("DRONE_EVENT")
    source_branch = env("DRONE_SOURCE_BRANCH")
    commit_branch = env("DRONE_COMMIT_BRANCH")
    branch = source_branch or commit_branch

    integration_owner = integration_push_owner(commit_branch or branch, event)
    if integration_owner:
        return integration_owner

    message = commit_message()
    email = commit_author_email()

    branch_agent = agent_from_branch(branch)
    trailer_agent = agent_from_trailer(message)
    subject_agent = agent_from_subject(message)
    email_agent = agent_from_email(email)

    warnings: list[str] = []
    observed = {
        "branch": branch_agent,
        "trailer": trailer_agent,
        "subject": subject_agent,
        "email": email_agent,
    }
    non_empty = {k: v for k, v in observed.items() if v}
    if len(set(non_empty.values())) > 1:
        detail = ", ".join(f"{k}={v}" for k, v in non_empty.items())
        warnings.append(f"Attribution mismatch: {detail}")

    # Development PRs are attributed to the feature owner. The branch is the
    # first choice for PR notifications because CI hard-rejects new source
    # branches without a trusted agent in the second path segment. Integration
    # pushes to main/devops return early above and are always owned by Eino.
    if branch_agent:
        agent = branch_agent
        method = "branch" if BRANCH_RE.match(branch or "") else "legacy branch"
    elif subject_agent:
        agent = subject_agent
        method = "commit subject"
    elif trailer_agent:
        agent = trailer_agent
        method = "commit trailer"
    elif email_agent:
        agent = email_agent
        method = "author email"
    else:
        agent = None
        method = "unresolved"
        warnings.append("No trusted agent found in branch, subject marker, Agent trailer, or author email")

    mention = TRUSTED_AGENTS[agent]["mention"] if agent else ""
    return Attribution(agent=agent, method=method, mention=mention, warnings=warnings)


def short_sha() -> str:
    sha = env("DRONE_COMMIT_SHA") or git_output("rev-parse", "HEAD")
    return sha[:12] if sha else "unknown"


def build_status() -> str:
    status = (sys.argv[1] if len(sys.argv) > 1 else "") or env("DRONE_BUILD_STATUS") or env("DRONE_STAGE_STATUS")
    status = status.lower() or "unknown"
    if status in {"success", "passing", "passed"}:
        return "success"
    if status in {"failure", "failed", "error", "killed"}:
        return "failure"
    return status


def duration_text() -> str:
    started = env("DRONE_BUILD_STARTED")
    if not started.isdigit():
        return "unknown"
    seconds = max(0, int(time.time()) - int(started))
    minutes, sec = divmod(seconds, 60)
    if minutes:
        return f"{minutes}m{sec:02d}s"
    return f"{sec}s"


def first_line(text: str) -> str:
    for line in (text or "").splitlines():
        line = line.strip()
        if line:
            return line
    return ""


def render_message() -> str:
    status = build_status()
    emoji = "✅" if status == "success" else "❌" if status == "failure" else "ℹ️"
    attr = resolve_attribution()

    repo = env("DRONE_REPO", "wujunhui99/agents_im")
    event = env("DRONE_BUILD_EVENT") or env("DRONE_EVENT") or "unknown"
    build_no = env("DRONE_BUILD_NUMBER", "unknown")
    stage = env("DRONE_STAGE_NAME") or env("DRONE_JOB_NAME") or "unknown"
    source = env("DRONE_SOURCE_BRANCH")
    target = env("DRONE_TARGET_BRANCH")
    branch = source or env("DRONE_COMMIT_BRANCH", "unknown")
    branch_line = f"{source} -> {target}" if source and target else branch
    author_name = commit_author_name()
    author_email = commit_author_email()
    author = f"{author_name} <{author_email}>" if author_email else author_name or "unknown"
    subject = first_line(commit_message()) or env("DRONE_COMMIT_MESSAGE", "") or "unknown"
    link = env("DRONE_BUILD_LINK") or env("DRONE_SYSTEM_PROTO", "https") + "://" + env("DRONE_SYSTEM_HOST", "drone.agenticim.xyz")

    agent_display = "unresolved"
    if attr.agent:
        agent_display = f"{attr.agent} {attr.mention}"

    lines = [
        f"agents_im CI/CD {status} {emoji}",
        f"Repo: {repo}",
        f"Build: #{build_no} ({event})",
        f"Pipeline: {stage}",
        f"Branch: {branch_line}",
        f"Commit: {short_sha()}",
        f"Commit message: {subject}",
        f"Author: {author}",
        f"Agent: {agent_display}",
        f"Attribution: {attr.method}",
        f"Duration: {duration_text()}",
        f"Logs: {link}",
    ]
    if attr.warnings:
        lines.append("Warnings:")
        lines.extend(f"- {warning}" for warning in attr.warnings)
    return "\n".join(lines)


def send_telegram(text: str) -> None:
    token = env("TELEGRAM_BOT_TOKEN")
    chat_id = env("TELEGRAM_CHAT_ID")
    if env("DRONE_TELEGRAM_DRY_RUN") == "1":
        print(text)
        return
    if not token or not chat_id:
        print("TELEGRAM_BOT_TOKEN or TELEGRAM_CHAT_ID is not configured; skipping Telegram notification")
        print(text)
        return

    data = urllib.parse.urlencode(
        {
            "chat_id": chat_id,
            "text": text,
            "disable_web_page_preview": "true",
        }
    ).encode()
    url = f"https://api.telegram.org/bot{token}/sendMessage"
    request = urllib.request.Request(url, data=data, method="POST")
    try:
        with urllib.request.urlopen(request, timeout=20) as response:
            payload = response.read().decode("utf-8", errors="replace")
            if response.status >= 300:
                raise RuntimeError(f"Telegram API returned HTTP {response.status}: {payload}")
            parsed = json.loads(payload)
            if not parsed.get("ok"):
                raise RuntimeError(f"Telegram API returned not ok: {payload}")
    except Exception as exc:
        print(f"failed to send Telegram notification: {exc}", file=sys.stderr)
        raise


if __name__ == "__main__":
    send_telegram(render_message())
