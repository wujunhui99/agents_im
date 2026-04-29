package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wujunhui99/agents_im/internal/gateway"
	"github.com/wujunhui99/agents_im/internal/gateway/delivery"
	gatewayws "github.com/wujunhui99/agents_im/internal/gateway/ws"
	"github.com/wujunhui99/agents_im/internal/presence"
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

const wsReadTimeout = 2 * time.Second

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

func TestWebSocketGatewayRegistersPresenceOnConnect(t *testing.T) {
	store := presence.NewMemoryStore()
	_, server, cleanup := newGatewayWSAppTestServerWithPresence(t, store)
	defer cleanup()

	conn := dialGatewayWS(t, server.URL, "usr_ws_presence_connect")
	defer conn.Close()

	waitFor(t, func() bool {
		connections, err := store.ListUserConnections(context.Background(), "usr_ws_presence_connect")
		return err == nil &&
			len(connections) == 1 &&
			connections[0].ConnectionID != "" &&
			connections[0].InstanceID == "gateway-test" &&
			connections[0].GatewayID == "gateway-test"
	}, "presence registration after websocket connect")
}

func TestWebSocketGatewayHeartbeatRefreshesPresenceTTL(t *testing.T) {
	store := presence.NewMemoryStore()
	_, server, cleanup := newGatewayWSAppTestServerWithPresence(t, store)
	defer cleanup()

	conn := dialGatewayWS(t, server.URL, "usr_ws_presence_heartbeat")
	defer conn.Close()

	var before time.Time
	waitFor(t, func() bool {
		connections, err := store.ListUserConnections(context.Background(), "usr_ws_presence_heartbeat")
		if err != nil || len(connections) != 1 {
			return false
		}
		before = connections[0].LastHeartbeatAt
		return !before.IsZero()
	}, "initial presence heartbeat timestamp")

	time.Sleep(time.Millisecond)
	writeCommand(t, conn, map[string]interface{}{
		"request_id": "req-presence-heartbeat",
		"type":       gatewayws.CommandHeartbeat,
		"payload":    map[string]interface{}{},
	})
	resp := readWSResponse(t, conn)
	if resp.Status != gateway.AckStatusOK {
		t.Fatalf("heartbeat status = %q", resp.Status)
	}

	waitFor(t, func() bool {
		connections, err := store.ListUserConnections(context.Background(), "usr_ws_presence_heartbeat")
		return err == nil && len(connections) == 1 && connections[0].LastHeartbeatAt.After(before)
	}, "presence heartbeat refresh")
}

func TestWebSocketGatewayUnregistersPresenceOnDisconnect(t *testing.T) {
	store := presence.NewMemoryStore()
	_, server, cleanup := newGatewayWSAppTestServerWithPresence(t, store)
	defer cleanup()

	conn := dialGatewayWS(t, server.URL, "usr_ws_presence_disconnect")
	waitFor(t, func() bool {
		online, err := store.IsUserOnline(context.Background(), "usr_ws_presence_disconnect")
		return err == nil && online
	}, "presence online after connect")

	_ = conn.Close()
	waitFor(t, func() bool {
		online, err := store.IsUserOnline(context.Background(), "usr_ws_presence_disconnect")
		return err == nil && !online
	}, "presence unregister after websocket disconnect")
}

