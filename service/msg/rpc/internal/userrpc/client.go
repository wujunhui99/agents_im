// Package userrpc 用 user-rpc 实现 msg-rpc AI 托管运行时所需的 repository.Repository：
// agent-create 工具路径(AgentAssemblyLogic.CreateAgentFromTool)的账号读写经属主 user-rpc，
// 不再走顶层 internal/repository accountRepo(执行 profiles.avatar_media_id 的 string scan / 空串写)。
// 是 gate #550(avatar_media_id text→bigint)「读路径迁出 internal/」的第 3 步(继 #551 auth、#553 agent-api)。
//
// internal/servicecontext/message(internal，rule 8 禁改)把 Accounts 与 Friendships 都取自
// ctx.AccountRepo(类型 repository.Repository)。故本适配器是 Composite：
//   - 账号方法(Create/GetByID/ExistsByIdentifier/...)经 user-rpc；
//   - 好友方法(EnsureAcceptedFriendship/...)委托给内部 postgres 好友 repo(friendships 表无 avatar，
//     非 #550 blocker；friends 域尾巴随 message/agent 迁移再脱)。
//
// 账号写中 user-rpc 不暴露的(UpdateProfile/UpdateAvatar/RenameIdentifier/ListByAccountType)在
// agent-create 路径不可达 → fail-loud(rule 1/2 禁假实现)。错误经 rpcerror.FromStatus 还原 apperror。
package userrpc

import (
	"context"
	"time"

	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
)

// Composite 把 user-rpc(账号) + 内部 postgres 好友 repo(好友)组合成 repository.Repository。
// 好友方法经内嵌接口自动委托，账号方法在下方显式覆写。
type Composite struct {
	repository.FriendshipRepository
	rpc userclient.User
}

var _ repository.Repository = (*Composite)(nil)

// NewComposite 组合 user-rpc 客户端与好友委托 repo(通常是 internal/repository 的 postgres repo)。
func NewComposite(rpc userclient.User, friendships repository.FriendshipRepository) *Composite {
	return &Composite{FriendshipRepository: friendships, rpc: rpc}
}

// Create 经 user-rpc 建账号(agent-create 工具创建 agent 账号；user-rpc 负责 avatar 默认值，
// #550 后为 0)。account_type 透传(agent)。
func (c *Composite) Create(ctx context.Context, account model.User) (model.User, error) {
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

func (c *Composite) GetByID(ctx context.Context, accountID string) (model.User, error) {
	resp, err := c.rpc.GetUserByID(ctx, &userclient.GetUserByIDRequest{UserId: accountID})
	if err != nil {
		return model.User{}, rpcerror.FromStatus(err)
	}
	return toUser(resp.GetUser()), nil
}

func (c *Composite) GetByIdentifier(ctx context.Context, identifier string) (model.User, error) {
	resp, err := c.rpc.GetUserByIdentifier(ctx, &userclient.GetUserByIdentifierRequest{Identifier: identifier})
	if err != nil {
		return model.User{}, rpcerror.FromStatus(err)
	}
	return toUser(resp.GetUser()), nil
}

func (c *Composite) ExistsByIdentifier(ctx context.Context, identifier string) (bool, error) {
	resp, err := c.rpc.ExistsByIdentifier(ctx, &userclient.ExistsByIdentifierRequest{Identifier: identifier})
	if err != nil {
		return false, rpcerror.FromStatus(err)
	}
	return resp.GetExists(), nil
}

func (c *Composite) ListByIDs(ctx context.Context, accountIDs []string) ([]model.User, error) {
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

// 以下账号写在 agent-create 工具路径不可达且 user-rpc 未暴露：fail-loud，禁止静默假成功。
func (c *Composite) ListByAccountType(context.Context, model.AccountType) ([]model.User, error) {
	return nil, unsupported("ListByAccountType")
}

func (c *Composite) RenameIdentifier(context.Context, string, string) (model.User, error) {
	return model.User{}, unsupported("RenameIdentifier")
}

func (c *Composite) UpdateProfile(context.Context, string, repository.AccountProfilePatch) (model.User, error) {
	return model.User{}, unsupported("UpdateProfile")
}

func (c *Composite) UpdateAvatar(context.Context, string, string, string) (model.User, error) {
	return model.User{}, unsupported("UpdateAvatar")
}

func unsupported(method string) error {
	return apperror.Internal("msg-rpc user-rpc adapter does not support account write " + method)
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

func formatVerifiedAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
