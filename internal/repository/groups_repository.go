package repository

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/model"
)

type GroupsRepository interface {
	CreateGroup(ctx context.Context, group model.Group, creatorUserID string, memberUserIDs []string) (model.Group, []model.GroupMember, error)
	GetGroup(ctx context.Context, groupID string) (model.Group, error)
	ListGroupsForUser(ctx context.Context, userID string) ([]model.Group, error)
	AddMember(ctx context.Context, groupID string, userID string) (model.GroupMember, bool, error)
	LeaveGroup(ctx context.Context, groupID string, userID string) (model.GroupMember, error)
	ListActiveMembers(ctx context.Context, groupID string) ([]model.GroupMember, error)
}
