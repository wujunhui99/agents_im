package logic

import (
	"context"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

type UserExistenceChecker interface {
	EnsureUserExists(ctx context.Context, userID string) error
}

type UserLogicExistenceChecker struct {
	userLogic *UserLogic
}

func NewUserLogicExistenceChecker(userLogic *UserLogic) UserLogicExistenceChecker {
	return UserLogicExistenceChecker{userLogic: userLogic}
}

func (c UserLogicExistenceChecker) EnsureUserExists(ctx context.Context, userID string) error {
	if c.userLogic == nil {
		return apperror.Internal("user existence checker is not configured")
	}

	_, err := c.userLogic.GetUserByID(ctx, GetUserByIDRequest{UserID: userID})
	return err
}

type GroupsLogic struct {
	repo       repository.GroupsRepository
	userExists UserExistenceChecker
}

func NewGroupsLogic(repo repository.GroupsRepository, userExists UserExistenceChecker) *GroupsLogic {
	return &GroupsLogic{repo: repo, userExists: userExists}
}

type GroupInfo struct {
	GroupID       string `json:"group_id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	CreatorUserID string `json:"creator_user_id"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type GroupMemberInfo struct {
	GroupID  string `json:"group_id"`
	UserID   string `json:"user_id"`
	State    string `json:"state"`
	JoinedAt string `json:"joined_at"`
	LeftAt   string `json:"left_at"`
}

type CreateGroupRequest struct {
	CreatorUserID string `json:"creator_user_id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
}

type GetGroupRequest struct {
	GroupID string `json:"group_id"`
}

type AddMemberRequest struct {
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
	GroupID string `json:"group_id"`
}

type MemberResponse struct {
	Member        GroupMemberInfo `json:"member"`
	AlreadyMember bool            `json:"already_member"`
}

type ListMembersResponse struct {
	GroupID string            `json:"group_id"`
	Members []GroupMemberInfo `json:"members"`
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

	group, _, err := l.repo.CreateGroup(ctx, model.Group{
		Name:        name,
		Description: description,
	}, creatorUserID)
	if err != nil {
		return GroupInfo{}, err
	}

	return toGroupInfo(group), nil
}

func (l *GroupsLogic) GetGroup(ctx context.Context, req GetGroupRequest) (GroupInfo, error) {
	groupID, err := normalizeRequiredID(req.GroupID, "group_id")
	if err != nil {
		return GroupInfo{}, err
	}

	group, err := l.repo.GetGroup(ctx, groupID)
	if err != nil {
		return GroupInfo{}, err
	}

	return toGroupInfo(group), nil
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

	if _, err := l.repo.GetGroup(ctx, groupID); err != nil {
		return MemberResponse{}, err
	}
	if err := l.ensureUserExists(ctx, operatorUserID); err != nil {
		return MemberResponse{}, err
	}
	if userID != operatorUserID {
		if err := l.ensureUserExists(ctx, userID); err != nil {
			return MemberResponse{}, err
		}
	}

	member, alreadyMember, err := l.repo.AddMember(ctx, groupID, userID)
	if err != nil {
		return MemberResponse{}, err
	}

	return MemberResponse{Member: toGroupMemberInfo(member), AlreadyMember: alreadyMember}, nil
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

	member, alreadyMember, err := l.repo.AddMember(ctx, groupID, userID)
	if err != nil {
		return MemberResponse{}, err
	}

	return MemberResponse{Member: toGroupMemberInfo(member), AlreadyMember: alreadyMember}, nil
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

	member, err := l.repo.LeaveGroup(ctx, groupID, userID)
	if err != nil {
		return MemberResponse{}, err
	}

	return MemberResponse{Member: toGroupMemberInfo(member)}, nil
}

func (l *GroupsLogic) ListMembers(ctx context.Context, req ListMembersRequest) (ListMembersResponse, error) {
	groupID, err := normalizeRequiredID(req.GroupID, "group_id")
	if err != nil {
		return ListMembersResponse{}, err
	}

	members, err := l.repo.ListActiveMembers(ctx, groupID)
	if err != nil {
		return ListMembersResponse{}, err
	}

	result := ListMembersResponse{
		GroupID: groupID,
		Members: make([]GroupMemberInfo, 0, len(members)),
	}
	for _, member := range members {
		result.Members = append(result.Members, toGroupMemberInfo(member))
	}

	return result, nil
}

func (l *GroupsLogic) ensureUserExists(ctx context.Context, userID string) error {
	if l.userExists == nil {
		return apperror.Internal("user existence checker is not configured")
	}
	return l.userExists.EnsureUserExists(ctx, userID)
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

func toGroupInfo(group model.Group) GroupInfo {
	return GroupInfo{
		GroupID:       group.GroupID,
		Name:          group.Name,
		Description:   group.Description,
		CreatorUserID: group.CreatorUserID,
		CreatedAt:     formatGroupTime(group.CreatedAt),
		UpdatedAt:     formatGroupTime(group.UpdatedAt),
	}
}

func toGroupMemberInfo(member model.GroupMember) GroupMemberInfo {
	return GroupMemberInfo{
		GroupID:  member.GroupID,
		UserID:   member.UserID,
		State:    member.State,
		JoinedAt: formatGroupTime(member.JoinedAt),
		LeftAt:   formatGroupTime(member.LeftAt),
	}
}

func formatGroupTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
