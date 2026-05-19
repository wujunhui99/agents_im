package message

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/config"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
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
				ctx.AIHostingRepo = nil
				return ctx
			}(),
			wantErr: "conversation AI hosting repository is not configured",
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
	serviceContext := NewServiceContextWithMedia(
		messageRepo,
		repository.NewMemoryMediaRepository(),
		nil,
		nil,
		config.DefaultJWTAuthConfig(),
	)
	if err := ConfigureConversationAIHosting(serviceContext, config.DeepSeekConfig{}, config.LLMObservabilityConfig{}); err != nil {
		t.Fatalf("configure conversation AI hosting: %v", err)
	}

	conversationID := repository.SingleConversationID("usr_hosted_owner", "usr_peer")
	if _, err := serviceContext.AIHostingRepo.SetConversationAIHostingEnabled(ctx, repository.ConversationAIHostingUpdate{
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

func completeAIHostingServiceContext() *ServiceContext {
	messageRepo := repository.NewMemoryMessageRepository()
	agentAuditRepo := repository.NewMemoryAgentAuditRepository()
	aiHostingRepo := repository.NewMemoryConversationAIHostingRepository()
	return &ServiceContext{
		MessageLogic:     business.NewMessageLogic(messageRepo),
		MessageRepo:      messageRepo,
		AgentHostingRepo: repository.NewMemoryAgentConversationHostingRepository(),
		AIHostingRepo:    aiHostingRepo,
		AIHostingLogic:   business.NewConversationAIHostingLogic(aiHostingRepo),
		AgentAuditRepo:   agentAuditRepo,
		AgentAuditLogic:  business.NewAgentAuditLogic(agentAuditRepo),
	}
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
