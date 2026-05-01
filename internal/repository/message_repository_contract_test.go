package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/wujunhui99/agents_im/internal/apperror"
)

func TestMessageRepositoryContractMemory(t *testing.T) {
	runMessageRepositoryContract(t, func(t *testing.T) MessageRepository {
		t.Helper()
		return NewMemoryMessageRepository()
	})
}

func TestMessageRepositoryContractPostgresOptIn(t *testing.T) {
	if os.Getenv("AGENTS_IM_TEST_POSTGRES_CONTRACT") != "1" {
		t.Skip("set AGENTS_IM_TEST_POSTGRES_CONTRACT=1 with DATABASE_URL or AGENTS_IM_POSTGRES_DSN to run PostgreSQL message repository contracts")
	}
	dsn := os.Getenv("AGENTS_IM_POSTGRES_DSN")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required for PostgreSQL message repository contracts")
	}

	migratePostgresMessageContract(t, context.Background(), dsn)
	runMessageRepositoryContract(t, func(t *testing.T) MessageRepository {
		t.Helper()
		repo, err := NewPostgresMessageRepository(dsn)
		if err != nil {
			t.Fatal(err)
		}
		return repo
	})
}

func runMessageRepositoryContract(t *testing.T, newRepo func(t *testing.T) MessageRepository) {
	t.Run("pagination order range and malicious order", func(t *testing.T) {
		repo := newRepo(t)
		ctx := context.Background()
		conversation := createContractMessages(t, ctx, repo, 6)

		messages, isEnd, nextSeq, err := repo.GetMessages(ctx, conversation.ID, 1, 0, 3, MessageStorageOrderAsc)
		if err != nil {
			t.Fatalf("pull asc first page: %v", err)
		}
		assertMessageSeqs(t, messages, []int64{1, 2, 3})
		if isEnd || nextSeq != 4 {
			t.Fatalf("asc first page cursor isEnd=%v nextSeq=%d, want false/4", isEnd, nextSeq)
		}

		messages, isEnd, nextSeq, err = repo.GetMessages(ctx, conversation.ID, 4, 0, 3, MessageStorageOrderAsc)
		if err != nil {
			t.Fatalf("pull asc second page: %v", err)
		}
		assertMessageSeqs(t, messages, []int64{4, 5, 6})
		if !isEnd || nextSeq != 7 {
			t.Fatalf("asc second page cursor isEnd=%v nextSeq=%d, want true/7", isEnd, nextSeq)
		}

		messages, isEnd, nextSeq, err = repo.GetMessages(ctx, conversation.ID, 0, 0, 2, MessageStorageOrderDesc)
		if err != nil {
			t.Fatalf("pull desc first page: %v", err)
		}
		assertMessageSeqs(t, messages, []int64{6, 5})
		if isEnd || nextSeq != 4 {
			t.Fatalf("desc first page cursor isEnd=%v nextSeq=%d, want false/4", isEnd, nextSeq)
		}

		messages, isEnd, nextSeq, err = repo.GetMessages(ctx, conversation.ID, 2, 5, 10, MessageStorageOrderDesc)
		if err != nil {
			t.Fatalf("pull desc range: %v", err)
		}
		assertMessageSeqs(t, messages, []int64{5, 4, 3, 2})
		if !isEnd || nextSeq != 1 {
			t.Fatalf("desc range cursor isEnd=%v nextSeq=%d, want true/1", isEnd, nextSeq)
		}

		messages, isEnd, nextSeq, err = repo.GetMessages(ctx, conversation.ID, 5, 4, 10, MessageStorageOrderAsc)
		if err != nil {
			t.Fatalf("pull empty explicit range: %v", err)
		}
		if len(messages) != 0 || !isEnd || nextSeq != 5 {
			t.Fatalf("empty explicit range messages=%+v isEnd=%v nextSeq=%d, want empty/true/5", messages, isEnd, nextSeq)
		}

		messages, isEnd, nextSeq, err = repo.GetMessages(ctx, conversation.ID, 7, 0, 10, MessageStorageOrderAsc)
		if err != nil {
			t.Fatalf("pull from seq beyond max: %v", err)
		}
		if len(messages) != 0 || !isEnd || nextSeq != 7 {
			t.Fatalf("from seq beyond max messages=%+v isEnd=%v nextSeq=%d, want empty/true/7", messages, isEnd, nextSeq)
		}

		_, _, _, err = repo.GetMessages(ctx, conversation.ID, -1, 0, 10, MessageStorageOrderAsc)
		assertAppErrorCode(t, err, apperror.CodeInvalidArgument)
		_, _, _, err = repo.GetMessages(ctx, conversation.ID, 1, -1, 10, MessageStorageOrderAsc)
		assertAppErrorCode(t, err, apperror.CodeInvalidArgument)
		_, _, _, err = repo.GetMessages(ctx, conversation.ID, 1, 0, -1, MessageStorageOrderAsc)
		assertAppErrorCode(t, err, apperror.CodeInvalidArgument)
		_, _, _, err = repo.GetMessages(ctx, conversation.ID, 1, 0, 10, "sideways")
		assertAppErrorCode(t, err, apperror.CodeInvalidArgument)

		maliciousOrder := "desc; delete from messages where conversation_id = '" + conversation.ID + "'; --"
		_, _, _, err = repo.GetMessages(ctx, conversation.ID, 1, 0, 10, maliciousOrder)
		assertAppErrorCode(t, err, apperror.CodeInvalidArgument)
		messages, _, _, err = repo.GetMessages(ctx, conversation.ID, 1, 0, 10, MessageStorageOrderAsc)
		if err != nil {
			t.Fatalf("pull after malicious order rejection: %v", err)
		}
		assertMessageSeqs(t, messages, []int64{1, 2, 3, 4, 5, 6})
	})

	t.Run("limit clipping uses limit plus one cursor", func(t *testing.T) {
		repo := newRepo(t)
		ctx := context.Background()
		conversation := createContractMessages(t, ctx, repo, maxMessagePullLimit+1)

		messages, isEnd, nextSeq, err := repo.GetMessages(ctx, conversation.ID, 1, 0, maxMessagePullLimit+499, MessageStorageOrderAsc)
		if err != nil {
			t.Fatalf("pull clipped page: %v", err)
		}
		if len(messages) != maxMessagePullLimit || isEnd || nextSeq != int64(maxMessagePullLimit+1) {
			t.Fatalf("clipped page len=%d isEnd=%v nextSeq=%d, want %d/false/%d", len(messages), isEnd, nextSeq, maxMessagePullLimit, maxMessagePullLimit+1)
		}
	})

	t.Run("conversation state access is participant scoped", func(t *testing.T) {
		repo := newRepo(t)
		ctx := context.Background()
		conversation := createContractMessages(t, ctx, repo, 1)

		states, err := repo.GetConversationSeqStates(ctx, conversation.UserB, []string{conversation.ID})
		if err != nil {
			t.Fatalf("receiver state lookup: %v", err)
		}
		if len(states) != 1 || states[0].HasReadSeq != 0 || states[0].UnreadCount != 1 {
			t.Fatalf("receiver state = %+v, want unread seq state", states)
		}

		_, err = repo.GetConversationSeqStates(ctx, conversation.Outsider, []string{conversation.ID})
		assertAppErrorCode(t, err, apperror.CodeNotFound)
		states, err = repo.GetConversationSeqStates(ctx, conversation.Outsider, nil)
		if err != nil {
			t.Fatalf("outsider visible state listing: %v", err)
		}
		if len(states) != 0 {
			t.Fatalf("outsider visible states = %+v, want empty", states)
		}

		updatedState, updated, err := repo.SetUserHasReadSeqMax(ctx, conversation.UserB, conversation.ID, 1)
		if err != nil {
			t.Fatalf("receiver mark read: %v", err)
		}
		if !updated || updatedState.HasReadSeq != 1 || updatedState.UnreadCount != 0 {
			t.Fatalf("receiver mark read state=%+v updated=%v, want read seq 1", updatedState, updated)
		}
		staleState, updated, err := repo.SetUserHasReadSeqMax(ctx, conversation.UserB, conversation.ID, 0)
		if err != nil {
			t.Fatalf("receiver stale mark read: %v", err)
		}
		if updated || staleState.HasReadSeq != 1 {
			t.Fatalf("stale mark read state=%+v updated=%v, want unchanged", staleState, updated)
		}
		_, _, err = repo.SetUserHasReadSeqMax(ctx, conversation.UserB, conversation.ID, 2)
		assertAppErrorCode(t, err, apperror.CodeInvalidArgument)
		_, _, err = repo.SetUserHasReadSeqMax(ctx, conversation.Outsider, conversation.ID, 0)
		assertAppErrorCode(t, err, apperror.CodeNotFound)
	})

	t.Run("create input rejects unsafe ids and upper bounds", func(t *testing.T) {
		repo := newRepo(t)
		ctx := context.Background()
		base := contractInput(messageContractPrefix(), 1)

		unsafeSender := base
		unsafeSender.SenderID = "usr:bad"
		_, _, err := repo.CreateMessageIdempotent(ctx, unsafeSender)
		assertAppErrorCode(t, err, apperror.CodeInvalidArgument)

		unsafeClientID := base
		unsafeClientID.ClientMsgID = "client" + "\x00" + "bad"
		_, _, err = repo.CreateMessageIdempotent(ctx, unsafeClientID)
		assertAppErrorCode(t, err, apperror.CodeInvalidArgument)

		overlongClientID := base
		overlongClientID.ClientMsgID = strings.Repeat("c", maxMessageIDLength+1)
		_, _, err = repo.CreateMessageIdempotent(ctx, overlongClientID)
		assertAppErrorCode(t, err, apperror.CodeInvalidArgument)

		overlongContent := base
		overlongContent.Content = strings.Repeat("x", maxMessageContentLength+1)
		_, _, err = repo.CreateMessageIdempotent(ctx, overlongContent)
		assertAppErrorCode(t, err, apperror.CodeInvalidArgument)

		overlongConversation := base
		overlongConversation.SenderID = strings.Repeat("a", maxMessageIDLength)
		overlongConversation.ReceiverID = strings.Repeat("b", maxMessageIDLength)
		overlongConversation.ParticipantUserIDs = []string{overlongConversation.SenderID, overlongConversation.ReceiverID}
		_, _, err = repo.CreateMessageIdempotent(ctx, overlongConversation)
		assertAppErrorCode(t, err, apperror.CodeInvalidArgument)
	})
}