func TestWebSocketGatewayPresenceTracksMultipleConnections(t *testing.T) {
	store := presence.NewMemoryStore()
	app, server, cleanup := newGatewayWSAppTestServerWithPresence(t, store)
	defer cleanup()

	connA := dialGatewayWS(t, server.URL, "usr_ws_presence_multi")
	defer connA.Close()
	connB := dialGatewayWS(t, server.URL, "usr_ws_presence_multi")
	defer connB.Close()

	waitFor(t, func() bool {
		connections, err := store.ListUserConnections(context.Background(), "usr_ws_presence_multi")
		return err == nil && len(connections) == 2 && app.Connections().UserCount("usr_ws_presence_multi") == 2
	}, "two websocket presence records")

	result, err := app.PushToUser(context.Background(), "usr_ws_presence_multi", delivery.NewMessageEvent(delivery.EventMessageReceived, delivery.Message{
		ServerMsgID:    "msg_presence_multi_1",
		ConversationID: "single:sender:usr_ws_presence_multi",
		Seq:            1,
		SenderID:       "sender",
		ReceiverID:     "usr_ws_presence_multi",
		ContentType:    "text",
	}))
	if err != nil {
		t.Fatalf("push with multiple presence connections: %v", err)
	}
	if result.DeliveredConnections != 2 || len(result.Recipients) != 1 || len(result.Recipients[0].Routes) != 2 {
		t.Fatalf("unexpected multiple connection routing result: %+v", result)
	}
	_ = readWSPushEvent(t, connA)
	_ = readWSPushEvent(t, connB)
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

func TestWebSocketGatewayPushFanoutDeliversToAllUserConnections(t *testing.T) {
	app, server, cleanup := newGatewayWSAppTestServer(t)
	defer cleanup()

	connA := dialGatewayWS(t, server.URL, "usr_ws_push")
	defer connA.Close()
	connB := dialGatewayWS(t, server.URL, "usr_ws_push")
	defer connB.Close()
	waitFor(t, func() bool {
		return app.Connections().UserCount("usr_ws_push") == 2
	}, "two websocket connections registered")

	event := delivery.NewMessageEvent(delivery.EventMessageReceived, delivery.Message{
		ServerMsgID:    "msg_push_1",
		ConversationID: "single:usr_ws_sender:usr_ws_push",
		Seq:            7,
		SenderID:       "usr_ws_sender",
		ReceiverID:     "usr_ws_push",
		ChatType:       "single",
		ContentType:    "text",
		Content:        "pushed message",
		ContentMetadata: map[string]interface{}{
			"encoding": "plain",
		},
	})
	result, err := app.PushToUser(context.Background(), "usr_ws_push", event)
	if err != nil {
		t.Fatalf("push to user: %v", err)
	}
	if result.DeliveredRecipients != 1 || result.DeliveredConnections != 2 || result.OfflineRecipients != 0 {
		t.Fatalf("unexpected push result: %+v", result)
	}

	pushA := readWSPushEvent(t, connA)
	pushB := readWSPushEvent(t, connB)
	for name, got := range map[string]delivery.Event{"connA": pushA, "connB": pushB} {
		if got.Type != delivery.EventMessageReceived {
			t.Fatalf("%s push type = %q, want %q", name, got.Type, delivery.EventMessageReceived)
		}
		if got.Data.ServerMsgID != "msg_push_1" || got.Data.ConversationID != "single:usr_ws_sender:usr_ws_push" || got.Data.Seq != 7 {
			t.Fatalf("%s push message mismatch: %+v", name, got.Data)
		}
		if got.Data.SenderID != "usr_ws_sender" || got.Data.ContentType != "text" || got.Data.ContentMetadata["encoding"] != "plain" {
			t.Fatalf("%s push content metadata mismatch: %+v", name, got.Data)
		}
	}
}

func TestWebSocketGatewayPushOfflineUserReturnsOfflineStatus(t *testing.T) {
	app := newGatewayWSApp(t)

	result, err := app.PushToUser(context.Background(), "usr_ws_offline", delivery.NewMessageEvent(delivery.EventMessageReceived, delivery.Message{
		ServerMsgID:    "msg_offline_1",
		ConversationID: "single:usr_ws_sender:usr_ws_offline",
		Seq:            1,
		SenderID:       "usr_ws_sender",
		ReceiverID:     "usr_ws_offline",
		ContentType:    "text",
	}))
	if err != nil {
		t.Fatalf("push to offline user returned error: %v", err)
	}
	if result.OfflineRecipients != 1 || result.DeliveredConnections != 0 || len(result.Recipients) != 1 {
		t.Fatalf("unexpected offline result: %+v", result)
	}
	if result.Recipients[0].Status != delivery.StatusOffline {
		t.Fatalf("recipient status = %q, want %q", result.Recipients[0].Status, delivery.StatusOffline)
	}
}

func TestWebSocketGatewayPushReturnsRoutedForRemotePresence(t *testing.T) {
	store := presence.NewMemoryStore()
	app := newGatewayWSAppWithPresence(t, store)
	err := store.RegisterConnection(context.Background(), presence.ConnectionMetadata{
		UserID:       "usr_ws_remote_route",
		ConnectionID: "conn_remote_1",
		InstanceID:   "gateway-remote",
		GatewayID:    "gateway-remote",
		Platform:     "web",
	}, time.Minute)
	if err != nil {
		t.Fatalf("register remote presence route: %v", err)
	}

	result, err := app.PushToUser(context.Background(), "usr_ws_remote_route", delivery.NewMessageEvent(delivery.EventMessageReceived, delivery.Message{
		ServerMsgID:    "msg_remote_route_1",
		ConversationID: "single:sender:usr_ws_remote_route",
		Seq:            1,
		SenderID:       "sender",
		ReceiverID:     "usr_ws_remote_route",
		ContentType:    "text",
	}))
	if err != nil {
		t.Fatalf("remote routed result should not be an error: %v", err)
	}
	if result.RoutedRecipients != 1 || len(result.Recipients) != 1 || result.Recipients[0].Status != delivery.StatusRouted {
		t.Fatalf("unexpected routed result: %+v", result)
	}
	if len(result.Recipients[0].Routes) != 1 || result.Recipients[0].Routes[0].Local {
		t.Fatalf("unexpected route metadata: %+v", result.Recipients[0].Routes)
	}
}

func TestWebSocketGatewayPushReportsPresenceLookupFailure(t *testing.T) {
	store := failingPresenceStore{err: errors.New("presence unavailable")}
	app, server, cleanup := newGatewayWSAppTestServerWithPresence(t, store)
	defer cleanup()

	conn := dialGatewayWS(t, server.URL, "usr_ws_presence_failure")
	defer conn.Close()
	waitFor(t, func() bool {
		return app.Connections().UserCount("usr_ws_presence_failure") == 1
	}, "local websocket connection despite presence lookup failure")

	result, err := app.PushToUser(context.Background(), "usr_ws_presence_failure", delivery.NewMessageEvent(delivery.EventMessageReceived, delivery.Message{
		ServerMsgID:    "msg_presence_failure_1",
		ConversationID: "single:sender:usr_ws_presence_failure",
		Seq:            1,
		SenderID:       "sender",
		ReceiverID:     "usr_ws_presence_failure",
		ContentType:    "text",
	}))
	if err == nil {
		t.Fatal("expected presence lookup failure")
	}
	if result.FailedRecipients != 1 || len(result.Recipients) != 1 || result.Recipients[0].Status != delivery.StatusFailed {
		t.Fatalf("unexpected presence failure result: %+v", result)
	}
}

func TestWebSocketGatewayPushDoesNotBreakCommandResponses(t *testing.T) {
	app, server, cleanup := newGatewayWSAppTestServer(t)
	defer cleanup()

	conn := dialGatewayWS(t, server.URL, "usr_ws_push_command")
	defer conn.Close()
	waitFor(t, func() bool {
		return app.Connections().UserCount("usr_ws_push_command") == 1
	}, "websocket connection registered before push command test")

	_, err := app.PushToUser(context.Background(), "usr_ws_push_command", delivery.NewMessageEvent(delivery.EventMessageReceived, delivery.Message{
		ServerMsgID:    "msg_push_command_1",
		ConversationID: "single:usr_ws_sender:usr_ws_push_command",
		Seq:            3,
		SenderID:       "usr_ws_sender",
		ReceiverID:     "usr_ws_push_command",
		ContentType:    "text",
		Content:        "before heartbeat",
	}))
	if err != nil {
		t.Fatalf("push before command response: %v", err)
	}
	push := readWSPushEvent(t, conn)
	if push.Type != delivery.EventMessageReceived || push.Data.ServerMsgID != "msg_push_command_1" {
		t.Fatalf("unexpected push event before command: %+v", push)
	}

	writeCommand(t, conn, map[string]interface{}{
		"request_id": "req-heartbeat-after-push",
		"type":       gatewayws.CommandHeartbeat,
		"payload":    map[string]interface{}{},
	})
	resp := readWSResponse(t, conn)
	if resp.RequestID != "req-heartbeat-after-push" || resp.Type != gatewayws.CommandHeartbeat || resp.Status != gateway.AckStatusOK {
		t.Fatalf("unexpected heartbeat response after push: %+v", resp)
	}
}

func TestWebSocketGatewayConnectionCloseCleansManager(t *testing.T) {
	app, server, cleanup := newGatewayWSAppTestServer(t)
	defer cleanup()

	conn := dialGatewayWS(t, server.URL, "usr_ws_close")
	waitFor(t, func() bool {
		return app.Connections().UserCount("usr_ws_close") == 1
	}, "websocket connection registered")

	_ = conn.Close()
	waitFor(t, func() bool {
		return app.Connections().UserCount("usr_ws_close") == 0 && app.Connections().Count() == 0
	}, "websocket connection unregistered after close")
}

func newGatewayWSTestServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()

	_, server, cleanup := newGatewayWSAppTestServer(t)
	return server, cleanup
}

