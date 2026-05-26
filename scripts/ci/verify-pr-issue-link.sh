#!/usr/bin/env bash
set -euo pipefail

# Enforce the agents_im issue lifecycle contract for pull_request builds.
# Rule: one issue -> one PR; PR body must contain exactly one GitHub closing keyword.
# This makes a PR merge to main via GitHub Merge Queue the explicit completion point for its issue.

if [[ "${DRONE_BUILD_EVENT:-}" != "pull_request" ]]; then
  echo "PR issue link check skipped: DRONE_BUILD_EVENT=${DRONE_BUILD_EVENT:-<unset>}"
  exit 0
fi

repo="${DRONE_REPO:-wujunhui99/agents_im}"
pr="${DRONE_PULL_REQUEST:-}"
if [[ -z "${pr}" ]]; then
  ref="${DRONE_COMMIT_REF:-}"
  if [[ "${ref}" =~ refs/pull/([0-9]+)/ ]]; then
    pr="${BASH_REMATCH[1]}"
  fi
fi

if [[ -z "${pr}" ]]; then
  echo "PR issue link check failed: cannot determine pull request number from Drone env." >&2
  exit 1
fi

body=""
if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
  body="$(gh pr view "${pr}" --repo "${repo}" --json body --jq .body 2>/dev/null || true)"
elif [[ -n "${GITHUB_TOKEN:-}" ]]; then
  body="$(python3 - "${repo}" "${pr}" <<'PY'
import json, os, sys, urllib.request
repo, pr = sys.argv[1], sys.argv[2]
token = os.environ["GITHUB_TOKEN"]
req = urllib.request.Request(
    f"https://api.github.com/repos/{repo}/pulls/{pr}",
    headers={"Authorization": f"Bearer {token}", "Accept": "application/vnd.github+json"},
)
with urllib.request.urlopen(req, timeout=20) as resp:
    print((json.load(resp).get("body") or ""))
PY
  )"
else
  echo "PR issue link check skipped: gh is not authenticated and GITHUB_TOKEN is not set." >&2
  echo "Drone secrets should provide a token if this gate is required in CI." >&2
  exit 0
fi

if [[ -z "${body//[[:space:]]/}" ]]; then
  echo "PR issue link check failed: PR #${pr} body is empty; include 'Closes #<issue>' exactly once." >&2
  exit 1
fi

matches="$({ printf '%s\n' "${body}" | grep -Eio '\b(close[sd]?|fix(e[sd])?|resolve[sd]?) +#[0-9]+' || true; } | sed -E 's/.*#([0-9]+)/#\1/' | sort -u)"
count="$(printf '%s\n' "${matches}" | sed '/^$/d' | wc -l | tr -d ' ')"

if [[ "${count}" != "1" ]]; then
  echo "PR issue link check failed: PR #${pr} must contain exactly one closing issue keyword." >&2
  echo "Expected one of: Closes #123, Fixes #123, Resolves #123" >&2
  echo "Found ${count}: ${matches:-<none>}" >&2
  exit 1
fi

echo "PR issue link check passed: PR #${pr} closes ${matches}."
