package auth

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/service/auth/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/auth/api/internal/types"
	authpb "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"github.com/wujunhui99/agents_im/service/auth/rpc/authclient"
	"google.golang.org/grpc"
)

func TestRegisterCallsAuthRPCAndPreservesProfileFields(t *testing.T) {
	client := &recordingAuthRPC{
		authResp: &authpb.AuthResponse{
			UserId:        "usr_1",
			Identifier:    "alice",
			Email:         "alice@example.test",
			DisplayName:   "Alice",
			Name:          "Alice Liddell",
			Gender:        "female",
			BirthDate:     "1996-05-02",
			Region:        "Shanghai",
			AccountType:   "user",
			AvatarMediaId: "med_avatar_1",
			AvatarUrl:     "/media/avatars/med_avatar_1",
			Token:         "[REDACTED]",
			ExpiresAt:     "2026-05-27T00:00:00Z",
		},
	}
	svcCtx := &svc.ServiceContext{AuthRPC: client}

	resp, err := NewRegisterLogic(context.Background(), svcCtx).Register(&types.RegisterReq{
		Identifier:            "alice",
		Email:                 "alice@example.test",
		EmailVerificationCode: "[REDACTED]",
		Password:              "[REDACTED]",
		DisplayName:           "Alice",
		Name:                  "Alice Liddell",
		Gender:                "female",
		BirthDate:             "1996-05-02",
		Region:                "Shanghai",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	if client.registerReq == nil {
		t.Fatalf("Register did not call auth RPC")
	}
	if got := client.registerReq.GetEmail(); got != "alice@example.test" {
		t.Fatalf("Register did not forward email to auth RPC: %q", got)
	}
	if got := resp.Data.Email; got != "alice@example.test" {
		t.Fatalf("Register response did not preserve email: %q", got)
	}
	if got := resp.Data.DisplayName; got != "Alice" {
		t.Fatalf("Register response did not preserve display name: %q", got)
	}
	if got := resp.Data.AvatarMediaID; got != "med_avatar_1" {
		t.Fatalf("Register response did not preserve avatar media id: %q", got)
	}
}

func TestValidateTokenCallsAuthRPC(t *testing.T) {
	client := &recordingAuthRPC{
		validateResp: &authpb.ValidateTokenResponse{
			Valid:      true,
			UserId:     "usr_1",
			Identifier: "alice",
			ExpiresAt:  "2026-05-27T00:00:00Z",
		},
	}
	svcCtx := &svc.ServiceContext{AuthRPC: client}

	resp, err := NewValidateTokenLogic(context.Background(), svcCtx).ValidateToken(&types.ValidateTokenReq{Token: "[REDACTED]"})
	if err != nil {
		t.Fatalf("ValidateToken returned error: %v", err)
	}

	if client.validateReq == nil {
		t.Fatalf("ValidateToken did not call auth RPC")
	}
	if !resp.Data.Valid || resp.Data.UserID != "usr_1" || resp.Data.Identifier != "alice" {
		t.Fatalf("unexpected validate response: %+v", resp.Data)
	}
}

type recordingAuthRPC struct {
	authclient.Auth

	registerReq  *authpb.RegisterRequest
	authResp     *authpb.AuthResponse
	validateReq  *authpb.ValidateTokenRequest
	validateResp *authpb.ValidateTokenResponse
}

func (c *recordingAuthRPC) Register(_ context.Context, in *authpb.RegisterRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	c.registerReq = in
	return c.authResp, nil
}

func (c *recordingAuthRPC) ValidateToken(_ context.Context, in *authpb.ValidateTokenRequest, _ ...grpc.CallOption) (*authpb.ValidateTokenResponse, error) {
	c.validateReq = in
	return c.validateResp, nil
}
