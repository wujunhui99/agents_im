package mail

import (
	"context"
	"fmt"
)

const ProviderTencentSES = "tencent_ses"

type SendStatus string

const (
	SendStatusAccepted SendStatus = "accepted"
)

type TemplateEmailSender interface {
	SendTemplateEmail(ctx context.Context, req TemplateEmailRequest) (TemplateEmailResponse, error)
}

type TemplateEmailRequest struct {
	Recipients     []string
	TemplateID     uint64
	TemplateData   map[string]string
	Subject        string
	FromEmail      string
	FromName       string
	IdempotencyKey string
}

type TemplateEmailResponse struct {
	Provider          string
	ProviderRequestID string
	ProviderMessageID string
	Status            SendStatus
}

type ProviderError struct {
	Provider  string
	Code      string
	Message   string
	RequestID string
}

func (e *ProviderError) Error() string {
	provider := e.Provider
	if provider == "" {
		provider = "email_provider"
	}
	if e.RequestID != "" {
		return fmt.Sprintf("%s error %s: %s (request_id=%s)", provider, e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("%s error %s: %s", provider, e.Code, e.Message)
}
