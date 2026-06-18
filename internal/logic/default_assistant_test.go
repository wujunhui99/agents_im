package logic

import (
	"context"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/model"
)

func TestDefaultAssistantBackfillCreatesCanonicalAccountAgentAndPrompt(t *testing.T) {
	ctx := context.Background()
	accountRepo := repository.NewMemoryRepository()
	agentRepo := repository.NewMemoryAgentRepository()
	registryRepo := repository.NewMemoryAgentRegistryRepository()
	provisioner := NewDefaultAssistantProvisioner(accountRepo, agentRepo, registryRepo)

	if _, err := provisioner.Backfill(ctx); err != nil {
		t.Fatalf("backfill default assistant: %v", err)
	}

	assistant, err := accountRepo.GetByIdentifier(ctx, DefaultAssistantIdentifier)
	if err != nil {
		t.Fatalf("get default assistant account: %v", err)
	}
	if assistant.AccountType != model.AccountTypeAgent {
		t.Fatalf("account_type = %q, want agent", assistant.AccountType)
	}
	if assistant.DisplayName != DefaultAssistantDisplayName || assistant.Name != DefaultAssistantIdentifier {
		t.Fatalf("profile = %+v, want display %q and name %q", assistant, DefaultAssistantDisplayName, DefaultAssistantIdentifier)
	}

	agentAccounts, err := accountRepo.ListByAccountType(ctx, model.AccountTypeAgent)
	if err != nil {
		t.Fatalf("list agent accounts: %v", err)
	}
	if len(agentAccounts) != 1 || agentAccounts[0].AccountID != assistant.AccountID {
		t.Fatalf("agent accounts = %+v, want exactly agent_creator account", agentAccounts)
	}

	agent, err := agentRepo.GetAgentByIMUserID(ctx, assistant.AccountID)
	if err != nil {
		t.Fatalf("get default assistant agent config: %v", err)
	}
	if agent.Name != DefaultAssistantAgentName || agent.Status != model.AgentStatusActive {
		t.Fatalf("agent config = %+v, want active %q", agent, DefaultAssistantAgentName)
	}

	agents, err := agentRepo.ListAgents(ctx, repository.AgentListFilter{})
	if err != nil {
		t.Fatalf("list agents: %v", err)
	}
	if len(agents) != 1 || agents[0].AccountID != assistant.AccountID {
		t.Fatalf("agents = %+v, want exactly one default assistant config", agents)
	}

	prompt, err := registryRepo.GetPromptByNameVersion(ctx, DefaultAssistantPromptName, DefaultAssistantPromptVersion)
	if err != nil {
		t.Fatalf("get default assistant prompt: %v", err)
	}
	if prompt.Status != model.AgentPromptStatusActive {
		t.Fatalf("prompt status = %q, want active", prompt.Status)
	}
	for _, want := range []string{"准确、简洁、友好", "不要编造事实", "可验证的下一步"} {
		if !strings.Contains(prompt.Content, want) {
			t.Fatalf("default prompt missing %q: %q", want, prompt.Content)
		}
	}
}

