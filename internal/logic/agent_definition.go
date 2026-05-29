package logic

import (
	"context"
	"strings"
	"unicode"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

const (
	defaultCreatedAgentDescription = "Created by agent_creator"
	defaultCreatedAgentPromptName  = "agent_system_prompt"
)

type AgentAssemblyLogic struct {
	accounts    repository.AccountRepository
	friendships repository.FriendshipRepository
	agents      repository.AgentRepository
	registry    repository.AgentRegistryRepository
}

type AgentAssemblyDependencies struct {
	Accounts    repository.AccountRepository
	Friendships repository.FriendshipRepository
	Agents      repository.AgentRepository
	Registry    repository.AgentRegistryRepository
}

func NewAgentAssemblyLogic(deps AgentAssemblyDependencies) *AgentAssemblyLogic {
	return &AgentAssemblyLogic{
		accounts:    deps.Accounts,
		friendships: deps.Friendships,
		agents:      deps.Agents,
		registry:    deps.Registry,
	}
}

type AgentPromptDefinition struct {
	PromptID            string `json:"prompt_id"`
	Name                string `json:"name"`
	Description         string `json:"description"`
	Content             string `json:"content"`
	VariablesSchemaJSON string `json:"variables_schema_json"`
	Version             string `json:"version"`
	Status              string `json:"status"`
	CreatedBy           string `json:"created_by"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
}

type AgentToolDefinition struct {
	ToolID           string `json:"tool_id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	ToolType         string `json:"tool_type"`
	MCPServerID      string `json:"mcp_server_id"`
	MCPToolName      string `json:"mcp_tool_name"`
	LocalHandlerKey  string `json:"local_handler_key"`
	BuiltinKey       string `json:"builtin_key"`
	InputSchemaJSON  string `json:"input_schema_json"`
	OutputSchemaJSON string `json:"output_schema_json"`
	PermissionLevel  string `json:"permission_level"`
	Status           string `json:"status"`
	AdminConfigured  bool   `json:"admin_configured"`
}

type AgentDefinition struct {
	Agent        AgentInfo             `json:"agent"`
	SystemPrompt AgentPromptDefinition `json:"system_prompt"`
	Tools        []AgentToolDefinition `json:"tools"`
}

type AgentDefinitionRequest struct {
	AgentID     string `json:"agent_id"`
	RequestedBy string `json:"requested_by"`
}

type UpdateAgentDefinitionRequest struct {
	AgentID      string   `json:"agent_id"`
	SystemPrompt string   `json:"system_prompt"`
	ToolNames    []string `json:"tool_names"`
	UpdatedBy    string   `json:"updated_by"`
}

type AgentCreateToolRequest struct {
	CreatorAgentID   string
	RequestingUserID string
	Identifier       string
	Name             string
	Description      string
	SystemPrompt     string
	ToolNames        []string
}

type AgentCreateToolResponse struct {
	AgentID      string   `json:"agent_id"`
	AccountID    string   `json:"account_id"`
	Identifier   string   `json:"identifier"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	PromptID     string   `json:"prompt_id"`
	ToolNames    []string `json:"tool_names"`
	FriendUserID string   `json:"friend_user_id"`
}

func (l *AgentAssemblyLogic) GetAgentDefinition(ctx context.Context, req AgentDefinitionRequest) (AgentDefinition, error) {
	if err := l.ensureDefinitionConfigured(); err != nil {
		return AgentDefinition{}, err
	}
	agentID, err := normalizeRequiredID(req.AgentID, "agent_id")
	if err != nil {
		return AgentDefinition{}, err
	}
	requestedBy, err := normalizeRequiredID(req.RequestedBy, "requested_by")
	if err != nil {
		return AgentDefinition{}, err
	}
	agent, err := l.agents.GetAgent(ctx, agentID)
	if err != nil {
		return AgentDefinition{}, err
	}
	if err := l.authorizeAgentDefinitionAccess(ctx, requestedBy, agent); err != nil {
		return AgentDefinition{}, err
	}
	prompt, err := l.activePromptDefinition(ctx, agentID)
	if err != nil {
		return AgentDefinition{}, err
	}
	tools, err := l.boundToolDefinitions(ctx, agentID)
	if err != nil {
		return AgentDefinition{}, err
	}
	return AgentDefinition{
		Agent:        toAgentInfo(agent),
		SystemPrompt: prompt,
		Tools:        tools,
	}, nil
}

func (l *AgentAssemblyLogic) UpdateAgentDefinition(ctx context.Context, req UpdateAgentDefinitionRequest) (AgentDefinition, error) {
	if err := l.ensureDefinitionConfigured(); err != nil {
		return AgentDefinition{}, err
	}
	agentID, err := normalizeRequiredID(req.AgentID, "agent_id")
	if err != nil {
		return AgentDefinition{}, err
	}
	updatedBy, err := normalizeRequiredID(req.UpdatedBy, "updated_by")
	if err != nil {
		return AgentDefinition{}, err
	}
	systemPrompt, err := normalizeSystemPrompt(req.SystemPrompt)
	if err != nil {
		return AgentDefinition{}, err
	}
	agent, err := l.agents.GetAgent(ctx, agentID)
	if err != nil {
		return AgentDefinition{}, err
	}
	if err := l.authorizeAgentDefinitionAccess(ctx, updatedBy, agent); err != nil {
		return AgentDefinition{}, err
	}
	tools, err := l.resolveDefinitionTools(ctx, req.ToolNames)
	if err != nil {
		return AgentDefinition{}, err
	}
	version, err := newDefinitionPromptVersion()
	if err != nil {
		return AgentDefinition{}, err
	}
	prompt, err := l.registry.CreatePrompt(ctx, model.AgentPrompt{
		Name:                definitionPromptName(agentID),
		Description:         "Active system prompt for agent " + agentID,
		Content:             systemPrompt,
		VariablesSchemaJSON: "{}",
		Version:             version,
		Status:              model.AgentPromptStatusActive,
		CreatedBy:           updatedBy,
	})
	if err != nil {
		return AgentDefinition{}, err
	}
	if _, err := l.registry.ReplacePromptBindings(ctx, agentID, []string{prompt.PromptID}, updatedBy); err != nil {
		return AgentDefinition{}, err
	}
	toolIDs := make([]string, 0, len(tools))
	for _, tool := range tools {
		toolIDs = append(toolIDs, tool.ToolID)
	}
	if _, err := l.registry.ReplaceToolBindings(ctx, agentID, toolIDs, updatedBy); err != nil {
		return AgentDefinition{}, err
	}
	return l.GetAgentDefinition(ctx, AgentDefinitionRequest{AgentID: agentID, RequestedBy: updatedBy})
}

func (l *AgentAssemblyLogic) CreateAgentFromTool(ctx context.Context, req AgentCreateToolRequest) (AgentCreateToolResponse, error) {
	if err := l.ensureCreateToolConfigured(); err != nil {
		return AgentCreateToolResponse{}, err
	}
	if postgresRepo, ok := l.accounts.(*repository.PostgresRepository); ok {
		var response AgentCreateToolResponse
		err := postgresRepo.TransactRepository(ctx, func(txRepo *repository.PostgresRepository) error {
			txLogic := NewAgentAssemblyLogic(AgentAssemblyDependencies{
				Accounts:    txRepo,
				Friendships: txRepo,
				Agents:      txRepo,
				Registry:    txRepo,
			})
			created, err := txLogic.createAgentFromTool(ctx, req)
			if err != nil {
				return err
			}
			response = created
			return nil
		})
		if err != nil {
			return AgentCreateToolResponse{}, err
		}
		return response, nil
	}
	return l.createAgentFromTool(ctx, req)
}

func (l *AgentAssemblyLogic) createAgentFromTool(ctx context.Context, req AgentCreateToolRequest) (AgentCreateToolResponse, error) {
	creatorAgentID, err := normalizeRequiredID(req.CreatorAgentID, "creator_agent_id")
	if err != nil {
		return AgentCreateToolResponse{}, err
	}
	if err := l.authorizeAgentCreateCaller(ctx, creatorAgentID); err != nil {
		return AgentCreateToolResponse{}, err
	}
	requestingUserID, err := normalizeRequiredID(req.RequestingUserID, "requesting_user_id")
	if err != nil {
		return AgentCreateToolResponse{}, err
	}
	requester, err := l.accounts.GetByID(ctx, requestingUserID)
	if err != nil {
		return AgentCreateToolResponse{}, err
	}
	if requester.AccountType != model.AccountTypeUser {
		return AgentCreateToolResponse{}, apperror.Forbidden("agent.create requester must be a human user account")
	}
	name, err := normalizeAgentName(req.Name)
	if err != nil {
		return AgentCreateToolResponse{}, err
	}
	description, err := normalizeAgentDescription(req.Description)
	if err != nil {
		return AgentCreateToolResponse{}, err
	}
	if description == "" {
		return AgentCreateToolResponse{}, apperror.InvalidArgument("description is required")
	}
	systemPrompt := strings.TrimSpace(req.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = generatedAgentSystemPrompt(name, description)
	}
	systemPrompt, err = normalizeSystemPrompt(systemPrompt)
	if err != nil {
		return AgentCreateToolResponse{}, err
	}
	tools, err := l.resolveCreatableTools(ctx, req.ToolNames)
	if err != nil {
		return AgentCreateToolResponse{}, err
	}
	identifier, err := l.createAgentIdentifier(ctx, req.Identifier, name)
	if err != nil {
		return AgentCreateToolResponse{}, err
	}

	account, err := NewUserLogic(l.accounts).CreateUser(ctx, CreateUserRequest{
		Identifier:  identifier,
		DisplayName: name,
		Name:        name,
		AccountType: string(model.AccountTypeAgent),
	})
	if err != nil {
		return AgentCreateToolResponse{}, err
	}
	agentLogic := NewAgentLogic(l.agents, NewUserLogicAccountTypeChecker(NewUserLogic(l.accounts)))
	agent, err := agentLogic.CreateAgent(ctx, CreateAgentRequest{
		AccountID:   account.AccountID,
		Name:        name,
		Description: description,
		Status:      model.AgentStatusActive,
		CreatedBy:   requestingUserID,
	})
	if err != nil {
		return AgentCreateToolResponse{}, err
	}
	version, err := newDefinitionPromptVersion()
	if err != nil {
		return AgentCreateToolResponse{}, err
	}
	prompt, err := l.registry.CreatePrompt(ctx, model.AgentPrompt{
		Name:                definitionPromptName(agent.AgentID),
		Description:         "Active system prompt for agent " + agent.AgentID,
		Content:             systemPrompt,
		VariablesSchemaJSON: "{}",
		Version:             version,
		Status:              model.AgentPromptStatusActive,
		CreatedBy:           requestingUserID,
	})
	if err != nil {
		return AgentCreateToolResponse{}, err
	}
	if _, err := l.registry.ReplacePromptBindings(ctx, agent.AgentID, []string{prompt.PromptID}, requestingUserID); err != nil {
		return AgentCreateToolResponse{}, err
	}
	toolIDs := make([]string, 0, len(tools))
	toolNames := make([]string, 0, len(tools))
	for _, tool := range tools {
		toolIDs = append(toolIDs, tool.ToolID)
		toolNames = append(toolNames, tool.Name)
	}
	if _, err := l.registry.ReplaceToolBindings(ctx, agent.AgentID, toolIDs, requestingUserID); err != nil {
		return AgentCreateToolResponse{}, err
	}
	if err := l.friendships.EnsureAcceptedFriendship(ctx, requestingUserID, account.AccountID); err != nil {
		return AgentCreateToolResponse{}, err
	}
	return AgentCreateToolResponse{
		AgentID:      agent.AgentID,
		AccountID:    account.AccountID,
		Identifier:   account.Identifier,
		Name:         agent.Name,
		Description:  agent.Description,
		PromptID:     prompt.PromptID,
		ToolNames:    toolNames,
		FriendUserID: requestingUserID,
	}, nil
}

func (l *AgentAssemblyLogic) authorizeAgentCreateCaller(ctx context.Context, creatorAgentID string) error {
	agent, err := l.agents.GetAgent(ctx, creatorAgentID)
	if err != nil {
		return err
	}
	if agent.Status != model.AgentStatusActive {
		return apperror.Forbidden("agent.create caller must be the active default AI assistant")
	}
	account, err := l.accounts.GetByID(ctx, agent.AccountID)
	if err != nil {
		return err
	}
	if account.AccountType != model.AccountTypeAgent || account.Identifier != DefaultAssistantIdentifier {
		return apperror.Forbidden("only the default AI assistant can create agents")
	}
	return nil
}

func (l *AgentAssemblyLogic) authorizeAgentDefinitionAccess(ctx context.Context, accountID string, agent model.Agent) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return apperror.Unauthenticated("user_id is required")
	}
	if accountID == agent.CreatedBy || accountID == agent.AccountID {
		return nil
	}
	if l.accounts != nil {
		user, err := l.accounts.GetByID(ctx, accountID)
		if err == nil && user.AccountType == model.AccountTypeAdmin {
			return nil
		}
	}
	return apperror.Forbidden("agent definition access denied")
}

func (l *AgentAssemblyLogic) activePromptDefinition(ctx context.Context, agentID string) (AgentPromptDefinition, error) {
	bindings, err := l.registry.ListPromptBindings(ctx, agentID)
	if err != nil {
		return AgentPromptDefinition{}, err
	}
	var active *model.AgentPrompt
	for _, binding := range bindings {
		prompt, err := l.registry.GetPrompt(ctx, binding.PromptID)
		if err != nil {
			return AgentPromptDefinition{}, err
		}
		if prompt.Status != model.AgentPromptStatusActive {
			continue
		}
		if active != nil {
			return AgentPromptDefinition{}, apperror.Internal("agent has multiple active system prompt bindings")
		}
		copied := prompt
		active = &copied
	}
	if active == nil {
		return AgentPromptDefinition{}, nil
	}
	return promptDefinition(*active), nil
}

func (l *AgentAssemblyLogic) boundToolDefinitions(ctx context.Context, agentID string) ([]AgentToolDefinition, error) {
	bindings, err := l.registry.ListToolBindings(ctx, agentID)
	if err != nil {
		return nil, err
	}
	tools := make([]AgentToolDefinition, 0, len(bindings))
	for _, binding := range bindings {
		tool, err := l.registry.GetTool(ctx, binding.ToolID)
		if err != nil {
			return nil, err
		}
		tools = append(tools, toolDefinition(tool))
	}
	return tools, nil
}

func (l *AgentAssemblyLogic) resolveDefinitionTools(ctx context.Context, names []string) ([]model.AgentTool, error) {
	toolNames, err := normalizeToolNames(names)
	if err != nil {
		return nil, err
	}
	tools := make([]model.AgentTool, 0, len(toolNames))
	for _, name := range toolNames {
		tool, err := l.registry.GetToolByName(ctx, name)
		if err != nil {
			return nil, err
		}
		if err := l.validateBindableTool(ctx, tool); err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

func (l *AgentAssemblyLogic) resolveCreatableTools(ctx context.Context, names []string) ([]model.AgentTool, error) {
	toolNames, err := normalizeToolNames(names)
	if err != nil {
		return nil, err
	}
	if len(toolNames) == 0 {
		return l.defaultCreatableTools(ctx)
	}
	tools := make([]model.AgentTool, 0, len(toolNames))
	for _, name := range toolNames {
		tool, err := l.registry.GetToolByName(ctx, name)
		if err != nil {
			return nil, err
		}
		if err := validateCreatableTool(tool); err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

func (l *AgentAssemblyLogic) defaultCreatableTools(ctx context.Context) ([]model.AgentTool, error) {
	activeTools, err := l.registry.ListActiveTools(ctx)
	if err != nil {
		return nil, err
	}
	tools := make([]model.AgentTool, 0, len(activeTools))
	for _, tool := range activeTools {
		if err := validateCreatableTool(tool); err != nil {
			continue
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

func (l *AgentAssemblyLogic) validateBindableTool(ctx context.Context, tool model.AgentTool) error {
	if tool.Status != model.AgentToolStatusActive || !tool.AdminConfigured {
		return apperror.InvalidArgument("only active admin-configured tools can be bound")
	}
	if tool.ToolType == model.AgentToolTypeMCP {
		server, err := l.registry.GetMCPServer(ctx, tool.MCPServerID)
		if err != nil {
			return err
		}
		if server.Status != model.AgentToolStatusActive || !server.AdminConfigured {
			return apperror.InvalidArgument("mcp tool server must be active and admin configured")
		}
	}
	return nil
}

func validateCreatableTool(tool model.AgentTool) error {
	if tool.Status != model.AgentToolStatusActive || !tool.AdminConfigured {
		return apperror.Forbidden("agent.create can only bind active admin-configured low-risk tools")
	}
	switch tool.ToolType {
	case model.AgentToolTypeLocal:
		if tool.LocalHandlerKey == model.LocalToolHandlerGetConversationContext && tool.Name == model.LocalToolHandlerGetConversationContext {
			return nil
		}
	case model.AgentToolTypeBuiltin:
		if tool.BuiltinKey == model.BuiltinToolReadConversationContext && tool.Name == model.BuiltinToolReadConversationContext {
			return nil
		}
	}
	return apperror.Forbidden("agent.create cannot bind high-risk or external tools by default")
}

func normalizeSystemPrompt(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument("system_prompt is required")
	}
	if len([]rune(value)) > 8000 {
		return "", apperror.InvalidArgument("system_prompt must be 8000 characters or fewer")
	}
	return value, nil
}

func generatedAgentSystemPrompt(name string, description string) string {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	return strings.TrimSpace("You are " + name + ".\n\nPurpose: " + description + "\n\nUse only the tools explicitly provided to you. Prefer concise, accurate answers. If required information is missing, ask a brief clarifying question instead of inventing facts.")
}

func normalizeToolNames(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		name := strings.TrimSpace(value)
		if name == "" {
			return nil, apperror.InvalidArgument("tool_names cannot contain empty values")
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result, nil
}

func (l *AgentAssemblyLogic) createAgentIdentifier(ctx context.Context, requested string, name string) (string, error) {
	if strings.TrimSpace(requested) != "" {
		return NormalizeIdentifier(requested)
	}
	base := identifierBaseFromName(name)
	for attempt := 0; attempt < 5; attempt++ {
		generated, err := idgen.NewString()
		if err != nil {
			return "", err
		}
		candidate, err := NormalizeIdentifier(identifierWithSuffix(base, generated))
		if err != nil {
			return "", err
		}
		exists, err := l.accounts.ExistsByIdentifier(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", apperror.AlreadyExists("could not allocate a unique agent identifier")
}

func identifierBaseFromName(name string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(name) {
		isASCIIAlpha := r >= 'a' && r <= 'z'
		isASCIIDigit := r >= '0' && r <= '9'
		if isASCIIAlpha || isASCIIDigit {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if unicode.IsSpace(r) || r == '_' || r == '-' {
			if b.Len() > 0 && !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	base := strings.Trim(b.String(), "_")
	if len(base) < 3 {
		return "agent"
	}
	if len(base) > 20 {
		base = strings.TrimRight(base[:20], "_")
	}
	if len(base) < 3 {
		return "agent"
	}
	return base
}

func identifierWithSuffix(base string, generated string) string {
	generated = strings.TrimSpace(generated)
	if len(generated) > 8 {
		generated = generated[len(generated)-8:]
	}
	maxBaseLen := 32 - len(generated) - 1
	if maxBaseLen < 3 {
		maxBaseLen = 3
	}
	if len(base) > maxBaseLen {
		base = strings.TrimRight(base[:maxBaseLen], "_")
	}
	if len(base) < 3 {
		base = "agent"
	}
	return base + "_" + generated
}

func definitionPromptName(agentID string) string {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return defaultCreatedAgentPromptName
	}
	return "agent_" + agentID + "_system_prompt"
}

func newDefinitionPromptVersion() (string, error) {
	generated, err := idgen.NewString()
	if err != nil {
		return "", err
	}
	return "v" + generated, nil
}

func promptDefinition(prompt model.AgentPrompt) AgentPromptDefinition {
	return AgentPromptDefinition{
		PromptID:            prompt.PromptID,
		Name:                prompt.Name,
		Description:         prompt.Description,
		Content:             prompt.Content,
		VariablesSchemaJSON: prompt.VariablesSchemaJSON,
		Version:             prompt.Version,
		Status:              string(prompt.Status),
		CreatedBy:           prompt.CreatedBy,
		CreatedAt:           formatTime(prompt.CreatedAt),
		UpdatedAt:           formatTime(prompt.UpdatedAt),
	}
}

func toolDefinition(tool model.AgentTool) AgentToolDefinition {
	return AgentToolDefinition{
		ToolID:           tool.ToolID,
		Name:             tool.Name,
		Description:      tool.Description,
		ToolType:         string(tool.ToolType),
		MCPServerID:      tool.MCPServerID,
		MCPToolName:      tool.MCPToolName,
		LocalHandlerKey:  tool.LocalHandlerKey,
		BuiltinKey:       tool.BuiltinKey,
		InputSchemaJSON:  tool.InputSchemaJSON,
		OutputSchemaJSON: tool.OutputSchemaJSON,
		PermissionLevel:  tool.PermissionLevel,
		Status:           string(tool.Status),
		AdminConfigured:  tool.AdminConfigured,
	}
}

func (l *AgentAssemblyLogic) ensureDefinitionConfigured() error {
	if l == nil || l.agents == nil || l.registry == nil {
		return apperror.Internal("agent definition logic is not configured")
	}
	return nil
}

func (l *AgentAssemblyLogic) ensureCreateToolConfigured() error {
	if err := l.ensureDefinitionConfigured(); err != nil {
		return err
	}
	if l.accounts == nil || l.friendships == nil {
		return apperror.Internal("agent create logic is not configured")
	}
	return nil
}
