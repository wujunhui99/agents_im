#!/usr/bin/env bash
set -euo pipefail

if (($# < 3)); then
  echo "usage: $0 <required-var> <label> <command> [args...]" >&2
  exit 2
fi

required_var="$1"
label="$2"
shift 2

env_file="${DRONE_CHANGES_ENV:-.drone-changes.env}"
if [[ ! -f "${env_file}" ]]; then
  echo "${env_file} is missing; cannot decide whether to run ${label}" >&2
  exit 1
fi

# shellcheck disable=SC1090
. "${env_file}"

required="${!required_var:-false}"
diff_basis="${change_diff_basis:-unknown}"

if [[ "${required}" != "true" ]]; then
  echo "${label} skipped: ${required_var}=false (${diff_basis})"
  exit 0
fi

echo "${label} required: ${required_var}=true (${diff_basis})"
exec "$@"
