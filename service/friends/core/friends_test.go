package core

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

type friendsLogicUserLookup struct {
	users map[string]UserProfile
}

func (l friendsLogicUserLookup) GetUserByID(_ context.Context, userID string) (UserProfile, error) {
	if user, ok := l.users[userID]; ok {
		return user, nil
	}
	return UserProfile{}, nil
}

func newFriendsLogicForTest() (*FriendsLogic, context.Context) {
	ctx := context.Background()
	repo := repository.NewMemoryRepository()
	logic := NewFriendsLogic(repo, friendsLogicUserLookup{users: map[string]UserProfile{
		"alice": {UserID: "alice", Identifier: "alice_001", DisplayName: "Alice"},
		"bob":   {UserID: "bob", Identifier: "bob_002", DisplayName: "Bob"},
	}})
	return logic, ctx
}

func TestFriendsLogicAddFriendCreatesPendingRequest(t *testing.T) {
	logic, ctx := newFriendsLogicForTest()

	result, err := logic.AddFriend(ctx, AddFriendRequest{UserID: "alice", FriendID: "bob"})
	if err != nil {
		t.Fatalf("AddFriend returned error: %v", err)
	}
	if !result.Created {
		t.Fatalf("Created = false, want true for new friend request")
	}
	if result.Friendship.Status != model.FriendshipStatusPending {
		t.Fatalf("status = %q, want %q", result.Friendship.Status, model.FriendshipStatusPending)
	}
	if result.Friendship.IsFriend {
		t.Fatalf("IsFriend = true, want false while request is pending")
	}

	aliceList, err := logic.ListFriends(ctx, ListFriendsRequest{UserID: "alice"})
	if err != nil {
		t.Fatalf("ListFriends(alice) returned error: %v", err)
	}
	if len(aliceList.Friends) != 0 {
		t.Fatalf("alice visible friends = %d, want 0 before acceptance", len(aliceList.Friends))
	}

	bobView, err := logic.GetFriendship(ctx, GetFriendshipRequest{UserID: "bob", FriendID: "alice"})
	if err != nil {
		t.Fatalf("GetFriendship(bob, alice) returned error: %v", err)
	}
	if bobView.Friendship.Status != model.FriendshipStatusPending {
		t.Fatalf("bob reverse status = %q, want %q", bobView.Friendship.Status, model.FriendshipStatusPending)
	}
	if bobView.Friendship.IsFriend {
		t.Fatalf("bob reverse IsFriend = true, want false before acceptance")
	}
}

func TestFriendsLogicAddFriendAcceptsPendingReverseRequest(t *testing.T) {
	logic, ctx := newFriendsLogicForTest()

	if _, err := logic.AddFriend(ctx, AddFriendRequest{UserID: "alice", FriendID: "bob"}); err != nil {
		t.Fatalf("initial AddFriend returned error: %v", err)
	}
	accepted, err := logic.AddFriend(ctx, AddFriendRequest{UserID: "bob", FriendID: "alice"})
	if err != nil {
		t.Fatalf("reverse AddFriend returned error: %v", err)
	}
	if !accepted.Created {
		t.Fatalf("Created = false, want true for accepting pending reverse request")
	}
	if accepted.Friendship.Status != model.FriendshipStatusAccepted {
		t.Fatalf("status = %q, want %q", accepted.Friendship.Status, model.FriendshipStatusAccepted)
	}
	if !accepted.Friendship.IsFriend {
		t.Fatalf("IsFriend = false, want true after acceptance")
	}

	aliceList, err := logic.ListFriends(ctx, ListFriendsRequest{UserID: "alice"})
	if err != nil {
		t.Fatalf("ListFriends(alice) returned error: %v", err)
	}
	bobList, err := logic.ListFriends(ctx, ListFriendsRequest{UserID: "bob"})
	if err != nil {
		t.Fatalf("ListFriends(bob) returned error: %v", err)
	}
	if len(aliceList.Friends) != 1 || aliceList.Friends[0].FriendID != "bob" || !aliceList.Friends[0].IsFriend {
		t.Fatalf("alice friends = %#v, want accepted bob", aliceList.Friends)
	}
	if len(bobList.Friends) != 1 || bobList.Friends[0].FriendID != "alice" || !bobList.Friends[0].IsFriend {
		t.Fatalf("bob friends = %#v, want accepted alice", bobList.Friends)
	}
}

