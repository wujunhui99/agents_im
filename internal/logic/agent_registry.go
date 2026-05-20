package logic

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type AgentRegistryLogic struct {
	repo repository.AgentRegistryRepository
}

func NewAgentRegistryLogic(repo repository.AgentRegistryRepository) *AgentRegistryLogic {
	return &AgentRegistryLogic{repo: repo}
}

type CreateAgentPromptRequest struct {
	Name                string
	Description         string
	Content             string
	VariablesSchemaJSON string
	Version             string
	Status              model.AgentPromptStatus
	CreatedBy           string
}

type RegisterMCPServerRequest struct {
	Name             string
	Transport        model.AgentMCPTransport
	URL              string
	ConfigJSON       string
	HeadersSecretRef string
	TimeoutSeconds   int
	Status           model.AgentToolStatus
	AdminConfigured  bool
	CreatedBy        string
}

type RegisterAgentToolRequest struct {
	Name             string
	Description      string
	ToolType         model.AgentToolType
	MCPServerID      string
	MCPToolName      string
	LocalHandlerKey  string
	BuiltinKey       string
	InputSchemaJSON  string
	OutputSchemaJSON string
	PermissionLevel  string
	Status           model.AgentToolStatus
	AdminConfigured  bool
	CreatedBy        string
}

type RegisterAgentSkillRequest struct {
	Name        string
	Description string
	Version     string
	ObjectKey   string
	SHA256      string
	ContentType string
	SizeBytes   int64
	Status      model.AgentSkillStatus
	CreatedBy   string
}

type BindAgentPromptRequest struct {
	AgentID   string
	PromptID  string
	CreatedBy string
}

type BindAgentToolRequest struct {
	AgentID   string
	ToolID    string
	CreatedBy string
}

type BindAgentSkillRequest struct {
	AgentID   string
	SkillID   string
	CreatedBy string
}

func (l *AgentRegistryLogic) CreatePrompt(ctx context.Context, req CreateAgentPromptRequest) (model.AgentPrompt, error) {
	name := strings.TrimSpace(req.Name)
	content := strings.TrimSpace(req.Content)
	version := strings.TrimSpace(req.Version)
	createdBy := strings.TrimSpace(req.CreatedBy)
	if name == "" {
		return model.AgentPrompt{}, apperror.InvalidArgument("prompt name is required")
	}
	if content == "" {
		return model.AgentPrompt{}, apperror.InvalidArgument("prompt content is required")
	}
	if version == "" {
		return model.AgentPrompt{}, apperror.InvalidArgument("prompt version is required")
	}
	if !validPromptStatus(req.Status) {
		return model.AgentPrompt{}, apperror.InvalidArgument("prompt status must be draft, active, or archived")
	}
	if createdBy == "" {
		return model.AgentPrompt{}, apperror.InvalidArgument("created_by is required")
	}
	variablesSchema, err := normalizeJSON(req.VariablesSchemaJSON)
	if err != nil {
		return model.AgentPrompt{}, apperror.InvalidArgument("variables_schema_json must be valid JSON")
	}

	return l.repo.CreatePrompt(ctx, model.AgentPrompt{
		Name:                name,
		Description:         strings.TrimSpace(req.Description),
		Content:             content,
		VariablesSchemaJSON: variablesSchema,
		Version:             version,
		Status:              req.Status,
		CreatedBy:           createdBy,
	})
}

func (l *AgentRegistryLogic) BindPrompt(ctx context.Context, req BindAgentPromptRequest) (model.AgentPromptBinding, bool, error) {
	agentID := strings.TrimSpace(req.AgentID)
	promptID := strings.TrimSpace(req.PromptID)
	createdBy := strings.TrimSpace(req.CreatedBy)
	if agentID == "" {
		return model.AgentPromptBinding{}, false, apperror.InvalidArgument("agent_id is required")
	}
	if promptID == "" {
		return model.AgentPromptBinding{}, false, apperror.InvalidArgument("prompt_id is required")
	}
	if createdBy == "" {
		return model.AgentPromptBinding{}, false, apperror.InvalidArgument("created_by is required")
	}
	prompt, err := l.repo.GetPrompt(ctx, promptID)
	if err != nil {
		return model.AgentPromptBinding{}, false, err
	}
	if prompt.Status != model.AgentPromptStatusActive {
		return model.AgentPromptBinding{}, false, apperror.InvalidArgument("only active prompts can be bound to agents")
	}

	return l.repo.BindPrompt(ctx, model.AgentPromptBinding{
		AgentID:   agentID,
		PromptID:  promptID,
		CreatedBy: createdBy,
	})
}

