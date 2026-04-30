package logic

import (
	"context"
	"reflect"
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

	attempts, err := repo.ListDeliveryAttemptsByMessage(ctx, sent.Message.ServerMsgID)
	if err != nil {
		t.Fatalf("list delivery attempts: %v", err)
	}
	if len(attempts) != 1 || attempts[0].RecipientUserID != "usr_receiver" {
		t.Fatalf("delivery attempts should match active recipient list: %+v", attempts)
	}

	_, err = messageLogic.GetConversationSeqs(ctx, GetConversationSeqsRequest{
		UserID:          "usr_left",
		ConversationIDs: []string{sent.Message.ConversationID},
	})
	if err == nil || apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("left member seq query error = %v, want NOT_FOUND", err)
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

	leftState := mustSeqState(t, messageLogic, "usr_later_left", first.Message.ConversationID)
	if leftState.MaxSeq != first.Message.Seq || leftState.UnreadCount != 1 {
		t.Fatalf("left member visible state should stop at first message: %+v", leftState)
	}

	pulled, err := messageLogic.PullMessages(ctx, PullMessagesRequest{
		UserID:         "usr_later_left",
		ConversationID: first.Message.ConversationID,
		FromSeq:        1,
		ToSeq:          second.Message.Seq,
		Limit:          10,
		Order:          "asc",
	})
	if err != nil {
		t.Fatalf("left member pull: %v", err)
	}
	if len(pulled.Messages) != 1 || pulled.Messages[0].ServerMsgID != first.Message.ServerMsgID {
		t.Fatalf("left member should only pull previously visible message: %+v", pulled.Messages)
	}

	_, err = messageLogic.MarkConversationAsRead(ctx, MarkConversationAsReadRequest{
		UserID:         "usr_later_left",
		ConversationID: first.Message.ConversationID,
		HasReadSeq:     second.Message.Seq,
	})
	if err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("left member mark read error = %v, want INVALID_ARGUMENT", err)
	}
}

type testGroupMemberLister struct {
	members []GroupMemberInfo
}

func (l *testGroupMemberLister) ListMembers(context.Context, ListMembersRequest) (ListMembersResponse, error) {
	members := append([]GroupMemberInfo(nil), l.members...)
	return ListMembersResponse{GroupID: "grp", Members: members}, nil
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
