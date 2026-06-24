package logic

import (
	"context"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
)

type UserExistenceChecker interface {
	EnsureUserExists(ctx context.Context, userID string) error
}

type UserProfileLookup interface {
	LookupUserProfile(ctx context.Context, userID string) (UserProfile, error)
}

type UserLogicExistenceChecker struct {
	userLogic *UserLogic
}

type GroupsLogic struct {
	repo       repository.GroupsRepository
	userExists UserExistenceChecker
}

const maxGroupMembersV1 = 200

func NewGroupsLogic(repo repository.GroupsRepository, userExists UserExistenceChecker) *GroupsLogic {
	return &GroupsLogic{repo: repo, userExists: userExists}
}

type GroupInfo struct {
	GroupID         string `json:"group_id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Announcement    string `json:"announcement"`
	AvatarMediaID   string `json:"avatar_media_id,omitempty"`
	AvatarURL       string `json:"avatar_url,omitempty"`
	CreatorUserID   string `json:"creator_user_id"`
	CurrentUserRole string `json:"current_user_role,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type GroupMemberInfo struct {
	GroupID       string `json:"group_id"`
	UserID        string `json:"user_id"`
	Role          string `json:"role"`
	State         string `json:"state"`
	JoinedAt      string `json:"joined_at"`
	LeftAt        string `json:"left_at"`
	Identifier    string `json:"identifier,omitempty"`
	DisplayName   string `json:"display_name,omitempty"`
	Name          string `json:"name,omitempty"`
	AvatarMediaID string `json:"avatar_media_id,omitempty"`
	AvatarURL     string `json:"avatar_url,omitempty"`
}

type CreateGroupRequest struct {
	CreatorUserID string   `json:"creator_user_id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	MemberUserIDs []string `json:"member_user_ids"`
}

type GetGroupRequest struct {
	GroupID         string `json:"group_id"`
	RequesterUserID string `json:"requester_user_id,omitempty"`
}

type AddMemberRequest struct {
	GroupID        string `json:"group_id"`
	OperatorUserID string `json:"operator_user_id"`
	UserID         string `json:"user_id"`
}

type UpdateGroupRequest struct {
	GroupID        string `json:"group_id"`
	OperatorUserID string `json:"operator_user_id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Announcement   string `json:"announcement"`
}

type KickMemberRequest struct {
	GroupID        string `json:"group_id"`
	OperatorUserID string `json:"operator_user_id"`
	UserID         string `json:"user_id"`
}

type JoinGroupRequest struct {
	GroupID string `json:"group_id"`
	UserID  string `json:"user_id"`
}

type LeaveGroupRequest struct {
	GroupID string `json:"group_id"`
	UserID  string `json:"user_id"`
}

type ListMembersRequest struct {
	GroupID         string `json:"group_id"`
	RequesterUserID string `json:"requester_user_id,omitempty"`
}

type ListGroupsRequest struct {
	UserID string `json:"user_id"`
}

type MemberResponse struct {
	Member        GroupMemberInfo `json:"member"`
	AlreadyMember bool            `json:"already_member"`
}

type ListMembersResponse struct {
	GroupID string            `json:"group_id"`
	Members []GroupMemberInfo `json:"members"`
}

type ListGroupsResponse struct {
	Groups []GroupInfo `json:"groups"`
}

func (l *GroupsLogic) CreateGroup(ctx context.Context, req CreateGroupRequest) (GroupInfo, error) {
	creatorUserID, err := normalizeRequiredID(req.CreatorUserID, "creator_user_id")
	if err != nil {
		return GroupInfo{}, err
	}

	name, err := normalizeGroupName(req.Name)
	if err != nil {
		return GroupInfo{}, err
	}
	description, err := normalizeGroupDescription(req.Description)
	if err != nil {
		return GroupInfo{}, err
	}

	if err := l.ensureUserExists(ctx, creatorUserID); err != nil {
		return GroupInfo{}, err
	}
	memberUserIDs, err := normalizeGroupMemberIDs(creatorUserID, req.MemberUserIDs)
	if err != nil {
		return GroupInfo{}, err
	}
	for _, userID := range memberUserIDs {
		if userID == creatorUserID {
			continue
		}
		if err := l.ensureUserExists(ctx, userID); err != nil {
			return GroupInfo{}, err
		}
	}

	group, _, err := l.repo.CreateGroup(ctx, model.Group{
		Name:        name,
		Description: description,
	}, creatorUserID, memberUserIDs)
	if err != nil {
		return GroupInfo{}, err
	}

	return toGroupInfo(group), nil
}

