// Package userrpc 实现 agent 域跨域端口（#606，脱顶层 internal/repository）：
//   - AccountClient → agentlogic.AccountPort：agent.create 工具路径的账号读/建经属主 user-rpc；
//   - FriendClient  → agentlogic.FriendPort：agent.create 建好友经属主 friends-rpc（EnsureFriendship）。
//
// 二者均为单向叶子调用（agent-rpc → user-rpc / friends-rpc），不在 rpc 间成环；账号写经 user-rpc
// 负责 avatar 默认值与校验（gate #550），好友写经 friends-rpc 幂等 accepted。错误经 rpcerror.FromStatus
// 还原 apperror。
package userrpc

import (
	"context"
	"time"

	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	friendsclient "github.com/wujunhui99/agents_im/service/friends/rpc/friendsclient"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
)

// AccountClient 把 user-rpc 暴露为 agentlogic.AccountPort（GetByID/ExistsByIdentifier/Create）。
type AccountClient struct {
	rpc userclient.User
}

func NewAccountClient(rpc userclient.User) *AccountClient {
	return &AccountClient{rpc: rpc}
}

func (c *AccountClient) GetByID(ctx context.Context, accountID string) (model.User, error) {
	resp, err := c.rpc.GetUserByID(ctx, &userclient.GetUserByIDRequest{UserId: accountID})
	if err != nil {
		return model.User{}, rpcerror.FromStatus(err)
	}
	return toUser(resp.GetUser()), nil
}

func (c *AccountClient) ExistsByIdentifier(ctx context.Context, identifier string) (bool, error) {
	resp, err := c.rpc.ExistsByIdentifier(ctx, &userclient.ExistsByIdentifierRequest{Identifier: identifier})
	if err != nil {
		return false, rpcerror.FromStatus(err)
	}
	return resp.GetExists(), nil
}

// Create 经 user-rpc 建账号（agent.create 工具建 agent 账号；user-rpc 负责 avatar 默认值与校验）。
func (c *AccountClient) Create(ctx context.Context, account model.User) (model.User, error) {
	resp, err := c.rpc.CreateUser(ctx, &userclient.CreateUserRequest{
		Identifier:      account.Identifier,
		DisplayName:     account.DisplayName,
		Name:            account.Name,
		Gender:          account.Gender,
		BirthDate:       account.BirthDate,
		Region:          account.Region,
		AccountType:     string(account.AccountType),
		Email:           account.Email,
		EmailVerifiedAt: formatVerifiedAt(account.EmailVerifiedAt),
	})
	if err != nil {
		return model.User{}, rpcerror.FromStatus(err)
	}
	return toUser(resp.GetUser()), nil
}

// FriendClient 把 friends-rpc 暴露为 agentlogic.FriendPort（幂等 accepted 好友）。
type FriendClient struct {
	rpc friendsclient.Friends
}

func NewFriendClient(rpc friendsclient.Friends) *FriendClient {
	return &FriendClient{rpc: rpc}
}

func (c *FriendClient) EnsureFriendship(ctx context.Context, userID string, friendID string) error {
	if _, err := c.rpc.EnsureFriendship(ctx, &friendsclient.EnsureFriendshipRequest{UserId: userID, FriendId: friendID}); err != nil {
		return rpcerror.FromStatus(err)
	}
	return nil
}

// toUser 把 user-rpc UserEntity 映射成 pkg/model.User。
func toUser(u *userclient.UserEntity) model.User {
	if u == nil {
		return model.User{}
	}
	return model.NewAccountProfile(
		model.Account{
			AccountID:   u.GetUserId(),
			Identifier:  u.GetIdentifier(),
			Email:       u.GetEmail(),
			AccountType: model.AccountType(u.GetAccountType()),
			CreatedAt:   fromUnixMilli(u.GetCreatedAt()),
			UpdatedAt:   fromUnixMilli(u.GetUpdatedAt()),
		},
		model.Profile{
			AccountID:     u.GetUserId(),
			DisplayName:   u.GetDisplayName(),
			Name:          u.GetName(),
			Gender:        u.GetGender(),
			BirthDate:     u.GetBirthDate(),
			Region:        u.GetRegion(),
			AvatarMediaID: u.GetAvatarMediaId(),
			AvatarURL:     u.GetAvatarUrl(),
			CreatedAt:     fromUnixMilli(u.GetCreatedAt()),
			UpdatedAt:     fromUnixMilli(u.GetUpdatedAt()),
		},
	)
}

func fromUnixMilli(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

func formatVerifiedAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