func (l *AgentRegistryLogic) RegisterMCPServer(ctx context.Context, req RegisterMCPServerRequest) (model.AgentMCPServer, error) {
	name := strings.TrimSpace(req.Name)
	serverURL := strings.TrimSpace(req.URL)
	createdBy := strings.TrimSpace(req.CreatedBy)
	if name == "" {
		return model.AgentMCPServer{}, apperror.InvalidArgument("mcp server name is required")
	}
	if !validMCPTransport(req.Transport) {
		return model.AgentMCPServer{}, apperror.InvalidArgument("mcp transport must be http, sse, or streamable_http")
	}
	if err := validateHTTPURL(serverURL, "mcp server url"); err != nil {
		return model.AgentMCPServer{}, err
	}
	configJSON, err := normalizeJSON(req.ConfigJSON)
	if err != nil {
		return model.AgentMCPServer{}, apperror.InvalidArgument("mcp config_json must be valid JSON")
	}
	if req.TimeoutSeconds <= 0 {
		return model.AgentMCPServer{}, apperror.InvalidArgument("mcp timeout_seconds must be greater than 0")
	}
	if !validToolStatus(req.Status) {
		return model.AgentMCPServer{}, apperror.InvalidArgument("mcp server status must be active, disabled, or archived")
	}
	if !req.AdminConfigured {
		return model.AgentMCPServer{}, apperror.InvalidArgument("mcp server must be admin configured")
	}
	if createdBy == "" {
		return model.AgentMCPServer{}, apperror.InvalidArgument("created_by is required")
	}

	return l.repo.CreateMCPServer(ctx, model.AgentMCPServer{
		Name:             name,
		Transport:        req.Transport,
		URL:              serverURL,
		ConfigJSON:       configJSON,
		HeadersSecretRef: strings.TrimSpace(req.HeadersSecretRef),
		TimeoutSeconds:   req.TimeoutSeconds,
		Status:           req.Status,
		AdminConfigured:  true,
		CreatedBy:        createdBy,
	})
}

func (l *AgentRegistryLogic) RegisterTool(ctx context.Context, req RegisterAgentToolRequest) (model.AgentTool, error) {
	name := strings.TrimSpace(req.Name)
	createdBy := strings.TrimSpace(req.CreatedBy)
	if name == "" {
		return model.AgentTool{}, apperror.InvalidArgument("tool name is required")
	}
	if !validToolType(req.ToolType) {
		return model.AgentTool{}, apperror.InvalidArgument("tool_type must be mcp, local, or builtin")
	}
	if !validToolStatus(req.Status) {
		return model.AgentTool{}, apperror.InvalidArgument("tool status must be active, disabled, or archived")
	}
	if !req.AdminConfigured {
		return model.AgentTool{}, apperror.InvalidArgument("tools must be admin configured")
	}
	if createdBy == "" {
		return model.AgentTool{}, apperror.InvalidArgument("created_by is required")
	}
	inputSchema, err := normalizeJSON(req.InputSchemaJSON)
	if err != nil {
		return model.AgentTool{}, apperror.InvalidArgument("input_schema_json must be valid JSON")
	}
	outputSchema, err := normalizeJSON(req.OutputSchemaJSON)
	if err != nil {
		return model.AgentTool{}, apperror.InvalidArgument("output_schema_json must be valid JSON")
	}

	tool := model.AgentTool{
		Name:             name,
		Description:      strings.TrimSpace(req.Description),
		ToolType:         req.ToolType,
		MCPServerID:      strings.TrimSpace(req.MCPServerID),
		MCPToolName:      strings.TrimSpace(req.MCPToolName),
		LocalHandlerKey:  strings.TrimSpace(req.LocalHandlerKey),
		BuiltinKey:       strings.TrimSpace(req.BuiltinKey),
		InputSchemaJSON:  inputSchema,
		OutputSchemaJSON: outputSchema,
		PermissionLevel:  normalizePermissionLevel(req.PermissionLevel),
		Status:           req.Status,
		AdminConfigured:  true,
		CreatedBy:        createdBy,
	}
	if err := l.validateToolShape(ctx, tool); err != nil {
		return model.AgentTool{}, err
	}

	return l.repo.RegisterTool(ctx, tool)
}

