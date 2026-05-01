package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/gateway"
	"github.com/wujunhui99/agents_im/internal/gateway/delivery"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/svc"
)

func TestWebSocketOriginPolicyAllowsSameOriginByDefault(t *testing.T) {
	_, server, cleanup := newWSTestServer(t)
	defer cleanup()

	header := authHeader(t, "usr_origin_same")
	header.Set("Origin", server.URL)
	conn, resp, err := websocket.DefaultDialer.Dial(testWSURL(server.URL, ""), header)
	if err != nil {
		t.Fatalf("same-origin websocket dial: status=%v err=%v", responseStatus(resp), err)
	}
	_ = conn.Close()
}

func TestWebSocketOriginPolicyRejectsCrossOriginByDefault(t *testing.T) {
	_, server, cleanup := newWSTestServer(t)
	defer cleanup()

	header := authHeader(t, "usr_origin_cross")
	header.Set("Origin", "https://evil.example.com")
	conn, resp, err := websocket.DefaultDialer.Dial(testWSURL(server.URL, ""), header)
	if err == nil {
		_ = conn.Close()
		t.Fatal("cross-origin websocket dial succeeded")
	}
	if responseStatus(resp) != http.StatusForbidden {
		t.Fatalf("cross-origin status = %v, want 403", responseStatus(resp))
	}
}

func TestWebSocketOriginPolicyUsesConfiguredExactOrigins(t *testing.T) {
	_, server, cleanup := newWSTestServer(t, WithGatewayWSConfig(config.GatewayWSConfig{
		AllowedOrigins:            []string{"https://chat.example.com"},
		CommandRateLimitPerSecond: 100,
		CommandRateLimitBurst:     100,
	}))
	defer cleanup()

	allowedHeader := authHeader(t, "usr_origin_allowed")
	allowedHeader.Set("Origin", "https://chat.example.com")
	allowedConn, allowedResp, err := websocket.DefaultDialer.Dial(testWSURL(server.URL, ""), allowedHeader)
	if err != nil {
		t.Fatalf("configured origin websocket dial: status=%v err=%v", responseStatus(allowedResp), err)
	}
	_ = allowedConn.Close()

	rejectedHeader := authHeader(t, "usr_origin_rejected")
	rejectedHeader.Set("Origin", server.URL)
	rejectedConn, rejectedResp, err := websocket.DefaultDialer.Dial(testWSURL(server.URL, ""), rejectedHeader)
	if err == nil {
		_ = rejectedConn.Close()
		t.Fatal("unconfigured same-origin dial succeeded while exact origins were configured")
	}
	if responseStatus(rejectedResp) != http.StatusForbidden {
		t.Fatalf("unconfigured origin status = %v, want 403", responseStatus(rejectedResp))
	}
}

func TestWebSocketQueryTokenAuthIsDisabledByDefault(t *testing.T) {
	_, server, cleanup := newWSTestServer(t)
	defer cleanup()

	conn, resp, err := websocket.DefaultDialer.Dial(testWSURL(server.URL, rawTokenForUser(t, "usr_query_default")), nil)
	if err == nil {
		_ = conn.Close()
		t.Fatal("query-token websocket dial succeeded with default config")
	}
	if responseStatus(resp) != http.StatusUnauthorized {
		t.Fatalf("query-token default status = %v, want 401", responseStatus(resp))
	}
}

func TestWebSocketQueryTokenAuthCanBeEnabled(t *testing.T) {
	_, server, cleanup := newWSTestServer(t, WithGatewayWSConfig(config.GatewayWSConfig{
		AllowQueryToken:           true,
		CommandRateLimitPerSecond: 100,
		CommandRateLimitBurst:     100,
	}))
	defer cleanup()

	conn, resp, err := websocket.DefaultDialer.Dial(testWSURL(server.URL, rawTokenForUser(t, "usr_query_enabled")), nil)
	if err != nil {
		t.Fatalf("query-token enabled dial: status=%v err=%v", responseStatus(resp), err)
	}
	_ = conn.Close()
}

func TestWebSocketPingLoopSendsPingFrames(t *testing.T) {
	_, server, cleanup := newWSTestServer(t, WithGatewayWSConfig(config.GatewayWSConfig{
		PingIntervalSeconds:       1,
		HeartbeatTimeoutSeconds:   5,
		CommandRateLimitPerSecond: 100,
		CommandRateLimitBurst:     100,
	}))
	defer cleanup()

	conn, resp, err := websocket.DefaultDialer.Dial(testWSURL(server.URL, ""), authHeader(t, "usr_ping_loop"))
	if err != nil {
		t.Fatalf("dial websocket for ping loop: status=%v err=%v", responseStatus(resp), err)
	}
	defer conn.Close()

	pingSeen := make(chan struct{})
	var once sync.Once
	conn.SetPingHandler(func(appData string) error {
		once.Do(func() { close(pingSeen) })
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	})
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			if _, _, err := conn.NextReader(); err != nil {
				return
			}
		}
	}()

	select {
	case <-pingSeen:
	case <-time.After(2500 * time.Millisecond):
		t.Fatal("timed out waiting for server ping frame")
	}
	_ = conn.Close()
	select {
	case <-readDone:
	case <-time.After(time.Second):
		t.Fatal("ping read loop did not exit after websocket close")
	}
}

