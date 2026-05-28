package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/objectstorage"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/response"
	groupssvc "github.com/wujunhui99/agents_im/internal/servicecontext/groups"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
	usersvc "github.com/wujunhui99/agents_im/internal/servicecontext/user"
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

	serviceContext := groupssvc.NewServiceContextWithAuth(
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

	serviceContext := groupssvc.NewServiceContextWithAuth(
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

func TestMessageFeedbackJSONRouteUsesApiPrefix(t *testing.T) {
	auth := testRouteJWTAuthConfig()
	feedbackRepo := repository.NewMemoryFeedbackRepository()
	serviceContext := messagesvc.NewServiceContextWithFeedback(
		repository.NewMemoryMessageRepository(),
		repository.NewMemoryMediaRepository(),
		feedbackRepo,
		nil,
		nil,
		auth,
	)
	router := newRouteTestMessageRouter(t, serviceContext)

	resp := httptest.NewRecorder()
	req := newRouteTestJSONRequest(http.MethodPost, "/api/feedback", `{"category":"bug","title":"feedback 405","content":"POST must not hit SPA static nginx"}`)
	req.Header.Set("Authorization", routeTestBearerTokenForUser(t, auth, "usr_feedback_route"))
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("feedback status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if contentType := resp.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("content type = %q, want JSON", contentType)
	}

	var envelope struct {
		Code string `json:"code"`
		Data struct {
			FeedbackID string `json:"feedbackId"`
			Status     string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode feedback JSON: %v; body = %s", err, resp.Body.String())
	}
	if envelope.Code != "OK" || envelope.Data.FeedbackID == "" || envelope.Data.Status != "new" {
		t.Fatalf("unexpected feedback envelope: %+v", envelope)
	}
}

func TestUserMediaAvatarRouteRedirectsToObjectStorage(t *testing.T) {
	mediaRepo := repository.NewMemoryMediaRepository()
	store := objectstorage.NewMemoryStore()
	const objectKey = "avatar/route-user/med_route_avatar.png"
	_, err := mediaRepo.CreateMediaObject(context.Background(), model.MediaObject{
		MediaID:          "med_route_avatar",
		OwnerUserID:      "route-user",
		Bucket:           "agents-im-media",
		ObjectKey:        objectKey,
		ContentType:      "image/png",
		SizeBytes:        128,
		OriginalFilename: "avatar.png",
		Purpose:          model.MediaPurposeAvatar,
		Status:           model.MediaStatusReady,
	})
	if err != nil {
		t.Fatalf("seed avatar media: %v", err)
	}
	store.PutObjectInfo(objectstorage.ObjectInfo{ObjectKey: objectKey, ContentType: "image/png", SizeBytes: 128})

	router := newRouteTestUserRouter(t, usersvc.NewServiceContextWithMedia(
		repository.NewMemoryRepository(),
		mediaRepo,
		store,
		"agents-im-media",
		testRouteJWTAuthConfig(),
	))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/media/avatars/med_route_avatar", nil)
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusTemporaryRedirect {
		t.Fatalf("avatar route status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if location := resp.Header().Get("Location"); !strings.Contains(location, url.PathEscape(objectKey)) {
		t.Fatalf("redirect location = %q, want escaped object key %q", location, url.PathEscape(objectKey))
	}
}

func newRouteTestMessageRouter(t *testing.T, serviceContext *messagesvc.ServiceContext) http.Handler {
	t.Helper()

	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
	server := rest.MustNewServer(rest.RestConf{
		ServiceConf: service.ServiceConf{Name: "message-route-test"},
		Host:        "127.0.0.1",
		Port:        8888,
	}, rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	t.Cleanup(server.Stop)
	RegisterMessageGoZeroHandlers(server, serviceContext)

	serverless, err := rest.NewServerless(server)
	if err != nil {
		t.Fatalf("build message route test router: %v", err)
	}
	return http.HandlerFunc(serverless.Serve)
}

func newRouteTestGroupsRouter(t *testing.T, serviceContext *groupssvc.ServiceContext) http.Handler {
	t.Helper()

	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
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

func newRouteTestUserRouter(t *testing.T, serviceContext *usersvc.ServiceContext) http.Handler {
	t.Helper()

	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
	server := rest.MustNewServer(rest.RestConf{
		ServiceConf: service.ServiceConf{Name: "user-route-test"},
		Host:        "127.0.0.1",
		Port:        8888,
	}, rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	t.Cleanup(server.Stop)
	RegisterUserGoZeroHandlers(server, serviceContext)

	serverless, err := rest.NewServerless(server)
	if err != nil {
		t.Fatalf("build user route test router: %v", err)
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
