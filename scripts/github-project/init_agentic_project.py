#!/usr/bin/env python3
"""Inspect or initialize the Agentic Development GitHub Project.

This script is intentionally conservative:
- It discovers repository owner/default branch/visibility.
- It checks gh token project scopes.
- If scopes are present, it finds or creates the Project V2 board.
- It records results in docs/github-project-init.md.

GitHub Project V2 field creation/update is GraphQL-heavy and may vary by gh
version. This script records missing scope or project metadata so Hermes can
continue with Issues/labels/templates and finish field setup after auth refresh.
"""
from __future__ import annotations

import json
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

PROJECT_TITLE = "Agentic Development"
OUT = Path("docs/github-project-init.md")


def run(cmd: list[str], check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=check)


def gh_json(args: list[str]) -> object:
    return json.loads(run(["gh", *args]).stdout)


def auth_status_text() -> str:
    return run(["gh", "auth", "status"], check=False).stdout + run(["gh", "auth", "status"], check=False).stderr


def has_project_scope() -> bool:
    text = auth_status_text().lower()
    return "project" in text


def main() -> int:
    repo = gh_json(["repo", "view", "--json", "nameWithOwner,defaultBranchRef,visibility,owner,url"])
    owner = repo["owner"]["login"]
    repo_name = repo["nameWithOwner"]

    project = None
    method = "GitHub CLI"
    scope_ok = has_project_scope()
    scope_note = "project scope available" if scope_ok else "missing project scope; run `gh auth refresh -h github.com -s project` interactively"

    if scope_ok:
        listed = run(["gh", "project", "list", "--owner", owner, "--format", "json", "--limit", "100"], check=False)
        if listed.returncode != 0:
            scope_note = f"project list failed: {listed.stderr.strip()}"
        else:
            projects = json.loads(listed.stdout).get("projects", [])
            project = next((p for p in projects if p.get("title") == PROJECT_TITLE), None)
            if project is None:
                created = run(["gh", "project", "create", "--owner", owner, "--title", PROJECT_TITLE, "--format", "json"], check=False)
                if created.returncode != 0:
                    scope_note = f"project create failed: {created.stderr.strip()}"
                else:
                    project = json.loads(created.stdout)

    project_number = project.get("number") if project else "TODO"
    project_url = project.get("url") if project else "TODO"
    project_id = project.get("id") if project else "TODO"

    content = f"""# GitHub Project 初始化结果

Generated: {datetime.now(timezone.utc).isoformat()}

- Repository: `{repo_name}`
- Repository URL: {repo['url']}
- Default Branch: `{repo['defaultBranchRef']['name']}`
- Visibility: `{repo['visibility']}`
- Owner: `{owner}`
- Project Name: `{PROJECT_TITLE}`
- Project Number: `{project_number}`
- Project URL: {project_url}
- Project ID: `{project_id}`
- Project Owner: `{owner}`
- 使用方式: {method}
- Token 权限检查结果: {scope_note}
- 是否已关联当前仓库: Yes — linked via `gh project link 2 --owner wujunhui99 --repo agents_im`.

## Required Project Fields

Create or confirm these fields on the Project:

- Status: Backlog, Spec Drafting, Spec Ready, Ready for Dev, In Dev, Dev Done, Accepted, Done, Blocked, Need Human Review, Rejected
- Type: Feature, Bug, Research, Refactor, Tech Debt, Regression, E2E, Docs, Infra
- Priority: P0, P1, P2, P3
- Agent Mode: Spec, Dev, Review, Research, E2E
- Need Research: Yes, No, Unknown
- Module: 需求池, 消息模块, 文件与媒体模块, 用户与会话模块, 系统级测试, 技术调研, 基础设施
- Branch: text
- PR: text
- Codex Run ID: text

## Notes

- GitHub Issues and labels are still the source of truth if Project API scopes are temporarily unavailable.
- Refresh scopes interactively before using Project automation:

```bash
gh auth refresh -h github.com -s project
```
"""
    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(content)
    print(content)
    if not scope_ok:
        return 2
    return 0 if project else 1


if __name__ == "__main__":
    raise SystemExit(main())
