#!/usr/bin/env python3
"""Create or update required labels for the agentic GitHub workflow.

Requires gh CLI auth with repo scope. Does not print tokens.
"""
from __future__ import annotations

import json
import subprocess
import sys
from dataclasses import dataclass


@dataclass(frozen=True)
class Label:
    name: str
    color: str
    description: str


LABELS = [
    Label("type:feature", "1f883d", "Product feature or enhancement"),
    Label("type:bug", "d73a4a", "Bug or production regression"),
    Label("type:research", "5319e7", "Research/specification task"),
    Label("type:refactor", "fbca04", "Refactor without intended behavior change"),
    Label("type:e2e", "0e8a16", "End-to-end/system regression task"),
    Label("type:regression", "b60205", "Regression coverage or fix"),
    Label("agent:spec", "bfdadc", "Handled by Codex/Hermes Spec Mode"),
    Label("agent:dev", "c2e0c6", "Handled by Codex Dev Mode"),
    Label("agent:review", "d4c5f9", "Needs agent/controller review"),
    Label("status:blocked", "000000", "Blocked by dependency, missing info, or failure"),
    Label("priority:p0", "b60205", "Urgent production/blocking priority"),
    Label("priority:p1", "d93f0b", "High priority"),
    Label("priority:p2", "fbca04", "Normal priority"),
    Label("priority:p3", "cfd3d7", "Low priority"),
    Label("need:human-review", "f9d0c4", "Requires human/product review"),
]


def run(cmd: list[str], check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=check)


def repo() -> str:
    out = run(["gh", "repo", "view", "--json", "nameWithOwner"]).stdout
    return json.loads(out)["nameWithOwner"]


def main() -> int:
    owner_repo = repo()
    existing_raw = run(["gh", "label", "list", "--repo", owner_repo, "--limit", "200", "--json", "name"]).stdout
    existing = {item["name"] for item in json.loads(existing_raw)}
    for label in LABELS:
        if label.name in existing:
            cmd = [
                "gh", "label", "edit", label.name,
                "--repo", owner_repo,
                "--color", label.color,
                "--description", label.description,
            ]
            action = "updated"
        else:
            cmd = [
                "gh", "label", "create", label.name,
                "--repo", owner_repo,
                "--color", label.color,
                "--description", label.description,
            ]
            action = "created"
        result = run(cmd, check=False)
        if result.returncode != 0:
            print(f"ERROR {label.name}: {result.stderr.strip()}", file=sys.stderr)
            return result.returncode
        print(f"{action}: {label.name}")
    print(f"labels ready for {owner_repo}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
