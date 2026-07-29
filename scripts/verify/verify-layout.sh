#!/usr/bin/env bash
# Target-layout lint (docs/refactor/v1/01-project-structure.md §6): prevent the
# retired top-level god-packages (internal/ api/ proto/ rpcgen/ cmd/) from
# regrowing, keep service entrypoints inside their service package, and enforce
# the pkg/ -> service/ one-way dependency edge. Runnable standalone.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"
cd "$(git rev-parse --show-toplevel)"

# A. Top-level directories stay clean — none of the retired god-packages may
#    reappear at the repo root (internal/ retired in #618; api/proto/rpcgen/cmd
#    single-sourced under service/<domain>/).
forbid_paths "retired top-level directory reappeared (single-source under service/<domain>/ instead)" \
  internal \
  api \
  proto \
  rpcgen \
  cmd

# B. No residual imports of the retired top-level packages anywhere in the tree.
for retired_pkg in internal api proto; do
  forbid_match "residual import of retired top-level package: github.com/wujunhui99/agents_im/${retired_pkg}" \
    -n "\"github.com/wujunhui99/agents_im/${retired_pkg}(/|\")" --glob '*.go' .
done

# C. pkg/ -> service/ must stay one-way: shared pkg/ helpers own no dependency on
#    any concrete service (domain leakage into pkg/ means a boundary mistake).
forbid_match "pkg/ must not import service/ (pkg -> service is a one-way edge)" \
  -n '"github.com/wujunhui99/agents_im/service' pkg/ --glob '*.go'

# D. Each service entrypoint lives inside its own service package (a slim
#    `package main`), never split into an entry/ sub-package or wired elsewhere.
if find service -type d -name entry | grep -q .; then
  echo "verify-layout: service entrypoint must be the service package itself, not an entry/ sub-package:" >&2
  find service -type d -name entry >&2
  exit 1
fi

services_out="$(make -s services)"
if [[ -z "${services_out}" ]]; then
  echo "verify-layout: 'make -s services' produced no services to check" >&2
  exit 1
fi

service_count=0
while IFS= read -r service_line; do
  [[ -n "${service_line//[[:space:]]/}" ]] || continue

  read -r svc_name svc_dir extra <<<"${service_line}"
  if [[ -z "${svc_name}" || -z "${svc_dir}" || -n "${extra}" ]]; then
    echo "verify-layout: malformed 'make -s services' line: ${service_line}" >&2
    exit 1
  fi
  if [[ "${svc_dir}" != ./service/* || ! -d "${svc_dir}" ]]; then
    echo "verify-layout: service package path must be an existing directory under ./service/: ${svc_name} ${svc_dir}" >&2
    exit 1
  fi

  service_count=$((service_count + 1))
  if ! package_name="$(go list -f '{{.Name}}' "${svc_dir}" 2>&1)"; then
    echo "verify-layout: cannot load service package: ${svc_name} ${svc_dir}" >&2
    printf '%s\n' "${package_name}" >&2
    exit 1
  fi
  if [[ "${package_name}" != "main" ]]; then
    echo "verify-layout: service entrypoint package must be 'main': ${svc_name} ${svc_dir} (got ${package_name})" >&2
    exit 1
  fi
done <<<"${services_out}"

if ((service_count == 0)); then
  echo "verify-layout: 'make -s services' produced no parseable services to check" >&2
  exit 1
fi

echo "layout verification passed"
