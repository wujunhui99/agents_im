package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
)

func TestAgentCreateAdapterRequiresRequestingUser(t *testing.T) {
	adapter, err := NewAgentCreateAdapter(validAgentCreateToolSpec(), AgentCreateHandlerFunc(func(context.Context, AgentCreateRequest) (AgentCreateResponse, error) {
		t.Fatal("handler should not be called without requesting_user_id")
		return AgentCreateResponse{}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = adapter.Invoke(context.Background(), ToolCall{
		AgentID:   "agent_creator_profile",
		ToolID:    "tool_agent_create",
		ToolName:  model.LocalToolHandlerAgentCreate,
		InputJSON: json.RawMessage(`{"name":"Research Agent","system_prompt":"Help with research."}`),
	})
	assertAppErrorCode(t, err, apperror.CodeForbidden)
}

func TestAgentCreateAdapterCallsBusinessHandler(t *testing.T) {
	var got AgentCreateRequest
	adapter, err := NewAgentCreateAdapter(validAgentCreateToolSpec(), AgentCreateHandlerFunc(func(_ context.Context, req AgentCreateRequest) (AgentCreateResponse, error) {
		got = req
		return AgentCreateResponse{
			AgentID:      "agent_123",
			AccountID:    "acct_123",
			Identifier:   "research_agent",
			Name:         req.Name,
			PromptID:     "prompt_123",
			ToolNames:    req.ToolNames,
			FriendUserID: req.RequestingUserID,
		}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	result, err := adapter.Invoke(context.Background(), ToolCall{
		RunID:            "run_agent_create",
		AgentID:          "agent_creator_profile",
		RequestingUserID: "usr_requester",
		ToolID:           "tool_agent_create",
		ToolName:         model.LocalToolHandlerAgentCreate,
		InputJSON:        json.RawMessage(`{"identifier":"research_agent","name":"Research Agent","system_prompt":"Help with research.","tool_names":["im.get_conversation_context"]}`),
		RequestID:        "req_agent_create",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.RequestingUserID != "usr_requester" || got.Identifier != "research_agent" || got.Name != "Research Agent" {
		t.Fatalf("handler request mismatch: %+v", got)
	}
	var output AgentCreateResponse
	if err := json.Unmarshal(result.OutputJSON, &output); err != nil {
		t.Fatal(err)
	}
	if output.AgentID != "agent_123" || output.FriendUserID != "usr_requester" || len(output.ToolNames) != 1 {
		t.Fatalf("adapter output mismatch: %+v", output)
	}
}

func TestAgentCreateAdapterAcceptsMinimalIntentAndPassesCreatorAgent(t *testing.T) {
	var got AgentCreateRequest
	adapter, err := NewAgentCreateAdapter(validAgentCreateToolSpec(), AgentCreateHandlerFunc(func(_ context.Context, req AgentCreateRequest) (AgentCreateResponse, error) {
		got = req
		return AgentCreateResponse{
			AgentID:      "agent_123",
			AccountID:    "acct_123",
			Identifier:   "research_agent",
			Name:         req.Name,
			Description:  req.Description,
			PromptID:     "prompt_123",
			FriendUserID: req.RequestingUserID,
		}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	result, err := adapter.Invoke(context.Background(), ToolCall{
		RunID:            "run_agent_create",
		AgentID:          "agent_default_assistant",
		RequestingUserID: "usr_requester",
		ToolID:           "tool_agent_create",
		ToolName:         model.LocalToolHandlerAgentCreate,
		InputJSON:        json.RawMessage(`{"name":"Research Agent","description":"Summarizes uploaded notes and recent chat context."}`),
		RequestID:        "req_agent_create",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.CreatorAgentID != "agent_default_assistant" {
		t.Fatalf("creator agent id = %q, want agent_default_assistant", got.CreatorAgentID)
	}
	if got.RequestingUserID != "usr_requester" || got.Name != "Research Agent" || got.Description == "" {
		t.Fatalf("handler request mismatch: %+v", got)
	}
	if got.SystemPrompt != "" {
		t.Fatalf("adapter should allow service-generated prompt, got %q", got.SystemPrompt)
	}
	if len(result.OutputJSON) == 0 {
		t.Fatal("adapter returned empty output")
	}
}

func validAgentCreateToolSpec() ToolSpec {
	return ToolSpec{
		ToolID:          "tool_agent_create",
		Name:            model.LocalToolHandlerAgentCreate,
		ToolType:        model.AgentToolTypeLocal,
		PermissionLevel: "restricted",
		Local: &LocalToolSpec{
			HandlerKey: model.LocalToolHandlerAgentCreate,
		},
	}
}