func TestDefaultAssistantBackfillBindsManagedToolsOnlyToDefaultAssistant(t *testing.T) {
	ctx := context.Background()
	accountRepo := repository.NewMemoryRepository()
	agentRepo := repository.NewMemoryAgentRepository()
	registryRepo := repository.NewMemoryAgentRegistryRepository()
	userLogic := NewUserLogic(accountRepo)

	otherAgentAccount := mustCreateDefaultAssistantTestAccount(t, ctx, userLogic, "agent_without_python_tool", model.AccountTypeAgent)
	otherAgent, err := agentRepo.CreateAgent(ctx, model.Agent{
		AccountID:   otherAgentAccount.UserID,
		IMUserID:    otherAgentAccount.UserID,
		Name:        "agent_without_python_tool",
		Description: "Non-default test agent",
		Status:      model.AgentStatusActive,
		CreatedBy:   otherAgentAccount.UserID,
	})
	if err != nil {
		t.Fatalf("create non-default agent: %v", err)
	}

	provisioner := NewDefaultAssistantProvisioner(accountRepo, agentRepo, registryRepo)
	if _, err := provisioner.Backfill(ctx); err != nil {
		t.Fatalf("backfill default assistant: %v", err)
	}

	assistant, err := accountRepo.GetByIdentifier(ctx, DefaultAssistantIdentifier)
	if err != nil {
		t.Fatalf("get default assistant account: %v", err)
	}
	defaultAgent, err := agentRepo.GetAgentByIMUserID(ctx, assistant.AccountID)
	if err != nil {
		t.Fatalf("get default assistant agent config: %v", err)
	}

	defaultBindings, err := registryRepo.ListToolBindings(ctx, defaultAgent.AgentID)
	if err != nil {
		t.Fatalf("list default assistant tool bindings: %v", err)
	}
	var pythonTool model.AgentTool
	var agentCreateTool model.AgentTool
	for _, binding := range defaultBindings {
		tool, err := registryRepo.GetTool(ctx, binding.ToolID)
		if err != nil {
			t.Fatalf("get bound tool %q: %v", binding.ToolID, err)
		}
		if tool.Name == model.LocalToolHandlerPythonExecute {
			pythonTool = tool
		}
		if tool.Name == model.LocalToolHandlerAgentCreate {
			agentCreateTool = tool
		}
	}
	if pythonTool.ToolID == "" {
		t.Fatalf("default assistant bindings = %+v, want python.execute tool binding", defaultBindings)
	}
	if agentCreateTool.ToolID == "" {
		t.Fatalf("default assistant bindings = %+v, want agent.create tool binding", defaultBindings)
	}
	if pythonTool.ToolType != model.AgentToolTypeLocal ||
		pythonTool.LocalHandlerKey != model.LocalToolHandlerPythonExecute ||
		pythonTool.PermissionLevel != "restricted" ||
		pythonTool.Status != model.AgentToolStatusActive ||
		!pythonTool.AdminConfigured {
		t.Fatalf("python.execute tool metadata = %+v, want active admin local restricted tool", pythonTool)
	}
	for _, want := range []string{`"code"`, `"timeout_seconds"`, `"files"`} {
		if !strings.Contains(pythonTool.InputSchemaJSON, want) {
			t.Fatalf("python.execute input schema missing %s: %s", want, pythonTool.InputSchemaJSON)
		}
	}
	if agentCreateTool.ToolType != model.AgentToolTypeLocal ||
		agentCreateTool.LocalHandlerKey != model.LocalToolHandlerAgentCreate ||
		agentCreateTool.PermissionLevel != "restricted" ||
		agentCreateTool.Status != model.AgentToolStatusActive ||
		!agentCreateTool.AdminConfigured {
		t.Fatalf("agent.create tool metadata = %+v, want active admin local restricted tool", agentCreateTool)
	}
	for _, want := range []string{`"name"`, `"system_prompt"`, `"tool_names"`} {
		if !strings.Contains(agentCreateTool.InputSchemaJSON, want) {
			t.Fatalf("agent.create input schema missing %s: %s", want, agentCreateTool.InputSchemaJSON)
		}
	}

	if _, err := provisioner.Backfill(ctx); err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	secondDefaultBindings, err := registryRepo.ListToolBindings(ctx, defaultAgent.AgentID)
	if err != nil {
		t.Fatalf("list default assistant tool bindings after second run: %v", err)
	}
	pythonBindingCount := 0
	agentCreateBindingCount := 0
	for _, binding := range secondDefaultBindings {
		if binding.ToolID == pythonTool.ToolID {
			pythonBindingCount++
		}
		if binding.ToolID == agentCreateTool.ToolID {
			agentCreateBindingCount++
		}
	}
	if pythonBindingCount != 1 {
		t.Fatalf("python.execute binding count after idempotent backfill = %d, want 1: %+v", pythonBindingCount, secondDefaultBindings)
	}
	if agentCreateBindingCount != 1 {
		t.Fatalf("agent.create binding count after idempotent backfill = %d, want 1: %+v", agentCreateBindingCount, secondDefaultBindings)
	}

	otherBindings, err := registryRepo.ListToolBindings(ctx, otherAgent.AgentID)
	if err != nil {
		t.Fatalf("list non-default agent tool bindings: %v", err)
	}
	for _, binding := range otherBindings {
		if binding.ToolID == pythonTool.ToolID {
			t.Fatalf("non-default agent received python.execute binding: %+v", otherBindings)
		}
		if binding.ToolID == agentCreateTool.ToolID {
			t.Fatalf("non-default agent received agent.create binding: %+v", otherBindings)
		}
	}
}

