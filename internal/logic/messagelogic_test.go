package logic

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestMessageLogicSendInputBounds(t *testing.T) {
	ctx := context.Background()

	t.Run("accepts client message and content at max length", func(t *testing.T) {
		messageLogic := NewMessageLogic(repository.NewMemoryMessageRepository())
		result, err := messageLogic.SendMessage(ctx, logicTestSendRequest(
			"usr_bound_sender",
			"usr_bound_receiver",
			strings.Repeat("c", 128),
			strings.Repeat("x", 4096),
		))
		if err != nil {
			t.Fatalf("send max-bound message: %v", err)
		}
		if result.Message.ClientMsgID != strings.Repeat("c", 128) || len([]rune(result.Message.Content)) != 4096 {
			t.Fatalf("stored max-bound message mismatch: %+v", result.Message)
		}
	})

	t.Run("rejects overlong client message id", func(t *testing.T) {
		messageLogic := NewMessageLogic(repository.NewMemoryMessageRepository())
		_, err := messageLogic.SendMessage(ctx, logicTestSendRequest(
			"usr_client_sender",
			"usr_client_receiver",
			strings.Repeat("c", 129),
			"hello",
		))
		assertLogicAppCode(t, err, apperror.CodeInvalidArgument)
	})

	t.Run("rejects overlong content", func(t *testing.T) {
		messageLogic := NewMessageLogic(repository.NewMemoryMessageRepository())
		_, err := messageLogic.SendMessage(ctx, logicTestSendRequest(
			"usr_content_sender",
			"usr_content_receiver",
			"client-content-over",
			strings.Repeat("x", 4097),
		))
		assertLogicAppCode(t, err, apperror.CodeInvalidArgument)
	})

	t.Run("rejects conversation delimiter in single chat ids", func(t *testing.T) {
		messageLogic := NewMessageLogic(repository.NewMemoryMessageRepository())
		_, err := messageLogic.SendMessage(ctx, logicTestSendRequest(
			"usr:sender",
			"usr_receiver",
			"client-delimiter-sender",
			"hello",
		))
		assertLogicAppCode(t, err, apperror.CodeInvalidArgument)

		_, err = messageLogic.SendMessage(ctx, logicTestSendRequest(
			"usr_sender",
			"usr:receiver",
			"client-delimiter-receiver",
			"hello",
		))
		assertLogicAppCode(t, err, apperror.CodeInvalidArgument)
	})

	t.Run("rejects conversation delimiter in group id", func(t *testing.T) {
		messageLogic := NewMessageLogic(repository.NewMemoryMessageRepository())
		_, err := messageLogic.SendMessage(ctx, SendMessageRequest{
			SenderID:    "usr_group_sender",
			GroupID:     "grp:bad",
			ChatType:    MessageChatTypeGroup,
			ClientMsgID: "client-delimiter-group",
			ContentType: MessageContentTypeText,
			Content:     "hello",
		})
		assertLogicAppCode(t, err, apperror.CodeInvalidArgument)
	})

	t.Run("rejects derived conversation id beyond pullable length", func(t *testing.T) {
		messageLogic := NewMessageLogic(repository.NewMemoryMessageRepository())
		_, err := messageLogic.SendMessage(ctx, logicTestSendRequest(
			strings.Repeat("a", 128),
			strings.Repeat("b", 128),
			"client-overlong-conversation",
			"hello",
		))
		assertLogicAppCode(t, err, apperror.CodeInvalidArgument)
	})
}

func TestMessageLogicPullBoundsAndParticipantAccess(t *testing.T) {
	ctx := context.Background()
	messageLogic := NewMessageLogic(repository.NewMemoryMessageRepository())

	var conversationID string
	for i := 1; i <= 501; i++ {
		result, err := messageLogic.SendMessage(ctx, logicTestSendRequest(
			"usr_pull_sender",
			"usr_pull_receiver",
			fmt.Sprintf("client-pull-bound-%03d", i),
			fmt.Sprintf("message %03d", i),
		))
		if err != nil {
			t.Fatalf("send message %d: %v", i, err)
		}
		conversationID = result.Message.ConversationID
	}

	pulled, err := messageLogic.PullMessages(ctx, PullMessagesRequest{
		UserID:         "usr_pull_receiver",
		ConversationID: conversationID,
		FromSeq:        1,
		Limit:          999,
		Order:          repository.MessageStorageOrderAsc,
	})
	if err != nil {
		t.Fatalf("pull clipped limit: %v", err)
	}
	if len(pulled.Messages) != 500 || pulled.IsEnd || pulled.NextSeq != 501 {
		t.Fatalf("clipped pull len=%d isEnd=%v nextSeq=%d, want 500/false/501", len(pulled.Messages), pulled.IsEnd, pulled.NextSeq)
	}

	_, err = messageLogic.PullMessages(ctx, PullMessagesRequest{
		UserID:         "usr_pull_receiver",
		ConversationID: conversationID,
		FromSeq:        -1,
		Limit:          10,
		Order:          "asc",
	})
	assertLogicAppCode(t, err, apperror.CodeInvalidArgument)

	_, err = messageLogic.PullMessages(ctx, PullMessagesRequest{
		UserID:         "usr_pull_receiver",
		ConversationID: conversationID,
		ToSeq:          -1,
		Limit:          10,
		Order:          "asc",
	})
	assertLogicAppCode(t, err, apperror.CodeInvalidArgument)

	_, err = messageLogic.PullMessages(ctx, PullMessagesRequest{
		UserID:         "usr_pull_receiver",
		ConversationID: conversationID,
		Limit:          -1,
		Order:          "asc",
	})
	assertLogicAppCode(t, err, apperror.CodeInvalidArgument)

	_, err = messageLogic.PullMessages(ctx, PullMessagesRequest{
		UserID:         "usr_pull_receiver",
		ConversationID: conversationID,
		Limit:          10,
		Order:          "desc; delete from messages; --",
	})
	assertLogicAppCode(t, err, apperror.CodeInvalidArgument)

	_, err = messageLogic.GetConversationSeqs(ctx, GetConversationSeqsRequest{
		UserID:          "usr_pull_outsider",
		ConversationIDs: []string{conversationID},
	})
	assertLogicAppCode(t, err, apperror.CodeNotFound)

	_, err = messageLogic.PullMessages(ctx, PullMessagesRequest{
		UserID:         "usr_pull_outsider",
		ConversationID: conversationID,
		Limit:          10,
		Order:          "asc",
	})
	assertLogicAppCode(t, err, apperror.CodeNotFound)

	_, err = messageLogic.MarkConversationAsRead(ctx, MarkConversationAsReadRequest{
		UserID:         "usr_pull_outsider",
		ConversationID: conversationID,
		HasReadSeq:     1,
	})
	assertLogicAppCode(t, err, apperror.CodeNotFound)
}

func logicTestSendRequest(senderID string, receiverID string, clientMsgID string, content string) SendMessageRequest {
	return SendMessageRequest{
		SenderID:    senderID,
		ReceiverID:  receiverID,
		ChatType:    MessageChatTypeSingle,
		ClientMsgID: clientMsgID,
		ContentType: MessageContentTypeText,
		Content:     content,
	}
}

func assertLogicAppCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()

	if err == nil {
		t.Fatalf("error is nil, want %s", want)
	}
	if got := apperror.From(err).Code; got != want {
		t.Fatalf("error code = %s from %v, want %s", got, err, want)
	}
}
