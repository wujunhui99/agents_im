// single-machine：进程内冒烟 monolith 账号/消息 logic 的核心链路
// （注册 → 加好友 → REST 语义发消息 → 拉取）。
// 03 §9 A3 起 WebSocket gateway 不再 in-process 装配 monolith（4 个 ws command
// 走 msg-rpc gRPC），ws 行为由 service/msggateway/internal/ws 测试覆盖，
// 本冒烟不再包含 ws leg。
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/wujunhui99/agents_im/common/middleware"
	"github.com/wujunhui99/agents_im/common/share/auth/token"
	authlogic "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/internal/auth/mailadapter"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func main() {
	ctx := context.Background()
	authSecret := "dev-jwt-secret-change-me"
	tokenManager := token.NewHMACTokenManager(authSecret, 24*time.Hour)

	userRepo := repository.NewMemoryRepository()
	userLogic := logic.NewUserLogic(userRepo)
	authRepository := authrepo.NewMemoryRepository()
	sessionStore := middleware.NewMemorySessionStore()
	authLogic := authlogic.NewAuthLogicWithOptions(authRepository, useradapter.NewLogicClient(userLogic), nil, tokenManager, authlogic.AuthOptions{
		VerificationRepo:          authRepository,
		Sessions:                  sessionStore,
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

	fmt.Println("single-process e2e passed")
	fmt.Printf("alice_user_id=%s\n", alice.UserID)
	fmt.Printf("bob_user_id=%s\n", bob.UserID)
	fmt.Printf("rest_conversation_id=%s\n", restSent.Message.ConversationID)
	fmt.Printf("rest_server_msg_id=%s\n", restSent.Message.ServerMsgID)
}

type e2eRegistrationMailer struct{}

func (e2eRegistrationMailer) SendTemplateEmail(context.Context, mailadapter.SendTemplateEmailRequest) error {
	return nil
}

func unique(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func must(label string, err error) {
	if err != nil {
		fail(fmt.Sprintf("%s: %v", label, err))
	}
}

func fail(message string) {
	panic(message)
}