func TestDefaultAssistantBackfillAddsAcceptedFriendshipForHumanUsersOnly(t *testing.T) {
	ctx := context.Background()
	accountRepo := repository.NewMemoryRepository()
	agentRepo := repository.NewMemoryAgentRepository()
	registryRepo := repository.NewMemoryAgentRegistryRepository()
	userLogic := NewUserLogic(accountRepo)

	alice := mustCreateDefaultAssistantTestAccount(t, ctx, userLogic, "alice_default_friend", model.AccountTypeUser)
	bob := mustCreateDefaultAssistantTestAccount(t, ctx, userLogic, "bob_default_friend", model.AccountTypeUser)
	admin := mustCreateDefaultAssistantTestAccount(t, ctx, userLogic, "admin_default_friend", model.AccountTypeAdmin)
	agentAccount := mustCreateDefaultAssistantTestAccount(t, ctx, userLogic, "agent_default_friend", model.AccountTypeAgent)

	provisioner := NewDefaultAssistantProvisioner(accountRepo, agentRepo, registryRepo)
	if _, err := provisioner.Backfill(ctx); err != nil {
		t.Fatalf("backfill default assistant: %v", err)
	}
	assistant, err := accountRepo.GetByIdentifier(ctx, DefaultAssistantIdentifier)
	if err != nil {
		t.Fatalf("get default assistant account: %v", err)
	}

	assertAcceptedDefaultAssistantFriendship(t, ctx, accountRepo, alice.UserID, assistant.UserID)
	assertAcceptedDefaultAssistantFriendship(t, ctx, accountRepo, assistant.UserID, alice.UserID)
	assertAcceptedDefaultAssistantFriendship(t, ctx, accountRepo, bob.UserID, assistant.UserID)
	assertAcceptedDefaultAssistantFriendship(t, ctx, accountRepo, assistant.UserID, bob.UserID)
	assertNoDefaultAssistantFriendship(t, ctx, accountRepo, admin.UserID, assistant.UserID)
	assertNoDefaultAssistantFriendship(t, ctx, accountRepo, agentAccount.UserID, assistant.UserID)
}

