package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/wujunhui99/agents_im/internal/gateway"
	gatewayws "github.com/wujunhui99/agents_im/internal/gateway/ws"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/svc"
)

type wsResponse struct {
	RequestID string          `json:"request_id"`
	Type      string          `json:"type"`
	Status    string          `json:"status"`
	Error     *wsResponseErr  `json:"error,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type wsResponseErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func TestWebSocketGatewayRejectsMissingAndInvalidToken(t *testing.T) {
	server, cleanup := newGatewayWSTestServer(t)
	defer cleanup()

	_, missingResp, missingErr := websocket.DefaultDialer.Dial(wsURL(server.URL, ""), nil)
	if missingErr == nil {
		t.Fatal("missing token dial succeeded")
	}
	if missingResp == nil || missingResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing token status = %v, want 401", responseStatus(missingResp))
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer invalid-token")
	_, invalidResp, invalidErr := websocket.DefaultDialer.Dial(wsURL(server.URL, ""), header)
	if invalidErr == nil {
		t.Fatal("invalid token dial succeeded")
	}
	if invalidResp == nil || invalidResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("invalid token status = %v, want 401", responseStatus(invalidResp))
	}
}

func TestWebSocketGatewayAcceptsValidTokenFromHeaderAndQuery(t *testing.T) {
	server, cleanup := newGatewayWSTestServer(t)
	defer cleanup()

	header := http.Header{}
	header.Set("Authorization", bearerTokenForUser(t, "usr_ws_header"))
	headerConn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL, ""), header)
	if err != nil {
		t.Fatalf("dial with authorization header: %v", err)
	}
	_ = headerConn.Close()

	rawToken := strings.TrimPrefix(bearerTokenForUser(t, "usr_ws_query"), "Bearer ")
	queryConn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL, rawToken), nil)
	if err != nil {
		t.Fatalf("dial with token query param: %v", err)
	}
	_ = queryConn.Close()
}

func TestWebSocketGatewayHeartbeatReturnsOK(t *testing.T) {
	server, cleanup := newGatewayWSTestServer(t)
	defer cleanup()

	conn := dialGatewayWS(t, server.URL, "usr_ws_heartbeat")
	defer conn.Close()

	writeCommand(t, conn, map[string]interface{}{
		"request_id": "req-heartbeat",
		"type":       gatewayws.CommandHeartbeat,
		"payload":    map[string]interface{}{},
	})

	resp := readWSResponse(t, conn)
	if resp.RequestID != "req-heartbeat" || resp.Type != gatewayws.CommandHeartbeat || resp.Status != gateway.AckStatusOK {
		t.Fatalf("unexpected heartbeat response envelope: %+v", resp)
	}

	var data struct {
		ConnectionID string `json:"connection_id"`
		UserID       string `json:"user_id"`
		ServerTime   int64  `json:"server_time"`
	}
	decodeRaw(t, resp.Data, &data)
	if data.ConnectionID == "" || data.UserID != "usr_ws_heartbeat" || data.ServerTime == 0 {
		t.Fatalf("unexpected heartbeat data: %+v", data)
	}
}

func TestWebSocketGatewaySendAndPullMessages(t *testing.T) {
	server, cleanup := newGatewayWSTestServer(t)
	defer cleanup()

	conn := dialGatewayWS(t, server.URL, "usr_ws_sender")
	defer conn.Close()

	writeCommand(t, conn, map[string]interface{}{
		"request_id": "req-send",
		"type":       gateway.CommandSendMessage,
		"payload": map[string]interface{}{
			"chatType":    "single",
			"receiverId":  "usr_ws_receiver",
			"clientMsgId": "client-ws-1",
			"contentType": "text",
			"content":     "hello over websocket",
		},
	})

	sendResp := readWSResponse(t, conn)
	if sendResp.Status != gateway.AckStatusOK || sendResp.Type != gateway.CommandSendMessage {
		t.Fatalf("unexpected send response envelope: %+v", sendResp)
	}
	var sent gateway.SendMessageCommandResponse
	decodeRaw(t, sendResp.Data, &sent)
	if sent.Message.ServerMsgID == "" || sent.Message.Seq != 1 {
		t.Fatalf("unexpected sent message ids: %+v", sent.Message)
	}
	if sent.Message.SenderID != "usr_ws_sender" || sent.Message.ReceiverID != "usr_ws_receiver" {
		t.Fatalf("send did not use websocket token user: %+v", sent.Message)
	}

	writeCommand(t, conn, map[string]interface{}{
		"request_id": "req-pull",
		"type":       gateway.CommandPullMessages,
		"payload": map[string]interface{}{
			"conversationId": sent.Message.ConversationID,
			"fromSeq":        1,
			"toSeq":          1,
			"limit":          10,
			"order":          "asc",
		},
	})

	pullResp := readWSResponse(t, conn)
	if pullResp.Status != gateway.AckStatusOK || pullResp.Type != gateway.CommandPullMessages {
		t.Fatalf("unexpected pull response envelope: %+v", pullResp)
	}
	var pulled gateway.PullMessagesCommandResponse
	decodeRaw(t, pullResp.Data, &pulled)
	if len(pulled.Messages) != 1 {
		t.Fatalf("pulled %d messages, want 1: %+v", len(pulled.Messages), pulled.Messages)
	}
	if pulled.Messages[0].ServerMsgID != sent.Message.ServerMsgID || pulled.Messages[0].Seq != 1 {
		t.Fatalf("pulled wrong message: %+v, sent %+v", pulled.Messages[0], sent.Message)
	}
}

func newGatewayWSTestServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()

	serviceContext := svc.NewMessageServiceContextWithAuth(
		repository.NewMemoryMessageRepository(),
		nil,
		nil,
		testJWTAuthConfig(),
	)
	server := httptest.NewServer(gatewayws.NewServer(serviceContext))
	return server, server.Close
}

func dialGatewayWS(t *testing.T, serverURL string, userID string) *websocket.Conn {
	t.Helper()

	header := http.Header{}
	header.Set("Authorization", bearerTokenForUser(t, userID))
	conn, _, err := websocket.DefaultDialer.Dial(wsURL(serverURL, ""), header)
	if err != nil {
		t.Fatalf("dial websocket gateway: %v", err)
	}
	return conn
}

func wsURL(serverURL string, rawToken string) string {
	u, err := url.Parse(serverURL)
	if err != nil {
		panic(err)
	}
	u.Scheme = "ws"
	u.Path = "/ws"
	if rawToken != "" {
		query := u.Query()
		query.Set("token", rawToken)
		u.RawQuery = query.Encode()
	}
	return u.String()
}

func writeCommand(t *testing.T, conn *websocket.Conn, command map[string]interface{}) {
	t.Helper()

	if err := conn.WriteJSON(command); err != nil {
		t.Fatalf("write websocket command: %v", err)
	}
}

func readWSResponse(t *testing.T, conn *websocket.Conn) wsResponse {
	t.Helper()

	var resp wsResponse
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("read websocket response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("websocket command returned error: %+v", resp.Error)
	}
	return resp
}

func decodeRaw(t *testing.T, raw json.RawMessage, dst interface{}) {
	t.Helper()

	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("decode websocket response data: %v; raw=%s", err, string(raw))
	}
}

func responseStatus(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}
