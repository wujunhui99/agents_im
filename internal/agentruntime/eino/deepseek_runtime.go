package eino

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/schema"
	"github.com/wujunhui99/agents_im/internal/agentruntime"
	llmdeepseek "github.com/wujunhui99/agents_im/internal/agentruntime/llm/deepseek"
	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/idgen"
	"github.com/wujunhui99/agents_im/internal/llmobs"
)

type DeepSeekRuntime struct {
	cfg        config.DeepSeekConfig
	llmobsSink llmobs.Sink
	llmobsCfg  llmobs.Config
}

type DeepSeekRuntimeOption func(*DeepSeekRuntime)

func WithLLMObservability(sink llmobs.Sink, cfg llmobs.Config) DeepSeekRuntimeOption {
	return func(runtime *DeepSeekRuntime) {
		runtime.llmobsSink = sink
		runtime.llmobsCfg = cfg
	}
}

func NewDeepSeekRuntime(cfg config.DeepSeekConfig, opts ...DeepSeekRuntimeOption) *DeepSeekRuntime {
	runtime := &DeepSeekRuntime{cfg: cfg}
	for _, opt := range opts {
		if opt != nil {
			opt(runtime)
		}
	}
	return runtime
}

func (r *DeepSeekRuntime) Run(ctx context.Context, req agentruntime.RunRequest) (agentruntime.RunResult, error) {
	normalized, err := agentruntime.NormalizeRunRequest(req)
	if err != nil {
		return agentruntime.RunResult{}, err
	}
	cfg := config.ResolveDeepSeekConfig(r.cfg)
	if strings.TrimSpace(normalized.Agent.Model.Model) != "" {
		cfg.Model = normalized.Agent.Model.Model
	}
	cm, err := llmdeepseek.NewChatModel(ctx, cfg)
	if err != nil {
		return agentruntime.RunResult{}, err
	}

	startedAt := time.Now().UTC()
	if r.llmobsSink != nil {
		ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{
			Type:      "DeepSeek",
			Name:      cfg.Model,
			Component: components.ComponentOfChatModel,
		}, llmobs.NewEinoCallbackHandler(r.llmobsSink, llmObsBaseEvent(normalized, cfg), r.llmobsCfg))
	}
	resp, err := cm.Generate(ctx, runtimeMessages(normalized))
	finishedAt := time.Now().UTC()
	if err != nil {
		return agentruntime.RunResult{}, fmt.Errorf("deepseek generate AI hosting reply: %w", err)
	}
	if resp == nil {
		return agentruntime.RunResult{}, apperror.Internal("deepseek returned empty response")
	}
	runID := normalized.RunID
	if runID == "" {
		generated, err := idgen.NewString()
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		runID = "run_" + generated
	}
	result := agentruntime.RunResult{
		RunID:     runID,
		FinalText: strings.TrimSpace(resp.Content),
		Model: agentruntime.ModelMetadata{
			Provider: "deepseek",
			Model:    cfg.Model,
		},
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
	}
	if resp.ResponseMeta != nil {
		result.FinishReason = resp.ResponseMeta.FinishReason
		if resp.ResponseMeta.Usage != nil {
			result.Usage = agentruntime.Usage{
				PromptTokens:     int64(resp.ResponseMeta.Usage.PromptTokens),
				CompletionTokens: int64(resp.ResponseMeta.Usage.CompletionTokens),
				ReasoningTokens:  int64(resp.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens),
				CachedTokens:     int64(resp.ResponseMeta.Usage.PromptTokenDetails.CachedTokens),
				TotalTokens:      int64(resp.ResponseMeta.Usage.TotalTokens),
			}
		}
	}
	return result, nil
}

func llmObsBaseEvent(req agentruntime.RunRequest, cfg config.DeepSeekConfig) llmobs.Event {
	runtimeMode := strings.TrimSpace(req.Metadata["runtime_mode"])
	if runtimeMode == "" {
		runtimeMode = strings.TrimSpace(req.TriggerType)
	}
	return llmobs.Event{
		TraceID:              strings.TrimSpace(req.TraceID),
		RequestID:            strings.TrimSpace(req.RequestID),
		AgentRunID:           strings.TrimSpace(req.RunID),
		ConversationID:       strings.TrimSpace(req.ConversationID),
		TriggerServerMsgID:   strings.TrimSpace(req.TriggerMessageID),
		HostedOwnerAccountID: strings.TrimSpace(req.AgentUserID),
		SenderAccountID:      strings.TrimSpace(req.RequestingUserID),
		AgentAccountID:       strings.TrimSpace(req.AgentUserID),
		ModelProvider:        strings.TrimSpace(req.Agent.Model.Provider),
		ModelName:            strings.TrimSpace(cfg.Model),
		PromptVersion:        strings.TrimSpace(req.Agent.Prompt.Version),
		PromptHash:           llmobs.PromptHash(req.Agent.Prompt.Content),
		RuntimeMode:          runtimeMode,
		Generation: llmobs.Generation{
			BoundedRecentMessageCount: len(req.Conversation),
			TriggerInContext:          llmObsTriggerInContext(req),
		},
	}
}

func llmObsTriggerInContext(req agentruntime.RunRequest) bool {
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

func runtimeMessages(req agentruntime.RunRequest) []*schema.Message {
	messages := []*schema.Message{schema.SystemMessage(req.Agent.Prompt.Content)}
	for _, message := range req.Conversation {
		text := strings.TrimSpace(message.Text)
		if text == "" {
			continue
		}
		if isCurrentTriggerMessage(req, message) {
			continue
		}
		if message.SenderType == agentruntime.SenderTypeAgent {
			messages = append(messages, schema.AssistantMessage(text, nil))
			continue
		}
		messages = append(messages, schema.UserMessage(text))
	}
	if current := runtimeCurrentTaskMessage(req); current != "" {
		messages = append(messages, schema.UserMessage(current))
	}
	return messages
}

func isCurrentTriggerMessage(req agentruntime.RunRequest, message agentruntime.ConversationMessage) bool {
	triggerMessageID := strings.TrimSpace(req.TriggerMessageID)
	if triggerMessageID != "" && strings.TrimSpace(message.ServerMsgID) == triggerMessageID {
		return true
	}
	if req.TriggerSeq <= 0 || message.Seq != req.TriggerSeq {
		return false
	}
	text := strings.TrimSpace(message.Text)
	if source := strings.TrimSpace(req.SourceMessageText); source != "" && text == source {
		return true
	}
	return text != "" && text == strings.TrimSpace(req.PromptText)
}

func runtimeCurrentTaskMessage(req agentruntime.RunRequest) string {
	promptText := strings.TrimSpace(req.PromptText)
	if promptText == "" {
		return ""
	}
	return strings.TrimSpace(`当前需要回复的对方消息：
` + promptText + `

请直接生成要发送给对方的回复。如果这条消息提出明确问题、请求或任务，直接回答或完成；只有缺少必要信息导致无法完成时才简短询问澄清。不要只回复“可以”“好的”“你说说”等泛泛确认。`)
}
