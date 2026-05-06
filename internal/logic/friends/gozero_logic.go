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
	friendship, err := toFriendship(l.ctx, l.svcCtx, result.Friendship)
	if err != nil {
		return nil, err
	}
	return &types.AddFriendResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.AddFriendData{
			Friendship: friendship,
			Created:    result.Created,
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
	friendship, err := toFriendship(l.ctx, l.svcCtx, result.Friendship)
	if err != nil {
		return nil, err
	}
	return &types.DeleteFriendResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.DeleteFriendData{
			Friendship: friendship,
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
	friendship, err := toFriendship(l.ctx, l.svcCtx, result.Friendship)
	if err != nil {
		return nil, err
	}
	return &types.FriendshipResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.FriendshipData{
			Friendship: friendship,
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
		view, err := toFriendship(l.ctx, l.svcCtx, friendship)
		if err != nil {
			return nil, err
		}
		friends = append(friends, view)
	}
	return &types.ListFriendsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    types.ListFriendsData{Friends: friends},
	}, nil
}

func toFriendship(ctx context.Context, svcCtx *svc.ServiceContext, friendship business.FriendshipView) (types.Friendship, error) {
	view := types.Friendship{
		UserID:    friendship.UserID,
		FriendID:  friendship.FriendID,
		Status:    friendship.Status,
		IsFriend:  friendship.IsFriend,
		CreatedAt: friendship.CreatedAt,
		UpdatedAt: friendship.UpdatedAt,
	}
	if friendship.Friend != nil {
		profile, err := toFriendProfile(ctx, svcCtx, *friendship.Friend)
		if err != nil {
			return types.Friendship{}, err
		}
		view.Friend = profile
	}
	return view, nil
}

func toFriendProfile(ctx context.Context, svcCtx *svc.ServiceContext, profile business.UserProfile) (types.FriendProfile, error) {
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
		AvatarURL:     profile.AvatarURL,
		CreatedAt:     profile.CreatedAt,
		UpdatedAt:     profile.UpdatedAt,
	}, nil
}
