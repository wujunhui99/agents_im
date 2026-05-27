#!/usr/bin/env bash
set -euo pipefail

# Enforce the multi-agent branch naming contract in CI.
# Required format:
#   <type>/<agent-name>/issue-<number>-<task-desc>
# Example:
#   fix/eino/issue-20-login-ci

ALLOWED_TYPES=(feature fix refactor docs test chore ci perf style hotfix)
TRUSTED_AGENTS=(codex eino helios hermes achilles furies gaia)

branch_name="${AGENT_BRANCH_NAME:-${DRONE_SOURCE_BRANCH:-${DRONE_COMMIT_BRANCH:-}}}"

if [[ -z "${branch_name}" ]]; then
  branch_name="$(git branch --show-current 2>/dev/null || true)"
fi

join_by() {
  local IFS="$1"
  shift
  printf '%s' "$*"
}

contains() {
  local needle="$1"
  shift
  local item
  for item in "$@"; do
    [[ "${item}" == "${needle}" ]] && return 0
  done
  return 1
}

fail() {
  cat >&2 <<MSG
Invalid agents_im agent branch name: ${branch_name:-<empty>}

CI requires source branches to use:
  <type>/<agent-name>/issue-<number>-<task-desc>

Example:
  fix/eino/issue-20-login-ci

Rules:
  - first segment <type> must be one of: $(join_by ', ' "${ALLOWED_TYPES[@]}")
  - second segment <agent-name> is required and must be one of: $(join_by ', ' "${TRUSTED_AGENTS[@]}")
  - third segment must start with issue-<number>-, for example issue-20-login-ci
  - use lowercase English slugs separated by '-'

Branches without a trusted agent name in the second path segment are rejected.
MSG
  exit 1
}

if [[ -z "${branch_name}" ]]; then
  fail
fi

# Integration branches are not task branches. PR source branches should never be
# these, but allow direct local/mainline checks to avoid false positives.
case "${branch_name}" in
  main|develop|devops)
    echo "Agent branch check skipped for integration branch: ${branch_name}"
    exit 0
    ;;
esac

IFS='/' read -r branch_type agent_name issue_slug extra <<< "${branch_name}"

if [[ -n "${extra:-}" || -z "${branch_type:-}" || -z "${agent_name:-}" || -z "${issue_slug:-}" ]]; then
  fail
fi

if ! contains "${branch_type}" "${ALLOWED_TYPES[@]}"; then
  fail
fi

if ! contains "${agent_name}" "${TRUSTED_AGENTS[@]}"; then
  fail
fi

if [[ ! "${issue_slug}" =~ ^issue-[0-9]+-[a-z0-9][a-z0-9-]*$ ]]; then
  fail
fi

if [[ "${branch_name}" != "${branch_name,,}" ]]; then
  fail
fi

if [[ "${branch_name}" =~ [[:space:]_] ]]; then
  fail
fi

echo "Agent branch check passed: ${branch_name}"
