#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import shlex
from pathlib import PurePosixPath


BACKEND_SERVICES = [
    "user-api",
    "auth-api",
    "friends-api",
    "message-api",
    "gateway-ws",
    "groups-api",
    "agent-api",
    "admin-api",
    "message-transfer",
    "user-rpc",
    "auth-rpc",
    "friends-rpc",
    "groups-rpc",
    "message-rpc",
    "msg-rpc",
    "third-rpc",
    "media-api",
    "media-rpc",
    "admin-rpc",
]

ALL_IMAGE_SERVICES = [*BACKEND_SERVICES, "web"]

CONFIG_ROLLOUT_SERVICES = [
    *ALL_IMAGE_SERVICES,
    "agents-im-minio-proxy",
    "prometheus",
    "grafana",
    "loki",
    "tempo",
    "otel-collector",
    "langfuse",
]

OBSERVABILITY_MANIFEST_ROLLOUTS = {
    "deploy/k8s/prometheus-grafana.yaml": ["prometheus", "grafana"],
    "deploy/k8s/loki.yaml": ["loki"],
    "deploy/k8s/tempo.yaml": ["tempo", "grafana"],
    "deploy/k8s/otel-collector.yaml": ["otel-collector"],
    "deploy/k8s/langfuse.yaml": ["langfuse"],
}

API_SERVICES = {
    "user": "user-api",
    "auth": "auth-api",
    "friends": "friends-api",
    "groups": "groups-api",
    "message": "message-api",
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

API_BACKEND_SERVICES = [
    "user-api",
    "auth-api",
    "friends-api",
    "message-api",
    "groups-api",
    "agent-api",
]

# Non-go-zero services whose main lives directly under service/<name>/ (cmd/ removed).
FLAT_SERVICE_DIRS = {
    "service/gateway-ws/": "gateway-ws",
    "service/message-api/": "message-api",
    "service/message-transfer/": "message-transfer",
}

# Only the message domain still rides the monolith internal/* tree (message-api /
# message-transfer). user/auth/friends/groups moved to service/<domain>/api and their
# legacy internal/{handler,logic,servicecontext}/<domain> scaffolding was deleted (#389).
INTERNAL_DOMAIN_SERVICE_PREFIXES = {
    "internal/handler/message/": ["message-api"],
    "internal/handler/admin/": ["admin-api"],
    # service/admin/{api,rpc}/** 由 service/<domain>/<kind> 通用规则精确路由；
    # 这里兜底任何 service/admin/ 顶层散文件，两者都重建。
    "service/admin/": ["admin-api", "admin-rpc"],
    "internal/logic/message/": ["message-api", "message-transfer"],
    "internal/servicecontext/message/": ["message-api", "message-transfer"],
}

INTERNAL_EXACT_SERVICE_PATHS = {
    # gozero_routes.go now only registers message handlers (RegisterMessageGoZeroHandlers).
    "internal/handler/gozero_routes.go": ["message-api"],
    "internal/handler/gozero_routes_test.go": ["message-api"],
    "internal/handler/admin_routes_test.go": ["message-api"],
}


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

    def add_api_backends(self) -> None:
        self.add_backends(API_BACKEND_SERVICES)

    def add_web(self) -> None:
        self.image_services.add("web")
        self.rollout_services.add("web")

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

    if path in {
        ".github/workflows/deploy.yml",
        "scripts/deploy-k3s.sh",
        ".drone.yml",
        "scripts/ci/drone-build-images.sh",
        "scripts/ci/drone-deploy.sh",
        "scripts/ci/drone-detect-deploy.sh",
        "scripts/detect-deploy-changes.py",
    }:
        # CI/deploy orchestration changes should exercise deploy/runtime wiring
        # without rebuilding every service image on each deploy-script fix.
        selection.add_rollout("groups-rpc")
        return

    service = service_from_yaml(path, "deploy/k8s/etc/")
    if service is not None:
        add_config_rollout(selection, service)
        return

    if path.startswith("deploy/k8s/"):
        observability_rollouts = OBSERVABILITY_MANIFEST_ROLLOUTS.get(path)
        if observability_rollouts is not None:
            selection.add_rollouts(observability_rollouts)
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

    if len(parts) == 2 and parts[0] == "api" and parts[1].endswith(".api"):
        domain = parts[1][: -len(".api")]
        service = API_SERVICES.get(domain)
        if service is None:
            selection.add_all_backends()
        else:
            selection.add_backend(service)
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