func TestWebSocketCommandRateLimitReturnsErrorEnvelope(t *testing.T) {
	_, server, cleanup := newWSTestServer(t, WithGatewayWSConfig(config.GatewayWSConfig{
		PingIntervalSeconds:       30,
		HeartbeatTimeoutSeconds:   75,
		CommandRateLimitPerSecond: 1,
		CommandRateLimitBurst:     1,
	}))
	defer cleanup()

	conn, resp, err := websocket.DefaultDialer.Dial(testWSURL(server.URL, ""), authHeader(t, "usr_rate_limit"))
	if err != nil {
		t.Fatalf("dial websocket for rate limit: status=%v err=%v", responseStatus(resp), err)
	}
	defer conn.Close()

	writeTestCommand(t, conn, "req-rate-1")
	first := readTestResponse(t, conn)
	if first.Status != gateway.AckStatusOK {
		t.Fatalf("first command status = %q, want ok: %+v", first.Status, first)
	}

	writeTestCommand(t, conn, "req-rate-2")
	second := readTestResponse(t, conn)
	if second.Status != gateway.AckStatusError || second.Error == nil {
		t.Fatalf("second command should be rate limited: %+v", second)
	}
	if second.RequestID != "req-rate-2" || second.Type != CommandHeartbeat || second.Error.Code != "RATE_LIMITED" {
		t.Fatalf("unexpected rate limit error envelope: %+v", second)
	}
}

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

func newWSTestServer(t *testing.T, opts ...ServerOption) (*Server, *httptest.Server, func()) {
	t.Helper()
	t.Setenv("GATEWAY_WS_ALLOWED_ORIGINS", "")
	t.Setenv("GATEWAY_WS_ALLOW_QUERY_TOKEN", "")
	t.Setenv("GATEWAY_WS_PING_INTERVAL_SECONDS", "")
	t.Setenv("GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS", "")
	t.Setenv("GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND", "")
	t.Setenv("GATEWAY_WS_COMMAND_RATE_LIMIT_BURST", "")

	serviceContext := svc.NewMessageServiceContextWithAuth(
		repository.NewMemoryMessageRepository(),
		nil,
		nil,
		testAuthConfig(),
	)
	app := NewServer(serviceContext, opts...)
	server := httptest.NewServer(app)
	return app, server, server.Close
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

func authHeader(t *testing.T, userID string) http.Header {
	t.Helper()

	header := http.Header{}
	header.Set("Authorization", "Bearer "+rawTokenForUser(t, userID))
	return header
}

func rawTokenForUser(t *testing.T, userID string) string {
	t.Helper()

	auth := testAuthConfig()
	manager := token.NewHMACTokenManager(auth.AccessSecret, time.Duration(auth.AccessExpire)*time.Second)
	rawToken, _, err := manager.Issue(userID, userID)
	if err != nil {
		t.Fatalf("issue jwt for websocket test: %v", err)
	}
	return rawToken
}

func testAuthConfig() config.JWTAuthConfig {
	return config.JWTAuthConfig{
		AccessSecret: "test-ws-jwt-secret-change-me",
		AccessExpire: 3600,
	}
}

func testWSURL(serverURL string, rawToken string) string {
	u, err := url.Parse(serverURL)
	if err != nil {
		panic(err)
	}
	u.Scheme = "ws"
	u.Path = "/ws"
	if strings.TrimSpace(rawToken) != "" {
		query := u.Query()
		query.Set("token", rawToken)
		u.RawQuery = query.Encode()
	}
	return u.String()
}

func writeTestCommand(t *testing.T, conn *websocket.Conn, requestID string) {
	t.Helper()
	if err := conn.WriteJSON(map[string]interface{}{
		"request_id": requestID,
		"type":       CommandHeartbeat,
		"payload":    map[string]interface{}{},
	}); err != nil {
		t.Fatalf("write websocket test command: %v", err)
	}
}

func readTestResponse(t *testing.T, conn *websocket.Conn) responseFrame {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set websocket read deadline: %v", err)
	}
	var resp responseFrame
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("read websocket test response: %v", err)
	}
	return resp
}

func responseStatus(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
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
