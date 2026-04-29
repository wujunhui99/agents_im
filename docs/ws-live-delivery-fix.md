# WebSocket Live Delivery Fix

## Background

Single-machine E2E for registration, adding friends, sending messages, and pulling messages already worked through the REST/message storage path. However, when two users were both connected to `gateway-ws`, a message sent with the WebSocket `send_message` command was persisted and ACKed but was not immediately pushed to the online receiver.

This meant the practical behavior was:

- sender receives `send_message` ACK: yes
- message is stored and can be pulled later: yes
- online receiver receives `message_received` push immediately: no

## Root Cause

`internal/gateway/ws/server.go` handled `send_message` by calling `MessageLogic.SendMessage(...)` and returning the ACK payload. The persisted message was never handed to the gateway delivery dispatcher.

The gateway already had the pieces needed for local single-instance delivery:

- connection manager
- presence-aware in-memory delivery dispatcher
- `PushToUser` / `PushToConversation`
- `message_received` delivery event type

The missing part was wiring the WebSocket send path to that dispatcher after a successful non-duplicate send.

## Changes Made

### `internal/gateway/ws/server.go`

After `MessageLogic.SendMessage(...)` succeeds, the WebSocket handler now calls a new helper:

- `pushNewMessage(ctx, senderID, message)`

The helper:

1. calculates online push recipients for the stored message;
2. excludes the sender from push recipients;
3. emits a `delivery.EventMessageReceived` event through `PushToConversation`;
4. logs delivery errors without failing the original send ACK.

For now, recipient derivation supports single-chat messages by pushing to `message.ReceiverID`. Group-chat live fanout remains intentionally deferred because the stored message does not currently expose the full group participant list at the gateway boundary.

A conversion helper was also added:

- `toDeliveryMessage(logic.Message) delivery.Message`

### `tests/websocket_gateway_test.go`

Added regression coverage:

- `TestWebSocketGatewaySendMessagePushesToOnlineReceiver`

The test opens two WebSocket connections, sends a single-chat message from the sender, and asserts the receiver gets a `message_received` push with the same server message ID, conversation ID, sequence, sender, receiver, and content.

### `tests/mvp_backend_test.go`

Updated `TestMVPBackendWebSocketSendPullMarkReadSmoke` to explicitly consume and assert the new live `message_received` push before continuing with sequence sync, pull, and mark-read commands. This keeps the existing MVP smoke test deterministic now that receiver connections can receive asynchronous pushes.

## Behavior After Fix

For single-machine single-chat E2E:

1. user A connects to WebSocket;
2. user B connects to WebSocket;
3. user A sends `send_message` to user B;
4. user A receives normal ACK;
5. user B receives immediate `message_received` push;
6. the message remains pullable via REST/WebSocket pull.

## Verification

Focused WebSocket regression tests passed:

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./tests -run 'TestWebSocketGatewaySendMessagePushesToOnlineReceiver|TestWebSocketGatewaySendAndPullMessages' -count=1
```

Result:

```text
ok  github.com/wujunhui99/agents_im/tests
```

Full project verification should still be run after this change before merging.

## Local Runtime E2E Check

Because the existing `.dev/bin/gateway-ws` process was owned by root in this environment, it could not be replaced in-place by the current user. To validate the rebuilt gateway binary without disrupting the root-owned dev process, a temporary gateway was started on port `18084` with the same Postgres datasource and JWT secret.

The runtime check registered two users through the existing auth API, connected both users to the temporary gateway, sent a single-chat WebSocket `send_message`, and verified both:

- sender ACK status was `ok`;
- receiver got `message_received` over WebSocket.

Result:

```json
{
  "ackStatus": "ok",
  "pushType": "message_received"
}
```
