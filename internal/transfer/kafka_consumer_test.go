package transfer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/wujunhui99/agents_im/internal/messaging"
)

func TestEnvelopeFromKafkaMessageMapsAcceptedEvent(t *testing.T) {
	event := kafkaConsumerAcceptedEvent()
	value, err := messaging.MarshalMessageEvent(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	envelope, err := EnvelopeFromKafkaMessage(kafka.Message{
		Topic:     messaging.DefaultMessageEventsTopic,
		Key:       []byte(event.ConversationID),
		Value:     value,
		Partition: 3,
		Offset:    42,
		Time:      time.UnixMilli(event.CreatedAt),
	})
	if err != nil {
		t.Fatalf("map kafka message: %v", err)
	}

	if envelope.ID != event.EventID || envelope.Topic != messaging.DefaultMessageEventsTopic || envelope.Key != event.ConversationID {
		t.Fatalf("envelope metadata mismatch: %+v", envelope)
	}
	if envelope.Partition != 3 || envelope.Offset != 42 || envelope.Attempt != 1 {
		t.Fatalf("transport metadata mismatch: %+v", envelope)
	}
	if envelope.ReceivedAt.IsZero() {
		t.Fatal("received_at should be set")
	}
	if envelope.Event.EventID != event.EventID ||
		envelope.Event.EventType != EventTypeMessageAccepted ||
		envelope.Event.ConversationID != event.ConversationID ||
		envelope.Event.ServerMsgID != event.ServerMsgID ||
		envelope.Event.Seq != event.Seq ||
		envelope.Event.SenderID != event.SenderID ||
		envelope.Event.TraceID != event.Payload.TraceID {
		t.Fatalf("mapped event mismatch: %+v", envelope.Event)
	}
	if len(envelope.Event.ReceiverIDs) != 2 || envelope.Event.ReceiverIDs[0] != "user_b" || envelope.Event.ReceiverIDs[1] != "user_c" {
		t.Fatalf("receiver ids mismatch: %+v", envelope.Event.ReceiverIDs)
	}

	envelope.RawPayload[0] = '['
	if string(value) == string(envelope.RawPayload) {
		t.Fatal("raw payload should be cloned from kafka message value")
	}
}

func TestEnvelopeFromKafkaMessageRejectsInvalidEvents(t *testing.T) {
	if _, err := EnvelopeFromKafkaMessage(kafka.Message{Value: []byte(`{not-json`)}); err == nil {
		t.Fatal("expected malformed JSON to fail")
	}

	readEvent := kafkaConsumerAcceptedEvent()
	readEvent.EventType = messaging.EventTypeMessageRead
	readEvent.ServerMsgID = ""
	readEvent.SenderID = ""
	readEvent.Seq = 0
	readEvent.Payload = messaging.MessageEventPayload{
		UserID:     "user_b",
		HasReadSeq: 10,
		ReadAt:     1710000001000,
	}
	value, err := messaging.MarshalMessageEvent(readEvent)
	if err != nil {
		t.Fatalf("marshal read event: %v", err)
	}
	if _, err := EnvelopeFromKafkaMessage(kafka.Message{Value: value}); err == nil || !strings.Contains(err.Error(), "unsupported transfer event_type") {
		t.Fatalf("expected read event to be rejected, got %v", err)
	}

	invalidAccepted := []byte(`{
		"event_id":"evt_invalid",
		"event_type":"message.accepted",
		"conversation_id":"single:user_a:user_b",
		"seq":1,
		"sender_id":"user_a",
		"chat_type":"single",
		"created_at":1710000000000,
		"payload":{"receiver_ids":["user_b"]}
	}`)
	if _, err := EnvelopeFromKafkaMessage(kafka.Message{Value: invalidAccepted}); err == nil || !strings.Contains(err.Error(), "server_msg_id") {
		t.Fatalf("expected invalid accepted event to fail, got %v", err)
	}
}

func TestKafkaEventConsumerConstructorDoesNotRequireLiveBroker(t *testing.T) {
	consumer, err := NewKafkaEventConsumer(KafkaEventConsumerConfig{
		Brokers: []string{" localhost:19092 "},
	})
	if err != nil {
		t.Fatalf("new kafka event consumer: %v", err)
	}
	defer consumer.Close()

	if consumer.topic != messaging.DefaultMessageEventsTopic || consumer.group != messaging.DefaultConsumerGroup {
		t.Fatalf("defaults mismatch: topic=%q group=%q", consumer.topic, consumer.group)
	}

	if _, err := NewKafkaEventConsumer(KafkaEventConsumerConfig{}); err == nil {
		t.Fatal("expected empty broker list to fail")
	}
}

func TestKafkaEventConsumerReceiveAndAckSemantics(t *testing.T) {
	event := kafkaConsumerAcceptedEvent()
	value, err := messaging.MarshalMessageEvent(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	reader := &fakeKafkaEventReader{
		messages: []kafka.Message{{
			Topic:     messaging.DefaultMessageEventsTopic,
			Key:       []byte(event.ConversationID),
			Value:     value,
			Partition: 2,
			Offset:    7,
		}},
	}
	consumer := &KafkaEventConsumer{
		reader: reader,
		topic:  messaging.DefaultMessageEventsTopic,
		group:  messaging.DefaultConsumerGroup,
	}

	envelope, err := consumer.Receive(context.Background())
	if err != nil {
		t.Fatalf("receive event: %v", err)
	}
	if envelope.Event.EventID != event.EventID {
		t.Fatalf("event id mismatch: %+v", envelope.Event)
	}
	if err := consumer.MarkSuccessful(context.Background(), envelope); err != nil {
		t.Fatalf("mark successful: %v", err)
	}
	if len(reader.commits) != 1 || reader.commits[0].Topic != messaging.DefaultMessageEventsTopic || reader.commits[0].Partition != 2 || reader.commits[0].Offset != 7 {
		t.Fatalf("commit mismatch: %+v", reader.commits)
	}
	if err := consumer.MarkRetry(context.Background(), envelope, RetryDecision{}); err != nil {
		t.Fatalf("mark retry: %v", err)
	}
	if err := consumer.MarkFailed(context.Background(), envelope, ProcessResult{}); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	if len(reader.commits) != 1 {
		t.Fatalf("retry/failed should not commit offsets, commits=%+v", reader.commits)
	}
}

type fakeKafkaEventReader struct {
	messages  []kafka.Message
	commits   []kafka.Message
	fetchErr  error
	commitErr error
	closed    bool
}

func (r *fakeKafkaEventReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if err := ctx.Err(); err != nil {
		return kafka.Message{}, err
	}
	if r.fetchErr != nil {
		return kafka.Message{}, r.fetchErr
	}
	if len(r.messages) == 0 {
		return kafka.Message{}, errors.New("no fake kafka messages")
	}
	message := r.messages[0]
	r.messages = r.messages[1:]
	return message, nil
}

func (r *fakeKafkaEventReader) CommitMessages(ctx context.Context, messages ...kafka.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.commitErr != nil {
		return r.commitErr
	}
	r.commits = append(r.commits, messages...)
	return nil
}

func (r *fakeKafkaEventReader) Close() error {
	r.closed = true
	return nil
}

func kafkaConsumerAcceptedEvent() messaging.MessageEvent {
	return messaging.MessageEvent{
		EventID:        "evt_transfer_1",
		EventType:      messaging.EventTypeMessageAccepted,
		ConversationID: "single:user_a:user_b",
		ServerMsgID:    "msg_transfer_1",
		Seq:            12,
		SenderID:       "user_a",
		ChatType:       messaging.ChatTypeSingle,
		CreatedAt:      1710000000000,
		Payload: messaging.MessageEventPayload{
			ClientMsgID: "client_transfer_1",
			ReceiverID:  "user_c",
			ReceiverIDs: []string{"user_b", "user_b"},
			ContentType: "text",
			Content:     []byte(`{"text":"hello"}`),
			TraceID:     "trace_transfer_1",
		},
	}
}