func TestDefaultAssistantBackfillIsIdempotentAndDuplicateFree(t *testing.T) {
	ctx := context.Background()
	accountRepo := repository.NewMemoryRepository()
	agentRepo := repository.NewMemoryAgentRepository()
	registryRepo := repository.NewMemoryAgentRegistryRepository()
	userLogic := NewUserLogic(accountRepo)
	alice := mustCreateDefaultAssistantTestAccount(t, ctx, userLogic, "alice_default_idempotent", model.AccountTypeUser)

	provisioner := NewDefaultAssistantProvisioner(accountRepo, agentRepo, registryRepo)
	if _, err := provisioner.Backfill(ctx); err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	firstAssistant, err := accountRepo.GetByIdentifier(ctx, DefaultAssistantIdentifier)
	if err != nil {
		t.Fatalf("get default assistant after first run: %v", err)
	}
	firstPrompt, err := registryRepo.GetPromptByNameVersion(ctx, DefaultAssistantPromptName, DefaultAssistantPromptVersion)
	if err != nil {
		t.Fatalf("get default assistant prompt after first run: %v", err)
	}
	if _, err := provisioner.Backfill(ctx); err != nil {
		t.Fatalf("second backfill: %v", err)
	}

	agentAccounts, err := accountRepo.ListByAccountType(ctx, model.AccountTypeAgent)
	if err != nil {
		t.Fatalf("list agent accounts: %v", err)
	}
	if len(agentAccounts) != 1 || agentAccounts[0].AccountID != firstAssistant.AccountID {
		t.Fatalf("agent accounts after two runs = %+v, want one stable account", agentAccounts)
	}
	agents, err := agentRepo.ListAgents(ctx, repository.AgentListFilter{})
	if err != nil {
		t.Fatalf("list agents: %v", err)
	}
	if len(agents) != 1 || agents[0].AccountID != firstAssistant.AccountID {
		t.Fatalf("agents after two runs = %+v, want one stable agent config", agents)
	}
	secondPrompt, err := registryRepo.GetPromptByNameVersion(ctx, DefaultAssistantPromptName, DefaultAssistantPromptVersion)
	if err != nil {
		t.Fatalf("get prompt after second run: %v", err)
	}
	if secondPrompt.PromptID != firstPrompt.PromptID {
		t.Fatalf("prompt id changed after idempotent backfill: first=%q second=%q", firstPrompt.PromptID, secondPrompt.PromptID)
	}

	aliceFriends, err := accountRepo.ListFriends(ctx, alice.UserID)
	if err != nil {
		t.Fatalf("list alice friends: %v", err)
	}
	if len(aliceFriends) != 1 || aliceFriends[0].FriendID != firstAssistant.AccountID || aliceFriends[0].Status != model.FriendshipStatusAccepted {
		t.Fatalf("alice friends after two runs = %+v, want exactly accepted agent_creator", aliceFriends)
	}
}

func TestUserLogicCreateUserEnsuresDefaultAssistantFriend(t *testing.T) {
	ctx := context.Background()
	accountRepo := repository.NewMemoryRepository()
	agentRepo := repository.NewMemoryAgentRepository()
	registryRepo := repository.NewMemoryAgentRegistryRepository()
	userLogic := NewUserLogic(accountRepo).WithDefaultAssistantProvisioner(
		NewDefaultAssistantProvisioner(accountRepo, agentRepo, registryRepo),
	)

	registered, err := userLogic.CreateUser(ctx, CreateUserRequest{
		Identifier:  "new_user_default_friend",
		DisplayName: "New User",
	})
	if err != nil {
		t.Fatalf("create user with default assistant: %v", err)
	}
	assistant, err := accountRepo.GetByIdentifier(ctx, DefaultAssistantIdentifier)
	if err != nil {
		t.Fatalf("get default assistant account: %v", err)
	}
	assertAcceptedDefaultAssistantFriendship(t, ctx, accountRepo, registered.UserID, assistant.UserID)
	assertAcceptedDefaultAssistantFriendship(t, ctx, accountRepo, assistant.UserID, registered.UserID)

	agentAccount, err := userLogic.CreateUser(ctx, CreateUserRequest{
		Identifier:  "agent_skip_default_friend",
		DisplayName: "Agent Skip",
		AccountType: string(model.AccountTypeAgent),
	})
	if err != nil {
		t.Fatalf("create agent account: %v", err)
	}
	adminAccount, err := userLogic.CreateUser(ctx, CreateUserRequest{
		Identifier:  "admin_skip_default_friend",
		DisplayName: "Admin Skip",
		AccountType: string(model.AccountTypeAdmin),
	})
	if err != nil {
		t.Fatalf("create admin account: %v", err)
	}
	assertNoDefaultAssistantFriendship(t, ctx, accountRepo, agentAccount.UserID, assistant.UserID)
	assertNoDefaultAssistantFriendship(t, ctx, accountRepo, adminAccount.UserID, assistant.UserID)
}

