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
assert_present "-q" service/msg/api/msg.api -- \
  "post /messages" "get /conversations/:conversation_id/messages" \
  "get /conversations/seqs" "post /conversations/:conversation_id/read" \
  "get /conversations/:conversation_id/ai-hosting" "put /conversations/:conversation_id/ai-hosting" \
  "post /api/feedback"

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
assert_present "-q" service/msg/rpc/msg.proto -- \
  "service Msg" "rpc SendMessage" "rpc PullMessages" "rpc GetConversationsSeqState" \
  "rpc MarkConversationAsRead" "message Message" "message ConversationSeqState"
assert_present "-q" service/third/rpc/mail.proto -- \
  "service Mail" "rpc SendTemplateEmail" "repeated string recipients" \
  "map<string, string> template_data" "string provider_request_id" "string provider_message_id"

# --- agent conversation hosting contract ---
assert_present "-q" service/msg/rpc/msg.proto db/migrations/003_agent_conversation_hosting.sql internal/repository/message_repository.go pkg/messaging/event.go -- \
  "message_origin" "agent_account_id" "trigger_server_msg_id" "agent_run_id" "allow_recursive_trigger"
assert_present "-q" service/msg/api/msg.api web/src/api/messages.ts web/src/models/messages.ts web/src/features/messages/MessagesPage.tsx -- \
  "messageOrigin" "agentAccountId" "triggerServerMsgId" "agentRunId" "allowRecursiveTrigger"
# AI 托管编排已迁出至属主 service/agent/rpc/internal/{orchestrator,aihosting}（#340）。
assert_present "-q" internal/logic/messagelogic.go service/agent/rpc/internal/orchestrator service/agent/rpc/internal/aihosting internal/repository db/migrations/003_agent_conversation_hosting.sql -- \
  "MessageCreatedHook" "SetMessageCreatedHook" "message.created:" "NewConversationHostingService" "OnMessageCreated" \
  "TryStartAgentTrigger" "FinishAgentTrigger" "agent_conversation_hosting" "agent_trigger_idempotency" \
  "MessageServiceResponseWriter" "SendMessage\(ctx"
assert_present "-q" service/agent/rpc/internal/orchestrator/hosting_test.go web/src/features/messages/MessagesPage.test.tsx -- \
  "TestConversationHostingWritesAIResponseThroughMessageServiceAndDeduplicates" "SetMessageCreatedHook" \
  "AI Agent" "messageOrigin: 'ai'" "deterministic-test"
forbid_match "agent orchestrator must write responses through MessageLogic/Message Service, not message repository or direct DB insert" \
  -n "CreateMessageIdempotent|insert into messages|insertMessage" service/agent/rpc/internal/orchestrator --glob '*.go'

# --- generated rpc/proto artifacts present and correct ---
rpc_generated_servers=(
  "service/user/rpc/internal/server/userserver.go:UserServer"
  "service/auth/rpc/internal/server/authserver.go:AuthServer"
  "service/friends/rpc/internal/server/friendsserver.go:FriendsServer"
  "service/groups/rpc/internal/server/groupsserver.go:GroupsServer"
  "service/admin/rpc/internal/server/adminserver.go:AdminServer"
  "service/msg/rpc/internal/server/msgserver.go:MsgServer"
  "service/third/rpc/internal/server/mailserver.go:MailServer"
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
  "service/auth/rpc/auth.go:RegisterAuthServer"
  "service/friends/rpc/friends.go:RegisterFriendsServer"
  "service/groups/rpc/groups.go:RegisterGroupsServer"
  "service/admin/rpc/admin.go:RegisterAdminServer"
  "service/msg/rpc/msg.go:RegisterMsgServer"
  "service/third/rpc/third.go:RegisterMailServer"
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
  "service/admin/api/admin.go:handler.RegisterHandlers"
  "service/msg/api/msg.go:handler.RegisterHandlers"
)
for entry_spec in "${api_register_patterns[@]}"; do
  rg -q "${entry_spec##*:}" "${entry_spec%%:*}"
done

