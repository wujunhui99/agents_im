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
while read -r svc_dir; do
  [[ -n "${svc_dir}" ]] || continue
  if [[ -z "$(grep -rl '^package main' "${svc_dir}" 2>/dev/null || true)" ]]; then
    echo "verify-layout: service has no 'package main' entrypoint in its package: ${svc_dir}" >&2
    exit 1
  fi
done < <(awk '{print $2}' <<<"${services_out}")

echo "layout verification passed"
