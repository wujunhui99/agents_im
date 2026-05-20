package agentim

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/zeromicro/go-zero/core/logx"
)

const defaultAsyncTriggerRunTimeout = 2 * time.Minute

type AgentTriggerRunner interface {
	Run(ctx context.Context, trigger AgentTrigger) (AgentRunOrchestratorResult, error)
}

type AgentTriggerRunnerFunc func(ctx context.Context, trigger AgentTrigger) (AgentRunOrchestratorResult, error)

func (f AgentTriggerRunnerFunc) Run(ctx context.Context, trigger AgentTrigger) (AgentRunOrchestratorResult, error) {
	if f == nil {
		return AgentRunOrchestratorResult{}, apperror.Internal("agent trigger runner is not configured")
	}
	return f(ctx, trigger)
}

type AgentAccountResolver interface {
	IsActiveAgentAccount(ctx context.Context, accountID string) (bool, error)
}

type AgentAccountResolverFunc func(ctx context.Context, accountID string) (bool, error)

func (f AgentAccountResolverFunc) IsActiveAgentAccount(ctx context.Context, accountID string) (bool, error) {
	if f == nil {
		return false, nil
	}
	return f(ctx, accountID)
}

type AgentRepositoryAccountResolver struct {
	repo repository.AgentRepository
}

func NewAgentRepositoryAccountResolver(repo repository.AgentRepository) AgentRepositoryAccountResolver {
	return AgentRepositoryAccountResolver{repo: repo}
}

func (r AgentRepositoryAccountResolver) IsActiveAgentAccount(ctx context.Context, accountID string) (bool, error) {
	if r.repo == nil {
		return false, apperror.Internal("agent repository is not configured")
	}
	agent, err := r.repo.GetAgentByIMUserID(ctx, strings.TrimSpace(accountID))
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return false, nil
		}
		return false, err
	}
	return agent.Status == model.AgentStatusActive, nil
}

func (r AgentRepositoryAccountResolver) AgentRepository() repository.AgentRepository {
	return r.repo
}

type ConversationHostingConfig struct {
	Repository           repository.AgentConversationHostingRepository
	AIHostingRepository  repository.ConversationAIHostingRepository
	Runner               AgentTriggerRunner
	AgentAccountResolver AgentAccountResolver
	GroupMembers         logic.GroupMemberLister
	ReadMarker           AgentTriggerReadMarker
	TriggerPolicy        TriggerPolicy
	RunTimeout           time.Duration
}

type ConversationHostingService struct {
	repo          repository.AgentConversationHostingRepository
	aiHostingRepo repository.ConversationAIHostingRepository
	runner        AgentTriggerRunner
	agentResolver AgentAccountResolver
	groupMembers  logic.GroupMemberLister
	readMarker    AgentTriggerReadMarker
	policy        TriggerPolicy
	runTimeout    time.Duration
}

type ConversationHostingMessageCreatedInput struct {
	EventID               string
	OperationID           string
	TraceID               string
	Message               logic.Message
	TargetAgentAccountIDs []string
}

type ConversationHostingResult struct {
	Triggered bool
	Responses []AgentResponseResult
	Response  AgentResponseResult
}

func NewConversationHostingService(config ConversationHostingConfig) (*ConversationHostingService, error) {
	if config.Repository == nil {
		return nil, apperror.Internal("agent conversation hosting repository is not configured")
	}
	if config.Runner == nil {
		return nil, apperror.Internal("agent trigger runner is not configured")
	}
	runTimeout := config.RunTimeout
	if runTimeout <= 0 {
		runTimeout = defaultAsyncTriggerRunTimeout
	}
	return &ConversationHostingService{
		repo:          config.Repository,
		aiHostingRepo: config.AIHostingRepository,
		runner:        config.Runner,
		agentResolver: config.AgentAccountResolver,
		groupMembers:  config.GroupMembers,
		readMarker:    config.ReadMarker,
		policy:        config.TriggerPolicy,
		runTimeout:    runTimeout,
	}, nil
}

