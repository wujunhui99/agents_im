package user

import (
	"context"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/pkg/objectstorage"
	"github.com/wujunhui99/agents_im/internal/repository"
	usersvc "github.com/wujunhui99/agents_im/internal/servicecontext/user"
	"github.com/wujunhui99/agents_im/internal/types"
)

func TestUpdateMeAvatarSetsCurrentUsersReadyAvatarAndReturnsDisplayURL(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryRepository()
	mediaRepo := repository.NewMemoryMediaRepository()
	store := objectstorage.NewMemoryStore()
	svcCtx := usersvc.NewServiceContextWithMedia(repo, mediaRepo, store, "agents-im-media", config.DefaultJWTAuthConfig())

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
	expectedAvatarURL := "/media/avatars/" + aliceAvatar.MediaID
	if resp.Data.AvatarURL != expectedAvatarURL {
		t.Fatalf("avatar_url = %q, want %q", resp.Data.AvatarURL, expectedAvatarURL)
	}
	if resp.Data.AvatarURLExpiresAt != 0 {
		t.Fatalf("avatar_url_expires_at = %d, want durable URL without persisted expiry", resp.Data.AvatarURLExpiresAt)
	}
	if strings.Contains(resp.Data.AvatarURL, "expiresAt") || strings.Contains(resp.Data.AvatarURL, "?") {
		t.Fatalf("avatar URL should not persist signed or expiring material: %q", resp.Data.AvatarURL)
	}

	getMe := NewGetMeLogic(context.WithValue(ctx, ctxuser.UserIDClaim, alice.UserID), svcCtx)
	me, err := getMe.GetMe(&types.GetMeReq{})
	if err != nil {
		t.Fatalf("get me after avatar update: %v", err)
	}
	if me.Data.AvatarURL != expectedAvatarURL {
		t.Fatalf("get me avatar_url = %q, want persisted %q", me.Data.AvatarURL, expectedAvatarURL)
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
