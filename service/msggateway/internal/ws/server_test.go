package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/auth/token"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/gateway"
	"github.com/wujunhui99/agents_im/pkg/gateway/delivery"
	"github.com/wujunhui99/agents_im/pkg/presence"
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

func TestWebSocketHandshakeRejectsInactiveSessionToken(t *testing.T) {
	auth := testAuthConfig()
	manager := token.NewHMACTokenManager(auth.AccessSecret, time.Duration(auth.AccessExpire)*time.Second)
	sessionStore := newTestSessionStore()
	userID := "usr_ws_active_session"
	const device = "web"
	inactiveToken, _, err := manager.Issue(userID, userID, device, "")
	if err != nil {
		t.Fatalf("issue inactive token: %v", err)
	}
	activeToken, activeClaims, err := manager.Issue(userID, userID, device, "")
	if err != nil {
		t.Fatalf("issue active token: %v", err)
	}
	// Only the active token's jti is registered, so the inactive one must be rejected.
	if err := sessionStore.SetActive(context.Background(), activeClaims.UserID, activeClaims.Device, activeClaims.JTI, time.Hour); err != nil {
		t.Fatalf("store active session: %v", err)
	}

	app, server, cleanup := newWSTestServer(t, WithSessionStore(sessionStore))
	app.tokenManager = manager
	defer cleanup()

	inactiveConn, inactiveResp, err := websocket.DefaultDialer.Dial(testWSURL(server.URL, ""), bearerHeader(inactiveToken))
	if err == nil {
		_ = inactiveConn.Close()
		t.Fatal("inactive-session websocket dial succeeded")
	}
	if responseStatus(inactiveResp) != http.StatusUnauthorized {
		t.Fatalf("inactive-session status = %v, want 401", responseStatus(inactiveResp))
	}

	activeConn, activeResp, err := websocket.DefaultDialer.Dial(testWSURL(server.URL, ""), bearerHeader(activeToken))
	if err != nil {
		t.Fatalf("active-session websocket dial: status=%v err=%v", responseStatus(activeResp), err)
	}
	_ = activeConn.Close()
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

func TestWebSocketLaterConnectionReplacesExistingUserConnection(t *testing.T) {
	app, server, cleanup := newWSTestServer(t, WithGatewayWSConfig(config.GatewayWSConfig{
		PingIntervalSeconds:       30,
		HeartbeatTimeoutSeconds:   75,
		CommandRateLimitPerSecond: 100,
		CommandRateLimitBurst:     100,
	}))
	defer cleanup()

	userID := "usr_single_ws"
	first, firstResp, err := websocket.DefaultDialer.Dial(testWSURL(server.URL, ""), authHeader(t, userID))
	if err != nil {
		t.Fatalf("first websocket dial: status=%v err=%v", responseStatus(firstResp), err)
	}
	defer first.Close()
	firstID := waitForSingleUserConnectionID(t, app, userID)

	second, secondResp, err := websocket.DefaultDialer.Dial(testWSURL(server.URL, ""), authHeader(t, userID))
	if err != nil {
		t.Fatalf("second websocket dial: status=%v err=%v", responseStatus(secondResp), err)
	}
	defer second.Close()

	syncTestWebSocket(t, second, "req-single-ws-replacement-ready")
	secondID := waitForSingleUserConnectionID(t, app, userID)
	if secondID == firstID {
		t.Fatalf("active connection id = %q, want later connection to replace older connection", secondID)
	}

	if err := first.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set first websocket read deadline: %v", err)
	}
	_, _, err = first.ReadMessage()
	if !websocket.IsCloseError(err, CloseCodeSessionReplaced) {
		t.Fatalf("older connection read error = %v, want close code %d", err, CloseCodeSessionReplaced)
	}
}

