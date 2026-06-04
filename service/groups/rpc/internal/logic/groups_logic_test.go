package logic

import (
	"context"
	"strings"
	"testing"

	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/svc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- fake models（只实现被测路径用到的方法，其余走内嵌接口的 nil 方法，未触发即可）---

type fakeGroupsModel struct {
	model.GroupsModel
	groups map[string]*model.Groups
}

func (f *fakeGroupsModel) FindOne(_ context.Context, groupID string) (*model.Groups, error) {
	if g, ok := f.groups[groupID]; ok {
		return g, nil
	}
	return nil, model.ErrNotFound
}

type fakeMembersModel struct {
	model.GroupMembersModel
	byKey  map[string]*model.GroupMembers   // groupID|accountID -> row
	active map[string][]*model.GroupMembers // groupID -> active rows
}

func (f *fakeMembersModel) FindOneByGroupIdAccountId(_ context.Context, groupID, accountID string) (*model.GroupMembers, error) {
	if m, ok := f.byKey[groupID+"|"+accountID]; ok {
		return m, nil
	}
	return nil, model.ErrNotFound
}

func (f *fakeMembersModel) FindActiveByGroup(_ context.Context, groupID string) ([]*model.GroupMembers, error) {
	return f.active[groupID], nil
}

func (f *fakeMembersModel) UpsertActiveMember(_ context.Context, groupID, accountID string, role int64) (*model.GroupMembers, error) {
	return &model.GroupMembers{GroupId: groupID, AccountId: accountID, Role: role, Status: model.MemberStatusActive}, nil
}

func (f *fakeMembersModel) SetMemberLeft(_ context.Context, groupID, accountID string) (*model.GroupMembers, error) {
	if m, ok := f.byKey[groupID+"|"+accountID]; ok && m.Status == model.MemberStatusActive {
		left := *m
		left.Status = model.MemberStatusLeft
		return &left, nil
	}
	return nil, model.ErrNotFound
}

func newSvc(g *fakeGroupsModel, m *fakeMembersModel) *svc.ServiceContext {
	return &svc.ServiceContext{GroupsModel: g, GroupMembersModel: m}
}

func member(groupID, accountID string, role, status int64) *model.GroupMembers {
	return &model.GroupMembers{GroupId: groupID, AccountId: accountID, Role: role, Status: status}
}

func wantCode(t *testing.T, err error, code codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", code)
	}
	if st, _ := status.FromError(err); st.Code() != code {
		t.Fatalf("expected code %s, got %s (%v)", code, st.Code(), err)
	}
}

// --- 纯校验 / 映射 ---

