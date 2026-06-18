package logic

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"
)

// mustTestMsgIDGen 构造一个单副本（machine 0）RoutedFlake，供 SendMessage 写路径分配 message_id。
func mustTestMsgIDGen(t *testing.T) *idgen.RoutedFlake {
	t.Helper()
	gen, err := idgen.NewRoutedFlake(idgen.RoutedFlakeConfig{HintBits: 1, MachineBits: 10, MachineID: 0})
	if err != nil {
		t.Fatalf("build test msg id generator: %v", err)
	}
	return gen
}

type capturedPublish struct {
	topic string
	event messaging.MessageEvent
}

type fakePublisher struct {
	mu        sync.Mutex
	published []capturedPublish
	err       error
}

func (f *fakePublisher) PublishEvent(_ context.Context, topic string, event messaging.MessageEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, capturedPublish{topic: topic, event: event.Clone()})
	return nil
}

// Kafka 模式：SendMessage 只 publish message.submitted，ACK seq=0，不碰数据层
// （svcCtx 不配任何 model——若误走 PG 路径会 nil panic，本身就是断言）。
func TestSendMessageDirectKafkaPublishesSubmittedAndAcksWithoutSeq(t *testing.T) {
	publisher := &fakePublisher{}
	svcCtx := &svc.ServiceContext{Producer: publisher, MsgIDGen: mustTestMsgIDGen(t)}

	resp, err := NewSendMessageLogic(context.Background(), svcCtx).SendMessage(&msg.SendMessageRequest{
		SenderId:    "usr_alice",
		ReceiverId:  "usr_bob",
		ChatType:    "single",
		ClientMsgId: "cmid-kafka-1",
		ContentType: "text",
		Content:     "hello kafka path",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	message := resp.GetMessage()
	if message.GetSeq() != 0 {
		t.Fatalf("kafka-mode ACK must not carry seq, got %d", message.GetSeq())
	}
	if message.GetServerMsgId() == "" || message.GetClientMsgId() != "cmid-kafka-1" {
		t.Fatalf("ack identity mismatch: %+v", message)
	}
	if message.GetConversationId() != "single:usr_alice:usr_bob" {
		t.Fatalf("conversation mismatch: %s", message.GetConversationId())
	}
	if resp.GetDeduplicated() {
		t.Fatal("kafka-mode ACK must not claim dedup (converges in msgtransfer)")
	}

	if len(publisher.published) != 1 {
		t.Fatalf("expected exactly 1 publish, got %d", len(publisher.published))
	}
	got := publisher.published[0]
	if got.topic != messaging.TopicToTransfer {
		t.Fatalf("expected topic %s, got %s", messaging.TopicToTransfer, got.topic)
	}
	event := got.event
	if event.EventType != messaging.EventTypeMessageSubmitted || event.Seq != 0 {
		t.Fatalf("expected message.submitted with seq=0, got %s seq=%d", event.EventType, event.Seq)
	}
	if event.ServerMsgID != message.GetServerMsgId() {
		t.Fatalf("event/ack server_msg_id mismatch: %s vs %s", event.ServerMsgID, message.GetServerMsgId())
	}
	if event.PartitionKey() != "single:usr_alice:usr_bob" {
		t.Fatalf("partition key must be conversation_id, got %s", event.PartitionKey())
	}
	if len(event.Payload.VisibleUserIDs) != 2 || event.Payload.PayloadHash == "" || event.Payload.SendTime == 0 {
		t.Fatalf("event payload incomplete: %+v", event.Payload)
	}
	if !strings.Contains(string(event.Payload.Content), "hello kafka path") {
		t.Fatalf("event content mismatch: %s", event.Payload.Content)
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("published event must validate: %v", err)
	}
}

func TestSendMessageDirectKafkaFailsClosedWhenPublishFails(t *testing.T) {
	publisher := &fakePublisher{err: context.DeadlineExceeded}
	svcCtx := &svc.ServiceContext{Producer: publisher, MsgIDGen: mustTestMsgIDGen(t)}

	_, err := NewSendMessageLogic(context.Background(), svcCtx).SendMessage(&msg.SendMessageRequest{
		SenderId:    "usr_alice",
		ReceiverId:  "usr_bob",
		ChatType:    "single",
		ClientMsgId: "cmid-kafka-2",
		ContentType: "text",
		Content:     "must fail",
	})
	if err == nil {
		t.Fatal("expected explicit failure when kafka publish fails (acks=all, no silent fallback)")
	}
}
