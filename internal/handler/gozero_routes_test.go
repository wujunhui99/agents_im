package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/response"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func TestGroupsReadRoutesRequireJWTAndMembership(t *testing.T) {
	ctx := context.Background()
	auth := testRouteJWTAuthConfig()
	userLogic := logic.NewUserLogic(repository.NewMemoryRepository())
	creator := mustCreateRouteTestUser(t, userLogic, "route_creator")
	member := mustCreateRouteTestUser(t, userLogic, "route_member")
	outsider := mustCreateRouteTestUser(t, userLogic, "route_outsider")

	serviceContext := svc.NewGroupsServiceContextWithAuth(
		repository.NewMemoryGroupsRepository(),
		logic.NewUserLogicExistenceChecker(userLogic),
		auth,
	)
	group, err := serviceContext.GroupsLogic.CreateGroup(ctx, logic.CreateGroupRequest{
		CreatorUserID: creator.UserID,
		Name:          "Route ACL",
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := serviceContext.GroupsLogic.JoinGroup(ctx, logic.JoinGroupRequest{
		GroupID: group.GroupID,
		UserID:  member.UserID,
	}); err != nil {
		t.Fatalf("join member: %v", err)
	}

	router := newRouteTestGroupsRouter(t, serviceContext)

	for _, target := range []string{
		"/groups/" + group.GroupID,
		"/groups/" + group.GroupID + "/members",
	} {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, target, nil)
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s status = %d, body = %s", target, resp.Code, resp.Body.String())
		}

		resp = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, target, nil)
		req.Header.Set("Authorization", routeTestBearerTokenForUser(t, auth, outsider.UserID))
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusForbidden {
			t.Fatalf("outsider %s status = %d, body = %s", target, resp.Code, resp.Body.String())
		}

		resp = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, target, nil)
		req.Header.Set("Authorization", routeTestBearerTokenForUser(t, auth, member.UserID))
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("member %s status = %d, body = %s", target, resp.Code, resp.Body.String())
		}
	}
}

func TestGroupsAddMemberRouteRequiresOwnerForOtherUsers(t *testing.T) {
	ctx := context.Background()
	auth := testRouteJWTAuthConfig()
	userLogic := logic.NewUserLogic(repository.NewMemoryRepository())
	creator := mustCreateRouteTestUser(t, userLogic, "route_add_creator")
	member := mustCreateRouteTestUser(t, userLogic, "route_add_member")
	invitee := mustCreateRouteTestUser(t, userLogic, "route_add_invitee")

	serviceContext := svc.NewGroupsServiceContextWithAuth(
		repository.NewMemoryGroupsRepository(),
		logic.NewUserLogicExistenceChecker(userLogic),
		auth,
	)
	group, err := serviceContext.GroupsLogic.CreateGroup(ctx, logic.CreateGroupRequest{
		CreatorUserID: creator.UserID,
		Name:          "Route Add ACL",
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := serviceContext.GroupsLogic.JoinGroup(ctx, logic.JoinGroupRequest{
		GroupID: group.GroupID,
		UserID:  member.UserID,
	}); err != nil {
		t.Fatalf("join member: %v", err)
	}

	router := newRouteTestGroupsRouter(t, serviceContext)
	target := "/groups/" + group.GroupID + "/members"
	body := `{"user_id":"` + invitee.UserID + `"}`

	memberResp := httptest.NewRecorder()
	memberReq := newRouteTestJSONRequest(http.MethodPost, target, body)
	memberReq.Header.Set("Authorization", routeTestBearerTokenForUser(t, auth, member.UserID))
	router.ServeHTTP(memberResp, memberReq)
	if memberResp.Code != http.StatusForbidden {
		t.Fatalf("member add status = %d, body = %s", memberResp.Code, memberResp.Body.String())
	}

	ownerResp := httptest.NewRecorder()
	ownerReq := newRouteTestJSONRequest(http.MethodPost, target, body)
	ownerReq.Header.Set("Authorization", routeTestBearerTokenForUser(t, auth, creator.UserID))
	router.ServeHTTP(ownerResp, ownerReq)
	if ownerResp.Code != http.StatusOK {
		t.Fatalf("owner add status = %d, body = %s", ownerResp.Code, ownerResp.Body.String())
	}
}

func newRouteTestGroupsRouter(t *testing.T, serviceContext *svc.ServiceContext) http.Handler {
	t.Helper()

	httpx.SetErrorHandler(response.GoZeroErrorHandler)
	server := rest.MustNewServer(rest.RestConf{
		ServiceConf: service.ServiceConf{Name: "groups-route-test"},
		Host:        "127.0.0.1",
		Port:        8888,
	}, rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	t.Cleanup(server.Stop)
	RegisterGroupsGoZeroHandlers(server, serviceContext)

	serverless, err := rest.NewServerless(server)
	if err != nil {
		t.Fatalf("build go-zero test router: %v", err)
	}
	return http.HandlerFunc(serverless.Serve)
}

func mustCreateRouteTestUser(t *testing.T, userLogic *logic.UserLogic, identifier string) logic.UserProfile {
	t.Helper()

	user, err := userLogic.CreateUser(context.Background(), logic.CreateUserRequest{Identifier: identifier})
	if err != nil {
		t.Fatalf("create user %q: %v", identifier, err)
	}
	return user
}

func testRouteJWTAuthConfig() config.JWTAuthConfig {
	return config.JWTAuthConfig{
		AccessSecret: "route-test-jwt-secret-change-me",
		AccessExpire: 3600,
	}
}

func routeTestBearerTokenForUser(t *testing.T, auth config.JWTAuthConfig, userID string) string {
	t.Helper()

	manager := token.NewHMACTokenManager(auth.AccessSecret, time.Duration(auth.AccessExpire)*time.Second)
	rawToken, _, err := manager.Issue(userID, userID)
	if err != nil {
		t.Fatalf("issue test jwt: %v", err)
	}
	return "Bearer " + rawToken
}

func newRouteTestJSONRequest(method string, target string, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}
