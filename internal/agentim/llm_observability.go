package agentim

import (
	"strconv"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/agentruntime"
	"github.com/wujunhui99/agents_im/internal/llmobs"
)

func llmObsStartedFromRequest(trigger AgentTrigger, req agentruntime.RunRequest, startedAt time.Time) llmobs.Event {
	event := llmObsBaseEvent(trigger, req)
	event.Type = llmobs.EventTypeRun
	event.Status = llmobs.StatusStarted
	event.StartedAt = startedAt.UTC()
	return event
}

func llmObsSucceededFromResult(trigger AgentTrigger, req agentruntime.RunRequest, result agentruntime.RunResult, response AgentResponseResult, startedAt time.Time, finishedAt time.Time) llmobs.Event {
	event := llmObsBaseEvent(trigger, req)
	event.Type = llmobs.EventTypeRun
	event.Status = llmobs.StatusSucceeded
	event.AgentRunID = strings.TrimSpace(result.RunID)
	if event.ModelProvider == "" {
		event.ModelProvider = strings.TrimSpace(result.Model.Provider)
	}
	if event.ModelName == "" {
		event.ModelName = strings.TrimSpace(result.Model.Model)
	}
	event.ResponseServerMsgID = strings.TrimSpace(response.Message.ServerMsgID)
	event.StartedAt = startedAt.UTC()
	event.FinishedAt = finishedAt.UTC()
	event.LatencyMs = nonNegativeDurationMs(event.FinishedAt.Sub(event.StartedAt))
	event.Generation = llmObsGeneration(req, result, event.LatencyMs)
	return event
}

func llmObsFailedFromTrigger(trigger AgentTrigger, runID string, startedAt time.Time, finishedAt time.Time, code string, err error) llmobs.Event {
	event := llmobs.Event{
		Type:                 llmobs.EventTypeRun,
		Status:               llmobs.StatusFailed,
		TraceID:              strings.TrimSpace(trigger.TraceID),
		RequestID:            strings.TrimSpace(trigger.RequestID),
		AgentRunID:           strings.TrimSpace(runID),
		ConversationID:       strings.TrimSpace(trigger.ConversationID),
		TriggerServerMsgID:   strings.TrimSpace(trigger.TriggerMessageID),
		HostedOwnerAccountID: strings.TrimSpace(trigger.AgentUserID),
		SenderAccountID:      strings.TrimSpace(trigger.RequestingUserID),
		AgentAccountID:       strings.TrimSpace(trigger.AgentUserID),
		RuntimeMode:          llmobs.RuntimeModeAIHostingAutoReply,
		StartedAt:            startedAt.UTC(),
		FinishedAt:           finishedAt.UTC(),
	}
	event.LatencyMs = nonNegativeDurationMs(event.FinishedAt.Sub(event.StartedAt))
	event.ErrorClass, event.ErrorMessage = llmobs.ErrorFields(err)
	if strings.TrimSpace(code) != "" {
		event.ErrorClass = strings.TrimSpace(code)
	}
	return event
}

func llmObsFailedFromRequest(trigger AgentTrigger, req agentruntime.RunRequest, result agentruntime.RunResult, startedAt time.Time, finishedAt time.Time, code string, err error) llmobs.Event {
	event := llmObsBaseEvent(trigger, req)
	event.Type = llmobs.EventTypeRun
	event.Status = llmobs.StatusFailed
	event.AgentRunID = firstNonEmpty(result.RunID, req.RunID)
	event.StartedAt = startedAt.UTC()
	event.FinishedAt = finishedAt.UTC()
	event.LatencyMs = nonNegativeDurationMs(event.FinishedAt.Sub(event.StartedAt))
	event.Generation = llmObsGeneration(req, result, event.LatencyMs)
	event.ErrorClass, event.ErrorMessage = llmobs.ErrorFields(err)
	if strings.TrimSpace(code) != "" {
		event.ErrorClass = strings.TrimSpace(code)
	}
	return event
}

func llmObsBaseEvent(trigger AgentTrigger, req agentruntime.RunRequest) llmobs.Event {
	promptVersion := strings.TrimSpace(req.Agent.Prompt.Version)
	promptHash := llmobs.PromptHash(req.Agent.Prompt.Content)
	modelProvider := strings.TrimSpace(req.Agent.Model.Provider)
	modelName := strings.TrimSpace(req.Agent.Model.Model)
	runtimeMode := strings.TrimSpace(req.Metadata["runtime_mode"])
	if runtimeMode == "" && strings.TrimSpace(req.Agent.Prompt.PromptID) == aiHostingPromptID {
		runtimeMode = llmobs.RuntimeModeAIHostingAutoReply
	}
	if runtimeMode == "" {
		runtimeMode = strings.TrimSpace(req.TriggerType)
	}
	return llmobs.Event{
		TraceID:              strings.TrimSpace(trigger.TraceID),
		RequestID:            strings.TrimSpace(trigger.RequestID),
		AgentRunID:           strings.TrimSpace(req.RunID),
		ConversationID:       strings.TrimSpace(trigger.ConversationID),
		TriggerServerMsgID:   strings.TrimSpace(trigger.TriggerMessageID),
		HostedOwnerAccountID: strings.TrimSpace(trigger.AgentUserID),
		SenderAccountID:      strings.TrimSpace(trigger.RequestingUserID),
		AgentAccountID:       strings.TrimSpace(trigger.AgentUserID),
		ModelProvider:        modelProvider,
		ModelName:            modelName,
		PromptVersion:        promptVersion,
		PromptHash:           promptHash,
		RuntimeMode:          runtimeMode,
		Generation:           llmObsGeneration(req, agentruntime.RunResult{}, 0),
	}
}

func llmObsGeneration(req agentruntime.RunRequest, result agentruntime.RunResult, latencyMs int64) llmobs.Generation {
	generation := llmobs.Generation{
		BoundedRecentMessageCount: recentMessageCount(req),
		TriggerInContext:          triggerInContext(req),
		FinishReason:              strings.TrimSpace(result.FinishReason),
		PromptTokens:              result.Usage.PromptTokens,
		CompletionTokens:          result.Usage.CompletionTokens,
		ReasoningTokens:           result.Usage.ReasoningTokens,
		CachedTokens:              result.Usage.CachedTokens,
		TotalTokens:               result.Usage.TotalTokens,
		LatencyMs:                 latencyMs,
		FinalOutputSummary:        llmobs.TextSummary(result.FinalText),
	}
	return generation
}

func recentMessageCount(req agentruntime.RunRequest) int {
	if raw := strings.TrimSpace(req.Metadata["recent_message_count"]); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed >= 0 {
			return parsed
		}
	}
	return len(req.Conversation)
}

func triggerInContext(req agentruntime.RunRequest) bool {
	triggerMessageID := strings.TrimSpace(req.TriggerMessageID)
	for _, message := range req.Conversation {
		if triggerMessageID != "" && strings.TrimSpace(message.ServerMsgID) == triggerMessageID {
			return true
		}
		if req.TriggerSeq > 0 && message.Seq == req.TriggerSeq {
			return true
		}
	}
	return false
}

func nonNegativeDurationMs(duration time.Duration) int64 {
	if duration < 0 {
		return 0
	}
	return duration.Milliseconds()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
