#!/usr/bin/env python3
from __future__ import annotations

import argparse
import functools
import json
import shlex
import shutil
import subprocess
from pathlib import Path, PurePosixPath


REPO_ROOT = Path(__file__).resolve().parents[1]


def _load_registry() -> tuple[list[str], dict[str, str], list[str], str]:
    """Read the service registry (single source of truth) from services.json."""
    registry = json.loads((Path(__file__).resolve().parent / "services.json").read_text())
    backend = [s["name"] for s in registry["backend"]]
    packages = {s["name"]: s["package"] for s in registry["backend"]}
    return backend, packages, registry["infra"], registry["web"]


BACKEND_SERVICES, BACKEND_SERVICE_PACKAGES, _INFRA_SERVICES, _WEB_SERVICE = _load_registry()

ALL_IMAGE_SERVICES = [*BACKEND_SERVICES, _WEB_SERVICE]

CONFIG_ROLLOUT_SERVICES = [*ALL_IMAGE_SERVICES, *_INFRA_SERVICES]

OBSERVABILITY_MANIFEST_ROLLOUTS = {
    "deploy/k8s/prometheus-grafana.yaml": ["prometheus", "grafana"],
    "deploy/k8s/loki.yaml": ["loki"],
    "deploy/k8s/tempo.yaml": ["tempo", "grafana"],
    "deploy/k8s/otel-collector.yaml": ["otel-collector"],
    "deploy/k8s/langfuse.yaml": ["langfuse"],
}

# k8s resources that take effect immediately on kubectl apply — no pod restart needed.
# Changes here trigger deploy (so kubectl apply runs) but skip kubectl rollout restart
# for all infra/middleware services.
K8S_APPLY_ONLY_MANIFESTS = {
    "deploy/k8s/kustomization.yaml",
    "deploy/k8s/namespace.yaml",
    "deploy/k8s/ingress.yaml",
    "deploy/k8s/services.yaml",
    "deploy/k8s/python-executor-rbac.yaml",
    "deploy/k8s/cert-manager-issuers.yaml",
}

API_SERVICES = {
    "user": "user-api",
    "auth": "auth-api",
    "friends": "friends-api",
    "groups": "groups-api",
    "msg": "msg-api",
    "agent": "agent-api",
    "admin": "admin-api",
    "media": "media-api",
}

PROTO_DOMAINS = {
    "user": "userpb",
    "auth": "authpb",
    "friends": "friendspb",
    "groups": "groupspb",
    "message": "messagepb",
    "mail": "mailpb",
}

# Non-go-zero services whose main lives directly under service/<name>/ (cmd/ removed).
FLAT_SERVICE_DIRS = {
    "service/msggateway/": "msggateway",
    "service/msgtransfer/": "msgtransfer",
    "service/push/": "push",
}

# message-api 已退役（#463）：REST 入口归 service/msg/api。AI 托管运行时 + 开关 CRUD
# #340 起整体迁出至属主 service/agent/rpc（trigger/runtime/hosting/orchestrator/imadapter），
# 由下方 service/<domain>/<kind> 通用规则精确路由到 agent-rpc / msg-rpc，无需 internal 前缀特例。
INTERNAL_DOMAIN_SERVICE_PREFIXES = {
    "internal/handler/admin/": ["admin-api"],
    # service/admin/{api,rpc}/** 由 service/<domain>/<kind> 通用规则精确路由；
    # 这里兜底任何 service/admin/ 顶层散文件，两者都重建。
    "service/admin/": ["admin-api", "admin-rpc"],
}

INTERNAL_EXACT_SERVICE_PATHS = {}