func (l *GroupsLogic) ListGroups(ctx context.Context, req ListGroupsRequest) (ListGroupsResponse, error) {
	userID, err := normalizeRequiredID(req.UserID, "user_id")
	if err != nil {
		return ListGroupsResponse{}, err
	}
	if err := l.ensureUserExists(ctx, userID); err != nil {
		return ListGroupsResponse{}, err
	}

	groups, err := l.repo.ListGroupsForUser(ctx, userID)
	if err != nil {
		return ListGroupsResponse{}, err
	}
	result := ListGroupsResponse{Groups: make([]GroupInfo, 0, len(groups))}
	for _, group := range groups {
		result.Groups = append(result.Groups, toGroupInfo(group))
	}
	return result, nil
}

func (l *GroupsLogic) GetGroup(ctx context.Context, req GetGroupRequest) (GroupInfo, error) {
	groupID, err := normalizeRequiredID(req.GroupID, "group_id")
	if err != nil {
		return GroupInfo{}, err
	}
	requesterUserID, err := normalizeOptionalID(req.RequesterUserID, "requester_user_id")
	if err != nil {
		return GroupInfo{}, err
	}

	group, err := l.repo.GetGroup(ctx, groupID)
	if err != nil {
		return GroupInfo{}, err
	}
	if requesterUserID != "" {
		member, err := l.activeMember(ctx, groupID, requesterUserID)
		if err != nil {
			return GroupInfo{}, err
		}
		return toGroupInfoWithRole(group, member.Role), nil
	}

	return toGroupInfo(group), nil
}

func (l *GroupsLogic) UpdateGroup(ctx context.Context, req UpdateGroupRequest) (GroupInfo, error) {
	groupID, err := normalizeRequiredID(req.GroupID, "group_id")
	if err != nil {
		return GroupInfo{}, err
	}
	operatorUserID, err := normalizeRequiredID(req.OperatorUserID, "operator_user_id")
	if err != nil {
		return GroupInfo{}, err
	}

	group, err := l.repo.GetGroup(ctx, groupID)
	if err != nil {
		return GroupInfo{}, err
	}
	operator, err := l.ensureCanManageGroup(ctx, groupID, operatorUserID)
	if err != nil {
		return GroupInfo{}, err
	}

	name := group.Name
	if strings.TrimSpace(req.Name) != "" {
		name, err = normalizeGroupName(req.Name)
		if err != nil {
			return GroupInfo{}, err
		}
	}
	description := group.Description
	announcement := req.Announcement
	if strings.TrimSpace(announcement) == "" {
		announcement = req.Description
	}
	if strings.TrimSpace(announcement) != "" {
		description, err = normalizeGroupDescription(announcement)
		if err != nil {
			return GroupInfo{}, err
		}
	}
	if name == group.Name && description == group.Description {
		return GroupInfo{}, apperror.InvalidArgument("name or announcement is required")
	}

	group.Name = name
	group.Description = description
	updated, err := l.repo.UpdateGroup(ctx, group)
	if err != nil {
		return GroupInfo{}, err
	}
	return toGroupInfoWithRole(updated, operator.Role), nil
}

