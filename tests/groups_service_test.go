package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/svc"
)

func TestGroupsLogicCreateJoinRepeatLeaveAndList(t *testing.T) {
	ctx := context.Background()
	userLogic := logic.NewUserLogic(repository.NewMemoryRepository())
	creator := mustCreateUser(t, userLogic, "creator_001")
	member := mustCreateUser(t, userLogic, "member_001")
	groupsLogic := logic.NewGroupsLogic(
		repository.NewMemoryGroupsRepository(),
		logic.NewUserLogicExistenceChecker(userLogic),
	)

	group, err := groupsLogic.CreateGroup(ctx, logic.CreateGroupRequest{
		CreatorUserID: creator.UserID,
		Name:          "Project Alpha",
		Description:   "first group",
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if group.GroupID == "" || group.CreatorUserID != creator.UserID {
		t.Fatalf("unexpected group: %+v", group)
	}

	members, err := groupsLogic.ListMembers(ctx, logic.ListMembersRequest{GroupID: group.GroupID})
	if err != nil {
		t.Fatalf("list members after create: %v", err)
	}
	if len(members.Members) != 1 || members.Members[0].UserID != creator.UserID {
		t.Fatalf("creator should be the first member: %+v", members.Members)
	}

	joined, err := groupsLogic.JoinGroup(ctx, logic.JoinGroupRequest{
		GroupID: group.GroupID,
		UserID:  member.UserID,
	})
	if err != nil {
		t.Fatalf("join group: %v", err)
	}
	if joined.AlreadyMember || joined.Member.UserID != member.UserID || joined.Member.State != model.MemberStateActive {
		t.Fatalf("unexpected join response: %+v", joined)
	}

	repeated, err := groupsLogic.JoinGroup(ctx, logic.JoinGroupRequest{
		GroupID: group.GroupID,
		UserID:  member.UserID,
	})
	if err != nil {
		t.Fatalf("repeat join group: %v", err)
	}
	if !repeated.AlreadyMember {
		t.Fatalf("repeat join should be idempotent: %+v", repeated)
	}

	members, err = groupsLogic.ListMembers(ctx, logic.ListMembersRequest{GroupID: group.GroupID})
	if err != nil {
		t.Fatalf("list members after join: %v", err)
	}
	if len(members.Members) != 2 {
		t.Fatalf("members should not contain duplicates: %+v", members.Members)
	}

	left, err := groupsLogic.LeaveGroup(ctx, logic.LeaveGroupRequest{
		GroupID: group.GroupID,
		UserID:  member.UserID,
	})
	if err != nil {
		t.Fatalf("leave group: %v", err)
	}
	if left.Member.State != model.MemberStateLeft || left.Member.LeftAt == "" {
		t.Fatalf("unexpected leave response: %+v", left)
	}

	members, err = groupsLogic.ListMembers(ctx, logic.ListMembersRequest{GroupID: group.GroupID})
	if err != nil {
		t.Fatalf("list members after leave: %v", err)
	}
	if len(members.Members) != 1 || members.Members[0].UserID != creator.UserID {
		t.Fatalf("left member should not be listed: %+v", members.Members)
	}
}

func TestGroupsLogicOwnerCannotLeaveWhenOnlyActiveMember(t *testing.T) {
	ctx := context.Background()
	userLogic := logic.NewUserLogic(repository.NewMemoryRepository())
	creator := mustCreateUser(t, userLogic, "creator_only_owner")
	groupsLogic := logic.NewGroupsLogic(
		repository.NewMemoryGroupsRepository(),
		logic.NewUserLogicExistenceChecker(userLogic),
	)

	group, err := groupsLogic.CreateGroup(ctx, logic.CreateGroupRequest{
		CreatorUserID: creator.UserID,
		Name:          "Owner Only",
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	_, err = groupsLogic.LeaveGroup(ctx, logic.LeaveGroupRequest{
		GroupID: group.GroupID,
		UserID:  creator.UserID,
	})
	if err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("owner-only leave error = %v, want FORBIDDEN", err)
	}

	members, err := groupsLogic.ListMembers(ctx, logic.ListMembersRequest{GroupID: group.GroupID})
	if err != nil {
		t.Fatalf("list members after rejected leave: %v", err)
	}
	if len(members.Members) != 1 || members.Members[0].UserID != creator.UserID {
		t.Fatalf("owner should remain active after rejected leave: %+v", members.Members)
	}
}

func TestGroupsLogicNotFoundPaths(t *testing.T) {
	ctx := context.Background()
	userLogic := logic.NewUserLogic(repository.NewMemoryRepository())
	creator := mustCreateUser(t, userLogic, "creator_002")
	groupsLogic := logic.NewGroupsLogic(
		repository.NewMemoryGroupsRepository(),
		logic.NewUserLogicExistenceChecker(userLogic),
	)

	_, err := groupsLogic.GetGroup(ctx, logic.GetGroupRequest{GroupID: "grp_missing"})
	if err == nil || apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing group error = %v, want NOT_FOUND", err)
	}

	_, err = groupsLogic.CreateGroup(ctx, logic.CreateGroupRequest{
		CreatorUserID: "usr_missing",
		Name:          "Missing Creator Group",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing creator error = %v, want NOT_FOUND", err)
	}

	group, err := groupsLogic.CreateGroup(ctx, logic.CreateGroupRequest{
		CreatorUserID: creator.UserID,
		Name:          "Existing Group",
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	_, err = groupsLogic.JoinGroup(ctx, logic.JoinGroupRequest{
		GroupID: "grp_missing",
		UserID:  creator.UserID,
	})
	if err == nil || apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("join missing group error = %v, want NOT_FOUND", err)
	}

	_, err = groupsLogic.JoinGroup(ctx, logic.JoinGroupRequest{
		GroupID: group.GroupID,
		UserID:  "usr_missing",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("join missing user error = %v, want NOT_FOUND", err)
	}
}

func TestGroupsHTTPHandlers(t *testing.T) {
	userLogic := logic.NewUserLogic(repository.NewMemoryRepository())
	creator := mustCreateUser(t, userLogic, "creator_003")
	member := mustCreateUser(t, userLogic, "member_003")
	serviceContext := svc.NewGroupsServiceContextWithAuth(
		repository.NewMemoryGroupsRepository(),
		logic.NewUserLogicExistenceChecker(userLogic),
		testJWTAuthConfig(),
	)
	mux := newGroupsGoZeroRouter(t, serviceContext)
	creatorBearer := bearerTokenForUser(t, creator.UserID)
	memberBearer := bearerTokenForUser(t, member.UserID)

	t.Run("rejects legacy X-User-Id header without bearer token", func(t *testing.T) {
		bypassResp := httptest.NewRecorder()
		bypassReq := newJSONRequest(http.MethodPost, "/groups", `{"name":"Header Only"}`)
		setRejectedLegacyXUserIDHeader(t, bypassReq, creator.UserID)
		mux.ServeHTTP(bypassResp, bypassReq)
		if bypassResp.Code != http.StatusUnauthorized {
			t.Fatalf("legacy X-User-Id rejection status = %d", bypassResp.Code)
		}
	})

	createResp := httptest.NewRecorder()
	createReq := newJSONRequest(http.MethodPost, "/groups", `{"name":"Team Chat","description":"team room"}`)
	createReq.Header.Set("Authorization", creatorBearer)
	mux.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create group status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	assertNoSecretFields(t, createResp.Body.String())

	var created envelope[logic.GroupInfo]
	decodeEnvelope(t, createResp.Body.Bytes(), &created)
	if created.Data.GroupID == "" || created.Data.CreatorUserID != creator.UserID {
		t.Fatalf("unexpected created group: %+v", created.Data)
	}

	getResp := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/groups/"+created.Data.GroupID, nil)
	mux.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get group status = %d, body = %s", getResp.Code, getResp.Body.String())
	}

	addResp := httptest.NewRecorder()
	addReq := newJSONRequest(http.MethodPost, "/groups/"+created.Data.GroupID+"/members", `{"user_id":"`+member.UserID+`"}`)
	addReq.Header.Set("Authorization", creatorBearer)
	mux.ServeHTTP(addResp, addReq)
	if addResp.Code != http.StatusOK {
		t.Fatalf("add member status = %d, body = %s", addResp.Code, addResp.Body.String())
	}
	var added envelope[logic.MemberResponse]
	decodeEnvelope(t, addResp.Body.Bytes(), &added)
	if added.Data.AlreadyMember || added.Data.Member.UserID != member.UserID {
		t.Fatalf("unexpected add response: %+v", added.Data)
	}

	repeatResp := httptest.NewRecorder()
	repeatReq := newJSONRequest(http.MethodPost, "/groups/"+created.Data.GroupID+"/members", `{"user_id":"`+member.UserID+`"}`)
	repeatReq.Header.Set("Authorization", creatorBearer)
	mux.ServeHTTP(repeatResp, repeatReq)
	if repeatResp.Code != http.StatusOK {
		t.Fatalf("repeat add status = %d, body = %s", repeatResp.Code, repeatResp.Body.String())
	}
	var repeated envelope[logic.MemberResponse]
	decodeEnvelope(t, repeatResp.Body.Bytes(), &repeated)
	if !repeated.Data.AlreadyMember {
		t.Fatalf("repeat add should report already_member: %+v", repeated.Data)
	}

	listResp := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/groups/"+created.Data.GroupID+"/members", nil)
	mux.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	var listed envelope[logic.ListMembersResponse]
	decodeEnvelope(t, listResp.Body.Bytes(), &listed)
	if len(listed.Data.Members) != 2 {
		t.Fatalf("unexpected member list: %+v", listed.Data.Members)
	}

	leaveResp := httptest.NewRecorder()
	leaveReq := httptest.NewRequest(http.MethodDelete, "/groups/"+created.Data.GroupID+"/members/me", nil)
	leaveReq.Header.Set("Authorization", memberBearer)
	mux.ServeHTTP(leaveResp, leaveReq)
	if leaveResp.Code != http.StatusOK {
		t.Fatalf("leave status = %d, body = %s", leaveResp.Code, leaveResp.Body.String())
	}

	listAfterLeaveResp := httptest.NewRecorder()
	listAfterLeaveReq := httptest.NewRequest(http.MethodGet, "/groups/"+created.Data.GroupID+"/members", nil)
	mux.ServeHTTP(listAfterLeaveResp, listAfterLeaveReq)
	var listedAfterLeave envelope[logic.ListMembersResponse]
	decodeEnvelope(t, listAfterLeaveResp.Body.Bytes(), &listedAfterLeave)
	if len(listedAfterLeave.Data.Members) != 1 || listedAfterLeave.Data.Members[0].UserID != creator.UserID {
		t.Fatalf("left member should be filtered: %+v", listedAfterLeave.Data.Members)
	}

	missingUserResp := httptest.NewRecorder()
	missingUserReq := newJSONRequest(http.MethodPost, "/groups/"+created.Data.GroupID+"/members", `{"user_id":"usr_missing"}`)
	missingUserReq.Header.Set("Authorization", creatorBearer)
	mux.ServeHTTP(missingUserResp, missingUserReq)
	if missingUserResp.Code != http.StatusNotFound {
		t.Fatalf("missing user status = %d, body = %s", missingUserResp.Code, missingUserResp.Body.String())
	}

	missingGroupResp := httptest.NewRecorder()
	missingGroupReq := httptest.NewRequest(http.MethodGet, "/groups/grp_missing", nil)
	mux.ServeHTTP(missingGroupResp, missingGroupReq)
	if missingGroupResp.Code != http.StatusNotFound {
		t.Fatalf("missing group status = %d, body = %s", missingGroupResp.Code, missingGroupResp.Body.String())
	}
}

func mustCreateUser(t *testing.T, userLogic *logic.UserLogic, identifier string) logic.UserProfile {
	t.Helper()

	user, err := userLogic.CreateUser(context.Background(), logic.CreateUserRequest{Identifier: identifier})
	if err != nil {
		t.Fatalf("create user %q: %v", identifier, err)
	}
	return user
}
