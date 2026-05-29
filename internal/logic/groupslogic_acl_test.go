package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

type staticUserExistenceChecker map[string]struct{}

func (c staticUserExistenceChecker) EnsureUserExists(_ context.Context, userID string) error {
	if _, ok := c[userID]; !ok {
		return apperror.NotFound("user not found")
	}
	return nil
}

func TestGroupsLogicReadRequiresActiveMemberWhenRequesterProvided(t *testing.T) {
	ctx := context.Background()
	groupsLogic := NewGroupsLogic(repository.NewMemoryGroupsRepository(), staticUserExistenceChecker{
		"creator":  {},
		"member":   {},
		"outsider": {},
	})

	group, err := groupsLogic.CreateGroup(ctx, CreateGroupRequest{
		CreatorUserID: "creator",
		Name:          "ACL Group",
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := groupsLogic.JoinGroup(ctx, JoinGroupRequest{
		GroupID: group.GroupID,
		UserID:  "member",
	}); err != nil {
		t.Fatalf("join member: %v", err)
	}

	for _, requesterUserID := range []string{"creator", "member"} {
		if _, err := groupsLogic.GetGroup(ctx, GetGroupRequest{
			GroupID:         group.GroupID,
			RequesterUserID: requesterUserID,
		}); err != nil {
			t.Fatalf("get group as %s: %v", requesterUserID, err)
		}
		if _, err := groupsLogic.ListMembers(ctx, ListMembersRequest{
			GroupID:         group.GroupID,
			RequesterUserID: requesterUserID,
		}); err != nil {
			t.Fatalf("list members as %s: %v", requesterUserID, err)
		}
	}

	if _, err := groupsLogic.GetGroup(ctx, GetGroupRequest{
		GroupID:         group.GroupID,
		RequesterUserID: "outsider",
	}); err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("get group as outsider error = %v, want FORBIDDEN", err)
	}
	if _, err := groupsLogic.ListMembers(ctx, ListMembersRequest{
		GroupID:         group.GroupID,
		RequesterUserID: "outsider",
	}); err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("list members as outsider error = %v, want FORBIDDEN", err)
	}

	if _, err := groupsLogic.GetGroup(ctx, GetGroupRequest{GroupID: group.GroupID}); err != nil {
		t.Fatalf("trusted internal get group without requester should remain compatible: %v", err)
	}
	if members, err := groupsLogic.ListMembers(ctx, ListMembersRequest{GroupID: group.GroupID}); err != nil {
		t.Fatalf("trusted internal list members without requester should remain compatible: %v", err)
	} else if len(members.Members) != 2 {
		t.Fatalf("trusted internal list should include active members: %+v", members.Members)
	}

	if _, err := groupsLogic.LeaveGroup(ctx, LeaveGroupRequest{
		GroupID: group.GroupID,
		UserID:  "member",
	}); err != nil {
		t.Fatalf("member leave: %v", err)
	}
	if _, err := groupsLogic.ListMembers(ctx, ListMembersRequest{
		GroupID:         group.GroupID,
		RequesterUserID: "member",
	}); err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("left member list error = %v, want FORBIDDEN", err)
	}
}

