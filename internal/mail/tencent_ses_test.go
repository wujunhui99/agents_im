package mail

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTencentSESProviderPassesTemplateDataForVerificationCode(t *testing.T) {
	var captured struct {
		FromEmailAddress string `json:"FromEmailAddress"`
		Destination      []string
		Subject          string
		Template         struct {
			TemplateID   uint64 `json:"TemplateID"`
			TemplateData string `json:"TemplateData"`
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-TC-Action") != "SendEmail" {
			t.Fatalf("unexpected action header: %q", r.Header.Get("X-TC-Action"))
		}
		if r.Header.Get("Authorization") == "" {
			t.Fatal("expected TC3 authorization header")
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Response":{"RequestId":"req-123","MessageId":"msg-456"}}`))
	}))
	t.Cleanup(server.Close)

	provider, err := NewTencentSESProvider(TencentSESConfig{
		SecretID:          "test-secret-id",
		SecretKey:         "test-secret-key",
		Region:            "ap-hongkong",
		Endpoint:          server.URL,
		FromEmailAddress:  "noreply@agenticim.xyz",
		DefaultTemplateID: "177952",
	}, server.Client())
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	resp, err := provider.SendTemplateEmail(context.Background(), TemplateEmailRequest{
		Recipients:     []string{"new-user@example.com"},
		TemplateID:     177952,
		TemplateData:   map[string]string{"code": "123456", "purpose": "register"},
		Subject:        "Verify your account",
		IdempotencyKey: "signup-email-1",
	})
	if err != nil {
		t.Fatalf("send template email: %v", err)
	}
	if resp.ProviderRequestID != "req-123" || resp.ProviderMessageID != "msg-456" || resp.Status != SendStatusAccepted {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if captured.FromEmailAddress != "noreply@agenticim.xyz" {
		t.Fatalf("unexpected from email: %q", captured.FromEmailAddress)
	}
	if len(captured.Destination) != 1 || captured.Destination[0] != "new-user@example.com" {
		t.Fatalf("unexpected destination: %#v", captured.Destination)
	}
	if captured.Template.TemplateID != 177952 {
		t.Fatalf("unexpected template id: %d", captured.Template.TemplateID)
	}

	var templateData map[string]string
	if err := json.Unmarshal([]byte(captured.Template.TemplateData), &templateData); err != nil {
		t.Fatalf("template data is not JSON object: %v", err)
	}
	if templateData["code"] != "123456" || templateData["purpose"] != "register" {
		t.Fatalf("template data mismatch: %#v", templateData)
	}
}

func TestTencentSESProviderReturnsNormalizedProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Response":{"Error":{"Code":"FailedOperation.TemplateUnavailable","Message":"template unavailable"},"RequestId":"req-error"}}`))
	}))
	t.Cleanup(server.Close)

	provider, err := NewTencentSESProvider(TencentSESConfig{
		SecretID:          "test-secret-id",
		SecretKey:         "test-secret-key",
		Region:            "ap-hongkong",
		Endpoint:          server.URL,
		FromEmailAddress:  "noreply@agenticim.xyz",
		DefaultTemplateID: "177952",
	}, server.Client())
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	_, err = provider.SendTemplateEmail(context.Background(), TemplateEmailRequest{
		Recipients:   []string{"new-user@example.com"},
		TemplateID:   177952,
		TemplateData: map[string]string{"code": "123456"},
	})
	if err == nil {
		t.Fatal("expected provider error")
	}

	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if providerErr.Code != "FailedOperation.TemplateUnavailable" || providerErr.RequestID != "req-error" {
		t.Fatalf("provider error was not normalized: %+v", providerErr)
	}
}
