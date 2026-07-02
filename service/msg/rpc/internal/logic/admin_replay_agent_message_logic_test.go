package logic

import (
	"context"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"
)

// humanTextSingleRow 构造一条发往 agent 账号的人类文本单聊消息（可重放）。
func humanTextSingleRow() *model.Messages {
	return &model.Messages{
		MessageId:         1001,
		ConversationId:    "single:usr_alice:agt_bot",
		Seq:               7,
		SenderAccountId:   "usr_alice",
		ReceiverAccountId: "agt_bot",
		ConversationType:  model.ConversationTypeSingle,
		ContentType:       model.ContentTypeTextValue,
		Content:           `{"text":"hi bot"}`,
		MessageOrigin:     model.MessageOriginHumanValue,
		ServerReceivedAt:  time.Unix(1700000000, 0).UTC(),
	}
}

func newAdminReplayLogic(row *model.Messages, err error, publisher *fakePublisher) *AdminReplayAgentMessageLogic {
	svcCtx := &svc.ServiceContext{
		Messages: fakeMessagesModel{findOne: func(context.Context, int64) (*model.Messages, error) { return row, err }},
		Producer: publisher,
	}
	return NewAdminReplayAgentMessageLogic(context.Background(), svcCtx)
}

func TestAdminReplayAgentMessagePublishesTrigger(t *testing.T) {
	publisher := &fakePublisher{}
	resp, err := newAdminReplayLogic(humanTextSingleRow(), nil, publisher).AdminReplayAgentMessage(&msg.AdminReplayAgentMessageRequest{
		ConversationId: "single:usr_alice:agt_bot",
		ServerMsgId:    "1001",
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !resp.GetTriggered() {
		t.Fatalf("expected triggered=true, got %+v", resp)
	}
	if len(publisher.published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(publisher.published))
	}
	got := publisher.published[0]
	if got.topic != messaging.TopicAgentTrigger {
		t.Errorf("topic = %q, want %q", got.topic, messaging.TopicAgentTrigger)
	}
	if got.event.EventType != messaging.EventTypeMessageAccepted {
		t.Errorf("event_type = %q, want %q", got.event.EventType, messaging.EventTypeMessageAccepted)
	}
	if got.event.ServerMsgID != "1001" || got.event.Seq != 7 {
		t.Errorf("server_msg_id/seq = %q/%d, want 1001/7", got.event.ServerMsgID, got.event.Seq)
	}
	if got.event.Payload.ReceiverID != "agt_bot" {
		t.Errorf("payload.receiver_id = %q, want agt_bot", got.event.Payload.ReceiverID)
	}
	if err := got.event.Validate(); err != nil {
		t.Errorf("published accepted event fails schema validation: %v", err)
	}
}

func TestAdminReplayAgentMessageRejectsAIOrigin(t *testing.T) {
	row := humanTextSingleRow()
	row.MessageOrigin = model.MessageOriginAIValue
	publisher := &fakePublisher{}
	_, err := newAdminReplayLogic(row, nil, publisher).AdminReplayAgentMessage(&msg.AdminReplayAgentMessageRequest{
		ConversationId: row.ConversationId,
		ServerMsgId:    "1001",
	})
	if err == nil {
		t.Fatalf("expected error for AI-origin replay")
	}
	if len(publisher.published) != 0 {
		t.Fatalf("must not publish on rejected replay, got %d", len(publisher.published))
	}
}

func TestAdminReplayAgentMessageNotFound(t *testing.T) {
	publisher := &fakePublisher{}
	_, err := newAdminReplayLogic(nil, model.ErrNotFound, publisher).AdminReplayAgentMessage(&msg.AdminReplayAgentMessageRequest{
		ConversationId: "single:usr_alice:agt_bot",
		ServerMsgId:    "1001",
	})
	if err == nil {
		t.Fatalf("expected not-found error")
	}
	if len(publisher.published) != 0 {
		t.Fatalf("must not publish when message not found")
	}
}
