package tests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/svc"
)

func TestFriendsLogicAddDuplicateDeleteAndList(t *testing.T) {
	repo := repository.NewMemoryRepository()
	userLogic := logic.NewUserLogic(repo)
	friendsLogic := logic.NewFriendsLogic(repo, userLogic)
	ctx := context.Background()

	alice := createFriendTestUser(t, ctx, userLogic, "alice_100")
	bob := createFriendTestUser(t, ctx, userLogic, "bob_100")

	added, err := friendsLogic.AddFriend(ctx, logic.AddFriendRequest{UserID: alice.UserID, FriendID: bob.UserID})
	if err != nil {
		t.Fatalf("add friend: %v", err)
	}
	if !added.Created || !added.Friendship.IsFriend || added.Friendship.Status != model.FriendshipStatusActive {
		t.Fatalf("unexpected add response: %+v", added)
	}

	duplicate, err := friendsLogic.AddFriend(ctx, logic.AddFriendRequest{UserID: alice.UserID, FriendID: bob.UserID})
	if err != nil {
		t.Fatalf("duplicate add friend: %v", err)
	}
	if duplicate.Created {
		t.Fatalf("duplicate add should be idempotent: %+v", duplicate)
	}

	aliceList, err := friendsLogic.ListFriends(ctx, logic.ListFriendsRequest{UserID: alice.UserID})
	if err != nil {
		t.Fatalf("list alice friends: %v", err)
	}
	if len(aliceList.Friends) != 1 || aliceList.Friends[0].FriendID != bob.UserID {
		t.Fatalf("unexpected alice friends: %+v", aliceList.Friends)
	}

	bobList, err := friendsLogic.ListFriends(ctx, logic.ListFriendsRequest{UserID: bob.UserID})
	if err != nil {
		t.Fatalf("list bob friends: %v", err)
	}
	if len(bobList.Friends) != 1 || bobList.Friends[0].FriendID != alice.UserID {
		t.Fatalf("friendship should be bidirectional: %+v", bobList.Friends)
	}

	deleted, err := friendsLogic.DeleteFriend(ctx, logic.DeleteFriendRequest{UserID: alice.UserID, FriendID: bob.UserID})
	if err != nil {
		t.Fatalf("delete friend: %v", err)
	}
	if !deleted.Deleted || deleted.Friendship.IsFriend || deleted.Friendship.Status != model.FriendshipStatusDeleted {
		t.Fatalf("unexpected delete response: %+v", deleted)
	}

	afterDelete, err := friendsLogic.ListFriends(ctx, logic.ListFriendsRequest{UserID: alice.UserID})
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(afterDelete.Friends) != 0 {
		t.Fatalf("deleted friendship should not be listed: %+v", afterDelete.Friends)
	}

	friendship, err := friendsLogic.GetFriendship(ctx, logic.GetFriendshipRequest{UserID: alice.UserID, FriendID: bob.UserID})
	if err != nil {
		t.Fatalf("get deleted friendship: %v", err)
	}
	if friendship.Friendship.IsFriend || friendship.Friendship.Status != model.FriendshipStatusDeleted {
		t.Fatalf("deleted friendship should be inactive: %+v", friendship.Friendship)
	}
}

func TestFriendsLogicCannotAddSelf(t *testing.T) {
	repo := repository.NewMemoryRepository()
	userLogic := logic.NewUserLogic(repo)
	friendsLogic := logic.NewFriendsLogic(repo, userLogic)
	ctx := context.Background()
	alice := createFriendTestUser(t, ctx, userLogic, "alice_self")

	_, err := friendsLogic.AddFriend(ctx, logic.AddFriendRequest{UserID: alice.UserID, FriendID: alice.UserID})
	if err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("self add error = %v, want INVALID_ARGUMENT", err)
	}
}

func TestFriendsLogicUserNotExists(t *testing.T) {
	repo := repository.NewMemoryRepository()
	userLogic := logic.NewUserLogic(repo)
	friendsLogic := logic.NewFriendsLogic(repo, userLogic)
	ctx := context.Background()
	alice := createFriendTestUser(t, ctx, userLogic, "alice_exists")

	_, err := friendsLogic.AddFriend(ctx, logic.AddFriendRequest{UserID: alice.UserID, FriendID: "usr_missing"})
	if err == nil || apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing target user error = %v, want NOT_FOUND", err)
	}

	_, err = friendsLogic.AddFriend(ctx, logic.AddFriendRequest{UserID: "usr_missing", FriendID: alice.UserID})
	if err == nil || apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing current user error = %v, want NOT_FOUND", err)
	}
}