class DeploySelection:
    def __init__(self) -> None:
        self.backend_services: set[str] = set()
        self.image_services: set[str] = set()
        self.rollout_services: set[str] = set()
        self.restart_services: set[str] = set()
        self.migration_required = False

    def add_backend(self, service: str) -> None:
        self.backend_services.add(service)
        self.image_services.add(service)
        self.rollout_services.add(service)

    def add_all_backends(self) -> None:
        for service in BACKEND_SERVICES:
            self.add_backend(service)

    def add_backends(self, services: list[str]) -> None:
        for service in services:
            self.add_backend(service)

    def add_web(self) -> None:
        self.image_services.add(_WEB_SERVICE)
        self.rollout_services.add(_WEB_SERVICE)

    def add_rollout(self, service: str, *, restart: bool = True) -> None:
        self.rollout_services.add(service)
        if restart:
            self.restart_services.add(service)

    def add_all_rollouts(self, *, restart: bool = True) -> None:
        for service in CONFIG_ROLLOUT_SERVICES:
            self.add_rollout(service, restart=restart)

    def add_rollouts(self, services: list[str], *, restart: bool = True) -> None:
        for service in services:
            self.add_rollout(service, restart=restart)

    def require_migration(self) -> None:
        self.migration_required = True


def normalize_path(path: str) -> str:
    normalized = PurePosixPath(path).as_posix()
    while normalized.startswith("./"):
        normalized = normalized[2:]
    return normalized


def is_doc_only(path: str) -> bool:
    name = path.rsplit("/", 1)[-1]
    return path.startswith("docs/") or name == "README.md" or name.endswith(".md")


def is_go_test_file(path: str) -> bool:
    return path.endswith("_test.go")


def is_meta_dotpath(path: str) -> bool:
    # Top-level dotfiles / dotdirs (e.g. .claude/, .github/, .gitignore)
    # are tooling/meta and never affect the deployed app, so they must not trigger
    # build or deploy. Genuine CI dotfiles (.drone.yml, .github/workflows/deploy.yml,
    # .dockerignore) are matched by the explicit deploy rules in classify_path before
    # this check is reached.
    return path.startswith(".")


def service_from_yaml(path: str, prefix: str) -> str | None:
    if not path.startswith(prefix) or not path.endswith(".yaml"):
        return None
    service = path[len(prefix) : -len(".yaml")]
    if "/" in service:
        return None
    return service


def add_config_rollout(selection: DeploySelection, service: str | None) -> None:
    if service in CONFIG_ROLLOUT_SERVICES:
        selection.add_rollout(service)
    else:
        selection.add_all_rollouts()


def go_tool_available() -> bool:
    return shutil.which("go") is not None


@functools.cache
def go_list_import_path(package_dir: str) -> str | None:
    if not go_tool_available():
        return None
    full_dir = REPO_ROOT / package_dir
    if not full_dir.exists() or not full_dir.is_dir():
        return None
    result = subprocess.run(
        ["go", "list", "-f", "{{.ImportPath}}", f"./{package_dir}"],
        cwd=REPO_ROOT,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        check=False,
    )
    if result.returncode != 0:
        return None
    return result.stdout.strip() or None


@functools.cache
def go_list_service_deps(service: str) -> frozenset[str] | None:
    if not go_tool_available():
        return None
    package = BACKEND_SERVICE_PACKAGES[service]
    result = subprocess.run(
        ["go", "list", "-deps", package],
        cwd=REPO_ROOT,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        check=False,
    )
    if result.returncode != 0:
        return None
    return frozenset(line for line in result.stdout.splitlines() if line)


def add_go_import_dependents(path: str, selection: DeploySelection) -> bool:
    """Route production Go changes by actual service import graph when possible.

    Returns True when the path was handled. If Go metadata is unavailable (for
    example a deleted file or a runner without the Go toolchain), callers should
    fall back to the conservative path rules below.
    """
    if not path.endswith(".go"):
        return False
    if is_go_test_file(path):
        return True

    package_dir = normalize_path(str(PurePosixPath(path).parent))
    package_import = go_list_import_path(package_dir)
    if package_import is None:
        return False

    affected: list[str] = []
    for service in BACKEND_SERVICES:
        deps = go_list_service_deps(service)
        if deps is None:
            return False
        if package_import in deps:
            affected.append(service)

    selection.add_backends(affected)
    return True