func (l *GroupsLogic) AddMember(ctx context.Context, req AddMemberRequest) (MemberResponse, error) {
	groupID, err := normalizeRequiredID(req.GroupID, "group_id")
	if err != nil {
		return MemberResponse{}, err
	}
	operatorUserID, err := normalizeRequiredID(req.OperatorUserID, "operator_user_id")
	if err != nil {
		return MemberResponse{}, err
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		userID = operatorUserID
	}
	userID, err = normalizeRequiredID(userID, "user_id")
	if err != nil {
		return MemberResponse{}, err
	}

	group, err := l.repo.GetGroup(ctx, groupID)
	if err != nil {
		return MemberResponse{}, err
	}
	if err := l.ensureUserExists(ctx, operatorUserID); err != nil {
		return MemberResponse{}, err
	}
	if userID != operatorUserID {
		if group.CreatorUserID != operatorUserID {
			return MemberResponse{}, apperror.Forbidden("only group owner can add another member")
		}
		if err := l.ensureUserExists(ctx, userID); err != nil {
			return MemberResponse{}, err
		}
	}
	if err := l.ensureCanAddActiveMember(ctx, groupID, userID); err != nil {
		return MemberResponse{}, err
	}

	member, alreadyMember, err := l.repo.AddMember(ctx, groupID, userID)
	if err != nil {
		return MemberResponse{}, err
	}

	memberInfo, err := l.hydrateGroupMemberInfo(ctx, member)
	if err != nil {
		return MemberResponse{}, err
	}
	return MemberResponse{Member: memberInfo, AlreadyMember: alreadyMember}, nil
}

func (l *GroupsLogic) JoinGroup(ctx context.Context, req JoinGroupRequest) (MemberResponse, error) {
	groupID, err := normalizeRequiredID(req.GroupID, "group_id")
	if err != nil {
		return MemberResponse{}, err
	}
	userID, err := normalizeRequiredID(req.UserID, "user_id")
	if err != nil {
		return MemberResponse{}, err
	}

	if _, err := l.repo.GetGroup(ctx, groupID); err != nil {
		return MemberResponse{}, err
	}
	if err := l.ensureUserExists(ctx, userID); err != nil {
		return MemberResponse{}, err
	}
	if err := l.ensureCanAddActiveMember(ctx, groupID, userID); err != nil {
		return MemberResponse{}, err
	}

	member, alreadyMember, err := l.repo.AddMember(ctx, groupID, userID)
	if err != nil {
		return MemberResponse{}, err
	}

	memberInfo, err := l.hydrateGroupMemberInfo(ctx, member)
	if err != nil {
		return MemberResponse{}, err
	}
	return MemberResponse{Member: memberInfo, AlreadyMember: alreadyMember}, nil
}

func (l *GroupsLogic) LeaveGroup(ctx context.Context, req LeaveGroupRequest) (MemberResponse, error) {
	groupID, err := normalizeRequiredID(req.GroupID, "group_id")
	if err != nil {
		return MemberResponse{}, err
	}
	userID, err := normalizeRequiredID(req.UserID, "user_id")
	if err != nil {
		return MemberResponse{}, err
	}

	group, err := l.repo.GetGroup(ctx, groupID)
	if err != nil {
		return MemberResponse{}, err
	}
	if group.CreatorUserID == userID {
		members, err := l.repo.ListActiveMembers(ctx, groupID)
		if err != nil {
			return MemberResponse{}, err
		}
		userIsActiveMember := false
		for _, member := range members {
			if member.UserID == userID {
				userIsActiveMember = true
				break
			}
		}
		if userIsActiveMember && len(members) <= 1 {
			return MemberResponse{}, apperror.Forbidden("group owner cannot leave as the only active member")
		}
	}

	member, err := l.repo.LeaveGroup(ctx, groupID, userID)
	if err != nil {
		return MemberResponse{}, err
	}

	memberInfo, err := l.hydrateGroupMemberInfo(ctx, member)
	if err != nil {
		return MemberResponse{}, err
	}
	return MemberResponse{Member: memberInfo}, nil
}

