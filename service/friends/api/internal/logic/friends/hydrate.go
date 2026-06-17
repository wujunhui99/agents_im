package friends

import (
	"context"
	"time"

	"github.com/wujunhui99/agents_im/service/friends/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/friends/api/internal/types"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"
)

// rfc3339FromUnixMilli 把 user-rpc 的 UnixMilli 时间戳渲染成对外 RFC3339(UTC) 串；0 → 空串
// （与旧 user-rpc formatTime 的零值行为一致，FriendProfile.CreatedAt/UpdatedAt 仍是 string）。
func rfc3339FromUnixMilli(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

// peerOf 返回某条 Friendship 在当前视角下需要展示资料的对端账号 id。
type peerOf func(types.Friendship) string

// peerIsFriend：列表好友 / 接受拒绝结果，对端是 friend_id。
func peerIsFriend(f types.Friendship) string { return f.FriendID }

// peerIsRequester：收到的好友请求（requester -> 我），对端是发起方 user_id。
func peerIsRequester(f types.Friendship) string { return f.UserID }

// hydrateFriendship 给单条 friendship 用 user-rpc 补全对端资料（peer 决定看谁）。
func hydrateFriendship(ctx context.Context, svcCtx *svc.ServiceContext, f *types.Friendship, peer peerOf) error {
	id := peer(*f)
	if id == "" {
		return nil
	}
	profiles, err := fetchProfiles(ctx, svcCtx, []string{id})
	if err != nil {
		return err
	}
	if u, ok := profiles[id]; ok {
		f.Friend = friendProfile(u)
	}
	return nil
}

// hydrateFriendships 批量补全一批 friendship 的对端资料：一次 GetUsersByIDs 取代 N 次单查（无 N+1）。
// 账号已注销（profile 缺失）按空资料降级，不阻断整列表。
func hydrateFriendships(ctx context.Context, svcCtx *svc.ServiceContext, items []types.Friendship, peer peerOf) error {
	profiles, err := fetchProfiles(ctx, svcCtx, peerIDs(items, peer))
	if err != nil {
		return err
	}
	applyProfiles(items, peer, profiles)
	return nil
}

// peerIDs 收集一批 friendship 的对端 id（去空）。
func peerIDs(items []types.Friendship, peer peerOf) []string {
	ids := make([]string, 0, len(items))
	for _, f := range items {
		if id := peer(f); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// applyProfiles 按对端 id 把资料写回每条 friendship。
func applyProfiles(items []types.Friendship, peer peerOf, profiles map[string]*userpb.UserEntity) {
	for i := range items {
		if u, ok := profiles[peer(items[i])]; ok {
			items[i].Friend = friendProfile(u)
		}
	}
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

func friendProfile(u *userpb.UserEntity) types.FriendProfile {
	if u == nil {
		return types.FriendProfile{}
	}
	return types.FriendProfile{
		UserID:        u.GetUserId(),
		Identifier:    u.GetIdentifier(),
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
