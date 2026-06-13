//go:build integration

package tests

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/wujunhui99/agents_im/common/share/agentaudit"
	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	authmodel "github.com/wujunhui99/agents_im/service/auth/core/model"
	authrepo "github.com/wujunhui99/agents_im/service/auth/core/repository"
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
	if alice.AccountType != model.AccountTypeUser {
		t.Fatalf("default postgres account_type = %q, want %q", alice.AccountType, model.AccountTypeUser)
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
	pgAgent, err := users.Create(ctx, model.User{
		Identifier:  "pg_agent",
		DisplayName: "PG Agent",
		Name:        "PG Agent",
		Gender:      "unknown",
		AccountType: model.AccountTypeAgent,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pgAgent.AccountType != model.AccountTypeAgent {
		t.Fatalf("explicit postgres agent account_type = %q, want %q", pgAgent.AccountType, model.AccountTypeAgent)
	}
	pgAdmin, err := users.Create(ctx, model.User{
		Identifier:  "pg_admin",
		DisplayName: "PG Admin",
		Name:        "PG Admin",
		Gender:      "unknown",
		AccountType: model.AccountTypeAdmin,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pgAdmin.AccountType != model.AccountTypeAdmin {
		t.Fatalf("explicit postgres admin account_type = %q, want %q", pgAdmin.AccountType, model.AccountTypeAdmin)
	}
	if _, err := users.Create(ctx, model.User{
		Identifier:  "pg_invalid_account_type",
		DisplayName: "PG Invalid",
		Name:        "PG Invalid",
		Gender:      "unknown",
		AccountType: model.AccountType("root"),
	}); err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("postgres invalid account_type error = %v, want INVALID_ARGUMENT", err)
	}

	if exists, err := users.ExistsByIdentifier(ctx, "pg_alice"); err != nil || !exists {
		t.Fatalf("alice should exist, exists=%v err=%v", exists, err)
	}

	credential, err := authCredentials.Create(ctx, authmodel.Credential{
		Identifier:   alice.Identifier,
		UserID:       alice.UserID,
		PasswordHash: "hash-for-integration",
		HashVersion:  authmodel.PasswordHashVersionBcrypt,
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
	if !created || friendship.Status != model.FriendshipStatusPending {
		t.Fatalf("friendship should be newly pending: created=%v status=%q", created, friendship.Status)
	}
	accepted, _, err := users.AcceptFriendRequest(ctx, bob.UserID, alice.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Status != model.FriendshipStatusAccepted {
		t.Fatalf("friendship should be accepted: status=%q", accepted.Status)
	}
	bobFriends, err := users.ListFriends(ctx, bob.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bobFriends) != 1 || bobFriends[0].FriendID != alice.UserID {
		t.Fatalf("reciprocal friendship missing: %+v", bobFriends)
	}

	group, creatorMembers, err := groups.CreateGroup(ctx, model.Group{Name: "PG Group"}, alice.UserID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(creatorMembers) != 1 || creatorMembers[0].UserID != alice.UserID {
		t.Fatalf("creator member mismatch: %+v", creatorMembers)
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
}

func TestPostgresMessageRepositoryConcurrentOrdering(t *testing.T) {
	ctx := context.Background()
	dsn := integrationPostgresDSN(t)
	migrateAndCleanPostgres(t, ctx, dsn)

	messages, err := repository.NewPostgresMessageRepository(dsn)
	if err != nil {
		t.Fatal(err)
	}

	const messageCount = 24
	const senderID = "usr_pg_order_sender"
	const receiverID = "usr_pg_order_receiver"
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]repository.Message, messageCount)
	errs := make([]error, messageCount)
	for i := 0; i < messageCount; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			message, deduplicated, err := messages.CreateMessageIdempotent(ctx, repository.CreateMessageInput{
				SenderID:           senderID,
				ReceiverID:         receiverID,
				ChatType:           repository.ChatTypeSingle,
				ClientMsgID:        fmt.Sprintf("client-pg-order-%02d", i+1),
				ContentType:        repository.ContentTypeText,
				Content:            fmt.Sprintf("postgres concurrent message %02d", i+1),
				ParticipantUserIDs: []string{senderID, receiverID},
			})
			if err != nil {
				errs[i] = err
				return
			}
			if deduplicated {
				errs[i] = fmt.Errorf("message %d unexpectedly deduplicated", i+1)
				return
			}
			results[i] = message
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent postgres send %d: %v", i+1, err)
		}
	}

	gotSeqs := make([]int64, 0, messageCount)
	for _, message := range results {
		gotSeqs = append(gotSeqs, message.Seq)
		if message.ConversationID != repository.SingleConversationID(senderID, receiverID) {
			t.Fatalf("conversation_id = %q, want single conversation", message.ConversationID)
		}
	}
	sort.Slice(gotSeqs, func(i, j int) bool { return gotSeqs[i] < gotSeqs[j] })
	for index, seq := range gotSeqs {
		want := int64(index + 1)
		if seq != want {
			t.Fatalf("postgres concurrent seqs = %v, want contiguous 1..%d", gotSeqs, messageCount)
		}
	}

	conversationID := repository.SingleConversationID(senderID, receiverID)
	pulled, isEnd, nextSeq, err := messages.GetMessages(ctx, conversationID, 1, 0, messageCount+1, repository.MessageStorageOrderAsc)
	if err != nil {
		t.Fatal(err)
	}
	if !isEnd || nextSeq != messageCount+1 || len(pulled) != messageCount {
		t.Fatalf("pull after concurrent sends len=%d isEnd=%v nextSeq=%d", len(pulled), isEnd, nextSeq)
	}
	for index, message := range pulled {
		if message.Seq != int64(index+1) {
			t.Fatalf("pulled message %d seq=%d, want %d", index, message.Seq, index+1)
		}
	}

	states, err := messages.GetConversationSeqStates(ctx, receiverID, []string{conversationID})
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 || states[0].MaxSeq != messageCount || states[0].LastMessage == nil || states[0].LastMessage.Seq != messageCount {
		t.Fatalf("postgres state should reflect max seq %d: %+v", messageCount, states)
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
		// 迁移 013 后 run_id/agent_id/tool_call_id 均为 bigint，必须传数字串。
		RunID:          "910001",
		AgentID:        "910002",
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
		ToolCallID: "910003",
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
	if len(toolCalls) != 1 || toolCalls[0].ToolCallID != "910003" {
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

	if os.Getenv("AGENTS_IM_SKIP_TEST_MIGRATIONS") != "1" {
		root := findRepoRoot(t)
		cmd := exec.CommandContext(ctx, "bash", "scripts/migrate-postgres.sh", "--host-psql")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "DATABASE_URL="+dsn)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("run PostgreSQL migrations: %v\n%s", err, output)
		}
	}

	if _, err := db.ExecContext(ctx, `
truncate table
  agent_python_execs,
  agent_file_reads,
  agent_tool_calls,
  agent_runs,
  message_outbox,
  user_conversation_states,
  messages,
  conversation_threads,
  group_members,
  groups,
  friendships,
  auth_credentials,
  profiles,
  accounts
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
