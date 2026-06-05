package logic

import (
	"context"
	"sort"
	"testing"
	"time"

	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- fake model（仅实现被测路径用到的方法，内嵌接口兜底其余）---

type fakeFriendshipsModel struct {
	model.FriendshipsModel
	rows map[string]*model.Friendships // account|friend -> row
}

func newFakeModel() *fakeFriendshipsModel {
	return &fakeFriendshipsModel{rows: map[string]*model.Friendships{}}
}

func pairKey(accountID, friendID string) string { return accountID + "|" + friendID }

func (f *fakeFriendshipsModel) WithSession(sqlx.Session) model.FriendshipsModel { return f }

func (f *fakeFriendshipsModel) Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {
	return fn(ctx, nil)
}

func (f *fakeFriendshipsModel) clone(accountID, friendID string) (*model.Friendships, bool) {
	if r, ok := f.rows[pairKey(accountID, friendID)]; ok {
		c := *r
		return &c, true
	}
	return nil, false
}

func (f *fakeFriendshipsModel) FindPairForUpdate(_ context.Context, accountID, friendID string) (*model.Friendships, error) {
	if c, ok := f.clone(accountID, friendID); ok {
		return c, nil
	}
	return nil, model.ErrNotFound
}

func (f *fakeFriendshipsModel) FindOneByAccountIdFriendAccountId(ctx context.Context, accountID, friendID string) (*model.Friendships, error) {
	return f.FindPairForUpdate(ctx, accountID, friendID)
}

func (f *fakeFriendshipsModel) UpsertStatus(_ context.Context, accountID, friendID string, statusVal int64) (*model.Friendships, error) {
	now := time.Now()
	row := &model.Friendships{AccountId: accountID, FriendAccountId: friendID, Status: statusVal, CreatedAt: now, UpdatedAt: now}
	f.rows[pairKey(accountID, friendID)] = row
	c := *row
	return &c, nil
}

func (f *fakeFriendshipsModel) ListByAccountStatus(_ context.Context, accountID string, statusVal int64) ([]*model.Friendships, error) {
	var out []*model.Friendships
	for _, r := range f.rows {
		if r.AccountId == accountID && r.Status == statusVal {
			c := *r
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FriendAccountId < out[j].FriendAccountId })
	return out, nil
}

func (f *fakeFriendshipsModel) ListByFriendStatus(_ context.Context, friendID string, statusVal int64) ([]*model.Friendships, error) {
	var out []*model.Friendships
	for _, r := range f.rows {
		if r.FriendAccountId == friendID && r.Status == statusVal {
			c := *r
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AccountId < out[j].AccountId })
	return out, nil
}

func (f *fakeFriendshipsModel) MarkAcceptedDeleted(_ context.Context, accountID, friendID string) (*model.Friendships, error) {
	if r, ok := f.rows[pairKey(accountID, friendID)]; ok && r.Status == model.FriendshipStatusAccepted {
		r.Status = model.FriendshipStatusDeleted
		c := *r
		return &c, nil
	}
	return nil, model.ErrNotFound
}

func (f *fakeFriendshipsModel) MarkAcceptedDeletedSilent(_ context.Context, accountID, friendID string) error {
	if r, ok := f.rows[pairKey(accountID, friendID)]; ok && r.Status == model.FriendshipStatusAccepted {
		r.Status = model.FriendshipStatusDeleted
	}
	return nil
}

func newSvc(m model.FriendshipsModel) *svc.ServiceContext {
	return &svc.ServiceContext{FriendshipModel: m}
}

func codeOf(t *testing.T, err error) codes.Code {
	t.Helper()
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error is not a grpc status: %v", err)
	}
	return st.Code()
}

func TestAddFriendCreatesPendingRequest(t *testing.T) {
	ctx := context.Background()
	svcCtx := newSvc(newFakeModel())

	resp, err := NewAddFriendLogic(ctx, svcCtx).AddFriend(&friends.AddFriendRequest{UserId: "alice", FriendId: "bob"})
	if err != nil {
		t.Fatalf("AddFriend returned error: %v", err)
	}
	if !resp.GetCreated() {
		t.Fatalf("Created = false, want true")
	}
	if resp.GetFriendship().GetStatus() != "pending" || resp.GetFriendship().GetIsFriend() {
		t.Fatalf("friendship = %#v, want pending / not friend", resp.GetFriendship())
	}
}