func TestGroupsLogicAddMemberRequiresOwnerWhenAddingAnotherUser(t *testing.T) {
	ctx := context.Background()
	groupsLogic := NewGroupsLogic(repository.NewMemoryGroupsRepository(), staticUserExistenceChecker{
		"creator": {},
		"member":  {},
		"invitee": {},
	})

	group, err := groupsLogic.CreateGroup(ctx, CreateGroupRequest{
		CreatorUserID: "creator",
		Name:          "Owner Add Only",
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := groupsLogic.JoinGroup(ctx, JoinGroupRequest{
		GroupID: group.GroupID,
		UserID:  "member",
	}); err != nil {
		t.Fatalf("join member: %v", err)
	}

	if _, err := groupsLogic.AddMember(ctx, AddMemberRequest{
		GroupID:        group.GroupID,
		OperatorUserID: "member",
		UserID:         "invitee",
	}); err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("member add another user error = %v, want FORBIDDEN", err)
	}

	members, err := groupsLogic.ListMembers(ctx, ListMembersRequest{GroupID: group.GroupID})
	if err != nil {
		t.Fatalf("trusted internal list after rejected add: %v", err)
	}
	if len(members.Members) != 2 {
		t.Fatalf("rejected add should not change active members: %+v", members.Members)
	}

	added, err := groupsLogic.AddMember(ctx, AddMemberRequest{
		GroupID:        group.GroupID,
		OperatorUserID: "creator",
		UserID:         "invitee",
	})
	if err != nil {
		t.Fatalf("owner add invitee: %v", err)
	}
	if added.AlreadyMember || added.Member.UserID != "invitee" {
		t.Fatalf("unexpected owner add response: %+v", added)
	}
}

func TestGroupsLogicUpdateRequiresOwnerOrAdmin(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryGroupsRepository()
	groupsLogic := NewGroupsLogic(repo, staticUserExistenceChecker{
		"creator": {},
		"admin":   {},
		"member":  {},
	})

	group, err := groupsLogic.CreateGroup(ctx, CreateGroupRequest{
		CreatorUserID: "creator",
		Name:          "Manageable Group",
		Description:   "old announcement",
		MemberUserIDs: []string{"admin", "member"},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := repo.SetMemberRole(ctx, group.GroupID, "admin", model.MemberRoleAdmin); err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	if _, err := groupsLogic.UpdateGroup(ctx, UpdateGroupRequest{
		GroupID:        group.GroupID,
		OperatorUserID: "member",
		Name:           "member rename",
	}); err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("member update error = %v, want FORBIDDEN", err)
	}

	updatedByAdmin, err := groupsLogic.UpdateGroup(ctx, UpdateGroupRequest{
		GroupID:        group.GroupID,
		OperatorUserID: "admin",
		Announcement:   "admin announcement",
	})
	if err != nil {
		t.Fatalf("admin update announcement: %v", err)
	}
	if updatedByAdmin.Name != "Manageable Group" || updatedByAdmin.Announcement != "admin announcement" || updatedByAdmin.CurrentUserRole != model.MemberRoleAdmin {
		t.Fatalf("unexpected admin update response: %+v", updatedByAdmin)
	}

	updatedByOwner, err := groupsLogic.UpdateGroup(ctx, UpdateGroupRequest{
		GroupID:        group.GroupID,
		OperatorUserID: "creator",
		Name:           "Owner Renamed Group",
	})
	if err != nil {
		t.Fatalf("owner update name: %v", err)
	}
	if updatedByOwner.Name != "Owner Renamed Group" || updatedByOwner.Announcement != "admin announcement" || updatedByOwner.CurrentUserRole != model.MemberRoleOwner {
		t.Fatalf("unexpected owner update response: %+v", updatedByOwner)
	}
}

func TestGroupsLogicKickMemberRoleConstraintsAndRemovesActiveMember(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryGroupsRepository()
	groupsLogic := NewGroupsLogic(repo, staticUserExistenceChecker{
		"creator": {},
		"admin":   {},
		"peer":    {},
		"member":  {},
		"target":  {},
	})

	group, err := groupsLogic.CreateGroup(ctx, CreateGroupRequest{
		CreatorUserID: "creator",
		Name:          "Kickable Group",
		MemberUserIDs: []string{"admin", "peer", "member", "target"},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := repo.SetMemberRole(ctx, group.GroupID, "admin", model.MemberRoleAdmin); err != nil {
		t.Fatalf("promote admin: %v", err)
	}
	if err := repo.SetMemberRole(ctx, group.GroupID, "peer", model.MemberRoleAdmin); err != nil {
		t.Fatalf("promote peer admin: %v", err)
	}

	if _, err := groupsLogic.KickMember(ctx, KickMemberRequest{
		GroupID:        group.GroupID,
		OperatorUserID: "member",
		UserID:         "target",
	}); err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("normal member kick error = %v, want FORBIDDEN", err)
	}

	if _, err := groupsLogic.KickMember(ctx, KickMemberRequest{
		GroupID:        group.GroupID,
		OperatorUserID: "admin",
		UserID:         "creator",
	}); err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("admin kick owner error = %v, want FORBIDDEN", err)
	}
	if _, err := groupsLogic.KickMember(ctx, KickMemberRequest{
		GroupID:        group.GroupID,
		OperatorUserID: "admin",
		UserID:         "peer",
	}); err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("admin kick admin error = %v, want FORBIDDEN", err)
	}

	kicked, err := groupsLogic.KickMember(ctx, KickMemberRequest{
		GroupID:        group.GroupID,
		OperatorUserID: "admin",
		UserID:         "target",
	})
	if err != nil {
		t.Fatalf("admin kick target: %v", err)
	}
	if kicked.Member.UserID != "target" || kicked.Member.State != model.MemberStateLeft {
		t.Fatalf("unexpected kicked response: %+v", kicked)
	}

	members, err := groupsLogic.ListMembers(ctx, ListMembersRequest{
		GroupID:         group.GroupID,
		RequesterUserID: "creator",
	})
	if err != nil {
		t.Fatalf("list after kick: %v", err)
	}
	for _, member := range members.Members {
		if member.UserID == "target" {
			t.Fatalf("kicked member still listed as active: %+v", members.Members)
		}
	}
}
