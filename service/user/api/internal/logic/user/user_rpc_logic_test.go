package user

import (
	"context"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/internal/ctxuser"
	"github.com/wujunhui99/agents_im/proto/userpb"
	"github.com/wujunhui99/agents_im/service/user/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/user/api/internal/types"
	"github.com/wujunhui99/agents_im/service/user/rpc/userservice"
	"google.golang.org/grpc"
)

func TestCreateUserCallsUserRPCAndPreservesEmail(t *testing.T) {
	client := &recordingUserRPC{
		createResp: userResponse("usr_1", "alice", "alice@example.test", "Alice"),
	}
	svcCtx := &svc.ServiceContext{UserRPC: client}

	resp, err := NewCreateUserLogic(context.Background(), svcCtx).CreateUser(&types.CreateUserReq{
		Identifier:  "alice",
		Email:       "alice@example.test",
		DisplayName: "Alice",
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	if client.createReq == nil {
		t.Fatalf("CreateUser did not call user RPC")
	}
	if got := client.createReq.GetEmail(); got != "alice@example.test" {
		t.Fatalf("CreateUser did not forward email to user RPC: %q", got)
	}
	if got := resp.Data.Email; got != "alice@example.test" {
		t.Fatalf("CreateUser response did not preserve email: %q", got)
	}
}

func TestUpdateMeCallsUserRPCWithAuthenticatedUser(t *testing.T) {
	client := &recordingUserRPC{
		updateProfileResp: userResponse("usr_1", "alice", "alice@example.test", "Alice Updated"),
	}
	svcCtx := &svc.ServiceContext{UserRPC: client}
	ctx := context.WithValue(context.Background(), ctxuser.UserIDClaim, "usr_1")

	resp, err := NewUpdateMeLogic(ctx, svcCtx).UpdateMe(&types.UpdateMeReq{
		DisplayName: "Alice Updated",
	})
	if err != nil {
		t.Fatalf("UpdateMe returned error: %v", err)
	}

	if client.updateProfileReq == nil {
		t.Fatalf("UpdateMe did not call user RPC")
	}
	if got := client.updateProfileReq.GetUserId(); got != "usr_1" {
		t.Fatalf("UpdateMe used wrong user id: %q", got)
	}
	if got := client.updateProfileReq.GetDisplayName(); got != "Alice Updated" {
		t.Fatalf("UpdateMe did not forward display name: %q", got)
	}
	if got := resp.Data.DisplayName; got != "Alice Updated" {
		t.Fatalf("UpdateMe response display name mismatch: %q", got)
	}
}

func TestUpdateMeRejectsImmutableFieldsBeforeRPC(t *testing.T) {
	client := &recordingUserRPC{}
	svcCtx := &svc.ServiceContext{UserRPC: client}
	ctx := context.WithValue(context.Background(), ctxuser.UserIDClaim, "usr_1")

	_, err := NewUpdateMeLogic(ctx, svcCtx).UpdateMe(&types.UpdateMeReq{
		UserID: "usr_2",
	})
	if err == nil {
		t.Fatalf("UpdateMe accepted immutable user_id change")
	}
	if !strings.Contains(err.Error(), "immutable profile fields") {
		t.Fatalf("UpdateMe returned unexpected error: %v", err)
	}
	if client.updateProfileReq != nil {
		t.Fatalf("UpdateMe called RPC after immutable field validation failed")
	}
}

func TestUpdateMeAvatarCallsUserRPCWithAuthenticatedUser(t *testing.T) {
	client := &recordingUserRPC{
		updateAvatarResp: userResponse("usr_1", "alice", "alice@example.test", "Alice"),
	}
	svcCtx := &svc.ServiceContext{UserRPC: client}
	ctx := context.WithValue(context.Background(), ctxuser.UserIDClaim, "usr_1")

	resp, err := NewUpdateMeAvatarLogic(ctx, svcCtx).UpdateMeAvatar(&types.UpdateMeAvatarReq{
		MediaID: "med_avatar_1",
	})
	if err != nil {
		t.Fatalf("UpdateMeAvatar returned error: %v", err)
	}

	if client.updateAvatarReq == nil {
		t.Fatalf("UpdateMeAvatar did not call user RPC")
	}
	if got := client.updateAvatarReq.GetUserId(); got != "usr_1" {
		t.Fatalf("UpdateMeAvatar used wrong user id: %q", got)
	}
	if got := client.updateAvatarReq.GetAvatarMediaId(); got != "med_avatar_1" {
		t.Fatalf("UpdateMeAvatar used wrong media id: %q", got)
	}
	if got := resp.Data.AvatarMediaID; got != "med_avatar_1" {
		t.Fatalf("UpdateMeAvatar response avatar mismatch: %q", got)
	}
}

type recordingUserRPC struct {
	userservice.UserService

	createReq         *userpb.CreateUserRequest
	createResp        *userpb.UserResponse
	updateProfileReq  *userpb.UpdateUserProfileRequest
	updateProfileResp *userpb.UserResponse
	updateAvatarReq   *userpb.UpdateUserAvatarRequest
	updateAvatarResp  *userpb.UserResponse
}

func (c *recordingUserRPC) CreateUser(_ context.Context, in *userpb.CreateUserRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	c.createReq = in
	return c.createResp, nil
}

func (c *recordingUserRPC) UpdateUserProfile(_ context.Context, in *userpb.UpdateUserProfileRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	c.updateProfileReq = in
	return c.updateProfileResp, nil
}

func (c *recordingUserRPC) UpdateUserAvatar(_ context.Context, in *userpb.UpdateUserAvatarRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	c.updateAvatarReq = in
	return c.updateAvatarResp, nil
}

func userResponse(userID string, identifier string, email string, displayName string) *userpb.UserResponse {
	return &userpb.UserResponse{
		User: &userpb.User{
			UserId:        userID,
			Identifier:    identifier,
			Email:         email,
			DisplayName:   displayName,
			Name:          displayName,
			AccountType:   "user",
			AvatarMediaId: "med_avatar_1",
			AvatarUrl:     "/media/avatars/med_avatar_1",
			CreatedAt:     "2026-05-27T00:00:00Z",
			UpdatedAt:     "2026-05-27T00:00:00Z",
		},
	}
}
