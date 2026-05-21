#!/usr/bin/env bash
set -euo pipefail

status="${1:-${DRONE_BUILD_STATUS:-}}"
status="${status:-unknown}"

bot_token="${TELEGRAM_BOT_TOKEN:-}"
chat_id="${TELEGRAM_CHAT_ID:-}"
if [[ -z "${bot_token}" || -z "${chat_id}" ]]; then
  if [[ "${TELEGRAM_NOTIFY_REQUIRED:-false}" == "true" ]]; then
    echo "Telegram notification failed: TELEGRAM_BOT_TOKEN or TELEGRAM_CHAT_ID is not configured." >&2
    exit 1
  fi
  echo "Telegram notification skipped: TELEGRAM_BOT_TOKEN or TELEGRAM_CHAT_ID is not configured." >&2
  exit 0
fi

repo="${DRONE_REPO:-wujunhui99/agents_im}"
build_number="${DRONE_BUILD_NUMBER:-unknown}"
build_link="${DRONE_BUILD_LINK:-${DRONE_SYSTEM_PROTO:-https}://${DRONE_SYSTEM_HOST:-drone.agenticim.xyz}/${repo}/${build_number}}"
event="${DRONE_BUILD_EVENT:-unknown}"
source_branch="${DRONE_SOURCE_BRANCH:-${DRONE_BRANCH:-unknown}}"
target_branch="${DRONE_TARGET_BRANCH:-${DRONE_BRANCH:-unknown}}"
commit_sha="${DRONE_COMMIT_SHA:-unknown}"
commit_short="${commit_sha:0:12}"
commit_message="${DRONE_COMMIT_MESSAGE:-}"
commit_message="${commit_message//$'\r'/ }"
commit_message="${commit_message//$'\n'/ }"
if [[ ${#commit_message} -gt 120 ]]; then
  commit_message="${commit_message:0:117}..."
fi

pipeline="${DRONE_STAGE_NAME:-${DRONE_BUILD_STAGE_NAME:-unknown}}"
step="${DRONE_STEP_NAME:-notify telegram}"
pr="${DRONE_PULL_REQUEST:-}"
pr_text="- PR: ${pr:-N/A}"
if [[ -n "${pr}" && "${pr}" != "0" ]]; then
  pr_text="- PR: #${pr}"
fi

task_type="CI"
case "${event}:${target_branch}" in
  push:main) task_type="CD production deploy" ;;
  push:devops) task_type="CI/CD performance lab" ;;
  pull_request:*) task_type="PR verification" ;;
  promote:*) task_type="Drone promotion" ;;
esac

started="${DRONE_BUILD_STARTED:-0}"
finished="${DRONE_BUILD_FINISHED:-0}"
now="$(date +%s)"
if ! [[ "${started}" =~ ^[0-9]+$ ]]; then started=0; fi
if ! [[ "${finished}" =~ ^[0-9]+$ ]]; then finished=0; fi
if [[ "${finished}" == "0" ]]; then finished="${now}"; fi
duration="unknown"
if (( started > 0 && finished >= started )); then
  seconds=$((finished - started))
  if (( seconds >= 3600 )); then
    duration="$((seconds / 3600))h $(((seconds % 3600) / 60))m $((seconds % 60))s"
  elif (( seconds >= 60 )); then
    duration="$((seconds / 60))m $((seconds % 60))s"
  else
    duration="${seconds}s"
  fi
fi

emoji="ℹ️"
case "${status}" in
  success) emoji="✅" ;;
  failure|error|killed) emoji="❌" ;;
  running|pending) emoji="⏳" ;;
esac

changed_summary=""
if [[ -f .drone-deploy.env ]]; then
  # shellcheck disable=SC1091
  source ./.drone-deploy.env || true
  changed_summary=$'\nDeploy selection:'
  changed_summary+=$'\n- build_required: '"${build_required:-unknown}"
  changed_summary+=$'\n- deploy_required: '"${deploy_required:-unknown}"
  changed_summary+=$'\n- config_only: '"${config_only:-unknown}"
  if [[ -n "${image_services_space:-}" ]]; then
    changed_summary+=$'\n- image_services: '"${image_services_space}"
  fi
  if [[ -n "${rollout_services:-}" ]]; then
    changed_summary+=$'\n- rollout_services: '"${rollout_services}"
  fi
fi

text="${emoji} agents_im Drone ${status}
- Type: ${task_type}
- Build: #${build_number}
- Pipeline: ${pipeline}
- Notification step: ${step}
- Event: ${event}
- Branch: ${source_branch} -> ${target_branch}
${pr_text}
- Commit: ${commit_short}
- Duration: ${duration}
- Link: ${build_link}"

if [[ -n "${commit_message}" ]]; then
  text+=$'\n- Message: '"${commit_message}"
fi
text+="${changed_summary}"

python3 - <<'PY' "${bot_token}" "${chat_id}" "${text}"
import json
import sys
import urllib.parse
import urllib.request

token, chat_id, text = sys.argv[1:4]
url = f"https://api.telegram.org/bot{token}/sendMessage"
payload = urllib.parse.urlencode({
    "chat_id": chat_id,
    "text": text,
    "disable_web_page_preview": "true",
}).encode()
req = urllib.request.Request(url, data=payload, method="POST")
try:
    with urllib.request.urlopen(req, timeout=20) as resp:
        body = resp.read().decode("utf-8", errors="replace")
        if resp.status < 200 or resp.status >= 300:
            print(f"Telegram notification failed with HTTP {resp.status}: {body}", file=sys.stderr)
            sys.exit(1)
        data = json.loads(body)
        if not data.get("ok"):
            print(f"Telegram notification failed: {body}", file=sys.stderr)
            sys.exit(1)
        print("Telegram notification sent.")
except Exception as exc:
    print(f"Telegram notification failed: {exc}", file=sys.stderr)
    sys.exit(1)
PY
