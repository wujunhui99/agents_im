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
    "message-transfer",
    "user-rpc",
    "auth-rpc",
    "friends-rpc",
    "groups-rpc",
    "message-rpc",
    "mail-rpc",
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
}

PROTO_DOMAINS = {
    "user": "userpb",
    "auth": "authpb",
    "friends": "friendspb",
    "groups": "groupspb",
    "message": "messagepb",
    "mail": "mailpb",
}


class DeploySelection:
    def __init__(self) -> None:
        self.backend_services: set[str] = set()
        self.image_services: set[str] = set()
        self.rollout_services: set[str] = set()
        self.restart_services: set[str] = set()

    def add_backend(self, service: str) -> None:
        self.backend_services.add(service)
        self.image_services.add(service)
        self.rollout_services.add(service)

    def add_all_backends(self) -> None:
        for service in BACKEND_SERVICES:
            self.add_backend(service)

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


def normalize_path(path: str) -> str:
    normalized = PurePosixPath(path).as_posix()
    while normalized.startswith("./"):
        normalized = normalized[2:]
    return normalized


def is_doc_only(path: str) -> bool:
    name = path.rsplit("/", 1)[-1]
    return path.startswith("docs/") or name == "README.md" or name.endswith(".md")


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

    parts = path.split("/")
    if len(parts) >= 3 and parts[0] == "cmd":
        service = parts[1]
        if service in BACKEND_SERVICES:
            selection.add_backend(service)
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

    if (
        path in {"go.mod", "go.sum", "Dockerfile", ".dockerignore"}
        or path.startswith("internal/")
        or path.startswith("db/")
        or path == "scripts/migrate-postgres.sh"
    ):
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
    }


def detect(event_name: str, ref: str, paths: list[str]) -> dict[str, str]:
    selection = DeploySelection()

    if ref not in {"refs/heads/main", "refs/heads/devops"}:
        return build_outputs(selection)

    if event_name == "workflow_dispatch":
        selection.add_all_backends()
        selection.add_web()
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
