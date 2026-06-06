#!/usr/bin/env bash
# Contract surface markers: API routes, proto rpcs, DB schema columns, generated
# artifacts, and required code symbols across backend services.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"
cd "$(git rev-parse --show-toplevel)"

# --- REST API route surface ---
assert_present "-q" service/user/api/user.api -- \
  "get /me" "patch /me" "post /users" "post /accounts" \
  "get /users/exists" "get /accounts/exists" "get /users/:identifier" "get /accounts/:identifier"
assert_present "-q" service/auth/api/auth.api -- \
  "post /auth/register" "post /auth/login" "post /auth/validate"
assert_present "-q" service/friends/api/friends.api -- \
  "post /friends" "delete /friends/:user_id" "get /friends" "get /friends/:user_id"
assert_present "-q" service/groups/api/groups.api -- \
  "post /groups" "get /groups/:group_id" "post /groups/:group_id/members" \
  "delete /groups/:group_id/members/me" "get /groups/:group_id/members"
assert_present "-q" api/message.api -- \
  "post /messages" "get /conversations/:conversation_id/messages" \
  "get /conversations/seqs" "post /conversations/:conversation_id/read"

# --- message ordering contract (schema + code + tests, backend & web) ---
assert_present "-q" db/migrations/001_init_postgres.sql -- \
  "messages_conversation_seq_uniq" "messages_sender_client_msg_uniq" "conversation_threads"
assert_present "-q" internal/repository/postgres_message.go web/src/features/messages/MessagesPage.tsx -- \
  "for update" "existingMessageForIdempotency" "orderedChatMessages" "conversationHasInFlightSend"
assert_present "-q" internal/repository/message_repository_contract_test.go web/src/features/messages/MessagesPage.test.tsx -- \
  "concurrent same conversation sends allocate contiguous seqs" "last message state follows max seq" \
  "renders shuffled server messages by authoritative seq" "one in-flight send per conversation"

# --- proto rpc/message surface ---
assert_present "-q" service/user/rpc/user.proto -- \
  "rpc CreateUser" "rpc GetUserByIdentifier" "rpc ExistsByIdentifier" "rpc GetUserByID" "rpc UpdateUserProfile" "rpc UpdateUserAvatar"
assert_present "-q" service/auth/rpc/auth.proto -- \
  "rpc Register" "rpc Login" "rpc ValidateToken" "rpc ParseToken"
assert_present "-q" service/friends/rpc/friends.proto -- \
  "rpc AddFriend" "rpc DeleteFriend" "rpc ListFriends" "rpc GetFriendship"
assert_present "-q" service/groups/rpc/groups.proto -- \
  "rpc CreateGroup" "rpc GetGroup" "rpc AddMember" "rpc JoinGroup" "rpc LeaveGroup" "rpc ListMembers"
assert_present "-q" internal/rpcgen/message/message.proto -- \
  "service MessageService" "rpc SendMessage" "rpc PullMessages" "rpc GetConversationSeqs" \
  "rpc MarkConversationAsRead" "message Message" "message ConversationSeqState"
assert_present "-q" service/third/rpc/mail.proto -- \
  "service MailService" "rpc SendTemplateEmail" "repeated string recipients" \
  "map<string, string> template_data" "string provider_request_id" "string provider_message_id"

# --- agent conversation hosting contract ---
assert_present "-q" internal/rpcgen/message/message.proto db/migrations/003_agent_conversation_hosting.sql internal/repository/message_repository.go pkg/messaging/event.go -- \
  "message_origin" "agent_account_id" "trigger_server_msg_id" "agent_run_id" "allow_recursive_trigger"
assert_present "-q" api/message.api web/src/api/messages.ts web/src/models/messages.ts web/src/features/messages/MessagesPage.tsx -- \
  "messageOrigin" "agentAccountId" "triggerServerMsgId" "agentRunId" "allowRecursiveTrigger"
