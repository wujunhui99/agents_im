// Package userrpc 用 user-rpc 实现 agent-api 期望的 repository.AccountRepository：
// agent CRUD / 定义读所需的账号资料(account_type 校验、authorize)经属主 user-rpc，
// 不再走顶层 internal/repository accountRepo(执行 profiles.avatar_media_id 的 string scan)。
// 是 gate #550(avatar_media_id text→bigint)「读路径迁出 internal/」的第 2 步(继 #551 auth)。
//
// agent-api HTTP 面只做只读账号查询(GetByID/ExistsByIdentifier/...)；账号写与好友写
// (CreateAgentFromTool 路径)在 agent-api 进程内不可达，对应方法 fail-loud(rule 1/2 禁假实现)。
// 错误经 rpcerror.FromStatus 还原 apperror，保证码映射与旧 internal/repository 路径一致。
package userrpc

import (
	"context"
	"time"

	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
)

// AccountClient 把 user-rpc 适配成 agent 逻辑期望的 repository.AccountRepository。
type AccountClient struct {
	rpc userclient.User
}

var _ repository.AccountRepository = (*AccountClient)(nil)

// NewAccountClient 包装一个 user-rpc 客户端。
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

func (c *AccountClient) GetByIdentifier(ctx context.Context, identifier string) (model.User, error) {
	resp, err := c.rpc.GetUserByIdentifier(ctx, &userclient.GetUserByIdentifierRequest{Identifier: identifier})
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

func (c *AccountClient) ListByIDs(ctx context.Context, accountIDs []string) ([]model.User, error) {
	resp, err := c.rpc.GetUsersByIDs(ctx, &userclient.GetUsersByIDsRequest{UserIds: accountIDs})
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

// 以下为 agent-api HTTP 面不可达的账号写/枚举/改名方法：fail-loud，禁止静默假成功。
// 账号写(建 agent 账号/改资料/改头像)的属主是 user-rpc，跨域写应由对应 BFF 编排，
// 不经此适配器；agent-api 当前无此调用路径。

func (c *AccountClient) Create(context.Context, model.User) (model.User, error) {
	return model.User{}, unsupported("Create")
}

func (c *AccountClient) ListByAccountType(context.Context, model.AccountType) ([]model.User, error) {
	return nil, unsupported("ListByAccountType")
}

func (c *AccountClient) RenameIdentifier(context.Context, string, string) (model.User, error) {
	return model.User{}, unsupported("RenameIdentifier")
}

func (c *AccountClient) UpdateProfile(context.Context, string, repository.AccountProfilePatch) (model.User, error) {
	return model.User{}, unsupported("UpdateProfile")
}

func (c *AccountClient) UpdateAvatar(context.Context, string, string, string) (model.User, error) {
	return model.User{}, unsupported("UpdateAvatar")
}

func unsupported(method string) error {
	return apperror.Internal("agent-api user-rpc adapter does not support account write " + method)
}

// toUser 把 user-rpc UserEntity 映射成 common/share/model.User。avatar 维持十进制串(wire 不变)；
// 时间戳按 user-rpc convert.unixMilli 的 UnixMilli(UTC)还原。
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

// fromUnixMilli 把 UnixMilli(UTC) 还原成 time.Time；0 → 零值。int64 解码不会失败，
// 不再有旧 parseTime(RFC3339 string) 的静默吞错路径。
func fromUnixMilli(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}
