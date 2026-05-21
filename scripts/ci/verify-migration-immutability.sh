#!/usr/bin/env bash
set -euo pipefail

# Prevent editing migrations that may already have been applied to shared or
# production databases. Add a new db/migrations/*.sql file instead.

MIGRATIONS_GLOB='db/migrations/*.sql'
BASE_REF="${MIGRATION_IMMUTABILITY_BASE_REF:-}"

has_ref() {
  git rev-parse --verify --quiet "$1^{commit}" >/dev/null
}

choose_base_ref() {
  if [[ -n "${BASE_REF}" ]]; then
    if has_ref "${BASE_REF}"; then
      printf '%s\n' "${BASE_REF}"
      return 0
    fi
    echo "configured migration immutability base ref does not exist: ${BASE_REF}" >&2
    return 1
  fi

  if [[ -n "${DRONE_TARGET_BRANCH:-}" ]] && has_ref "origin/${DRONE_TARGET_BRANCH}"; then
    printf '%s\n' "origin/${DRONE_TARGET_BRANCH}"
    return 0
  fi

  if has_ref origin/develop; then
    printf '%s\n' origin/develop
    return 0
  fi

  if has_ref origin/main; then
    printf '%s\n' origin/main
    return 0
  fi

  if has_ref HEAD~1; then
    printf '%s\n' HEAD~1
    return 0
  fi

  echo "could not determine base ref for migration immutability check" >&2
  return 1
}

report_forbidden_changes() {
  local title="$1"
  local changes="$2"
  if [[ -z "${changes}" ]]; then
    return 0
  fi

  cat >&2 <<MSG
${title}

Existing db/migrations/*.sql files are immutable once published.
Do not modify, delete, rename, copy, or type-change historical migrations.
Create a new numbered migration file instead.

Forbidden changes:
${changes}
MSG
  return 1
}

main() {
  local failed=0

  # Catch local unstaged/staged edits as well as committed PR diffs.
  local working_changes staged_changes base_ref pr_changes
  working_changes="$(git diff --name-status --find-renames --diff-filter=MDRTUXB HEAD -- ${MIGRATIONS_GLOB} || true)"
  staged_changes="$(git diff --cached --name-status --find-renames --diff-filter=MDRTUXB -- ${MIGRATIONS_GLOB} || true)"

  if ! report_forbidden_changes "Forbidden working-tree migration edits detected." "${working_changes}"; then
    failed=1
  fi
  if ! report_forbidden_changes "Forbidden staged migration edits detected." "${staged_changes}"; then
    failed=1
  fi

  base_ref="$(choose_base_ref)" || failed=1
  if [[ "${failed}" == "0" && -n "${base_ref}" ]]; then
    pr_changes="$(git diff --name-status --find-renames --diff-filter=MDRTUXB "${base_ref}"...HEAD -- ${MIGRATIONS_GLOB} || true)"
    if ! report_forbidden_changes "Forbidden committed migration edits detected against ${base_ref}." "${pr_changes}"; then
      failed=1
    fi
  fi

  if [[ "${failed}" != "0" ]]; then
    exit 1
  fi

  echo "Migration immutability check passed."
}

main "$@"