assert_present "-q" internal/logic/messagelogic.go internal/agentim internal/repository db/migrations/003_agent_conversation_hosting.sql -- \
  "MessageCreatedHook" "SetMessageCreatedHook" "message.created:" "NewConversationHostingService" "OnMessageCreated" \
  "TryStartAgentTrigger" "FinishAgentTrigger" "agent_conversation_hosting" "agent_trigger_idempotency" \
  "MessageServiceResponseWriter" "SendMessage\(ctx"
assert_present "-q" internal/agentim/hosting_test.go web/src/features/messages/MessagesPage.test.tsx -- \
  "TestConversationHostingWritesAIResponseThroughMessageServiceAndDeduplicates" "SetMessageCreatedHook" \
  "AI Agent" "messageOrigin: 'ai'" "deterministic-test"
forbid_match "agentim must write responses through MessageLogic/Message Service, not message repository or direct DB insert" \
  -n "CreateMessageIdempotent|insert into messages|insertMessage" internal/agentim --glob '*.go'

# --- generated rpc/proto artifacts present and correct ---
rpc_generated_servers=(
  "service/user/rpc/internal/server/userserver.go:UserServer"
  "service/auth/rpc/internal/server/auth_service_server.go:AuthServiceServer"
  "service/friends/rpc/internal/server/friendsserver.go:FriendsServer"
  "service/groups/rpc/internal/server/groupsserver.go:GroupsServer"
  "internal/rpcgen/message/internal/server/message_service_server.go:MessageServiceServer"
  "service/third/rpc/internal/server/mail_service_server.go:MailServiceServer"
)
for server_spec in "${rpc_generated_servers[@]}"; do
  file="${server_spec%%:*}"
  server_name="${server_spec##*:}"
  rg -q "Code generated by goctl. DO NOT EDIT." "$file"
  rg -q "type ${server_name} struct" "$file"
  rg -q "Unimplemented${server_name}" "$file"
done

rpc_generated_entrypoints=(
  "service/user/rpc/user.go:RegisterUserServer"
  "service/auth/rpc/auth.go:RegisterAuthServiceServer"
  "service/friends/rpc/friends.go:RegisterFriendsServer"
  "service/groups/rpc/groups.go:RegisterGroupsServer"
  "internal/rpcgen/message/message.go:RegisterMessageServiceServer"
  "service/third/rpc/third.go:RegisterMailServiceServer"
)
for entrypoint_spec in "${rpc_generated_entrypoints[@]}"; do
  rg -q "${entrypoint_spec##*:}" "${entrypoint_spec%%:*}"
done

api_register_patterns=(
  "service/user/api/user.go:handler.RegisterHandlers"
  "service/auth/api/auth.go:handler.RegisterHandlers"
  "service/friends/api/friends.go:handler.RegisterHandlers"
  "service/groups/api/groups.go:handler.RegisterHandlers"
  "service/agent/api/agent.go:handler.RegisterHandlers"
)
for entry_spec in "${api_register_patterns[@]}"; do
  rg -q "${entry_spec##*:}" "${entry_spec%%:*}"
done

rpc_generated_proto_files=(
  "service/user/rpc/user/user.pb.go"
  "service/user/rpc/user/user_grpc.pb.go"
  "service/auth/rpc/auth/auth.pb.go"
  "service/auth/rpc/auth/auth_grpc.pb.go"
  "internal/rpcgen/message/messagepb/message.pb.go"
  "internal/rpcgen/message/messagepb/message_grpc.pb.go"
  "service/third/rpc/mail/mail.pb.go"
  "service/third/rpc/mail/mail_grpc.pb.go"
)
for file in "${rpc_generated_proto_files[@]}"; do
  rg -q "Code generated by protoc-gen-go" "$file"
done

# --- gateway / websocket / transfer code surface ---
assert_present "-q" internal/gateway/contract.go tests/gateway_contract_test.go -- \
  "send_message" "pull_messages" "get_conversation_seqs" "mark_conversation_read" "heartbeat" \
  "SendMessage" "PullMessages" "GetConversationSeqs" "MarkConversationAsRead"