func (l *AgentRegistryLogic) BindTool(ctx context.Context, req BindAgentToolRequest) (model.AgentToolBinding, bool, error) {
	agentID := strings.TrimSpace(req.AgentID)
	toolID := strings.TrimSpace(req.ToolID)
	createdBy := strings.TrimSpace(req.CreatedBy)
	if agentID == "" {
		return model.AgentToolBinding{}, false, apperror.InvalidArgument("agent_id is required")
	}
	if toolID == "" {
		return model.AgentToolBinding{}, false, apperror.InvalidArgument("tool_id is required")
	}
	if createdBy == "" {
		return model.AgentToolBinding{}, false, apperror.InvalidArgument("created_by is required")
	}
	tool, err := l.repo.GetTool(ctx, toolID)
	if err != nil {
		return model.AgentToolBinding{}, false, err
	}
	if tool.Status != model.AgentToolStatusActive {
		return model.AgentToolBinding{}, false, apperror.InvalidArgument("only active tools can be bound to agents")
	}
	if tool.ToolType == model.AgentToolTypeMCP {
		server, err := l.repo.GetMCPServer(ctx, tool.MCPServerID)
		if err != nil {
			return model.AgentToolBinding{}, false, err
		}
		if !server.AdminConfigured || server.Status != model.AgentToolStatusActive {
			return model.AgentToolBinding{}, false, apperror.InvalidArgument("mcp tool server must be active and admin configured")
		}
	}

	return l.repo.BindTool(ctx, model.AgentToolBinding{
		AgentID:   agentID,
		ToolID:    toolID,
		CreatedBy: createdBy,
	})
}

func (l *AgentRegistryLogic) CanAgentUseTool(ctx context.Context, agentID string, toolID string) (bool, error) {
	agentID = strings.TrimSpace(agentID)
	toolID = strings.TrimSpace(toolID)
	if agentID == "" {
		return false, apperror.InvalidArgument("agent_id is required")
	}
	if toolID == "" {
		return false, apperror.InvalidArgument("tool_id is required")
	}
	tool, err := l.repo.GetTool(ctx, toolID)
	if err != nil {
		return false, err
	}
	if tool.Status != model.AgentToolStatusActive || !tool.AdminConfigured {
		return false, nil
	}
	if tool.ToolType == model.AgentToolTypeMCP {
		server, err := l.repo.GetMCPServer(ctx, tool.MCPServerID)
		if err != nil {
			return false, err
		}
		if server.Status != model.AgentToolStatusActive || !server.AdminConfigured {
			return false, nil
		}
	}
	if _, err := l.repo.GetToolBinding(ctx, agentID, toolID); err != nil {
		if appErr := apperror.From(err); appErr.Code == apperror.CodeNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (l *AgentRegistryLogic) RegisterSkill(ctx context.Context, req RegisterAgentSkillRequest) (model.AgentSkill, error) {
	name := strings.TrimSpace(req.Name)
	version := strings.TrimSpace(req.Version)
	objectKey := strings.TrimSpace(req.ObjectKey)
	sha256 := strings.ToLower(strings.TrimSpace(req.SHA256))
	contentType := strings.TrimSpace(req.ContentType)
	createdBy := strings.TrimSpace(req.CreatedBy)
	if name == "" {
		return model.AgentSkill{}, apperror.InvalidArgument("skill name is required")
	}
	if version == "" {
		return model.AgentSkill{}, apperror.InvalidArgument("skill version is required")
	}
	if objectKey == "" {
		return model.AgentSkill{}, apperror.InvalidArgument("skill object_key is required")
	}
	if !sha256Pattern.MatchString(sha256) {
		return model.AgentSkill{}, apperror.InvalidArgument("skill sha256 must be a lowercase hex sha256 digest")
	}
	if contentType == "" {
		return model.AgentSkill{}, apperror.InvalidArgument("skill content_type is required")
	}
	if req.SizeBytes <= 0 {
		return model.AgentSkill{}, apperror.InvalidArgument("skill size_bytes must be greater than 0")
	}
	if !validSkillStatus(req.Status) {
		return model.AgentSkill{}, apperror.InvalidArgument("skill status must be draft, active, or archived")
	}
	if createdBy == "" {
		return model.AgentSkill{}, apperror.InvalidArgument("created_by is required")
	}

	return l.repo.RegisterSkill(ctx, model.AgentSkill{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		Version:     version,
		ObjectKey:   objectKey,
		SHA256:      sha256,
		ContentType: contentType,
		SizeBytes:   req.SizeBytes,
		Status:      req.Status,
		CreatedBy:   createdBy,
	})
}

func (l *AgentRegistryLogic) BindSkill(ctx context.Context, req BindAgentSkillRequest) (model.AgentSkillBinding, bool, error) {
	agentID := strings.TrimSpace(req.AgentID)
	skillID := strings.TrimSpace(req.SkillID)
	createdBy := strings.TrimSpace(req.CreatedBy)
	if agentID == "" {
		return model.AgentSkillBinding{}, false, apperror.InvalidArgument("agent_id is required")
	}
	if skillID == "" {
		return model.AgentSkillBinding{}, false, apperror.InvalidArgument("skill_id is required")
	}
	if createdBy == "" {
		return model.AgentSkillBinding{}, false, apperror.InvalidArgument("created_by is required")
	}
	skill, err := l.repo.GetSkill(ctx, skillID)
	if err != nil {
		return model.AgentSkillBinding{}, false, err
	}
	if skill.Status != model.AgentSkillStatusActive {
		return model.AgentSkillBinding{}, false, apperror.InvalidArgument("only active skills can be bound to agents")
	}

	return l.repo.BindSkill(ctx, model.AgentSkillBinding{
		AgentID:   agentID,
		SkillID:   skillID,
		CreatedBy: createdBy,
	})
}

func (l *AgentRegistryLogic) validateToolShape(ctx context.Context, tool model.AgentTool) error {
	switch tool.ToolType {
	case model.AgentToolTypeMCP:
		if tool.MCPServerID == "" {
			return apperror.InvalidArgument("mcp tools require mcp_server_id")
		}
		if tool.MCPToolName == "" {
			return apperror.InvalidArgument("mcp tools require mcp_tool_name")
		}
		if tool.LocalHandlerKey != "" || tool.BuiltinKey != "" {
			return apperror.InvalidArgument("mcp tools cannot include local handler or builtin keys")
		}
		server, err := l.repo.GetMCPServer(ctx, tool.MCPServerID)
		if err != nil {
			return err
		}
		if !server.AdminConfigured {
			return apperror.InvalidArgument("mcp server must be admin configured")
		}
		if tool.Status == model.AgentToolStatusActive && server.Status != model.AgentToolStatusActive {
			return apperror.InvalidArgument("active mcp tools require an active mcp server")
		}
	case model.AgentToolTypeLocal:
		if tool.MCPServerID != "" || tool.MCPToolName != "" || tool.BuiltinKey != "" {
			return apperror.InvalidArgument("local tools can only use local handler metadata")
		}
		if !allowedLocalHandlerKey(tool.LocalHandlerKey) {
			return apperror.InvalidArgument("local handler_key is not whitelisted")
		}
	case model.AgentToolTypeBuiltin:
		if tool.MCPServerID != "" || tool.MCPToolName != "" || tool.LocalHandlerKey != "" {
			return apperror.InvalidArgument("builtin tools can only use builtin_key metadata")
		}
		if !allowedBuiltinKey(tool.BuiltinKey) {
			return apperror.InvalidArgument("builtin_key is not whitelisted")
		}
	default:
		return apperror.InvalidArgument("tool_type must be mcp, local, or builtin")
	}
	return nil
}

func normalizeJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}", nil
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return "", err
	}
	normalized, err := json.Marshal(decoded)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