def classify_path(path: str, selection: DeploySelection) -> None:
    if not path:
        return

    if path == "deploy/README.md":
        # Deployment runbook changes should exercise the config-only deploy path
        # so CI/CD maintenance can validate Drone deploy credentials and rollout.
        selection.add_rollout("groups-rpc")
        return

    if is_doc_only(path):
        return

    if add_go_import_dependents(path, selection):
        return

    if path in {
        ".github/workflows/deploy.yml",
        "scripts/deploy-k3s.sh",
        ".drone.yml",
        "scripts/ci/drone-build-images.sh",
        "scripts/ci/drone-deploy.sh",
        "scripts/ci/drone-detect-deploy.sh",
        "scripts/detect-deploy-changes.py",
        "scripts/services.json",
        "scripts/services.sh",
    }:
        # CI/deploy orchestration changes should exercise deploy/runtime wiring
        # without rebuilding every service image on each deploy-script fix.
        # services.json/services.sh are the deploy-time service registry read by
        # deploy-k3s.sh, so they take the same config-only path.
        selection.add_rollout("groups-rpc")
        return

    if path in {
        "scripts/drone-watch.sh",
        "scripts/bootstrap-server.sh",
        "scripts/test-deploy-k3s.sh",
        "scripts/dev-up.sh",
        "scripts/dev-demo-data.sh",
    } or path.startswith("scripts/dev/"):
        # Agent-local tooling, one-time server provisioning, the deploy-script test
        # harness, and the local dev-up stack (incl. its scripts/dev/ config
        # templates) never affect the deployed app — must not hit the fail-safe
        # full rebuild below.
        return

    if path.startswith("scripts/ci/") or path.startswith("scripts/verify"):
        # Remaining scripts/ci/ (verification steps, telegram notify — the deploy
        # orchestration trio is matched by the canary set above) and the verify
        # script family run only inside CI; they never reach the deployed app.
        return

    if path.startswith("tests/"):
        # Top-level tests/ is CI-only: Go *_test.go files are not compiled into
        # service binaries and tests/ci python suites run only in verification.
        # They must not trigger the fail-safe full rebuild.
        return

    service = service_from_yaml(path, "deploy/k8s/etc/")
    if service is not None:
        add_config_rollout(selection, service)
        return

    if path == "deploy/k8s/secrets.example.yaml":
        return

    if path.startswith("deploy/k8s/"):
        observability_rollouts = OBSERVABILITY_MANIFEST_ROLLOUTS.get(path)
        if observability_rollouts is not None:
            selection.add_rollouts(observability_rollouts)
        elif path in K8S_APPLY_ONLY_MANIFESTS:
            # Routing/RBAC/ingress resources take effect on apply — use canary rollout
            # so the deploy step runs without restarting infra middleware.
            selection.add_rollout("groups-rpc", restart=False)
        else:
            selection.add_all_rollouts()
        return

    service = service_from_yaml(path, "etc/")
    if service is not None:
        add_config_rollout(selection, service)
        return

    if path.startswith("web/"):
        selection.add_web()
        return

    for prefix, service in FLAT_SERVICE_DIRS.items():
        if path.startswith(prefix):
            selection.add_backend(service)
            return

    parts = path.split("/")
    if len(parts) >= 4 and parts[0] == "service":
        domain = parts[1]
        kind = parts[2]
        if kind == "api":
            service = API_SERVICES.get(domain)
            if service is None:
                selection.add_all_backends()
            else:
                selection.add_backend(service)
            return
        if kind == "rpc":
            rpc_service = f"{domain}-rpc"
            if rpc_service in BACKEND_SERVICES:
                selection.add_backend(rpc_service)
            else:
                selection.add_all_backends()
            return

    if path.startswith("proto/"):
        if len(parts) >= 2:
            proto_name = parts[1]
            if proto_name.endswith(".proto"):
                domain = proto_name[: -len(".proto")]
                if domain in PROTO_DOMAINS:
                    selection.add_all_backends()
                    return
            if proto_name in PROTO_DOMAINS.values():
                selection.add_all_backends()
                return
        selection.add_all_backends()
        return

    if path in {"go.mod", "go.sum", "Dockerfile", ".dockerignore"}:
        selection.add_all_backends()
        return

    if is_meta_dotpath(path):
        # Reached only for dot-paths not claimed by the explicit deploy rules above
        # (.drone.yml, .github/workflows/deploy.yml, .dockerignore). Meta/tooling only.
        return

    exact_services = INTERNAL_EXACT_SERVICE_PATHS.get(path)
    if exact_services is not None:
        selection.add_backends(exact_services)
        return

    for prefix, services in INTERNAL_DOMAIN_SERVICE_PREFIXES.items():
        if path.startswith(prefix):
            selection.add_backends(services)
            return

    if path.startswith("internal/rpcgen/"):
        selection.add_all_backends()
        return

    if path.startswith("internal/"):
        # Shared internal packages (model/repository/config/response/auth/etc.) can
        # be imported by multiple APIs, RPCs, workers, or migrations. Keep those
        # conservative while domain-specific handler/logic/servicecontext paths
        # above remain selective.
        selection.add_all_backends()
        return

    if path.startswith("db/migrations/") or path == "scripts/migrate-postgres.sh":
        selection.require_migration()
        selection.add_all_backends()
        return

    if path.startswith("db/"):
        selection.add_all_backends()
        return

    # Fail safe for non-doc paths that do not yet have a precise ownership rule.
    selection.add_all_backends()