func (s *ConversationHostingService) HandleMessageCreated(ctx context.Context, input ConversationHostingMessageCreatedInput) (ConversationHostingResult, error) {
	if s == nil || s.repo == nil {
		return ConversationHostingResult{}, apperror.Internal("agent conversation hosting repository is not configured")
	}
	if s.runner == nil {
		return ConversationHostingResult{}, apperror.Internal("agent trigger runner is not configured")
	}
	if strings.TrimSpace(input.EventID) == "" {
		return ConversationHostingResult{}, apperror.InvalidArgument("event_id is required")
	}
	if input.Message.ServerMsgID == "" {
		return ConversationHostingResult{}, apperror.InvalidArgument("server_msg_id is required")
	}
	if input.Message.MessageOrigin == logic.MessageOriginSystem {
		return ConversationHostingResult{}, nil
	}
	if input.Message.MessageOrigin == logic.MessageOriginAI && !input.Message.AllowRecursiveTrigger {
		return ConversationHostingResult{}, nil
	}

	targetAgentIDs, policy, err := s.targetAgentAccountIDs(ctx, input)
	if err != nil {
		return ConversationHostingResult{}, err
	}
	if len(targetAgentIDs) == 0 {
		return ConversationHostingResult{}, nil
	}

	event := MessageCreatedEvent{
		EventID:          strings.TrimSpace(input.EventID),
		OperationID:      strings.TrimSpace(input.OperationID),
		TraceID:          strings.TrimSpace(input.TraceID),
		ConversationID:   input.Message.ConversationID,
		ConversationType: input.Message.ChatType,
		Message: MessageEnvelope{
			ServerMsgID: input.Message.ServerMsgID,
			ClientMsgID: input.Message.ClientMsgID,
			Seq:         input.Message.Seq,
			SenderID:    input.Message.SenderID,
			SenderType:  senderTypeForMessage(input.Message),
			ReceiverID:  input.Message.ReceiverID,
			GroupID:     input.Message.GroupID,
			ContentType: input.Message.ContentType,
			Text:        input.Message.Content,
			AtUserIDs:   groupAtUserIDs(input.Message, targetAgentIDs),
			AgentMetadata: AgentMessageMetadata{
				AgentRunID:            input.Message.AgentRunID,
				TriggerMessageID:      input.Message.TriggerServerMsgID,
				AllowRecursiveTrigger: input.Message.AllowRecursiveTrigger,
			},
		},
		TargetAgentUserIDs: targetAgentIDs,
	}
	triggers, err := BuildMessageCreatedTriggers(event, policy)
	if err != nil {
		return ConversationHostingResult{}, err
	}
	if len(triggers) == 0 {
		return ConversationHostingResult{}, nil
	}

	result := ConversationHostingResult{}
	for _, trigger := range triggers {
		ran, err := s.acceptAndScheduleTrigger(ctx, trigger)
		if err != nil {
			return result, err
		}
		if !ran {
			continue
		}
		result.Triggered = true
	}
	return result, nil
}

func (s *ConversationHostingService) OnMessageCreated(ctx context.Context, input logic.MessageCreatedHookInput) error {
	_, err := s.HandleMessageCreated(ctx, ConversationHostingMessageCreatedInput{
		EventID:               input.EventID,
		OperationID:           input.OperationID,
		TraceID:               input.TraceID,
		Message:               input.Message,
		TargetAgentAccountIDs: nil,
	})
	return err
}