func TestAddFriendAcceptsPendingReverseRequest(t *testing.T) {
	ctx := context.Background()
	svcCtx := newSvc(newFakeModel())

	if _, err := NewAddFriendLogic(ctx, svcCtx).AddFriend(&friends.AddFriendRequest{UserId: "alice", FriendId: "bob"}); err != nil {
		t.Fatalf("first AddFriend returned error: %v", err)
	}
	accepted, err := NewAddFriendLogic(ctx, svcCtx).AddFriend(&friends.AddFriendRequest{UserId: "bob", FriendId: "alice"})
	if err != nil {
		t.Fatalf("reverse AddFriend returned error: %v", err)
	}
	if accepted.GetFriendship().GetStatus() != "accepted" || !accepted.GetFriendship().GetIsFriend() {
		t.Fatalf("friendship = %#v, want accepted / is friend", accepted.GetFriendship())
	}

	aliceList, err := NewListFriendsLogic(ctx, svcCtx).ListFriends(&friends.ListFriendsRequest{UserId: "alice"})
	if err != nil {
		t.Fatalf("ListFriends(alice) error: %v", err)
	}
	if len(aliceList.GetFriends()) != 1 || aliceList.GetFriends()[0].GetFriendId() != "bob" {
		t.Fatalf("alice friends = %#v, want [bob]", aliceList.GetFriends())
	}
}

func TestListRequestsSeparatesIncomingAndOutgoing(t *testing.T) {
	ctx := context.Background()
	svcCtx := newSvc(newFakeModel())

	if _, err := NewAddFriendLogic(ctx, svcCtx).AddFriend(&friends.AddFriendRequest{UserId: "alice", FriendId: "bob"}); err != nil {
		t.Fatalf("AddFriend error: %v", err)
	}

	bobReq, err := NewListFriendRequestsLogic(ctx, svcCtx).ListFriendRequests(&friends.ListFriendRequestsRequest{UserId: "bob"})
	if err != nil {
		t.Fatalf("ListFriendRequests(bob) error: %v", err)
	}
	if len(bobReq.GetIncoming()) != 1 || bobReq.GetIncoming()[0].GetUserId() != "alice" || len(bobReq.GetOutgoing()) != 0 {
		t.Fatalf("bob requests incoming=%#v outgoing=%#v", bobReq.GetIncoming(), bobReq.GetOutgoing())
	}

	aliceReq, err := NewListFriendRequestsLogic(ctx, svcCtx).ListFriendRequests(&friends.ListFriendRequestsRequest{UserId: "alice"})
	if err != nil {
		t.Fatalf("ListFriendRequests(alice) error: %v", err)
	}
	if len(aliceReq.GetOutgoing()) != 1 || aliceReq.GetOutgoing()[0].GetFriendId() != "bob" || len(aliceReq.GetIncoming()) != 0 {
		t.Fatalf("alice requests incoming=%#v outgoing=%#v", aliceReq.GetIncoming(), aliceReq.GetOutgoing())
	}
}

func TestAcceptFriendRequestAcceptsBothDirections(t *testing.T) {
	ctx := context.Background()
	svcCtx := newSvc(newFakeModel())

	if _, err := NewAddFriendLogic(ctx, svcCtx).AddFriend(&friends.AddFriendRequest{UserId: "alice", FriendId: "bob"}); err != nil {
		t.Fatalf("AddFriend error: %v", err)
	}
	accepted, err := NewAcceptFriendRequestLogic(ctx, svcCtx).AcceptFriendRequest(&friends.FriendRequestDecisionRequest{UserId: "bob", FriendId: "alice"})
	if err != nil {
		t.Fatalf("AcceptFriendRequest error: %v", err)
	}
	if accepted.GetFriendship().GetStatus() != "accepted" || !accepted.GetUpdated() {
		t.Fatalf("accepted = %#v", accepted)
	}
	bobList, err := NewListFriendsLogic(ctx, svcCtx).ListFriends(&friends.ListFriendsRequest{UserId: "bob"})
	if err != nil {
		t.Fatalf("ListFriends(bob) error: %v", err)
	}
	if len(bobList.GetFriends()) != 1 || bobList.GetFriends()[0].GetFriendId() != "alice" {
		t.Fatalf("bob friends = %#v, want [alice]", bobList.GetFriends())
	}
}

func TestAcceptFriendRequestMissingReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	svcCtx := newSvc(newFakeModel())

	_, err := NewAcceptFriendRequestLogic(ctx, svcCtx).AcceptFriendRequest(&friends.FriendRequestDecisionRequest{UserId: "bob", FriendId: "alice"})
	if err == nil || codeOf(t, err) != codes.NotFound {
		t.Fatalf("err = %v, want NotFound", err)
	}
}

