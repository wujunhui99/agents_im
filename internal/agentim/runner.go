package agentim

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/domain/agentaudit"
)

type AgentRuntime interface {
	Run(ctx context.Context, trigger AgentTrigger) (AgentRuntimeResult, error)
}

type AgentRuntimeResult struct {
	FinalText             string
	OutputSummary         agentaudit.Summary
	AllowRecursiveTrigger bool
}

type AgentRunAuditRecorder interface {
	RecordAgentRun(ctx context.Context, input agentaudit.CreateRunInput) (agentaudit.AgentRun, error)
}

type AgentRunOrchestrator struct {
	runtime AgentRuntime
	audit   AgentRunAuditRecorder
	writer  ResponseWriter
	now     func() time.Time
}

type AgentRunOrchestratorResult struct {
	AuditRun agentaudit.AgentRun
	Response AgentResponseResult
}

type AgentRunOrchestratorConfig struct {
	Runtime AgentRuntime
	Audit   AgentRunAuditRecorder
	Writer  ResponseWriter
	Now     func() time.Time
}

func NewAgentRunOrchestrator(config AgentRunOrchestratorConfig) (*AgentRunOrchestrator, error) {
	if config.Runtime == nil {
		return nil, apperror.Internal("agent runtime is not configured")
	}
	if config.Audit == nil {
		return nil, apperror.Internal("agent audit recorder is not configured")
	}
	if config.Writer == nil {
		return nil, apperror.Internal("agent response writer is not configured")
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &AgentRunOrchestrator{
		runtime: config.Runtime,
		audit:   config.Audit,
		writer:  config.Writer,
		now:     now,
	}, nil
}

func (o *AgentRunOrchestrator) Run(ctx context.Context, trigger AgentTrigger) (AgentRunOrchestratorResult, error) {
	if o == nil || o.runtime == nil {
		return AgentRunOrchestratorResult{}, apperror.Internal("agent runtime is not configured")
	}
	if o.audit == nil {
		return AgentRunOrchestratorResult{}, apperror.Internal("agent audit recorder is not configured")
	}
	if o.writer == nil {
		return AgentRunOrchestratorResult{}, apperror.Internal("agent response writer is not configured")
	}

	normalized, err := normalizeAgentTriggerForRun(trigger)
	if err != nil {
		return AgentRunOrchestratorResult{}, err
	}

	now := o.now
	if now == nil {
		now = time.Now
	}
	startedAt := now().UTC()
	runtimeResult, err := o.runtime.Run(ctx, normalized)
	finishedAt := now().UTC()
	if err != nil {
		auditErr := o.recordFailedRun(ctx, normalized, startedAt, finishedAt, "runtime_error", err)
		if auditErr != nil {
			return AgentRunOrchestratorResult{}, errors.Join(err, fmt.Errorf("record failed agent run audit: %w", auditErr))
		}
		return AgentRunOrchestratorResult{}, err
	}

	finalText := strings.TrimSpace(runtimeResult.FinalText)
	if finalText == "" {
		emptyTextErr := apperror.InvalidArgument("runtime final text is required")
		auditErr := o.recordFailedRun(ctx, normalized, startedAt, finishedAt, "empty_final_text", emptyTextErr)
		if auditErr != nil {
			return AgentRunOrchestratorResult{}, errors.Join(emptyTextErr, fmt.Errorf("record failed agent run audit: %w", auditErr))
		}
		return AgentRunOrchestratorResult{}, emptyTextErr
	}

	auditRun, err := o.audit.RecordAgentRun(ctx, agentaudit.CreateRunInput{
		AgentID:          normalized.AgentUserID,
		ConversationID:   normalized.ConversationID,
		TriggerMessageID: normalized.TriggerMessageID,
		RequestingUserID: normalized.RequestingUserID,
		Status:           agentaudit.StatusSucceeded,
		InputSummary:     agentRunInputSummary(normalized),
		OutputSummary:    agentRunOutputSummary(finalText, runtimeResult.OutputSummary),
		TraceID:          normalized.TraceID,
		RequestID:        normalized.RequestID,
		StartedAt:        startedAt,
		FinishedAt:       finishedAt,
	})
	if err != nil {
		return AgentRunOrchestratorResult{}, fmt.Errorf("record agent run audit: %w", err)
	}
	if strings.TrimSpace(auditRun.RunID) == "" {
		return AgentRunOrchestratorResult{}, apperror.Internal("agent audit returned empty run_id")
	}

	responseReq, err := buildAgentResponseRequest(normalized, auditRun.RunID, finalText, runtimeResult.AllowRecursiveTrigger)
	if err != nil {
		return AgentRunOrchestratorResult{}, err
	}
	response, err := o.writer.WriteAgentResponse(ctx, responseReq)
	if err != nil {
		return AgentRunOrchestratorResult{}, fmt.Errorf("write agent response through message service: %w", err)
	}

	return AgentRunOrchestratorResult{
		AuditRun: auditRun,
		Response: response,
	}, nil
}

func (o *AgentRunOrchestrator) recordFailedRun(ctx context.Context, trigger AgentTrigger, startedAt time.Time, finishedAt time.Time, code string, cause error) error {
	_, err := o.audit.RecordAgentRun(ctx, agentaudit.CreateRunInput{
		AgentID:          trigger.AgentUserID,
		ConversationID:   trigger.ConversationID,
		TriggerMessageID: trigger.TriggerMessageID,
		RequestingUserID: trigger.RequestingUserID,
		Status:           agentaudit.StatusFailed,
		InputSummary:     agentRunInputSummary(trigger),
		ErrorCode:        code,
		ErrorMessage:     cause.Error(),
		TraceID:          trigger.TraceID,
		RequestID:        trigger.RequestID,
		StartedAt:        startedAt,
		FinishedAt:       finishedAt,
	})
	return err
}

func normalizeAgentTriggerForRun(trigger AgentTrigger) (AgentTrigger, error) {
	requestID, err := normalizeRequired(trigger.RequestID, "request_id")
	if err != nil {
		return AgentTrigger{}, err
	}
	triggerType, err := normalizeTriggerType(trigger.TriggerType)
	if err != nil {
		return AgentTrigger{}, err
	}
	agentUserID, err := normalizeRequired(trigger.AgentUserID, "agent_user_id")
	if err != nil {
		return AgentTrigger{}, err
	}
	requestingUserID, err := normalizeRequired(trigger.RequestingUserID, "requesting_user_id")
	if err != nil {
		return AgentTrigger{}, err
	}
	conversationID, err := normalizeRequired(trigger.ConversationID, "conversation_id")
	if err != nil {
		return AgentTrigger{}, err
	}
	conversationType, err := normalizeConversationType(trigger.ConversationType)
	if err != nil {
		return AgentTrigger{}, err
	}
	triggerMessageID := normalizeOptional(trigger.TriggerMessageID)
	if triggerType != TriggerTypeAdminManualRun {
		if triggerMessageID == "" {
			return AgentTrigger{}, apperror.InvalidArgument("trigger_message_id is required")
		}
		if trigger.TriggerSeq <= 0 {
			return AgentTrigger{}, apperror.InvalidArgument("trigger_seq must be greater than 0")
		}
	}

	trigger.RequestID = requestID
	trigger.TriggerType = triggerType
	trigger.AgentUserID = agentUserID
	trigger.RequestingUserID = requestingUserID
	trigger.ConversationID = conversationID
	trigger.ConversationType = conversationType
	trigger.TriggerMessageID = triggerMessageID
	trigger.EventID = normalizeOptional(trigger.EventID)
	trigger.OperationID = normalizeOptional(trigger.OperationID)
	trigger.TraceID = normalizeOptional(trigger.TraceID)
	trigger.PromptText = strings.TrimSpace(trigger.PromptText)
	trigger.ReplyToMessageID = normalizeOptional(trigger.ReplyToMessageID)
	trigger.SourceAgentRunID = normalizeOptional(trigger.SourceAgentRunID)
	trigger.SourceAgentUserID = normalizeOptional(trigger.SourceAgentUserID)
	trigger.SourceMessageID = normalizeOptional(trigger.SourceMessageID)
	trigger.SourceMessageText = strings.TrimSpace(trigger.SourceMessageText)
	trigger.SourceContentType = normalizeOptional(trigger.SourceContentType)
	trigger.TargetAgentUserIDs = uniqueNonEmptyIDs(trigger.TargetAgentUserIDs)
	return trigger, nil
}

func normalizeTriggerType(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch value {
	case TriggerTypeUserPrivateMessage, TriggerTypeGroupMention, TriggerTypeAdminManualRun:
		return value, nil
	default:
		return "", apperror.InvalidArgument("trigger_type is invalid")
	}
}

func buildAgentResponseRequest(trigger AgentTrigger, runID string, finalText string, allowRecursiveTrigger bool) (AgentResponseRequest, error) {
	receiverUserID, groupID, err := responseTargetForTrigger(trigger)
	if err != nil {
		return AgentResponseRequest{}, err
	}
	replyToMessageID := trigger.ReplyToMessageID
	if replyToMessageID == "" {
		replyToMessageID = trigger.TriggerMessageID
	}
	return AgentResponseRequest{
		RequestID:              agentResponseRequestID(trigger.RequestID),
		OperationID:            trigger.OperationID,
		TraceID:                trigger.TraceID,
		AgentRunID:             runID,
		AgentUserID:            trigger.AgentUserID,
		ConversationID:         trigger.ConversationID,
		ConversationType:       trigger.ConversationType,
		ReceiverUserID:         receiverUserID,
		GroupID:                groupID,
		ReplyToMessageID:       replyToMessageID,
		Text:                   finalText,
		AllowRecursiveTrigger:  allowRecursiveTrigger,
		TargetAgentUserIDs:     append([]string(nil), trigger.TargetAgentUserIDs...),
		TriggerMessageID:       trigger.TriggerMessageID,
		SourceTriggerRequestID: trigger.RequestID,
	}, nil
}

func responseTargetForTrigger(trigger AgentTrigger) (string, string, error) {
	switch trigger.ConversationType {
	case ConversationTypeSingle:
		receiverID, err := singleConversationPeer(trigger.ConversationID, trigger.AgentUserID)
		if err != nil {
			return "", "", err
		}
		return receiverID, "", nil
	case ConversationTypeGroup:
		groupID, err := groupConversationID(trigger.ConversationID)
		if err != nil {
			return "", "", err
		}
		return "", groupID, nil
	default:
		return "", "", apperror.InvalidArgument("conversation_type must be single or group")
	}
}

func singleConversationPeer(conversationID string, agentUserID string) (string, error) {
	parts := strings.Split(conversationID, ":")
	if len(parts) != 3 || parts[0] != ConversationTypeSingle {
		return "", apperror.InvalidArgument("single conversation_id must be single:<user_id>:<user_id>")
	}
	left := strings.TrimSpace(parts[1])
	right := strings.TrimSpace(parts[2])
	switch {
	case left == "" || right == "":
		return "", apperror.InvalidArgument("single conversation_id contains empty user_id")
	case left == agentUserID && right != agentUserID:
		return right, nil
	case right == agentUserID && left != agentUserID:
		return left, nil
	default:
		return "", apperror.InvalidArgument("single conversation_id must include exactly one agent_user_id")
	}
}

func groupConversationID(conversationID string) (string, error) {
	groupID, ok := strings.CutPrefix(conversationID, ConversationTypeGroup+":")
	if !ok {
		return "", apperror.InvalidArgument("group conversation_id must be group:<group_id>")
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return "", apperror.InvalidArgument("group conversation_id contains empty group_id")
	}
	return groupID, nil
}

func agentResponseRequestID(triggerRequestID string) string {
	const suffix = ":response"
	if len([]rune(triggerRequestID))+len([]rune(suffix)) <= 128 {
		return triggerRequestID + suffix
	}
	sum := sha256.Sum256([]byte(triggerRequestID))
	return "agent-response:" + hex.EncodeToString(sum[:])
}

func agentRunInputSummary(trigger AgentTrigger) agentaudit.Summary {
	return agentaudit.Summary{
		"request_id":            trigger.RequestID,
		"event_id":              trigger.EventID,
		"operation_id":          trigger.OperationID,
		"trace_id":              trigger.TraceID,
		"trigger_type":          trigger.TriggerType,
		"agent_user_id":         trigger.AgentUserID,
		"requesting_user_id":    trigger.RequestingUserID,
		"conversation_id":       trigger.ConversationID,
		"conversation_type":     trigger.ConversationType,
		"trigger_message_id":    trigger.TriggerMessageID,
		"trigger_seq":           trigger.TriggerSeq,
		"prompt_text":           trigger.PromptText,
		"recursive_trigger":     trigger.RecursiveTrigger,
		"source_agent_run_id":   trigger.SourceAgentRunID,
		"source_agent_user_id":  trigger.SourceAgentUserID,
		"source_message_id":     trigger.SourceMessageID,
		"source_message_seq":    trigger.SourceMessageSeq,
		"source_content_type":   trigger.SourceContentType,
		"target_agent_user_ids": append([]string(nil), trigger.TargetAgentUserIDs...),
	}
}

func agentRunOutputSummary(finalText string, runtimeSummary agentaudit.Summary) agentaudit.Summary {
	summary := agentaudit.Summary{
		"final_text":       finalText,
		"final_text_bytes": len([]byte(finalText)),
	}
	for key, value := range runtimeSummary {
		if key == "final_text" || key == "final_text_bytes" {
			continue
		}
		summary[key] = value
	}
	return summary
}
