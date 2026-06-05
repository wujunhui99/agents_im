package groups

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/types"
	groupspb "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"
)

// hydrateMember 用 user-rpc 补全单个成员的资料字段（identifier/display_name/name/avatar）。
func hydrateMember(ctx context.Context, svcCtx *svc.ServiceContext, member *groupspb.GroupMember) (types.GroupMember, error) {
	m := toGroupMember(member)
	if member == nil || member.GetUserId() == "" {
		return m, nil
	}
	resp, err := svcCtx.UserRPC.GetUserByID(ctx, &userpb.GetUserByIDRequest{UserId: member.GetUserId()})
	if err != nil {
		return types.GroupMember{}, apiError(err)
	}
	applyProfile(&m, resp.GetUser())
	return m, nil
}

// hydrateMembers 批量补全成员列表资料：一次 user-rpc.GetUsersByIDs 取代 N 次 GetUserByID。
// 账号已注销的成员（profile 缺失）按空资料降级返回，不阻断整列表。
func hydrateMembers(ctx context.Context, svcCtx *svc.ServiceContext, members []*groupspb.GroupMember) ([]types.GroupMember, error) {
	ids := make([]string, 0, len(members))
	for _, member := range members {
		if member != nil && member.GetUserId() != "" {
			ids = append(ids, member.GetUserId())
		}
	}

	profiles, err := fetchProfiles(ctx, svcCtx, ids)
	if err != nil {
		return nil, err
	}

	items := make([]types.GroupMember, len(members))
	for i, member := range members {
		m := toGroupMember(member)
		if member != nil {
			applyProfile(&m, profiles[member.GetUserId()])
		}
		items[i] = m
	}
	return items, nil
}

// fetchProfiles 批量取用户资料，按 user_id 索引；缺失的 id 不在返回 map 中。
func fetchProfiles(ctx context.Context, svcCtx *svc.ServiceContext, userIDs []string) (map[string]*userpb.UserEntity, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	resp, err := svcCtx.UserRPC.GetUsersByIDs(ctx, &userpb.GetUsersByIDsRequest{UserIds: userIDs})
	if err != nil {
		return nil, apiError(err)
	}
	profiles := make(map[string]*userpb.UserEntity, len(resp.GetUsers()))
	for _, u := range resp.GetUsers() {
		profiles[u.GetUserId()] = u
	}
	return profiles, nil
}

func applyProfile(m *types.GroupMember, u *userpb.UserEntity) {
	if u == nil {
		return
	}
	m.Identifier = u.GetIdentifier()
	m.DisplayName = humanReadableName(u)
	m.Name = u.GetName()
	m.AvatarMediaID = u.GetAvatarMediaId()
	m.AvatarURL = u.GetAvatarUrl()
}

func humanReadableName(u *userpb.UserEntity) string {
	for _, v := range []string{u.GetDisplayName(), u.GetName(), u.GetIdentifier()} {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return strings.TrimSpace(u.GetUserId())
}

// ensureUsersExist 建群/加成员前校验用户存在（user-rpc 批量）。任一不存在即返回 NotFound。
func ensureUsersExist(ctx context.Context, svcCtx *svc.ServiceContext, userIDs ...string) error {
	ids := make([]string, 0, len(userIDs))
	seen := make(map[string]struct{}, len(userIDs))
	for _, id := range userIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}

	profiles, err := fetchProfiles(ctx, svcCtx, ids)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, ok := profiles[id]; !ok {
			return apperror.NotFound("account not found")
		}
	}
	return nil
}