assert_present "-q" internal/gateway/ws internal/gateway/contract.go tests/websocket_gateway_test.go -- \
  "HandleWebSocket" "Validate\(rawToken\)" "Register" "CommandHeartbeat" "CommandSendMessage" \
  "CommandPullMessages" "CommandGetConversationSeqs" "CommandMarkConversationRead"
assert_present "-q" internal/gateway/ws/server.go tests/websocket_gateway_test.go -- \
  "RequestIDCamel" "Payload" "frontendErrorCode" "VALIDATION_ERROR" \
  "TestWebSocketGatewayReconnectSyncFlow" "TestWebSocketGatewayPullMessagesIsDuplicateSafe" \
  "TestWebSocketGatewayPullMessagesFromMissingSeq" "TestWebSocketGatewayInvalidCommandReturnsFrontendErrorEnvelope"
rg -q "TestWebSocketOriginPolicyUsesConfiguredExactOrigins" internal/gateway/ws/server_test.go

assert_present "-q" internal/transfer -- \
  "type MessageEvent struct" "type Envelope struct" "type EventConsumer interface" "type DeliveryDispatcher interface" \
  "type IdempotencyStore interface" "type RetryDecision struct" "type Worker struct" "func NewWorker" \
  "func \(w \*Worker\) Start" "func \(w \*Worker\) RunOnce" "func \(w \*Worker\) Stop" "NewInMemoryConsumer" "type NoopDispatcher struct"
assert_present "-q" internal/transfer/worker_test.go -- \
  "TestWorkerConsumesEventAndMarksSuccessful" "TestWorkerIdempotencySkipsDuplicateDispatch" \
  "TestWorkerRetryableFailureDoesNotMarkSuccessful" "TestWorkerContextCancellationStopsLoop"

rg -q "LoadMessageTransferConfig" pkg/config/config.go
rg -q "message-transfer" service/message-transfer/main.go etc/message-transfer.yaml
rg -q "ConsumerGroup|Consumer\.Group" etc/message-transfer.yaml pkg/config/config.go
rg -q "Topic|Consumer\.Topic" etc/message-transfer.yaml pkg/config/config.go
rg -q "WorkerID|Worker\.ID" etc/message-transfer.yaml pkg/config/config.go

assert_present "-q" internal/gateway/delivery internal/gateway/ws tests/websocket_gateway_test.go -- \
  "type Dispatcher interface" "DeliverToUser" "DeliverToConversation" "EventMessageReceived" "EventMessageDelivered" \
  "StatusOffline" "NewInMemoryDeliveryDispatcher" "PushToUser" "PushToConversation" "UserConnections"
assert_present "-q" internal/transfer/gateway -- \
  "type Dispatcher struct" "func NewDispatcher" "func \(d \*Dispatcher\) Dispatch" "EventMessageReceived" \
  "DeliverToConversation" "StatusOffline" "DispatchRetryable" "ErrNoRecipients"
assert_present "-q" internal/transfer/gateway/dispatcher_test.go -- \
  "TestDispatcherDeliversMessageAcceptedToGateway" "TestDispatcherOfflineRecipientsAreCompletedWithoutDeliveredUsers" \
  "TestDispatcherNoRecipientsFailsWithoutCallingGateway" "TestWorkerIdempotencySkipsDuplicateGatewayDispatch" \
  "TestWorkerRetryDecisionForGatewayError"

assert_present "-q" internal/repository internal/transfer db/migrations/001_init_postgres.sql -- \
  "DeliveryRecipientUserIDs" "type DeliveryAttemptRecorder interface" "MetricsDeliveryAttemptRecorder" "RecipientDeliveryResult" "message_outbox"
forbid_match "removed message V2 table still referenced: message_idempotency_keys" \
  -q "message_idempotency_keys" db/migrations/001_init_postgres.sql internal/repository --glob '*.go'

assert_present "-q" internal/gateway/ws internal/gateway/delivery pkg/presence tests/websocket_gateway_test.go -- \
  "WithPresenceStore" "WithPresenceTTL" "WithInstanceID" "RegisterConnection" "Heartbeat" "UnregisterConnection" \
  "ListUserConnections" "InstanceID" "StatusRouted" "type Route struct"
rg -q "Presence:" etc/gateway-ws.yaml

