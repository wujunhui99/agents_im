package transfer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/gateway/delivery"
)

func TestGatewayHTTPDispatcherPostsConversationDelivery(t *testing.T) {
	var got struct {
		ConversationID   string         `json:"conversation_id"`
		RecipientUserIDs []string       `json:"recipient_user_ids"`
		Event            delivery.Event `json:"event"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/internal/delivery/conversation" {
			t.Fatalf("path = %s, want /internal/delivery/conversation", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(delivery.Result{
			ConversationID:       got.ConversationID,
			DeliveredRecipients:  1,
			DeliveredConnections: 1,
			Recipients: []delivery.RecipientResult{{
				UserID: got.RecipientUserIDs[0],
				Status: delivery.StatusDelivered,
			}},
		})
	}))
	defer server.Close()

	dispatcher := NewGatewayHTTPDispatcher(GatewayHTTPDispatcherConfig{
		Endpoint: server.URL,
		Client:   server.Client(),
		Timeout:  time.Second,
	})
	result := dispatcher.Dispatch(context.Background(), Envelope{Event: MessageEvent{
		EventID:        "evt-1",
		EventType:      EventTypeMessageAccepted,
		ConversationID: "single:sender:receiver",
		Seq:            42,
		ServerMsgID:    "msg-42",
		SenderID:       "sender",
		ReceiverID:     "receiver",
		ReceiverIDs:    []string{"receiver"},
		ChatType:       "single",
		ClientMsgID:    "client-42",
		ContentType:    "text",
		Content:        "hello",
		ContentMetadata: map[string]interface{}{
			"encoding": "plain",
		},
		SendTime:  1700000000000,
		CreatedAt: 1700000000100,
		TraceID:   "trace-1",
	}})

	if result.Status != StatusSucceeded || result.Retryable || result.Err != nil {
		t.Fatalf("dispatch result = %+v", result)
	}
	if len(result.DeliveredUserIDs) != 1 || result.DeliveredUserIDs[0] != "receiver" {
		t.Fatalf("delivered user ids = %+v", result.DeliveredUserIDs)
	}
	if got.ConversationID != "single:sender:receiver" {
		t.Fatalf("conversation_id = %q", got.ConversationID)
	}
	if len(got.RecipientUserIDs) != 1 || got.RecipientUserIDs[0] != "receiver" {
		t.Fatalf("recipient_user_ids = %+v", got.RecipientUserIDs)
	}
	if got.Event.Type != delivery.EventMessageReceived {
		t.Fatalf("event type = %q, want %q", got.Event.Type, delivery.EventMessageReceived)
	}
	if got.Event.Data.ServerMsgID != "msg-42" || got.Event.Data.ClientMsgID != "client-42" || got.Event.Data.Seq != 42 {
		t.Fatalf("event message identity mismatch: %+v", got.Event.Data)
	}
	if got.Event.Data.SenderID != "sender" || got.Event.Data.ReceiverID != "receiver" || got.Event.Data.Content != "hello" {
		t.Fatalf("event message payload mismatch: %+v", got.Event.Data)
	}
}

func TestGatewayHTTPDispatcherRetriesServerErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporary failure", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	dispatcher := NewGatewayHTTPDispatcher(GatewayHTTPDispatcherConfig{
		Endpoint: server.URL,
		Client:   server.Client(),
		Timeout:  time.Second,
	})
	result := dispatcher.Dispatch(context.Background(), Envelope{Event: MessageEvent{
		ConversationID: "c1",
		ServerMsgID:    "m1",
		SenderID:       "sender",
		ReceiverIDs:    []string{"receiver"},
	}})

	if result.Status != StatusRetryable || !result.Retryable || result.Err == nil {
		t.Fatalf("dispatch result = %+v, want retryable error", result)
	}
}