type contractConversation struct {
	ID       string
	UserA    string
	UserB    string
	Outsider string
}

func createContractMessages(t *testing.T, ctx context.Context, repo MessageRepository, count int) contractConversation {
	t.Helper()

	prefix := messageContractPrefix()
	userA := prefix + "_a"
	userB := prefix + "_b"
	outside := prefix + "_outside"
	var conversationID string
	for seq := 1; seq <= count; seq++ {
		input := contractInput(prefix, seq)
		message, deduplicated, err := repo.CreateMessageIdempotent(ctx, input)
		if err != nil {
			t.Fatalf("create message %d: %v", seq, err)
		}
		if deduplicated {
			t.Fatalf("message %d unexpectedly deduplicated", seq)
		}
		if message.Seq != int64(seq) {
			t.Fatalf("message %d seq=%d, want %d", seq, message.Seq, seq)
		}
		if conversationID == "" {
			conversationID = message.ConversationID
		}
		if message.ConversationID != conversationID {
			t.Fatalf("message %d conversation_id=%q, want %q", seq, message.ConversationID, conversationID)
		}
	}
	return contractConversation{ID: conversationID, UserA: userA, UserB: userB, Outsider: outside}
}

func contractInput(prefix string, seq int) CreateMessageInput {
	userA := prefix + "_a"
	userB := prefix + "_b"
	senderID := userA
	receiverID := userB
	if seq%2 == 0 {
		senderID = userB
		receiverID = userA
	}
	return CreateMessageInput{
		SenderID:           senderID,
		ReceiverID:         receiverID,
		ChatType:           ChatTypeSingle,
		ClientMsgID:        fmt.Sprintf("%s_client_%03d", prefix, seq),
		ContentType:        ContentTypeText,
		Content:            fmt.Sprintf("message %03d", seq),
		ParticipantUserIDs: []string{userA, userB},
	}
}

func messageContractPrefix() string {
	return fmt.Sprintf("msg_contract_%d", time.Now().UnixNano())
}

func assertMessageSeqs(t *testing.T, messages []Message, want []int64) {
	t.Helper()

	got := make([]int64, 0, len(messages))
	for _, message := range messages {
		got = append(got, message.Seq)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("message seqs = %v, want %v", got, want)
	}
}

func assertAppErrorCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()

	if err == nil {
		t.Fatalf("error is nil, want %s", want)
	}
	if got := apperror.From(err).Code; got != want {
		t.Fatalf("error code = %s from %v, want %s", got, err, want)
	}
}

func migratePostgresMessageContract(t *testing.T, ctx context.Context, dsn string) {
	t.Helper()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	root := findRepositoryTestRoot(t)
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
			t.Fatalf("apply migration %s: %v", filepath.Base(migrationPath), err)
		}
	}
}

func findRepositoryTestRoot(t *testing.T) string {
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
