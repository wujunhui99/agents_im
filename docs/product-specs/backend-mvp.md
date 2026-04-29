# Backend MVP Product Spec

## Goal

Finish the IM backend to the minimum level needed before frontend development starts.

The MVP backend must support a web frontend demo where users can register, log in, manage profiles, add friends, create/join groups, send one-to-one and group messages over WebSocket, receive online messages, reconnect, sync missed messages, mark messages read, and run the full stack locally with docker-compose.

## In Scope

### Account and auth

- Register with username/password through Auth Service.
- Login returns JWT access token.
- Query `/me` and update basic profile.
- Search users by unique identifier.
- Passwords and auth secrets remain owned by Auth Service.

### Friends MVP

- `AddFriend` immediately creates an accepted friendship.
- Adding the same friend is idempotent.
- Self-add is rejected.
- Adding a non-existent user is rejected.
- `DeleteFriend` removes the relationship for MVP purposes.
- `ListFriends` returns current accepted friends.

Friend request approval, blacklist, privacy controls, and recommendations are non-MVP.

### Groups MVP

- Group creator becomes owner and member.
- Groups are open-join by default.
- Members may leave.
- Owner leaving rules may be minimal: reject owner leaving if they are the only owner/member, or dissolve only if explicitly implemented.
- Group message send requires sender membership.
- `ListGroups` and `ListMembers` must be frontend-ready.

Invite-only groups, admin roles beyond owner, moderation, mute, announcement, and join approval are non-MVP.

### Messaging MVP

- WebSocket Gateway accepts JWT-authenticated connections.
- `send_message` writes message through Message Service and returns command ACK.
- Message Service persists to PostgreSQL and outbox in one transaction.
- Outbox publisher publishes message events to Kafka/Redpanda-compatible producer.
- Transfer worker consumes Kafka events and calls Gateway delivery dispatcher.
- Gateway dispatcher pushes `message_received` events to local online WebSocket connections.
- Missed messages are recovered by conversation seq sync.
- `mark_conversation_read` updates read state and unread count.

### Delivery semantics

MVP delivery states:

- `accepted`: message persisted by Message Service.
- `published`: message event published by outbox publisher.
- `delivered`: Gateway pushed the message to at least one online connection for that recipient.
- `offline`: no online route/connection exists for recipient.
- `failed`: delivery attempt failed after retry policy or non-retryable error.

`delivered` is not the same as `read`. Read state remains `has_read_seq`.

### Reconnect and sync

After connect/reconnect, frontend must be able to:

1. Call `get_conversation_seqs`.
2. Compare local last seq with server `max_seq`.
3. Call `pull_messages` from local last seq + 1.
4. Mark read with `mark_conversation_read`.

Online WebSocket delivery is best-effort; PostgreSQL message history is authoritative.

### Health and observability

- `/healthz` reports process alive.
- `/readyz` reports basic readiness/config/dependency status where safe.
- Basic metrics exist for messages, delivery attempts, transfer events, and WebSocket connections.
- Logs include request/trace/connection IDs without secrets.

### Frontend contract

The backend must provide a frontend handoff document with:

- REST endpoint examples.
- WebSocket connection/auth examples.
- WebSocket command examples.
- Server push event examples.
- Error envelope examples.
- Local docker-compose startup instructions.

## Non-MVP

- Native mobile push: APNs, FCM, vendor push.
- True multi-Gateway remote RPC/PubSub delivery if single Gateway demo works.
- End-to-end encryption.
- Media upload/CDN.
- Production Kubernetes/Helm.
- Full Agent Service runtime.
- Advanced friend requests, blacklist, privacy settings.
- Advanced group roles/moderation.