func TestWebSocketHeartbeatTimeoutUnregistersPresence(t *testing.T) {
	store := presence.NewMemoryStore()
	app, server, cleanup := newWSTestServer(t,
		WithPresenceStore(store),
		WithPresenceTTL(5*time.Second),
		WithGatewayWSConfig(config.GatewayWSConfig{
			PingIntervalSeconds:       30,
			HeartbeatTimeoutSeconds:   1,
			CommandRateLimitPerSecond: 100,
			CommandRateLimitBurst:     100,
		}),
	)
	defer cleanup()

	userID := "usr_timeout_presence"
	conn, resp, err := websocket.DefaultDialer.Dial(testWSURL(server.URL, ""), authHeader(t, userID))
	if err != nil {
		t.Fatalf("websocket dial: status=%v err=%v", responseStatus(resp), err)
	}
	defer conn.Close()

	if !waitForCondition(2*time.Second, func() bool {
		online, err := store.IsUserOnline(context.Background(), userID)
		return err == nil && online && app.Connections().UserCount(userID) == 1
	}) {
		online, _ := store.IsUserOnline(context.Background(), userID)
		t.Fatalf("user should become online after websocket register, online=%v count=%d", online, app.Connections().UserCount(userID))
	}

	if !waitForCondition(4*time.Second, func() bool {
		online, err := store.IsUserOnline(context.Background(), userID)
		return err == nil && !online && app.Connections().UserCount(userID) == 0
	}) {
		online, _ := store.IsUserOnline(context.Background(), userID)
		t.Fatalf("user should be offline after heartbeat timeout, online=%v count=%d", online, app.Connections().UserCount(userID))
	}
}

func TestGatewayInternalUserPresenceEndpointReportsOnlineState(t *testing.T) {
	store := presence.NewMemoryStore()
	app, server, cleanup := newWSTestServer(t,
		WithPresenceStore(store),
		WithPresenceTTL(5*time.Second),
		WithGatewayWSConfig(config.GatewayWSConfig{
			PingIntervalSeconds:       30,
			HeartbeatTimeoutSeconds:   75,
			CommandRateLimitPerSecond: 100,
			CommandRateLimitBurst:     100,
		}),
	)
	defer cleanup()

	internalMux := http.NewServeMux()
	internalMux.HandleFunc("/internal/presence/user", app.HandleInternalUserPresence)
	internalServer := httptest.NewServer(internalMux)
	defer internalServer.Close()

	userID := "usr_presence_endpoint"
	offline := fetchPresenceState(t, internalServer.URL, userID)
	if offline.UserID != userID || offline.Online {
		t.Fatalf("initial presence state = %+v, want offline", offline)
	}

	conn, resp, err := websocket.DefaultDialer.Dial(testWSURL(server.URL, ""), authHeader(t, userID))
	if err != nil {
		t.Fatalf("websocket dial: status=%v err=%v", responseStatus(resp), err)
	}
	defer conn.Close()

	if !waitForCondition(2*time.Second, func() bool {
		return fetchPresenceState(t, internalServer.URL, userID).Online
	}) {
		t.Fatalf("presence endpoint did not report user online")
	}

	_ = conn.Close()
	if !waitForCondition(2*time.Second, func() bool {
		return !fetchPresenceState(t, internalServer.URL, userID).Online
	}) {
		t.Fatalf("presence endpoint did not report user offline after close")
	}
}

func TestSendMessageCommandReturnsACKWithoutLocalFanout(t *testing.T) {
	// 03 §9 A3：send 后 gateway 不做本地 fanout——message_received 由
	// msgtransfer 经 /internal/delivery/conversation 下发，包括 sender 自己。
	server, recorder := newCommandTestServer()
	resp := dispatchSendCommand(t, server, "usr_sender", "req-single-push", map[string]interface{}{
		"chatType":    "single",
		"receiverId":  "usr_receiver",
		"clientMsgId": "client-single-push",
		"contentType": "text",
		"content":     "hello receiver",
	})

	sent := assertSendOK(t, resp, false)
	if sent.Message.SenderID != "usr_sender" || sent.Message.ReceiverID != "usr_receiver" {
		t.Fatalf("send ack message mismatch: %+v", sent.Message)
	}
	if calls := recorder.Calls(); len(calls) != 0 {
		t.Fatalf("delivery calls = %d, want 0 (no gateway-local fanout after A3)", len(calls))
	}
}

