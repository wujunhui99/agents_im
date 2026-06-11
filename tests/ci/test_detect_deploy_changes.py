#!/usr/bin/env python3
"""Unit tests for scripts/detect-deploy-changes.py (OB-8).

The detector decides which services build/deploy/migrate for a given changeset.
These tests pin the documented behavior (see docs/refactor/v1/05-observability-cicd.md
§6.3): docs-only, web-only, backend-only, migration, config-only, ref gating,
meta dotpaths, and workflow_dispatch.
"""
from __future__ import annotations

import importlib.util
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
DETECTOR = REPO_ROOT / "scripts" / "detect-deploy-changes.py"

_spec = importlib.util.spec_from_file_location("detect_deploy_changes", DETECTOR)
_module = importlib.util.module_from_spec(_spec)
assert _spec and _spec.loader
_spec.loader.exec_module(_module)
detect = _module.detect

MAIN = "refs/heads/main"
DEVOPS = "refs/heads/devops"


class DetectDeployChangesTest(unittest.TestCase):
    def detect(self, paths, ref=MAIN, event="push"):
        return detect(event, ref, paths)

    def test_docs_only_does_not_deploy(self):
        out = self.detect(["docs/guide.md", "README.md", "docs/refactor/v1/05-observability-cicd.md"])
        self.assertEqual(out["build_required"], "false")
        self.assertEqual(out["deploy_required"], "false")
        self.assertEqual(out["config_only"], "false")
        self.assertEqual(out["migration_required"], "false")

    def test_web_only_builds_web_image_no_backend(self):
        out = self.detect(["web/src/App.tsx"])
        self.assertEqual(out["web_required"], "true")
        self.assertEqual(out["build_required"], "true")
        self.assertNotIn("user-api", out["backend_services"])

    def test_backend_only_builds_affected_service(self):
        out = self.detect(["service/user/api/user.go"])
        self.assertEqual(out["build_required"], "true")
        self.assertEqual(out["web_required"], "false")
        self.assertIn("user-api", out["backend_services"])
        self.assertNotIn("auth-api", out["backend_services"])

    def test_migration_sets_migration_required(self):
        out = self.detect(["db/migrations/0001_init.sql"])
        self.assertEqual(out["migration_required"], "true")
        self.assertEqual(out["build_required"], "true")

    def test_config_only_skips_image_build(self):
        out = self.detect(["deploy/k8s/etc/user-api.yaml"])
        self.assertEqual(out["build_required"], "false")
        self.assertEqual(out["deploy_required"], "true")
        self.assertEqual(out["config_only"], "true")
        self.assertIn("user-api", out["rollout_services"])

    def test_non_release_ref_is_inert(self):
        out = self.detect(["service/user/api/user.go"], ref="refs/heads/fix/claude/issue-1-x")
        self.assertEqual(out["build_required"], "false")
        self.assertEqual(out["deploy_required"], "false")

    def test_meta_dotpaths_do_not_deploy(self):
        out = self.detect([".claude/settings.json", ".gitignore", ".hermes/state"])
        self.assertEqual(out["build_required"], "false")
        self.assertEqual(out["deploy_required"], "false")

    def test_workflow_dispatch_builds_everything(self):
        out = self.detect([], event="workflow_dispatch")
        self.assertEqual(out["build_required"], "true")
        self.assertEqual(out["web_required"], "true")
        self.assertEqual(out["migration_required"], "true")

    def test_shared_internal_package_is_conservative(self):
        # Shared internal packages can be imported widely → rebuild all backends.
        out = self.detect(["internal/repository/message_memory.go"])
        self.assertEqual(out["build_required"], "true")
        self.assertIn("user-api", out["backend_services"])
        self.assertIn("msg-api", out["backend_services"])

    def test_local_tooling_scripts_do_not_deploy(self):
        # Agent watcher / provisioning / deploy-script test harness never affect
        # the deployed app (#486) — must not hit the fail-safe full rebuild.
        for path in [
            "scripts/drone-watch.sh",
            "scripts/bootstrap-server.sh",
            "scripts/test-deploy-k3s.sh",
        ]:
            out = self.detect([path])
            self.assertEqual(out["build_required"], "false", path)
            self.assertEqual(out["deploy_required"], "false", path)


if __name__ == "__main__":
    unittest.main()
