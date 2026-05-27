package logic

import (
	business "github.com/wujunhui99/agents_im/internal/logic"
	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
)

func toGroup(info business.GroupInfo) *groups.Group {
	return &groups.Group{
		GroupId:         info.GroupID,
		Name:            info.Name,
		Description:     info.Description,
		Announcement:    info.Announcement,
		AvatarMediaId:   info.AvatarMediaID,
		AvatarUrl:       info.AvatarURL,
		CreatorUserId:   info.CreatorUserID,
		CurrentUserRole: info.CurrentUserRole,
		CreatedAt:       info.CreatedAt,
		UpdatedAt:       info.UpdatedAt,
	}
}

func toGroupMember(info business.GroupMemberInfo) *groups.GroupMember {
	return &groups.GroupMember{
		GroupId:       info.GroupID,
		UserId:        info.UserID,
		Role:          info.Role,
		State:         info.State,
		JoinedAt:      info.JoinedAt,
		LeftAt:        info.LeftAt,
		Identifier:    info.Identifier,
		DisplayName:   info.DisplayName,
		Name:          info.Name,
		AvatarMediaId: info.AvatarMediaID,
		AvatarUrl:     info.AvatarURL,
	}
}

func toMemberResponse(info business.MemberResponse) *groups.MemberResponse {
	return &groups.MemberResponse{Member: toGroupMember(info.Member), AlreadyMember: info.AlreadyMember}
}
