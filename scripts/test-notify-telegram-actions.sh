#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

if ! output="$(
  env -i \
    PATH="${PATH}" \
    HOME="${HOME}" \
    GITHUB_WORKFLOW="CI" \
    GITHUB_REPOSITORY="wujunhui99/agents_im" \
    GITHUB_REF_NAME="develop" \
    GITHUB_SHA="1234567890abcdef" \
    GITHUB_ACTOR="octocat" \
    GITHUB_SERVER_URL="https://github.com" \
    GITHUB_RUN_ID="1001" \
    ACTIONS_NOTIFY_STATUS="success" \
    bash "${ROOT_DIR}/scripts/notify-telegram-actions.sh" \
      2>&1
)"; then
  echo "expected missing credentials to skip without failing" >&2
  printf '%s\n' "${output}" >&2
  exit 1
fi

if ! grep -Fq "Telegram notification skipped: TELEGRAM_BOT_TOKEN or TELEGRAM_CHAT_ID is not configured" <<<"${output}"; then
  echo "expected missing credential skip log" >&2
  printf '%s\n' "${output}" >&2
  exit 1
fi

if ! dry_run="$(
  env -i \
    PATH="${PATH}" \
    HOME="${HOME}" \
    TELEGRAM_BOT_TOKEN="fake-token-for-test" \
    TELEGRAM_CHAT_ID="fake-chat-for-test" \
    ACTIONS_TELEGRAM_DRY_RUN="1" \
    GITHUB_WORKFLOW="Deploy to k3s" \
    GITHUB_REPOSITORY="wujunhui99/agents_im" \
    GITHUB_REF_NAME="main" \
    GITHUB_SHA="abcdef1234567890" \
    GITHUB_ACTOR="octocat" \
    GITHUB_SERVER_URL="https://github.com" \
    GITHUB_RUN_ID="1002" \
    ACTIONS_NOTIFY_STATUS="failure" \
    bash "${ROOT_DIR}/scripts/notify-telegram-actions.sh" \
      2>&1
)"; then
  echo "expected dry-run notification to succeed" >&2
  printf '%s\n' "${dry_run}" >&2
  exit 1
fi

for expected in \
  "Workflow: Deploy to k3s" \
  "Status: failure" \
  "Repository: wujunhui99/agents_im" \
  "Ref: main" \
  "SHA: abcdef1" \
  "Actor: octocat" \
  "Run: https://github.com/wujunhui99/agents_im/actions/runs/1002"; do
  if ! grep -Fq "${expected}" <<<"${dry_run}"; then
    echo "dry-run output missing ${expected}" >&2
    printf '%s\n' "${dry_run}" >&2
    exit 1
  fi
done

if grep -Fq "fake-token-for-test" <<<"${dry_run}"; then
  echo "dry-run output must not include the bot token" >&2
  printf '%s\n' "${dry_run}" >&2
  exit 1
fi

FAKE_CURL="${TMP_DIR}/curl"
cat >"${FAKE_CURL}" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
echo "simulated Telegram outage" >&2
exit 22
FAKE
chmod +x "${FAKE_CURL}"

if ! failure_output="$(
  env -i \
    PATH="${TMP_DIR}:${PATH}" \
    HOME="${HOME}" \
    TELEGRAM_BOT_TOKEN="fake-token-for-test" \
    TELEGRAM_CHAT_ID="fake-chat-for-test" \
    GITHUB_WORKFLOW="CI" \
    GITHUB_REPOSITORY="wujunhui99/agents_im" \
    GITHUB_REF_NAME="develop" \
    GITHUB_SHA="1234567890abcdef" \
    GITHUB_ACTOR="octocat" \
    GITHUB_SERVER_URL="https://github.com" \
    GITHUB_RUN_ID="1003" \
    ACTIONS_NOTIFY_STATUS="cancelled" \
    bash "${ROOT_DIR}/scripts/notify-telegram-actions.sh" \
      2>&1
)"; then
  echo "expected Telegram send failure to remain best-effort" >&2
  printf '%s\n' "${failure_output}" >&2
  exit 1
fi

if ! grep -Fq "::warning::Telegram notification failed; continuing workflow" <<<"${failure_output}"; then
  echo "expected best-effort warning on Telegram send failure" >&2
  printf '%s\n' "${failure_output}" >&2
  exit 1
fi

if grep -Fq "fake-token-for-test" <<<"${failure_output}"; then
  echo "failure output must not include the bot token" >&2
  printf '%s\n' "${failure_output}" >&2
  exit 1
fi
