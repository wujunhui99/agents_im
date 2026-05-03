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

func TestFriendsLogicPendingAcceptRejectDeleteAndList(t *testing.T) {
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
	if !added.Created || added.Friendship.IsFriend || added.Friendship.Status != model.FriendshipStatusPending {
		t.Fatalf("unexpected add response: %+v", added)
	}
	if added.Friendship.Friend == nil || added.Friendship.Friend.Identifier != bob.Identifier {
		t.Fatalf("add response should include target profile: %+v", added.Friendship)
	}

	duplicate, err := friendsLogic.AddFriend(ctx, logic.AddFriendRequest{UserID: alice.UserID, FriendID: bob.UserID})
	if err != nil {
		t.Fatalf("duplicate add friend: %v", err)
	}
	if duplicate.Created || duplicate.Friendship.Status != model.FriendshipStatusPending || duplicate.Friendship.IsFriend {
		t.Fatalf("duplicate pending add should be idempotent: %+v", duplicate)
	}

	aliceList, err := friendsLogic.ListFriends(ctx, logic.ListFriendsRequest{UserID: alice.UserID})
	if err != nil {
		t.Fatalf("list alice friends: %v", err)
	}
	if len(aliceList.Friends) != 0 {
		t.Fatalf("pending friendship should not be listed for alice: %+v", aliceList.Friends)
	}

	bobList, err := friendsLogic.ListFriends(ctx, logic.ListFriendsRequest{UserID: bob.UserID})
	if err != nil {
		t.Fatalf("list bob friends: %v", err)
	}
	if len(bobList.Friends) != 0 {
		t.Fatalf("pending friendship should not be listed for bob: %+v", bobList.Friends)
	}

	aliceRequests, err := friendsLogic.ListFriendRequests(ctx, logic.ListFriendRequestsRequest{UserID: alice.UserID})
	if err != nil {
		t.Fatalf("list alice requests: %v", err)
	}
	if len(aliceRequests.Incoming) != 0 || len(aliceRequests.Outgoing) != 1 || aliceRequests.Outgoing[0].FriendID != bob.UserID {
		t.Fatalf("unexpected alice requests: %+v", aliceRequests)
	}

	bobRequests, err := friendsLogic.ListFriendRequests(ctx, logic.ListFriendRequestsRequest{UserID: bob.UserID})
	if err != nil {
		t.Fatalf("list bob requests: %v", err)
	}
	if len(bobRequests.Incoming) != 1 || bobRequests.Incoming[0].FriendID != alice.UserID || len(bobRequests.Outgoing) != 0 {
		t.Fatalf("unexpected bob requests: %+v", bobRequests)
	}
	if bobRequests.Incoming[0].Friend == nil || bobRequests.Incoming[0].Friend.Identifier != alice.Identifier {
		t.Fatalf("incoming request should include requester profile: %+v", bobRequests.Incoming[0])
	}

	if _, err := friendsLogic.AcceptFriend(ctx, logic.AcceptFriendRequest{UserID: alice.UserID, FriendID: bob.UserID}); err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("requester accept error = %v, want FORBIDDEN", err)
	}
	if _, err := friendsLogic.RejectFriend(ctx, logic.RejectFriendRequest{UserID: alice.UserID, FriendID: bob.UserID}); err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("requester reject error = %v, want FORBIDDEN", err)
	}

	accepted, err := friendsLogic.AcceptFriend(ctx, logic.AcceptFriendRequest{UserID: bob.UserID, FriendID: alice.UserID})
	if err != nil {
		t.Fatalf("accept friend: %v", err)
	}
	if !accepted.Accepted || !accepted.Friendship.IsFriend || accepted.Friendship.Status != model.FriendshipStatusAccepted {
		t.Fatalf("unexpected accept response: %+v", accepted)
	}

	aliceList, err = friendsLogic.ListFriends(ctx, logic.ListFriendsRequest{UserID: alice.UserID})
	if err != nil {
		t.Fatalf("list alice accepted friends: %v", err)
	}
	if len(aliceList.Friends) != 1 || aliceList.Friends[0].FriendID != bob.UserID || aliceList.Friends[0].Friend == nil || aliceList.Friends[0].Friend.Identifier != bob.Identifier {
		t.Fatalf("unexpected alice accepted friends: %+v", aliceList.Friends)
	}

	bobList, err = friendsLogic.ListFriends(ctx, logic.ListFriendsRequest{UserID: bob.UserID})
	if err != nil {
		t.Fatalf("list bob accepted friends: %v", err)
	}
	if len(bobList.Friends) != 1 || bobList.Friends[0].FriendID != alice.UserID {
		t.Fatalf("accepted friendship should be bidirectional: %+v", bobList.Friends)
	}

	acceptedDuplicate, err := friendsLogic.AddFriend(ctx, logic.AddFriendRequest{UserID: alice.UserID, FriendID: bob.UserID})
	if err != nil {
		t.Fatalf("duplicate accepted add friend: %v", err)
	}
	if acceptedDuplicate.Created || !acceptedDuplicate.Friendship.IsFriend || acceptedDuplicate.Friendship.Status != model.FriendshipStatusAccepted {
		t.Fatalf("duplicate accepted add should be idempotent: %+v", acceptedDuplicate)
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

	readded, err := friendsLogic.AddFriend(ctx, logic.AddFriendRequest{UserID: alice.UserID, FriendID: bob.UserID})
	if err != nil {
		t.Fatalf("re-add deleted friend: %v", err)
	}
	if !readded.Created || readded.Friendship.IsFriend || readded.Friendship.Status != model.FriendshipStatusPending {
		t.Fatalf("re-add should create a new pending request: %+v", readded)
	}

	rejected, err := friendsLogic.RejectFriend(ctx, logic.RejectFriendRequest{UserID: bob.UserID, FriendID: alice.UserID})
	if err != nil {
		t.Fatalf("reject friend: %v", err)
	}
	if !rejected.Rejected || rejected.Friendship.IsFriend || rejected.Friendship.Status != model.FriendshipStatusRejected {
		t.Fatalf("unexpected reject response: %+v", rejected)
	}

	afterReject, err := friendsLogic.ListFriends(ctx, logic.ListFriendsRequest{UserID: alice.UserID})
	if err != nil {
		t.Fatalf("list after reject: %v", err)
	}
	if len(afterReject.Friends) != 0 {
		t.Fatalf("rejected friendship should not be listed: %+v", afterReject.Friends)
	}

	rejectedFriendship, err := friendsLogic.GetFriendship(ctx, logic.GetFriendshipRequest{UserID: alice.UserID, FriendID: bob.UserID})
	if err != nil {
		t.Fatalf("get rejected friendship: %v", err)
	}
	if rejectedFriendship.Friendship.IsFriend || rejectedFriendship.Friendship.Status != model.FriendshipStatusRejected {
		t.Fatalf("rejected friendship should be inactive: %+v", rejectedFriendship.Friendship)
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

func TestFriendsLogicNeverAddedStatusIsNone(t *testing.T) {
	repo := repository.NewMemoryRepository()
	userLogic := logic.NewUserLogic(repo)
	friendsLogic := logic.NewFriendsLogic(repo, userLogic)
	ctx := context.Background()
	alice := createFriendTestUser(t, ctx, userLogic, "alice_none")
	bob := createFriendTestUser(t, ctx, userLogic, "bob_none")

	friendship, err := friendsLogic.GetFriendship(ctx, logic.GetFriendshipRequest{UserID: alice.UserID, FriendID: bob.UserID})
	if err != nil {
		t.Fatalf("get never-added friendship: %v", err)
	}
	if friendship.Friendship.IsFriend || friendship.Friendship.Status != model.FriendshipStatusNone {
		t.Fatalf("never-added friendship should be none: %+v", friendship.Friendship)
	}
}

func TestFriendsHTTPHandlers(t *testing.T) {
	serviceContext := svc.NewServiceContextWithAuth(repository.NewMemoryRepository(), testJWTAuthConfig())
	mux := newFriendsGoZeroRouter(t, serviceContext)
	ctx := context.Background()

	alice := createFriendTestUser(t, ctx, serviceContext.UserLogic, "alice_http")
	bob := createFriendTestUser(t, ctx, serviceContext.UserLogic, "bob_http")
	aliceBearer := bearerTokenForUser(t, alice.UserID)
	bobBearer := bearerTokenForUser(t, bob.UserID)

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
	if added.Data.Friendship.IsFriend || added.Data.Friendship.Status != model.FriendshipStatusPending {
		t.Fatalf("add should create pending request: %+v", added.Data.Friendship)
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
	if duplicate.Data.Created || duplicate.Data.Friendship.Status != model.FriendshipStatusPending {
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
	if len(list.Data.Friends) != 0 {
		t.Fatalf("pending friendship should not be listed: %+v", list.Data.Friends)
	}

	requestsResp := httptest.NewRecorder()
	requestsReq := httptest.NewRequest(http.MethodGet, "/friends/requests", nil)
	requestsReq.Header.Set("Authorization", aliceBearer)
	mux.ServeHTTP(requestsResp, requestsReq)
	if requestsResp.Code != http.StatusOK {
		t.Fatalf("list requests status = %d, body = %s", requestsResp.Code, requestsResp.Body.String())
	}
	var requests envelope[logic.ListFriendRequestsResponse]
	decodeEnvelope(t, requestsResp.Body.Bytes(), &requests)
	if len(requests.Data.Outgoing) != 1 || requests.Data.Outgoing[0].FriendID != bob.UserID || len(requests.Data.Incoming) != 0 {
		t.Fatalf("unexpected alice request response: %+v", requests.Data)
	}

	forbiddenAcceptResp := httptest.NewRecorder()
	forbiddenAcceptReq := httptest.NewRequest(http.MethodPost, "/friends/"+bob.UserID+"/accept", nil)
	forbiddenAcceptReq.Header.Set("Authorization", aliceBearer)
	mux.ServeHTTP(forbiddenAcceptResp, forbiddenAcceptReq)
	if forbiddenAcceptResp.Code != http.StatusForbidden {
		t.Fatalf("requester accept status = %d, body = %s", forbiddenAcceptResp.Code, forbiddenAcceptResp.Body.String())
	}

	acceptResp := httptest.NewRecorder()
	acceptReq := httptest.NewRequest(http.MethodPost, "/friends/"+alice.UserID+"/accept", nil)
	acceptReq.Header.Set("Authorization", bobBearer)
	mux.ServeHTTP(acceptResp, acceptReq)
	if acceptResp.Code != http.StatusOK {
		t.Fatalf("accept status = %d, body = %s", acceptResp.Code, acceptResp.Body.String())
	}
	var accepted envelope[logic.AcceptFriendResponse]
	decodeEnvelope(t, acceptResp.Body.Bytes(), &accepted)
	if !accepted.Data.Accepted || !accepted.Data.Friendship.IsFriend || accepted.Data.Friendship.Status != model.FriendshipStatusAccepted {
		t.Fatalf("unexpected accept response: %+v", accepted.Data)
	}

	listResp = httptest.NewRecorder()
	listReq = httptest.NewRequest(http.MethodGet, "/friends", nil)
	listReq.Header.Set("Authorization", aliceBearer)
	mux.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list accepted status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	decodeEnvelope(t, listResp.Body.Bytes(), &list)
	if len(list.Data.Friends) != 1 || list.Data.Friends[0].FriendID != bob.UserID {
		t.Fatalf("unexpected accepted list response: %+v", list.Data.Friends)
	}
	if list.Data.Friends[0].Friend.UserID != bob.UserID || list.Data.Friends[0].Friend.Identifier != bob.Identifier {
		t.Fatalf("list response should include friend profile for chat open: %+v", list.Data.Friends[0])
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
	if !got.Data.Friendship.IsFriend || got.Data.Friendship.Status != model.FriendshipStatusAccepted {
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

	readdResp := httptest.NewRecorder()
	readdReq := newJSONRequest(http.MethodPost, "/friends", fmt.Sprintf(`{"user_id":"%s"}`, bob.UserID))
	readdReq.Header.Set("Authorization", aliceBearer)
	mux.ServeHTTP(readdResp, readdReq)
	if readdResp.Code != http.StatusOK {
		t.Fatalf("re-add status = %d, body = %s", readdResp.Code, readdResp.Body.String())
	}

	rejectResp := httptest.NewRecorder()
	rejectReq := httptest.NewRequest(http.MethodPost, "/friends/"+alice.UserID+"/reject", nil)
	rejectReq.Header.Set("Authorization", bobBearer)
	mux.ServeHTTP(rejectResp, rejectReq)
	if rejectResp.Code != http.StatusOK {
		t.Fatalf("reject status = %d, body = %s", rejectResp.Code, rejectResp.Body.String())
	}
	var rejected envelope[logic.RejectFriendResponse]
	decodeEnvelope(t, rejectResp.Body.Bytes(), &rejected)
	if !rejected.Data.Rejected || rejected.Data.Friendship.IsFriend || rejected.Data.Friendship.Status != model.FriendshipStatusRejected {
		t.Fatalf("unexpected reject response: %+v", rejected.Data)
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
