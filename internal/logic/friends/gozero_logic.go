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

type AddFriendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddFriendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddFriendLogic {
	return &AddFriendLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddFriendLogic) AddFriend(req *types.AddFriendReq) (*types.AddFriendResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.FriendsLogic.AddFriend(l.ctx, business.AddFriendRequest{
		UserID:   userID,
		FriendID: req.UserID,
	})
	if err != nil {
		return nil, err
	}
	return &types.AddFriendResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.AddFriendData{
			Friendship: toFriendship(result.Friendship),
			Created:    result.Created,
		},
	}, nil
}

type AcceptFriendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAcceptFriendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AcceptFriendLogic {
	return &AcceptFriendLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AcceptFriendLogic) AcceptFriend(req *types.FriendPathReq) (*types.AcceptFriendResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.FriendsLogic.AcceptFriend(l.ctx, business.AcceptFriendRequest{
		UserID:   userID,
		FriendID: req.UserID,
	})
	if err != nil {
		return nil, err
	}
	return &types.AcceptFriendResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.AcceptFriendData{
			Friendship: toFriendship(result.Friendship),
			Accepted:   result.Accepted,
		},
	}, nil
}

type RejectFriendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRejectFriendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RejectFriendLogic {
	return &RejectFriendLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RejectFriendLogic) RejectFriend(req *types.FriendPathReq) (*types.RejectFriendResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.FriendsLogic.RejectFriend(l.ctx, business.RejectFriendRequest{
		UserID:   userID,
		FriendID: req.UserID,
	})
	if err != nil {
		return nil, err
	}
	return &types.RejectFriendResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.RejectFriendData{
			Friendship: toFriendship(result.Friendship),
			Rejected:   result.Rejected,
		},
	}, nil
}

type DeleteFriendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteFriendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteFriendLogic {
	return &DeleteFriendLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteFriendLogic) DeleteFriend(req *types.FriendPathReq) (*types.DeleteFriendResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.FriendsLogic.DeleteFriend(l.ctx, business.DeleteFriendRequest{
		UserID:   userID,
		FriendID: req.UserID,
	})
	if err != nil {
		return nil, err
	}
	return &types.DeleteFriendResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.DeleteFriendData{
			Friendship: toFriendship(result.Friendship),
			Deleted:    result.Deleted,
		},
	}, nil
}

type GetFriendshipLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetFriendshipLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFriendshipLogic {
	return &GetFriendshipLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetFriendshipLogic) GetFriendship(req *types.FriendPathReq) (*types.FriendshipResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.FriendsLogic.GetFriendship(l.ctx, business.GetFriendshipRequest{
		UserID:   userID,
		FriendID: req.UserID,
	})
	if err != nil {
		return nil, err
	}
	return &types.FriendshipResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.FriendshipData{
			Friendship: toFriendship(result.Friendship),
		},
	}, nil
}

type ListFriendsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListFriendsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFriendsLogic {
	return &ListFriendsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListFriendsLogic) ListFriends(req *types.ListFriendsReq) (*types.ListFriendsResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.FriendsLogic.ListFriends(l.ctx, business.ListFriendsRequest{UserID: userID})
	if err != nil {
		return nil, err
	}

	friends := make([]types.Friendship, 0, len(result.Friends))
	for _, friendship := range result.Friends {
		friends = append(friends, toFriendship(friendship))
	}
	return &types.ListFriendsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    types.ListFriendsData{Friends: friends},
	}, nil
}

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

func (l *ListFriendRequestsLogic) ListFriendRequests(req *types.ListFriendRequestsReq) (*types.ListFriendRequestsResp, error) {
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

func toFriendship(friendship business.FriendshipView) types.Friendship {
	view := types.Friendship{
		UserID:    friendship.UserID,
		FriendID:  friendship.FriendID,
		Status:    friendship.Status,
		IsFriend:  friendship.IsFriend,
		CreatedAt: friendship.CreatedAt,
		UpdatedAt: friendship.UpdatedAt,
	}
	if friendship.Friend != nil {
		view.Friend = toFriendProfile(*friendship.Friend)
	}
	return view
}

func toFriendProfile(profile business.UserProfile) types.FriendProfile {
	return types.FriendProfile{
		UserID:        profile.UserID,
		Identifier:    profile.Identifier,
		DisplayName:   profile.DisplayName,
		Name:          profile.Name,
		Gender:        profile.Gender,
		BirthDate:     profile.BirthDate,
		Region:        profile.Region,
		AccountType:   profile.AccountType,
		AvatarMediaID: profile.AvatarMediaID,
		CreatedAt:     profile.CreatedAt,
		UpdatedAt:     profile.UpdatedAt,
	}
}
