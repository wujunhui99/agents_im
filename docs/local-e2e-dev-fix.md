# Local E2E Dev Environment Fix

## Background

During single-machine E2E validation, the default development ports `8080-8085` were occupied by root-owned `.dev/bin/*` processes from an earlier startup. The current user could not stop those processes or overwrite root-owned `.dev/bin` and `.dev/etc` files.

This caused two practical problems:

1. `scripts/dev-up.sh --services-only` did not exist, so restarting host services without Docker access was not possible.
2. The existing `gateway-ws` on `8084` kept running an old binary, so the WebSocket live-delivery fix could not be validated on the default port without elevated permissions.

## Changes Made

### `scripts/dev-up.sh`

Added a `--services-only` mode:

```bash
scripts/dev-up.sh --services-only
```

This mode:

- skips Docker middleware startup;
- skips migrations;
- restarts only host Go services.

It is useful when Postgres/Redis/Redpanda are already running or managed externally, and the developer only needs to rebuild/restart Go services.

The script now also supports per-service port overrides:

```bash
USER_API_PORT=18080 \
AUTH_API_PORT=18081 \
FRIENDS_API_PORT=18082 \
MESSAGE_API_PORT=18083 \
GATEWAY_WS_PORT=18084 \
GROUPS_API_PORT=18085 \
AGENTS_IM_DEV_STATE_DIR=/tmp/agents-im-dev-e2e \
PATH=/tmp/go/bin:$HOME/go/bin:$PATH \
scripts/dev-up.sh --services-only
```

This lets a developer run a clean non-root service set on alternate ports even when the default ports are blocked by old processes.

### `cmd/single-machine-e2e`

Added a single-process E2E command that does not require Docker or bound HTTP ports:

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go run ./cmd/single-machine-e2e
```

It validates the core single-machine product flow in-process using real business logic and a real WebSocket gateway test server:

1. register Alice;
2. register Bob;
3. add Bob as Alice's friend;
4. send a single-chat message through message logic;
5. pull the message as Bob;
6. connect Alice and Bob to WebSocket;
7. send `send_message` over WebSocket;
8. assert Alice receives ACK and Bob receives live `message_received` push.

This command is meant as a fast smoke check for the exact local E2E scenario when the external dev environment is unavailable or polluted by stale/root-owned processes.

## Verification

Single-process E2E passed:

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go run ./cmd/single-machine-e2e
```

Observed result:

```text
single-process e2e passed
alice_user_id=usr_000001
bob_user_id=usr_000002
rest_conversation_id=single:usr_000001:usr_000002
ws_server_msg_id=msg_000002
```

## Remaining Environment Note

In this specific machine state, default ports `8080-8085` are still held by root-owned processes under `/home/ws/project/agents_im/.dev/bin`. Without elevated permissions, the current user cannot stop or replace those processes. Use alternate ports with `AGENTS_IM_DEV_STATE_DIR` or clean the root-owned processes externally before using default ports.
