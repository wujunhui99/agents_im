// Package userrpc 用 user-rpc 实现 auth 的 useradapter.UserClient：auth 注册/登录读用户资料
// 经属主 user-rpc，不再走 internal/logic.UserLogic（EPIC #527 / #551，为 #550 头像列改型解阻）。
// rpc 之间确需互调时（auth 注册必须建 user），错误经 rpcerror.FromStatus 还原成 apperror，
// 让 AuthLogic/上层 ToStatus 的码映射与旧 internal/logic 路径一致。
package userrpc

import (
	"context"
	"time"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
)

// Client 把 user-rpc 适配成 auth 期望的 useradapter.UserClient。
type Client struct {
	rpc userclient.User
}

var _ useradapter.UserClient = (*Client)(nil)

// NewClient 包装一个 user-rpc 客户端。
func NewClient(rpc userclient.User) *Client {
	return &Client{rpc: rpc}
}

func (c *Client) ExistsByIdentifier(ctx context.Context, identifier string) (useradapter.ExistsResult, error) {
	resp, err := c.rpc.ExistsByIdentifier(ctx, &userclient.ExistsByIdentifierRequest{Identifier: identifier})
	if err != nil {
		return useradapter.ExistsResult{}, rpcerror.FromStatus(err)
	}
	return useradapter.ExistsResult{Identifier: resp.GetIdentifier(), Exists: resp.GetExists()}, nil
}

func (c *Client) CreateUser(ctx context.Context, req useradapter.CreateUserRequest) (useradapter.UserProfile, error) {
	resp, err := c.rpc.CreateUser(ctx, &userclient.CreateUserRequest{
		Identifier:      req.Identifier,
		Email:           req.Email,
		EmailVerifiedAt: formatVerifiedAt(req.EmailVerifiedAt),
		DisplayName:     req.DisplayName,
		Name:            req.Name,
		Gender:          req.Gender,
		BirthDate:       req.BirthDate,
		Region:          req.Region,
		// account_type 留空 = user（user-rpc NormalizeAccountType 默认）；auth 注册只建普通用户。
	})
	if err != nil {
		return useradapter.UserProfile{}, rpcerror.FromStatus(err)
	}
	return toProfile(resp.GetUser()), nil
}

func (c *Client) GetUserByID(ctx context.Context, userID string) (useradapter.UserProfile, error) {
	resp, err := c.rpc.GetUserByID(ctx, &userclient.GetUserByIDRequest{UserId: userID})
	if err != nil {
		return useradapter.UserProfile{}, rpcerror.FromStatus(err)
	}
	return toProfile(resp.GetUser()), nil
}

// toProfile 把 user-rpc UserEntity 映射成 auth 的 UserProfile。EmailVerifiedAt 不回读
// （UserEntity 不带该字段；auth 只写不读），留零值。
func toProfile(u *userclient.UserEntity) useradapter.UserProfile {
	if u == nil {
		return useradapter.UserProfile{}
	}
	return useradapter.UserProfile{
		UserID:        u.GetUserId(),
		Identifier:    u.GetIdentifier(),
		Email:         u.GetEmail(),
		DisplayName:   u.GetDisplayName(),
		Name:          u.GetName(),
		Gender:        u.GetGender(),
		BirthDate:     u.GetBirthDate(),
		Region:        u.GetRegion(),
		AccountType:   u.GetAccountType(),
		AvatarMediaID: u.GetAvatarMediaId(),
		AvatarURL:     u.GetAvatarUrl(),
		CreatedAt:     rfc3339FromUnixMilli(u.GetCreatedAt()),
		UpdatedAt:     rfc3339FromUnixMilli(u.GetUpdatedAt()),
	}
}

// formatVerifiedAt 把 EmailVerifiedAt 转成 wire 上的 RFC3339（零值=未验证→空串）。
func formatVerifiedAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// rfc3339FromUnixMilli 把 user-rpc 的 UnixMilli 时间戳还原成 RFC3339(UTC) 串；0 → 空串
// （与旧 user-rpc formatTime 的零值行为一致，UserProfile.CreatedAt/UpdatedAt 仍是 string）。
func rfc3339FromUnixMilli(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}
