package tests

import (
	"reflect"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/gateway"
)

func TestGatewayCommandNames(t *testing.T) {
	tests := map[string]string{
		"send":      gateway.CommandSendMessage,
		"pull":      gateway.CommandPullMessages,
		"seqs":      gateway.CommandGetConversationSeqs,
		"mark_read": gateway.CommandMarkConversationRead,
		"heartbeat": gateway.CommandHeartbeat,
	}

	want := map[string]string{
		"send":      "send_message",
		"pull":      "pull_messages",
		"seqs":      "get_conversation_seqs",
		"mark_read": "mark_conversation_read",
		"heartbeat": "heartbeat",
	}

	for name, got := range tests {
		if got != want[name] {
			t.Fatalf("%s command = %q, want %q", name, got, want[name])
		}
	}
}

func TestGatewayRequestMappingInjectsConnectionUser(t *testing.T) {
	send := gateway.MapSendMessageRequest("user_a", gateway.SendMessageCommandRequest{
		ChatType:    "single",
		ReceiverID:  "user_b",
		ClientMsgID: "client-1",
		ContentType: "text",
		Content:     "hello",
	})
	if send.SenderID != "user_a" || send.ReceiverID != "user_b" || send.ClientMsgID != "client-1" {
		t.Fatalf("unexpected send mapping: %+v", send)
	}

	pull := gateway.MapPullMessagesRequest("user_a", gateway.PullMessagesCommandRequest{
		ConversationID: "single:user_a:user_b",
		FromSeq:        1,
		ToSeq:          10,
		Limit:          50,
		Order:          "asc",
	})
	if pull.UserID != "user_a" || pull.ConversationID != "single:user_a:user_b" || pull.FromSeq != 1 || pull.ToSeq != 10 {
		t.Fatalf("unexpected pull mapping: %+v", pull)
	}

	seqs := gateway.MapGetConversationSeqsRequest("user_a", gateway.GetConversationSeqsCommandRequest{
		ConversationIDs: []string{"single:user_a:user_b"},
	})
	if seqs.UserID != "user_a" || !reflect.DeepEqual(seqs.ConversationIDs, []string{"single:user_a:user_b"}) {
		t.Fatalf("unexpected seqs mapping: %+v", seqs)
	}

	read := gateway.MapMarkConversationReadRequest("user_a", gateway.MarkConversationReadCommandRequest{
		ConversationID: "single:user_a:user_b",
		HasReadSeq:     10,
	})
	if read.UserID != "user_a" || read.ConversationID != "single:user_a:user_b" || read.HasReadSeq != 10 {
		t.Fatalf("unexpected read mapping: %+v", read)
	}
}

func TestGatewayResponseMappingPreservesFields(t *testing.T) {
	message := gateway.MessageSnapshot{
		ServerMsgID:    "msg_1",
		ClientMsgID:    "client-1",
		ConversationID: "single:user_a:user_b",
		Seq:            1,
		SenderID:       "user_a",
		ReceiverID:     "user_b",
		ChatType:       "single",
		ContentType:    "text",
		Content:        "hello",
		SendTime:       1710000000000,
		CreatedAt:      1710000000000,
	}

	send := gateway.MapSendMessageResponse(gateway.SendMessageRPCResponse{Message: message, Deduplicated: true})
	if !send.Deduplicated || send.Message.ServerMsgID != "msg_1" || send.Message.Seq != 1 {
		t.Fatalf("unexpected send response mapping: %+v", send)
	}

	pull := gateway.MapPullMessagesResponse(gateway.PullMessagesRPCResponse{
		Messages: []gateway.MessageSnapshot{message},
		IsEnd:    false,
		NextSeq:  2,
	})
	if pull.IsEnd || pull.NextSeq != 2 || len(pull.Messages) != 1 {
		t.Fatalf("unexpected pull response mapping: %+v", pull)
	}

	read := gateway.MapMarkConversationReadResponse(gateway.MarkConversationAsReadRPCResponse{
		ConversationID: "single:user_a:user_b",
		HasReadSeq:     1,
		MaxSeq:         2,
		UnreadCount:    1,
		Updated:        true,
	})
	if read.ConversationID != "single:user_a:user_b" || read.HasReadSeq != 1 || read.MaxSeq != 2 || read.UnreadCount != 1 || !read.Updated {
		t.Fatalf("unexpected read response mapping: %+v", read)
	}
}
