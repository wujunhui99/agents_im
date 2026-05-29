#!/usr/bin/env bash
# 监控最新一次 Drone CI 构建，完成后退出（由 Bash run_in_background 调用）
set -euo pipefail

TOKEN=$(grep DRONE_TOKEN secret/drone_token | cut -d= -f2 | tr -d '\n')
prev_key=""

while true; do
  result=$(curl -sf --max-time 10 -H "Authorization: Bearer ${TOKEN}" \
    "https://drone.agenticim.xyz/api/repos/wujunhui99/agents_im/builds?limit=1" 2>/dev/null || echo '[]')
  build_num=$(echo "$result" | python3 -c "import sys,json; b=json.load(sys.stdin)[0]; print(b['number'])" 2>/dev/null)
  build_state=$(echo "$result" | python3 -c "import sys,json; b=json.load(sys.stdin)[0]; print(b['status'])" 2>/dev/null)
  build_msg=$(echo "$result" | python3 -c "import sys,json; b=json.load(sys.stdin)[0]; print(b['message'][:60])" 2>/dev/null)
  key="${build_num}:${build_state}"
  if [ "$key" != "$prev_key" ] && [ -n "$build_state" ]; then
    echo "[$(date +%H:%M:%S)] #${build_num} ${build_state} — ${build_msg}"
    prev_key="$key"
  fi
  case "$build_state" in success|failure|error|killed) break ;; esac
  sleep 5
done
