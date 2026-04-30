package ws

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/gateway"
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