func (l *GroupsLogic) KickMember(ctx context.Context, req KickMemberRequest) (MemberResponse, error) {
	groupID, err := normalizeRequiredID(req.GroupID, "group_id")
	if err != nil {
		return MemberResponse{}, err
	}
	operatorUserID, err := normalizeRequiredID(req.OperatorUserID, "operator_user_id")
	if err != nil {
		return MemberResponse{}, err
	}
	userID, err := normalizeRequiredID(req.UserID, "user_id")
	if err != nil {
		return MemberResponse{}, err
	}
	if operatorUserID == userID {
		return MemberResponse{}, apperror.InvalidArgument("use leave group to remove yourself")
	}

	if _, err := l.repo.GetGroup(ctx, groupID); err != nil {
		return MemberResponse{}, err
	}
	operator, err := l.ensureCanManageGroup(ctx, groupID, operatorUserID)
	if err != nil {
		return MemberResponse{}, err
	}
	target, err := l.activeMember(ctx, groupID, userID)
	if err != nil {
		return MemberResponse{}, err
	}
	if target.Role == model.MemberRoleOwner {
		return MemberResponse{}, apperror.Forbidden("group owner cannot be kicked")
	}
	if operator.Role == model.MemberRoleAdmin && target.Role != model.MemberRoleMember {
		return MemberResponse{}, apperror.Forbidden("group admin can only kick normal members")
	}

	member, err := l.repo.RemoveMember(ctx, groupID, userID)
	if err != nil {
		return MemberResponse{}, err
	}
	memberInfo, err := l.hydrateGroupMemberInfo(ctx, member)
	if err != nil {
		return MemberResponse{}, err
	}
	return MemberResponse{Member: memberInfo}, nil
}

func (l *GroupsLogic) ListMembers(ctx context.Context, req ListMembersRequest) (ListMembersResponse, error) {
	groupID, err := normalizeRequiredID(req.GroupID, "group_id")
	if err != nil {
		return ListMembersResponse{}, err
	}
	requesterUserID, err := normalizeOptionalID(req.RequesterUserID, "requester_user_id")
	if err != nil {
		return ListMembersResponse{}, err
	}

	members, err := l.repo.ListActiveMembers(ctx, groupID)
	if err != nil {
		return ListMembersResponse{}, err
	}
	if requesterUserID != "" && !containsActiveMember(members, requesterUserID) {
		return ListMembersResponse{}, apperror.Forbidden("requester is not a group member")
	}

	result := ListMembersResponse{
		GroupID: groupID,
		Members: make([]GroupMemberInfo, 0, len(members)),
	}
	for _, member := range members {
		memberInfo, err := l.hydrateGroupMemberInfo(ctx, member)
		if err != nil {
			return ListMembersResponse{}, err
		}
		result.Members = append(result.Members, memberInfo)
	}

	return result, nil
}

func (l *GroupsLogic) ensureCanAddActiveMember(ctx context.Context, groupID string, userID string) error {
	members, err := l.repo.ListActiveMembers(ctx, groupID)
	if err != nil {
		return err
	}
	for _, member := range members {
		if member.UserID == userID && member.State == model.MemberStateActive {
			return nil
		}
	}
	if len(members) >= maxGroupMembersV1 {
		return apperror.InvalidArgument("group member limit is 200")
	}
	return nil
}

func (l *GroupsLogic) ensureUserExists(ctx context.Context, userID string) error {
	if l.userExists == nil {
		return apperror.Internal("user existence checker is not configured")
	}
	return l.userExists.EnsureUserExists(ctx, userID)
}

func (l *GroupsLogic) activeMember(ctx context.Context, groupID string, userID string) (model.GroupMember, error) {
	members, err := l.repo.ListActiveMembers(ctx, groupID)
	if err != nil {
		return model.GroupMember{}, err
	}
	for _, member := range members {
		if member.UserID == userID && member.State == model.MemberStateActive {
			member.Role = normalizeMemberRole(member.Role)
			return member, nil
		}
	}
	return model.GroupMember{}, apperror.Forbidden("requester is not a group member")
}