rpc_generated_proto_files=(
  "service/user/rpc/user/user.pb.go"
  "service/user/rpc/user/user_grpc.pb.go"
  "service/auth/rpc/auth/auth.pb.go"
  "service/auth/rpc/auth/auth_grpc.pb.go"
  "service/msg/rpc/msg/msg.pb.go"
  "service/msg/rpc/msg/msg_grpc.pb.go"
  "service/third/rpc/mail/mail.pb.go"
  "service/third/rpc/mail/mail_grpc.pb.go"
  "service/admin/rpc/admin/admin.pb.go"
  "service/admin/rpc/admin/admin_grpc.pb.go"
)
for file in "${rpc_generated_proto_files[@]}"; do
  rg -q "Code generated by protoc-gen-go" "$file"
done

# --- gateway / websocket / transfer code surface ---
assert_present "-q" pkg/gateway/contract.go tests/gateway_contract_test.go -- \
  "send_message" "pull_messages" "get_conversation_seqs" "mark_conversation_read" "heartbeat" \
  "SendMessage" "PullMessages" "GetConversationSeqs" "MarkConversationAsRead"
assert_present "-q" service/msggateway/internal/ws pkg/gateway/contract.go -- \
  "HandleWebSocket" "Validate\(rawToken\)" "Register" "CommandHeartbeat" "CommandSendMessage" \
  "CommandPullMessages" "CommandGetConversationSeqs" "CommandMarkConversationRead"
assert_present "-q" service/msggateway/internal/ws/server.go service/msggateway/internal/ws/gateway_ws_test.go -- \
  "RequestIDCamel" "Payload" "frontendErrorCode" "VALIDATION_ERROR" \
  "TestWebSocketGatewayReconnectSyncFlow" "TestWebSocketGatewayPullMessagesIsDuplicateSafe" \
  "TestWebSocketGatewayPullMessagesFromMissingSeq" "TestWebSocketGatewayInvalidCommandReturnsFrontendErrorEnvelope"
rg -q "TestWebSocketOriginPolicyUsesConfiguredExactOrigins" service/msggateway/internal/ws/server_test.go

# push 服务（03 §9 C2-C3）：在线广播 + 二段式离线。gateway 投递自 msgtransfer 拆出。
assert_present "-q" service/push/internal/gateway service/push/internal/pusher -- \
  "type Broadcaster struct" "func \(b \*Broadcaster\) Broadcast" "type OnlineHandler struct" \
  "type OfflineHandler struct" "type OfflinePusher interface" "AuditOfflinePusher" "TopicToOfflinePush"
assert_present "-q" service/push/internal/gateway/broadcaster_test.go service/push/internal/pusher/online_test.go service/push/internal/pusher/offline_test.go -- \
  "TestBroadcastAggregatesDeliveredAcrossGateways" "TestBroadcastErrorsWhenAnyGatewayFails" \
  "TestOnlineHandlerProducesOfflineForMissedRecipients" "TestOnlineHandlerBroadcastErrorRetriesBatch" \
  "TestOfflineHandlerPushesRecipients"
rg -q "GroupPushOnline|GroupPushOffline" pkg/messaging/topics.go
rg -q "LoadPushConfig" pkg/config/push.go

rg -q "LoadMessageTransferConfig" pkg/config/config.go
rg -q "msgtransfer" service/msgtransfer/msgtransfer.go etc/msgtransfer.yaml
rg -q "ConsumerGroup|Consumer\.Group" etc/msgtransfer.yaml pkg/config/config.go
rg -q "Topic|Consumer\.Topic" etc/msgtransfer.yaml pkg/config/config.go
rg -q "WorkerID|Worker\.ID" etc/msgtransfer.yaml pkg/config/config.go

assert_present "-q" pkg/gateway/delivery service/msggateway/internal/ws -- \
  "type Dispatcher interface" "DeliverToUser" "DeliverToConversation" "EventMessageReceived" "EventMessageDelivered" \
  "StatusOffline" "NewInMemoryDeliveryDispatcher" "PushToUser" "PushToConversation" "UserConnections"
