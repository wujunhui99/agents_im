package msg

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/types"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msgclient"
	msgpb "github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"google.golang.org/grpc"
)

// fakeMsgRPC 只覆盖 SendMessage；其余方法走内嵌接口（调用即 panic，测试不应触达）。
type fakeMsgRPC struct {
	msgclient.Msg
	sendFn func(ctx context.Context, in *msgpb.SendMessageRequest, opts ...grpc.CallOption) (*msgpb.SendMessageResponse, error)
}

func (f *fakeMsgRPC) SendMessage(ctx context.Context, in *msgpb.SendMessageRequest, opts ...grpc.CallOption) (*msgpb.SendMessageResponse, error) {
	return f.sendFn(ctx, in, opts...)
}

func ctxWithUser(userID string) context.Context {
	return context.WithValue(context.Background(), ctxuser.UserIDClaim, userID)
}

// HTTP 入口的发送者契约：sender 一律取 JWT 用户，body 里的 senderId 只允许等于 JWT 用户。
// （原 tests/TestMessageHTTPHandlersUseJWTUser 随 message-api 退役迁移至此。）
func TestSendMessageUsesJWTUser(t *testing.T) {
	var gotSender string
	fake := &fakeMsgRPC{sendFn: func(_ context.Context, in *msgpb.SendMessageRequest, _ ...grpc.CallOption) (*msgpb.SendMessageResponse, error) {
		gotSender = in.GetSenderId()
		return &msgpb.SendMessageResponse{Message: &msgpb.Message{
			ServerMsgId: "1", SenderId: in.GetSenderId(), ConversationId: "single:usr_receiver:usr_sender", Seq: 1,
		}}, nil
	}}
	logic := NewSendMessageLogic(ctxWithUser("usr_sender"), &svc.ServiceContext{MsgRPC: fake})

	resp, err := logic.SendMessage(&types.SendMessageReq{
		ReceiverID: "usr_receiver", ChatType: "single", ClientMsgID: "client-jwt-1", ContentType: "text", Content: "hello",
	})
	if err != nil {
		t.Fatalf("send error = %v", err)
	}
	if gotSender != "usr_sender" || resp.Data.Message.SenderID != "usr_sender" {
		t.Fatalf("message sender did not use token user: rpc sender=%q resp sender=%q", gotSender, resp.Data.Message.SenderID)
	}
}

func TestSendMessageRejectsSenderMismatch(t *testing.T) {
	logic := NewSendMessageLogic(ctxWithUser("usr_sender"), &svc.ServiceContext{MsgRPC: &fakeMsgRPC{}})

	_, err := logic.SendMessage(&types.SendMessageReq{
		SenderID: "usr_other", ReceiverID: "usr_receiver", ChatType: "single", ClientMsgID: "client-jwt-2", ContentType: "text", Content: "hello",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("sender mismatch error = %v, want INVALID_ARGUMENT", err)
	}
}

func TestSendMessageRequiresAuthenticatedUser(t *testing.T) {
	logic := NewSendMessageLogic(context.Background(), &svc.ServiceContext{MsgRPC: &fakeMsgRPC{}})

	_, err := logic.SendMessage(&types.SendMessageReq{
		ReceiverID: "usr_receiver", ChatType: "single", ClientMsgID: "client-jwt-3", ContentType: "text", Content: "hello",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeUnauthenticated {
		t.Fatalf("unauthenticated error = %v, want UNAUTHENTICATED", err)
	}
}
