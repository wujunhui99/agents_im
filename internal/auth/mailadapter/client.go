package mailadapter

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/rpcgen/mail/mailservice"
	"github.com/wujunhui99/agents_im/proto/mailpb"
)

type SendTemplateEmailRequest struct {
	Recipients     []string
	TemplateID     uint64
	TemplateData   map[string]string
	Subject        string
	FromEmail      string
	FromName       string
	IdempotencyKey string
}

type Client interface {
	SendTemplateEmail(ctx context.Context, req SendTemplateEmailRequest) error
}

type RPCClient struct {
	client mailservice.MailService
}

func NewRPCClient(client mailservice.MailService) *RPCClient {
	return &RPCClient{client: client}
}

func (c *RPCClient) SendTemplateEmail(ctx context.Context, req SendTemplateEmailRequest) error {
	_, err := c.client.SendTemplateEmail(ctx, &mailpb.SendTemplateEmailRequest{
		Recipients:     append([]string(nil), req.Recipients...),
		TemplateId:     req.TemplateID,
		TemplateData:   cloneTemplateData(req.TemplateData),
		Subject:        req.Subject,
		FromEmail:      req.FromEmail,
		FromName:       req.FromName,
		IdempotencyKey: req.IdempotencyKey,
	})
	return err
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
