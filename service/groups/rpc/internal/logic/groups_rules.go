package logic

import (
	"context"
	"errors"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/model"
)

const maxGroupMembersV1 = 200

// --- role / status 整型 <-> 字符串映射（整型取值见 model/vars.go）---

func memberRoleToString(role int64) string {
	switch role {
	case model.MemberRoleOwner:
		return "owner"
	case model.MemberRoleAdmin:
		return "admin"
	default:
		return "member"
	}
}

func memberStateToString(status int64) string {
	if status == model.MemberStatusLeft {
		return "left"
	}
	return "active"
}

// --- 输入校验（不做规范化：入参的清洗由客户端负责，后端只守完整性/防滥用底线）---

func validateRequiredID(value, field string) (string, error) {
	if value == "" {
		return "", apperror.InvalidArgument(field + " is required")
	}
	if len([]rune(value)) > 64 {
		return "", apperror.InvalidArgument(field + " must be 64 characters or fewer")
	}
	return value, nil
}

func validateOptionalID(value, field string) (string, error) {
	if value == "" {
		return "", nil
	}
	return validateRequiredID(value, field)
}

func validateGroupName(value string) (string, error) {
	if value == "" {
		return "", apperror.InvalidArgument("name is required")
	}
	if len([]rune(value)) > 64 {
		return "", apperror.InvalidArgument("name must be 64 characters or fewer")
	}
	return value, nil
}

func validateGroupDescription(value string) (string, error) {
	if len([]rune(value)) > 256 {
		return "", apperror.InvalidArgument("description must be 256 characters or fewer")
	}
	return value, nil
}

// validateGroupMemberIDs 去重后返回成员列表（首位为 creator），并强制成员数上限（防 DoS）。
func validateGroupMemberIDs(creatorUserID string, rawMemberUserIDs []string) ([]string, error) {
	seen := map[string]struct{}{creatorUserID: {}}
	memberUserIDs := []string{creatorUserID}
	for _, userID := range rawMemberUserIDs {
		if _, err := validateRequiredID(userID, "member_user_ids"); err != nil {
			return nil, err
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		memberUserIDs = append(memberUserIDs, userID)
	}
	if len(memberUserIDs) > maxGroupMembersV1 {
		return nil, apperror.InvalidArgument("group member limit is 200")
	}
	return memberUserIDs, nil
}

// --- 成员/权限辅助（读 group_members）---

// activeMember 返回某用户在群内的 active 成员行；非成员返回 Forbidden。
func activeMember(ctx context.Context, m model.GroupMembersModel, groupID, userID string) (*model.GroupMembers, error) {
	row, err := m.FindOneByGroupIdAccountId(ctx, groupID, userID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, apperror.Forbidden("requester is not a group member")
		}
		return nil, err
	}
	if row.Status != model.MemberStatusActive {
		return nil, apperror.Forbidden("requester is not a group member")
	}
	return row, nil
}

// ensureCanManageGroup 校验操作者是 owner/admin。
func ensureCanManageGroup(ctx context.Context, m model.GroupMembersModel, groupID, userID string) (*model.GroupMembers, error) {
	member, err := activeMember(ctx, m, groupID, userID)
	if err != nil {
		return nil, err
	}
	if member.Role != model.MemberRoleOwner && member.Role != model.MemberRoleAdmin {
		return nil, apperror.Forbidden("only group owner or admin can manage group")
	}
	return member, nil
}

func containsActiveMember(members []*model.GroupMembers, userID string) bool {
	for _, member := range members {
		if member.AccountId == userID && member.Status == model.MemberStatusActive {
			return true
		}
	}
	return false
}

// notFoundAs 把 model.ErrNotFound 翻译为带语义信息的 NotFound，其它错误原样返回。
func notFoundAs(err error, msg string) error {
	if errors.Is(err, model.ErrNotFound) {
		return apperror.NotFound(msg)
	}
	return err
}

// addActiveMember 把 userID 加入群（AddMember/JoinGroup 共用）：已是 active 成员则原样返回
// 并标记 AlreadyMember；否则校验成员上限后 upsert 为 active（creator 取 owner 角色，其余 member）。
func addActiveMember(ctx context.Context, m model.GroupMembersModel, group *model.Groups, userID string) (*groups.MemberResponse, error) {
	existing, err := m.FindOneByGroupIdAccountId(ctx, group.GroupId, userID)
	if err == nil && existing.Status == model.MemberStatusActive {
		return &groups.MemberResponse{Member: toGroupMember(existing), AlreadyMember: true}, nil
	}
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}

	members, err := m.FindActiveByGroup(ctx, group.GroupId)
	if err != nil {
		return nil, err
	}
	if len(members) >= maxGroupMembersV1 {
		return nil, apperror.InvalidArgument("group member limit is 200")
	}

	role := model.MemberRoleMember
	if group.CreatorAccountId == userID {
		role = model.MemberRoleOwner
	}
	row, err := m.UpsertActiveMember(ctx, group.GroupId, userID, role)
	if err != nil {
		return nil, err
	}
	return &groups.MemberResponse{Member: toGroupMember(row), AlreadyMember: false}, nil
}