func TestValidate(t *testing.T) {
	if _, err := validateRequiredID("", "group_id"); err == nil {
		t.Fatal("empty id should fail")
	}
	if _, err := validateRequiredID(strings.Repeat("x", 65), "group_id"); err == nil {
		t.Fatal("over-long id should fail")
	}
	if _, err := validateGroupName(strings.Repeat("n", 65)); err == nil {
		t.Fatal("over-long name should fail")
	}
	// 成员上限 200（含 creator）：creator + 200 个唯一成员 = 201，应被拒。
	raw := make([]string, 0, 201)
	for i := 0; i < 201; i++ {
		raw = append(raw, itoa(i))
	}
	if _, err := validateGroupMemberIDs("creator", raw); err == nil {
		t.Fatal("exceeding member limit should fail")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "u0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return "u" + string(b)
}

func TestRoleStateMapping(t *testing.T) {
	cases := map[int64]string{model.MemberRoleOwner: "owner", model.MemberRoleAdmin: "admin", model.MemberRoleMember: "member", 99: "member"}
	for in, want := range cases {
		if got := memberRoleToString(in); got != want {
			t.Fatalf("role %d -> %s, want %s", in, got, want)
		}
	}
	if memberStateToString(model.MemberStatusLeft) != "left" || memberStateToString(model.MemberStatusActive) != "active" {
		t.Fatal("state mapping wrong")
	}
}

// --- 业务规则（fake model）---

func TestGetGroupAttachesRequesterRole(t *testing.T) {
	g := &fakeGroupsModel{groups: map[string]*model.Groups{"g1": {GroupId: "g1", Name: "G", CreatorAccountId: "owner"}}}
	m := &fakeMembersModel{byKey: map[string]*model.GroupMembers{"g1|admin1": member("g1", "admin1", model.MemberRoleAdmin, model.MemberStatusActive)}}
	l := NewGetGroupLogic(context.Background(), newSvc(g, m))

	resp, err := l.GetGroup(&groups.GetGroupRequest{GroupId: "g1", RequesterUserId: "admin1"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.GetGroup().GetCurrentUserRole() != "admin" {
		t.Fatalf("want role admin, got %s", resp.GetGroup().GetCurrentUserRole())
	}

	// 群不存在 -> NotFound
	_, err = NewGetGroupLogic(context.Background(), newSvc(g, m)).GetGroup(&groups.GetGroupRequest{GroupId: "missing"})
	wantCode(t, err, codes.NotFound)
}

func TestAddMemberAlreadyActive(t *testing.T) {
	g := &fakeGroupsModel{groups: map[string]*model.Groups{"g1": {GroupId: "g1", CreatorAccountId: "owner"}}}
	m := &fakeMembersModel{byKey: map[string]*model.GroupMembers{
		"g1|owner": member("g1", "owner", model.MemberRoleOwner, model.MemberStatusActive),
		"g1|u2":    member("g1", "u2", model.MemberRoleMember, model.MemberStatusActive),
	}}
	l := NewAddMemberLogic(context.Background(), newSvc(g, m))
	resp, err := l.AddMember(&groups.AddMemberRequest{GroupId: "g1", OperatorUserId: "owner", UserId: "u2"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !resp.GetAlreadyMember() {
		t.Fatal("expected AlreadyMember=true for active member")
	}
}

func TestAddMemberNonOwnerCannotAddOther(t *testing.T) {
	g := &fakeGroupsModel{groups: map[string]*model.Groups{"g1": {GroupId: "g1", CreatorAccountId: "owner"}}}
	m := &fakeMembersModel{byKey: map[string]*model.GroupMembers{}}
	l := NewAddMemberLogic(context.Background(), newSvc(g, m))
	_, err := l.AddMember(&groups.AddMemberRequest{GroupId: "g1", OperatorUserId: "u9", UserId: "u2"})
	wantCode(t, err, codes.PermissionDenied)
}

func TestKickMemberRules(t *testing.T) {
	g := &fakeGroupsModel{groups: map[string]*model.Groups{"g1": {GroupId: "g1", CreatorAccountId: "owner"}}}
	m := &fakeMembersModel{byKey: map[string]*model.GroupMembers{
		"g1|owner": member("g1", "owner", model.MemberRoleOwner, model.MemberStatusActive),
		"g1|admin": member("g1", "admin", model.MemberRoleAdmin, model.MemberStatusActive),
		"g1|u3":    member("g1", "u3", model.MemberRoleMember, model.MemberStatusActive),
	}}

	// admin 踢 owner -> owner 不可被踢
	_, err := NewKickMemberLogic(context.Background(), newSvc(g, m)).KickMember(&groups.KickMemberRequest{GroupId: "g1", OperatorUserId: "admin", UserId: "owner"})
	wantCode(t, err, codes.PermissionDenied)

	// owner 踢普通成员 -> 成功
	resp, err := NewKickMemberLogic(context.Background(), newSvc(g, m)).KickMember(&groups.KickMemberRequest{GroupId: "g1", OperatorUserId: "owner", UserId: "u3"})
	if err != nil {
		t.Fatalf("owner kick member should succeed: %v", err)
	}
	if resp.GetMember().GetState() != "left" {
		t.Fatalf("kicked member should be left, got %s", resp.GetMember().GetState())
	}

	// 踢自己 -> InvalidArgument
	_, err = NewKickMemberLogic(context.Background(), newSvc(g, m)).KickMember(&groups.KickMemberRequest{GroupId: "g1", OperatorUserId: "owner", UserId: "owner"})
	wantCode(t, err, codes.InvalidArgument)
}

func TestLeaveGroupOwnerOnlyMember(t *testing.T) {
	g := &fakeGroupsModel{groups: map[string]*model.Groups{"g1": {GroupId: "g1", CreatorAccountId: "owner"}}}
	m := &fakeMembersModel{
		byKey:  map[string]*model.GroupMembers{"g1|owner": member("g1", "owner", model.MemberRoleOwner, model.MemberStatusActive)},
		active: map[string][]*model.GroupMembers{"g1": {member("g1", "owner", model.MemberRoleOwner, model.MemberStatusActive)}},
	}
	_, err := NewLeaveGroupLogic(context.Background(), newSvc(g, m)).LeaveGroup(&groups.LeaveGroupRequest{GroupId: "g1", UserId: "owner"})
	wantCode(t, err, codes.PermissionDenied)
}
