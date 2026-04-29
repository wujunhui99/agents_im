package tests

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestMessageSingleChatSeqStartsAtOne(t *testing.T) {
	messageLogic := newMessageTestLogic()

	result, err := messageLogic.SendMessage(context.Background(), testSendRequest("usr_b", "usr_a", "client-1", "hello"))
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if result.Deduplicated {
		t.Fatal("first send should not be deduplicated")
	}
	if result.Message.Seq != 1 {
		t.Fatalf("seq = %d, want 1", result.Message.Seq)
	}
	wantConversationID := repository.SingleConversationID("usr_b", "usr_a")
	if result.Message.ConversationID != wantConversationID {
		t.Fatalf("conversation_id = %q, want %q", result.Message.ConversationID, wantConversationID)
	}
	if result.Message.ServerMsgID == "" {
		t.Fatal("server_msg_id is empty")
	}
}

func TestMessageSendIdempotentRetry(t *testing.T) {
	messageLogic := newMessageTestLogic()
	ctx := context.Background()
	req := testSendRequest("usr_a", "usr_b", "client-retry", "hello")

	first, err := messageLogic.SendMessage(ctx, req)
	if err != nil {
		t.Fatalf("first send: %v", err)
	}
	second, err := messageLogic.SendMessage(ctx, req)
	if err != nil {
		t.Fatalf("retry send: %v", err)
	}
	if !second.Deduplicated {
		t.Fatal("retry should be deduplicated")
	}
	if second.Message.ServerMsgID != first.Message.ServerMsgID || second.Message.Seq != first.Message.Seq {
		t.Fatalf("deduplicated response changed message: first=%+v second=%+v", first.Message, second.Message)
	}

	states, err := messageLogic.GetConversationSeqs(ctx, logic.GetConversationSeqsRequest{
		UserID:          "usr_a",
		ConversationIDs: []string{first.Message.ConversationID},
	})
	if err != nil {
		t.Fatalf("get seqs: %v", err)
	}
	if states.States[0].MaxSeq != 1 {
		t.Fatalf("idempotent retry should not advance max seq: %+v", states.States[0])
	}
}

func TestMessageSendIdempotencyConflict(t *testing.T) {
	messageLogic := newMessageTestLogic()
	ctx := context.Background()

	if _, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-conflict", "hello")); err != nil {
		t.Fatalf("first send: %v", err)
	}
	_, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-conflict", "changed"))
	if err == nil || apperror.From(err).Code != apperror.CodeAlreadyExists {
		t.Fatalf("conflict error = %v, want ALREADY_EXISTS", err)
	}
}

func TestMessagePullBySeqRange(t *testing.T) {
	messageLogic := newMessageTestLogic()
	ctx := context.Background()

	first, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-pull-1", "one"))
	if err != nil {
		t.Fatalf("send first: %v", err)
	}
	if _, err := messageLogic.SendMessage(ctx, testSendRequest("usr_b", "usr_a", "client-pull-2", "two")); err != nil {
		t.Fatalf("send second: %v", err)
	}
	if _, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-pull-3", "three")); err != nil {
		t.Fatalf("send third: %v", err)
	}

	pulled, err := messageLogic.PullMessages(ctx, logic.PullMessagesRequest{
		UserID:         "usr_b",
		ConversationID: first.Message.ConversationID,
		FromSeq:        2,
		ToSeq:          3,
		Limit:          10,
		Order:          "asc",
	})
	if err != nil {
		t.Fatalf("pull messages: %v", err)
	}
	if len(pulled.Messages) != 2 {
		t.Fatalf("pulled %d messages, want 2: %+v", len(pulled.Messages), pulled.Messages)
	}
	if pulled.Messages[0].Seq != 2 || pulled.Messages[1].Seq != 3 {
		t.Fatalf("messages not pulled in asc seq order: %+v", pulled.Messages)
	}
	if !pulled.IsEnd || pulled.NextSeq != 4 {
		t.Fatalf("unexpected pull cursor: %+v", pulled)
	}
}

func TestMessageSenderReadSeqAdvancesAfterSend(t *testing.T) {
	messageLogic := newMessageTestLogic()
	ctx := context.Background()

	sent, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-read-sender", "hello"))
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	senderState := mustMessageState(t, messageLogic, "usr_a", sent.Message.ConversationID)
	if senderState.HasReadSeq != sent.Message.Seq || senderState.UnreadCount != 0 {
		t.Fatalf("sender read state did not advance: %+v", senderState)
	}

	receiverState := mustMessageState(t, messageLogic, "usr_b", sent.Message.ConversationID)
	if receiverState.HasReadSeq != 0 || receiverState.UnreadCount != 1 {
		t.Fatalf("receiver should have one unread message: %+v", receiverState)
	}
}

