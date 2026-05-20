package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestAgentCreateToolCreatesDefinitionAndFriendship(t *testing.T) {
	ctx := context.Background()
	accounts := repository.NewMemoryRepository()
	agents := repository.NewMemoryAgentRepository()
	registry := repository.NewMemoryAgentRegistryRepository()
	assembly := NewAgentAssemblyLogic(AgentAssemblyDependencies{
		Accounts:    accounts,
		Friendships: accounts,
		Agents:      agents,
		Registry:    registry,
	})
	userLogic := NewUserLogic(accounts)
	requester, err := userLogic.CreateUser(ctx, CreateUserRequest{
		Identifier:  "agent_requester",
		DisplayName: "Agent Requester",
	})
	if err != nil {
		t.Fatal(err)
	}
	contextTool := mustRegisterDefinitionTool(t, ctx, registry, model.AgentTool{
		ToolID:          "tool_context",
		Name:            model.LocalToolHandlerGetConversationContext,
		ToolType:        model.AgentToolTypeLocal,
		LocalHandlerKey: model.LocalToolHandlerGetConversationContext,
	})

	created, err := assembly.CreateAgentFromTool(ctx, AgentCreateToolRequest{
		RequestingUserID: requester.UserID,
		Identifier:       "research_agent",
		Name:             "Research Agent",
		Description:      "Summarizes notes",
		SystemPrompt:     "You summarize bounded research notes.",
		ToolNames:        []string{model.LocalToolHandlerGetConversationContext},
	})
	if err != nil {
		t.Fatalf("create agent from tool: %v", err)
	}
	if created.AgentID == "" || created.AccountID == "" || created.Identifier != "research_agent" {
		t.Fatalf("created response missing identifiers: %+v", created)
	}

	account, err := accounts.GetByID(ctx, created.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	if account.AccountType != model.AccountTypeAgent || account.DisplayName != "Research Agent" {
		t.Fatalf("created account = %+v, want agent profile", account)
	}
	agent, err := agents.GetAgent(ctx, created.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if agent.AccountID != created.AccountID || agent.Status != model.AgentStatusActive {
		t.Fatalf("created agent = %+v, want active account-backed agent", agent)
	}
	definition, err := assembly.GetAgentDefinition(ctx, AgentDefinitionRequest{AgentID: created.AgentID, RequestedBy: requester.UserID})
	if err != nil {
		t.Fatalf("get created definition: %v", err)
	}
	if definition.SystemPrompt.Content != "You summarize bounded research notes." {
		t.Fatalf("system prompt = %q", definition.SystemPrompt.Content)
	}
	if len(definition.Tools) != 1 || definition.Tools[0].ToolID != contextTool.ToolID {
		t.Fatalf("definition tools = %+v, want context tool", definition.Tools)
	}
	assertAcceptedDefinitionFriendship(t, ctx, accounts, requester.UserID, created.AccountID)
	assertAcceptedDefinitionFriendship(t, ctx, accounts, created.AccountID, requester.UserID)
}

func TestAgentCreateToolRejectsHighRiskToolsBeforeCreatingAccount(t *testing.T) {
	ctx := context.Background()
	accounts := repository.NewMemoryRepository()
	agents := repository.NewMemoryAgentRepository()
	registry := repository.NewMemoryAgentRegistryRepository()
	assembly := NewAgentAssemblyLogic(AgentAssemblyDependencies{
		Accounts:    accounts,
		Friendships: accounts,
		Agents:      agents,
		Registry:    registry,
	})
	userLogic := NewUserLogic(accounts)
	requester, err := userLogic.CreateUser(ctx, CreateUserRequest{
		Identifier:  "agent_risk_requester",
		DisplayName: "Risk Requester",
	})
	if err != nil {
		t.Fatal(err)
	}
	mustRegisterDefinitionTool(t, ctx, registry, model.AgentTool{
		ToolID:          "tool_python",
		Name:            model.LocalToolHandlerPythonExecute,
		ToolType:        model.AgentToolTypeLocal,
		LocalHandlerKey: model.LocalToolHandlerPythonExecute,
	})
	mustRegisterDefinitionTool(t, ctx, registry, model.AgentTool{
		ToolID:          "tool_send",
		Name:            model.LocalToolHandlerSendAgentMessage,
		ToolType:        model.AgentToolTypeLocal,
		LocalHandlerKey: model.LocalToolHandlerSendAgentMessage,
	})

	for _, toolName := range []string{model.LocalToolHandlerPythonExecute, model.LocalToolHandlerSendAgentMessage} {
		_, err := assembly.CreateAgentFromTool(ctx, AgentCreateToolRequest{
			RequestingUserID: requester.UserID,
			Identifier:       "blocked_" + toolNameToIdentifierSuffix(toolName),
			Name:             "Blocked Agent",
			SystemPrompt:     "Should not be created.",
			ToolNames:        []string{toolName},
		})
		assertAppErrorCode(t, err, apperror.CodeForbidden)
	}
	agentAccounts, err := accounts.ListByAccountType(ctx, model.AccountTypeAgent)
	if err != nil {
		t.Fatal(err)
	}
	if len(agentAccounts) != 0 {
		t.Fatalf("high-risk tool request created agent accounts: %+v", agentAccounts)
	}
}

func TestAgentDefinitionUpdateReplacesPromptAndTools(t *testing.T) {
	ctx := context.Background()
	accounts := repository.NewMemoryRepository()
	agents := repository.NewMemoryAgentRepository()
	registry := repository.NewMemoryAgentRegistryRepository()
	assembly := NewAgentAssemblyLogic(AgentAssemblyDependencies{
		Accounts: accounts,
		Agents:   agents,
		Registry: registry,
	})
	userLogic := NewUserLogic(accounts)
	agentAccount, err := userLogic.CreateUser(ctx, CreateUserRequest{
		Identifier:  "definition_agent",
		DisplayName: "Definition Agent",
		AccountType: string(model.AccountTypeAgent),
	})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := userLogic.CreateUser(ctx, CreateUserRequest{
		Identifier:  "definition_owner",
		DisplayName: "Definition Owner",
	})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := agents.CreateAgent(ctx, model.Agent{
		AccountID:   agentAccount.UserID,
		Name:        "Definition Agent",
		Description: "Initial",
		Status:      model.AgentStatusActive,
		CreatedBy:   owner.UserID,
	})
	if err != nil {
		t.Fatal(err)
	}
	contextTool := mustRegisterDefinitionTool(t, ctx, registry, model.AgentTool{
		ToolID:          "tool_definition_context",
		Name:            model.LocalToolHandlerGetConversationContext,
		ToolType:        model.AgentToolTypeLocal,
		LocalHandlerKey: model.LocalToolHandlerGetConversationContext,
	})
	pythonTool := mustRegisterDefinitionTool(t, ctx, registry, model.AgentTool{
		ToolID:          "tool_definition_python",
		Name:            model.LocalToolHandlerPythonExecute,
		ToolType:        model.AgentToolTypeLocal,
		LocalHandlerKey: model.LocalToolHandlerPythonExecute,
	})

	definition, err := assembly.UpdateAgentDefinition(ctx, UpdateAgentDefinitionRequest{
		AgentID:      agent.AgentID,
		SystemPrompt: "Use the updated system prompt.",
		ToolNames:    []string{contextTool.Name, pythonTool.Name},
		UpdatedBy:    owner.UserID,
	})
	if err != nil {
		t.Fatalf("update definition: %v", err)
	}
	if definition.SystemPrompt.Content != "Use the updated system prompt." {
		t.Fatalf("updated prompt = %q", definition.SystemPrompt.Content)
	}
	if len(definition.Tools) != 2 {
		t.Fatalf("updated tools = %+v, want two", definition.Tools)
	}

	definition, err = assembly.UpdateAgentDefinition(ctx, UpdateAgentDefinitionRequest{
		AgentID:      agent.AgentID,
		SystemPrompt: "Use the replacement prompt.",
		ToolNames:    []string{contextTool.Name},
		UpdatedBy:    owner.UserID,
	})
	if err != nil {
		t.Fatalf("replace definition: %v", err)
	}
	if definition.SystemPrompt.Content != "Use the replacement prompt." {
		t.Fatalf("replacement prompt = %q", definition.SystemPrompt.Content)
	}
	if len(definition.Tools) != 1 || definition.Tools[0].ToolID != contextTool.ToolID {
		t.Fatalf("replacement tools = %+v, want only context tool", definition.Tools)
	}
	promptBindings, err := registry.ListPromptBindings(ctx, agent.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(promptBindings) != 1 || promptBindings[0].PromptID != definition.SystemPrompt.PromptID {
		t.Fatalf("prompt bindings = %+v, want only active replacement", promptBindings)
	}
}

func TestAgentDefinitionUpdateRejectsNonOwner(t *testing.T) {
	ctx := context.Background()
	accounts := repository.NewMemoryRepository()
	agents := repository.NewMemoryAgentRepository()
	registry := repository.NewMemoryAgentRegistryRepository()
	assembly := NewAgentAssemblyLogic(AgentAssemblyDependencies{
		Accounts: accounts,
		Agents:   agents,
		Registry: registry,
	})
	userLogic := NewUserLogic(accounts)
	owner, err := userLogic.CreateUser(ctx, CreateUserRequest{Identifier: "acl_owner", DisplayName: "ACL Owner"})
	if err != nil {
		t.Fatal(err)
	}
	intruder, err := userLogic.CreateUser(ctx, CreateUserRequest{Identifier: "acl_intruder", DisplayName: "ACL Intruder"})
	if err != nil {
		t.Fatal(err)
	}
	agentAccount, err := userLogic.CreateUser(ctx, CreateUserRequest{
		Identifier:  "acl_agent",
		DisplayName: "ACL Agent",
		AccountType: string(model.AccountTypeAgent),
	})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := agents.CreateAgent(ctx, model.Agent{
		AccountID: agentAccount.UserID,
		Name:      "ACL Agent",
		Status:    model.AgentStatusActive,
		CreatedBy: owner.UserID,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = assembly.GetAgentDefinition(ctx, AgentDefinitionRequest{AgentID: agent.AgentID, RequestedBy: intruder.UserID})
	assertAppErrorCode(t, err, apperror.CodeForbidden)
	_, err = assembly.UpdateAgentDefinition(ctx, UpdateAgentDefinitionRequest{
		AgentID:      agent.AgentID,
		SystemPrompt: "intruder prompt",
		UpdatedBy:    intruder.UserID,
	})
	assertAppErrorCode(t, err, apperror.CodeForbidden)
}

func mustRegisterDefinitionTool(t *testing.T, ctx context.Context, registry repository.AgentRegistryRepository, tool model.AgentTool) model.AgentTool {
	t.Helper()
	if tool.InputSchemaJSON == "" {
		tool.InputSchemaJSON = `{"type":"object"}`
	}
	if tool.OutputSchemaJSON == "" {
		tool.OutputSchemaJSON = `{"type":"object"}`
	}
	if tool.PermissionLevel == "" {
		tool.PermissionLevel = "agent_bound"
	}
	tool.Status = model.AgentToolStatusActive
	tool.AdminConfigured = true
	tool.CreatedBy = "usr_admin"
	created, err := registry.RegisterTool(ctx, tool)
	if err != nil {
		t.Fatalf("register tool %q: %v", tool.Name, err)
	}
	return created
}

func assertAcceptedDefinitionFriendship(t *testing.T, ctx context.Context, repo repository.FriendshipRepository, userID string, friendID string) {
	t.Helper()
	friendship, err := repo.GetFriendship(ctx, userID, friendID)
	if err != nil {
		t.Fatalf("get friendship %s -> %s: %v", userID, friendID, err)
	}
	if friendship.Status != model.FriendshipStatusAccepted {
		t.Fatalf("friendship %s -> %s = %q, want accepted", userID, friendID, friendship.Status)
	}
}

func toolNameToIdentifierSuffix(name string) string {
	switch name {
	case model.LocalToolHandlerPythonExecute:
		return "python"
	case model.LocalToolHandlerSendAgentMessage:
		return "send"
	default:
		return "tool"
	}
}
