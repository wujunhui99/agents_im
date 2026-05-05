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
		if message.SenderType == agentruntime.SenderTypeAgent {
			messages = append(messages, schema.AssistantMessage(text, nil))
			continue
		}
		messages = append(messages, schema.UserMessage(text))
	}
	if len(messages) == 1 {
		messages = append(messages, schema.UserMessage(req.PromptText))
	}
	return messages
}
