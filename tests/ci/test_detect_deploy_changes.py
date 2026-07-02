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

    def test_shared_internal_package_uses_import_graph(self):
        # Shared packages are routed by go list -deps when the package is present;
        # this keeps deploys narrower than the old fail-safe all-backends path.
        # #617: msg-rpc/agent-rpc no longer import internal/repository (runtime
        # message/groups reads go through owner gRPC); only user-rpc + admin-rpc
        # still depend on internal/repository, so the graph must narrow to them.
        out = self.detect(["internal/repository/message_memory.go"])
        self.assertEqual(out["build_required"], "true")
        self.assertIn("user-rpc", out["backend_services"])
        self.assertIn("admin-rpc", out["backend_services"])
        self.assertNotIn("msg-rpc", out["backend_services"])
        self.assertNotIn("agent-rpc", out["backend_services"])
        self.assertNotIn("user-api", out["backend_services"])
        self.assertNotIn("msg-api", out["backend_services"])

    def test_go_test_files_do_not_deploy(self):
        out = self.detect(["service/agent/rpc/internal/aihosting/service_context_test.go"])
        self.assertEqual(out["build_required"], "false")
        self.assertEqual(out["deploy_required"], "false")

    def test_pkg_change_uses_import_graph(self):
        out = self.detect(["pkg/gateway/contract.go"])
        self.assertEqual(out["build_required"], "true")
        self.assertIn("msggateway", out["backend_services"])
        self.assertNotIn("user-api", out["backend_services"])

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

    def test_local_dev_stack_does_not_deploy(self):
        # The dev-up local stack and its scripts/dev/ config templates only run on
        # a developer machine; they never affect the deployed app.
        for path in [
            "scripts/dev-up.sh",
            "scripts/dev-demo-data.sh",
            "scripts/dev/etc/user-api.yaml.tmpl",
        ]:
            out = self.detect([path])
            self.assertEqual(out["build_required"], "false", path)
            self.assertEqual(out["deploy_required"], "false", path)

    def test_service_registry_takes_config_only_path(self):
        # services.json / services.sh are the deploy-time service registry read by
        # deploy-k3s.sh: a config-only rollout, not a full image rebuild.
        for path in ["scripts/services.json", "scripts/services.sh"]:
            out = self.detect([path])
            self.assertEqual(out["build_required"], "false", path)
            self.assertEqual(out["deploy_required"], "true", path)
            self.assertEqual(out["config_only"], "true", path)

    def test_ci_verification_scripts_do_not_deploy(self):
        # Verification-only CI scripts run inside the pipeline; the deploy
        # orchestration trio (drone-build-images/drone-deploy/drone-detect-deploy)
        # keeps its groups-rpc canary instead (#488).
        for path in [
            "scripts/ci/drone-backend-verify.sh",
            "scripts/ci/drone-detect-changes.sh",
            "scripts/ci/drone-postgres-integration.sh",
            "scripts/ci/drone-markdown-link-check.sh",
            "scripts/ci/drone-telegram-notify.py",
            "scripts/ci/verify-agent-branch-name.sh",
            "scripts/verify-static.sh",
            "scripts/verify/verify-gozero-boundaries.sh",
        ]:
            out = self.detect([path])
            self.assertEqual(out["build_required"], "false", path)
            self.assertEqual(out["deploy_required"], "false", path)
        out = self.detect(["scripts/ci/drone-deploy.sh"])
        self.assertEqual(out["deploy_required"], "true")
        self.assertEqual(out["build_required"], "false")
        out = self.detect(["scripts/migrate-postgres.sh"])
        self.assertEqual(out["migration_required"], "true")

    def test_top_level_tests_do_not_deploy(self):
        # tests/ is CI-only (go test files are not compiled into binaries;
        # tests/ci python suites run in verification only).
        out = self.detect(
            ["tests/ci/test_detect_deploy_changes.py", "tests/message_service_test.go"]
        )
        self.assertEqual(out["build_required"], "false")
        self.assertEqual(out["deploy_required"], "false")

    def test_k8s_apply_only_manifests_deploy_without_infra_restart(self):
        # Routing/RBAC/ingress manifests take effect on kubectl apply — deploy runs
        # but no infra middleware restart (groups-rpc canary, restart=False).
        for path in [
            "deploy/k8s/kustomization.yaml",
            "deploy/k8s/namespace.yaml",
            "deploy/k8s/ingress.yaml",
            "deploy/k8s/services.yaml",
            "deploy/k8s/python-executor-rbac.yaml",
            "deploy/k8s/cert-manager-issuers.yaml",
        ]:
            out = self.detect([path])
            self.assertEqual(out["build_required"], "false", path)
            self.assertEqual(out["deploy_required"], "true", path)
            self.assertEqual(out["config_only"], "true", path)
            self.assertIn("groups-rpc", out["rollout_services"], path)
            # No explicit restart — infra middleware not in restart_services.
            self.assertEqual(out["restart_services"], "''", path)

    def test_k8s_secrets_example_does_not_deploy(self):
        out = self.detect(["deploy/k8s/secrets.example.yaml"])
        self.assertEqual(out["build_required"], "false")
        self.assertEqual(out["deploy_required"], "false")


if __name__ == "__main__":
    unittest.main()
