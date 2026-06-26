package aihosting

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/pythonexec"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/convhosting"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/registrytest"
	runtimetools "github.com/wujunhui99/agents_im/service/agent/rpc/internal/runtime/tools"
)

func TestConfigureConversationAIHostingFailsOnMissingRequiredDependencies(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *ServiceContext
		wantErr string
	}{
		{
			name:    "nil context",
			ctx:     nil,
			wantErr: "message service context is not configured",
		},
		{
			name: "missing message logic",
			ctx: func() *ServiceContext {
				ctx := completeAIHostingServiceContext()
				ctx.MessageLogic = nil
				return ctx
			}(),
			wantErr: "message logic is not configured",
		},
		{
			name: "missing message repository",
			ctx: func() *ServiceContext {
				ctx := completeAIHostingServiceContext()
				ctx.MessageRepo = nil
				return ctx
			}(),
			wantErr: "message repository is not configured",
		},
		{
			name: "missing agent hosting repository",
			ctx: func() *ServiceContext {
				ctx := completeAIHostingServiceContext()
				ctx.AgentHostingRepo = nil
				return ctx
			}(),
			wantErr: "agent conversation hosting repository is not configured",
		},
		{
			name: "missing conversation AI hosting repository",
			ctx: func() *ServiceContext {
				ctx := completeAIHostingServiceContext()
				ctx.AIHostingStore = nil
				return ctx
			}(),
			wantErr: "conversation AI hosting store is not configured",
		},
		{
			name: "missing agent audit repository",
			ctx: func() *ServiceContext {
				ctx := completeAIHostingServiceContext()
				ctx.AgentAuditRepo = nil
				return ctx
			}(),
			wantErr: "agent audit repository is not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ConfigureConversationAIHosting(tt.ctx, config.DeepSeekConfig{}, config.LLMObservabilityConfig{})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ConfigureConversationAIHosting error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestConfigureConversationAIHostingWiresReadMarkerForDirectChatAIHosting(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")

	ctx := context.Background()
	messageRepo := repository.NewMemoryMessageRepository()
	serviceContext := NewServiceContextWithMediaValidator(
		messageRepo,
		nil,
		nil,
		nil,
		appconfig.DefaultJWTAuthConfig(),
	)
	if err := ConfigureConversationAIHosting(serviceContext, config.DeepSeekConfig{}, config.LLMObservabilityConfig{}); err != nil {
		t.Fatalf("configure conversation AI hosting: %v", err)
	}

	conversationID := repository.SingleConversationID("usr_hosted_owner", "usr_peer")
	if _, err := serviceContext.AIHostingStore.SetConversationAIHostingEnabled(ctx, convhosting.Update{
		OwnerAccountID:    "usr_hosted_owner",
		ConversationID:    conversationID,
		Enabled:           true,
		MaxRecentMessages: 30,
	}); err != nil {
		t.Fatalf("enable conversation AI hosting: %v", err)
	}

	trigger, err := serviceContext.MessageLogic.SendMessage(ctx, business.SendMessageRequest{
		SenderID:    "usr_peer",
		ReceiverID:  "usr_hosted_owner",
		ChatType:    business.MessageChatTypeSingle,
		ClientMsgID: "client-prod-config-read-marker",
		ContentType: business.MessageContentTypeText,
		Content:     "hello from peer",
	})
	if err != nil {
		t.Fatalf("send hosted trigger: %v", err)
	}

	seqs, err := serviceContext.MessageLogic.GetConversationSeqs(ctx, business.GetConversationSeqsRequest{
		UserID:          "usr_hosted_owner",
		ConversationIDs: []string{conversationID},
	})
	if err != nil {
		t.Fatalf("get hosted owner seqs: %v", err)
	}
	if len(seqs.States) != 1 {
		t.Fatalf("seq states = %+v, want one state", seqs.States)
	}
	state := seqs.States[0]
	if state.HasReadSeq != trigger.Message.Seq || state.UnreadCount != 0 {
		t.Fatalf("hosted owner read state = %+v, want hasReadSeq %d unread 0", state, trigger.Message.Seq)
	}

	waitForAgentAuditRuns(t, serviceContext.AgentAuditRepo, 1)
}

