package aihosting

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/pythonexec"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/agaudit"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/aghosting"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/convhosting"
	agentim "github.com/wujunhui99/agents_im/service/agent/rpc/internal/orchestrator"
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
			name: "missing message history reader",
			ctx: func() *ServiceContext {
				ctx := completeAIHostingServiceContext()
				ctx.MessageHistory = nil
				return ctx
			}(),
			wantErr: "message history reader is not configured",
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
				ctx.AgentAudit = nil
				return ctx
			}(),
			wantErr: "agent audit store is not configured",
		},
		{
			name: "missing agent response sender",
			ctx: func() *ServiceContext {
				ctx := completeAIHostingServiceContext()
				ctx.AgentResponseSender = nil
				return ctx
			}(),
			wantErr: "agent response sender is not configured",
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

// TestConfigureConversationAIHostingWiresReadMarkerForDirectChatAIHosting 验证 Configure 装配出的
// HostingService 对直聊 AI 托管触发：先经注入的 gRPC 读端口（ReadAdvancer）同步推进已读，再异步跑 run
// （#617：已读经 msg-rpc MarkConversationAsRead，本测用 fake 替身）。
func TestConfigureConversationAIHostingWiresReadMarkerForDirectChatAIHosting(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")

	ctx := context.Background()
	conversationID := singleTestConvID("usr_hosted_owner", "usr_peer")
	history := &fakeMessageHistory{}
	reads := newFakeReadAdvancer()
	serviceContext := NewServiceContext(appconfig.DefaultJWTAuthConfig())
	serviceContext.MessageHistory = history
	serviceContext.ReadAdvancer = reads
	serviceContext.AgentResponseSender = &fakeSender{}
	if err := ConfigureConversationAIHosting(serviceContext, config.DeepSeekConfig{}, config.LLMObservabilityConfig{}); err != nil {
		t.Fatalf("configure conversation AI hosting: %v", err)
	}

	if _, err := serviceContext.AIHostingStore.SetConversationAIHostingEnabled(ctx, convhosting.Update{
		OwnerAccountID:    "usr_hosted_owner",
		ConversationID:    conversationID,
		Enabled:           true,
		MaxRecentMessages: 30,
	}); err != nil {
		t.Fatalf("enable conversation AI hosting: %v", err)
	}

	trigger := agentim.Message{
		ServerMsgID:    "srv_hosted_trigger",
		ClientMsgID:    "client-prod-config-read-marker",
		ConversationID: conversationID,
		Seq:            7,
		SenderID:       "usr_peer",
		ReceiverID:     "usr_hosted_owner",
		ChatType:       agentim.MessageChatTypeSingle,
		ContentType:    agentim.MessageContentTypeText,
		Content:        "hello from peer",
		MessageOrigin:  agentim.MessageOriginHuman,
	}
	if _, err := serviceContext.HostingService.HandleMessageCreated(ctx, agentim.ConversationHostingMessageCreatedInput{
		EventID: "evt_prod_config_read_marker",
		Message: trigger,
	}); err != nil {
		t.Fatalf("handle hosted trigger: %v", err)
	}

	// 已读在跑 run 前就应同步推进到触发消息 seq（经注入的 ReadAdvancer）。
	if got := reads.readSeq("usr_hosted_owner", conversationID); got != trigger.Seq {
		t.Fatalf("hosted owner read seq = %d, want %d", got, trigger.Seq)
	}
	// run 被调度并落审计（DEEPSEEK_API_KEY 缺失下会失败，但审计仍记录一条 run）。
	waitForAgentAuditRuns(t, serviceContext.AgentAudit, 1)
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
	aiHostingStore := convhosting.NewMemoryStore()
	return &ServiceContext{
		MessageHistory:      &fakeMessageHistory{},
		ReadAdvancer:        newFakeReadAdvancer(),
		AgentResponseSender: &fakeSender{},
		AgentHostingRepo:    aghosting.NewMemoryStore(),
		AIHostingStore:      aiHostingStore,
		AIHostingLogic:      convhosting.NewConversationAIHostingLogic(aiHostingStore),
		AgentAudit:          agaudit.NewMemoryStore(),
	}
}

// singleTestConvID 复刻 single:<lower>:<higher> 会话 id 约定。
func singleTestConvID(a, b string) string {
	if a <= b {
		return "single:" + a + ":" + b
	}
	return "single:" + b + ":" + a
}

// fakeMessageHistory / fakeReadAdvancer / fakeSender 是 runtime 跨域读写端口的进程内替身
// （生产由 msgrpc 适配器经 msg-rpc gRPC 承接，#617）。
type fakeMessageHistory struct {
	msgs []agentim.Message
}

func (f *fakeMessageHistory) GetRecentMessages(context.Context, agentim.RecentMessagesRequest) ([]agentim.Message, error) {
	return append([]agentim.Message(nil), f.msgs...), nil
}

type fakeReadAdvancer struct {
	mu    sync.Mutex
	reads map[string]int64
}

func newFakeReadAdvancer() *fakeReadAdvancer {
	return &fakeReadAdvancer{reads: map[string]int64{}}
}

func (f *fakeReadAdvancer) MarkConversationRead(_ context.Context, accountID, conversationID string, seq int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := accountID + "|" + conversationID
	if seq > f.reads[key] {
		f.reads[key] = seq
	}
	return nil
}

func (f *fakeReadAdvancer) readSeq(accountID, conversationID string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads[accountID+"|"+conversationID]
}

type fakeSender struct {
	mu   sync.Mutex
	sent []agentim.SendMessageRequest
}

func (f *fakeSender) SendMessage(_ context.Context, req agentim.SendMessageRequest) (agentim.SendMessageResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, req)
	convID := singleTestConvID(req.SenderID, req.ReceiverID)
	if req.ChatType == agentim.MessageChatTypeGroup {
		convID = "group:" + req.GroupID
	}
	return agentim.SendMessageResponse{Message: agentim.Message{
		ServerMsgID:           "srv_" + req.ClientMsgID,
		ClientMsgID:           req.ClientMsgID,
		ConversationID:        convID,
		Seq:                   1,
		SenderID:              req.SenderID,
		ReceiverID:            req.ReceiverID,
		GroupID:               req.GroupID,
		ChatType:              req.ChatType,
		ContentType:           req.ContentType,
		Content:               req.Content,
		MessageOrigin:         req.MessageOrigin,
		AgentAccountID:        req.AgentAccountID,
		TriggerServerMsgID:    req.TriggerServerMsgID,
		AgentRunID:            req.AgentRunID,
		AllowRecursiveTrigger: req.AllowRecursiveTrigger,
	}}, nil
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

func waitForAgentAuditRuns(t *testing.T, store agaudit.Store, want int64) {
	t.Helper()
	counter, ok := store.(interface {
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
