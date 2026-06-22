package orchestrator

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	agentruntime "github.com/wujunhui99/agents_im/service/agent/rpc/internal/runtime"
)

func TestGroupExplicitTargetAgentCreatesOneRunAndMessageServiceReply(t *testing.T) {
	ctx := context.Background()
	h := newGroupAgentHarness(t, []string{"usr_sender", "usr_peer", "agent_1"})

	human := h.sendGroupHuman(t, "client-group-target", "@agent_1 summarize this")
	result, err := h.hosting.HandleMessageCreated(ctx, ConversationHostingMessageCreatedInput{
		EventID:               "evt_group_target_1",
		Message:               human.Message,
		TargetAgentAccountIDs: []string{"agent_1"},
	})
	if err != nil {
		t.Fatalf("handle targeted group message: %v", err)
	}
	if !result.Triggered {
		t.Fatalf("targeted group message did not trigger agent: %+v", result)
	}

	pulled := waitForPulledMessageCount(t, h.messageLogic, "usr_peer", human.Message.ConversationID, 2)
	if calls := h.runtimeCallCount(); calls != 1 {
		t.Fatalf("runtime calls = %d, want 1", calls)
	}
	if len(pulled.Messages) != 2 {
		t.Fatalf("group timeline length = %d, want human + ai", len(pulled.Messages))
	}

	aiMessage := pulled.Messages[1]
	if aiMessage.MessageOrigin != logic.MessageOriginAI {
		t.Fatalf("group agent response origin = %q, want ai: %+v", aiMessage.MessageOrigin, aiMessage)
	}
	if aiMessage.SenderID != "agent_1" || aiMessage.AgentAccountID != "agent_1" {
		t.Fatalf("group agent response sender metadata mismatch: %+v", aiMessage)
	}
	if aiMessage.ChatType != logic.MessageChatTypeGroup || aiMessage.GroupID != "grp_agent_chat" {
		t.Fatalf("group agent response target mismatch: %+v", aiMessage)
	}
	if aiMessage.Seq != human.Message.Seq+1 {
		t.Fatalf("group agent response seq = %d, want %d", aiMessage.Seq, human.Message.Seq+1)
	}
	if aiMessage.TriggerServerMsgID != human.Message.ServerMsgID || aiMessage.AgentRunID == "" {
		t.Fatalf("group agent response trigger metadata mismatch: %+v", aiMessage)
	}
	if aiMessage.AllowRecursiveTrigger {
		t.Fatalf("group agent response should suppress recursion by default: %+v", aiMessage)
	}
}

func TestGroupMessageWithoutExplicitTargetDoesNotTriggerAgentNoise(t *testing.T) {
	ctx := context.Background()
	h := newGroupAgentHarness(t, []string{"usr_sender", "usr_peer", "agent_1"})

	human := h.sendGroupHuman(t, "client-group-chatter", "plain group chatter")
	result, err := h.hosting.HandleMessageCreated(ctx, ConversationHostingMessageCreatedInput{
		EventID: "evt_group_chatter_1",
		Message: human.Message,
	})
	if err != nil {
		t.Fatalf("handle untargeted group message: %v", err)
	}
	if result.Triggered {
		t.Fatalf("untargeted group chatter triggered agent: %+v", result)
	}
	if calls := h.runtimeCallCount(); calls != 0 {
		t.Fatalf("runtime calls = %d, want 0", calls)
	}

	pulled, err := h.messageLogic.PullMessages(ctx, logic.PullMessagesRequest{
		UserID:         "usr_peer",
		ConversationID: human.Message.ConversationID,
		FromSeq:        1,
		Limit:          10,
		Order:          "asc",
	})
	if err != nil {
		t.Fatalf("pull group chatter: %v", err)
	}
	if len(pulled.Messages) != 1 {
		t.Fatalf("untargeted group chatter produced agent noise: %+v", pulled.Messages)
	}
}

func TestGroupAIReplyDoesNotTriggerAnotherAgentByDefault(t *testing.T) {
	ctx := context.Background()
	h := newGroupAgentHarness(t, []string{"usr_sender", "usr_peer", "agent_1", "agent_2"})

	human := h.sendGroupHuman(t, "client-group-loop-human", "@agent_1 answer")
	if _, err := h.hosting.HandleMessageCreated(ctx, ConversationHostingMessageCreatedInput{
		EventID:               "evt_group_loop_human_1",
		Message:               human.Message,
		TargetAgentAccountIDs: []string{"agent_1"},
	}); err != nil {
		t.Fatalf("handle initial group target: %v", err)
	}
	pulled := waitForPulledMessageCount(t, h.messageLogic, "usr_peer", human.Message.ConversationID, 2)
	aiMessage := pulled.Messages[1]

	result, err := h.hosting.HandleMessageCreated(ctx, ConversationHostingMessageCreatedInput{
		EventID:               "evt_group_loop_ai_1",
		Message:               aiMessage,
		TargetAgentAccountIDs: []string{"agent_2"},
	})
	if err != nil {
		t.Fatalf("handle group ai response: %v", err)
	}
	if result.Triggered {
		t.Fatalf("ai-origin group response triggered another agent without recursion opt-in: %+v", result)
	}
	if calls := h.runtimeCallCount(); calls != 1 {
		t.Fatalf("runtime calls = %d, want only the original human-triggered run", calls)
	}
}

