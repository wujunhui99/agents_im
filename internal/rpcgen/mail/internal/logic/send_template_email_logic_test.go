package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/wujunhui99/agents_im/internal/mail"
	"github.com/wujunhui99/agents_im/internal/rpcgen/mail/internal/config"
	"github.com/wujunhui99/agents_im/internal/rpcgen/mail/internal/svc"
	"github.com/wujunhui99/agents_im/proto/mailpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type recordingMailProvider struct {
	req  mail.TemplateEmailRequest
	resp mail.TemplateEmailResponse
	err  error
}

func (p *recordingMailProvider) SendTemplateEmail(ctx context.Context, req mail.TemplateEmailRequest) (mail.TemplateEmailResponse, error) {
	p.req = req
	return p.resp, p.err
}

func TestSendTemplateEmailLogicCallsProviderAndReturnsProviderIDs(t *testing.T) {
	provider := &recordingMailProvider{
		resp: mail.TemplateEmailResponse{
			Provider:          "tencent_ses",
			ProviderRequestID: "req-123",
			ProviderMessageID: "msg-456",
			Status:            mail.SendStatusAccepted,
		},
	}
	logic := NewSendTemplateEmailLogic(context.Background(), &svc.ServiceContext{
		Config:            config.Config{},
		MailProvider:      provider,
		DefaultTemplateID: 177952,
	})

	resp, err := logic.SendTemplateEmail(&mailpb.SendTemplateEmailRequest{
		Recipients:     []string{"new-user@example.com"},
		TemplateId:     177952,
		TemplateData:   map[string]string{"code": "123456"},
		Subject:        "Verify your account",
		FromEmail:      "security@example.com",
		FromName:       "Agents IM",
		IdempotencyKey: "signup-email-1",
	})
	if err != nil {
		t.Fatalf("send template email: %v", err)
	}
	if resp.GetProviderRequestId() != "req-123" || resp.GetProviderMessageId() != "msg-456" || resp.GetStatus() != string(mail.SendStatusAccepted) {
		t.Fatalf("unexpected rpc response: %+v", resp)
	}
	if provider.req.TemplateData["code"] != "123456" {
		t.Fatalf("template data was not passed through: %#v", provider.req.TemplateData)
	}
	if provider.req.FromEmail != "security@example.com" || provider.req.FromName != "Agents IM" {
		t.Fatalf("from override was not passed through: %+v", provider.req)
	}
	if provider.req.IdempotencyKey != "signup-email-1" {
		t.Fatalf("idempotency key was not passed through: %+v", provider.req)
	}
}

func TestSendTemplateEmailLogicReturnsProviderErrorAsGRPCError(t *testing.T) {
	provider := &recordingMailProvider{
		err: &mail.ProviderError{
			Provider:  "tencent_ses",
			Code:      "FailedOperation.TemplateUnavailable",
			Message:   "template unavailable",
			RequestID: "req-error",
		},
	}
	logic := NewSendTemplateEmailLogic(context.Background(), &svc.ServiceContext{
		Config:            config.Config{},
		MailProvider:      provider,
		DefaultTemplateID: 177952,
	})

	_, err := logic.SendTemplateEmail(&mailpb.SendTemplateEmailRequest{
		Recipients:   []string{"new-user@example.com"},
		TemplateId:   177952,
		TemplateData: map[string]string{"code": "123456"},
	})
	if err == nil {
		t.Fatal("expected provider error")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected grpc status error, got %T: %v", err, err)
	}
	if st.Code() != codes.Unavailable {
		t.Fatalf("unexpected grpc code: %v", st.Code())
	}
	if got := st.Message(); got == "" || got == "template unavailable" {
		t.Fatalf("expected normalized provider context in message, got %q", got)
	}
}

func TestSendTemplateEmailLogicRequiresTemplateIDOrDefault(t *testing.T) {
	logic := NewSendTemplateEmailLogic(context.Background(), &svc.ServiceContext{
		Config:       config.Config{},
		MailProvider: &recordingMailProvider{},
	})

	_, err := logic.SendTemplateEmail(&mailpb.SendTemplateEmailRequest{
		Recipients:   []string{"new-user@example.com"},
		TemplateData: map[string]string{"code": "123456"},
	})
	if err == nil {
		t.Fatal("expected missing template id error")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unexpected grpc code: %v", status.Code(err))
	}
}

func TestSendTemplateEmailLogicMapsUnexpectedProviderError(t *testing.T) {
	provider := &recordingMailProvider{err: errors.New("network down")}
	logic := NewSendTemplateEmailLogic(context.Background(), &svc.ServiceContext{
		Config:            config.Config{},
		MailProvider:      provider,
		DefaultTemplateID: 177952,
	})

	_, err := logic.SendTemplateEmail(&mailpb.SendTemplateEmailRequest{
		Recipients:   []string{"new-user@example.com"},
		TemplateData: map[string]string{"code": "123456"},
	})
	if err == nil {
		t.Fatal("expected provider error")
	}
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("unexpected grpc code: %v", status.Code(err))
	}
}
