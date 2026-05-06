package eino

import (
	"context"
	"errors"
	"strings"
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

func TestRuntimeMessagesUsesPromptTextAsCurrentTaskWhenConversationContextExists(t *testing.T) {
	clearTask := "请总结一下这段日志的风险点。"
	req := agentruntime.RunRequest{
		RequestID:        "req_1",
		TriggerType:      agentruntime.TriggerTypeUserPrivateMessage,
		AgentUserID:      "usr_a",
		RequestingUserID: "usr_b",
		ConversationID:   "single:usr_a:usr_b",
		ConversationType: agentruntime.ConversationTypeSingle,
		TriggerMessageID: "msg_2",
		TriggerSeq:       2,
		PromptText:       clearTask,
		Agent: agentruntime.AgentConfig{
			AgentID:     "ai-hosting:usr_a",
			AgentUserID: "usr_a",
			Status:      agentruntime.AgentStatusActive,
			Prompt: agentruntime.PromptRef{
				PromptID: "conversation-ai-hosting-v1",
				Content:  "只输出要发送的回复文本。",
			},
			Model: agentruntime.ModelConfig{
				Provider: "deepseek",
				Model:    "deepseek-test",
			},
		},
		Conversation: []agentruntime.ConversationMessage{
			{
				ServerMsgID: "msg_1",
				Seq:         1,
				SenderID:    "usr_a",
				SenderType:  agentruntime.SenderTypeAgent,
				ContentType: agentruntime.ContentTypeText,
				Text:        "把日志发我。",
			},
			{
				ServerMsgID: "msg_2",
				Seq:         2,
				SenderID:    "usr_b",
				SenderType:  agentruntime.SenderTypeUser,
				ContentType: agentruntime.ContentTypeText,
				Text:        clearTask,
			},
		},
	}

	messages := runtimeMessages(req)
	if len(messages) < 3 {
		t.Fatalf("runtime messages = %+v, want system, prior context, and explicit current task", messages)
	}
	current := messages[len(messages)-1].Content
	for _, want := range []string{"当前需要回复的对方消息", clearTask, "直接回答或完成", "不要只回复"} {
		if !strings.Contains(current, want) {
			t.Fatalf("current task message missing %q: %q", want, current)
		}
	}
	if current == clearTask {
		t.Fatalf("current task should include direct-answer instructions, got only raw prompt_text")
	}
	for i, msg := range messages[:len(messages)-1] {
		if strings.Contains(msg.Content, "当前需要回复的对方消息") {
			t.Fatalf("current task instruction appeared before final message at index %d: %+v", i, messages)
		}
	}
}
