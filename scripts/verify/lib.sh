# shellcheck shell=bash
# Shared helpers for the verify-*-static.sh static gates.
# Source this from each split script: `source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"`.
# All checks assume CWD is the repo root.

# require_files <file>...: every path must exist as a regular file.
require_files() {
  local f
  for f in "$@"; do
    if [[ ! -f "$f" ]]; then
      echo "missing required file: $f" >&2
      exit 1
    fi
  done
}

# forbid_paths <message> <path>...: none of the paths may exist (file or dir).
forbid_paths() {
  local msg="$1"; shift
  local p
  for p in "$@"; do
    if [[ -e "$p" ]]; then
      echo "${msg}: $p" >&2
      exit 1
    fi
  done
}

# syntax_check <script>...: bash -n each shell script.
syntax_check() {
  local s
  for s in "$@"; do
    bash -n "$s"
  done
}

# assert_present <rg-flags> <target>... -- <pattern>...
# Each pattern must match in at least one target. Replaces the repeated
# `for pattern in "${arr[@]}"; do rg -q "$pattern" files; done` blocks.
# rg-flags is one token, e.g. -q (regex) or -qF (fixed string).
assert_present() {
  local flags="$1"; shift
  local targets=()
  while [[ $# -gt 0 && "$1" != "--" ]]; do
    targets+=("$1"); shift
  done
  shift || true # drop the --
  local p
  for p in "$@"; do
    if ! rg "$flags" -- "$p" "${targets[@]}"; then
      echo "verify: required pattern not found: ${p} (searched: ${targets[*]})" >&2
      exit 1
    fi
  done
}

# forbid_match <message> <rg-args>...: fail if rg matches anything.
forbid_match() {
  local msg="$1"; shift
  if rg "$@"; then
    echo "$msg" >&2
    exit 1
  fi
}
