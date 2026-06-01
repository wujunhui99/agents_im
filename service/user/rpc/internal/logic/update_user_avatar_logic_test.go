package logic

import (
	"context"
	"testing"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/svc"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"
)

func TestUpdateUserAvatarValidatesMediaAndUpdatesProfile(t *testing.T) {
	ctx := context.Background()
	accountRepo := repository.NewMemoryRepository()
	mediaRepo := repository.NewMemoryMediaRepository()
	userLogic := business.NewUserLogic(accountRepo)
	svcCtx := &svc.ServiceContext{
		UserLogic:  userLogic,
		MediaLogic: business.NewMediaLogic(mediaRepo, nil, ""),
	}

	profile, err := userLogic.CreateUser(ctx, business.CreateUserRequest{
		Identifier:  "rpc_avatar_user",
		DisplayName: "RPC Avatar User",
		AccountType: string(model.AccountTypeUser),
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	media, err := mediaRepo.CreateMediaObject(ctx, model.MediaObject{
		MediaID:     "med_rpc_avatar",
		OwnerUserID: profile.UserID,
		Bucket:      "agents-im-media",
		ObjectKey:   "users/" + profile.UserID + "/media/med_rpc_avatar/avatar.png",
		ContentType: "image/png",
		SizeBytes:   1024,
		Purpose:     model.MediaPurposeAvatar,
		Status:      model.MediaStatusReady,
	})
	if err != nil {
		t.Fatalf("create avatar media: %v", err)
	}

	resp, err := NewUpdateUserAvatarLogic(ctx, svcCtx).UpdateUserAvatar(&userpb.UpdateUserAvatarRequest{
		UserId:        profile.UserID,
		AvatarMediaId: media.MediaID,
	})
	if err != nil {
		t.Fatalf("UpdateUserAvatar returned error: %v", err)
	}

	if got := resp.GetUser().GetAvatarMediaId(); got != media.MediaID {
		t.Fatalf("avatar_media_id = %q, want %q", got, media.MediaID)
	}
	if got := resp.GetUser().GetAvatarUrl(); got != "/media/avatars/"+media.MediaID {
		t.Fatalf("avatar_url = %q", got)
	}
}
