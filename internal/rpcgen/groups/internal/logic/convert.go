package logic

import (
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/proto/groupspb"
)

func toGroup(info business.GroupInfo) *groupspb.Group {
	return &groupspb.Group{
		GroupId:       info.GroupID,
		Name:          info.Name,
		Description:   info.Description,
		CreatorUserId: info.CreatorUserID,
		CreatedAt:     info.CreatedAt,
		UpdatedAt:     info.UpdatedAt,
	}
}

func toGroupMember(info business.GroupMemberInfo) *groupspb.GroupMember {
	return &groupspb.GroupMember{
		GroupId:  info.GroupID,
		UserId:   info.UserID,
		State:    info.State,
		JoinedAt: info.JoinedAt,
		LeftAt:   info.LeftAt,
	}
}
