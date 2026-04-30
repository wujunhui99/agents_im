package ws

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"testing"

	"github.com/wujunhui99/agents_im/internal/gateway"
	"github.com/wujunhui99/agents_im/internal/gateway/delivery"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/svc"
)

func TestSendMessagePushesSingleReceiverOnly(t *testing.T) {
	server, recorder := newCommandTestServer(nil)
	resp := dispatchSendCommand(t, server, "usr_sender", "req-single-push", map[string]interface{}{
		"chatType":    logic.MessageChatTypeSingle,
		"receiverId":  "usr_receiver",
		"clientMsgId": "client-single-push",
		"contentType": logic.MessageContentTypeText,
		"content":     "hello receiver",
	})

	assertSendOK(t, resp, false)
	calls := recorder.Calls()
	if len(calls) != 1 {
		t.Fatalf("delivery calls = %d, want 1", len(calls))
	}
	if !reflect.DeepEqual(calls[0].recipientUserIDs, []string{"usr_receiver"}) {
		t.Fatalf("single push recipients = %+v, want [usr_receiver]", calls[0].recipientUserIDs)
	}
	if calls[0].event.Data.SenderID != "usr_sender" || calls[0].event.Data.ReceiverID != "usr_receiver" {
		t.Fatalf("single push message mismatch: %+v", calls[0].event.Data)
	}
}

func TestSendMessagePushesGroupActiveMembersExceptSender(t *testing.T) {
	groups := &commandTestGroupLister{
		members: []logic.GroupMemberInfo{
			{UserID: "usr_sender", State: "active"},
			{UserID: "usr_member_b", State: "active"},
			{UserID: "usr_member_c", State: "active"},
			{UserID: "usr_sender", State: "active"},
			{UserID: "usr_left", State: "left"},
		},
	}
	server, recorder := newCommandTestServer(groups)
	resp := dispatchSendCommand(t, server, "usr_sender", "req-group-push", map[string]interface{}{
		"chatType":    logic.MessageChatTypeGroup,
		"groupId":     "grp_ws_push",
		"clientMsgId": "client-group-push",
		"contentType": logic.MessageContentTypeText,
		"content":     "hello group",
	})

	sent := assertSendOK(t, resp, false)
	calls := recorder.Calls()
	if len(calls) != 1 {
		t.Fatalf("delivery calls = %d, want 1", len(calls))
	}
	if calls[0].conversationID != sent.Message.ConversationID {
		t.Fatalf("conversation id = %q, want %q", calls[0].conversationID, sent.Message.ConversationID)
	}
	if !reflect.DeepEqual(calls[0].recipientUserIDs, []string{"usr_member_b", "usr_member_c"}) {
		t.Fatalf("group push recipients = %+v, want active members except sender", calls[0].recipientUserIDs)
	}
	if calls[0].event.Data.GroupID != "grp_ws_push" || calls[0].event.Data.ReceiverID != "" {
		t.Fatalf("group push message mismatch: %+v", calls[0].event.Data)
	}
}

func TestSendMessageDeduplicatedRetryDoesNotPushAgain(t *testing.T) {
	server, recorder := newCommandTestServer(nil)
	payload := map[string]interface{}{
		"chatType":    logic.MessageChatTypeSingle,
		"receiverId":  "usr_receiver",
		"clientMsgId": "client-dedup-push",
		"contentType": logic.MessageContentTypeText,
		"content":     "hello once",
	}

	first := dispatchSendCommand(t, server, "usr_sender", "req-dedup-first", payload)
	assertSendOK(t, first, false)
	second := dispatchSendCommand(t, server, "usr_sender", "req-dedup-second", payload)
	assertSendOK(t, second, true)

	calls := recorder.Calls()
	if len(calls) != 1 {
		t.Fatalf("delivery calls after deduplicated retry = %d, want 1", len(calls))
	}
}

func newCommandTestServer(groups logic.GroupMemberLister) (*Server, *recordingDeliveryDispatcher) {
	recorder := &recordingDeliveryDispatcher{}
	serviceContext := svc.NewMessageServiceContext(repository.NewMemoryMessageRepository(), nil, groups)
	return NewServer(serviceContext, WithDeliveryDispatcher(recorder)), recorder
}

func dispatchSendCommand(t *testing.T, server *Server, userID string, requestID string, payload map[string]interface{}) responseFrame {
	t.Helper()

	raw, err := json.Marshal(map[string]interface{}{
		"requestId": requestID,
		"command":   gateway.CommandSendMessage,
		"payload":   payload,
	})
	if err != nil {
		t.Fatalf("marshal send command: %v", err)
	}
	return server.handleCommand(context.Background(), &Connection{ID: "conn_" + userID, UserID: userID}, raw)
}

func assertSendOK(t *testing.T, resp responseFrame, deduplicated bool) gateway.SendMessageCommandResponse {
	t.Helper()

	if resp.Status != gateway.AckStatusOK || resp.Error != nil || resp.Type != gateway.CommandSendMessage {
		t.Fatalf("unexpected send response: %+v", resp)
	}
	sent, ok := resp.Data.(gateway.SendMessageCommandResponse)
	if !ok {
		t.Fatalf("send response data type = %T, want gateway.SendMessageCommandResponse", resp.Data)
	}
	if sent.Deduplicated != deduplicated {
		t.Fatalf("deduplicated = %v, want %v", sent.Deduplicated, deduplicated)
	}
	if sent.Message.ServerMsgID == "" {
		t.Fatalf("send response missing server_msg_id: %+v", sent)
	}
	return sent
}

type commandTestGroupLister struct {
	members []logic.GroupMemberInfo
}

func (l *commandTestGroupLister) ListMembers(_ context.Context, req logic.ListMembersRequest) (logic.ListMembersResponse, error) {
	return logic.ListMembersResponse{
		GroupID: req.GroupID,
		Members: append([]logic.GroupMemberInfo(nil), l.members...),
	}, nil
}

type recordingDeliveryDispatcher struct {
	mu    sync.Mutex
	calls []recordingDeliveryCall
}

type recordingDeliveryCall struct {
	conversationID   string
	recipientUserIDs []string
	event            delivery.Event
}

func (d *recordingDeliveryDispatcher) DeliverToUser(ctx context.Context, userID string, event delivery.Event) (delivery.Result, error) {
	return d.DeliverToConversation(ctx, event.Data.ConversationID, []string{userID}, event)
}

func (d *recordingDeliveryDispatcher) DeliverToConversation(_ context.Context, conversationID string, recipientUserIDs []string, event delivery.Event) (delivery.Result, error) {
	d.mu.Lock()
	d.calls = append(d.calls, recordingDeliveryCall{
		conversationID:   conversationID,
		recipientUserIDs: append([]string(nil), recipientUserIDs...),
		event:            event,
	})
	d.mu.Unlock()

	result := delivery.Result{ConversationID: conversationID}
	for _, userID := range recipientUserIDs {
		result.AddRecipient(delivery.RecipientResult{
			UserID:                 userID,
			Status:                 delivery.StatusDelivered,
			DeliveredConnectionIDs: []string{"conn_" + userID},
		})
	}
	return result, nil
}

func (d *recordingDeliveryDispatcher) Calls() []recordingDeliveryCall {
	d.mu.Lock()
	defer d.mu.Unlock()

	return append([]recordingDeliveryCall(nil), d.calls...)
}