func (s *ConversationHostingService) targetAgentAccountIDs(ctx context.Context, input ConversationHostingMessageCreatedInput) ([]string, TriggerPolicy, error) {
	targets := uniqueNonEmptyIDs(input.TargetAgentAccountIDs)
	policy := s.policy
	if input.Message.MessageOrigin == logic.MessageOriginHuman && input.Message.ChatType == logic.MessageChatTypeSingle && s.aiHostingRepo != nil {
		hosting, err := s.aiHostingRepo.GetEnabledConversationAIHosting(ctx, input.Message.ConversationID)
		if err != nil && apperror.From(err).Code != apperror.CodeNotFound {
			return nil, TriggerPolicy{}, err
		}
		if err == nil && hosting.Enabled && hosting.OwnerAccountID != input.Message.SenderID {
			targets = append(targets, hosting.OwnerAccountID)
		}
	}
	hosting, err := s.repo.GetAgentConversationHosting(ctx, input.Message.ConversationID)
	if err != nil && apperror.From(err).Code != apperror.CodeNotFound {
		return nil, TriggerPolicy{}, err
	}
	if err == nil && hosting.Enabled {
		targets = append(targets, hosting.AgentAccountID)
		if hosting.AllowAgentMessageRecursion {
			policy.AllowAgentMessageRecursion = true
		}
	}
	if input.Message.ChatType == logic.MessageChatTypeSingle && input.Message.ReceiverID != "" && s.agentResolver != nil {
		active, err := s.agentResolver.IsActiveAgentAccount(ctx, input.Message.ReceiverID)
		if err != nil {
			return nil, TriggerPolicy{}, err
		}
		if active {
			targets = append(targets, input.Message.ReceiverID)
		}
	}
	return uniqueNonEmptyIDs(targets), policy, nil
}

func (s *ConversationHostingService) acceptAndScheduleTrigger(ctx context.Context, trigger AgentTrigger) (bool, error) {
	if err := s.validateTriggerAuthorization(ctx, trigger); err != nil {
		return s.recordRejectedTrigger(ctx, trigger, err)
	}

	started, err := s.repo.TryStartAgentTrigger(ctx, repository.AgentTriggerStartInput{
		IdempotencyKey:     trigger.RequestID,
		ConversationID:     trigger.ConversationID,
		AgentAccountID:     trigger.AgentUserID,
		TriggerServerMsgID: trigger.TriggerMessageID,
		TriggerEventID:     trigger.EventID,
		RunningTTL:         s.triggerRunningTTL(),
	})
	if err != nil {
		return false, err
	}
	if !started {
		return false, nil
	}

	if err := s.markTriggerRead(ctx, trigger); err != nil {
		finishErr := s.repo.FinishAgentTrigger(ctx, repository.AgentTriggerFinishInput{
			IdempotencyKey: trigger.RequestID,
			Status:         repository.AgentTriggerStatusFailed,
			ErrorMessage:   err.Error(),
		})
		if finishErr != nil {
			return true, errors.Join(err, fmt.Errorf("record failed agent trigger: %w", finishErr))
		}
		return true, err
	}

	s.runAcceptedTriggerAsync(trigger)
	return true, nil
}

func (s *ConversationHostingService) validateTriggerAuthorization(ctx context.Context, trigger AgentTrigger) error {
	if trigger.ConversationType != ConversationTypeGroup {
		return nil
	}
	if s.groupMembers == nil {
		return apperror.Internal("group membership validator is not configured")
	}
	groupID, err := groupConversationID(trigger.ConversationID)
	if err != nil {
		return err
	}
	members, err := s.groupMembers.ListMembers(ctx, logic.ListMembersRequest{
		GroupID:         groupID,
		RequesterUserID: trigger.RequestingUserID,
	})
	if err != nil {
		return err
	}
	for _, member := range members.Members {
		if strings.TrimSpace(member.UserID) == trigger.AgentUserID && (member.State == "" || member.State == "active") {
			return nil
		}
	}
	return apperror.Forbidden("target agent is not an active group member")
}

