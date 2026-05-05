package user

import (
	"context"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/objectstorage"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/wujunhui99/agents_im/internal/types"
)

func TestUpdateMeAvatarSetsCurrentUsersReadyAvatarAndReturnsDisplayURL(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryRepository()
	mediaRepo := repository.NewMemoryMediaRepository()
	store := objectstorage.NewMemoryStore()
	svcCtx := svc.NewUserServiceContextWithMedia(repo, mediaRepo, store, "agents-im-media", config.DefaultJWTAuthConfig())

	alice, err := svcCtx.UserLogic.CreateUser(ctx, business.CreateUserRequest{
		Identifier:  "alice_avatar",
		DisplayName: "Alice",
		AccountType: string(model.AccountTypeUser),
	})
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := svcCtx.UserLogic.CreateUser(ctx, business.CreateUserRequest{
		Identifier:  "bob_avatar",
		DisplayName: "Bob",
		AccountType: string(model.AccountTypeUser),
	})
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}

	aliceAvatar := createAvatarMediaForUserTest(t, mediaRepo, alice.UserID, "med_alice_avatar")
	bobAvatar := createAvatarMediaForUserTest(t, mediaRepo, bob.UserID, "med_bob_avatar")

	logic := NewUpdateMeAvatarLogic(context.WithValue(ctx, ctxuser.UserIDClaim, alice.UserID), svcCtx)
	resp, err := logic.UpdateMeAvatar(&types.UpdateMeAvatarReq{MediaID: aliceAvatar.MediaID})
	if err != nil {
		t.Fatalf("update avatar: %v", err)
	}
	if resp.Data.AvatarMediaID != aliceAvatar.MediaID {
		t.Fatalf("avatar_media_id = %q, want %q", resp.Data.AvatarMediaID, aliceAvatar.MediaID)
	}
	if resp.Data.AvatarURL == "" || resp.Data.AvatarURLExpiresAt == 0 {
		t.Fatalf("avatar display fields missing from response: %+v", resp.Data)
	}
	if strings.Contains(resp.Data.AvatarURL, "secret") {
		t.Fatalf("avatar URL should not expose signed private material: %q", resp.Data.AvatarURL)
	}

	_, err = logic.UpdateMeAvatar(&types.UpdateMeAvatarReq{MediaID: bobAvatar.MediaID})
	assertUserGoZeroCode(t, err, apperror.CodeForbidden)
}

func createAvatarMediaForUserTest(t *testing.T, repo repository.MediaRepository, ownerUserID string, mediaID string) model.MediaObject {
	t.Helper()
	media, err := repo.CreateMediaObject(context.Background(), model.MediaObject{
		MediaID:     mediaID,
		OwnerUserID: ownerUserID,
		Bucket:      "agents-im-media",
		ObjectKey:   "users/" + ownerUserID + "/media/" + mediaID + "/avatar.png",
		ContentType: "image/png",
		SizeBytes:   1024,
		Purpose:     model.MediaPurposeAvatar,
		Status:      model.MediaStatusReady,
	})
	if err != nil {
		t.Fatalf("create avatar media: %v", err)
	}
	return media
}

func assertUserGoZeroCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s", code)
	}
	if got := apperror.From(err).Code; got != code {
		t.Fatalf("error code = %s, want %s: %v", got, code, err)
	}
}