func validateHTTPURL(raw string, field string) error {
	if raw == "" {
		return apperror.InvalidArgument(field + " is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return apperror.InvalidArgument(field + " must be a valid http or https URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return apperror.InvalidArgument(field + " must be a valid http or https URL")
	}
	return nil
}

func normalizePermissionLevel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "agent_bound"
	}
	return value
}

func validPromptStatus(status model.AgentPromptStatus) bool {
	switch status {
	case model.AgentPromptStatusDraft, model.AgentPromptStatusActive, model.AgentPromptStatusArchived:
		return true
	default:
		return false
	}
}

func validToolType(toolType model.AgentToolType) bool {
	switch toolType {
	case model.AgentToolTypeMCP, model.AgentToolTypeLocal, model.AgentToolTypeBuiltin:
		return true
	default:
		return false
	}
}

func validToolStatus(status model.AgentToolStatus) bool {
	switch status {
	case model.AgentToolStatusActive, model.AgentToolStatusDisabled, model.AgentToolStatusArchived:
		return true
	default:
		return false
	}
}

func validMCPTransport(transport model.AgentMCPTransport) bool {
	switch transport {
	case model.AgentMCPTransportHTTP, model.AgentMCPTransportSSE, model.AgentMCPTransportStreamableHTTP:
		return true
	default:
		return false
	}
}

func validSkillStatus(status model.AgentSkillStatus) bool {
	switch status {
	case model.AgentSkillStatusDraft, model.AgentSkillStatusActive, model.AgentSkillStatusArchived:
		return true
	default:
		return false
	}
}

func allowedLocalHandlerKey(key string) bool {
	switch key {
	case model.LocalToolHandlerGetConversationContext,
		model.LocalToolHandlerReadSkillFile,
		model.LocalToolHandlerSendAgentMessage,
		model.LocalToolHandlerPythonExecute:
		return true
	default:
		return false
	}
}

func allowedBuiltinKey(key string) bool {
	switch key {
	case model.BuiltinToolReadConversationContext,
		model.BuiltinToolReadSkillFile,
		model.BuiltinToolSendAgentMessage:
		return true
	default:
		return false
	}
}
