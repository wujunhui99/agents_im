#!/usr/bin/env bash
set -euo pipefail

token="${TELEGRAM_BOT_TOKEN:-}"
chat_id="${TELEGRAM_CHAT_ID:-}"

if [[ -z "${token}" || -z "${chat_id}" ]]; then
  echo "Telegram notification skipped: TELEGRAM_BOT_TOKEN or TELEGRAM_CHAT_ID is not configured"
  exit 0
fi

workflow="${GITHUB_WORKFLOW:-unknown}"
status="${ACTIONS_NOTIFY_STATUS:-unknown}"
repository="${GITHUB_REPOSITORY:-unknown}"
ref_name="${GITHUB_REF_NAME:-}"
sha="${GITHUB_SHA:-}"
actor="${GITHUB_ACTOR:-unknown}"
server_url="${GITHUB_SERVER_URL:-https://github.com}"
run_id="${GITHUB_RUN_ID:-}"
details="${ACTIONS_NOTIFY_DETAILS:-}"

if [[ -z "${ref_name}" ]]; then
  ref_name="${GITHUB_REF:-unknown}"
  ref_name="${ref_name#refs/heads/}"
  ref_name="${ref_name#refs/tags/}"
fi

if [[ -n "${sha}" ]]; then
  short_sha="${sha:0:7}"
else
  short_sha="unknown"
fi

if [[ "${repository}" != "unknown" && -n "${run_id}" ]]; then
  run_url="${server_url}/${repository}/actions/runs/${run_id}"
else
  run_url="unknown"
fi

message="$(
  cat <<MSG
Workflow: ${workflow}
Status: ${status}
Repository: ${repository}
Ref: ${ref_name}
SHA: ${short_sha}
Actor: ${actor}
Run: ${run_url}
MSG
)"

if [[ -n "${details}" ]]; then
  message="${message}"$'\n\n'"Jobs:"$'\n'"${details}"
fi

dry_run="${ACTIONS_TELEGRAM_DRY_RUN:-}"
if [[ "${dry_run}" == "1" || "${dry_run}" == "true" ]]; then
  echo "Telegram notification dry-run:"
  printf '%s\n' "${message}"
  exit 0
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "::warning::Telegram notification skipped; python3 is required to encode the request payload"
  exit 0
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "::warning::Telegram notification skipped; curl is not available"
  exit 0
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT
payload_file="${tmp_dir}/telegram-payload.json"
response_file="${tmp_dir}/telegram-response.json"

if ! TELEGRAM_CHAT_ID="${chat_id}" \
  ACTIONS_NOTIFY_MESSAGE="${message}" \
  python3 - <<'PY' >"${payload_file}"
import json
import os

payload = {
    "chat_id": os.environ["TELEGRAM_CHAT_ID"],
    "text": os.environ["ACTIONS_NOTIFY_MESSAGE"],
    "disable_web_page_preview": True,
}
print(json.dumps(payload, ensure_ascii=False))
PY
then
  echo "::warning::Telegram notification skipped; failed to encode the request payload"
  exit 0
fi

telegram_url="https://api.telegram.org/bot${token}/sendMessage"
if curl \
  --fail \
  --silent \
  --show-error \
  --connect-timeout 10 \
  --max-time 20 \
  --retry 2 \
  --header "Content-Type: application/json" \
  --data @"${payload_file}" \
  --output "${response_file}" \
  "${telegram_url}"; then
  echo "Telegram notification sent"
else
  echo "::warning::Telegram notification failed; continuing workflow"
fi

exit 0
