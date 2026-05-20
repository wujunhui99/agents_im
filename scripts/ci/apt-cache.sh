#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${script_dir}/cache-env.sh"

apt_get_cached() {
  if ! command -v apt-get >/dev/null 2>&1; then
    echo "apt-get is required by this Drone image" >&2
    exit 1
  fi

  local lock_file="${CI_CACHE_ROOT}/locks/apt.lock"
  (
    flock 9
    apt-get \
      -o "Dir::Cache=${APT_CACHE_DIR}" \
      -o "Dir::State::lists=${APT_CACHE_DIR}/lists" \
      -o "APT::Keep-Downloaded-Packages=true" \
      -o "Binary::apt::APT::Keep-Downloaded-Packages=true" \
      update

    DEBIAN_FRONTEND=noninteractive apt-get \
      -o "Dir::Cache=${APT_CACHE_DIR}" \
      -o "Dir::State::lists=${APT_CACHE_DIR}/lists" \
      -o "Dir::Cache::archives=${APT_CACHE_DIR}/archives" \
      -o "APT::Keep-Downloaded-Packages=true" \
      -o "Binary::apt::APT::Keep-Downloaded-Packages=true" \
      install -y --no-install-recommends "$@"
  ) 9>"${lock_file}"
}

apt_cache_summary() {
  if [[ -d "${APT_CACHE_DIR}/archives" ]]; then
    du -sh "${APT_CACHE_DIR}/archives" 2>/dev/null || true
  fi
}
