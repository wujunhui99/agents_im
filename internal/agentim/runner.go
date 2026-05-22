package agentim

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/agentruntime"
	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/domain/agentaudit"
	"github.com/wujunhui99/agents_im/internal/llmobs"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/zeromicro/go-zero/core/logx"
)

type RuntimeRequestBuilder interface {
	BuildRuntimeRequest(ctx context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error)
}

type RuntimeRequestBuilderFunc func(ctx context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error)

func (f RuntimeRequestBuilderFunc) BuildRuntimeRequest(ctx context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error) {
	if f == nil {
		return agentruntime.RunRequest{}, apperror.Internal("agent runtime request builder is not configured")
	}
	return f(ctx, trigger)
}

type AgentRunAuditRecorder interface {
	RecordAgentRun(ctx context.Context, input agentaudit.CreateRunInput) (agentaudit.AgentRun, error)
}

type AgentRunOrchestrator struct {
	runtime        agentruntime.Runtime
	requestBuilder RuntimeRequestBuilder
	audit          AgentRunAuditRecorder
	writer         ResponseWriter
	llmobsSink     llmobs.Sink
	now            func() time.Time
}

type AgentRunOrchestratorResult struct {
	AuditRun agentaudit.AgentRun
	Response AgentResponseResult
}

type AgentRunOrchestratorConfig struct {
	Runtime              agentruntime.Runtime
	RequestBuilder       RuntimeRequestBuilder
	Audit                AgentRunAuditRecorder
	Writer               ResponseWriter
	LLMObservabilitySink llmobs.Sink
	Now                  func() time.Time
}

func NewAgentRunOrchestrator(config AgentRunOrchestratorConfig) (*AgentRunOrchestrator, error) {
	if config.Runtime == nil {
		return nil, apperror.Internal("agent runtime is not configured")
	}
	if config.RequestBuilder == nil {
		return nil, apperror.Internal("agent runtime request builder is not configured")
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
		runtime:        config.Runtime,
		requestBuilder: config.RequestBuilder,
		audit:          config.Audit,
		writer:         config.Writer,
		llmobsSink:     config.LLMObservabilitySink,
		now:            now,
	}, nil
}

