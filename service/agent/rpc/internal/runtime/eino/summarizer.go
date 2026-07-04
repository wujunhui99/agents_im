package eino

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/orchestrator"
	llmdeepseek "github.com/wujunhui99/agents_im/service/agent/rpc/internal/runtime/llm/deepseek"
)

// DeepSeekSummarizer 用 DeepSeek 把「旧长期记忆 + 旧用户画像 + 一批待折叠的轮」重摘要成新的
// 长期记忆与用户画像（#688）。实现 orchestrator.ConversationSummarizer。
type DeepSeekSummarizer struct {
	cfg              config.DeepSeekConfig
	chatModelFactory deepSeekChatModelFactory
}

var _ orchestrator.ConversationSummarizer = (*DeepSeekSummarizer)(nil)

func NewDeepSeekSummarizer(cfg config.DeepSeekConfig) *DeepSeekSummarizer {
	return &DeepSeekSummarizer{cfg: cfg, chatModelFactory: llmdeepseek.NewChatModel}
}

func (s *DeepSeekSummarizer) Summarize(ctx context.Context, in orchestrator.SummarizeInput) (orchestrator.SummarizeResult, error) {
	if s == nil {
		return orchestrator.SummarizeResult{}, apperror.Internal("deepseek summarizer is not configured")
	}
	factory := s.chatModelFactory
	if factory == nil {
		factory = llmdeepseek.NewChatModel
	}
	cm, err := factory(ctx, s.cfg)
	if err != nil {
		return orchestrator.SummarizeResult{}, err
	}
	messages := []*schema.Message{
		schema.SystemMessage(summarizerSystemPrompt(in.MaxMemoryChars, in.MaxProfileChars)),
		schema.UserMessage(summarizerUserPayload(in)),
	}
	resp, err := cm.Generate(ctx, messages)
	if err != nil {
		return orchestrator.SummarizeResult{}, fmt.Errorf("deepseek summarize generate: %w", err)
	}
	if resp == nil || strings.TrimSpace(resp.Content) == "" {
		return orchestrator.SummarizeResult{}, apperror.Internal("deepseek summarize returned empty content")
	}
	memory, profile, err := parseSummaryJSON(resp.Content)
	if err != nil {
		return orchestrator.SummarizeResult{}, err
	}
	return orchestrator.SummarizeResult{Memory: memory, Profile: profile}, nil
}

func summarizerSystemPrompt(maxMemoryChars, maxProfileChars int) string {
	if maxMemoryChars <= 0 {
		maxMemoryChars = 4096
	}
	if maxProfileChars <= 0 {
		maxProfileChars = 2048
	}
	return strings.TrimSpace(fmt.Sprintf(`你是一个对话记忆压缩器。你会收到「已有长期记忆」「已有用户画像」和「最近若干轮对话」。
请把它们**融合重写**成新的长期记忆和用户画像，供后续对话保持连贯。

要求：
- long_term_memory：概括这些对话与既有记忆里对未来对话有用的事实、结论、任务进展、约定；**允许丢弃**不再重要的旧信息，务必精简，**不超过 %d 个字符**。
- user_profile：只保留关于用户本人的稳定信息（身份、职业、长期偏好、称呼、语言、目标等）；对话里出现的新个人信息要合并进来，过时的可更新或删除，**不超过 %d 个字符**。
- 用中文，客观记述，不要复述系统规则，不要编造。
- 只输出一个 JSON 对象，形如 {"memory":"...","profile":"..."}，不要额外文字或代码块。`, maxMemoryChars, maxProfileChars))
}

func summarizerUserPayload(in orchestrator.SummarizeInput) string {
	var b strings.Builder
	b.WriteString("已有长期记忆：\n")
	if m := strings.TrimSpace(in.ExistingMemory); m != "" {
		b.WriteString(m)
	} else {
		b.WriteString("(无)")
	}
	b.WriteString("\n\n已有用户画像：\n")
	if p := strings.TrimSpace(in.ExistingProfile); p != "" {
		b.WriteString(p)
	} else {
		b.WriteString("(无)")
	}
	b.WriteString("\n\n最近若干轮对话（user=用户，assistant=助手）：\n")
	for i, round := range in.Rounds {
		user := strings.TrimSpace(strings.Join(round.User, "\n"))
		if user != "" {
			b.WriteString(fmt.Sprintf("[%d] 用户: %s\n", i+1, user))
		}
		if a := strings.TrimSpace(round.Assistant); a != "" {
			b.WriteString(fmt.Sprintf("[%d] 助手: %s\n", i+1, a))
		}
	}
	return b.String()
}

// parseSummaryJSON 从模型输出里提取 {"memory","profile"}。容忍代码块围栏与前后噪声：截取首个 { 到末个 }。
func parseSummaryJSON(content string) (memory string, profile string, err error) {
	raw := strings.TrimSpace(content)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return "", "", apperror.Internal("deepseek summarize output is not JSON")
	}
	var parsed struct {
		Memory  string `json:"memory"`
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal([]byte(raw[start:end+1]), &parsed); err != nil {
		return "", "", fmt.Errorf("parse summarize json: %w", err)
	}
	return strings.TrimSpace(parsed.Memory), strings.TrimSpace(parsed.Profile), nil
}
