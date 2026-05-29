package messaging

import (
	"strings"
	"testing"
)

func TestMessageEventMarshalUsesContractFields(t *testing.T) {
	event := sampleAcceptedEvent()

	data, err := MarshalMessageEvent(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	encoded := string(data)
	for _, field := range []string{
		`"event_id"`,
		`"event_type"`,
		`"conversation_id"`,
		`"server_msg_id"`,
		`"sender_id"`,
		`"chat_type"`,
		`"created_at"`,
		`"payload"`,
	} {
		if !strings.Contains(encoded, field) {
			t.Fatalf("encoded event missing %s: %s", field, encoded)
		}
	}

	decoded, err := UnmarshalMessageEvent(data)
	if err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if decoded.EventID != event.EventID || decoded.PartitionKey() != event.ConversationID {
		t.Fatalf("decoded event mismatch: %+v", decoded)
	}
	if len(decoded.Payload.ReceiverIDs) != 1 || decoded.Payload.ReceiverIDs[0] != "user_b" {
		t.Fatalf("decoded receiver ids mismatch: %+v", decoded.Payload.ReceiverIDs)
	}
}

func TestMessageEventValidateRejectsInvalidPayloadContent(t *testing.T) {
	event := sampleAcceptedEvent()
	event.Payload.Content = []byte(`{not-json`)

	if err := event.Validate(); err == nil {
		t.Fatal("expected invalid JSON content to fail validation")
	}
}

func TestReadEventValidateRequiresReader(t *testing.T) {
	event := MessageEvent{
		EventID:        "evt_read_1",
		EventType:      EventTypeMessageRead,
		ConversationID: "single:user_a:user_b",
		Seq:            10,
		ChatType:       ChatTypeSingle,
		CreatedAt:      1710000000500,
		Payload: MessageEventPayload{
			HasReadSeq: 10,
			ReadAt:     1710000000500,
		},
	}

	if err := event.Validate(); err == nil {
		t.Fatal("expected missing payload.user_id to fail validation")
	}
	event.Payload.UserID = "user_b"
	if err := event.Validate(); err != nil {
		t.Fatalf("valid read event failed validation: %v", err)
	}
}

func sampleAcceptedEvent() MessageEvent {
	return MessageEvent{
		EventID:        "evt_accepted_1",
		EventType:      EventTypeMessageAccepted,
		ConversationID: "single:user_a:user_b",
		ServerMsgID:    "msg_000001",
		Seq:            1,
		SenderID:       "user_a",
		ChatType:       ChatTypeSingle,
		CreatedAt:      1710000000000,
		Payload: MessageEventPayload{
			ClientMsgID: "client_1",
			ReceiverID:  "user_b",
			ReceiverIDs: []string{"user_b"},
			ContentType: "text",
			Content:     []byte(`{"text":"hello"}`),
			TraceID:     "trace_1",
		},
	}
}