# --- presence / config code surface ---
assert_present "-q" pkg/config/config.go -- \
  "type RedisConfig" "type PresenceConfig" "ResolveRedisConfig" "ResolvePresenceConfig" "ResolvePresenceDriver"
assert_present "-q" pkg/presence -- \
  "type PresenceStore interface" "RegisterConnection" "Heartbeat" "UnregisterConnection" "ListUserConnections" \
  "IsUserOnline" "github.com/redis/go-redis/v9" ":user:" ":conn:"
rg -q "REDIS_ADDR is required.*skip|t\.Skip" pkg/presence/redis_integration_test.go

# --- messaging event schema & read receipt ---
assert_present "-q" pkg/messaging/event.go pkg/messaging/event_test.go -- \
  "type MessageEvent struct" "event_id" "event_type" "conversation_id" "server_msg_id" "sender_id" \
  "chat_type" "created_at" "payload" "message.accepted" "message.read"
assert_present "-q" internal/domain/readreceipt/read_receipt.go tests/read_receipts_test.go -- \
  "NormalizeMarkRead" "CanAdvanceReadSeq" "UnreadCount" "ErrReadSeqExceedsMax"

# --- JWT auth contract ---
for file in service/user/api/user.api service/friends/api/friends.api service/groups/api/groups.api api/message.api api/media.api service/agent/api/agent.api; do
  rg -q "jwt:\s+Auth" "$file"
done
for file in etc/auth-api.yaml etc/user-api.yaml etc/friends-api.yaml etc/groups-api.yaml etc/message-api.yaml etc/auth-rpc.yaml; do
  rg -q "AccessSecret" "$file"
  rg -q "AccessExpire" "$file"
done
rg -q "type JWTAuthConfig" pkg/config/config.go
rg -q "AccessSecret" pkg/config/config.go service/auth/rpc/internal/config/config.go
rg -q "AccessExpire" pkg/config/config.go service/auth/rpc/internal/config/config.go
rg -q "user_id" pkg/ctxuser/user.go
rg -q "ctxuser\.UserID" internal/logic/message
rg -q "sender_id must match authenticated user" internal/logic/message/send_message_logic.go
assert_present "-q" tests -- \
  "bearerTokenForUser" "legacy X-User-Id rejection" "invalid token status" "message sender did not use token user"
rg -q "ExistsByIdentifier" internal/auth
rg -q "CreateUser" internal/auth
rg -q "PasswordHash" internal/auth/model/credential.go
rg -q "Salt" internal/auth/model/credential.go

# --- social MVP authorization contract ---
assert_present "-q" internal -- \
  "CodeForbidden" "sender is not a group member" "group membership validator is not configured" \
  "group owner cannot leave as the only active member"
assert_present "-q" tests -- \
  "TestMessageGroupSendRequiresActiveMembership" "client-group-outsider" "client-group-left"

# --- account/persistence schema & storage wiring ---
assert_present "-qF" db/migrations/001_init_postgres.sql -- \
  "create table if not exists accounts" "create table if not exists profiles" \
  "account_id text primary key" "account_type smallint not null default 1"
rg -q "NewGroupsRepositoryForStorage" service/message-api/main.go service/gateway-ws/main.go internal/rpcgen/message/internal/svc/service_context.go
rg -q "NewMessageLogicWithMediaValidator" internal/rpcgen/message/internal/svc/service_context.go
rg -q "NewMessageRepositoryForStorage" internal/rpcgen/message/internal/svc/service_context.go
assert_present "-q" db/migrations/001_init_postgres.sql -- \
  "accounts" "profiles" "auth_credentials" "friendships" "groups" "group_members" \
  "media_objects" "messages" "conversation_threads" "user_conversation_states" "message_outbox"