func newGatewayWSAppTestServer(t *testing.T) (*gatewayws.Server, *httptest.Server, func()) {
	t.Helper()

	app := newGatewayWSApp(t)
	server := httptest.NewServer(app)
	return app, server, server.Close
}

func newGatewayWSAppTestServerWithPresence(t *testing.T, store presence.PresenceStore) (*gatewayws.Server, *httptest.Server, func()) {
	t.Helper()

	app := newGatewayWSAppWithPresence(t, store)
	server := httptest.NewServer(app)
	return app, server, server.Close
}

func newGatewayWSApp(t *testing.T) *gatewayws.Server {
	t.Helper()

	return newGatewayWSAppWithPresence(t, presence.NewMemoryStore())
}

func newGatewayWSAppWithPresence(t *testing.T, store presence.PresenceStore) *gatewayws.Server {
	t.Helper()

	serviceContext := svc.NewMessageServiceContextWithAuth(
		repository.NewMemoryMessageRepository(),
		nil,
		nil,
		testJWTAuthConfig(),
	)
	return gatewayws.NewServer(
		serviceContext,
		gatewayws.WithPresenceStore(store),
		gatewayws.WithPresenceTTL(time.Minute),
		gatewayws.WithInstanceID("gateway-test"),
	)
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

	setReadDeadline(t, conn)
	var resp wsResponse
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("read websocket response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("websocket command returned error: %+v", resp.Error)
	}
	return resp
}

