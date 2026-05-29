package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
)

type AgentCreateHandler interface {
	CreateAgent(ctx context.Context, req AgentCreateRequest) (AgentCreateResponse, error)
}

type AgentCreateHandlerFunc func(ctx context.Context, req AgentCreateRequest) (AgentCreateResponse, error)

func (f AgentCreateHandlerFunc) CreateAgent(ctx context.Context, req AgentCreateRequest) (AgentCreateResponse, error) {
	if f == nil {
		return AgentCreateResponse{}, apperror.Internal("agent.create handler is not configured")
	}
	return f(ctx, req)
}

type AgentCreateRequest struct {
	CreatorAgentID   string   `json:"-"`
	RequestingUserID string   `json:"-"`
	Identifier       string   `json:"identifier,omitempty"`
	Name             string   `json:"name"`
	Description      string   `json:"description,omitempty"`
	SystemPrompt     string   `json:"system_prompt"`
	ToolNames        []string `json:"tool_names,omitempty"`
}

type AgentCreateResponse struct {
	AgentID      string   `json:"agent_id"`
	AccountID    string   `json:"account_id"`
	Identifier   string   `json:"identifier"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	PromptID     string   `json:"prompt_id"`
	ToolNames    []string `json:"tool_names"`
	FriendUserID string   `json:"friend_user_id"`
}

type AgentCreateAdapter struct {
	spec    ToolSpec
	handler AgentCreateHandler
}

func NewAgentCreateAdapter(spec ToolSpec, handler AgentCreateHandler) (*AgentCreateAdapter, error) {
	if !IsAgentCreateToolSpec(spec) {
		return nil, apperror.InvalidArgument("agent create adapter requires a local agent.create tool spec")
	}
	if handler == nil {
		return nil, apperror.Internal("agent.create handler is not configured")
	}
	return &AgentCreateAdapter{spec: spec, handler: handler}, nil
}

func IsAgentCreateToolSpec(spec ToolSpec) bool {
	return spec.ToolType == model.AgentToolTypeLocal &&
		spec.Local != nil &&
		strings.TrimSpace(spec.Local.HandlerKey) == model.LocalToolHandlerAgentCreate
}

func (a *AgentCreateAdapter) Spec() ToolSpec {
	if a == nil {
		return ToolSpec{}
	}
	return a.spec
}

func (a *AgentCreateAdapter) Invoke(ctx context.Context, call ToolCall) (ToolResult, error) {
	if a == nil {
		return ToolResult{}, apperror.Internal("agent.create adapter is nil")
	}
	if ctx == nil {
		return ToolResult{}, apperror.InvalidArgument("context is required")
	}
	if strings.TrimSpace(call.ToolID) != a.spec.ToolID {
		return ToolResult{}, apperror.InvalidArgument("tool_id does not match agent.create adapter")
	}
	creatorAgentID := strings.TrimSpace(call.AgentID)
	if creatorAgentID == "" {
		return ToolResult{}, apperror.InvalidArgument("agent.create requires agent_id")
	}
	requestingUserID := strings.TrimSpace(call.RequestingUserID)
	if requestingUserID == "" {
		return ToolResult{}, apperror.Forbidden("agent.create requires requesting_user_id")
	}
	input, err := decodeAgentCreateInput(call.InputJSON)
	if err != nil {
		return ToolResult{}, err
	}
	input.CreatorAgentID = creatorAgentID
	input.RequestingUserID = requestingUserID
	resp, err := a.handler.CreateAgent(ctx, input)
	if err != nil {
		return ToolResult{}, err
	}
	output, err := json.Marshal(resp)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{OutputJSON: output}, nil
}

func decodeAgentCreateInput(raw json.RawMessage) (AgentCreateRequest, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return AgentCreateRequest{}, apperror.InvalidArgument("input_json is required")
	}
	var input AgentCreateRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return AgentCreateRequest{}, apperror.InvalidArgument("agent.create input_json is invalid: " + err.Error())
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return AgentCreateRequest{}, apperror.InvalidArgument("agent.create input_json must contain a single JSON object")
	}
	input.Identifier = strings.TrimSpace(input.Identifier)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.SystemPrompt = strings.TrimSpace(input.SystemPrompt)
	for i := range input.ToolNames {
		input.ToolNames[i] = strings.TrimSpace(input.ToolNames[i])
	}
	return input, nil
}
