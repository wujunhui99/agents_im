#!/usr/bin/env bash
# Shared cache configuration for Drone verification steps.
# The Drone pipeline mounts a trusted host cache volume at /cache. When that
# mount is absent (local runs), fall back to /tmp so scripts remain runnable.
set -euo pipefail

ci_cache_root="${CI_CACHE_ROOT:-/cache}"
if [[ ! -d "${ci_cache_root}" || ! -w "${ci_cache_root}" ]]; then
  ci_cache_root="${TMPDIR:-/tmp}/agents-im-ci-cache"
fi

export CI_CACHE_ROOT="${ci_cache_root}"
export GOMODCACHE="${GOMODCACHE:-${CI_CACHE_ROOT}/go/pkg/mod}"
export GOCACHE="${GOCACHE:-${CI_CACHE_ROOT}/go/build}"
export GOBIN="${GOBIN:-${CI_CACHE_ROOT}/go/bin}"
export GOPATH="${GOPATH:-${CI_CACHE_ROOT}/go}"
export npm_config_cache="${npm_config_cache:-${CI_CACHE_ROOT}/npm}"
export NPM_CONFIG_CACHE="${npm_config_cache}"
export APT_CACHE_DIR="${APT_CACHE_DIR:-${CI_CACHE_ROOT}/apt}"
export PATH="${GOBIN}:/tmp/go/bin:${HOME}/go/bin:${PATH}"

mkdir -p \
  "${GOMODCACHE}" \
  "${GOCACHE}" \
  "${GOBIN}" \
  "${GOPATH}" \
  "${npm_config_cache}" \
  "${APT_CACHE_DIR}/archives/partial" \
  "${APT_CACHE_DIR}/lists/partial" \
  "${CI_CACHE_ROOT}/locks"

ci_cache_summary() {
  echo "[ci-cache] root=${CI_CACHE_ROOT}"
  echo "[ci-cache] GOMODCACHE=${GOMODCACHE}"
  echo "[ci-cache] GOCACHE=${GOCACHE}"
  echo "[ci-cache] GOBIN=${GOBIN}"
  echo "[ci-cache] npm_config_cache=${npm_config_cache}"
  echo "[ci-cache] APT_CACHE_DIR=${APT_CACHE_DIR}"
}
