package logic

import (
	"time"

	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/model"
)

// toGroup 把 groups 行映射为 proto。group 表无 announcement/avatar 列：
// announcement 沿用 description，avatar 字段恒空（与历史行为一致）。
func toGroup(g *model.Groups, currentUserRole string) *groups.Group {
	return &groups.Group{
		GroupId:         g.GroupId,
		Name:            g.Name,
		Description:     g.Description,
		Announcement:    g.Description,
		CreatorUserId:   g.CreatorAccountId,
		CurrentUserRole: currentUserRole,
		CreatedAt:       formatTime(g.CreatedAt),
		UpdatedAt:       formatTime(g.UpdatedAt),
	}
}

// toGroupMember 把 group_members 行映射为 proto。
// profile 字段（identifier/display_name/name/avatar_*）属 user 域，由 BFF 聚合 user-rpc 补全，rpc 留空。
func toGroupMember(m *model.GroupMembers) *groups.GroupMember {
	member := &groups.GroupMember{
		GroupId:  m.GroupId,
		UserId:   m.AccountId,
		Role:     memberRoleToString(m.Role),
		State:    memberStateToString(m.Status),
		JoinedAt: formatTime(m.JoinTime),
	}
	if m.LeftAt.Valid {
		member.LeftAt = formatTime(m.LeftAt.Time)
	}
	return member
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