func (o *AgentRunOrchestrator) Run(ctx context.Context, trigger AgentTrigger) (AgentRunOrchestratorResult, error) {
	ctx, span := observability.StartSpan(ctx, "agent.run")
	defer span.End()
	if o == nil || o.runtime == nil {
		err := apperror.Internal("agent runtime is not configured")
		observability.RecordSpanError(span, err)
		return AgentRunOrchestratorResult{}, err
	}
	if o.audit == nil {
		return AgentRunOrchestratorResult{}, apperror.Internal("agent audit recorder is not configured")
	}
	if o.requestBuilder == nil {
		return AgentRunOrchestratorResult{}, apperror.Internal("agent runtime request builder is not configured")
	}
	if o.writer == nil {
		return AgentRunOrchestratorResult{}, apperror.Internal("agent response writer is not configured")
	}

	normalized, err := normalizeAgentTriggerForRun(trigger)
	if err != nil {
		observability.RecordSpanError(span, err)
		return AgentRunOrchestratorResult{}, err
	}
	if normalized.TraceID == "" {
		traceContext := observability.TraceContextFromContext(ctx)
		normalized.TraceID = traceContext.TraceID
	}

	now := o.now
	if now == nil {
		now = time.Now
	}
	startedAt := now().UTC()

	runtimeReq, err := o.requestBuilder.BuildRuntimeRequest(ctx, normalized)
	if err != nil {
		observability.RecordSpanError(span, err)
		finishedAt := now().UTC()
		o.observeLLMRun(ctx, llmObsFailedFromTrigger(normalized, "", startedAt, finishedAt, "runtime_request_error", err))
		auditRun, auditErr := o.recordFailedRun(ctx, normalized, "", startedAt, finishedAt, "runtime_request_error", err)
		if auditErr != nil {
			return AgentRunOrchestratorResult{}, errors.Join(err, fmt.Errorf("record failed agent run audit: %w", auditErr))
		}
		o.notifyRunFailure(ctx, normalized, auditRun.RunID, "runtime_request_error", err)
		return AgentRunOrchestratorResult{AuditRun: auditRun}, err
	}
	runtimeReq, err = normalizeRuntimeRequestForTrigger(runtimeReq, normalized)
	if err != nil {
		observability.RecordSpanError(span, err)
		finishedAt := now().UTC()
		o.observeLLMRun(ctx, llmObsFailedFromRequest(normalized, runtimeReq, agentruntime.RunResult{}, startedAt, finishedAt, "runtime_request_invalid", err))
		auditRun, auditErr := o.recordFailedRun(ctx, normalized, runtimeReq.RunID, startedAt, finishedAt, "runtime_request_invalid", err)
		if auditErr != nil {
			return AgentRunOrchestratorResult{}, errors.Join(err, fmt.Errorf("record failed agent run audit: %w", auditErr))
		}
		o.notifyRunFailure(ctx, normalized, auditRun.RunID, "runtime_request_invalid", err)
		return AgentRunOrchestratorResult{AuditRun: auditRun}, err
	}

	o.observeLLMRun(ctx, llmObsStartedFromRequest(normalized, runtimeReq, startedAt))
	runtimeResult, err := o.runtime.Run(ctx, runtimeReq)
	finishedAt := now().UTC()
	if err != nil {
		observability.RecordSpanError(span, err)
		o.observeLLMRun(ctx, llmObsFailedFromRequest(normalized, runtimeReq, agentruntime.RunResult{}, startedAt, finishedAt, "runtime_error", err))
		auditRun, auditErr := o.recordFailedRun(ctx, normalized, runtimeReq.RunID, startedAt, finishedAt, "runtime_error", err)
		if auditErr != nil {
			return AgentRunOrchestratorResult{}, errors.Join(err, fmt.Errorf("record failed agent run audit: %w", auditErr))
		}
		o.notifyRunFailure(ctx, normalized, auditRun.RunID, "runtime_error", err)
		return AgentRunOrchestratorResult{AuditRun: auditRun}, err
	}
	runtimeResult, err = agentruntime.NormalizeRunResult(runtimeResult)
	if err != nil {
		observability.RecordSpanError(span, err)
		code := "runtime_result_invalid"
		if strings.Contains(err.Error(), "final_text") {
			code = "empty_final_text"
		}
		o.observeLLMRun(ctx, llmObsFailedFromRequest(normalized, runtimeReq, runtimeResult, startedAt, finishedAt, code, err))
		auditRun, auditErr := o.recordFailedRun(ctx, normalized, runtimeResult.RunID, startedAt, finishedAt, code, err)
		if auditErr != nil {
			return AgentRunOrchestratorResult{}, errors.Join(err, fmt.Errorf("record failed agent run audit: %w", auditErr))
		}
		o.notifyRunFailure(ctx, normalized, auditRun.RunID, code, err)
		return AgentRunOrchestratorResult{AuditRun: auditRun}, err
	}
	runtimeResult.FinalText = strings.TrimSpace(runtimeResult.FinalText)
	if runtimeReq.RunID != "" && runtimeResult.RunID != runtimeReq.RunID {
		err := apperror.Internal("runtime returned mismatched run_id")
		observability.RecordSpanError(span, err)
		o.observeLLMRun(ctx, llmObsFailedFromRequest(normalized, runtimeReq, runtimeResult, startedAt, finishedAt, "runtime_result_invalid", err))
		auditRun, auditErr := o.recordFailedRun(ctx, normalized, runtimeReq.RunID, startedAt, finishedAt, "runtime_result_invalid", err)
		if auditErr != nil {
			return AgentRunOrchestratorResult{}, errors.Join(err, fmt.Errorf("record failed agent run audit: %w", auditErr))
		}
		o.notifyRunFailure(ctx, normalized, auditRun.RunID, "runtime_result_invalid", err)
		return AgentRunOrchestratorResult{AuditRun: auditRun}, err
	}

	finalText := runtimeResult.FinalText

	auditRun, err := o.audit.RecordAgentRun(ctx, agentaudit.CreateRunInput{
		RunID:            runtimeResult.RunID,
		AgentID:          normalized.AgentUserID,
		ConversationID:   normalized.ConversationID,
		TriggerMessageID: normalized.TriggerMessageID,
		RequestingUserID: normalized.RequestingUserID,
		Status:           agentaudit.StatusSucceeded,
		InputSummary:     agentRunInputSummary(normalized),
		OutputSummary:    agentRunOutputSummary(runtimeResult),
		TraceID:          normalized.TraceID,
		RequestID:        normalized.RequestID,
		StartedAt:        startedAt,
		FinishedAt:       finishedAt,
	})
	if err != nil {
		observability.RecordSpanError(span, err)
		return AgentRunOrchestratorResult{}, fmt.Errorf("record agent run audit: %w", err)
	}
	if strings.TrimSpace(auditRun.RunID) == "" {
		err := apperror.Internal("agent audit returned empty run_id")
		observability.RecordSpanError(span, err)
		return AgentRunOrchestratorResult{}, err
	}

	responseReq, err := buildAgentResponseRequest(normalized, auditRun.RunID, finalText, allowRecursiveTriggerFromRuntimeResult(runtimeReq.Agent.Policy, runtimeResult))
	if err != nil {
		observability.RecordSpanError(span, err)
		return AgentRunOrchestratorResult{}, err
	}
	response, err := o.writer.WriteAgentResponse(ctx, responseReq)
	if err != nil {
		observability.RecordSpanError(span, err)
		o.observeLLMRun(ctx, llmObsFailedFromRequest(normalized, runtimeReq, runtimeResult, startedAt, finishedAt, "response_write_error", err))
		return AgentRunOrchestratorResult{}, fmt.Errorf("write agent response through message service: %w", err)
	}
	o.observeLLMRun(ctx, llmObsSucceededFromResult(normalized, runtimeReq, runtimeResult, response, startedAt, finishedAt))

	return AgentRunOrchestratorResult{
		AuditRun: auditRun,
		Response: response,
	}, nil
}

