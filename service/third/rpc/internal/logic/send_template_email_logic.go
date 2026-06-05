package logic

import (
	"context"
	"errors"
	"strings"

	mailprovider "github.com/wujunhui99/agents_im/service/third/rpc/internal/provider"
	"github.com/wujunhui99/agents_im/service/third/rpc/internal/svc"
	mailpb "github.com/wujunhui99/agents_im/service/third/rpc/mail"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SendTemplateEmailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendTemplateEmailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendTemplateEmailLogic {
	return &SendTemplateEmailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SendTemplateEmailLogic) SendTemplateEmail(in *mailpb.SendTemplateEmailRequest) (*mailpb.SendTemplateEmailResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	recipients := normalizeRecipients(in.GetRecipients())
	if len(recipients) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one recipient is required")
	}
	templateID := in.GetTemplateId()
	if templateID == 0 {
		templateID = l.svcCtx.DefaultTemplateID
	}
	if templateID == 0 {
		return nil, status.Error(codes.InvalidArgument, "template_id is required")
	}
	if l.svcCtx.MailProvider == nil {
		return nil, status.Error(codes.FailedPrecondition, "mail provider is not configured")
	}

	resp, err := l.svcCtx.MailProvider.SendTemplateEmail(l.ctx, mailprovider.TemplateEmailRequest{
		Recipients:     recipients,
		TemplateID:     templateID,
		TemplateData:   cloneTemplateData(in.GetTemplateData()),
		Subject:        strings.TrimSpace(in.GetSubject()),
		FromEmail:      strings.TrimSpace(in.GetFromEmail()),
		FromName:       strings.TrimSpace(in.GetFromName()),
		IdempotencyKey: strings.TrimSpace(in.GetIdempotencyKey()),
	})
	if err != nil {
		return nil, providerErrorToStatus(err)
	}

	return &mailpb.SendTemplateEmailResponse{
		Provider:          resp.Provider,
		ProviderRequestId: resp.ProviderRequestID,
		ProviderMessageId: resp.ProviderMessageID,
		Status:            string(resp.Status),
	}, nil
}

func normalizeRecipients(values []string) []string {
	recipients := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			recipients = append(recipients, value)
		}
	}
	return recipients
}

func cloneTemplateData(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func providerErrorToStatus(err error) error {
	var providerErr *mailprovider.ProviderError
	if errors.As(err, &providerErr) {
		message := providerErr.Code
		if providerErr.Message != "" {
			message += ": " + providerErr.Message
		}
		if providerErr.RequestID != "" {
			message += " (request_id=" + providerErr.RequestID + ")"
		}
		return status.Error(codes.Unavailable, message)
	}
	return status.Error(codes.Unavailable, "mail provider unavailable")
}
