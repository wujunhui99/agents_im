package eino

import (
	"context"
	"errors"
	"strings"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/wujunhui99/agents_im/internal/agentruntime"
	runtimetools "github.com/wujunhui99/agents_im/internal/agentruntime/tools"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/config"
	immodel "github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/pythonexec"
)

func TestDeepSeekRuntimeFailsClosedWhenProviderConfigMissing(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_MODEL", "")

	runtime := NewDeepSeekRuntime(config.DeepSeekConfig{})
	_, err := runtime.Run(context.Background(), agentruntime.RunRequest{
		RequestID:        "req_1",
		TriggerType:      agentruntime.TriggerTypeUserPrivateMessage,
		AgentUserID:      "usr_a",
		RequestingUserID: "usr_b",
		ConversationID:   "single:usr_a:usr_b",
		ConversationType: agentruntime.ConversationTypeSingle,
		TriggerMessageID: "msg_1",
		TriggerSeq:       1,
		PromptText:       "hello",
		Agent: agentruntime.AgentConfig{
			AgentID:     "ai-hosting:usr_a",
			AgentUserID: "usr_a",
			Status:      agentruntime.AgentStatusActive,
			Prompt: agentruntime.PromptRef{
				PromptID: "conversation-ai-hosting-v1",
				Content:  "Reply as the hosted user.",
			},
			Model: agentruntime.ModelConfig{
				Provider: "deepseek",
				Model:    config.DefaultDeepSeekModel,
			},
		},
	})
	if !errors.Is(err, config.ErrDeepSeekAPIKeyMissing) {
		t.Fatalf("runtime error = %v, want missing DeepSeek key", err)
	}
}

func TestDeepSeekRuntimeGeneratedRunIDIsPostgresAuditCompatible(t *testing.T) {
	ctx := context.Background()
	fakeModel := &scriptedToolCallingModel{responses: []*schema.Message{schema.AssistantMessage("ok", nil)}}
	runtime := NewDeepSeekRuntime(
		config.DeepSeekConfig{APIKey: "test-key", BaseURL: "https://deepseek.example.invalid", Model: "deepseek-test"},
		WithChatModelFactory(func(context.Context, config.DeepSeekConfig) (einomodel.ToolCallingChatModel, error) {
			return fakeModel, nil
		}),
	)
	req := validPythonToolRuntimeRequest()
	req.RunID = ""
	req.Agent.Tools = nil
	result, err := runtime.Run(ctx, req)
	if err != nil {
		t.Fatalf("run deepseek runtime: %v", err)
	}
	if result.RunID == "" || strings.Contains(result.RunID, "run_") {
		t.Fatalf("generated run_id = %q, want numeric database-compatible id", result.RunID)
	}
	for _, ch := range result.RunID {
		if ch < '0' || ch > '9' {
			t.Fatalf("generated run_id = %q, want digits only", result.RunID)
		}
	}
}