func TestSendMessageCommandForwardsAuthenticatedSenderToBackend(t *testing.T) {
	server, _ := newCommandTestServer()
	backend := server.backend.(*fakeBackend)
	dispatchSendCommand(t, server, "usr_sender", "req-sender-bind", map[string]interface{}{
		"chatType":    "single",
		"receiverId":  "usr_receiver",
		"clientMsgId": "client-sender-bind",
		"contentType": "text",
		"content":     "hello",
	})

	sends := backend.SendCalls()
	if len(sends) != 1 {
		t.Fatalf("backend send calls = %d, want 1", len(sends))
	}
	if sends[0].SenderID != "usr_sender" {
		t.Fatalf("backend sender = %q, want authenticated user", sends[0].SenderID)
	}
}

func TestSendMessageDeduplicatedRetryKeepsACKShape(t *testing.T) {
	server, recorder := newCommandTestServer()
	payload := map[string]interface{}{
		"chatType":    "single",
		"receiverId":  "usr_receiver",
		"clientMsgId": "client-dedup-push",
		"contentType": "text",
		"content":     "hello once",
	}

	first := dispatchSendCommand(t, server, "usr_sender", "req-dedup-first", payload)
	assertSendOK(t, first, false)
	second := dispatchSendCommand(t, server, "usr_sender", "req-dedup-second", payload)
	assertSendOK(t, second, true)

	if calls := recorder.Calls(); len(calls) != 0 {
		t.Fatalf("delivery calls after sends = %d, want 0", len(calls))
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

	app := NewServer(testAuthConfig(), newFakeBackend(), opts...)
	server := httptest.NewServer(app)
	return app, server, server.Close
}

func newCommandTestServer() (*Server, *recordingDeliveryDispatcher) {
	recorder := &recordingDeliveryDispatcher{}
	app := NewServer(config.DefaultJWTAuthConfig(), newFakeBackend())
	app.dispatcher = recorder
	return app, recorder
}

type testSessionStore struct {
	active map[string]string
}

func newTestSessionStore() *testSessionStore {
	return &testSessionStore{active: map[string]string{}}
}

func (s *testSessionStore) SetActive(_ context.Context, userID, device, jti string, _ time.Duration) error {
	s.active[userID+"\x00"+device] = jti
	return nil
}

func (s *testSessionStore) Validate(_ context.Context, userID, device, jti string) error {
	if s.active[userID+"\x00"+device] != jti {
		return apperror.Unauthenticated("token session is not active")
	}
	return nil
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

	return bearerHeader(rawTokenForUser(t, userID))
}

func bearerHeader(rawToken string) http.Header {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+rawToken)
	return header
}

func rawTokenForUser(t *testing.T, userID string) string {
	t.Helper()

	auth := testAuthConfig()
	manager := token.NewHMACTokenManager(auth.AccessSecret, time.Duration(auth.AccessExpire)*time.Second)
	rawToken, _, err := manager.Issue(userID, userID, "", "")
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

func syncTestWebSocket(t *testing.T, conn *websocket.Conn, requestID string) {
	t.Helper()

	writeTestCommand(t, conn, requestID)
	resp := readTestResponse(t, conn)
	if resp.RequestID != requestID || resp.Type != CommandHeartbeat || resp.Status != gateway.AckStatusOK {
		t.Fatalf("unexpected websocket sync heartbeat response: %+v", resp)
	}
}

func waitForSingleUserConnectionID(t *testing.T, app *Server, userID string) string {
	t.Helper()

	var ids []string
	if !waitForCondition(2*time.Second, func() bool {
		ids = app.Connections().UserConnectionIDs(userID)
		return len(ids) == 1
	}) {
		t.Fatalf("user connection ids = %+v, want exactly one connection for %s", ids, userID)
	}
	return ids[0]
}

type presenceStateResponse struct {
	UserID string `json:"user_id"`
	Online bool   `json:"online"`
}

func fetchPresenceState(t *testing.T, serverURL string, userID string) presenceStateResponse {
	t.Helper()

	resp, err := http.Get(serverURL + "/internal/presence/user?user_id=" + url.QueryEscape(userID))
	if err != nil {
		t.Fatalf("fetch presence state: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("presence state status = %d", resp.StatusCode)
	}
	var state presenceStateResponse
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		t.Fatalf("decode presence state: %v", err)
	}
	return state
}

func waitForCondition(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return condition()
}

func responseStatus(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
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
