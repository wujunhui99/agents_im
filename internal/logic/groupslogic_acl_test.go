package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/internal/apperror"
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