func TestRuntimeMessagesUsesPromptTextAsCurrentTaskWhenConversationContextExists(t *testing.T) {
	clearTask := "请总结一下这段日志的风险点。"
	req := agentruntime.RunRequest{
		RequestID:        "req_1",
		TriggerType:      agentruntime.TriggerTypeUserPrivateMessage,
		AgentUserID:      "usr_a",
		RequestingUserID: "usr_b",
		ConversationID:   "single:usr_a:usr_b",
		ConversationType: agentruntime.ConversationTypeSingle,
		TriggerMessageID: "msg_2",
		TriggerSeq:       2,
		PromptText:       clearTask,
		Agent: agentruntime.AgentConfig{
			AgentID:     "ai-hosting:usr_a",
			AgentUserID: "usr_a",
			Status:      agentruntime.AgentStatusActive,
			Prompt: agentruntime.PromptRef{
				PromptID: "conversation-ai-hosting-v1",
				Content:  "只输出要发送的回复文本。",
			},
			Model: agentruntime.ModelConfig{
				Provider: "deepseek",
				Model:    "deepseek-test",
			},
		},
		Conversation: []agentruntime.ConversationMessage{
			{
				ServerMsgID: "msg_1",
				Seq:         1,
				SenderID:    "usr_a",
				SenderType:  agentruntime.SenderTypeAgent,
				ContentType: agentruntime.ContentTypeText,
				Text:        "把日志发我。",
			},
			{
				ServerMsgID: "msg_2",
				Seq:         2,
				SenderID:    "usr_b",
				SenderType:  agentruntime.SenderTypeUser,
				ContentType: agentruntime.ContentTypeText,
				Text:        clearTask,
			},
		},
	}

	messages := runtimeMessages(req)
	if len(messages) < 3 {
		t.Fatalf("runtime messages = %+v, want system, prior context, and explicit current task", messages)
	}
	current := messages[len(messages)-1].Content
	for _, want := range []string{"当前需要回复的对方消息", clearTask, "直接回答或完成", "不要只回复"} {
		if !strings.Contains(current, want) {
			t.Fatalf("current task message missing %q: %q", want, current)
		}
	}
	if current == clearTask {
		t.Fatalf("current task should include direct-answer instructions, got only raw prompt_text")
	}
	for i, msg := range messages[:len(messages)-1] {
		if strings.Contains(msg.Content, "当前需要回复的对方消息") {
			t.Fatalf("current task instruction appeared before final message at index %d: %+v", i, messages)
		}
	}
}

func TestDeepSeekRuntimeExecutesPythonToolCallAndContinuesToFinalAnswer(t *testing.T) {
	ctx := context.Background()
	toolProvider := pythonExecuteToolProvider(t, ctx, &runtimeFakePythonExecutor{
		resp: &pythonexec.Response{
			RunID:      "run_python",
			AuditID:    "req_python",
			Stdout:     "2\n",
			ResultJSON: []byte(`null`),
		},
	})
	fakeModel := &scriptedToolCallingModel{
		responses: []*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{{
				ID:   "call_python_1",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "python_execute",
					Arguments: `{"code":"print(1 + 1)"}`,
				},
			}}),
			schema.AssistantMessage("计算结果是 2。", nil),
		},
	}
	runtime := NewDeepSeekRuntime(
		config.DeepSeekConfig{APIKey: "test-key", BaseURL: "https://deepseek.example.invalid", Model: "deepseek-test"},
		WithChatModelFactory(func(context.Context, config.DeepSeekConfig) (einomodel.ToolCallingChatModel, error) {
			return fakeModel, nil
		}),
		WithToolProvider(toolProvider),
	)

	result, err := runtime.Run(ctx, validPythonToolRuntimeRequest())
	if err != nil {
		t.Fatalf("run deepseek runtime with python tool call: %v", err)
	}
	if result.FinalText != "计算结果是 2。" {
		t.Fatalf("final text = %q, want model answer after tool result", result.FinalText)
	}
	if len(fakeModel.boundTools) != 1 || fakeModel.boundTools[0].Name != "python_execute" {
		t.Fatalf("bound tools = %+v, want DeepSeek-safe python_execute", fakeModel.boundTools)
	}
	if fakeModel.generateCalls != 2 {
		t.Fatalf("generate calls = %d, want tool call turn and final turn", fakeModel.generateCalls)
	}
	secondInput := fakeModel.inputs[1]
	if len(secondInput) < 2 {
		t.Fatalf("second model input = %+v, want assistant tool call and tool result appended", secondInput)
	}
	if got := secondInput[len(secondInput)-2]; got.Role != schema.Assistant || len(got.ToolCalls) != 1 {
		t.Fatalf("second model input missing assistant tool-call message: %+v", secondInput)
	}
	if got := secondInput[len(secondInput)-1]; got.Role != schema.Tool ||
		got.ToolCallID != "call_python_1" ||
		got.ToolName != "python_execute" ||
		!strings.Contains(got.Content, `"stdout":"2\n"`) {
		t.Fatalf("second model input missing python tool result: %+v", got)
	}
	if len(result.ToolCalls) != 1 ||
		result.ToolCalls[0].ToolName != immodel.LocalToolHandlerPythonExecute ||
		result.ToolCalls[0].Status != "succeeded" ||
		result.ToolCalls[0].DurationMs < 0 {
		t.Fatalf("runtime tool call result = %+v, want succeeded python.execute call", result.ToolCalls)
	}
}

