package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/svc"
	friendspb "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	msgpb "github.com/wujunhui99/agents_im/service/msg/rpc/msg"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

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
	if l.svcCtx.UserRPC == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("admin account repository is not configured"))
	}
	resp, err := l.svcCtx.UserRPC.SearchAccounts(l.ctx, &userpb.SearchAccountsRequest{
		Query: strings.TrimSpace(in.GetQuery()),
		Limit: int32(normalizeAdminLimit(int(in.GetLimit()), 20, 100)),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	users := resp.GetUsers()
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
	if l.svcCtx.UserRPC == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("admin account repository is not configured"))
	}
	accountID, err := validateRequiredAdminID(in.GetAccountId(), "account_id", adminAccountIDMaxLen)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	user, err := l.svcCtx.UserRPC.GetUserByID(l.ctx, &userpb.GetUserByIDRequest{UserId: accountID})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	return &admin.UserDetailResponse{User: adminUserPB(user.GetUser())}, nil
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
	accountID, err := validateRequiredAdminID(in.GetAccountId(), "account_id", adminAccountIDMaxLen)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if _, err := l.svcCtx.UserRPC.GetUserByID(l.ctx, &userpb.GetUserByIDRequest{UserId: accountID}); err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	// 好友关系只读经属主 friends-rpc（#618，脱 internal/repository FriendshipRepository）；
	// 好友资料由 admin 再聚合属主 user-rpc 补全（friends-rpc 是叶子，不跨域取资料）。
	friendsResp, err := l.svcCtx.FriendsRPC.ListFriends(l.ctx, &friendspb.ListFriendsRequest{UserId: accountID})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	friendships := friendsResp.GetFriends()
	out := make([]*admin.AdminFriend, 0, len(friendships))
	for _, friendship := range friendships {
		view := &admin.AdminFriend{
			UserId:    friendship.GetUserId(),
			FriendId:  friendship.GetFriendId(),
			Status:    friendship.GetStatus(),
			IsFriend:  friendship.GetIsFriend(),
			CreatedAt: friendship.GetCreatedAt(),
			UpdatedAt: friendship.GetUpdatedAt(),
		}
		friend, err := l.svcCtx.UserRPC.GetUserByID(l.ctx, &userpb.GetUserByIDRequest{UserId: friendship.GetFriendId()})
		if err != nil {
			return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
		}
		view.Friend = adminUserPB(friend.GetUser())
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
	accountID, err := validateRequiredAdminID(in.GetAccountId(), "account_id", adminAccountIDMaxLen)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if _, err := l.svcCtx.UserRPC.GetUserByID(l.ctx, &userpb.GetUserByIDRequest{UserId: accountID}); err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	// 用户会话 seq 视图经属主 msg-rpc（#618，脱 internal/repository）：空 conversation_ids 取该用户全部会话。
	statesResp, err := l.svcCtx.MsgRPC.GetConversationsSeqState(l.ctx, &msgpb.GetConversationsSeqStateRequest{UserId: accountID})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	return &admin.UserConversationsResponse{Conversations: adminConversationsPB(statesResp.GetStates())}, nil
}