func TestDefaultAssistantBackfillRenamesLegacyAgentFather(t *testing.T) {
	ctx := context.Background()
	accountRepo := repository.NewMemoryRepository()
	agentRepo := repository.NewMemoryAgentRepository()
	registryRepo := repository.NewMemoryAgentRegistryRepository()
	userLogic := NewUserLogic(accountRepo)

	legacy := mustCreateDefaultAssistantTestAccount(t, ctx, userLogic, DefaultAssistantLegacyIdentifier, model.AccountTypeAgent)
	if _, err := agentRepo.CreateAgent(ctx, model.Agent{
		AccountID: legacy.UserID,
		Name:      DefaultAssistantLegacyIdentifier,
		Status:    model.AgentStatusDisabled,
		CreatedBy: legacy.UserID,
	}); err != nil {
		t.Fatalf("create legacy agent config: %v", err)
	}

	provisioner := NewDefaultAssistantProvisioner(accountRepo, agentRepo, registryRepo)
	if _, err := provisioner.Backfill(ctx); err != nil {
		t.Fatalf("backfill default assistant with legacy account: %v", err)
	}

	canonical, err := accountRepo.GetByIdentifier(ctx, DefaultAssistantIdentifier)
	if err != nil {
		t.Fatalf("get renamed canonical account: %v", err)
	}
	if canonical.UserID != legacy.UserID {
		t.Fatalf("renamed account id = %q, want legacy id %q", canonical.UserID, legacy.UserID)
	}
	if _, err := accountRepo.GetByIdentifier(ctx, DefaultAssistantLegacyIdentifier); err == nil {
		t.Fatalf("legacy identifier %q still resolves after migration", DefaultAssistantLegacyIdentifier)
	}
	agent, err := agentRepo.GetAgentByIMUserID(ctx, canonical.UserID)
	if err != nil {
		t.Fatalf("get migrated default assistant agent config: %v", err)
	}
	if agent.Name != DefaultAssistantAgentName || agent.Status != model.AgentStatusActive {
		t.Fatalf("migrated agent config = %+v, want active canonical name", agent)
	}
}

func mustCreateDefaultAssistantTestAccount(t *testing.T, ctx context.Context, userLogic *UserLogic, identifier string, accountType model.AccountType) UserProfile {
	t.Helper()
	profile, err := userLogic.CreateUser(ctx, CreateUserRequest{
		Identifier:  identifier,
		DisplayName: identifier,
		AccountType: string(accountType),
	})
	if err != nil {
		t.Fatalf("create account %q: %v", identifier, err)
	}
	return profile
}

func assertAcceptedDefaultAssistantFriendship(t *testing.T, ctx context.Context, repo repository.FriendshipRepository, userID string, friendID string) {
	t.Helper()
	friendship, err := repo.GetFriendship(ctx, userID, friendID)
	if err != nil {
		t.Fatalf("get friendship %s -> %s: %v", userID, friendID, err)
	}
	if friendship.Status != model.FriendshipStatusAccepted {
		t.Fatalf("friendship %s -> %s status = %q, want accepted", userID, friendID, friendship.Status)
	}
}

func assertNoDefaultAssistantFriendship(t *testing.T, ctx context.Context, repo repository.FriendshipRepository, userID string, assistantID string) {
	t.Helper()
	friends, err := repo.ListFriends(ctx, userID)
	if err != nil {
		t.Fatalf("list friends for %s: %v", userID, err)
	}
	for _, friendship := range friends {
		if friendship.FriendID == assistantID {
			t.Fatalf("account %s should not have default assistant friendship: %+v", userID, friends)
		}
	}
}
