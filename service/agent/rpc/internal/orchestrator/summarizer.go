package orchestrator

import (
	"context"

	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/privconv"
)

// ConversationSummarizer 把「旧长期记忆 + 旧用户画像 + 一批待折叠的轮」重摘要成新的长期记忆
// 与用户画像（#688）。实现应保证输出精简、可丢弃旧信息，长度上限由 store 侧 ApplySummary 截断兜底。
type ConversationSummarizer interface {
	Summarize(ctx context.Context, in SummarizeInput) (SummarizeResult, error)
}

// SummarizeInput 是一次摘要的输入。Rounds 是被折叠的轮（通常是最早的 18 轮）。
type SummarizeInput struct {
	ExistingMemory  string
	ExistingProfile string
	Rounds          []privconv.Round
	MaxMemoryChars  int
	MaxProfileChars int
}

// SummarizeResult 是重摘要产物：新的长期记忆与用户画像。
type SummarizeResult struct {
	Memory  string
	Profile string
}