func TestRejectFriendRequestRemovesFromPending(t *testing.T) {
	ctx := context.Background()
	svcCtx := newSvc(newFakeModel())

	if _, err := NewAddFriendLogic(ctx, svcCtx).AddFriend(&friends.AddFriendRequest{UserId: "alice", FriendId: "bob"}); err != nil {
		t.Fatalf("AddFriend error: %v", err)
	}
	rejected, err := NewRejectFriendRequestLogic(ctx, svcCtx).RejectFriendRequest(&friends.FriendRequestDecisionRequest{UserId: "bob", FriendId: "alice"})
	if err != nil {
		t.Fatalf("RejectFriendRequest error: %v", err)
	}
	if rejected.GetFriendship().GetStatus() != "rejected" || rejected.GetFriendship().GetIsFriend() {
		t.Fatalf("rejected = %#v", rejected.GetFriendship())
	}
	bobReq, err := NewListFriendRequestsLogic(ctx, svcCtx).ListFriendRequests(&friends.ListFriendRequestsRequest{UserId: "bob"})
	if err != nil {
		t.Fatalf("ListFriendRequests(bob) error: %v", err)
	}
	if len(bobReq.GetIncoming()) != 0 || len(bobReq.GetOutgoing()) != 0 {
		t.Fatalf("bob requests not empty: %#v / %#v", bobReq.GetIncoming(), bobReq.GetOutgoing())
	}
}

func TestDeleteFriendRequiresAcceptedFriendship(t *testing.T) {
	ctx := context.Background()
	svcCtx := newSvc(newFakeModel())

	_, err := NewDeleteFriendLogic(ctx, svcCtx).DeleteFriend(&friends.DeleteFriendRequest{UserId: "alice", FriendId: "bob"})
	if err == nil || codeOf(t, err) != codes.NotFound {
		t.Fatalf("err = %v, want NotFound", err)
	}

	// 建立 accepted 后删除成功，反向也被置 deleted。
	if _, err := NewAddFriendLogic(ctx, svcCtx).AddFriend(&friends.AddFriendRequest{UserId: "alice", FriendId: "bob"}); err != nil {
		t.Fatalf("AddFriend error: %v", err)
	}
	if _, err := NewAcceptFriendRequestLogic(ctx, svcCtx).AcceptFriendRequest(&friends.FriendRequestDecisionRequest{UserId: "bob", FriendId: "alice"}); err != nil {
		t.Fatalf("AcceptFriendRequest error: %v", err)
	}
	deleted, err := NewDeleteFriendLogic(ctx, svcCtx).DeleteFriend(&friends.DeleteFriendRequest{UserId: "alice", FriendId: "bob"})
	if err != nil {
		t.Fatalf("DeleteFriend error: %v", err)
	}
	if !deleted.GetDeleted() || deleted.GetFriendship().GetStatus() != "deleted" {
		t.Fatalf("deleted = %#v", deleted)
	}
}

func TestGetFriendshipReturnsNoneAndReversePending(t *testing.T) {
	ctx := context.Background()
	svcCtx := newSvc(newFakeModel())

	none, err := NewGetFriendshipLogic(ctx, svcCtx).GetFriendship(&friends.GetFriendshipRequest{UserId: "alice", FriendId: "bob"})
	if err != nil {
		t.Fatalf("GetFriendship error: %v", err)
	}
	if none.GetFriendship().GetStatus() != "none" {
		t.Fatalf("status = %q, want none", none.GetFriendship().GetStatus())
	}

	// alice 发请求后，bob 视角看到的是反向 pending。
	if _, err := NewAddFriendLogic(ctx, svcCtx).AddFriend(&friends.AddFriendRequest{UserId: "alice", FriendId: "bob"}); err != nil {
		t.Fatalf("AddFriend error: %v", err)
	}
	reverse, err := NewGetFriendshipLogic(ctx, svcCtx).GetFriendship(&friends.GetFriendshipRequest{UserId: "bob", FriendId: "alice"})
	if err != nil {
		t.Fatalf("GetFriendship error: %v", err)
	}
	if reverse.GetFriendship().GetStatus() != "pending" {
		t.Fatalf("status = %q, want pending", reverse.GetFriendship().GetStatus())
	}
}

func TestValidationRejectsBadPairs(t *testing.T) {
	ctx := context.Background()
	svcCtx := newSvc(newFakeModel())

	if _, err := NewAddFriendLogic(ctx, svcCtx).AddFriend(&friends.AddFriendRequest{UserId: "alice", FriendId: "alice"}); err == nil || codeOf(t, err) != codes.InvalidArgument {
		t.Fatalf("same-id err = %v, want InvalidArgument", err)
	}
	if _, err := NewAddFriendLogic(ctx, svcCtx).AddFriend(&friends.AddFriendRequest{UserId: "", FriendId: "bob"}); err == nil || codeOf(t, err) != codes.InvalidArgument {
		t.Fatalf("empty user_id err = %v, want InvalidArgument", err)
	}
}