# 下行推送 gRPC 面（03 §6.2）：msggateway 暴露 GatewayService，push 经 headless DNS 广播。
assert_present "-q" service/msggateway/gateway.proto service/msggateway/internal/grpcserver -- \
  "service GatewayService" "BatchPushOneMsg" "type Server struct" "PushToConversation" "delivery.Event"
rg -q "GatewayGRPC" pkg/config/config.go etc/msggateway.yaml

assert_present "-q" internal/repository db/migrations/001_init_postgres.sql -- \
  "DeliveryRecipientUserIDs" "message_outbox"
forbid_match "removed message V2 table still referenced: message_idempotency_keys" \
  -q "message_idempotency_keys" db/migrations/001_init_postgres.sql internal/repository --glob '*.go'

assert_present "-q" service/msggateway/internal/ws pkg/gateway/delivery pkg/presence -- \
  "WithPresenceStore" "WithPresenceTTL" "WithInstanceID" "RegisterConnection" "Heartbeat" "UnregisterConnection" \
  "ListUserConnections" "InstanceID" "StatusRouted" "type Route struct"
rg -q "Presence:" etc/msggateway.yaml

# --- presence / config code surface ---
assert_present "-q" pkg/config/config.go -- \
  "type RedisConfig" "type PresenceConfig" "ResolveRedisConfig" "ResolvePresenceConfig" "ResolvePresenceDriver"
assert_present "-q" pkg/presence -- \
  "type PresenceStore interface" "RegisterConnection" "Heartbeat" "UnregisterConnection" "ListUserConnections" \
  "IsUserOnline" "github.com/redis/go-redis/v9" ":user:" ":conn:"
rg -q "REDIS_ADDR is required.*skip|t\.Skip" pkg/presence/redis_integration_test.go

# --- messaging event schema ---
assert_present "-q" pkg/messaging/event.go pkg/messaging/event_test.go -- \
  "type MessageEvent struct" "event_id" "event_type" "conversation_id" "server_msg_id" "sender_id" \
  "chat_type" "created_at" "payload" "message.accepted" "message.read"

# --- JWT auth contract ---
for file in service/user/api/user.api service/friends/api/friends.api service/groups/api/groups.api service/msg/api/msg.api service/media/api/media.api service/agent/api/agent.api; do
  rg -q "jwt:\s+Auth" "$file"
done
for file in etc/auth-api.yaml etc/user-api.yaml etc/friends-api.yaml etc/groups-api.yaml etc/msg-api.yaml etc/auth-rpc.yaml; do
  rg -q "AccessSecret" "$file"
  rg -q "AccessExpire" "$file"
done
rg -q "type JWTAuthConfig" pkg/config/config.go
rg -q "AccessSecret" pkg/config/config.go service/auth/rpc/internal/config/config.go
rg -q "AccessExpire" pkg/config/config.go service/auth/rpc/internal/config/config.go
rg -q "user_id" pkg/ctxuser/user.go
rg -q "ctxuser\.UserID" service/msg/api/internal/logic/msg
rg -q "sender_id must match authenticated user" service/msg/api/internal/logic/msg/send_message_logic.go
assert_present "-q" tests -- \
  "bearerTokenForUser"
assert_present "-q" service/msggateway/internal/ws -- \
  "invalid token status"
assert_present "-q" service/msg/api/internal/logic/msg -- \
  "message sender did not use token user" "TestSendMessageUsesJWTUser" "TestSendMessageRejectsSenderMismatch"
# auth-rpc 注册/登录直调下游 user-rpc（去 adapter，#563）；凭据数据层落 goctl model。
rg -q "ExistsByIdentifier" service/auth/rpc/internal/logic
rg -q "CreateUser" service/auth/rpc/internal/logic
rg -q "PasswordHash" service/auth/rpc/internal/model/auth_credentials_model.go

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
rg -q "NewGroupsRepositoryForStorage" service/msg/rpc/internal/svc/servicecontext.go
rg -q "NewMessageLogicWithMediaValidator" service/agent/rpc/internal/aihosting/service_context.go
# AI 托管运行时的 message 历史读数据层随 agent 域归位（#340）。
rg -q "NewMessageRepositoryForStorage" service/agent/rpc/internal/svc/servicecontext.go
assert_present "-q" db/migrations/001_init_postgres.sql -- \
  "accounts" "profiles" "auth_credentials" "friendships" "groups" "group_members" \
  "media_objects" "messages" "conversation_threads" "user_conversation_states" "message_outbox"
