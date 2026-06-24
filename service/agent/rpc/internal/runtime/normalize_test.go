package runtime

import (
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

func TestNormalizeRunRequestPreservesTriggerAndConfigSnapshots(t *testing.T) {
	req := validRunRequest()
	req.RequestID = " req_1 "
	req.TriggerType = " GROUP_MENTION "
	req.AgentUserID = " agent_user_1 "
	req.ConversationType = " GROUP "
	req.TargetAgentUserIDs = []string{" agent_user_1 ", "agent_user_1", "", "agent_user_2"}
	req.Metadata = map[string]string{" trace_key ": " trace_value ", "": "ignored"}
	req.Agent.AgentID = " agent_profile_1 "
	req.Agent.AgentUserID = " agent_user_1 "
	req.Agent.Status = " ACTIVE "
	req.Agent.Model.Metadata = map[string]string{" provider_key ": " provider_value "}
	req.Conversation = []ConversationMessage{{
		ServerMsgID: " msg_1 ",
		Seq:         42,
		SenderID:    " user_1 ",
		SenderType:  " USER ",
		ContentType: " TEXT ",
		Text:        "hello agent",
		CreatedAtMs: 1000,
	}}

	normalized, err := NormalizeRunRequest(req)
	if err != nil {
		t.Fatalf("normalize run request: %v", err)
	}

	if normalized.RequestID != "req_1" {
		t.Fatalf("request_id = %q", normalized.RequestID)
	}
	if normalized.TriggerType != TriggerTypeGroupMention {
		t.Fatalf("trigger_type = %q", normalized.TriggerType)
	}
	if normalized.AgentUserID != "agent_user_1" || normalized.Agent.AgentID != "agent_profile_1" {
		t.Fatalf("agent identifiers were not normalized: %+v", normalized)
	}
	if normalized.ConversationType != ConversationTypeGroup {
		t.Fatalf("conversation_type = %q", normalized.ConversationType)
	}
	if len(normalized.TargetAgentUserIDs) != 2 || normalized.TargetAgentUserIDs[0] != "agent_user_1" || normalized.TargetAgentUserIDs[1] != "agent_user_2" {
		t.Fatalf("target agent ids not normalized: %#v", normalized.TargetAgentUserIDs)
	}
	if normalized.Metadata["trace_key"] != "trace_value" {
		t.Fatalf("metadata not normalized: %#v", normalized.Metadata)
	}
	if normalized.Agent.Model.Metadata["provider_key"] != "provider_value" {
		t.Fatalf("model metadata not normalized: %#v", normalized.Agent.Model.Metadata)
	}
	if normalized.Conversation[0].SenderID != "user_1" || normalized.Conversation[0].ContentType != ContentTypeText {
		t.Fatalf("conversation message not normalized: %+v", normalized.Conversation[0])
	}

	req.Metadata[" trace_key "] = "mutated"
	req.Agent.Model.Metadata[" provider_key "] = "mutated"
	if normalized.Metadata["trace_key"] != "trace_value" || normalized.Agent.Model.Metadata["provider_key"] != "provider_value" {
		t.Fatal("normalized request aliases caller-owned maps")
	}
}

func TestNormalizeRunRequestRejectsEmptyPromptAndAgentIdentifiers(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RunRequest)
	}{
		{
			name: "missing request agent user id",
			mutate: func(req *RunRequest) {
				req.AgentUserID = " "
			},
		},
		{
			name: "missing config agent id",
			mutate: func(req *RunRequest) {
				req.Agent.AgentID = " "
			},
		},
		{
			name: "missing config agent user id",
			mutate: func(req *RunRequest) {
				req.Agent.AgentUserID = " "
			},
		},
		{
			name: "mismatched agent user id",
			mutate: func(req *RunRequest) {
				req.Agent.AgentUserID = "agent_user_2"
			},
		},
		{
			name: "missing prompt id",
			mutate: func(req *RunRequest) {
				req.Agent.Prompt.PromptID = " "
			},
		},
		{
			name: "missing system prompt content",
			mutate: func(req *RunRequest) {
				req.Agent.Prompt.Content = " "
			},
		},
		{
			name: "missing trigger prompt text",
			mutate: func(req *RunRequest) {
				req.PromptText = " "
			},
		},
		{
			name: "missing model provider",
			mutate: func(req *RunRequest) {
				req.Agent.Model.Provider = " "
			},
		},
		{
			name: "missing model name",
			mutate: func(req *RunRequest) {
				req.Agent.Model.Model = " "
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validRunRequest()
			tc.mutate(&req)
			_, err := NormalizeRunRequest(req)
			assertInvalidArgument(t, err)
		})
	}
}