func TestDeepSeekRuntimeExecutesAgentCreateWithSafeToolAlias(t *testing.T) {
	ctx := context.Background()
	var got runtimetools.AgentCreateRequest
	toolProvider := agentCreateToolProvider(t, ctx, runtimetools.AgentCreateHandlerFunc(func(_ context.Context, req runtimetools.AgentCreateRequest) (runtimetools.AgentCreateResponse, error) {
		got = req
		return runtimetools.AgentCreateResponse{
			AgentID:      "agent_created",
			AccountID:    "acct_created",
			Identifier:   "research_agent",
			Name:         req.Name,
			Description:  req.Description,
			PromptID:     "prompt_created",
			FriendUserID: req.RequestingUserID,
		}, nil
	}))
	fakeModel := &scriptedToolCallingModel{
		responses: []*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{{
				ID:   "call_agent_create_1",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "agent_create",
					Arguments: `{"name":"Research Agent","description":"Summarizes uploaded notes."}`,
				},
			}}),
			schema.AssistantMessage("已创建 Research Agent。", nil),
		},
	}
	runtime := NewDeepSeekRuntime(
		config.DeepSeekConfig{APIKey: "test-key", BaseURL: "https://deepseek.example.invalid", Model: "deepseek-test"},
		WithChatModelFactory(func(context.Context, config.DeepSeekConfig) (einomodel.ToolCallingChatModel, error) {
			return fakeModel, nil
		}),
		WithToolProvider(toolProvider),
	)

	req := validPythonToolRuntimeRequest()
	req.PromptText = "创建一个 Research Agent"
	req.Agent.Tools = []agentruntime.ToolRef{{
		ToolID:          "tool_agent_create",
		Name:            immodel.LocalToolHandlerAgentCreate,
		ToolType:        string(immodel.AgentToolTypeLocal),
		LocalHandlerKey: immodel.LocalToolHandlerAgentCreate,
	}}
	result, err := runtime.Run(ctx, req)
	if err != nil {
		t.Fatalf("run deepseek runtime with agent.create tool call: %v", err)
	}
	if result.FinalText != "已创建 Research Agent。" {
		t.Fatalf("final text = %q, want model answer after tool result", result.FinalText)
	}
	if len(fakeModel.boundTools) != 1 || fakeModel.boundTools[0].Name != "agent_create" {
		t.Fatalf("bound tools = %+v, want DeepSeek-safe agent_create", fakeModel.boundTools)
	}
	if got.CreatorAgentID != "agent_default_assistant" || got.RequestingUserID != "usr_user" {
		t.Fatalf("agent.create handler request ids = %+v, want creator agent and requesting user", got)
	}
	if got.Name != "Research Agent" || got.Description != "Summarizes uploaded notes." || got.SystemPrompt != "" {
		t.Fatalf("agent.create handler request content = %+v, want minimal intent", got)
	}
	if len(result.ToolCalls) != 1 ||
		result.ToolCalls[0].ToolName != immodel.LocalToolHandlerAgentCreate ||
		result.ToolCalls[0].Status != "succeeded" {
		t.Fatalf("runtime tool call result = %+v, want succeeded agent.create call", result.ToolCalls)
	}
}

