//go:build integration

package tests

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/wujunhui99/agents_im/internal/apperror"
	authmodel "github.com/wujunhui99/agents_im/internal/auth/model"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
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
	migration, err := os.ReadFile(filepath.Join(root, "db", "migrations", "001_init_postgres.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
truncate table
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
