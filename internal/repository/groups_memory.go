package repository

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
)

type MemoryGroupsRepository struct {
	mu          sync.RWMutex
	nextGroupID uint64
	groups      map[string]model.Group
	members     map[string]map[string]model.GroupMember
	now         func() time.Time
}

func NewMemoryGroupsRepository() *MemoryGroupsRepository {
	return &MemoryGroupsRepository{
		groups:  make(map[string]model.Group),
		members: make(map[string]map[string]model.GroupMember),
		now:     time.Now,
	}
}

func (r *MemoryGroupsRepository) CreateGroup(_ context.Context, group model.Group, creatorUserID string, memberUserIDs []string) (model.Group, []model.GroupMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextGroupID++
	if group.GroupID == "" {
		group.GroupID = fmt.Sprintf("grp_%06d", r.nextGroupID)
	}
	now := r.now().UTC()
	group.CreatorUserID = creatorUserID
	group.CreatedAt = now
	group.UpdatedAt = now

	r.groups[group.GroupID] = group.Clone()
	r.members[group.GroupID] = make(map[string]model.GroupMember, len(memberUserIDs))

	members := make([]model.GroupMember, 0, len(memberUserIDs))
	seen := make(map[string]struct{}, len(memberUserIDs)+1)
	addMember := func(userID string) {
		if userID == "" {
			return
		}
		if _, ok := seen[userID]; ok {
			return
		}
		seen[userID] = struct{}{}
		member := model.GroupMember{
			GroupID:  group.GroupID,
			UserID:   userID,
			Role:     memberRoleForUser(group, userID),
			State:    model.MemberStateActive,
			JoinedAt: now,
		}
		r.members[group.GroupID][userID] = member.Clone()
		members = append(members, member.Clone())
	}
	addMember(creatorUserID)
	for _, userID := range memberUserIDs {
		addMember(userID)
	}

	return group.Clone(), members, nil
}

func (r *MemoryGroupsRepository) GetGroup(_ context.Context, groupID string) (model.Group, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	group, exists := r.groups[groupID]
	if !exists {
		return model.Group{}, apperror.NotFound("group not found")
	}

	return group.Clone(), nil
}

func (r *MemoryGroupsRepository) UpdateGroup(_ context.Context, group model.Group) (model.Group, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stored, exists := r.groups[group.GroupID]
	if !exists {
		return model.Group{}, apperror.NotFound("group not found")
	}

	stored.Name = group.Name
	stored.Description = group.Description
	stored.AvatarMediaID = group.AvatarMediaID
	stored.AvatarURL = group.AvatarURL
	stored.UpdatedAt = r.now().UTC()
	r.groups[group.GroupID] = stored.Clone()
	return stored.Clone(), nil
}

func (r *MemoryGroupsRepository) ListGroupsForUser(_ context.Context, userID string) ([]model.Group, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	groups := make([]model.Group, 0)
	for groupID, groupMembers := range r.members {
		member, exists := groupMembers[userID]
		if !exists || member.State != model.MemberStateActive {
			continue
		}
		group, exists := r.groups[groupID]
		if !exists {
			continue
		}
		groups = append(groups, group.Clone())
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].UpdatedAt.Equal(groups[j].UpdatedAt) {
			return groups[i].GroupID < groups[j].GroupID
		}
		return groups[i].UpdatedAt.After(groups[j].UpdatedAt)
	})
	return groups, nil
}

func (r *MemoryGroupsRepository) AddMember(_ context.Context, groupID string, userID string) (model.GroupMember, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.groups[groupID]; !exists {
		return model.GroupMember{}, false, apperror.NotFound("group not found")
	}

	groupMembers, exists := r.members[groupID]
	if !exists {
		groupMembers = make(map[string]model.GroupMember)
		r.members[groupID] = groupMembers
	}

	if member, exists := groupMembers[userID]; exists {
		if member.State == model.MemberStateActive {
			return member.Clone(), true, nil
		}

		now := r.now().UTC()
		member.Role = memberRoleForUser(r.groups[groupID], userID)
		member.State = model.MemberStateActive
		member.JoinedAt = now
		member.LeftAt = time.Time{}
		groupMembers[userID] = member.Clone()
		return member.Clone(), false, nil
	}

	member := model.GroupMember{
		GroupID:  groupID,
		UserID:   userID,
		Role:     memberRoleForUser(r.groups[groupID], userID),
		State:    model.MemberStateActive,
		JoinedAt: r.now().UTC(),
	}
	groupMembers[userID] = member.Clone()
	return member.Clone(), false, nil
}

func (r *MemoryGroupsRepository) LeaveGroup(_ context.Context, groupID string, userID string) (model.GroupMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.groups[groupID]; !exists {
		return model.GroupMember{}, apperror.NotFound("group not found")
	}

	groupMembers, exists := r.members[groupID]
	if !exists {
		return model.GroupMember{}, apperror.NotFound("member not found")
	}

	member, exists := groupMembers[userID]
	if !exists || member.State != model.MemberStateActive {
		return model.GroupMember{}, apperror.NotFound("member not found")
	}

	member.State = model.MemberStateLeft
	member.LeftAt = r.now().UTC()
	groupMembers[userID] = member.Clone()
	return member.Clone(), nil
}

func (r *MemoryGroupsRepository) RemoveMember(ctx context.Context, groupID string, userID string) (model.GroupMember, error) {
	return r.LeaveGroup(ctx, groupID, userID)
}

func (r *MemoryGroupsRepository) SetMemberRole(_ context.Context, groupID string, userID string, role string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.groups[groupID]; !exists {
		return apperror.NotFound("group not found")
	}
	groupMembers, exists := r.members[groupID]
	if !exists {
		return apperror.NotFound("member not found")
	}
	member, exists := groupMembers[userID]
	if !exists || member.State != model.MemberStateActive {
		return apperror.NotFound("member not found")
	}
	member.Role = normalizeStoredMemberRole(role)
	groupMembers[userID] = member.Clone()
	return nil
}

func (r *MemoryGroupsRepository) ListActiveMembers(_ context.Context, groupID string) ([]model.GroupMember, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.groups[groupID]; !exists {
		return nil, apperror.NotFound("group not found")
	}

	groupMembers := r.members[groupID]
	members := make([]model.GroupMember, 0, len(groupMembers))
	for _, member := range groupMembers {
		if member.State == model.MemberStateActive {
			members = append(members, member.Clone())
		}
	}
	sort.Slice(members, func(i, j int) bool {
		return members[i].UserID < members[j].UserID
	})

	return members, nil
}

func memberRoleForUser(group model.Group, userID string) string {
	if group.CreatorUserID == userID {
		return model.MemberRoleOwner
	}
	return model.MemberRoleMember
}

func normalizeStoredMemberRole(role string) string {
	switch role {
	case model.MemberRoleOwner, model.MemberRoleAdmin, model.MemberRoleMember:
		return role
	default:
		return model.MemberRoleMember
	}
}
