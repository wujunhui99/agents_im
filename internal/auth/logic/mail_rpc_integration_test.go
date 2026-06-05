package logic

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/auth/mailadapter"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/common/share/auth/token"
	"github.com/wujunhui99/agents_im/pkg/config"
	mailpb "github.com/wujunhui99/agents_im/service/third/rpc/mail"
	"google.golang.org/grpc"
)

func TestRegistrationEmailCodeUsesMailRPCClientFromListConfig(t *testing.T) {
	clearMailRPCEnvForLogicTest(t)
	endpoint, server := startRecordingMailRPCServer(t)

	configPath := filepath.Join(t.TempDir(), "auth-api.yaml")
	if err := os.WriteFile(configPath, []byte(`
Name: auth-api
Host: 127.0.0.1
Port: 18081
MailRPC:
  Endpoints:
    - `+endpoint+`
  Timeout: 5000
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.LoadAPIConfig(configPath)
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	mailer, err := mailadapter.NewOptionalRPCClient(cfg.MailRPC)
	if err != nil {
		t.Fatalf("build mail rpc client: %v", err)
	}
	if mailer == nil {
		t.Fatalf("mail rpc client should be configured from list-shaped MailRPC.Endpoints")
	}

	repo := authrepo.NewMemoryRepository()
	authLogic := NewAuthLogicWithOptions(
		repo,
		newAuthProfileClient(),
		NewPasswordHasher(),
		token.NewHMACTokenManager("unit-test-secret", time.Hour),
		AuthOptions{
			VerificationRepo:          repo,
			Mailer:                    mailer,
			RegistrationCodeGenerator: fixedRegistrationCode("246810"),
			Clock:                     func() time.Time { return time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC) },
		},
	)

	resp, err := authLogic.RequestRegistrationEmailCode(context.Background(), RegistrationEmailCodeRequest{
		Email: "mailrpc-list@example.com",
	})
	if err != nil {
		t.Fatalf("request registration email code: %v", err)
	}
	if resp.Email != "mailrpc-list@example.com" {
		t.Fatalf("response email = %q, want normalized request email", resp.Email)
	}

	requests := server.requests()
	if len(requests) != 1 {
		t.Fatalf("mail rpc request count = %d, want 1", len(requests))
	}
	if got := requests[0].GetTemplateData()["code"]; got != "246810" {
		t.Fatalf("mail rpc code template data = %q, want generated code", got)
	}
}

type recordingMailRPCServer struct {
	mailpb.UnimplementedMailServiceServer

	mu       sync.Mutex
	captured []*mailpb.SendTemplateEmailRequest
}

func (s *recordingMailRPCServer) SendTemplateEmail(_ context.Context, req *mailpb.SendTemplateEmailRequest) (*mailpb.SendTemplateEmailResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.captured = append(s.captured, req)
	return &mailpb.SendTemplateEmailResponse{
		Provider: "test",
		Status:   "accepted",
	}, nil
}

func (s *recordingMailRPCServer) requests() []*mailpb.SendTemplateEmailRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*mailpb.SendTemplateEmailRequest, len(s.captured))
	copy(out, s.captured)
	return out
}

func startRecordingMailRPCServer(t *testing.T) (string, *recordingMailRPCServer) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	recorder := &recordingMailRPCServer{}
	mailpb.RegisterMailServiceServer(server, recorder)
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	return listener.Addr().String(), recorder
}

func clearMailRPCEnvForLogicTest(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"AUTH_MAIL_RPC_TARGET",
		"AGENTS_IM_MAIL_RPC_TARGET",
		"MAIL_RPC_TARGET",
		"AUTH_MAIL_RPC_ENDPOINTS",
		"AGENTS_IM_MAIL_RPC_ENDPOINTS",
		"MAIL_RPC_ENDPOINTS",
	} {
		t.Setenv(key, "")
	}
}
