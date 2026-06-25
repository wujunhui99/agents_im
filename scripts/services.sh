#!/usr/bin/env bash
# Shell reader for scripts/services.json (the single source of truth for the
# service registry). Source this file, then call the helpers below. Requires
# python3 (already a hard dependency of deploy-k3s.sh).
#
#   source scripts/services.sh
#   mapfile -t backend < <(services_backend_names)   # deployed backends, canonical order
#   mapfile -t infra   < <(services_infra_names)     # monitoring / infra deployments
#   pkg="$(services_package user-api)"               # go main package for a backend

SERVICES_JSON="${SERVICES_JSON:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/services.json}"

_services_query() {
  python3 - "${SERVICES_JSON}" "$@" <<'PY'
import json
import sys

data = json.load(open(sys.argv[1]))
query = sys.argv[2]
if query == "backend":
    print("\n".join(s["name"] for s in data["backend"]))
elif query == "infra":
    print("\n".join(data["infra"]))
elif query == "web":
    print(data["web"])
elif query == "package":
    name = sys.argv[3]
    for s in data["backend"]:
        if s["name"] == name:
            print(s["package"])
            break
    else:
        sys.exit(1)
else:
    sys.exit(2)
PY
}

services_backend_names() { _services_query backend; }
services_infra_names() { _services_query infra; }
services_web_name() { _services_query web; }
services_package() { _services_query package "$1"; }
