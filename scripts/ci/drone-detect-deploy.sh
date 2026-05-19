#!/usr/bin/env bash
set -euo pipefail

out_file="${1:-.drone-deploy.env}"

event="${DRONE_BUILD_EVENT:-push}"
branch="${DRONE_BRANCH:-}"
commit="${DRONE_COMMIT_SHA:-HEAD}"
before="${DRONE_COMMIT_BEFORE:-}"

ref="refs/heads/${branch}"
changed=()

if [[ "${event}" != "promote" ]]; then
  if [[ -n "${before}" && ! "${before}" =~ ^0+$ ]] && git cat-file -e "${before}^{commit}" 2>/dev/null; then
    mapfile -t changed < <(git diff --name-only "${before}" "${commit}")
  elif git rev-parse --verify "${commit}^" >/dev/null 2>&1; then
    mapfile -t changed < <(git diff --name-only "${commit}^" "${commit}")
  else
    mapfile -t changed < <(git ls-files)
  fi
fi

outputs="$(python3 scripts/detect-deploy-changes.py \
  --event-name "${event}" \
  --ref "${ref}" \
  "${changed[@]}")"

printf '%s\n' "${outputs}" | tee "${out_file}"
