#!/usr/bin/env bash
set -euo pipefail

export PATH=/tmp/go/bin:"${HOME}/go/bin:${PATH}"

if command -v apt-get >/dev/null 2>&1; then
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y postgresql-client
else
  echo "apt-get is required by the Drone PostgreSQL integration image" >&2
  exit 1
fi

: "${DATABASE_URL:?DATABASE_URL is required and must point to the Drone postgres service}"

for i in $(seq 1 30); do
  if pg_isready -h postgres -U agents_im -d agents_im >/dev/null 2>&1; then
    break
  fi
  if [[ "$i" == "30" ]]; then
    echo "postgres service did not become ready" >&2
    exit 1
  fi
  sleep 2
done

bash scripts/migrate-postgres.sh --host-psql
go test -tags=integration ./tests
# msg-rpc 读路径 roundtrip（拉历史 → seq 状态 → 已读）：直连 PG，DATABASE_URL 即可。
# 此前缺失导致 #531 的 message_id bigint 化后 QuerySeqState 的 bigint=text join 未被 CI 捕获。
go test -tags=integration -timeout 180s ./service/msg/rpc/...
# msgtransfer Kafka 链路（03 §9 B1）：需要 redpanda + redis service（.drone.yml）。
go test -tags=integration -timeout 180s ./service/msgtransfer/...