rg -q "StorageDriver" pkg/config/config.go etc/*.yaml
rg -q "ObjectStorageConfig" pkg/config/config.go
rg -q "NewStore" pkg/objectstorage/factory.go
rg -q "PresignPut" pkg/objectstorage/store.go pkg/objectstorage/minio.go
rg -q "NewMediaRepositoryForStorage" internal/repository/postgres_common.go service/user/api/user.go service/message-api/main.go service/gateway-ws/main.go
rg -q "ValidateMessageMedia" internal/logic/messagelogic.go
rg -q "media_objects" db/migrations/001_init_postgres.sql
rg -q "NewPostgresRepository" internal/repository/postgres_user_friends.go internal/auth/repository/postgres.go
rg -q "NewPostgresGroupsRepository" internal/repository/postgres_groups.go
rg -q "NewPostgresMessageRepository" internal/repository/postgres_message.go
rg -q "docker compose" scripts/migrate-postgres.sh

# --- outbox schema/code/publisher ---
assert_present "-q" db/migrations/001_init_postgres.sql -- \
  "event_id" "event_type" "aggregate_type" "aggregate_id" "conversation_id" "message_id" \
  "payload jsonb" "attempt_count" "next_attempt_at" "locked_by" "locked_until" "published_at"
assert_present "-q" internal/repository/message_outbox_repository.go internal/repository/postgres_outbox.go internal/repository/postgres_message.go internal/repository/message_memory.go tests/message_service_test.go tests/postgres_persistence_integration_test.go -- \
  "type OutboxRepository interface" "OutboxEventTypeMessageCreated" "PollPending" "MarkPublished" "MarkFailed" \
  "messageCreatedOutboxPayload" "insertMessageOutboxEvent" "SKIP LOCKED"
assert_present "-q" internal/outboxpublisher -- \
  "package outboxpublisher" "MessageEventFromOutbox" "messaging.MessageEvent" "EventTypeMessageAccepted"
rg -q "TestMessageEventFromOutboxBuildsAcceptedEvent" internal/outboxpublisher/message_event_test.go
rg -q "TestMessageEventFromOutboxIncludesAISenderInSingleChatReceiverIDs" internal/outboxpublisher/message_event_test.go
rg -q "TestMessageEventFromOutboxRejectsMalformedPayload" internal/outboxpublisher/message_event_test.go

# --- observability code surface ---
assert_present "-q" pkg/health pkg/observability -- \
  "StatusReady" "ReadinessHandler" "MetricMessageSends" "MetricDeliveryAttempts" "MetricTransferEvents" \
  "MetricWebSocketCurrent" "TraceMiddleware" "HeaderTraceID"
# user/auth/friends/groups api migrated to go-zero native Telemetry (yaml ServiceConf.Telemetry);
# agent-api still uses observability.TraceMiddlewareFunc.
for api_main in service/agent/api/agent.go; do
  rg -q "TraceMiddlewareFunc" "$api_main"
done
assert_present "-q" internal/handler/gozero_routes.go service/user/api/user.go service/auth/api/auth.go service/friends/api/friends.go service/groups/api/groups.go service/agent/api/agent.go service/gateway-ws/main.go service/message-transfer/main.go -- \
  "/readyz" "/metrics" "ReadinessHandler" "MetricsHandler"
assert_present "-q" internal/logic/messagelogic.go internal/gateway/ws internal/transfer/worker.go -- \
  "RecordMessageSend" "RecordDeliveryAttempt" "RecordTransferEvent" "SetWebSocketConnections" "RecordWebSocketConnectionEvent"
rg -q "Observability:" etc/message-transfer.yaml
rg -q "MESSAGE_TRANSFER_OBSERVABILITY_PORT" .env.example
rg -q "agents_im_message_sends_total" pkg/observability/metrics.go
rg -q "trace_id" internal/gateway/ws/server.go
assert_present "-q" pkg/llmobs internal/agenteval internal/agentim internal/agentruntime/eino pkg/config .env.example -- \
  "RuntimeModeAIHostingAutoReply" "NewEinoCallbackHandler" "ErrLangfuseConfigMissing" "langfuseIngestionPath" \
  "LLMObservability" "LANGFUSE_PUBLIC_KEY" "PythonGoPerformanceCaseID" "ai_hosting.python_go_performance.v1"
rg -q "LLM_OBSERVABILITY_BACKEND=noop" .env.example
