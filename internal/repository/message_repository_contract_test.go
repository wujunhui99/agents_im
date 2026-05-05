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
	"sync"
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
	t.Run("concurrent same conversation sends allocate contiguous seqs", func(t *testing.T) {
		repo := newRepo(t)
		ctx := context.Background()
		prefix := messageContractPrefix()
		const messageCount = 32

		start := make(chan struct{})
		var wg sync.WaitGroup
		results := make([]Message, messageCount)
		errs := make([]error, messageCount)
		for i := 0; i < messageCount; i++ {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				input := contractInput(prefix, i+1)
				message, deduplicated, err := repo.CreateMessageIdempotent(ctx, input)
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
				t.Fatalf("concurrent create %d: %v", i+1, err)
			}
		}

		conversationID := repositoryConversationID(t, results)
		gotSeqs := make([]int64, 0, messageCount)
		for _, message := range results {
			if message.ConversationID != conversationID {
				t.Fatalf("message conversation_id=%q, want %q", message.ConversationID, conversationID)
			}
			gotSeqs = append(gotSeqs, message.Seq)
		}
		sort.Slice(gotSeqs, func(i, j int) bool { return gotSeqs[i] < gotSeqs[j] })
		wantSeqs := make([]int64, 0, messageCount)
		for seq := int64(1); seq <= messageCount; seq++ {
			wantSeqs = append(wantSeqs, seq)
		}
		if !reflect.DeepEqual(gotSeqs, wantSeqs) {
			t.Fatalf("allocated seqs = %v, want %v", gotSeqs, wantSeqs)
		}

		pulled, isEnd, nextSeq, err := repo.GetMessages(ctx, conversationID, 1, 0, messageCount+1, MessageStorageOrderAsc)
		if err != nil {
			t.Fatalf("pull after concurrent sends: %v", err)
		}
		assertMessageSeqs(t, pulled, wantSeqs)
		if !isEnd || nextSeq != messageCount+1 {
			t.Fatalf("pull cursor isEnd=%v nextSeq=%d, want true/%d", isEnd, nextSeq, messageCount+1)
		}

		state := mustRepositorySeqState(t, repo, ctx, prefix+"_a", conversationID)
		if state.MaxSeq != messageCount || state.LastMessage == nil || state.LastMessage.Seq != messageCount {
			t.Fatalf("state should reflect max seq %d, got %+v", messageCount, state)
		}
	})

	t.Run("idempotent retry preserves seq without consuming another seq", func(t *testing.T) {
		repo := newRepo(t)
		ctx := context.Background()
		prefix := messageContractPrefix()

		input := contractInput(prefix, 1)
		first, deduplicated, err := repo.CreateMessageIdempotent(ctx, input)
		if err != nil {
			t.Fatalf("first send: %v", err)
		}
		if deduplicated || first.Seq != 1 {
			t.Fatalf("first send message=%+v deduplicated=%v, want seq 1 without dedupe", first, deduplicated)
		}

		again, deduplicated, err := repo.CreateMessageIdempotent(ctx, input)
		if err != nil {
			t.Fatalf("retry send: %v", err)
		}
		if !deduplicated || again.ServerMsgID != first.ServerMsgID || again.Seq != first.Seq {
			t.Fatalf("retry should return original message: first=%+v again=%+v deduplicated=%v", first, again, deduplicated)
		}

		conflicting := input
		conflicting.Content = "different payload"
		_, _, err = repo.CreateMessageIdempotent(ctx, conflicting)
		assertAppErrorCode(t, err, apperror.CodeAlreadyExists)

		secondInput := contractInput(prefix, 2)
		second, deduplicated, err := repo.CreateMessageIdempotent(ctx, secondInput)
		if err != nil {
			t.Fatalf("second unique send: %v", err)
		}
		if deduplicated || second.Seq != 2 {
			t.Fatalf("second unique send message=%+v deduplicated=%v, want seq 2 without dedupe", second, deduplicated)
		}

		pulled, _, _, err := repo.GetMessages(ctx, first.ConversationID, 1, 0, 10, MessageStorageOrderAsc)
		if err != nil {
			t.Fatalf("pull after idempotent retry: %v", err)
		}
		assertMessageSeqs(t, pulled, []int64{1, 2})
	})

	t.Run("last message state follows max seq instead of latest timestamp", func(t *testing.T) {
		repo := newRepo(t)
		ctx := context.Background()
		prefix := messageContractPrefix()

		firstInput := contractInput(prefix, 1)
		secondInput := contractInput(prefix, 2)
		firstInput.SenderID = prefix + "_a"
		firstInput.ReceiverID = prefix + "_b"
		firstInput.ParticipantUserIDs = []string{prefix + "_a", prefix + "_b"}
		secondInput.SenderID = prefix + "_a"
		secondInput.ReceiverID = prefix + "_b"
		secondInput.ParticipantUserIDs = firstInput.ParticipantUserIDs
		secondInput.Content = "seq two but older timestamp"

		setRepositoryNow(repo, time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC))
		first, _, err := repo.CreateMessageIdempotent(ctx, firstInput)
		if err != nil {
			t.Fatalf("first send: %v", err)
		}
		setRepositoryNow(repo, time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC))
		second, _, err := repo.CreateMessageIdempotent(ctx, secondInput)
		if err != nil {
			t.Fatalf("second send: %v", err)
		}
		if first.SendTime <= second.SendTime {
			t.Fatalf("test setup expected second send_time before first: first=%d second=%d", first.SendTime, second.SendTime)
		}

		state := mustRepositorySeqState(t, repo, ctx, prefix+"_b", first.ConversationID)
		if state.MaxSeq != 2 || state.LastMessage == nil || state.LastMessage.ServerMsgID != second.ServerMsgID {
			t.Fatalf("last message should be max seq message, state=%+v second=%+v", state, second)
		}
		if state.MaxSeqTime != second.SendTime {
			t.Fatalf("max seq time = %d, want second seq send_time %d", state.MaxSeqTime, second.SendTime)
		}
	})

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

	t.Run("legacy direct conversation missing user states is repaired for participants only", func(t *testing.T) {
		repo := newRepo(t)
		ctx := context.Background()
		conversation := createContractMessages(t, ctx, repo, 2)
		removeRepositoryConversationStateForTest(t, repo, ctx, conversation.UserA, conversation.ID)
		removeRepositoryConversationStateForTest(t, repo, ctx, conversation.UserB, conversation.ID)

		for _, userID := range []string{conversation.UserA, conversation.UserB} {
			states, err := repo.GetConversationSeqStates(ctx, userID, []string{conversation.ID})
			if err != nil {
				t.Fatalf("participant %s explicit seq repair: %v", userID, err)
			}
			if len(states) != 1 || states[0].MaxSeq != 2 || states[0].HasReadSeq != 0 || states[0].UnreadCount != 2 {
				t.Fatalf("participant %s repaired state = %+v, want max=2 read=0 unread=2", userID, states)
			}

			messages, isEnd, nextSeq, err := userScopedReaderForTest(t, repo).GetMessagesForUser(ctx, userID, conversation.ID, 1, 0, 50, MessageStorageOrderAsc)
			if err != nil {
				t.Fatalf("participant %s repaired pull: %v", userID, err)
			}
			assertMessageSeqs(t, messages, []int64{1, 2})
			if !isEnd || nextSeq != 3 {
				t.Fatalf("participant %s pull cursor isEnd=%v nextSeq=%d, want true/3", userID, isEnd, nextSeq)
			}
		}

		setRepositoryVisibleStartSeqForTest(t, repo, ctx, conversation.UserB, conversation.ID, 2)
		messages, _, _, err := userScopedReaderForTest(t, repo).GetMessagesForUser(ctx, conversation.UserB, conversation.ID, 1, 0, 50, MessageStorageOrderAsc)
		if err != nil {
			t.Fatalf("participant wrong visible_start_seq repair pull: %v", err)
		}
		assertMessageSeqs(t, messages, []int64{1, 2})

		removeRepositoryConversationStateForTest(t, repo, ctx, conversation.UserA, conversation.ID)
		states, err := repo.GetConversationSeqStates(ctx, conversation.UserA, nil)
		if err != nil {
			t.Fatalf("empty seq listing should repair legacy direct state: %v", err)
		}
		if len(states) != 1 || states[0].ConversationID != conversation.ID || states[0].MaxSeq != 2 {
			t.Fatalf("empty seq listing states = %+v, want repaired direct conversation", states)
		}

		_, err = repo.GetConversationSeqStates(ctx, conversation.Outsider, []string{conversation.ID})
		assertAppErrorCode(t, err, apperror.CodeNotFound)
		_, _, _, err = userScopedReaderForTest(t, repo).GetMessagesForUser(ctx, conversation.Outsider, conversation.ID, 1, 0, 50, MessageStorageOrderAsc)
		assertAppErrorCode(t, err, apperror.CodeNotFound)
		outsiderStates, err := repo.GetConversationSeqStates(ctx, conversation.Outsider, nil)
		if err != nil {
			t.Fatalf("outsider empty seq listing: %v", err)
		}
		if len(outsiderStates) != 0 {
			t.Fatalf("outsider states = %+v, want empty", outsiderStates)
		}
	})

	t.Run("group new member keeps join visibility boundary", func(t *testing.T) {
		repo := newRepo(t)
		ctx := context.Background()
		prefix := messageContractPrefix()
		groupID := prefix + "_group"
		oldMember := prefix + "_old"
		newMember := prefix + "_new"

		first, deduplicated, err := repo.CreateMessageIdempotent(ctx, CreateMessageInput{
			SenderID:           oldMember,
			GroupID:            groupID,
			ChatType:           ChatTypeGroup,
			ClientMsgID:        prefix + "_group_client_1",
			ContentType:        ContentTypeText,
			Content:            "before join",
			ParticipantUserIDs: []string{oldMember},
		})
		if err != nil {
			t.Fatalf("first group send: %v", err)
		}
		if deduplicated || first.Seq != 1 {
			t.Fatalf("first group message=%+v deduplicated=%v, want seq 1 without dedupe", first, deduplicated)
		}

		second, deduplicated, err := repo.CreateMessageIdempotent(ctx, CreateMessageInput{
			SenderID:           oldMember,
			GroupID:            groupID,
			ChatType:           ChatTypeGroup,
			ClientMsgID:        prefix + "_group_client_2",
			ContentType:        ContentTypeText,
			Content:            "after join",
			ParticipantUserIDs: []string{oldMember, newMember},
		})
		if err != nil {
			t.Fatalf("second group send: %v", err)
		}
		if deduplicated || second.Seq != 2 {
			t.Fatalf("second group message=%+v deduplicated=%v, want seq 2 without dedupe", second, deduplicated)
		}

		newMemberMessages, _, _, err := userScopedReaderForTest(t, repo).GetMessagesForUser(ctx, newMember, first.ConversationID, 1, 0, 50, MessageStorageOrderAsc)
		if err != nil {
			t.Fatalf("new member pull: %v", err)
		}
		assertMessageSeqs(t, newMemberMessages, []int64{2})

		oldMemberMessages, _, _, err := userScopedReaderForTest(t, repo).GetMessagesForUser(ctx, oldMember, first.ConversationID, 1, 0, 50, MessageStorageOrderAsc)
		if err != nil {
			t.Fatalf("old member pull: %v", err)
		}
		assertMessageSeqs(t, oldMemberMessages, []int64{1, 2})
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

func repositoryConversationID(t *testing.T, messages []Message) string {
	t.Helper()

	if len(messages) == 0 {
		t.Fatal("messages is empty")
	}
	conversationID := messages[0].ConversationID
	if conversationID == "" {
		t.Fatal("first message conversation id is empty")
	}
	return conversationID
}

func mustRepositorySeqState(t *testing.T, repo MessageRepository, ctx context.Context, userID string, conversationID string) ConversationSeqState {
	t.Helper()

	states, err := repo.GetConversationSeqStates(ctx, userID, []string{conversationID})
	if err != nil {
		t.Fatalf("get conversation seq state: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("got %d states, want 1: %+v", len(states), states)
	}
	return states[0]
}

func setRepositoryNow(repo MessageRepository, now time.Time) {
	switch r := repo.(type) {
	case *MemoryMessageRepository:
		r.now = func() time.Time { return now }
	case *PostgresMessageRepository:
		r.now = func() time.Time { return now }
	}
}

func userScopedReaderForTest(t *testing.T, repo MessageRepository) UserScopedMessageReader {
	t.Helper()

	reader, ok := repo.(UserScopedMessageReader)
	if !ok {
		t.Fatalf("repository %T does not implement UserScopedMessageReader", repo)
	}
	return reader
}

func removeRepositoryConversationStateForTest(t *testing.T, repo MessageRepository, ctx context.Context, userID string, conversationID string) {
	t.Helper()

	switch r := repo.(type) {
	case *MemoryMessageRepository:
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.visibleStartSeqs, userConversationStateKey(userID, conversationID))
		delete(r.readStates, userConversationStateKey(userID, conversationID))
	case *PostgresMessageRepository:
		if _, err := r.conn.ExecCtx(ctx, `
delete from user_conversation_states
where account_id = $1 and conversation_id = $2
`, userID, conversationID); err != nil {
			t.Fatalf("delete postgres conversation state: %v", err)
		}
	default:
		t.Fatalf("unsupported repository type %T", repo)
	}
}

func setRepositoryVisibleStartSeqForTest(t *testing.T, repo MessageRepository, ctx context.Context, userID string, conversationID string, seq int64) {
	t.Helper()

	switch r := repo.(type) {
	case *MemoryMessageRepository:
		r.mu.Lock()
		defer r.mu.Unlock()
		r.visibleStartSeqs[userConversationStateKey(userID, conversationID)] = seq
	case *PostgresMessageRepository:
		if _, err := r.conn.ExecCtx(ctx, `
update user_conversation_states
set visible_start_seq = $3
where account_id = $1 and conversation_id = $2
`, userID, conversationID, seq); err != nil {
			t.Fatalf("update postgres visible_start_seq: %v", err)
		}
	default:
		t.Fatalf("unsupported repository type %T", repo)
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