func TestDeepSeekRuntimeContinuesWhenAgentCreateRejectsUnsupportedToolName(t *testing.T) {
	ctx := context.Background()
	toolProvider := agentCreateToolProvider(t, ctx, runtimetools.AgentCreateHandlerFunc(func(_ context.Context, req runtimetools.AgentCreateRequest) (runtimetools.AgentCreateResponse, error) {
		if len(req.ToolNames) != 1 || req.ToolNames[0] != "python_execute" {
			t.Fatalf("agent.create tool names = %+v, want unsafe python alias from model", req.ToolNames)
		}
		return runtimetools.AgentCreateResponse{}, apperror.NotFound("tool not found")
	}))
	fakeModel := &scriptedToolCallingModel{
		responses: []*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{{
				ID:   "call_agent_create_unsupported_tool",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "agent_create",
					Arguments: `{"name":"Python大师","description":"精通 Python 的专家","tool_names":["python_execute"]}`,
				},
			}}),
			schema.AssistantMessage("我不能给新 Agent 绑定 Python 执行工具，但可以先创建不带高风险工具的专家 Agent，或请管理员配置。", nil),
		},
	}
	runtime := NewDeepSeekRuntime(
		config.DeepSeekConfig{APIKey: "test-key", BaseURL: "https://deepseek.example.invalid", Model: "deepseek-test"},
		WithChatModelFactory(func(context.Context, config.DeepSeekConfig) (einomodel.ToolCallingChatModel, error) {
			return fakeModel, nil
		}),
		WithToolProvider(toolProvider),
	)

	req := validPythonToolRuntimeRequest()
	req.PromptText = "你好！"
	req.Agent.Tools = []agentruntime.ToolRef{{
		ToolID:          "tool_agent_create",
		Name:            immodel.LocalToolHandlerAgentCreate,
		ToolType:        string(immodel.AgentToolTypeLocal),
		LocalHandlerKey: immodel.LocalToolHandlerAgentCreate,
	}}
	result, err := runtime.Run(ctx, req)
	if err != nil {
		t.Fatalf("runtime should let model recover from rejected agent.create input: %v", err)
	}
	if !strings.Contains(result.FinalText, "不能给新 Agent 绑定 Python") {
		t.Fatalf("final text = %q, want model-visible tool rejection explanation", result.FinalText)
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].Status != "failed" || result.ToolCalls[0].ErrorCode != string(apperror.CodeNotFound) {
		t.Fatalf("tool call result = %+v, want failed NOT_FOUND recorded", result.ToolCalls)
	}
	secondInput := fakeModel.inputs[1]
	if got := secondInput[len(secondInput)-1]; got.Role != schema.Tool || !strings.Contains(got.Content, "tool not found") {
		t.Fatalf("second model input missing recoverable tool error: %+v", got)
	}
}

func TestDeepSeekRuntimeReturnsVisibleErrorWhenPythonExecutorDisabled(t *testing.T) {
	ctx := context.Background()
	fakeModel := &scriptedToolCallingModel{
		responses: []*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{{
				ID:   "call_python_disabled",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "python_execute",
					Arguments: `{"code":"print(1 + 1)"}`,
				},
			}}),
		},
	}
	runtime := NewDeepSeekRuntime(
		config.DeepSeekConfig{APIKey: "test-key", BaseURL: "https://deepseek.example.invalid", Model: "deepseek-test"},
		WithChatModelFactory(func(context.Context, config.DeepSeekConfig) (einomodel.ToolCallingChatModel, error) {
			return fakeModel, nil
		}),
		WithToolProvider(pythonExecuteToolProvider(t, ctx, pythonexec.NewDisabledExecutor())),
	)

	_, err := runtime.Run(ctx, validPythonToolRuntimeRequest())
	if err == nil || !strings.Contains(err.Error(), "python executor is disabled") {
		t.Fatalf("runtime error = %v, want visible disabled executor error", err)
	}
}