def ordered(values: set[str], service_order: list[str]) -> list[str]:
    return [service for service in service_order if service in values]


def shell_value(value: str) -> str:
    return shlex.quote(value)


def build_outputs(selection: DeploySelection) -> dict[str, str]:
    backend_services = ordered(selection.backend_services, BACKEND_SERVICES)
    image_services = ordered(selection.image_services, ALL_IMAGE_SERVICES)
    rollout_services = ordered(selection.rollout_services, CONFIG_ROLLOUT_SERVICES)
    restart_services = ordered(
        selection.restart_services - selection.image_services,
        CONFIG_ROLLOUT_SERVICES,
    )

    build_required = bool(backend_services or "web" in image_services)
    deploy_required = build_required or bool(rollout_services)
    config_only = deploy_required and not build_required

    return {
        "build_required": str(build_required).lower(),
        "deploy_required": str(deploy_required).lower(),
        "config_only": str(config_only).lower(),
        "backend_services": shell_value(json.dumps(backend_services, separators=(",", ":"))),
        "web_required": str("web" in image_services).lower(),
        "image_services": shell_value(json.dumps(image_services, separators=(",", ":"))),
        "image_services_space": shell_value(" ".join(image_services)),
        "rollout_services": shell_value(" ".join(rollout_services)),
        "restart_services": shell_value(" ".join(restart_services)),
        "migration_required": str(selection.migration_required).lower(),
    }


def detect(event_name: str, ref: str, paths: list[str]) -> dict[str, str]:
    selection = DeploySelection()

    if ref not in {"refs/heads/main", "refs/heads/devops"}:
        return build_outputs(selection)

    if event_name == "workflow_dispatch":
        selection.add_all_backends()
        selection.add_web()
        selection.require_migration()
        return build_outputs(selection)

    if ref == "refs/heads/devops" and any(
        normalize_path(path) in {".drone.yml", "scripts/ci/drone-build-images.sh"}
        for path in paths
    ):
        # CI/CD lab branch: changes to the image-build pipeline itself should
        # force a warm-cache build measurement. Main keeps these paths as
        # config-only deploy orchestration changes.
        selection.add_all_backends()

    for raw_path in paths:
        classify_path(normalize_path(raw_path), selection)

    outputs = build_outputs(selection)
    if ref == "refs/heads/devops":
        # The devops branch is a CI/CD performance lab. It should exercise image
        # build selection and cache timing without touching production runtime.
        if outputs["build_required"] == "true":
            outputs["deploy_required"] = "true"
        else:
            outputs["deploy_required"] = "false"
        outputs["config_only"] = "false"
        outputs["rollout_services"] = shell_value("")
        outputs["restart_services"] = shell_value("")
    return outputs


def main() -> None:
    parser = argparse.ArgumentParser(description="Classify deploy workflow changes.")
    parser.add_argument("--event-name", required=True)
    parser.add_argument("--ref", required=True)
    parser.add_argument("paths", nargs="*")
    args = parser.parse_args()

    for key, value in detect(args.event_name, args.ref, args.paths).items():
        print(f"{key}={value}")


if __name__ == "__main__":
    main()