func (o *AgentRunOrchestrator) observeLLMRun(ctx context.Context, event llmobs.Event) {
	if o == nil || o.llmobsSink == nil {
		return
	}
	result, err := o.llmobsSink.Observe(ctx, event)
	if err != nil {
		backend := result.Backend
		if backend == "" {
			backend = llmobs.BackendNoop
		}
		logx.Errorf("llm observability export failed backend=%q trace_id=%q request_id=%q event_type=%q status=%q: %s",
			backend, event.TraceID, event.RequestID, event.Type, event.Status, llmobs.RedactPlainText(err.Error()))
	}
}

func (o *AgentRunOrchestrator) recordFailedRun(ctx context.Context, trigger AgentTrigger, runID string, startedAt time.Time, finishedAt time.Time, code string, cause error) (agentaudit.AgentRun, error) {
	return o.audit.RecordAgentRun(ctx, agentaudit.CreateRunInput{
		RunID:            runID,
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
}

func (o *AgentRunOrchestrator) notifyRunFailure(ctx context.Context, trigger AgentTrigger, runID string, code string, cause error) {
	if o == nil || o.writer == nil {
		return
	}
	text := userVisibleRunFailureText(code, cause)
	if strings.TrimSpace(text) == "" {
		return
	}
	responseReq, err := buildAgentResponseRequest(trigger, runID, text, false)
	if err != nil {
		logx.Errorf("build agent failure notice failed request_id=%q conversation_id=%q agent_account_id=%q: %v",
			trigger.RequestID, trigger.ConversationID, trigger.AgentUserID, err)
		return
	}
	if _, err := o.writer.WriteAgentResponse(ctx, responseReq); err != nil {
		logx.Errorf("write agent failure notice failed request_id=%q conversation_id=%q agent_account_id=%q: %v",
			trigger.RequestID, trigger.ConversationID, trigger.AgentUserID, err)
	}
}

func userVisibleRunFailureText(code string, cause error) string {
	category := "服务暂时不可用"
	message := ""
	if cause != nil {
		message = strings.ToLower(cause.Error())
		if app := apperror.From(cause); app.Message != "" {
			message = strings.ToLower(app.Message + " " + cause.Error())
		}
	}
	if strings.Contains(message, "tool not found") || strings.Contains(message, "tool") || strings.Contains(message, "工具") {
		category = "工具配置不可用"
	} else if strings.Contains(message, "timeout") || strings.Contains(message, "deadline") || strings.Contains(message, "超时") {
		category = "模型响应超时"
	} else if strings.Contains(message, "api key") || strings.Contains(message, "unauthorized") || strings.Contains(message, "forbidden") || strings.Contains(message, "权限") {
		category = "模型或工具权限配置不可用"
	} else if strings.Contains(strings.ToLower(strings.TrimSpace(code)), "empty_final_text") {
		category = "模型没有返回有效回复"
	}
	return "抱歉，AI 助手这次处理失败了。错误类型：" + category + "。我已经记录了这次失败，请稍后重试或联系管理员排查。"
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

func normalizeRuntimeRequestForTrigger(req agentruntime.RunRequest, trigger AgentTrigger) (agentruntime.RunRequest, error) {
	normalized, err := agentruntime.NormalizeRunRequest(req)
	if err != nil {
		return agentruntime.RunRequest{}, err
	}
	if normalized.RequestID != trigger.RequestID {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime request_id must match trigger")
	}
	if normalized.EventID != trigger.EventID {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime event_id must match trigger")
	}
	if normalized.OperationID != trigger.OperationID {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime operation_id must match trigger")
	}
	if normalized.TraceID != trigger.TraceID {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime trace_id must match trigger")
	}
	if normalized.TriggerType != trigger.TriggerType {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime trigger_type must match trigger")
	}
	if normalized.AgentUserID != trigger.AgentUserID {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime agent_user_id must match trigger")
	}
	if normalized.RequestingUserID != trigger.RequestingUserID {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime requesting_user_id must match trigger")
	}
	if normalized.ConversationID != trigger.ConversationID {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime conversation_id must match trigger")
	}
	if normalized.ConversationType != trigger.ConversationType {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime conversation_type must match trigger")
	}
	if normalized.TriggerMessageID != trigger.TriggerMessageID {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime trigger_message_id must match trigger")
	}
	if normalized.TriggerSeq != trigger.TriggerSeq {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime trigger_seq must match trigger")
	}
	if normalized.ReplyToMessageID != trigger.ReplyToMessageID {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime reply_to_message_id must match trigger")
	}
	if normalized.SourceAgentRunID != trigger.SourceAgentRunID {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime source_agent_run_id must match trigger")
	}
	if normalized.SourceAgentUserID != trigger.SourceAgentUserID {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime source_agent_user_id must match trigger")
	}
	if normalized.SourceMessageID != trigger.SourceMessageID {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime source_message_id must match trigger")
	}
	if normalized.SourceMessageSeq != trigger.SourceMessageSeq {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime source_message_seq must match trigger")
	}
	if normalized.SourceMessageText != trigger.SourceMessageText {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime source_message_text must match trigger")
	}
	if normalized.SourceContentType != trigger.SourceContentType {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime source_content_type must match trigger")
	}
	if !sameStringSlice(normalized.TargetAgentUserIDs, trigger.TargetAgentUserIDs) {
		return agentruntime.RunRequest{}, apperror.InvalidArgument("runtime target_agent_user_ids must match trigger")
	}
	return normalized, nil
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

func sameStringSlice(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func agentRunOutputSummary(result agentruntime.RunResult) agentaudit.Summary {
	summary := agentaudit.Summary{
		"final_text":       result.FinalText,
		"final_text_bytes": len([]byte(result.FinalText)),
	}
	if result.FinishReason != "" {
		summary["finish_reason"] = result.FinishReason
	}
	if result.Model.Provider != "" || result.Model.Model != "" || result.Model.ResponseID != "" {
		summary["model"] = agentaudit.Summary{
			"provider":    result.Model.Provider,
			"model":       result.Model.Model,
			"version":     result.Model.ModelVersion,
			"response_id": result.Model.ResponseID,
		}
	}
	if result.Usage.TotalTokens > 0 || result.Usage.PromptTokens > 0 || result.Usage.CompletionTokens > 0 || result.Usage.ReasoningTokens > 0 || result.Usage.CachedTokens > 0 {
		summary["usage"] = agentaudit.Summary{
			"prompt_tokens":     result.Usage.PromptTokens,
			"completion_tokens": result.Usage.CompletionTokens,
			"reasoning_tokens":  result.Usage.ReasoningTokens,
			"cached_tokens":     result.Usage.CachedTokens,
			"total_tokens":      result.Usage.TotalTokens,
		}
	}
	if len(result.ToolCalls) > 0 {
		toolCalls := make([]agentaudit.Summary, 0, len(result.ToolCalls))
		for _, call := range result.ToolCalls {
			toolCalls = append(toolCalls, agentaudit.Summary{
				"tool_call_id":  call.ToolCallID,
				"tool_id":       call.ToolID,
				"tool_name":     call.ToolName,
				"status":        call.Status,
				"error_code":    call.ErrorCode,
				"duration_ms":   call.DurationMs,
				"metadata_keys": mapKeys(call.Metadata),
			})
		}
		summary["tool_calls"] = toolCalls
	}
	if len(result.Metadata) > 0 {
		summary["metadata"] = result.Metadata
	}
	return summary
}

func allowRecursiveTriggerFromRuntimeResult(policy agentruntime.RuntimePolicy, result agentruntime.RunResult) bool {
	if !policy.AllowAgentMessageRecursion {
		return false
	}
	raw := strings.TrimSpace(result.Metadata["allow_recursive_trigger"])
	if raw == "" {
		return false
	}
	allowed, err := strconv.ParseBool(raw)
	return err == nil && allowed
}

func mapKeys(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
