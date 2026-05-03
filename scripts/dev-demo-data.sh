#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AUTH_API_URL="${AUTH_API_URL:-http://127.0.0.1:8081}"
FRIENDS_API_URL="${FRIENDS_API_URL:-http://127.0.0.1:8082}"
MESSAGE_API_URL="${MESSAGE_API_URL:-http://127.0.0.1:8083}"
GROUPS_API_URL="${GROUPS_API_URL:-http://127.0.0.1:8085}"
DEMO_PASSWORD="${DEMO_PASSWORD:-local-demo-password}"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "required command not found: $1" >&2
    exit 1
  fi
}

json_get() {
  local file="$1"
  local path="$2"
  python3 - "$file" "$path" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    value = json.load(fh)

for part in sys.argv[2].split("."):
    value = value[part]

print(value)
PY
}

request_json() {
  local method="$1"
  local url="$2"
  local body="$3"
  local bearer_token="$4"
  local output_file="$5"
  local -a args=(--silent --show-error --output "${output_file}" --write-out "%{http_code}" --request "${method}" --header "Content-Type: application/json")

  if [[ -n "${bearer_token}" ]]; then
    args+=(--header "Authorization: Bearer ${bearer_token}")
  fi
  if [[ -n "${body}" ]]; then
    args+=(--data "${body}")
  fi
  args+=("${url}")

  curl "${args[@]}"
}

register_or_login() {
  local identifier="$1"
  local display_name="$2"
  local output_file="$3"
  local status

  status="$(request_json POST "${AUTH_API_URL}/auth/register" "{\"identifier\":\"${identifier}\",\"password\":\"${DEMO_PASSWORD}\",\"display_name\":\"${display_name}\"}" "" "${output_file}")"
  if [[ "${status}" == "200" ]]; then
    return 0
  fi
  if [[ "${status}" != "409" ]]; then
    echo "register ${identifier} failed with HTTP ${status}" >&2
    cat "${output_file}" >&2
    exit 1
  fi

  status="$(request_json POST "${AUTH_API_URL}/auth/login" "{\"identifier\":\"${identifier}\",\"password\":\"${DEMO_PASSWORD}\"}" "" "${output_file}")"
  if [[ "${status}" != "200" ]]; then
    echo "login ${identifier} failed with HTTP ${status}" >&2
    cat "${output_file}" >&2
    exit 1
  fi
}

main() {
  cd "${ROOT_DIR}"
  require_command curl
  require_command python3

  tmp_dir="$(mktemp -d)"
  trap 'rm -rf -- "$tmp_dir"' EXIT

  local alice_json="${tmp_dir}/alice.json"
  local bob_json="${tmp_dir}/bob.json"
  register_or_login "alice_demo" "Alice Demo" "${alice_json}"
  register_or_login "bob_demo" "Bob Demo" "${bob_json}"

  local alice_id bob_id alice_token bob_token
  alice_id="$(json_get "${alice_json}" "data.user_id")"
  bob_id="$(json_get "${bob_json}" "data.user_id")"
  alice_token="$(json_get "${alice_json}" "data.token")"
  bob_token="$(json_get "${bob_json}" "data.token")"

  local friend_json="${tmp_dir}/friend.json"
  local status
  status="$(request_json POST "${FRIENDS_API_URL}/friends" "{\"user_id\":\"${bob_id}\"}" "${alice_token}" "${friend_json}")"
  if [[ "${status}" != "200" ]]; then
    echo "add friend failed with HTTP ${status}" >&2
    cat "${friend_json}" >&2
    exit 1
  fi
  status="$(request_json POST "${FRIENDS_API_URL}/friends/${alice_id}/accept" '{}' "${bob_token}" "${friend_json}")"
  if [[ "${status}" != "200" ]]; then
    echo "accept friend failed with HTTP ${status}" >&2
    cat "${friend_json}" >&2
    exit 1
  fi

  local group_json="${tmp_dir}/group.json"
  status="$(request_json POST "${GROUPS_API_URL}/groups" "{\"name\":\"Frontend Demo\",\"description\":\"MVP smoke room\"}" "${alice_token}" "${group_json}")"
  if [[ "${status}" != "200" ]]; then
    echo "create group failed with HTTP ${status}" >&2
    cat "${group_json}" >&2
    exit 1
  fi

  local group_id
  group_id="$(json_get "${group_json}" "data.group_id")"

  local member_json="${tmp_dir}/member.json"
  status="$(request_json POST "${GROUPS_API_URL}/groups/${group_id}/members" "{\"user_id\":\"${bob_id}\"}" "${alice_token}" "${member_json}")"
  if [[ "${status}" != "200" ]]; then
    echo "add group member failed with HTTP ${status}" >&2
    cat "${member_json}" >&2
    exit 1
  fi

  local client_msg_id="demo-$(date +%s)"
  local send_json="${tmp_dir}/send.json"
  status="$(request_json POST "${MESSAGE_API_URL}/messages" "{\"receiverId\":\"${bob_id}\",\"chatType\":\"single\",\"clientMsgId\":\"${client_msg_id}\",\"contentType\":\"text\",\"content\":\"hello from local demo\"}" "${alice_token}" "${send_json}")"
  if [[ "${status}" != "200" ]]; then
    echo "send message failed with HTTP ${status}" >&2
    cat "${send_json}" >&2
    exit 1
  fi

  local conversation_id
  conversation_id="$(json_get "${send_json}" "data.message.conversationId")"

  local pull_json="${tmp_dir}/pull.json"
  status="$(request_json GET "${MESSAGE_API_URL}/conversations/${conversation_id}/messages?fromSeq=1&limit=10&order=asc" "" "${bob_token}" "${pull_json}")"
  if [[ "${status}" != "200" ]]; then
    echo "pull messages failed with HTTP ${status}" >&2
    cat "${pull_json}" >&2
    exit 1
  fi

  local read_json="${tmp_dir}/read.json"
  status="$(request_json POST "${MESSAGE_API_URL}/conversations/${conversation_id}/read" '{"hasReadSeq":1}' "${bob_token}" "${read_json}")"
  if [[ "${status}" != "200" ]]; then
    echo "mark read failed with HTTP ${status}" >&2
    cat "${read_json}" >&2
    exit 1
  fi

  echo "demo data ready"
  echo "alice_user_id=${alice_id}"
  echo "bob_user_id=${bob_id}"
  echo "group_id=${group_id}"
  echo "conversation_id=${conversation_id}"
}

main "$@"
