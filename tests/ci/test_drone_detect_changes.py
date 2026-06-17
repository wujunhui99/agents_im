#!/usr/bin/env python3
from __future__ import annotations

import os
import subprocess
import tempfile
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
SCRIPT = REPO_ROOT / "scripts" / "ci" / "drone-detect-changes.sh"


def run(cmd: list[str], cwd: Path, env: dict[str, str] | None = None) -> str:
    merged_env = os.environ.copy()
    if env:
        merged_env.update(env)
    result = subprocess.run(
        cmd,
        cwd=cwd,
        env=merged_env,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=True,
    )
    return result.stdout


def write(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")


def read_env(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        key, value = line.split("=", 1)
        values[key] = value.strip("'")
    return values


class DroneDetectChangesTest(unittest.TestCase):
    def make_repo(self) -> tuple[tempfile.TemporaryDirectory[str], Path]:
        tmp = tempfile.TemporaryDirectory()
        repo = Path(tmp.name)
        run(["git", "init", "-b", "main"], repo)
        run(["git", "config", "user.email", "ci@example.test"], repo)
        run(["git", "config", "user.name", "CI"], repo)
        write(repo / "README.md", "initial\n")
        write(repo / "web/src/App.tsx", "export const App = 1;\n")
        run(["git", "add", "."], repo)
        run(["git", "commit", "-m", "initial"], repo)
        run(["git", "update-ref", "refs/remotes/origin/main", "main"], repo)
        run(["git", "checkout", "-b", "feature/codex/issue-1-ci"], repo)
        return tmp, repo

    def run_detector(self, repo: Path) -> dict[str, str]:
        out_file = repo / ".drone-changes.env"
        sha = run(["git", "rev-parse", "HEAD"], repo).strip()
        run(
            ["bash", str(SCRIPT), str(out_file)],
            repo,
            {
                "DRONE_BUILD_EVENT": "pull_request",
                "DRONE_TARGET_BRANCH": "main",
                "DRONE_COMMIT_SHA": sha,
            },
        )
        return read_env(out_file)

    def test_backend_only_pr_requires_backend_only(self):
        tmp, repo = self.make_repo()
        with tmp:
            write(repo / "service/user/api/user.go", "package main\n")
            run(["git", "add", "."], repo)
            run(["git", "commit", "-m", "backend"], repo)

            values = self.run_detector(repo)
            self.assertEqual(values["frontend_required"], "false")
            self.assertEqual(values["markdown_required"], "false")
            self.assertEqual(values["backend_required"], "true")

    def test_web_only_pr_requires_frontend_only(self):
        tmp, repo = self.make_repo()
        with tmp:
            write(repo / "web/src/App.tsx", "export const App = 2;\n")
            run(["git", "add", "."], repo)
            run(["git", "commit", "-m", "frontend"], repo)

            values = self.run_detector(repo)
            self.assertEqual(values["frontend_required"], "true")
            self.assertEqual(values["markdown_required"], "false")
            self.assertEqual(values["backend_required"], "false")

    def test_markdown_only_pr_requires_markdown_only(self):
        tmp, repo = self.make_repo()
        with tmp:
            write(repo / "docs/guide.md", "guide\n")
            run(["git", "add", "."], repo)
            run(["git", "commit", "-m", "docs"], repo)

            values = self.run_detector(repo)
            self.assertEqual(values["frontend_required"], "false")
            self.assertEqual(values["markdown_required"], "true")
            self.assertEqual(values["backend_required"], "false")

    def test_web_and_markdown_pr_skips_backend(self):
        tmp, repo = self.make_repo()
        with tmp:
            write(repo / "web/README.md", "web docs\n")
            run(["git", "add", "."], repo)
            run(["git", "commit", "-m", "web docs"], repo)

            values = self.run_detector(repo)
            self.assertEqual(values["frontend_required"], "true")
            self.assertEqual(values["markdown_required"], "true")
            self.assertEqual(values["backend_required"], "false")

    def test_web_change_in_earlier_pr_commit_still_requires_frontend(self):
        tmp, repo = self.make_repo()
        with tmp:
            write(repo / "web/src/App.tsx", "export const App = 2;\n")
            run(["git", "add", "."], repo)
            run(["git", "commit", "-m", "frontend"], repo)
            write(repo / "service/user/api/user.go", "package main\n")
            run(["git", "add", "."], repo)
            run(["git", "commit", "-m", "backend"], repo)

            values = self.run_detector(repo)
            self.assertEqual(values["frontend_required"], "true")
            self.assertEqual(values["backend_required"], "true")


if __name__ == "__main__":
    unittest.main()
