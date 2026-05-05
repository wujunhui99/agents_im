package logic

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestGroupSendUsesActiveParticipantsForRecipientsAndVisibility(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryMessageRepository()
	groups := &testGroupMemberLister{
		members: []GroupMemberInfo{
			{UserID: "usr_sender", State: "active"},
			{UserID: "usr_receiver", State: "active"},
			{UserID: "usr_sender", State: "active"},
			{UserID: "usr_left", State: "left"},
		},
	}
	messageLogic := NewMessageLogicWithValidators(repo, nil, groups)

	sent, err := messageLogic.SendMessage(ctx, groupSendRequest("usr_sender", "grp_delivery", "client-group-active", "hello"))
	if err != nil {
		t.Fatalf("send group message: %v", err)
	}
	if !reflect.DeepEqual(sent.RecipientUserIDs, []string{"usr_receiver"}) {
		t.Fatalf("recipient user ids = %+v, want [usr_receiver]", sent.RecipientUserIDs)
	}

	_, err = messageLogic.GetConversationSeqs(ctx, GetConversationSeqsRequest{
		UserID:          "usr_left",
		ConversationIDs: []string{sent.Message.ConversationID},
	})
	if err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("left member seq query error = %v, want FORBIDDEN", err)
	}
}

func TestGroupMemberLeftCannotSeeNewMessages(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryMessageRepository()
	groups := &testGroupMemberLister{
		members: []GroupMemberInfo{
			{UserID: "usr_sender", State: "active"},
			{UserID: "usr_receiver", State: "active"},
			{UserID: "usr_later_left", State: "active"},
		},
	}
	messageLogic := NewMessageLogicWithValidators(repo, nil, groups)

	first, err := messageLogic.SendMessage(ctx, groupSendRequest("usr_sender", "grp_visibility", "client-group-first", "first"))
	if err != nil {
		t.Fatalf("send first group message: %v", err)
	}
	groups.members = []GroupMemberInfo{
		{UserID: "usr_sender", State: "active"},
		{UserID: "usr_receiver", State: "active"},
	}
	second, err := messageLogic.SendMessage(ctx, groupSendRequest("usr_sender", "grp_visibility", "client-group-second", "second"))
	if err != nil {
		t.Fatalf("send second group message: %v", err)
	}
	if !reflect.DeepEqual(second.RecipientUserIDs, []string{"usr_receiver"}) {
		t.Fatalf("second recipient user ids = %+v, want [usr_receiver]", second.RecipientUserIDs)
	}

	_, err = messageLogic.GetConversationSeqs(ctx, GetConversationSeqsRequest{
		UserID:          "usr_later_left",
		ConversationIDs: []string{first.Message.ConversationID},
	})
	if err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("left member explicit seq query error = %v, want FORBIDDEN", err)
	}

	_, err = messageLogic.PullMessages(ctx, PullMessagesRequest{
		UserID:         "usr_later_left",
		ConversationID: first.Message.ConversationID,
		FromSeq:        1,
		ToSeq:          second.Message.Seq,
		Limit:          10,
		Order:          "asc",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("left member explicit pull error = %v, want FORBIDDEN", err)
	}

	_, err = messageLogic.MarkConversationAsRead(ctx, MarkConversationAsReadRequest{
		UserID:         "usr_later_left",
		ConversationID: first.Message.ConversationID,
		HasReadSeq:     second.Message.Seq,
	})
	if err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("left member mark read error = %v, want FORBIDDEN", err)
	}
}

