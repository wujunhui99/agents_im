package friends

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListFriendRequestsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListFriendRequestsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFriendRequestsLogic {
	return &ListFriendRequestsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListFriendRequestsLogic) ListFriendRequests(_ *types.ListFriendRequestsReq) (*types.ListFriendRequestsResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.FriendsLogic.ListFriendRequests(l.ctx, business.ListFriendRequestsRequest{UserID: userID})
	if err != nil {
		return nil, err
	}

	incoming := make([]types.Friendship, 0, len(result.Incoming))
	for _, friendship := range result.Incoming {
		incoming = append(incoming, toFriendship(friendship))
	}
	outgoing := make([]types.Friendship, 0, len(result.Outgoing))
	for _, friendship := range result.Outgoing {
		outgoing = append(outgoing, toFriendship(friendship))
	}

	return &types.ListFriendRequestsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.ListFriendRequestsData{
			Incoming: incoming,
			Outgoing: outgoing,
		},
	}, nil
}

type AcceptFriendRequestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAcceptFriendRequestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AcceptFriendRequestLogic {
	return &AcceptFriendRequestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AcceptFriendRequestLogic) AcceptFriendRequest(req *types.FriendPathReq) (*types.FriendRequestDecisionResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.FriendsLogic.AcceptFriendRequest(l.ctx, business.FriendRequestDecisionRequest{
		UserID:   userID,
		FriendID: req.UserID,
	})
	if err != nil {
		return nil, err
	}
	return &types.FriendRequestDecisionResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.FriendRequestDecisionData{
			Friendship: toFriendship(result.Friendship),
			Updated:    result.Updated,
		},
	}, nil
}

type RejectFriendRequestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRejectFriendRequestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RejectFriendRequestLogic {
	return &RejectFriendRequestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RejectFriendRequestLogic) RejectFriendRequest(req *types.FriendPathReq) (*types.FriendRequestDecisionResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.FriendsLogic.RejectFriendRequest(l.ctx, business.FriendRequestDecisionRequest{
		UserID:   userID,
		FriendID: req.UserID,
	})
	if err != nil {
		return nil, err
	}
	return &types.FriendRequestDecisionResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.FriendRequestDecisionData{
			Friendship: toFriendship(result.Friendship),
			Updated:    result.Updated,
		},
	}, nil
}