func TestFriendsHTTPHandlers(t *testing.T) {
	serviceContext := svc.NewServiceContextWithAuth(repository.NewMemoryRepository(), testJWTAuthConfig())
	mux := newFriendsGoZeroRouter(t, serviceContext)
	ctx := context.Background()

	alice := createFriendTestUser(t, ctx, serviceContext.UserLogic, "alice_http")
	bob := createFriendTestUser(t, ctx, serviceContext.UserLogic, "bob_http")
	aliceBearer := bearerTokenForUser(t, alice.UserID)

	t.Run("rejects legacy X-User-Id header without bearer token", func(t *testing.T) {
		bypassResp := httptest.NewRecorder()
		bypassReq := newJSONRequest(http.MethodPost, "/friends", fmt.Sprintf(`{"user_id":"%s"}`, bob.UserID))
		setRejectedLegacyXUserIDHeader(t, bypassReq, alice.UserID)
		mux.ServeHTTP(bypassResp, bypassReq)
		if bypassResp.Code != http.StatusUnauthorized {
			t.Fatalf("legacy X-User-Id rejection status = %d", bypassResp.Code)
		}
	})

	addResp := httptest.NewRecorder()
	addReq := newJSONRequest(http.MethodPost, "/friends", fmt.Sprintf(`{"user_id":"%s"}`, bob.UserID))
	addReq.Header.Set("Authorization", aliceBearer)
	mux.ServeHTTP(addResp, addReq)
	if addResp.Code != http.StatusOK {
		t.Fatalf("add status = %d, body = %s", addResp.Code, addResp.Body.String())
	}
	var added envelope[logic.AddFriendResponse]
	decodeEnvelope(t, addResp.Body.Bytes(), &added)
	if !added.Data.Created || added.Data.Friendship.FriendID != bob.UserID {
		t.Fatalf("unexpected add response: %+v", added.Data)
	}
	if added.Data.Friendship.UserID != alice.UserID {
		t.Fatalf("friendship did not use token user: %+v", added.Data.Friendship)
	}

	duplicateResp := httptest.NewRecorder()
	duplicateReq := newJSONRequest(http.MethodPost, "/friends", fmt.Sprintf(`{"user_id":"%s"}`, bob.UserID))
	duplicateReq.Header.Set("Authorization", aliceBearer)
	mux.ServeHTTP(duplicateResp, duplicateReq)
	if duplicateResp.Code != http.StatusOK {
		t.Fatalf("duplicate status = %d, body = %s", duplicateResp.Code, duplicateResp.Body.String())
	}
	var duplicate envelope[logic.AddFriendResponse]
	decodeEnvelope(t, duplicateResp.Body.Bytes(), &duplicate)
	if duplicate.Data.Created {
		t.Fatalf("duplicate add should return created=false: %+v", duplicate.Data)
	}

	listResp := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/friends", nil)
	listReq.Header.Set("Authorization", aliceBearer)
	mux.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	var list envelope[logic.ListFriendsResponse]
	decodeEnvelope(t, listResp.Body.Bytes(), &list)
	if len(list.Data.Friends) != 1 || list.Data.Friends[0].FriendID != bob.UserID {
		t.Fatalf("unexpected list response: %+v", list.Data.Friends)
	}

	getResp := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/friends/"+bob.UserID, nil)
	getReq.Header.Set("Authorization", aliceBearer)
	mux.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getResp.Code, getResp.Body.String())
	}
	var got envelope[logic.GetFriendshipResponse]
	decodeEnvelope(t, getResp.Body.Bytes(), &got)
	if !got.Data.Friendship.IsFriend || got.Data.Friendship.Status != model.FriendshipStatusActive {
		t.Fatalf("unexpected friendship response: %+v", got.Data.Friendship)
	}

	deleteResp := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/friends/"+bob.UserID, nil)
	deleteReq.Header.Set("Authorization", aliceBearer)
	mux.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}

	afterDeleteResp := httptest.NewRecorder()
	afterDeleteReq := httptest.NewRequest(http.MethodGet, "/friends", nil)
	afterDeleteReq.Header.Set("Authorization", aliceBearer)
	mux.ServeHTTP(afterDeleteResp, afterDeleteReq)
	if afterDeleteResp.Code != http.StatusOK {
		t.Fatalf("list after delete status = %d, body = %s", afterDeleteResp.Code, afterDeleteResp.Body.String())
	}
	var afterDelete envelope[logic.ListFriendsResponse]
	decodeEnvelope(t, afterDeleteResp.Body.Bytes(), &afterDelete)
	if len(afterDelete.Data.Friends) != 0 {
		t.Fatalf("deleted friendship should not be listed: %+v", afterDelete.Data.Friends)
	}
}

func createFriendTestUser(t *testing.T, ctx context.Context, userLogic *logic.UserLogic, identifier string) logic.UserProfile {
	t.Helper()

	user, err := userLogic.CreateUser(ctx, logic.CreateUserRequest{Identifier: identifier})
	if err != nil {
		t.Fatalf("create %s: %v", identifier, err)
	}
	return user
}
