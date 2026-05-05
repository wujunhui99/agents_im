package friends

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/objectstorage"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/wujunhui99/agents_im/internal/types"
)

func TestListFriendsIncludesAcceptedContactAvatarDisplayURL(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryRepository()
	mediaRepo := repository.NewMemoryMediaRepository()
	store := objectstorage.NewMemoryStore()
	svcCtx := svc.NewUserServiceContextWithMedia(repo, mediaRepo, store, "agents-im-media", config.DefaultJWTAuthConfig())

	alice, err := svcCtx.UserLogic.CreateUser(ctx, business.CreateUserRequest{
		Identifier:  "alice_friend_avatar",
		DisplayName: "Alice",
		AccountType: string(model.AccountTypeUser),
	})
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := svcCtx.UserLogic.CreateUser(ctx, business.CreateUserRequest{
		Identifier:  "bob_friend_avatar",
		DisplayName: "Bob",
		AccountType: string(model.AccountTypeUser),
	})
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}
	bobAvatar := createFriendAvatarMediaForTest(t, mediaRepo, bob.UserID, "med_bob_friend_avatar")
	if _, err := svcCtx.UserLogic.UpdateUserAvatar(ctx, business.UpdateUserAvatarRequest{UserID: bob.UserID, MediaID: bobAvatar.MediaID}); err != nil {
		t.Fatalf("set bob avatar fixture: %v", err)
	}
	if _, err := svcCtx.FriendsLogic.AddFriend(ctx, business.AddFriendRequest{UserID: alice.UserID, FriendID: bob.UserID}); err != nil {
		t.Fatalf("add friend request: %v", err)
	}
	if _, err := svcCtx.FriendsLogic.AcceptFriendRequest(ctx, business.FriendRequestDecisionRequest{UserID: bob.UserID, FriendID: alice.UserID}); err != nil {
		t.Fatalf("accept friend request: %v", err)
	}

	logic := NewListFriendsLogic(context.WithValue(ctx, ctxuser.UserIDClaim, alice.UserID), svcCtx)
	resp, err := logic.ListFriends(&types.ListFriendsReq{})
	if err != nil {
		t.Fatalf("list friends: %v", err)
	}
	if len(resp.Data.Friends) != 1 {
		t.Fatalf("friends length = %d, want 1: %+v", len(resp.Data.Friends), resp.Data.Friends)
	}
	friend := resp.Data.Friends[0].Friend
	if friend.AvatarMediaID != bobAvatar.MediaID {
		t.Fatalf("friend avatar_media_id = %q, want %q", friend.AvatarMediaID, bobAvatar.MediaID)
	}
	if friend.AvatarURL == "" || friend.AvatarURLExpiresAt == 0 {
		t.Fatalf("friend avatar display fields missing: %+v", friend)
	}
}

func createFriendAvatarMediaForTest(t *testing.T, repo repository.MediaRepository, ownerUserID string, mediaID string) model.MediaObject {
	t.Helper()
	media, err := repo.CreateMediaObject(context.Background(), model.MediaObject{
		MediaID:     mediaID,
		OwnerUserID: ownerUserID,
		Bucket:      "agents-im-media",
		ObjectKey:   "users/" + ownerUserID + "/media/" + mediaID + "/avatar.webp",
		ContentType: "image/webp",
		SizeBytes:   2048,
		Purpose:     model.MediaPurposeAvatar,
		Status:      model.MediaStatusReady,
	})
	if err != nil {
		t.Fatalf("create avatar media: %v", err)
	}
	return media
}