rg -q "StorageDriver" pkg/config/config.go etc/*.yaml
rg -q "ObjectStorageConfig" pkg/config/config.go
rg -q "NewStore" pkg/objectstorage/factory.go
rg -q "PresignPut" pkg/objectstorage/store.go pkg/objectstorage/s3.go
rg -q "NewMediaRepositoryForStorage" internal/repository/postgres_common.go service/user/api/user.go service/msg/rpc/internal/svc/servicecontext.go
rg -q "ValidateMessageMedia" internal/logic/messagelogic.go
rg -q "media_objects" db/migrations/001_init_postgres.sql
rg -q "NewPostgresRepository" internal/repository/postgres_common.go
rg -q "NewPostgresGroupsRepository" internal/repository/postgres_groups.go
rg -q "NewPostgresMessageRepository" internal/repository/postgres_message.go
rg -q "docker compose" scripts/migrate-postgres.sh

# --- outbox 退役（03 §9 B3b）：写链路唯一原语 = Kafka，message_outbox 表保留 90 天观察 ---
assert_present "-q" db/migrations/001_init_postgres.sql -- "message_outbox"
forbid_match "retired outbox write path resurrected (03 §9 B3b)" \
  -q "insertMessageOutboxEvent|OutboxRepository|outboxpublisher" internal service --glob '*.go'

# --- observability code surface ---
assert_present "-q" pkg/health pkg/observability -- \
  "StatusReady" "ReadinessHandler" "MetricMessageSends" "MetricDeliveryAttempts" "MetricTransferEvents" \
  "MetricWebSocketCurrent" "TraceMiddleware" "HeaderTraceID"
# user/auth/friends/groups api migrated to go-zero native Telemetry (yaml ServiceConf.Telemetry);
# agent-api still uses observability.TraceMiddlewareFunc.
for api_main in service/agent/api/agent.go; do
  rg -q "TraceMiddlewareFunc" "$api_main"
done
assert_present "-q" service/user/api/user.go service/auth/api/auth.go service/friends/api/friends.go service/groups/api/groups.go service/agent/api/agent.go service/msg/api/msg.go service/msggateway/msggateway.go service/msgtransfer/msgtransfer.go service/push/push.go -- \
  "/readyz" "/metrics" "ReadinessHandler" "MetricsHandler"
assert_present "-q" internal/logic/messagelogic.go service/msggateway/internal/ws -- \
  "RecordMessageSend" "RecordDeliveryAttempt" "SetWebSocketConnections" "RecordWebSocketConnectionEvent"
# push 在线/离线投递指标（03 §9 C2-C3）。
assert_present "-q" service/push/internal/pusher pkg/observability/metrics.go -- \
  "RecordPushOnline" "RecordPushOffline" "agents_im_push_online_total" "agents_im_push_offline_total"
rg -q "Observability:" etc/msgtransfer.yaml etc/push.yaml
rg -q "MESSAGE_TRANSFER_OBSERVABILITY_PORT" .env.example
rg -q "agents_im_message_sends_total" pkg/observability/metrics.go
rg -q "trace_id" service/msggateway/internal/ws/server.go
assert_present "-q" pkg/llmobs service/agent/rpc/internal/orchestrator service/agent/rpc/internal/runtime/eino pkg/config .env.example -- \
  "RuntimeModeAIHostingAutoReply" "NewEinoCallbackHandler" "ErrLangfuseConfigMissing" "langfuseIngestionPath" \
  "LLMObservability" "LANGFUSE_PUBLIC_KEY"
rg -q "LLM_OBSERVABILITY_BACKEND=noop" .env.example
