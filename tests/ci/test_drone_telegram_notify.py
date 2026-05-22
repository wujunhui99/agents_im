#!/usr/bin/env python3
from __future__ import annotations

import os
import subprocess
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
NOTIFY_SCRIPT = REPO_ROOT / "scripts" / "ci" / "drone-telegram-notify.py"


class DroneTelegramNotifyAttributionTest(unittest.TestCase):
    def run_notify(self, **overrides: str) -> subprocess.CompletedProcess[str]:
        env = {
            **os.environ,
            "DRONE_TELEGRAM_DRY_RUN": "1",
            "DRONE_BUILD_STATUS": "failure",
            "DRONE_REPO": "wujunhui99/agents_im",
            "DRONE_BUILD_EVENT": "pull_request",
            "DRONE_STAGE_NAME": "verification",
            "DRONE_SOURCE_BRANCH": "fix/hermes/issue-123-attribution-check",
            "DRONE_TARGET_BRANCH": "develop",
            "DRONE_COMMIT_BRANCH": "",
            "DRONE_COMMIT_SHA": "0123456789abcdef",
            "DRONE_COMMIT_AUTHOR": "Hermes (AI Agent)",
            "DRONE_COMMIT_AUTHOR_EMAIL": "hermes@agents.noreply.local",
            "DRONE_COMMIT_MESSAGE": (
                "fix(ci)[hermes]: check agent attribution\n\n"
                "Issue: #123\nAgent: hermes\nHuman-Owner: junhui\n"
            ),
            "DRONE_BUILD_STARTED": "0",
        }
        env.update(overrides)
        return subprocess.run(
            ["python3", str(NOTIFY_SCRIPT), "failure"],
            cwd=REPO_ROOT,
            env=env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=False,
        )

    def test_pr_notification_uses_development_branch_agent(self) -> None:
        result = self.run_notify(
            DRONE_SOURCE_BRANCH="fix/hermes/issue-123-attribution-check",
            DRONE_COMMIT_AUTHOR="Hermes (AI Agent)",
            DRONE_COMMIT_AUTHOR_EMAIL="hermes@agents.noreply.local",
            DRONE_COMMIT_MESSAGE=(
                "fix(ci)[hermes]: check agent attribution\n\n"
                "Issue: #123\nAgent: hermes\nHuman-Owner: junhui\n"
            ),
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Agent: hermes @ws_ubuntu_hermes_bot", result.stdout)
        self.assertIn("Attribution: branch", result.stdout)
        self.assertNotIn("Attribution mismatch", result.stdout)

    def test_main_push_notification_routes_to_eino_release_owner(self) -> None:
        result = self.run_notify(
            DRONE_BUILD_EVENT="push",
            DRONE_SOURCE_BRANCH="",
            DRONE_TARGET_BRANCH="",
            DRONE_COMMIT_BRANCH="main",
            DRONE_COMMIT_AUTHOR="Hermes (AI Agent)",
            DRONE_COMMIT_AUTHOR_EMAIL="hermes@agents.noreply.local",
            DRONE_COMMIT_MESSAGE=(
                "feat(agent)[hermes]: add agent runtime feature\n\n"
                "Issue: #123\nAgent: hermes\nHuman-Owner: junhui\n"
            ),
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Agent: eino @eino_hermes_bot", result.stdout)
        self.assertIn("Attribution: main release owner", result.stdout)
        self.assertNotIn("Attribution mismatch", result.stdout)

    def test_devops_push_notification_routes_to_eino_owner(self) -> None:
        result = self.run_notify(
            DRONE_BUILD_EVENT="push",
            DRONE_SOURCE_BRANCH="",
            DRONE_TARGET_BRANCH="",
            DRONE_COMMIT_BRANCH="devops",
            DRONE_COMMIT_AUTHOR="Achilles (AI Agent)",
            DRONE_COMMIT_AUTHOR_EMAIL="achilles@agents.noreply.local",
            DRONE_COMMIT_MESSAGE=(
                "ci(trace)[achilles]: adjust observability config\n\n"
                "Issue: #124\nAgent: achilles\nHuman-Owner: junhui\n"
            ),
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Agent: eino @eino_hermes_bot", result.stdout)
        self.assertIn("Attribution: devops owner", result.stdout)
        self.assertNotIn("Attribution mismatch", result.stdout)


if __name__ == "__main__":
    unittest.main()
