package eino

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/wujunhui99/agents_im/internal/agentruntime"
	llmdeepseek "github.com/wujunhui99/agents_im/internal/agentruntime/llm/deepseek"
	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/idgen"
)

type DeepSeekRuntime struct {
	cfg config.DeepSeekConfig
}

func NewDeepSeekRuntime(cfg config.DeepSeekConfig) *DeepSeekRuntime {
	return &DeepSeekRuntime{cfg: cfg}
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
