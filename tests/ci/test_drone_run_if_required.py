#!/usr/bin/env python3
from __future__ import annotations

import subprocess
import tempfile
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
SCRIPT = REPO_ROOT / "scripts" / "ci" / "drone-run-if-required.sh"


class DroneRunIfRequiredTest(unittest.TestCase):
    def test_skips_when_flag_is_false(self):
        with tempfile.TemporaryDirectory() as tmp:
            workdir = Path(tmp)
            (workdir / ".drone-changes.env").write_text(
                "backend_required=false\nchange_diff_basis='test'\n",
                encoding="utf-8",
            )
            marker = workdir / "marker"
            subprocess.run(
                [
                    "bash",
                    str(SCRIPT),
                    "backend_required",
                    "backend verification",
                    "touch",
                    str(marker),
                ],
                cwd=workdir,
                check=True,
            )
            self.assertFalse(marker.exists())

    def test_runs_when_flag_is_true(self):
        with tempfile.TemporaryDirectory() as tmp:
            workdir = Path(tmp)
            (workdir / ".drone-changes.env").write_text(
                "backend_required=true\nchange_diff_basis='test'\n",
                encoding="utf-8",
            )
            marker = workdir / "marker"
            subprocess.run(
                [
                    "bash",
                    str(SCRIPT),
                    "backend_required",
                    "backend verification",
                    "touch",
                    str(marker),
                ],
                cwd=workdir,
                check=True,
            )
            self.assertTrue(marker.exists())


if __name__ == "__main__":
    unittest.main()
