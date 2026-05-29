package groups

import (
	"github.com/wujunhui99/agents_im/pkg/apperror"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/types"
)

func groupResp(group business.GroupInfo) *types.GroupResp {
	return &types.GroupResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    toGroup(group),
	}
}

func toGroup(group business.GroupInfo) types.Group {
	return types.Group{
		GroupID:         group.GroupID,
		Name:            group.Name,
		Description:     group.Description,
		Announcement:    group.Announcement,
		AvatarMediaID:   group.AvatarMediaID,
		AvatarURL:       group.AvatarURL,
		CreatorUserID:   group.CreatorUserID,
		CurrentUserRole: group.CurrentUserRole,
		CreatedAt:       group.CreatedAt,
		UpdatedAt:       group.UpdatedAt,
	}
}

func memberResp(member business.MemberResponse) *types.MemberResp {
	return &types.MemberResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.MemberData{
			Member:        toGroupMember(member.Member),
			AlreadyMember: member.AlreadyMember,
		},
	}
}

func toGroupMember(member business.GroupMemberInfo) types.GroupMember {
	return types.GroupMember{
		GroupID:       member.GroupID,
		UserID:        member.UserID,
		Role:          member.Role,
		State:         member.State,
		JoinedAt:      member.JoinedAt,
		LeftAt:        member.LeftAt,
		Identifier:    member.Identifier,
		DisplayName:   member.DisplayName,
		Name:          member.Name,
		AvatarMediaID: member.AvatarMediaID,
		AvatarURL:     member.AvatarURL,
	}
}