func TestNormalizeRunRequestRejectsUnsafeToolTypesAndInvalidToolShapes(t *testing.T) {
	tests := []struct {
		name string
		tool ToolRef
	}{
		{
			name: "shell tool type",
			tool: ToolRef{
				ToolID:   "tool_shell",
				Name:     "shell",
				ToolType: "shell",
			},
		},
		{
			name: "mcp missing server",
			tool: ToolRef{
				ToolID:      "tool_mcp",
				Name:        "calendar.search",
				ToolType:    ToolTypeMCP,
				MCPToolName: "calendar.search",
			},
		},
		{
			name: "local includes mcp metadata",
			tool: ToolRef{
				ToolID:          "tool_local",
				Name:            "skill.read_file",
				ToolType:        ToolTypeLocal,
				LocalHandlerKey: "skill.read_file",
				MCPServerID:     "mcp_srv_1",
			},
		},
		{
			name: "builtin missing key",
			tool: ToolRef{
				ToolID:   "tool_builtin",
				Name:     "skill.read_file",
				ToolType: ToolTypeBuiltin,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validRunRequest()
			req.Agent.Tools = []ToolRef{tc.tool}
			_, err := NormalizeRunRequest(req)
			assertInvalidArgument(t, err)
		})
	}
}

func TestNormalizeRunResultValidatesRequiredFieldsAndUsage(t *testing.T) {
	startedAt := time.Date(2026, 4, 30, 1, 2, 3, 0, time.FixedZone("UTC+8", 8*60*60))
	result := RunResult{
		RunID:     " run_1 ",
		FinalText: "done",
		Model: ModelMetadata{
			Provider: " deepseek ",
			Model:    " deepseek-v4-pro ",
			Metadata: map[string]string{" response_tier ": " standard "},
		},
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
		ToolCalls: []ToolCallResult{{
			ToolCallID: " call_1 ",
			ToolID:     " tool_1 ",
			ToolName:   " skill.read_file ",
			DurationMs: 12,
			Metadata:   map[string]string{" tool_key ": " tool_value "},
		}},
		StartedAt: startedAt,
	}

	normalized, err := NormalizeRunResult(result)
	if err != nil {
		t.Fatalf("normalize run result: %v", err)
	}
	if normalized.RunID != "run_1" || normalized.FinalText != "done" {
		t.Fatalf("result required fields not normalized: %+v", normalized)
	}
	if normalized.Model.Provider != "deepseek" || normalized.Model.Metadata["response_tier"] != "standard" {
		t.Fatalf("model metadata not normalized: %+v", normalized.Model)
	}
	if normalized.ToolCalls[0].ToolName != "skill.read_file" || normalized.ToolCalls[0].Metadata["tool_key"] != "tool_value" {
		t.Fatalf("tool call result not normalized: %+v", normalized.ToolCalls[0])
	}
	if normalized.StartedAt.Location() != time.UTC {
		t.Fatalf("started_at not normalized to UTC: %v", normalized.StartedAt)
	}

	invalidResults := []RunResult{
		{RunID: "", FinalText: "done"},
		{RunID: "run_1", FinalText: " "},
		{RunID: "run_1", FinalText: "done", Usage: Usage{TotalTokens: -1}},
		{RunID: "run_1", FinalText: "done", ToolCalls: []ToolCallResult{{DurationMs: -1}}},
	}
	for _, invalid := range invalidResults {
		_, err := NormalizeRunResult(invalid)
		assertInvalidArgument(t, err)
	}
}

func validRunRequest() RunRequest {
	return RunRequest{
		RequestID:        "req_1",
		EventID:          "evt_1",
		OperationID:      "op_1",
		TraceID:          "trace_1",
		TriggerType:      TriggerTypeUserPrivateMessage,
		AgentUserID:      "agent_user_1",
		RequestingUserID: "user_1",
		ConversationID:   "single:agent_user_1:user_1",
		ConversationType: ConversationTypeSingle,
		TriggerMessageID: "msg_1",
		TriggerSeq:       10,
		PromptText:       "hello",
		ReplyToMessageID: "msg_1",
		Agent: AgentConfig{
			AgentID:     "agent_profile_1",
			AgentUserID: "agent_user_1",
			Name:        "Support Agent",
			Status:      AgentStatusActive,
			Prompt: PromptRef{
				PromptID: "prompt_1",
				Content:  "Answer using the configured support policy.",
				Version:  "v1",
			},
			Model: ModelConfig{
				Provider: "deepseek",
				Model:    "deepseek-v4-pro",
			},
			Tools: []ToolRef{{
				ToolID:     "tool_1",
				Name:       "skill.read_file",
				ToolType:   ToolTypeBuiltin,
				BuiltinKey: "skill.read_file",
			}},
			Skills: []SkillRef{{
				SkillID:     "skill_1",
				Name:        "Support Skill",
				Version:     "v1",
				ObjectKey:   "skills/support/v1/SKILL.md",
				SHA256:      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
				ContentType: "text/markdown",
				SizeBytes:   128,
			}},
			Policy: RuntimePolicy{
				MaxToolCalls:                   3,
				MaxRunDuration:                 time.Minute,
				RequireMessageServiceWriteback: true,
			},
		},
	}
}

func assertInvalidArgument(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected invalid argument error")
	}
	if appErr := apperror.From(err); appErr.Code != apperror.CodeInvalidArgument {
		t.Fatalf("expected invalid argument error, got %v", err)
	}
}