func TestDeepSeekRuntimeEnforcesMaxToolCalls(t *testing.T) {
	ctx := context.Background()
	runtimeReq := validPythonToolRuntimeRequest()
	runtimeReq.Agent.Policy.MaxToolCalls = 1
	fakeModel := &scriptedToolCallingModel{
		responses: []*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{{
				ID:   "call_python_1",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "python_execute",
					Arguments: `{"code":"print(1)"}`,
				},
			}}),
			schema.AssistantMessage("", []schema.ToolCall{{
				ID:   "call_python_2",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "python_execute",
					Arguments: `{"code":"print(2)"}`,
				},
			}}),
		},
	}
	runtime := NewDeepSeekRuntime(
		config.DeepSeekConfig{APIKey: "test-key", BaseURL: "https://deepseek.example.invalid", Model: "deepseek-test"},
		WithChatModelFactory(func(context.Context, config.DeepSeekConfig) (einomodel.ToolCallingChatModel, error) {
			return fakeModel, nil
		}),
		WithToolProvider(pythonExecuteToolProvider(t, ctx, &runtimeFakePythonExecutor{
			resp: &pythonexec.Response{
				RunID:      "run_python",
				AuditID:    "req_python",
				Stdout:     "ok\n",
				ResultJSON: []byte(`null`),
			},
		})),
	)

	_, err := runtime.Run(ctx, runtimeReq)
	if err == nil || !strings.Contains(err.Error(), "max tool calls") {
		t.Fatalf("runtime error = %v, want max tool calls enforcement", err)
	}
}

func pythonExecuteToolProvider(t *testing.T, ctx context.Context, executor pythonexec.Executor) runtimetools.Provider {
	t.Helper()
	repo := repository.NewMemoryAgentRegistryRepository()
	_, err := repo.RegisterTool(ctx, immodel.AgentTool{
		ToolID:           "tool_python_execute",
		Name:             immodel.LocalToolHandlerPythonExecute,
		Description:      "Execute bounded Python code in the configured sandbox.",
		ToolType:         immodel.AgentToolTypeLocal,
		LocalHandlerKey:  immodel.LocalToolHandlerPythonExecute,
		InputSchemaJSON:  `{"type":"object","properties":{"code":{"type":"string"},"timeout_seconds":{"type":"integer"},"files":{"type":"array","items":{"type":"string"}}},"required":["code"]}`,
		OutputSchemaJSON: `{"type":"object"}`,
		PermissionLevel:  "restricted",
		Status:           immodel.AgentToolStatusActive,
		AdminConfigured:  true,
		CreatedBy:        "agent_creator",
	})
	if err != nil {
		t.Fatalf("register python.execute tool: %v", err)
	}
	_, _, err = repo.BindTool(ctx, immodel.AgentToolBinding{
		AgentID:   "agent_default_assistant",
		ToolID:    "tool_python_execute",
		CreatedBy: "agent_creator",
	})
	if err != nil {
		t.Fatalf("bind python.execute tool: %v", err)
	}
	provider, err := runtimetools.NewResolver(repo, runtimetools.WithAdapterCatalog(runtimetools.NewDefaultLocalAdapterCatalog(executor)))
	if err != nil {
		t.Fatalf("new tool resolver: %v", err)
	}
	return provider
}

