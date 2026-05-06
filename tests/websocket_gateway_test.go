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
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/gateway"
	"github.com/wujunhui99/agents_im/internal/gateway/delivery"
	gatewayws "github.com/wujunhui99/agents_im/internal/gateway/ws"
	"github.com/wujunhui99/agents_im/internal/presence"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/svc"
)

type wsResponse struct {
	RequestID      string          `json:"request_id"`
	RequestIDCamel string          `json:"requestId"`
	TraceID        string          `json:"trace_id"`
	TraceIDCamel   string          `json:"traceId"`
	Type           string          `json:"type"`
	Command        string          `json:"command"`
	Status         string          `json:"status"`
	Error          *wsResponseErr  `json:"error,omitempty"`
	Data           json.RawMessage `json:"data,omitempty"`
	Payload        json.RawMessage `json:"payload,omitempty"`
}

type wsResponseErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (r wsResponse) frontendRequestID() string {
	if r.RequestIDCamel != "" {
		return r.RequestIDCamel
	}
	return r.RequestID
}

func (r wsResponse) commandName() string {
	if r.Command != "" {
		return r.Command
	}
	return r.Type
}

func (r wsResponse) responsePayload() json.RawMessage {
	if len(r.Payload) > 0 {
		return r.Payload
	}
	return r.Data
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

func TestWebSocketGatewayAcceptsValidTokenFromHeaderAndConfiguredQuery(t *testing.T) {
	server, cleanup := newGatewayWSTestServer(t)
	defer cleanup()

	header := http.Header{}
	header.Set("Authorization", bearerTokenForUser(t, "usr_ws_header"))
	headerConn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL, ""), header)
	if err != nil {
		t.Fatalf("dial with authorization header: %v", err)
	}
	_ = headerConn.Close()

	queryServer, queryCleanup := newGatewayWSTestServer(t, gatewayws.WithGatewayWSConfig(config.GatewayWSConfig{
		AllowQueryToken:           true,
		CommandRateLimitPerSecond: 100,
		CommandRateLimitBurst:     100,
	}))
	defer queryCleanup()

	rawToken := strings.TrimPrefix(bearerTokenForUser(t, "usr_ws_query"), "Bearer ")
	queryConn, _, err := websocket.DefaultDialer.Dial(wsURL(queryServer.URL, rawToken), nil)
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
	if resp.TraceID == "" {
		t.Fatalf("heartbeat response should include trace_id: %+v", resp)
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

func TestWebSocketGatewayPresenceKeepsOnlyLatestConnection(t *testing.T) {
	store := presence.NewMemoryStore()
	app, server, cleanup := newGatewayWSAppTestServerWithPresence(t, store)
	defer cleanup()

	connA := dialGatewayWS(t, server.URL, "usr_ws_presence_multi")
	defer connA.Close()
	waitFor(t, func() bool {
		return app.Connections().UserCount("usr_ws_presence_multi") == 1
	}, "initial websocket connection registered")

	connB := dialGatewayWS(t, server.URL, "usr_ws_presence_multi")
	defer connB.Close()
	syncGatewayWSConnection(t, connB, "req-presence-multi-ready")

	waitFor(t, func() bool {
		connections, err := store.ListUserConnections(context.Background(), "usr_ws_presence_multi")
		return err == nil && len(connections) == 1 && app.Connections().UserCount("usr_ws_presence_multi") == 1
	}, "latest websocket presence record")

	if err := connA.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set replaced connection deadline: %v", err)
	}
	_, _, err := connA.ReadMessage()
	if !websocket.IsCloseError(err, gatewayws.CloseCodeSessionReplaced) {
		t.Fatalf("replaced connection read error = %v, want close code %d", err, gatewayws.CloseCodeSessionReplaced)
	}

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
	if result.DeliveredConnections != 1 || len(result.Recipients) != 1 || len(result.Recipients[0].Routes) != 1 {
		t.Fatalf("unexpected single connection routing result: %+v", result)
	}
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

func TestWebSocketGatewaySendMessagePushesToOnlineReceiver(t *testing.T) {
	app, server, cleanup := newGatewayWSAppTestServer(t)
	defer cleanup()

	senderConn := dialGatewayWS(t, server.URL, "usr_ws_live_sender")
	defer senderConn.Close()
	receiverConn := dialGatewayWS(t, server.URL, "usr_ws_live_receiver")
	defer receiverConn.Close()
	waitFor(t, func() bool {
		return app.Connections().UserCount("usr_ws_live_receiver") == 1
	}, "receiver websocket registered before live-push send")

	sent := sendWSMessage(t, senderConn, "req-live-send", "usr_ws_live_receiver", "client-live-1", "live hello")
	push := readWSPushEvent(t, receiverConn)
	if push.Type != delivery.EventMessageReceived {
		t.Fatalf("push type = %q, want %q", push.Type, delivery.EventMessageReceived)
	}
	if push.Data.ServerMsgID != sent.Message.ServerMsgID || push.Data.ConversationID != sent.Message.ConversationID || push.Data.Seq != sent.Message.Seq {
		t.Fatalf("push message identity mismatch: push=%+v sent=%+v", push.Data, sent.Message)
	}
	if push.Data.SenderID != "usr_ws_live_sender" || push.Data.ReceiverID != "usr_ws_live_receiver" || push.Data.Content != "live hello" {
		t.Fatalf("push message payload mismatch: %+v", push.Data)
	}
}

func TestWebSocketGatewayReconnectSyncFlow(t *testing.T) {
	server, cleanup := newGatewayWSTestServer(t)
	defer cleanup()

	senderConn := dialGatewayWS(t, server.URL, "usr_ws_sync_sender")
	defer senderConn.Close()

	first := sendWSMessage(t, senderConn, "req-sync-send-1", "usr_ws_sync_receiver", "client-sync-1", "one")
	second := sendWSMessage(t, senderConn, "req-sync-send-2", "usr_ws_sync_receiver", "client-sync-2", "two")
	conversationID := first.Message.ConversationID
	if second.Message.ConversationID != conversationID || second.Message.Seq != 2 {
		t.Fatalf("unexpected sync setup messages: first=%+v second=%+v", first.Message, second.Message)
	}

	receiverConn := dialGatewayWS(t, server.URL, "usr_ws_sync_receiver")
	defer receiverConn.Close()

	writeCommand(t, receiverConn, map[string]interface{}{
		"requestId": "req-sync-seqs",
		"command":   gateway.CommandGetConversationSeqs,
		"payload":   map[string]interface{}{},
	})
	seqsResp := readWSResponse(t, receiverConn)
	if seqsResp.frontendRequestID() != "req-sync-seqs" || seqsResp.commandName() != gateway.CommandGetConversationSeqs {
		t.Fatalf("unexpected seqs response envelope: %+v", seqsResp)
	}
	var seqs gateway.GetConversationSeqsCommandResponse
	decodeRaw(t, seqsResp.responsePayload(), &seqs)
	if len(seqs.States) != 1 {
		t.Fatalf("got %d conversation states, want 1: %+v", len(seqs.States), seqs.States)
	}
	state := seqs.States[0]
	if state.ConversationID != conversationID || state.MaxSeq != 2 || state.HasReadSeq != 0 || state.UnreadCount != 2 {
		t.Fatalf("unexpected reconnect seq state: %+v", state)
	}
	if state.LastMessage == nil || state.LastMessage.Seq != 2 || state.LastMessage.ServerMsgID != second.Message.ServerMsgID {
		t.Fatalf("unexpected reconnect last message: %+v", state.LastMessage)
	}

	writeCommand(t, receiverConn, map[string]interface{}{
		"requestId": "req-sync-pull",
		"command":   gateway.CommandPullMessages,
		"payload": map[string]interface{}{
			"conversationId": conversationID,
			"fromSeq":        int64(1),
			"toSeq":          state.MaxSeq,
			"limit":          int32(10),
			"order":          "asc",
		},
	})
	pullResp := readWSResponse(t, receiverConn)
	if pullResp.frontendRequestID() != "req-sync-pull" || pullResp.commandName() != gateway.CommandPullMessages {
		t.Fatalf("unexpected pull response envelope: %+v", pullResp)
	}
	var pulled gateway.PullMessagesCommandResponse
	decodeRaw(t, pullResp.responsePayload(), &pulled)
	if len(pulled.Messages) != 2 || pulled.Messages[0].Seq != 1 || pulled.Messages[1].Seq != 2 {
		t.Fatalf("unexpected reconnect pull payload: %+v", pulled)
	}

	writeCommand(t, receiverConn, map[string]interface{}{
		"requestId": "req-sync-read",
		"command":   gateway.CommandMarkConversationRead,
		"payload": map[string]interface{}{
			"conversationId": conversationID,
			"hasReadSeq":     state.MaxSeq,
		},
	})
	readResp := readWSResponse(t, receiverConn)
	var marked gateway.MarkConversationReadCommandResponse
	decodeRaw(t, readResp.responsePayload(), &marked)
	if marked.ConversationID != conversationID || marked.HasReadSeq != 2 || marked.UnreadCount != 0 || !marked.Updated {
		t.Fatalf("unexpected reconnect read payload: %+v", marked)
	}
}

func TestWebSocketGatewayPullMessagesIsDuplicateSafe(t *testing.T) {
	server, cleanup := newGatewayWSTestServer(t)
	defer cleanup()

	senderConn := dialGatewayWS(t, server.URL, "usr_ws_dup_sender")
	defer senderConn.Close()
	first := sendWSMessage(t, senderConn, "req-dup-send-1", "usr_ws_dup_receiver", "client-dup-1", "one")
	second := sendWSMessage(t, senderConn, "req-dup-send-2", "usr_ws_dup_receiver", "client-dup-2", "two")

	receiverConn := dialGatewayWS(t, server.URL, "usr_ws_dup_receiver")
	defer receiverConn.Close()

	firstPull := pullWSMessages(t, receiverConn, "req-dup-pull-1", first.Message.ConversationID, 1, 2)
	secondPull := pullWSMessages(t, receiverConn, "req-dup-pull-2", first.Message.ConversationID, 1, 2)
	if len(firstPull.Messages) != 2 || len(secondPull.Messages) != 2 {
		t.Fatalf("unexpected duplicate pull sizes: first=%+v second=%+v", firstPull, secondPull)
	}
	if firstPull.Messages[0].ServerMsgID != secondPull.Messages[0].ServerMsgID ||
		firstPull.Messages[1].ServerMsgID != secondPull.Messages[1].ServerMsgID ||
		firstPull.Messages[0].ServerMsgID != first.Message.ServerMsgID ||
		firstPull.Messages[1].ServerMsgID != second.Message.ServerMsgID {
		t.Fatalf("duplicate pull changed message identity: first=%+v second=%+v", firstPull.Messages, secondPull.Messages)
	}

	writeCommand(t, receiverConn, map[string]interface{}{
		"requestId": "req-dup-seqs",
		"command":   gateway.CommandGetConversationSeqs,
		"payload": map[string]interface{}{
			"conversationIds": []string{first.Message.ConversationID},
		},
	})
	seqsResp := readWSResponse(t, receiverConn)
	var seqs gateway.GetConversationSeqsCommandResponse
	decodeRaw(t, seqsResp.responsePayload(), &seqs)
	if len(seqs.States) != 1 || seqs.States[0].HasReadSeq != 0 || seqs.States[0].UnreadCount != 2 {
		t.Fatalf("pull should not mark read: %+v", seqs.States)
	}
}

func TestWebSocketGatewayPullMessagesFromMissingSeq(t *testing.T) {
	server, cleanup := newGatewayWSTestServer(t)
	defer cleanup()

	senderConn := dialGatewayWS(t, server.URL, "usr_ws_missing_sender")
	defer senderConn.Close()
	first := sendWSMessage(t, senderConn, "req-missing-send-1", "usr_ws_missing_receiver", "client-missing-1", "one")
	second := sendWSMessage(t, senderConn, "req-missing-send-2", "usr_ws_missing_receiver", "client-missing-2", "two")
	third := sendWSMessage(t, senderConn, "req-missing-send-3", "usr_ws_missing_receiver", "client-missing-3", "three")

	receiverConn := dialGatewayWS(t, server.URL, "usr_ws_missing_receiver")
	defer receiverConn.Close()

	pulled := pullWSMessages(t, receiverConn, "req-missing-pull", first.Message.ConversationID, 2, 3)
	if len(pulled.Messages) != 2 {
		t.Fatalf("pulled %d messages, want missing seqs 2 and 3: %+v", len(pulled.Messages), pulled.Messages)
	}
	if pulled.Messages[0].Seq != 2 || pulled.Messages[0].ServerMsgID != second.Message.ServerMsgID ||
		pulled.Messages[1].Seq != 3 || pulled.Messages[1].ServerMsgID != third.Message.ServerMsgID {
		t.Fatalf("unexpected missing seq pull payload: %+v", pulled.Messages)
	}
	if pulled.Messages[0].ServerMsgID == first.Message.ServerMsgID {
		t.Fatalf("missing seq pull returned already-local seq 1: %+v", pulled.Messages)
	}
}

func TestWebSocketGatewayInvalidCommandReturnsFrontendErrorEnvelope(t *testing.T) {
	server, cleanup := newGatewayWSTestServer(t)
	defer cleanup()

	conn := dialGatewayWS(t, server.URL, "usr_ws_invalid_command")
	defer conn.Close()

	writeCommand(t, conn, map[string]interface{}{
		"requestId": "req-invalid-command",
		"command":   "unknown_command",
		"payload":   map[string]interface{}{},
	})

	resp := readWSResponseAllowError(t, conn)
	if resp.frontendRequestID() != "req-invalid-command" {
		t.Fatalf("requestId = %q, want req-invalid-command; full response=%+v", resp.frontendRequestID(), resp)
	}
	if resp.Status != gateway.AckStatusError {
		t.Fatalf("status = %q, want error; full response=%+v", resp.Status, resp)
	}
	if resp.Error == nil {
		t.Fatalf("missing error object: %+v", resp)
	}
	if resp.Error.Code != "VALIDATION_ERROR" || resp.Error.Message == "" {
		t.Fatalf("unexpected error envelope: %+v", resp.Error)
	}
	if len(resp.Payload) != 0 {
		t.Fatalf("error response should not include payload: %s", string(resp.Payload))
	}
}

func TestWebSocketGatewayPushDeliversToLatestUserConnectionOnly(t *testing.T) {
	app, server, cleanup := newGatewayWSAppTestServer(t)
	defer cleanup()

	connA := dialGatewayWS(t, server.URL, "usr_ws_push")
	defer connA.Close()
	waitFor(t, func() bool {
		return app.Connections().UserCount("usr_ws_push") == 1
	}, "initial websocket connection registered")

	connB := dialGatewayWS(t, server.URL, "usr_ws_push")
	defer connB.Close()
	syncGatewayWSConnection(t, connB, "req-push-latest-ready")
	waitFor(t, func() bool {
		return app.Connections().UserCount("usr_ws_push") == 1
	}, "latest websocket connection registered")

	if err := connA.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set replaced connection deadline: %v", err)
	}
	_, _, err := connA.ReadMessage()
	if !websocket.IsCloseError(err, gatewayws.CloseCodeSessionReplaced) {
		t.Fatalf("replaced connection read error = %v, want close code %d", err, gatewayws.CloseCodeSessionReplaced)
	}

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
	if result.DeliveredRecipients != 1 || result.DeliveredConnections != 1 || result.OfflineRecipients != 0 {
		t.Fatalf("unexpected push result: %+v", result)
	}

	push := readWSPushEvent(t, connB)
	if push.Type != delivery.EventMessageReceived {
		t.Fatalf("push type = %q, want %q", push.Type, delivery.EventMessageReceived)
	}
	if push.Data.ServerMsgID != "msg_push_1" || push.Data.ConversationID != "single:usr_ws_sender:usr_ws_push" || push.Data.Seq != 7 {
		t.Fatalf("push message mismatch: %+v", push.Data)
	}
	if push.Data.SenderID != "usr_ws_sender" || push.Data.ContentType != "text" || push.Data.ContentMetadata["encoding"] != "plain" {
		t.Fatalf("push content metadata mismatch: %+v", push.Data)
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

func newGatewayWSTestServer(t *testing.T, opts ...gatewayws.ServerOption) (*httptest.Server, func()) {
	t.Helper()

	_, server, cleanup := newGatewayWSAppTestServer(t, opts...)
	return server, cleanup
}

func newGatewayWSAppTestServer(t *testing.T, opts ...gatewayws.ServerOption) (*gatewayws.Server, *httptest.Server, func()) {
	t.Helper()

	app := newGatewayWSApp(t, opts...)
	server := httptest.NewServer(app)
	return app, server, server.Close
}

func newGatewayWSAppTestServerWithPresence(t *testing.T, store presence.PresenceStore) (*gatewayws.Server, *httptest.Server, func()) {
	t.Helper()

	app := newGatewayWSAppWithPresence(t, store)
	server := httptest.NewServer(app)
	return app, server, server.Close
}

func newGatewayWSApp(t *testing.T, opts ...gatewayws.ServerOption) *gatewayws.Server {
	t.Helper()

	return newGatewayWSAppWithPresence(t, presence.NewMemoryStore(), opts...)
}

func newGatewayWSAppWithPresence(t *testing.T, store presence.PresenceStore, opts ...gatewayws.ServerOption) *gatewayws.Server {
	t.Helper()

	serviceContext := svc.NewMessageServiceContextWithAuth(
		repository.NewMemoryMessageRepository(),
		nil,
		nil,
		testJWTAuthConfig(),
	)
	serverOpts := []gatewayws.ServerOption{
		gatewayws.WithPresenceStore(store),
		gatewayws.WithPresenceTTL(time.Minute),
		gatewayws.WithInstanceID("gateway-test"),
	}
	serverOpts = append(serverOpts, opts...)
	return gatewayws.NewServer(serviceContext, serverOpts...)
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

	resp := readWSResponseAllowError(t, conn)
	if resp.Error != nil {
		t.Fatalf("websocket command returned error: %+v", resp.Error)
	}
	return resp
}

func readWSResponseAllowError(t *testing.T, conn *websocket.Conn) wsResponse {
	t.Helper()

	setReadDeadline(t, conn)
	var resp wsResponse
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("read websocket response: %v", err)
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

func syncGatewayWSConnection(t *testing.T, conn *websocket.Conn, requestID string) {
	t.Helper()

	writeCommand(t, conn, map[string]interface{}{
		"request_id": requestID,
		"type":       gatewayws.CommandHeartbeat,
		"payload":    map[string]interface{}{},
	})
	resp := readWSResponse(t, conn)
	if resp.frontendRequestID() != requestID || resp.commandName() != gatewayws.CommandHeartbeat || resp.Status != gateway.AckStatusOK {
		t.Fatalf("unexpected websocket sync heartbeat response: %+v", resp)
	}
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

func sendWSMessage(t *testing.T, conn *websocket.Conn, requestID string, receiverID string, clientMsgID string, content string) gateway.SendMessageCommandResponse {
	t.Helper()

	writeCommand(t, conn, map[string]interface{}{
		"requestId": requestID,
		"command":   gateway.CommandSendMessage,
		"payload": map[string]interface{}{
			"chatType":    "single",
			"receiverId":  receiverID,
			"clientMsgId": clientMsgID,
			"contentType": "text",
			"content":     content,
		},
	})

	resp := readWSResponse(t, conn)
	if resp.frontendRequestID() != requestID || resp.commandName() != gateway.CommandSendMessage || resp.Status != gateway.AckStatusOK {
		t.Fatalf("unexpected send response envelope: %+v", resp)
	}
	var sent gateway.SendMessageCommandResponse
	decodeRaw(t, resp.responsePayload(), &sent)
	if sent.Message.ServerMsgID == "" {
		t.Fatalf("send response missing server message id: %+v", sent)
	}
	return sent
}

func pullWSMessages(t *testing.T, conn *websocket.Conn, requestID string, conversationID string, fromSeq int64, toSeq int64) gateway.PullMessagesCommandResponse {
	t.Helper()

	writeCommand(t, conn, map[string]interface{}{
		"requestId": requestID,
		"command":   gateway.CommandPullMessages,
		"payload": map[string]interface{}{
			"conversationId": conversationID,
			"fromSeq":        fromSeq,
			"toSeq":          toSeq,
			"limit":          int32(10),
			"order":          "asc",
		},
	})

	resp := readWSResponse(t, conn)
	if resp.frontendRequestID() != requestID || resp.commandName() != gateway.CommandPullMessages || resp.Status != gateway.AckStatusOK {
		t.Fatalf("unexpected pull response envelope: %+v", resp)
	}
	var pulled gateway.PullMessagesCommandResponse
	decodeRaw(t, resp.responsePayload(), &pulled)
	return pulled
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
