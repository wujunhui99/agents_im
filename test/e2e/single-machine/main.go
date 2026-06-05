package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wujunhui99/agents_im/common/share/auth/token"
	authlogic "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/internal/auth/mailadapter"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
	"github.com/wujunhui99/agents_im/internal/gateway"
	"github.com/wujunhui99/agents_im/internal/gateway/delivery"
	gatewayws "github.com/wujunhui99/agents_im/internal/gateway/ws"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	gatewaysvc "github.com/wujunhui99/agents_im/internal/servicecontext/gateway"
	"github.com/wujunhui99/agents_im/pkg/config"
)

func main() {
	ctx := context.Background()
	authSecret := "dev-jwt-secret-change-me"
	tokenManager := token.NewHMACTokenManager(authSecret, 24*time.Hour)

	userRepo := repository.NewMemoryRepository()
	userLogic := logic.NewUserLogic(userRepo)
	authRepository := authrepo.NewMemoryRepository()
	authLogic := authlogic.NewAuthLogicWithOptions(authRepository, useradapter.NewLogicClient(userLogic), nil, tokenManager, authlogic.AuthOptions{
		VerificationRepo:          authRepository,
		Mailer:                    e2eRegistrationMailer{},
		RegistrationCodeGenerator: func() (string, error) { return "123456", nil },
		RegistrationSendCooldown:  time.Nanosecond,
	})

	aliceEmail := unique("alice") + "@example.com"
	_, err := authLogic.RequestRegistrationEmailCode(ctx, authlogic.RegistrationEmailCodeRequest{Email: aliceEmail})
	must("request alice registration code", err)
	alice, err := authLogic.Register(ctx, authlogic.RegisterRequest{Identifier: unique("alice"), Email: aliceEmail, EmailVerificationCode: "123456", Password: "password123", DisplayName: "Alice E2E"})
	must("register alice", err)
	bobEmail := unique("bob") + "@example.com"
	_, err = authLogic.RequestRegistrationEmailCode(ctx, authlogic.RegistrationEmailCodeRequest{Email: bobEmail})
	must("request bob registration code", err)
	bob, err := authLogic.Register(ctx, authlogic.RegisterRequest{Identifier: unique("bob"), Email: bobEmail, EmailVerificationCode: "123456", Password: "password123", DisplayName: "Bob E2E"})
	must("register bob", err)

	_, _, err = userRepo.AddFriend(ctx, alice.UserID, bob.UserID)
	must("add friend", err)

	messageRepo := repository.NewMemoryMessageRepository()
	messageLogic := logic.NewMessageLogicWithValidators(messageRepo, logic.NewUserLogicExistenceChecker(userLogic), nil)
	restSent, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
		SenderID:    alice.UserID,
		ReceiverID:  bob.UserID,
		ChatType:    logic.MessageChatTypeSingle,
		ClientMsgID: unique("rest"),
		ContentType: logic.MessageContentTypeText,
		Content:     "hello from single process rest e2e",
	})
	must("send rest message", err)
	pulled, err := messageLogic.PullMessages(ctx, logic.PullMessagesRequest{UserID: bob.UserID, ConversationID: restSent.Message.ConversationID, FromSeq: 1, Limit: 10, Order: "asc"})
	must("pull rest message", err)
	if len(pulled.Messages) != 1 || pulled.Messages[0].ServerMsgID != restSent.Message.ServerMsgID {
		fail("pull did not return sent REST message")
	}

	wsServer := gatewayws.NewServer(
		gatewaysvc.NewServiceContext(messageLogic, testAuth(authSecret)),
		gatewayws.WithActiveSessionRepository(authRepository),
	)
	httpServer := httptest.NewServer(wsServer)
	defer httpServer.Close()

	aliceConn := dial(httpServer.URL, alice.Token)
	defer aliceConn.Close()
	bobConn := dial(httpServer.URL, bob.Token)
	defer bobConn.Close()

	ackCh := readMatching(aliceConn, func(frame map[string]any) bool {
		return frame["command"] == gateway.CommandSendMessage || frame["type"] == gateway.CommandSendMessage
	})
	pushCh := readMatching(bobConn, func(frame map[string]any) bool { return frame["type"] == delivery.EventMessageReceived })
	must("write websocket send", aliceConn.WriteJSON(map[string]any{
		"requestId": "single-process-ws-send",
		"command":   gateway.CommandSendMessage,
		"payload": map[string]any{
			"chatType":    logic.MessageChatTypeSingle,
			"receiverId":  bob.UserID,
			"clientMsgId": unique("ws"),
			"contentType": logic.MessageContentTypeText,
			"content":     "hello from single process websocket e2e",
		},
	}))

	ack := <-ackCh
	push := <-pushCh
	if ack["status"] != gateway.AckStatusOK {
		fail(fmt.Sprintf("unexpected websocket ack: %+v", ack))
	}
	ackPayload := ack["payload"].(map[string]any)
	ackMessage := ackPayload["message"].(map[string]any)
	pushData := push["data"].(map[string]any)
	if pushData["server_msg_id"] != ackMessage["serverMsgId"] {
		fail(fmt.Sprintf("push server_msg_id mismatch: ack=%+v push=%+v", ackMessage, pushData))
	}

	fmt.Println("single-process e2e passed")
	fmt.Printf("alice_user_id=%s\n", alice.UserID)
	fmt.Printf("bob_user_id=%s\n", bob.UserID)
	fmt.Printf("rest_conversation_id=%s\n", restSent.Message.ConversationID)
	fmt.Printf("ws_server_msg_id=%s\n", ackMessage["serverMsgId"])
}

func testAuth(secret string) config.JWTAuthConfig {
	return config.JWTAuthConfig{AccessSecret: secret, AccessExpire: 86400}
}

type e2eRegistrationMailer struct{}

func (e2eRegistrationMailer) SendTemplateEmail(context.Context, mailadapter.SendTemplateEmailRequest) error {
	return nil
}

func unique(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func dial(serverURL string, token string) *websocket.Conn {
	u, err := url.Parse(serverURL)
	must("parse server url", err)
	u.Scheme = "ws"
	u.Path = "/ws"
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	must("dial websocket", err)
	return conn
}

func readMatching(conn *websocket.Conn, predicate func(map[string]any) bool) <-chan map[string]any {
	ch := make(chan map[string]any, 1)
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			_ = conn.SetReadDeadline(deadline)
			var frame map[string]any
			if err := conn.ReadJSON(&frame); err != nil {
				fail(fmt.Sprintf("read websocket frame: %v", err))
			}
			if predicate(frame) {
				ch <- normalizeJSON(frame)
				return
			}
		}
		fail("timed out waiting for websocket frame")
	}()
	return ch
}

func normalizeJSON(frame map[string]any) map[string]any {
	raw, err := json.Marshal(frame)
	must("marshal frame", err)
	var normalized map[string]any
	must("unmarshal frame", json.Unmarshal(raw, &normalized))
	return normalized
}

func must(label string, err error) {
	if err != nil {
		fail(fmt.Sprintf("%s: %v", label, err))
	}
}

func fail(message string) {
	panic(message)
}
