#!/usr/bin/env bash
set -euo pipefail

out_file="${1:-.drone-changes.env}"
event="${DRONE_BUILD_EVENT:-}"
commit="${DRONE_COMMIT_SHA:-HEAD}"
before="${DRONE_COMMIT_BEFORE:-}"
target_branch="${DRONE_TARGET_BRANCH:-}"

has_commit() {
  git cat-file -e "$1^{commit}" 2>/dev/null
}

has_ref() {
  git rev-parse --verify --quiet "$1^{commit}" >/dev/null
}

shell_quote() {
  local value="$1"
  printf "'%s'" "${value//\'/\'\\\'\'}"
}

fetch_target_ref() {
  local branch="$1"
  local ref="origin/${branch}"

  [[ -n "${branch}" ]] || return 1
  if has_ref "${ref}"; then
    printf '%s\n' "${ref}"
    return 0
  fi

  if git remote get-url origin >/dev/null 2>&1; then
    git fetch --no-tags --depth=200 origin \
      "+refs/heads/${branch}:refs/remotes/origin/${branch}" >/dev/null 2>&1 || true
  fi

  if has_ref "${ref}"; then
    printf '%s\n' "${ref}"
    return 0
  fi

  return 1
}

detect_changed_paths() {
  local base_ref

  if [[ "${event}" == "pull_request" ]] && base_ref="$(fetch_target_ref "${target_branch}")"; then
    if ! git merge-base "${base_ref}" "${commit}" >/dev/null 2>&1; then
      if [[ "$(git rev-parse --is-shallow-repository 2>/dev/null || printf false)" == "true" ]]; then
        git fetch --no-tags --deepen=1000 origin \
          "+refs/heads/${target_branch}:refs/remotes/origin/${target_branch}" >/dev/null 2>&1 || true
      fi
    fi

    if git merge-base "${base_ref}" "${commit}" >/dev/null 2>&1; then
      diff_basis="${base_ref}...${commit}"
      mapfile -t changed_paths < <(git diff --name-only "${base_ref}...${commit}")
      return 0
    fi

    echo "Could not find merge base for ${base_ref} and ${commit}; using two-dot diff." >&2
    diff_basis="${base_ref}..${commit}"
    mapfile -t changed_paths < <(git diff --name-only "${base_ref}" "${commit}")
    return 0
  fi

  if [[ -n "${before}" && ! "${before}" =~ ^0+$ ]] && has_commit "${before}" && has_commit "${commit}"; then
    diff_basis="${before}..${commit}"
    mapfile -t changed_paths < <(git diff --name-only "${before}" "${commit}")
    return 0
  fi

  if has_ref "${commit}^"; then
    diff_basis="${commit}^..${commit}"
    mapfile -t changed_paths < <(git diff --name-only "${commit}^" "${commit}")
    return 0
  fi

  echo "Could not determine changed files; failing open and running all verification." >&2
  diff_basis="git-ls-files"
  mapfile -t changed_paths < <(git ls-files)
}

diff_basis=""
changed_paths=()
detect_changed_paths

frontend_required=false
markdown_required=false
backend_required=false

for path in "${changed_paths[@]}"; do
  path="${path#./}"

  if [[ "${path}" == web/* ]]; then
    frontend_required=true
  fi

  if [[ "${path}" == *.md ]]; then
    markdown_required=true
  fi

  if [[ "${path}" != web/* && "${path}" != *.md ]]; then
    backend_required=true
  fi
done

{
  printf 'frontend_required=%s\n' "${frontend_required}"
  printf 'markdown_required=%s\n' "${markdown_required}"
  printf 'backend_required=%s\n' "${backend_required}"
  printf 'change_diff_basis=%s\n' "$(shell_quote "${diff_basis}")"
  printf 'change_count=%s\n' "${#changed_paths[@]}"
} >"${out_file}"

echo "Change detection (${diff_basis}): frontend=${frontend_required} markdown=${markdown_required} backend=${backend_required} files=${#changed_paths[@]}"