func TestMessageMarkReadRejectsGreaterThanMaxSeq(t *testing.T) {
	messageLogic := newMessageTestLogic()
	ctx := context.Background()

	sent, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-read-reject", "hello"))
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	_, err = messageLogic.MarkConversationAsRead(ctx, logic.MarkConversationAsReadRequest{
		UserID:         "usr_b",
		ConversationID: sent.Message.ConversationID,
		HasReadSeq:     sent.Message.Seq + 1,
	})
	if err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("mark read error = %v, want INVALID_ARGUMENT", err)
	}
}

func TestMessageMarkReadIsMonotonic(t *testing.T) {
	messageLogic := newMessageTestLogic()
	ctx := context.Background()

	first, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-mono-1", "one"))
	if err != nil {
		t.Fatalf("send first: %v", err)
	}
	if _, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-mono-2", "two")); err != nil {
		t.Fatalf("send second: %v", err)
	}

	advanced, err := messageLogic.MarkConversationAsRead(ctx, logic.MarkConversationAsReadRequest{
		UserID:         "usr_b",
		ConversationID: first.Message.ConversationID,
		HasReadSeq:     2,
	})
	if err != nil {
		t.Fatalf("mark read to 2: %v", err)
	}
	if !advanced.Updated || advanced.HasReadSeq != 2 || advanced.UnreadCount != 0 {
		t.Fatalf("unexpected advanced read state: %+v", advanced)
	}

	regressed, err := messageLogic.MarkConversationAsRead(ctx, logic.MarkConversationAsReadRequest{
		UserID:         "usr_b",
		ConversationID: first.Message.ConversationID,
		HasReadSeq:     1,
	})
	if err != nil {
		t.Fatalf("mark read to 1: %v", err)
	}
	if regressed.Updated || regressed.HasReadSeq != 2 || regressed.UnreadCount != 0 {
		t.Fatalf("mark read should be monotonic: %+v", regressed)
	}
}

func TestMessageConversationSeqQueryUnreadCount(t *testing.T) {
	messageLogic := newMessageTestLogic()
	ctx := context.Background()

	first, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-unread-1", "one"))
	if err != nil {
		t.Fatalf("send first: %v", err)
	}
	if _, err := messageLogic.SendMessage(ctx, testSendRequest("usr_a", "usr_b", "client-unread-2", "two")); err != nil {
		t.Fatalf("send second: %v", err)
	}

	state := mustMessageState(t, messageLogic, "usr_b", first.Message.ConversationID)
	if state.MaxSeq != 2 || state.HasReadSeq != 0 || state.UnreadCount != 2 {
		t.Fatalf("unexpected unread state before read: %+v", state)
	}
	if state.LastMessage == nil || state.LastMessage.Seq != 2 {
		t.Fatalf("last message missing from state: %+v", state)
	}

	if _, err := messageLogic.MarkConversationAsRead(ctx, logic.MarkConversationAsReadRequest{
		UserID:         "usr_b",
		ConversationID: first.Message.ConversationID,
		HasReadSeq:     1,
	}); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	state = mustMessageState(t, messageLogic, "usr_b", first.Message.ConversationID)
	if state.MaxSeq != 2 || state.HasReadSeq != 1 || state.UnreadCount != 1 {
		t.Fatalf("unexpected unread state after read: %+v", state)
	}
}

func newMessageTestLogic() *logic.MessageLogic {
	return logic.NewMessageLogic(repository.NewMemoryMessageRepository())
}

func testSendRequest(senderID string, receiverID string, clientMsgID string, content string) logic.SendMessageRequest {
	return logic.SendMessageRequest{
		SenderID:    senderID,
		ReceiverID:  receiverID,
		ChatType:    logic.MessageChatTypeSingle,
		ClientMsgID: clientMsgID,
		ContentType: logic.MessageContentTypeText,
		Content:     content,
	}
}

func mustMessageState(t *testing.T, messageLogic *logic.MessageLogic, userID string, conversationID string) logic.ConversationSeqState {
	t.Helper()

	result, err := messageLogic.GetConversationSeqs(context.Background(), logic.GetConversationSeqsRequest{
		UserID:          userID,
		ConversationIDs: []string{conversationID},
	})
	if err != nil {
		t.Fatalf("get conversation seqs: %v", err)
	}
	if len(result.States) != 1 {
		t.Fatalf("got %d states, want 1", len(result.States))
	}
	return result.States[0]
}