func TestGroupNewMemberHistoryStartsAfterJoinBoundary(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryMessageRepository()
	groups := &testGroupMemberLister{
		members: []GroupMemberInfo{
			{UserID: "usr_sender", State: "active"},
			{UserID: "usr_receiver", State: "active"},
		},
	}
	messageLogic := NewMessageLogicWithValidators(repo, nil, groups)

	first, err := messageLogic.SendMessage(ctx, groupSendRequest("usr_sender", "grp_joined_only", "client-before-join", "before join"))
	if err != nil {
		t.Fatalf("send first group message: %v", err)
	}

	groups.members = []GroupMemberInfo{
		{UserID: "usr_sender", State: "active"},
		{UserID: "usr_receiver", State: "active"},
		{UserID: "usr_new_member", State: "active"},
	}
	second, err := messageLogic.SendMessage(ctx, groupSendRequest("usr_sender", "grp_joined_only", "client-after-join", "after join"))
	if err != nil {
		t.Fatalf("send second group message: %v", err)
	}
	sort.Strings(second.RecipientUserIDs)
	if !reflect.DeepEqual(second.RecipientUserIDs, []string{"usr_new_member", "usr_receiver"}) {
		t.Fatalf("second recipients = %+v, want new member and receiver", second.RecipientUserIDs)
	}

	newMemberState := mustSeqState(t, messageLogic, "usr_new_member", first.Message.ConversationID)
	if newMemberState.MaxSeq != second.Message.Seq || newMemberState.UnreadCount != 1 {
		t.Fatalf("new member state = %+v, want max seq %d and one unread", newMemberState, second.Message.Seq)
	}

	pulled, err := messageLogic.PullMessages(ctx, PullMessagesRequest{
		UserID:         "usr_new_member",
		ConversationID: first.Message.ConversationID,
		FromSeq:        1,
		ToSeq:          second.Message.Seq,
		Limit:          10,
		Order:          "asc",
	})
	if err != nil {
		t.Fatalf("new member pull: %v", err)
	}
	if len(pulled.Messages) != 1 || pulled.Messages[0].ServerMsgID != second.Message.ServerMsgID {
		t.Fatalf("new member messages = %+v, want only post-join message %s", pulled.Messages, second.Message.ServerMsgID)
	}
}

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
	assertLogicAppCode(t, err, apperror.CodeForbidden)

	_, err = messageLogic.PullMessages(ctx, PullMessagesRequest{
		UserID:         "usr_pull_outsider",
		ConversationID: conversationID,
		Limit:          10,
		Order:          "asc",
	})
	assertLogicAppCode(t, err, apperror.CodeForbidden)

	_, err = messageLogic.MarkConversationAsRead(ctx, MarkConversationAsReadRequest{
		UserID:         "usr_pull_outsider",
		ConversationID: conversationID,
		HasReadSeq:     1,
	})
	assertLogicAppCode(t, err, apperror.CodeForbidden)
}

type testGroupMemberLister struct {
	members []GroupMemberInfo
}

func (l *testGroupMemberLister) ListMembers(_ context.Context, req ListMembersRequest) (ListMembersResponse, error) {
	members := append([]GroupMemberInfo(nil), l.members...)
	if strings.TrimSpace(req.RequesterUserID) != "" {
		active := false
		for _, member := range members {
			if member.UserID == req.RequesterUserID && (member.State == "" || member.State == "active") {
				active = true
				break
			}
		}
		if !active {
			return ListMembersResponse{}, apperror.Forbidden("requester is not a group member")
		}
	}
	return ListMembersResponse{GroupID: req.GroupID, Members: members}, nil
}

func groupSendRequest(senderID, groupID, clientMsgID, content string) SendMessageRequest {
	return SendMessageRequest{
		SenderID:    senderID,
		GroupID:     groupID,
		ChatType:    MessageChatTypeGroup,
		ClientMsgID: clientMsgID,
		ContentType: MessageContentTypeText,
		Content:     content,
	}
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

func mustSeqState(t *testing.T, messageLogic *MessageLogic, userID, conversationID string) ConversationSeqState {
	t.Helper()

	states, err := messageLogic.GetConversationSeqs(context.Background(), GetConversationSeqsRequest{
		UserID:          userID,
		ConversationIDs: []string{conversationID},
	})
	if err != nil {
		t.Fatalf("get seq state for %s: %v", userID, err)
	}
	if len(states.States) != 1 {
		t.Fatalf("got %d states, want 1: %+v", len(states.States), states.States)
	}
	return states.States[0]
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
