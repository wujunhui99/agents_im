package model

import "time"

type AgentPromptStatus string

const (
	AgentPromptStatusDraft    AgentPromptStatus = "draft"
	AgentPromptStatusActive   AgentPromptStatus = "active"
	AgentPromptStatusArchived AgentPromptStatus = "archived"
)

type AgentToolType string

const (
	AgentToolTypeMCP     AgentToolType = "mcp"
	AgentToolTypeLocal   AgentToolType = "local"
	AgentToolTypeBuiltin AgentToolType = "builtin"
)

type AgentToolStatus string

const (
	AgentToolStatusActive   AgentToolStatus = "active"
	AgentToolStatusDisabled AgentToolStatus = "disabled"
	AgentToolStatusArchived AgentToolStatus = "archived"
)

type AgentMCPTransport string

const (
	AgentMCPTransportHTTP           AgentMCPTransport = "http"
	AgentMCPTransportSSE            AgentMCPTransport = "sse"
	AgentMCPTransportStreamableHTTP AgentMCPTransport = "streamable_http"
)

const (
	LocalToolHandlerGetConversationContext = "im.get_conversation_context"
	LocalToolHandlerReadSkillFile          = "skill.read_file"
	LocalToolHandlerSendAgentMessage       = "im.send_agent_message"
	LocalToolHandlerPythonExecute          = "python.execute"
	LocalToolHandlerAgentCreate            = "agent.create"
)

const (
	BuiltinToolReadConversationContext = "im.get_conversation_context"
	BuiltinToolReadSkillFile           = "skill.read_file"
	BuiltinToolSendAgentMessage        = "im.send_agent_message"
)

type AgentSkillStatus string

const (
	AgentSkillStatusDraft    AgentSkillStatus = "draft"
	AgentSkillStatusActive   AgentSkillStatus = "active"
	AgentSkillStatusArchived AgentSkillStatus = "archived"
)

type AgentPrompt struct {
	PromptID            string
	Name                string
	Description         string
	Content             string
	VariablesSchemaJSON string
	Version             string
	Status              AgentPromptStatus
	CreatedBy           string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (p AgentPrompt) Clone() AgentPrompt {
	return p
}

type AgentMCPServer struct {
	ServerID         string
	Name             string
	Transport        AgentMCPTransport
	URL              string
	ConfigJSON       string
	HeadersSecretRef string
	TimeoutSeconds   int
	Status           AgentToolStatus
	AdminConfigured  bool
	CreatedBy        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (s AgentMCPServer) Clone() AgentMCPServer {
	return s
}

type AgentTool struct {
	ToolID           string
	Name             string
	Description      string
	ToolType         AgentToolType
	MCPServerID      string
	MCPToolName      string
	LocalHandlerKey  string
	BuiltinKey       string
	InputSchemaJSON  string
	OutputSchemaJSON string
	PermissionLevel  string
	Status           AgentToolStatus
	AdminConfigured  bool
	CreatedBy        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (t AgentTool) Clone() AgentTool {
	return t
}

type AgentSkill struct {
	SkillID     string
	Name        string
	Description string
	Version     string
	ObjectKey   string
	SHA256      string
	ContentType string
	SizeBytes   int64
	Status      AgentSkillStatus
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (s AgentSkill) Clone() AgentSkill {
	return s
}

type AgentPromptBinding struct {
	AgentID   string
	PromptID  string
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (b AgentPromptBinding) Clone() AgentPromptBinding {
	return b
}

type AgentToolBinding struct {
	AgentID   string
	ToolID    string
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (b AgentToolBinding) Clone() AgentToolBinding {
	return b
}

type AgentSkillBinding struct {
	AgentID   string
	SkillID   string
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (b AgentSkillBinding) Clone() AgentSkillBinding {
	return b
}
