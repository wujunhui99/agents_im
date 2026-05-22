#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

runtime_latest_matches="$({
  grep -RInE 'image: [^[:space:]]+:latest\b' "${ROOT_DIR}/deploy" "${ROOT_DIR}/docker-compose.yml" 2>/dev/null || true
  grep -RInE 'IMAGE_TAG="\$\{IMAGE_TAG:-latest\}"' "${ROOT_DIR}/scripts" 2>/dev/null || true
  grep -RInE -- '--tag "\$\{registry\}/\$\{service\}:latest"' "${ROOT_DIR}/scripts" 2>/dev/null || true
})"

if [[ -n "${runtime_latest_matches}" ]]; then
  echo "runtime deployment images must not use :latest, default IMAGE_TAG=latest, or publish mutable latest tags" >&2
  printf '%s\n' "${runtime_latest_matches}" >&2
  exit 1
fi