func (l *GroupsLogic) ensureCanManageGroup(ctx context.Context, groupID string, userID string) (model.GroupMember, error) {
	member, err := l.activeMember(ctx, groupID, userID)
	if err != nil {
		return model.GroupMember{}, err
	}
	if member.Role != model.MemberRoleOwner && member.Role != model.MemberRoleAdmin {
		return model.GroupMember{}, apperror.Forbidden("only group owner or admin can manage group")
	}
	return member, nil
}

func containsActiveMember(members []model.GroupMember, userID string) bool {
	for _, member := range members {
		if member.UserID == userID && member.State == model.MemberStateActive {
			return true
		}
	}
	return false
}

func normalizeRequiredID(value string, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument(field + " is required")
	}
	if len([]rune(value)) > 64 {
		return "", apperror.InvalidArgument(field + " must be 64 characters or fewer")
	}
	return value, nil
}

func normalizeOptionalID(value string, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	return normalizeRequiredID(value, field)
}

func normalizeGroupName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument("name is required")
	}
	if len([]rune(value)) > 64 {
		return "", apperror.InvalidArgument("name must be 64 characters or fewer")
	}
	return value, nil
}

func normalizeGroupDescription(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > 256 {
		return "", apperror.InvalidArgument("description must be 256 characters or fewer")
	}
	return value, nil
}

func normalizeGroupMemberIDs(creatorUserID string, rawMemberUserIDs []string) ([]string, error) {
	seen := map[string]struct{}{
		creatorUserID: {},
	}
	memberUserIDs := []string{creatorUserID}
	for _, rawUserID := range rawMemberUserIDs {
		userID, err := normalizeRequiredID(rawUserID, "member_user_ids")
		if err != nil {
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

func (l *GroupsLogic) hydrateGroupMemberInfo(ctx context.Context, member model.GroupMember) (GroupMemberInfo, error) {
	info := toGroupMemberInfo(member)
	lookup, ok := l.userExists.(UserProfileLookup)
	if !ok || lookup == nil {
		return info, nil
	}
	profile, err := lookup.LookupUserProfile(ctx, member.UserID)
	if err != nil {
		return GroupMemberInfo{}, err
	}
	info.Identifier = profile.Identifier
	info.DisplayName = humanReadableUserName(profile)
	info.Name = profile.Name
	info.AvatarMediaID = profile.AvatarMediaID
	info.AvatarURL = profile.AvatarURL
	return info, nil
}

func humanReadableUserName(profile UserProfile) string {
	for _, value := range []string{profile.DisplayName, profile.Name, profile.Identifier} {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return strings.TrimSpace(profile.UserID)
}

func toGroupInfo(group model.Group) GroupInfo {
	return toGroupInfoWithRole(group, "")
}

func toGroupInfoWithRole(group model.Group, currentUserRole string) GroupInfo {
	role := ""
	if currentUserRole != "" {
		role = normalizeMemberRole(currentUserRole)
	}
	return GroupInfo{
		GroupID:         group.GroupID,
		Name:            group.Name,
		Description:     group.Description,
		Announcement:    group.Description,
		AvatarMediaID:   group.AvatarMediaID,
		AvatarURL:       group.AvatarURL,
		CreatorUserID:   group.CreatorUserID,
		CurrentUserRole: role,
		CreatedAt:       formatGroupTime(group.CreatedAt),
		UpdatedAt:       formatGroupTime(group.UpdatedAt),
	}
}

func toGroupMemberInfo(member model.GroupMember) GroupMemberInfo {
	return GroupMemberInfo{
		GroupID:  member.GroupID,
		UserID:   member.UserID,
		Role:     normalizeMemberRole(member.Role),
		State:    member.State,
		JoinedAt: formatGroupTime(member.JoinedAt),
		LeftAt:   formatGroupTime(member.LeftAt),
	}
}

func normalizeMemberRole(role string) string {
	switch role {
	case model.MemberRoleOwner, model.MemberRoleAdmin, model.MemberRoleMember:
		return role
	default:
		return model.MemberRoleMember
	}
}

func formatGroupTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