func readWSPushEvent(t *testing.T, conn *websocket.Conn) delivery.Event {
	t.Helper()

	setReadDeadline(t, conn)
	var event delivery.Event
	if err := conn.ReadJSON(&event); err != nil {
		t.Fatalf("read websocket push event: %v", err)
	}
	return event
}

func setReadDeadline(t *testing.T, conn *websocket.Conn) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(wsReadTimeout)); err != nil {
		t.Fatalf("set websocket read deadline: %v", err)
	}
}

func waitFor(t *testing.T, condition func() bool, description string) {
	t.Helper()

	deadline := time.Now().Add(wsReadTimeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", description)
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

type failingPresenceStore struct {
	err error
}

func (s failingPresenceStore) RegisterConnection(context.Context, presence.ConnectionMetadata, time.Duration) error {
	return nil
}

func (s failingPresenceStore) Heartbeat(context.Context, string, string, time.Duration) error {
	return nil
}

func (s failingPresenceStore) UnregisterConnection(context.Context, string, string) error {
	return nil
}

func (s failingPresenceStore) ListUserConnections(context.Context, string) ([]presence.ConnectionMetadata, error) {
	return nil, s.err
}

func (s failingPresenceStore) IsUserOnline(context.Context, string) (bool, error) {
	return false, s.err
}