func TestGroupNonMemberAgentTargetRecordsFailureWithoutRunningAgent(t *testing.T) {
	ctx := context.Background()
	h := newGroupAgentHarness(t, []string{"usr_sender", "usr_peer"})

	human := h.sendGroupHuman(t, "client-group-nonmember-agent", "@agent_1 summarize")
	result, err := h.hosting.HandleMessageCreated(ctx, ConversationHostingMessageCreatedInput{
		EventID:               "evt_group_nonmember_agent_1",
		Message:               human.Message,
		TargetAgentAccountIDs: []string{"agent_1"},
	})
	if err != nil {
		t.Fatalf("handle non-member agent target: %v", err)
	}
	if result.Triggered {
		t.Fatalf("non-member agent target should not be scheduled: %+v", result)
	}
	if calls := h.runtimeCallCount(); calls != 0 {
		t.Fatalf("runtime calls = %d, want 0 for unauthorized group target", calls)
	}

	finish := h.hostingRepo.requireFinish(t, "evt_group_nonmember_agent_1:agent_1")
	if finish.Status != repository.AgentTriggerStatusFailed {
		t.Fatalf("non-member trigger status = %q, want failed", finish.Status)
	}
	if !strings.Contains(finish.ErrorMessage, "target agent is not an active group member") {
		t.Fatalf("non-member trigger error = %q, want group membership failure", finish.ErrorMessage)
	}
}

type groupAgentHarness struct {
	messageRepo  *repository.MemoryMessageRepository
	messageLogic *logic.MessageLogic
	hostingRepo  *recordingAgentHostingRepository
	hosting      *ConversationHostingService

	mu              sync.Mutex
	runtimeCalls    int
	runtimeRequests []agentruntime.RunRequest
}

func newGroupAgentHarness(t *testing.T, activeMemberIDs []string) *groupAgentHarness {
	t.Helper()

	messageRepo := repository.NewMemoryMessageRepository()
	groups := newAgentIMTestGroupMemberLister("grp_agent_chat", activeMemberIDs)
	messageLogic := logic.NewMessageLogicWithValidators(messageRepo, nil, groups)
	hostingRepo := &recordingAgentHostingRepository{inner: repository.NewMemoryAgentConversationHostingRepository()}
	auditRepo := repository.NewMemoryAgentAuditRepository()
	auditLogic := logic.NewAgentAuditLogic(auditRepo)
	writer, err := NewMessageServiceResponseWriter(messageLogic)
	if err != nil {
		t.Fatalf("new response writer: %v", err)
	}

	h := &groupAgentHarness{
		messageRepo:  messageRepo,
		messageLogic: messageLogic,
		hostingRepo:  hostingRepo,
	}
	runtime := agentruntime.RuntimeFunc(func(_ context.Context, req agentruntime.RunRequest) (agentruntime.RunResult, error) {
		h.mu.Lock()
		h.runtimeCalls++
		h.runtimeRequests = append(h.runtimeRequests, req)
		h.mu.Unlock()
		return agentruntime.RunResult{
			RunID:     req.RunID,
			FinalText: "group agent reply: " + req.PromptText,
		}, nil
	})
	orchestrator, err := NewAgentRunOrchestrator(AgentRunOrchestratorConfig{
		Runtime: runtime,
		RequestBuilder: RuntimeRequestBuilderFunc(func(_ context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error) {
			return groupRuntimeRequest(trigger), nil
		}),
		Audit:  auditLogic,
		Writer: writer,
		Now: func() time.Time {
			return time.Unix(300, 0)
		},
	})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	hosting, err := NewConversationHostingService(ConversationHostingConfig{
		Repository:   hostingRepo,
		Runner:       orchestrator,
		GroupMembers: groups,
	})
	if err != nil {
		t.Fatalf("new hosting service: %v", err)
	}
	h.hosting = hosting
	return h
}

func (h *groupAgentHarness) sendGroupHuman(t *testing.T, clientMsgID string, text string) logic.SendMessageResponse {
	t.Helper()

	resp, err := h.messageLogic.SendMessage(context.Background(), logic.SendMessageRequest{
		SenderID:    "usr_sender",
		GroupID:     "grp_agent_chat",
		ChatType:    logic.MessageChatTypeGroup,
		ClientMsgID: clientMsgID,
		ContentType: logic.MessageContentTypeText,
		Content:     text,
	})
	if err != nil {
		t.Fatalf("send group human message: %v", err)
	}
	return resp
}

func (h *groupAgentHarness) runtimeCallCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.runtimeCalls
}