func (s *ConversationHostingService) recordRejectedTrigger(ctx context.Context, trigger AgentTrigger, cause error) (bool, error) {
	started, err := s.repo.TryStartAgentTrigger(ctx, repository.AgentTriggerStartInput{
		IdempotencyKey:     trigger.RequestID,
		ConversationID:     trigger.ConversationID,
		AgentAccountID:     trigger.AgentUserID,
		TriggerServerMsgID: trigger.TriggerMessageID,
		TriggerEventID:     trigger.EventID,
		RunningTTL:         s.triggerRunningTTL(),
	})
	if err != nil {
		return false, err
	}
	if !started {
		return false, nil
	}
	if err := s.repo.FinishAgentTrigger(ctx, repository.AgentTriggerFinishInput{
		IdempotencyKey: trigger.RequestID,
		Status:         repository.AgentTriggerStatusFailed,
		ErrorMessage:   cause.Error(),
	}); err != nil {
		return false, errors.Join(cause, fmt.Errorf("record rejected agent trigger: %w", err))
	}
	return false, nil
}

func (s *ConversationHostingService) markTriggerRead(ctx context.Context, trigger AgentTrigger) error {
	if s.readMarker == nil || trigger.ConversationType != ConversationTypeSingle {
		return nil
	}
	return s.readMarker.MarkTriggerRead(ctx, AgentTriggerReadMark{
		AccountID:      trigger.AgentUserID,
		ConversationID: trigger.ConversationID,
		TriggerSeq:     trigger.TriggerSeq,
	})
}

func (s *ConversationHostingService) runAcceptedTriggerAsync(trigger AgentTrigger) {
	runTimeout := s.runTimeout
	if runTimeout <= 0 {
		runTimeout = defaultAsyncTriggerRunTimeout
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
		defer cancel()
		if _, err := s.runAcceptedTrigger(ctx, trigger); err != nil {
			logx.Errorf("agent hosting async trigger failed request_id=%q conversation_id=%q agent_account_id=%q trigger_server_msg_id=%q: %v",
				trigger.RequestID, trigger.ConversationID, trigger.AgentUserID, trigger.TriggerMessageID, err)
		}
	}()
}

func (s *ConversationHostingService) triggerRunningTTL() time.Duration {
	ttl := repository.DefaultAgentTriggerRunningTTL
	if s == nil {
		return ttl
	}
	runTimeout := s.runTimeout
	if runTimeout <= 0 {
		runTimeout = defaultAsyncTriggerRunTimeout
	}
	if withGrace := runTimeout + time.Minute; withGrace > ttl {
		return withGrace
	}
	return ttl
}

func (s *ConversationHostingService) runAcceptedTrigger(ctx context.Context, trigger AgentTrigger) (AgentResponseResult, error) {
	run, err := s.runner.Run(ctx, trigger)
	if err != nil {
		finishErr := s.repo.FinishAgentTrigger(ctx, repository.AgentTriggerFinishInput{
			IdempotencyKey: trigger.RequestID,
			Status:         repository.AgentTriggerStatusFailed,
			ErrorMessage:   err.Error(),
		})
		if finishErr != nil {
			return AgentResponseResult{}, errors.Join(err, fmt.Errorf("record failed agent trigger: %w", finishErr))
		}
		return AgentResponseResult{}, err
	}
	if err := s.repo.FinishAgentTrigger(ctx, repository.AgentTriggerFinishInput{
		IdempotencyKey:      trigger.RequestID,
		Status:              repository.AgentTriggerStatusSucceeded,
		ResponseServerMsgID: run.Response.Message.ServerMsgID,
	}); err != nil {
		return AgentResponseResult{}, err
	}
	return run.Response, nil
}

func senderTypeForMessage(message logic.Message) string {
	if message.MessageOrigin == logic.MessageOriginAI {
		return SenderTypeAgent
	}
	return SenderTypeUser
}

func groupAtUserIDs(message logic.Message, targetAgentIDs []string) []string {
	if message.ChatType != logic.MessageChatTypeGroup {
		return nil
	}
	return append([]string(nil), targetAgentIDs...)
}
