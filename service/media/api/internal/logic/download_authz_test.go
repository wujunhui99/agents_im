package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/friends/rpc/friendsclient"
	"github.com/wujunhui99/agents_im/service/groups/rpc/groupsclient"
	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msgclient"
	"google.golang.org/grpc"
)

const (
	azUploader  = "323130844539310080"
	azRequester = "999000111222333444"
	azMediaID   = "58537781550383104"
	azMsgID     = "77001100220033004"
	azGroupID   = "66001100220033004"
)

// --- fakes（窄接口，各只实现一个方法）---

type fakeMediaMeta struct {
	uploader string
	err      error
}

func (f fakeMediaMeta) GetMedia(_ context.Context, _ *mediaclient.GetMediaRequest, _ ...grpc.CallOption) (*mediaclient.MediaObject, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &mediaclient.MediaObject{OwnerUserId: f.uploader, Status: "ready"}, nil
}

type fakeMessageRef struct {
	chatType, groupID, peer, mediaID string
	err                              error
}

func (f fakeMessageRef) GetMessageRef(_ context.Context, _ *msgclient.GetMessageRefRequest, _ ...grpc.CallOption) (*msgclient.GetMessageRefResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &msgclient.GetMessageRefResponse{ChatType: f.chatType, GroupId: f.groupID, PeerAccountId: f.peer, MediaId: f.mediaID}, nil
}

type fakeFriendship struct {
	allow map[string]bool // requester|peer -> requester 仍把对方当好友
}

func (f fakeFriendship) GetFriendship(_ context.Context, in *friendsclient.GetFriendshipRequest, _ ...grpc.CallOption) (*friendsclient.GetFriendshipResponse, error) {
	return &friendsclient.GetFriendshipResponse{Friendship: &friendsclient.Friendship{IsFriend: f.allow[in.GetUserId()+"|"+in.GetFriendId()]}}, nil
}

type fakeMembership struct {
	allow map[string]bool // groupID|user -> active 成员
}

func (f fakeMembership) IsMember(_ context.Context, in *groupsclient.IsMemberRequest, _ ...grpc.CallOption) (*groupsclient.IsMemberResponse, error) {
	return &groupsclient.IsMemberResponse{IsMember: f.allow[in.GetGroupId()+"|"+in.GetUserId()]}, nil
}

// authzCase 跑一个 authorizeDownload，断言错误码（want=="" 表示期望放行）。
func runAuthz(t *testing.T, requester, mediaID, msgID string, media mediaMetaGetter, msg messageRefGetter, friend friendshipGetter, group memberChecker) error {
	t.Helper()
	return authorizeDownload(context.Background(), requester, mediaID, msgID, media, msg, friend, group)
}

func wantForbidden(t *testing.T, err error) {
	t.Helper()
	if err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("expected Forbidden, got %v", err)
	}
}

// uploader 本人放行：无需 msg_id / 跨域校验。
func TestAuthorizeDownloadUploaderAllowed(t *testing.T) {
	err := runAuthz(t, azUploader, azMediaID, "", fakeMediaMeta{uploader: azUploader}, fakeMessageRef{}, fakeFriendship{}, fakeMembership{})
	if err != nil {
		t.Fatalf("uploader should be allowed: %v", err)
	}
}

// 非 uploader 未传 msg_id → InvalidArgument。
func TestAuthorizeDownloadNonUploaderRequiresMsgID(t *testing.T) {
	err := runAuthz(t, azRequester, azMediaID, "", fakeMediaMeta{uploader: azUploader}, fakeMessageRef{}, fakeFriendship{}, fakeMembership{})
	if err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

// 链路不符：消息引用的 media_id 与入参不一致 → 拒绝（防越权，§4 核心）。
func TestAuthorizeDownloadLinkMismatchForbidden(t *testing.T) {
	msg := fakeMessageRef{chatType: chatTypeSingle, peer: azUploader, mediaID: "111111111111111111"}
	friend := fakeFriendship{allow: map[string]bool{azRequester + "|" + azUploader: true}}
	err := runAuthz(t, azRequester, azMediaID, azMsgID, fakeMediaMeta{uploader: azUploader}, msg, friend, fakeMembership{})
	wantForbidden(t, err)
}

// 私聊：requester 仍把对方当好友（含对方已删 requester——只看 requester→peer）→ 放行。
func TestAuthorizeDownloadPrivateFriendAllowed(t *testing.T) {
	msg := fakeMessageRef{chatType: chatTypeSingle, peer: azUploader, mediaID: azMediaID}
	friend := fakeFriendship{allow: map[string]bool{azRequester + "|" + azUploader: true}}
	if err := runAuthz(t, azRequester, azMediaID, azMsgID, fakeMediaMeta{uploader: azUploader}, msg, friend, fakeMembership{}); err != nil {
		t.Fatalf("private friend should be allowed: %v", err)
	}
}

// 私聊：requester 已删对方（requester→peer 不在）→ 拒绝。
func TestAuthorizeDownloadPrivateRequesterDeletedPeerForbidden(t *testing.T) {
	msg := fakeMessageRef{chatType: chatTypeSingle, peer: azUploader, mediaID: azMediaID}
	err := runAuthz(t, azRequester, azMediaID, azMsgID, fakeMediaMeta{uploader: azUploader}, msg, fakeFriendship{allow: map[string]bool{}}, fakeMembership{})
	wantForbidden(t, err)
}

// 群聊：requester 是 active 成员 → 放行。
func TestAuthorizeDownloadGroupMemberAllowed(t *testing.T) {
	msg := fakeMessageRef{chatType: chatTypeGroup, groupID: azGroupID, mediaID: azMediaID}
	group := fakeMembership{allow: map[string]bool{azGroupID + "|" + azRequester: true}}
	if err := runAuthz(t, azRequester, azMediaID, azMsgID, fakeMediaMeta{uploader: azUploader}, msg, fakeFriendship{}, group); err != nil {
		t.Fatalf("group member should be allowed: %v", err)
	}
}

// 群聊：requester 非成员（已退群）→ 拒绝。
func TestAuthorizeDownloadGroupNonMemberForbidden(t *testing.T) {
	msg := fakeMessageRef{chatType: chatTypeGroup, groupID: azGroupID, mediaID: azMediaID}
	err := runAuthz(t, azRequester, azMediaID, azMsgID, fakeMediaMeta{uploader: azUploader}, msg, fakeFriendship{}, fakeMembership{allow: map[string]bool{}})
	wantForbidden(t, err)
}