func groupRuntimeRequest(trigger AgentTrigger) agentruntime.RunRequest {
	return agentruntime.RunRequest{
		RunID:              "run_" + trigger.TriggerMessageID,
		RequestID:          trigger.RequestID,
		EventID:            trigger.EventID,
		OperationID:        trigger.OperationID,
		TraceID:            trigger.TraceID,
		TriggerType:        trigger.TriggerType,
		AgentUserID:        trigger.AgentUserID,
		RequestingUserID:   trigger.RequestingUserID,
		ConversationID:     trigger.ConversationID,
		ConversationType:   trigger.ConversationType,
		TriggerMessageID:   trigger.TriggerMessageID,
		TriggerSeq:         trigger.TriggerSeq,
		PromptText:         trigger.PromptText,
		ReplyToMessageID:   trigger.ReplyToMessageID,
		SourceMessageID:    trigger.SourceMessageID,
		SourceMessageSeq:   trigger.SourceMessageSeq,
		SourceMessageText:  trigger.SourceMessageText,
		SourceContentType:  trigger.SourceContentType,
		TargetAgentUserIDs: append([]string(nil), trigger.TargetAgentUserIDs...),
		Agent: agentruntime.AgentConfig{
			AgentID:     "agent_profile_" + trigger.AgentUserID,
			AgentUserID: trigger.AgentUserID,
			Name:        "Group Agent",
			Status:      agentruntime.AgentStatusActive,
			Prompt: agentruntime.PromptRef{
				PromptID: "prompt_group_agent",
				Content:  "Reply in the group conversation.",
			},
			Model: agentruntime.ModelConfig{
				Provider: "deterministic-test",
				Model:    "deterministic-v1",
			},
			Policy: agentruntime.RuntimePolicy{
				RequireMessageServiceWriteback: true,
			},
		},
		Conversation: []agentruntime.ConversationMessage{{
			ServerMsgID: trigger.SourceMessageID,
			Seq:         trigger.SourceMessageSeq,
			SenderID:    trigger.RequestingUserID,
			SenderType:  agentruntime.SenderTypeUser,
			ContentType: trigger.SourceContentType,
			Text:        trigger.SourceMessageText,
		}},
	}
}

type agentIMTestGroupMemberLister struct {
	groupID string
	members []logic.GroupMemberInfo
}

func newAgentIMTestGroupMemberLister(groupID string, activeMemberIDs []string) *agentIMTestGroupMemberLister {
	members := make([]logic.GroupMemberInfo, 0, len(activeMemberIDs))
	for _, userID := range activeMemberIDs {
		members = append(members, logic.GroupMemberInfo{
			GroupID: groupID,
			UserID:  userID,
			State:   "active",
			Role:    "member",
		})
	}
	return &agentIMTestGroupMemberLister{groupID: groupID, members: members}
}

func (l *agentIMTestGroupMemberLister) ListMembers(_ context.Context, req logic.ListMembersRequest) (logic.ListMembersResponse, error) {
	if strings.TrimSpace(req.GroupID) != l.groupID {
		return logic.ListMembersResponse{}, nil
	}
	members := append([]logic.GroupMemberInfo(nil), l.members...)
	if strings.TrimSpace(req.RequesterUserID) != "" {
		active := false
		for _, member := range members {
			if member.UserID == req.RequesterUserID && (member.State == "" || member.State == "active") {
				active = true
				break
			}
		}
		if !active {
			return logic.ListMembersResponse{}, apperror.Forbidden("requester is not a group member")
		}
	}
	return logic.ListMembersResponse{GroupID: req.GroupID, Members: members}, nil
}

type recordingAgentHostingRepository struct {
	inner *repository.MemoryAgentConversationHostingRepository
	mu    sync.Mutex

	starts   []repository.AgentTriggerStartInput
	finishes []repository.AgentTriggerFinishInput
}

func (r *recordingAgentHostingRepository) UpsertAgentConversationHosting(ctx context.Context, hosting repository.AgentConversationHosting) (repository.AgentConversationHosting, error) {
	return r.inner.UpsertAgentConversationHosting(ctx, hosting)
}

func (r *recordingAgentHostingRepository) GetAgentConversationHosting(ctx context.Context, conversationID string) (repository.AgentConversationHosting, error) {
	return r.inner.GetAgentConversationHosting(ctx, conversationID)
}

func (r *recordingAgentHostingRepository) TryStartAgentTrigger(ctx context.Context, input repository.AgentTriggerStartInput) (bool, error) {
	started, err := r.inner.TryStartAgentTrigger(ctx, input)
	r.mu.Lock()
	r.starts = append(r.starts, input)
	r.mu.Unlock()
	return started, err
}

func (r *recordingAgentHostingRepository) FinishAgentTrigger(ctx context.Context, input repository.AgentTriggerFinishInput) error {
	err := r.inner.FinishAgentTrigger(ctx, input)
	r.mu.Lock()
	r.finishes = append(r.finishes, input)
	r.mu.Unlock()
	return err
}

func (r *recordingAgentHostingRepository) requireFinish(t *testing.T, idempotencyKey string) repository.AgentTriggerFinishInput {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		r.mu.Lock()
		for _, finish := range r.finishes {
			if finish.IdempotencyKey == idempotencyKey {
				r.mu.Unlock()
				return finish
			}
		}
		r.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for finish of %q", idempotencyKey)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