func TestFriendsLogicListRequestsSeparatesIncomingAndOutgoing(t *testing.T) {
	logic, ctx := newFriendsLogicForTest()
	if _, err := logic.AddFriend(ctx, AddFriendRequest{UserID: "alice", FriendID: "bob"}); err != nil {
		t.Fatalf("AddFriend returned error: %v", err)
	}

	requests, err := logic.ListFriendRequests(ctx, ListFriendRequestsRequest{UserID: "bob"})
	if err != nil {
		t.Fatalf("ListFriendRequests(bob) returned error: %v", err)
	}
	if len(requests.Incoming) != 1 || requests.Incoming[0].UserID != "alice" || requests.Incoming[0].FriendID != "bob" {
		t.Fatalf("incoming = %#v, want alice -> bob pending", requests.Incoming)
	}
	if len(requests.Outgoing) != 0 {
		t.Fatalf("bob outgoing = %#v, want empty", requests.Outgoing)
	}

	aliceRequests, err := logic.ListFriendRequests(ctx, ListFriendRequestsRequest{UserID: "alice"})
	if err != nil {
		t.Fatalf("ListFriendRequests(alice) returned error: %v", err)
	}
	if len(aliceRequests.Incoming) != 0 {
		t.Fatalf("alice incoming = %#v, want empty", aliceRequests.Incoming)
	}
	if len(aliceRequests.Outgoing) != 1 || aliceRequests.Outgoing[0].UserID != "alice" || aliceRequests.Outgoing[0].FriendID != "bob" {
		t.Fatalf("alice outgoing = %#v, want alice -> bob pending", aliceRequests.Outgoing)
	}
}

func TestFriendsLogicAcceptFriendRequestAcceptsBothDirections(t *testing.T) {
	logic, ctx := newFriendsLogicForTest()
	if _, err := logic.AddFriend(ctx, AddFriendRequest{UserID: "alice", FriendID: "bob"}); err != nil {
		t.Fatalf("AddFriend returned error: %v", err)
	}

	accepted, err := logic.AcceptFriendRequest(ctx, FriendRequestDecisionRequest{UserID: "bob", FriendID: "alice"})
	if err != nil {
		t.Fatalf("AcceptFriendRequest returned error: %v", err)
	}
	if accepted.Friendship.Status != model.FriendshipStatusAccepted || !accepted.Friendship.IsFriend {
		t.Fatalf("accepted = %#v, want accepted friend", accepted.Friendship)
	}

	aliceList, err := logic.ListFriends(ctx, ListFriendsRequest{UserID: "alice"})
	if err != nil {
		t.Fatalf("ListFriends(alice) returned error: %v", err)
	}
	bobList, err := logic.ListFriends(ctx, ListFriendsRequest{UserID: "bob"})
	if err != nil {
		t.Fatalf("ListFriends(bob) returned error: %v", err)
	}
	if len(aliceList.Friends) != 1 || aliceList.Friends[0].Status != model.FriendshipStatusAccepted {
		t.Fatalf("alice friends = %#v, want accepted bob", aliceList.Friends)
	}
	if len(bobList.Friends) != 1 || bobList.Friends[0].Status != model.FriendshipStatusAccepted {
		t.Fatalf("bob friends = %#v, want accepted alice", bobList.Friends)
	}
}

func TestFriendsLogicRejectFriendRequestRemovesFromPendingLists(t *testing.T) {
	logic, ctx := newFriendsLogicForTest()
	if _, err := logic.AddFriend(ctx, AddFriendRequest{UserID: "alice", FriendID: "bob"}); err != nil {
		t.Fatalf("AddFriend returned error: %v", err)
	}

	rejected, err := logic.RejectFriendRequest(ctx, FriendRequestDecisionRequest{UserID: "bob", FriendID: "alice"})
	if err != nil {
		t.Fatalf("RejectFriendRequest returned error: %v", err)
	}
	if rejected.Friendship.Status != model.FriendshipStatusRejected || rejected.Friendship.IsFriend {
		t.Fatalf("rejected = %#v, want rejected non-friend", rejected.Friendship)
	}

	bobRequests, err := logic.ListFriendRequests(ctx, ListFriendRequestsRequest{UserID: "bob"})
	if err != nil {
		t.Fatalf("ListFriendRequests(bob) returned error: %v", err)
	}
	if len(bobRequests.Incoming) != 0 || len(bobRequests.Outgoing) != 0 {
		t.Fatalf("bob requests = %#v, want empty after reject", bobRequests)
	}
	aliceView, err := logic.GetFriendship(ctx, GetFriendshipRequest{UserID: "alice", FriendID: "bob"})
	if err != nil {
		t.Fatalf("GetFriendship(alice,bob) returned error: %v", err)
	}
	if aliceView.Friendship.Status != model.FriendshipStatusRejected {
		t.Fatalf("alice view status = %q, want rejected", aliceView.Friendship.Status)
	}
}