func TestConversationAIHostingToolProviderUsesConfiguredPythonExecutor(t *testing.T) {
	ctx := context.Background()
	registryRepo := registrytest.NewMemoryStore()
	if _, err := registryRepo.RegisterTool(ctx, model.AgentTool{
		ToolID:           "tool_python_execute",
		Name:             model.LocalToolHandlerPythonExecute,
		Description:      "Execute bounded Python code in the configured sandbox.",
		ToolType:         model.AgentToolTypeLocal,
		LocalHandlerKey:  model.LocalToolHandlerPythonExecute,
		InputSchemaJSON:  `{"type":"object","properties":{"code":{"type":"string"}},"required":["code"]}`,
		OutputSchemaJSON: `{"type":"object"}`,
		PermissionLevel:  "restricted",
		Status:           model.AgentToolStatusActive,
		AdminConfigured:  true,
		CreatedBy:        "agent_creator",
	}); err != nil {
		t.Fatalf("register python.execute tool: %v", err)
	}
	if _, _, err := registryRepo.BindTool(ctx, model.AgentToolBinding{
		AgentID:   "agent_default_assistant",
		ToolID:    "tool_python_execute",
		CreatedBy: "agent_creator",
	}); err != nil {
		t.Fatalf("bind python.execute tool: %v", err)
	}
	executor := &recordingPythonExecutor{
		resp: &pythonexec.Response{
			RunID:      "run_python",
			AuditID:    "req_python",
			Stdout:     "2\n",
			ResultJSON: []byte(`null`),
		},
	}

	provider, err := newConversationAIHostingToolProviderWithAgentCreate(registryRepo, executor, nil)
	if err != nil {
		t.Fatalf("build runtime tool provider: %v", err)
	}
	resolved, err := provider.ResolveTool(ctx, runtimetools.ResolveToolRequest{
		AgentID:         "agent_default_assistant",
		ToolID:          "tool_python_execute",
		RequireAdapters: true,
		RunID:           "run_python",
		RequestID:       "req_python",
	})
	if err != nil {
		t.Fatalf("resolve python.execute adapter: %v", err)
	}
	if !resolved.HasAdapter() {
		t.Fatal("resolved python.execute tool has no adapter")
	}
	result, err := resolved.Adapter.Invoke(ctx, runtimetools.ToolCall{
		RunID:     "run_python",
		AgentID:   "agent_default_assistant",
		ToolID:    "tool_python_execute",
		ToolName:  model.LocalToolHandlerPythonExecute,
		InputJSON: json.RawMessage(`{"code":"print(1 + 1)"}`),
		RequestID: "req_python",
	})
	if err != nil {
		t.Fatalf("invoke python.execute adapter: %v", err)
	}
	if !strings.Contains(string(result.OutputJSON), `"stdout":"2\n"`) {
		t.Fatalf("python.execute output = %s, want stdout from configured executor", result.OutputJSON)
	}
	if executor.calls != 1 || !strings.Contains(executor.lastReq.Code, "1 + 1") {
		t.Fatalf("configured executor was not called correctly: calls=%d req=%+v", executor.calls, executor.lastReq)
	}
}

func completeAIHostingServiceContext() *ServiceContext {
	messageRepo := repository.NewMemoryMessageRepository()
	agentAuditRepo := repository.NewMemoryAgentAuditRepository()
	aiHostingStore := convhosting.NewMemoryStore()
	return &ServiceContext{
		MessageLogic:     business.NewMessageLogicWithMediaValidator(messageRepo, nil, nil, nil),
		MessageRepo:      messageRepo,
		AgentHostingRepo: repository.NewMemoryAgentConversationHostingRepository(),
		AIHostingStore:   aiHostingStore,
		AIHostingLogic:   convhosting.NewConversationAIHostingLogic(aiHostingStore),
		AgentAuditRepo:   agentAuditRepo,
		AgentAuditLogic:  business.NewAgentAuditLogic(agentAuditRepo),
	}
}

type recordingPythonExecutor struct {
	calls   int
	lastReq pythonexec.Request
	resp    *pythonexec.Response
	err     error
}

func (e *recordingPythonExecutor) Execute(_ context.Context, req pythonexec.Request) (*pythonexec.Response, error) {
	e.calls++
	e.lastReq = req
	if e.err != nil {
		return nil, e.err
	}
	return e.resp, nil
}

func waitForAgentAuditRuns(t *testing.T, repo repository.AgentAuditRepository, want int64) {
	t.Helper()
	counter, ok := repo.(interface {
		CountAgentRuns(context.Context, string) (int64, error)
	})
	if !ok {
		return
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, err := counter.CountAgentRuns(context.Background(), "")
		if err == nil && got >= want {
			return
		}
		if time.Now().After(deadline) {
			if err != nil {
				t.Fatalf("count agent runs: %v", err)
			}
			t.Fatalf("timed out waiting for %d agent audit runs", want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
