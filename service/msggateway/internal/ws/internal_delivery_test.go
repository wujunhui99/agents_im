package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/gateway/delivery"
)

func TestWebSocketGatewayInternalConversationDeliveryEndpointPushesToOnlineReceiver(t *testing.T) {
	app, wsServer, cleanup := newGatewayWSAppTestServer(t)
	defer cleanup()
	internalMux := http.NewServeMux()
	internalMux.HandleFunc("/internal/delivery/conversation", app.HandleInternalConversationDelivery)
	internalServer := httptest.NewServer(internalMux)
	defer internalServer.Close()

	receiverConn := dialGatewayWS(t, wsServer.URL, "usr_ws_internal_delivery")
	defer receiverConn.Close()
	waitFor(t, func() bool {
		return app.Connections().UserCount("usr_ws_internal_delivery") == 1
	}, "receiver websocket registered")

	response := postJSON(t, internalServer.URL+"/internal/delivery/conversation", map[string]interface{}{
		"conversation_id":    "single:sender:usr_ws_internal_delivery",
		"recipient_user_ids": []string{"usr_ws_internal_delivery"},
		"event": delivery.NewMessageEvent(delivery.EventMessageReceived, delivery.Message{
			ServerMsgID:    "msg_internal_delivery_1",
			ConversationID: "single:sender:usr_ws_internal_delivery",
			Seq:            9,
			SenderID:       "sender",
			ReceiverID:     "usr_ws_internal_delivery",
			ContentType:    "text",
			Content:        "from transfer",
		}),
	})
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("internal delivery status = %d, want 200", response.StatusCode)
	}
	var result delivery.Result
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode delivery result: %v", err)
	}
	if result.DeliveredRecipients != 1 || result.DeliveredConnections != 1 {
		t.Fatalf("unexpected delivery result: %+v", result)
	}

	push := readWSPushEvent(t, receiverConn)
	if push.Type != delivery.EventMessageReceived {
		t.Fatalf("push type = %q, want %q", push.Type, delivery.EventMessageReceived)
	}
	if push.Data.ServerMsgID != "msg_internal_delivery_1" || push.Data.Content != "from transfer" || push.Data.Seq != 9 {
		t.Fatalf("unexpected push payload: %+v", push.Data)
	}
}

func postJSON(t *testing.T, url string, payload interface{}) *http.Response {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("post json: %v", err)
	}
	return response
}
