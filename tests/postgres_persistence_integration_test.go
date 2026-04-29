//go:build integration

package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/wujunhui99/agents_im/internal/apperror"
	authmodel "github.com/wujunhui99/agents_im/internal/auth/model"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/domain/agentaudit"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestPostgresUserAuthFriendsGroupsRepositories(t *testing.T) {
	ctx := context.Background()
	dsn := integrationPostgresDSN(t)
	migrateAndCleanPostgres(t, ctx, dsn)

	users, err := repository.NewPostgresRepository(dsn)
	if err != nil {
		t.Fatal(err)
	}
	authCredentials, err := authrepo.NewPostgresRepository(dsn)
	if err != nil {
		t.Fatal(err)
	}
	groups, err := repository.NewPostgresGroupsRepository(dsn)
	if err != nil {
		t.Fatal(err)
	}

	alice, err := users.Create(ctx, model.User{
		Identifier:  "pg_alice",
		DisplayName: "Alice",
		Name:        "Alice",
		Gender:      "unknown",
	})
	if err != nil {
		t.Fatal(err)
	}
	bob, err := users.Create(ctx, model.User{
		Identifier:  "pg_bob",
		DisplayName: "Bob",
		Name:        "Bob",
		Gender:      "unknown",
	})
	if err != nil {
		t.Fatal(err)
	}

	if exists, err := users.ExistsByIdentifier(ctx, "pg_alice"); err != nil || !exists {
		t.Fatalf("alice should exist, exists=%v err=%v", exists, err)
	}

	credential, err := authCredentials.Create(ctx, authmodel.Credential{
		Identifier:   alice.Identifier,
		UserID:       alice.UserID,
		PasswordHash: "hash-for-integration",
		Salt:         "salt-for-integration",
		HashVersion:  "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	loadedCredential, err := authCredentials.GetByIdentifier(ctx, alice.Identifier)
	if err != nil {
		t.Fatal(err)
	}
	if loadedCredential.UserID != credential.UserID {
		t.Fatalf("loaded credential user id mismatch: got %q want %q", loadedCredential.UserID, credential.UserID)
	}

	friendship, created, err := users.AddFriend(ctx, alice.UserID, bob.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if !created || friendship.Status != model.FriendshipStatusActive {
		t.Fatalf("friendship should be newly active: created=%v status=%q", created, friendship.Status)
	}
	bobFriends, err := users.ListFriends(ctx, bob.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bobFriends) != 1 || bobFriends[0].FriendID != alice.UserID {
		t.Fatalf("reciprocal friendship missing: %+v", bobFriends)
	}

	group, creator, err := groups.CreateGroup(ctx, model.Group{Name: "PG Group"}, alice.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if creator.UserID != alice.UserID {
		t.Fatalf("creator member mismatch: %+v", creator)
	}
	member, alreadyMember, err := groups.AddMember(ctx, group.GroupID, bob.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if alreadyMember || member.State != model.MemberStateActive {
		t.Fatalf("bob should be newly active member: already=%v member=%+v", alreadyMember, member)
	}
	members, err := groups.ListActiveMembers(ctx, group.GroupID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 active members, got %d", len(members))
	}
	left, err := groups.LeaveGroup(ctx, group.GroupID, bob.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if left.State != model.MemberStateLeft || left.LeftAt.IsZero() {
		t.Fatalf("leave group did not mark left state: %+v", left)
	}
}

func TestPostgresMessageRepositoryIdempotencyAndReadState(t *testing.T) {
	ctx := context.Background()
	dsn := integrationPostgresDSN(t)
	migrateAndCleanPostgres(t, ctx, dsn)

	messages, err := repository.NewPostgresMessageRepository(dsn)
	if err != nil {
		t.Fatal(err)
	}

	input := repository.CreateMessageInput{
		SenderID:           "usr_pg_sender",
		ReceiverID:         "usr_pg_receiver",
		ChatType:           repository.ChatTypeSingle,
		ClientMsgID:        "client-msg-1",
		ContentType:        repository.ContentTypeText,
		Content:            "hello postgres",
		ParticipantUserIDs: []string{"usr_pg_sender", "usr_pg_receiver"},
	}
	first, deduplicated, err := messages.CreateMessageIdempotent(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if deduplicated || first.Seq != 1 {
		t.Fatalf("first send should allocate seq 1 without dedupe: message=%+v dedupe=%v", first, deduplicated)
	}
	deliveryAttempts, err := messages.ListDeliveryAttemptsByMessage(ctx, first.ServerMsgID)
	if err != nil {
		t.Fatal(err)
	}
	if len(deliveryAttempts) != 1 ||
		deliveryAttempts[0].RecipientUserID != input.ReceiverID ||
		deliveryAttempts[0].Status != repository.DeliveryStatusAccepted {
		t.Fatalf("accepted delivery attempt mismatch: %+v", deliveryAttempts)
	}

	again, deduplicated, err := messages.CreateMessageIdempotent(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if !deduplicated || again.ServerMsgID != first.ServerMsgID || again.Seq != first.Seq {
		t.Fatalf("same payload should deduplicate to original: first=%+v again=%+v dedupe=%v", first, again, deduplicated)
	}

	conflicting := input
	conflicting.Content = "different payload"
	if _, _, err := messages.CreateMessageIdempotent(ctx, conflicting); err == nil {
		t.Fatal("expected idempotency conflict")
	} else if appErr := apperror.From(err); appErr.Code != apperror.CodeAlreadyExists {
		t.Fatalf("expected already exists conflict, got %v", err)
	}

	conversationID := repository.SingleConversationID(input.SenderID, input.ReceiverID)
	pulled, isEnd, nextSeq, err := messages.GetMessages(ctx, conversationID, 1, 0, 10, "asc")
	if err != nil {
		t.Fatal(err)
	}
	if !isEnd || nextSeq != 2 || len(pulled) != 1 || pulled[0].Content != input.Content {
		t.Fatalf("unexpected pull result messages=%+v isEnd=%v nextSeq=%d", pulled, isEnd, nextSeq)
	}

	receiverStates, err := messages.GetConversationSeqStates(ctx, input.ReceiverID, []string{conversationID})
	if err != nil {
		t.Fatal(err)
	}
	if len(receiverStates) != 1 || receiverStates[0].UnreadCount != 1 || receiverStates[0].HasReadSeq != 0 {
		t.Fatalf("receiver unread state mismatch: %+v", receiverStates)
	}
	senderStates, err := messages.GetConversationSeqStates(ctx, input.SenderID, []string{conversationID})
	if err != nil {
		t.Fatal(err)
	}
	if len(senderStates) != 1 || senderStates[0].UnreadCount != 0 || senderStates[0].HasReadSeq != 1 {
		t.Fatalf("sender read state mismatch: %+v", senderStates)
	}

	updatedState, updated, err := messages.SetUserHasReadSeqMax(ctx, input.ReceiverID, conversationID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !updated || updatedState.HasReadSeq != 1 || updatedState.UnreadCount != 0 {
		t.Fatalf("receiver mark read mismatch: state=%+v updated=%v", updatedState, updated)
	}
	staleState, updated, err := messages.SetUserHasReadSeqMax(ctx, input.ReceiverID, conversationID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if updated || staleState.HasReadSeq != 1 {
		t.Fatalf("stale read seq should not move state backward: state=%+v updated=%v", staleState, updated)
	}
	if _, _, err := messages.SetUserHasReadSeqMax(ctx, input.ReceiverID, conversationID, 2); err == nil {
		t.Fatal("expected read seq beyond max to fail")
	}

	outboxEvents, err := messages.PollPending(ctx, "pg-worker-1", 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(outboxEvents) != 1 {
		t.Fatalf("expected one outbox event, got %d: %+v", len(outboxEvents), outboxEvents)
	}
	outboxEvent := outboxEvents[0]
	if outboxEvent.EventType != repository.OutboxEventTypeMessageCreated ||
		outboxEvent.AggregateType != repository.OutboxAggregateTypeMessage ||
		outboxEvent.AggregateID != first.ServerMsgID ||
		outboxEvent.ServerMsgID != first.ServerMsgID ||
		outboxEvent.ConversationID != conversationID ||
		outboxEvent.Seq != first.Seq {
		t.Fatalf("unexpected outbox event metadata: %+v", outboxEvent)
	}
	var payload repository.MessageCreatedOutboxPayload
	if err := json.Unmarshal(outboxEvent.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Message.ServerMsgID != first.ServerMsgID || payload.Message.Content != first.Content {
		t.Fatalf("outbox payload message mismatch: %+v", payload.Message)
	}

	if err := messages.MarkFailed(ctx, outboxEvent.EventID, "pg-worker-1", repository.OutboxFailure{
		NextAttemptAt: time.Now().Add(-time.Second),
		LastError:     "retryable test failure",
	}); err != nil {
		t.Fatal(err)
	}
	retriedOutboxEvents, err := messages.PollPending(ctx, "pg-worker-2", 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(retriedOutboxEvents) != 1 || retriedOutboxEvents[0].AttemptCount != 1 {
		t.Fatalf("expected retried outbox event with attempt count 1: %+v", retriedOutboxEvents)
	}
	if err := messages.MarkPublished(ctx, retriedOutboxEvents[0].EventID, "pg-worker-2"); err != nil {
		t.Fatal(err)
	}
	deliveryAttempts, err = messages.ListDeliveryAttemptsByMessage(ctx, first.ServerMsgID)
	if err != nil {
		t.Fatal(err)
	}
	if len(deliveryAttempts) != 1 || deliveryAttempts[0].Status != repository.DeliveryStatusPublished {
		t.Fatalf("published delivery attempt mismatch: %+v", deliveryAttempts)
	}
	if err := messages.RecordDeliveryAttemptResult(ctx, repository.RecordDeliveryAttemptInput{
		ServerMsgID:     first.ServerMsgID,
		ConversationID:  conversationID,
		RecipientUserID: input.ReceiverID,
		Status:          repository.DeliveryStatusDelivered,
		AttemptCount:    1,
	}); err != nil {
		t.Fatal(err)
	}
	deliveryAttempts, err = messages.ListDeliveryAttemptsByMessage(ctx, first.ServerMsgID)
	if err != nil {
		t.Fatal(err)
	}
	if len(deliveryAttempts) != 1 ||
		deliveryAttempts[0].Status != repository.DeliveryStatusDelivered ||
		deliveryAttempts[0].AttemptCount != 1 {
		t.Fatalf("delivered delivery attempt mismatch: %+v", deliveryAttempts)
	}
	remainingOutboxEvents, err := messages.PollPending(ctx, "pg-worker-3", 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(remainingOutboxEvents) != 0 {
		t.Fatalf("published outbox event should not be pending: %+v", remainingOutboxEvents)
	}
}

func TestPostgresAgentAuditRepositoryAppendOnlyAndRedaction(t *testing.T) {
	ctx := context.Background()
	dsn := integrationPostgresDSN(t)
	migrateAndCleanPostgres(t, ctx, dsn)

	audit, err := repository.NewPostgresAgentAuditRepository(dsn)
	if err != nil {
		t.Fatal(err)
	}

	run, err := audit.CreateAgentRun(ctx, agentaudit.CreateRunInput{
		RunID:          "run_pg_audit_1",
		AgentID:        "agent_pg_1",
		ConversationID: "single:usr_pg_1:agent_pg_1",
		Status:         agentaudit.StatusSucceeded,
		InputSummary: agentaudit.Summary{
			"prompt":       "hello",
			"access_token": "must-not-leak",
		},
		TraceID:   "trace_pg_1",
		RequestID: "req_pg_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.InputSummary["access_token"] != agentaudit.RedactedValue {
		t.Fatalf("postgres run summary did not redact token: %+v", run.InputSummary)
	}

	if _, err := audit.CreateAgentToolCall(ctx, agentaudit.CreateToolCallInput{
		ToolCallID: "tool_pg_1",
		RunID:      run.RunID,
		AgentID:    run.AgentID,
		ToolName:   "im.get_conversation_context",
		Status:     agentaudit.StatusSucceeded,
	}); err != nil {
		t.Fatal(err)
	}
	toolCalls, err := audit.ListAgentToolCallsByRunID(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(toolCalls) != 1 || toolCalls[0].ToolCallID != "tool_pg_1" {
		t.Fatalf("postgres tool calls mismatch: %+v", toolCalls)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `update agent_runs set status = 'failed' where run_id = $1`, run.RunID); err == nil {
		t.Fatal("expected append-only trigger to reject update")
	}
}

func integrationPostgresDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("AGENTS_IM_POSTGRES_DSN")
	}
	if dsn == "" {
		t.Skip("DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required for PostgreSQL integration tests")
	}
	return dsn
}

func migrateAndCleanPostgres(t *testing.T, ctx context.Context, dsn string) {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	root := findRepoRoot(t)
	migrations, err := filepath.Glob(filepath.Join(root, "db", "migrations", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(migrations)
	for _, migrationPath := range migrations {
		migration, err := os.ReadFile(migrationPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `
truncate table
  agent_python_execs,
  agent_file_reads,
  agent_tool_calls,
  agent_runs,
  delivery_attempts,
  message_outbox,
  message_idempotency_keys,
  user_conversation_states,
  messages,
  conversation_threads,
  group_members,
  groups,
  friendships,
  auth_credentials,
  users
cascade
`); err != nil {
		t.Fatal(err)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
