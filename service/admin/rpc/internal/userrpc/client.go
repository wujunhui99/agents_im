// Package userrpc 用 user-rpc 实现 admin-rpc 的 repository.AdminAccountRepository：
// 管理后台跨域账号只读(用户详情/搜索/计数)经属主 user-rpc，不再走顶层 internal/repository
// accountRepo(执行 profiles.avatar_media_id 的 string scan)。是 gate #550(avatar_media_id
// text→bigint)「读路径迁出 internal/」的末步(继 #551 auth、#553 agent-api、#555 msg-rpc)。
//
// 错误经 rpcerror.FromStatus 还原 apperror，保码映射与旧 internal/repository 路径一致。
// 好友只读(friendships 表无 avatar)仍由 admin svc 的 internal postgres repo 承载，非本适配器职责。
package userrpc

import (
	"context"
	"time"

	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
)

// AdminAccountClient 把 user-rpc 适配成 admin 期望的 repository.AdminAccountRepository。
type AdminAccountClient struct {
	rpc userclient.User
}

var _ repository.AdminAccountRepository = (*AdminAccountClient)(nil)

// NewAdminAccountClient 包装一个 user-rpc 客户端。
func NewAdminAccountClient(rpc userclient.User) *AdminAccountClient {
	return &AdminAccountClient{rpc: rpc}
}

func (c *AdminAccountClient) GetByID(ctx context.Context, accountID string) (model.User, error) {
	resp, err := c.rpc.GetUserByID(ctx, &userclient.GetUserByIDRequest{UserId: accountID})
	if err != nil {
		return model.User{}, rpcerror.FromStatus(err)
	}
	return toUser(resp.GetUser()), nil
}

func (c *AdminAccountClient) SearchAccounts(ctx context.Context, filter repository.AccountSearchFilter) ([]model.User, error) {
	resp, err := c.rpc.SearchAccounts(ctx, &userclient.SearchAccountsRequest{
		Query: filter.Query,
		Limit: int32(filter.Limit),
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	entities := resp.GetUsers()
	users := make([]model.User, 0, len(entities))
	for _, entity := range entities {
		users = append(users, toUser(entity))
	}
	return users, nil
}

func (c *AdminAccountClient) CountAccounts(ctx context.Context) (int64, error) {
	resp, err := c.rpc.CountAccounts(ctx, &userclient.CountAccountsRequest{})
	if err != nil {
		return 0, rpcerror.FromStatus(err)
	}
	return resp.GetCount(), nil
}

// toUser 把 user-rpc UserEntity 映射成 common/share/model.User。avatar 维持十进制串(wire 不变)；
// 时间戳按 user-rpc convert.formatTime 的 RFC3339(UTC)还原。
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
			CreatedAt:   parseTime(u.GetCreatedAt()),
			UpdatedAt:   parseTime(u.GetUpdatedAt()),
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
			CreatedAt:     parseTime(u.GetCreatedAt()),
			UpdatedAt:     parseTime(u.GetUpdatedAt()),
		},
	)
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}
