package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

const adminAccountIDMaxLen = 128

// ---- SearchUsers ----

type SearchUsersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchUsersLogic {
	return &SearchUsersLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *SearchUsersLogic) SearchUsers(in *admin.UserSearchRequest) (*admin.UserSearchResponse, error) {
	if l.svcCtx.Accounts == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("admin account repository is not configured"))
	}
	users, err := l.svcCtx.Accounts.SearchAccounts(l.ctx, repository.AccountSearchFilter{
		Query: strings.TrimSpace(in.GetQuery()),
		Limit: normalizeAdminLimit(int(in.GetLimit()), 20, 100),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	out := make([]*admin.AdminUser, 0, len(users))
	for _, user := range users {
		out = append(out, adminUserPB(user))
	}
	return &admin.UserSearchResponse{Users: out}, nil
}

// ---- GetUserDetail ----

type GetUserDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserDetailLogic {
	return &GetUserDetailLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetUserDetailLogic) GetUserDetail(in *admin.UserDetailRequest) (*admin.UserDetailResponse, error) {
	if l.svcCtx.Accounts == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("admin account repository is not configured"))
	}
	accountID, err := validateRequiredAdminID(in.GetAccountId(), "account_id", adminAccountIDMaxLen)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	user, err := l.svcCtx.Accounts.GetByID(l.ctx, accountID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &admin.UserDetailResponse{User: adminUserPB(user)}, nil
}

// ---- GetUserFriends ----

type GetUserFriendsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserFriendsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserFriendsLogic {
	return &GetUserFriendsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetUserFriendsLogic) GetUserFriends(in *admin.UserFriendsRequest) (*admin.UserFriendsResponse, error) {
	if l.svcCtx.Accounts == nil || l.svcCtx.Friends == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("admin friend repositories are not configured"))
	}
	accountID, err := validateRequiredAdminID(in.GetAccountId(), "account_id", adminAccountIDMaxLen)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if _, err := l.svcCtx.Accounts.GetByID(l.ctx, accountID); err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	friendships, err := l.svcCtx.Friends.ListFriends(l.ctx, accountID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	out := make([]*admin.AdminFriend, 0, len(friendships))
	for _, friendship := range friendships {
		view := &admin.AdminFriend{
			UserId:    friendship.UserID,
			FriendId:  friendship.FriendID,
			Status:    friendship.Status,
			IsFriend:  friendship.Status == model.FriendshipStatusAccepted,
			CreatedAt: formatAdminTime(friendship.CreatedAt),
			UpdatedAt: formatAdminTime(friendship.UpdatedAt),
		}
		friend, err := l.svcCtx.Accounts.GetByID(l.ctx, friendship.FriendID)
		if err != nil {
			return nil, rpcerror.ToStatus(err)
		}
		view.Friend = adminUserPB(friend)
		out = append(out, view)
	}
	return &admin.UserFriendsResponse{Friends: out}, nil
}

// ---- GetUserConversations ----

type GetUserConversationsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserConversationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserConversationsLogic {
	return &GetUserConversationsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetUserConversationsLogic) GetUserConversations(in *admin.UserConversationsRequest) (*admin.UserConversationsResponse, error) {
	if l.svcCtx.Accounts == nil || l.svcCtx.Messages == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("admin conversation repositories are not configured"))
	}
	accountID, err := validateRequiredAdminID(in.GetAccountId(), "account_id", adminAccountIDMaxLen)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if _, err := l.svcCtx.Accounts.GetByID(l.ctx, accountID); err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	states, err := l.svcCtx.Messages.GetConversationSeqStates(l.ctx, accountID, nil)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &admin.UserConversationsResponse{Conversations: adminConversationsPB(states)}, nil
}
