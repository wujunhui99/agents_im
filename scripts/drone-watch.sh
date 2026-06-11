#!/usr/bin/env bash
# 监控最新一次 Drone CI 构建，完成后退出（由 Bash run_in_background 调用）。
# 瞬时 API 失败（curl 超时/网络抖动/响应不完整）只跳过本 tick 继续轮询，不退出（#486）。
set -uo pipefail

TOKEN=$(grep DRONE_TOKEN secret/drone_token | cut -d= -f2 | tr -d '\n')
prev_key=""

while true; do
  line=$(curl -sf --max-time 10 -H "Authorization: Bearer ${TOKEN}" \
    "https://drone.agenticim.xyz/api/repos/wujunhui99/agents_im/builds?limit=1" 2>/dev/null |
    python3 -c '
import json, sys
try:
    b = json.load(sys.stdin)[0]
    lines = b.get("message", "").splitlines()
    print(b["number"], b["status"], lines[0][:60] if lines else "")
except Exception:
    pass
' 2>/dev/null) || line=""
  if [ -n "$line" ]; then
    build_num=${line%% *}
    rest=${line#* }
    build_state=${rest%% *}
    key="${build_num}:${build_state}"
    if [ "$key" != "$prev_key" ]; then
      echo "[$(date +%H:%M:%S)] #${line}"
      prev_key="$key"
    fi
    case "$build_state" in success|failure|error|killed) exit 0 ;; esac
  fi
  sleep 5
done
