package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	agententry "github.com/wujunhui99/agents_im/service/agent/api/entry"
)

func TestAgentLogicCreateRequiresAgentAccountType(t *testing.T) {
	ctx := context.Background()
	agentRepo := repository.NewMemoryAgentRepository()
	agentLogic := logic.NewAgentLogic(agentRepo, testAccountTypeChecker{
		accountTypes: map[string]string{
			"usr_agent": logic.AccountTypeAgent,
			"usr_user":  string(model.AccountTypeUser),
		},
	})

	created, err := agentLogic.CreateAgent(ctx, logic.CreateAgentRequest{
		IMUserID:    "usr_agent",
		Name:        "Support Bot",
		Description: "handles support triage",
		CreatedBy:   "usr_admin",
	})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if created.AgentID == "" || created.IMUserID != "usr_agent" || created.Status != logic.AgentStatusDisabled {
		t.Fatalf("unexpected created agent: %+v", created)
	}
	assertNumericSnowflakeID(t, created.AgentID)

	_, err = agentLogic.CreateAgent(ctx, logic.CreateAgentRequest{
		IMUserID:  "usr_agent",
		Name:      "Duplicate Bot",
		CreatedBy: "usr_admin",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeAlreadyExists {
		t.Fatalf("duplicate im_user_id error = %v, want ALREADY_EXISTS", err)
	}

	_, err = agentLogic.CreateAgent(ctx, logic.CreateAgentRequest{
		IMUserID:  "usr_user",
		Name:      "Wrong Type Bot",
		CreatedBy: "usr_admin",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("user-type account binding error = %v, want FORBIDDEN", err)
	}

	_, err = agentLogic.CreateAgent(ctx, logic.CreateAgentRequest{
		IMUserID:  "usr_missing",
		Name:      "Missing User Bot",
		CreatedBy: "usr_admin",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing user binding error = %v, want NOT_FOUND", err)
	}
}

func TestAgentLogicFailClosedWithoutAccountTypeChecker(t *testing.T) {
	ctx := context.Background()
	agentRepo := repository.NewMemoryAgentRepository()
	agentLogic := logic.NewAgentLogic(agentRepo, nil)

	_, err := agentLogic.CreateAgent(ctx, logic.CreateAgentRequest{
		IMUserID:  "usr_agent",
		Name:      "Support Bot",
		CreatedBy: "usr_admin",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeInternal {
		t.Fatalf("missing account type checker error = %v, want INTERNAL", err)
	}

	listed, listErr := agentLogic.ListAgents(ctx, logic.ListAgentsRequest{})
	if listErr != nil {
		t.Fatalf("list after failed create: %v", listErr)
	}
	if len(listed.Agents) != 0 {
		t.Fatalf("agent should not be created when checker is unavailable: %+v", listed.Agents)
	}
}

func TestAgentLogicUsesUserLogicAccountTypeChecker(t *testing.T) {
	ctx := context.Background()
	userRepo := repository.NewMemoryRepository()
	userLogic := logic.NewUserLogic(userRepo)
	agentUser, err := userLogic.CreateUser(ctx, logic.CreateUserRequest{
		Identifier:  "agentuser",
		DisplayName: "Agent User",
		AccountType: logic.AccountTypeAgent,
	})
	if err != nil {
		t.Fatalf("create agent user: %v", err)
	}
	assertNumericSnowflakeID(t, agentUser.UserID)
	userTypeAccount, err := userLogic.CreateUser(ctx, logic.CreateUserRequest{
		Identifier:  "humantypeuser",
		DisplayName: "Human Type User",
	})
	if err != nil {
		t.Fatalf("create user-type account: %v", err)
	}
	assertNumericSnowflakeID(t, userTypeAccount.UserID)

	agentLogic := logic.NewAgentLogic(
		repository.NewMemoryAgentRepository(),
		logic.NewUserLogicAccountTypeChecker(userLogic),
	)
	agent, err := agentLogic.CreateAgent(ctx, logic.CreateAgentRequest{
		IMUserID:  agentUser.UserID,
		Name:      "Real Checker Bot",
		CreatedBy: "usr_admin",
	})
	if err != nil {
		t.Fatalf("create agent with real account type checker: %v", err)
	}
	assertNumericSnowflakeID(t, agent.AgentID)
	if agent.IMUserID != agentUser.UserID {
		t.Fatalf("agent im_user_id = %q, want account id %q", agent.IMUserID, agentUser.UserID)
	}
	if _, err := agentLogic.CreateAgent(ctx, logic.CreateAgentRequest{
		IMUserID:  userTypeAccount.UserID,
		Name:      "Wrong Type Bot",
		CreatedBy: "usr_admin",
	}); err == nil || apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("user-type account binding error = %v, want FORBIDDEN", err)
	}
}

func TestAgentLogicUpdateListStatusAndArchive(t *testing.T) {
	ctx := context.Background()
	agentRepo := repository.NewMemoryAgentRepository()
	agentLogic := logic.NewAgentLogic(agentRepo, testAccountTypeChecker{
		accountTypes: map[string]string{
			"usr_agent_one": logic.AccountTypeAgent,
			"usr_agent_two": logic.AccountTypeAgent,
		},
	})

	first := mustCreateAgent(t, agentLogic, "usr_agent_one", "First Bot")
	second := mustCreateAgent(t, agentLogic, "usr_agent_two", "Second Bot")

	updated, err := agentLogic.UpdateAgent(ctx, logic.UpdateAgentRequest{
		AgentID:     first.AgentID,
		Name:        ptr("Renamed Bot"),
		Description: ptr("updated description"),
	})
	if err != nil {
		t.Fatalf("update agent: %v", err)
	}
	if updated.Name != "Renamed Bot" || updated.Description != "updated description" {
		t.Fatalf("unexpected updated agent: %+v", updated)
	}

	active, err := agentLogic.UpdateAgentStatus(ctx, logic.UpdateAgentStatusRequest{
		AgentID: second.AgentID,
		Status:  logic.AgentStatusActive,
	})
	if err != nil {
		t.Fatalf("activate agent: %v", err)
	}
	if active.Status != logic.AgentStatusActive {
		t.Fatalf("agent was not activated: %+v", active)
	}

	activeList, err := agentLogic.ListAgents(ctx, logic.ListAgentsRequest{Status: logic.AgentStatusActive})
	if err != nil {
		t.Fatalf("list active agents: %v", err)
	}
	if len(activeList.Agents) != 1 || activeList.Agents[0].AgentID != second.AgentID {
		t.Fatalf("unexpected active list: %+v", activeList.Agents)
	}

	_, err = agentLogic.UpdateAgentStatus(ctx, logic.UpdateAgentStatusRequest{
		AgentID: first.AgentID,
		Status:  "running",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("invalid status error = %v, want INVALID_ARGUMENT", err)
	}

	archived, err := agentLogic.ArchiveAgent(ctx, logic.AgentPathRequest{AgentID: first.AgentID})
	if err != nil {
		t.Fatalf("archive agent: %v", err)
	}
	if archived.Status != logic.AgentStatusArchived {
		t.Fatalf("agent was not archived: %+v", archived)
	}
}

func TestAgentHTTPHandlers(t *testing.T) {
	serviceContext := agententry.NewServiceContextWithAuth(
		repository.NewMemoryAgentRepository(),
		testAccountTypeChecker{accountTypes: map[string]string{
			"usr_agent": logic.AccountTypeAgent,
			"usr_user":  string(model.AccountTypeUser),
		}},
		testJWTAuthConfig(),
	)
	mux := newAgentAPIServiceRouter(t, serviceContext)
	adminBearer := bearerTokenForUser(t, "usr_admin")

	missingTokenResp := httptest.NewRecorder()
	missingTokenReq := newJSONRequest(http.MethodPost, "/agents", `{"im_user_id":"usr_agent","name":"Support Bot"}`)
	mux.ServeHTTP(missingTokenResp, missingTokenReq)
	if missingTokenResp.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d", missingTokenResp.Code)
	}

	createResp := httptest.NewRecorder()
	createReq := newJSONRequest(http.MethodPost, "/agents", `{"im_user_id":"usr_agent","name":"Support Bot","description":"support"}`)
	createReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create agent status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	assertNoSecretFields(t, createResp.Body.String())

	var created envelope[logic.AgentInfo]
	decodeEnvelope(t, createResp.Body.Bytes(), &created)
	if created.Data.AgentID == "" || created.Data.CreatedBy != "usr_admin" {
		t.Fatalf("unexpected created agent: %+v", created.Data)
	}

	getResp := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/agents/"+created.Data.AgentID, nil)
	getReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get agent status = %d, body = %s", getResp.Code, getResp.Body.String())
	}

	updateResp := httptest.NewRecorder()
	updateReq := newJSONRequest(http.MethodPatch, "/agents/"+created.Data.AgentID, `{"name":"Renamed Bot"}`)
	updateReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(updateResp, updateReq)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update agent status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	statusResp := httptest.NewRecorder()
	statusReq := newJSONRequest(http.MethodPatch, "/agents/"+created.Data.AgentID+"/status", `{"status":"active"}`)
	statusReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(statusResp, statusReq)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status update = %d, body = %s", statusResp.Code, statusResp.Body.String())
	}
	var activated envelope[logic.AgentInfo]
	decodeEnvelope(t, statusResp.Body.Bytes(), &activated)
	if activated.Data.Status != logic.AgentStatusActive {
		t.Fatalf("unexpected activated agent: %+v", activated.Data)
	}

	listResp := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/agents?status=active", nil)
	listReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	var listed envelope[logic.ListAgentsResponse]
	decodeEnvelope(t, listResp.Body.Bytes(), &listed)
	if len(listed.Data.Agents) != 1 || listed.Data.Agents[0].AgentID != created.Data.AgentID {
		t.Fatalf("unexpected list response: %+v", listed.Data.Agents)
	}

	forbiddenResp := httptest.NewRecorder()
	forbiddenReq := newJSONRequest(http.MethodPost, "/agents", `{"im_user_id":"usr_user","name":"Wrong Type Bot"}`)
	forbiddenReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(forbiddenResp, forbiddenReq)
	if forbiddenResp.Code != http.StatusForbidden {
		t.Fatalf("user-type account binding status = %d, body = %s", forbiddenResp.Code, forbiddenResp.Body.String())
	}

	deleteResp := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/agents/"+created.Data.AgentID, nil)
	deleteReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete/archive status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}
	var archived envelope[logic.AgentInfo]
	decodeEnvelope(t, deleteResp.Body.Bytes(), &archived)
	if archived.Data.Status != logic.AgentStatusArchived {
		t.Fatalf("delete should archive agent: %+v", archived.Data)
	}
}

func mustCreateAgent(t *testing.T, agentLogic *logic.AgentLogic, imUserID string, name string) logic.AgentInfo {
	t.Helper()

	agent, err := agentLogic.CreateAgent(context.Background(), logic.CreateAgentRequest{
		IMUserID:  imUserID,
		Name:      name,
		CreatedBy: "usr_admin",
	})
	if err != nil {
		t.Fatalf("create agent %q: %v", imUserID, err)
	}
	return agent
}

func ptr(value string) *string {
	return &value
}

type testAccountTypeChecker struct {
	accountTypes map[string]string
}

func (c testAccountTypeChecker) EnsureUserAccountType(_ context.Context, userID string, accountType string) error {
	actual, exists := c.accountTypes[userID]
	if !exists {
		return apperror.NotFound("user not found")
	}
	if actual != accountType {
		return apperror.Forbidden("im_user_id must reference account_type=agent")
	}
	return nil
}
