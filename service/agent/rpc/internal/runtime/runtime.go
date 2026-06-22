package runtime

import (
	"context"
	"time"
)

const (
	ConversationTypeSingle = "single"
	ConversationTypeGroup  = "group"

	SenderTypeUser  = "user"
	SenderTypeAgent = "agent"

	ContentTypeText = "text"

	TriggerTypeUserPrivateMessage = "user_private_message"
	TriggerTypeGroupMention       = "group_mention"
	TriggerTypeAdminManualRun     = "admin_manual_run"

	AgentStatusDraft    = "draft"
	AgentStatusActive   = "active"
	AgentStatusDisabled = "disabled"
	AgentStatusArchived = "archived"

	ToolTypeMCP     = "mcp"
	ToolTypeLocal   = "local"
	ToolTypeBuiltin = "builtin"
)

type Runtime interface {
	Run(ctx context.Context, req RunRequest) (RunResult, error)
}

type RuntimeFunc func(ctx context.Context, req RunRequest) (RunResult, error)

func (f RuntimeFunc) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	return f(ctx, req)
}

type RunRequest struct {
	RunID              string                `json:"run_id,omitempty"`
	RequestID          string                `json:"request_id"`
	EventID            string                `json:"event_id,omitempty"`
	OperationID        string                `json:"operation_id,omitempty"`
	TraceID            string                `json:"trace_id,omitempty"`
	TriggerType        string                `json:"trigger_type"`
	AgentUserID        string                `json:"agent_user_id"`
	RequestingUserID   string                `json:"requesting_user_id"`
	ConversationID     string                `json:"conversation_id"`
	ConversationType   string                `json:"conversation_type"`
	TriggerMessageID   string                `json:"trigger_message_id,omitempty"`
	TriggerSeq         int64                 `json:"trigger_seq,omitempty"`
	PromptText         string                `json:"prompt_text"`
	ReplyToMessageID   string                `json:"reply_to_message_id,omitempty"`
	RecursiveTrigger   bool                  `json:"recursive_trigger,omitempty"`
	SourceAgentRunID   string                `json:"source_agent_run_id,omitempty"`
	SourceAgentUserID  string                `json:"source_agent_user_id,omitempty"`
	SourceMessageID    string                `json:"source_message_id,omitempty"`
	SourceMessageSeq   int64                 `json:"source_message_seq,omitempty"`
	SourceMessageText  string                `json:"source_message_text,omitempty"`
	SourceContentType  string                `json:"source_content_type,omitempty"`
	TargetAgentUserIDs []string              `json:"target_agent_user_ids,omitempty"`
	Agent              AgentConfig           `json:"agent"`
	Conversation       []ConversationMessage `json:"conversation,omitempty"`
	Metadata           map[string]string     `json:"metadata,omitempty"`
}

type AgentConfig struct {
	AgentID     string        `json:"agent_id"`
	AgentUserID string        `json:"agent_user_id"`
	Name        string        `json:"name,omitempty"`
	Description string        `json:"description,omitempty"`
	Status      string        `json:"status,omitempty"`
	Prompt      PromptRef     `json:"prompt"`
	Model       ModelConfig   `json:"model"`
	Tools       []ToolRef     `json:"tools,omitempty"`
	Skills      []SkillRef    `json:"skills,omitempty"`
	Policy      RuntimePolicy `json:"policy,omitempty"`
}

type PromptRef struct {
	PromptID            string `json:"prompt_id"`
	Name                string `json:"name,omitempty"`
	Description         string `json:"description,omitempty"`
	Content             string `json:"content"`
	Version             string `json:"version,omitempty"`
	VariablesSchemaJSON string `json:"variables_schema_json,omitempty"`
}

type ModelConfig struct {
	Provider      string            `json:"provider"`
	Model         string            `json:"model"`
	ModelVersion  string            `json:"model_version,omitempty"`
	BaseURL       string            `json:"base_url,omitempty"`
	CredentialRef string            `json:"credential_ref,omitempty"`
	Temperature   *float64          `json:"temperature,omitempty"`
	MaxTokens     int               `json:"max_tokens,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type ToolRef struct {
	ToolID           string `json:"tool_id"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	ToolType         string `json:"tool_type"`
	MCPServerID      string `json:"mcp_server_id,omitempty"`
	MCPToolName      string `json:"mcp_tool_name,omitempty"`
	LocalHandlerKey  string `json:"local_handler_key,omitempty"`
	BuiltinKey       string `json:"builtin_key,omitempty"`
	InputSchemaJSON  string `json:"input_schema_json,omitempty"`
	OutputSchemaJSON string `json:"output_schema_json,omitempty"`
	PermissionLevel  string `json:"permission_level,omitempty"`
}

type SkillRef struct {
	SkillID     string `json:"skill_id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`
	ObjectKey   string `json:"object_key,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
}

type RuntimePolicy struct {
	MaxToolCalls                   int           `json:"max_tool_calls,omitempty"`
	MaxRunDuration                 time.Duration `json:"max_run_duration,omitempty"`
	MaxRecursiveDepth              int           `json:"max_recursive_depth,omitempty"`
	AllowAgentMessageRecursion     bool          `json:"allow_agent_message_recursion,omitempty"`
	RequireMessageServiceWriteback bool          `json:"require_message_service_writeback,omitempty"`
}

type ConversationMessage struct {
	ServerMsgID string `json:"server_msg_id,omitempty"`
	Seq         int64  `json:"seq,omitempty"`
	SenderID    string `json:"sender_id"`
	SenderType  string `json:"sender_type"`
	ContentType string `json:"content_type"`
	Text        string `json:"text"`
	AgentRunID  string `json:"agent_run_id,omitempty"`
	CreatedAtMs int64  `json:"created_at_ms,omitempty"`
}

type RunResult struct {
	RunID        string            `json:"run_id"`
	FinalText    string            `json:"final_text"`
	Model        ModelMetadata     `json:"model,omitempty"`
	Usage        Usage             `json:"usage,omitempty"`
	FinishReason string            `json:"finish_reason,omitempty"`
	ToolCalls    []ToolCallResult  `json:"tool_calls,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	StartedAt    time.Time         `json:"started_at,omitempty"`
	FinishedAt   time.Time         `json:"finished_at,omitempty"`
}

type ModelMetadata struct {
	Provider     string            `json:"provider,omitempty"`
	Model        string            `json:"model,omitempty"`
	ModelVersion string            `json:"model_version,omitempty"`
	ResponseID   string            `json:"response_id,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type Usage struct {
	PromptTokens     int64 `json:"prompt_tokens,omitempty"`
	CompletionTokens int64 `json:"completion_tokens,omitempty"`
	ReasoningTokens  int64 `json:"reasoning_tokens,omitempty"`
	CachedTokens     int64 `json:"cached_tokens,omitempty"`
	TotalTokens      int64 `json:"total_tokens,omitempty"`
}

type ToolCallResult struct {
	ToolCallID   string            `json:"tool_call_id,omitempty"`
	ToolID       string            `json:"tool_id,omitempty"`
	ToolName     string            `json:"tool_name,omitempty"`
	Status       string            `json:"status,omitempty"`
	ErrorCode    string            `json:"error_code,omitempty"`
	ErrorMessage string            `json:"error_message,omitempty"`
	DurationMs   int64             `json:"duration_ms,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}