func agentCreateToolProvider(t *testing.T, ctx context.Context, handler runtimetools.AgentCreateHandler) runtimetools.Provider {
	t.Helper()
	repo := repository.NewMemoryAgentRegistryRepository()
	_, err := repo.RegisterTool(ctx, immodel.AgentTool{
		ToolID:           "tool_agent_create",
		Name:             immodel.LocalToolHandlerAgentCreate,
		Description:      "Create a new Agent through the server-side agent assembly workflow.",
		ToolType:         immodel.AgentToolTypeLocal,
		LocalHandlerKey:  immodel.LocalToolHandlerAgentCreate,
		InputSchemaJSON:  `{"type":"object","properties":{"name":{"type":"string"},"description":{"type":"string"},"system_prompt":{"type":"string"}},"required":["name","description"]}`,
		OutputSchemaJSON: `{"type":"object"}`,
		PermissionLevel:  "restricted",
		Status:           immodel.AgentToolStatusActive,
		AdminConfigured:  true,
		CreatedBy:        "agent_creator",
	})
	if err != nil {
		t.Fatalf("register agent.create tool: %v", err)
	}
	_, _, err = repo.BindTool(ctx, immodel.AgentToolBinding{
		AgentID:   "agent_default_assistant",
		ToolID:    "tool_agent_create",
		CreatedBy: "agent_creator",
	})
	if err != nil {
		t.Fatalf("bind agent.create tool: %v", err)
	}
	catalog := runtimetools.AdapterCatalogFunc(func(spec runtimetools.ToolSpec) (runtimetools.ToolAdapter, bool, error) {
		if !runtimetools.IsAgentCreateToolSpec(spec) {
			return nil, false, nil
		}
		adapter, err := runtimetools.NewAgentCreateAdapter(spec, handler)
		if err != nil {
			return nil, false, err
		}
		return adapter, true, nil
	})
	provider, err := runtimetools.NewResolver(repo, runtimetools.WithAdapterCatalog(catalog))
	if err != nil {
		t.Fatalf("new tool resolver: %v", err)
	}
	return provider
}

func validPythonToolRuntimeRequest() agentruntime.RunRequest {
	return agentruntime.RunRequest{
		RunID:            "run_python",
		RequestID:        "req_python",
		TriggerType:      agentruntime.TriggerTypeUserPrivateMessage,
		AgentUserID:      "agent_creator",
		RequestingUserID: "usr_user",
		ConversationID:   "single:agent_creator:usr_user",
		ConversationType: agentruntime.ConversationTypeSingle,
		TriggerMessageID: "msg_python",
		TriggerSeq:       1,
		PromptText:       "用 Python 算一下 1+1",
		Agent: agentruntime.AgentConfig{
			AgentID:     "agent_default_assistant",
			AgentUserID: "agent_creator",
			Name:        "agent_creator",
			Status:      agentruntime.AgentStatusActive,
			Prompt: agentruntime.PromptRef{
				PromptID: "prompt_default",
				Content:  "You may use approved tools when needed.",
			},
			Model: agentruntime.ModelConfig{
				Provider: "deepseek",
				Model:    "deepseek-test",
			},
			Policy: agentruntime.RuntimePolicy{
				MaxToolCalls: 4,
			},
		},
	}
}

type scriptedToolCallingModel struct {
	responses     []*schema.Message
	inputs        [][]*schema.Message
	boundTools    []*schema.ToolInfo
	generateCalls int
}

func (m *scriptedToolCallingModel) Generate(_ context.Context, input []*schema.Message, _ ...einomodel.Option) (*schema.Message, error) {
	m.generateCalls++
	copied := append([]*schema.Message(nil), input...)
	m.inputs = append(m.inputs, copied)
	if len(m.responses) == 0 {
		return nil, errors.New("no scripted model response")
	}
	resp := m.responses[0]
	m.responses = m.responses[1:]
	return resp, nil
}

func (m *scriptedToolCallingModel) Stream(context.Context, []*schema.Message, ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream is not implemented in scripted model")
}

func (m *scriptedToolCallingModel) WithTools(tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	m.boundTools = append([]*schema.ToolInfo(nil), tools...)
	return m, nil
}

type runtimeFakePythonExecutor struct {
	resp *pythonexec.Response
	err  error
}

func (e *runtimeFakePythonExecutor) Execute(_ context.Context, req pythonexec.Request) (*pythonexec.Response, error) {
	if e.err != nil {
		return nil, e.err
	}
	if e.resp != nil {
		resp := *e.resp
		resp.RunID = req.Policy.RunID
		resp.AuditID = req.Policy.AuditID
		return &resp, nil
	}
	return &pythonexec.Response{
		RunID:      req.Policy.RunID,
		AuditID:    req.Policy.AuditID,
		ResultJSON: []byte(`null`),
	}, nil
}
