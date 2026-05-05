package eino

import (
	"context"
	"errors"
	"testing"

	"github.com/wujunhui99/agents_im/internal/agentruntime"
	"github.com/wujunhui99/agents_im/internal/config"
)

func TestDeepSeekRuntimeFailsClosedWhenProviderConfigMissing(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_MODEL", "")

	runtime := NewDeepSeekRuntime(config.DeepSeekConfig{})
	_, err := runtime.Run(context.Background(), agentruntime.RunRequest{
		RequestID:        "req_1",
		TriggerType:      agentruntime.TriggerTypeUserPrivateMessage,
		AgentUserID:      "usr_a",
		RequestingUserID: "usr_b",
		ConversationID:   "single:usr_a:usr_b",
		ConversationType: agentruntime.ConversationTypeSingle,
		TriggerMessageID: "msg_1",
		TriggerSeq:       1,
		PromptText:       "hello",
		Agent: agentruntime.AgentConfig{
			AgentID:     "ai-hosting:usr_a",
			AgentUserID: "usr_a",
			Status:      agentruntime.AgentStatusActive,
			Prompt: agentruntime.PromptRef{
				PromptID: "conversation-ai-hosting-v1",
				Content:  "Reply as the hosted user.",
			},
			Model: agentruntime.ModelConfig{
				Provider: "deepseek",
				Model:    config.DefaultDeepSeekModel,
			},
		},
	})
	if !errors.Is(err, config.ErrDeepSeekAPIKeyMissing) {
		t.Fatalf("runtime error = %v, want missing DeepSeek key", err)
	}
}
