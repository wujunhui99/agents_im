package groups

import (
	"context"
	"strings"
	"sync"

	"github.com/wujunhui99/agents_im/service/groups/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/types"
	groupspb "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"
)

// hydrateConcurrency 限制并发补全成员资料的协程数。
const hydrateConcurrency = 16

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

// hydrateMembers 并发补全成员列表资料。同群成员 account_id 唯一，按下标并发写，无需去重。
func hydrateMembers(ctx context.Context, svcCtx *svc.ServiceContext, members []*groupspb.GroupMember) ([]types.GroupMember, error) {
	items := make([]types.GroupMember, len(members))
	sem := make(chan struct{}, hydrateConcurrency)
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	for i, member := range members {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, member *groupspb.GroupMember) {
			defer wg.Done()
			defer func() { <-sem }()
			hm, err := hydrateMember(ctx, svcCtx, member)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			items[i] = hm
		}(i, member)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return items, nil
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

// ensureUsersExist 建群/加成员前校验用户存在（user-rpc）。任一不存在即返回对应错误。
func ensureUsersExist(ctx context.Context, svcCtx *svc.ServiceContext, userIDs ...string) error {
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
		if _, err := svcCtx.UserRPC.GetUserByID(ctx, &userpb.GetUserByIDRequest{UserId: id}); err != nil {
			return apiError(err)
		}
	}
	return nil
}
