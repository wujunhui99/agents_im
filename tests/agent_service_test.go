package tests

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
)

// The agent API HTTP handler test lives in service/agent/api (package main),
// because agent wiring is under service/agent/api/internal/* which the external
// tests package cannot import. These tests cover the agent business logic.

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
