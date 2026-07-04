package orchestrator

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/privconv"
	agentruntime "github.com/wujunhui99/agents_im/service/agent/rpc/internal/runtime"
)

// stubPrivConvStore 只为请求构建器测试提供固定 rounds 快照；写路径不被调用。
type stubPrivConvStore struct {
	snapshot privconv.Snapshot
}

func (s stubPrivConvStore) AppendUserMessage(context.Context, privconv.AppendUserMessageInput) (bool, error) {
	return false, nil
}
func (s stubPrivConvStore) CommitRound(context.Context, privconv.CommitRoundInput) (bool, error) {
	return false, nil
}
func (s stubPrivConvStore) DiscardPending(context.Context, privconv.DiscardPendingInput) (bool, error) {
	return false, nil
}
func (s stubPrivConvStore) Load(context.Context, string) (privconv.Snapshot, error) {
	return s.snapshot, nil
}

// 私聊请求构建器（PrivateConvContext）从 store rounds 展开成 user/assistant 交互历史（每轮 user
// 批次改行合并成一条 user），当轮消息由 PromptText 承载，不调 msg-rpc。
func TestBuildRuntimeRequestFromPrivConv(t *testing.T) {
	store := stubPrivConvStore{snapshot: privconv.Snapshot{
		Rounds: []privconv.Round{
			{User: []string{"你好", "在吗"}, Assistant: "在的，有什么可以帮你？", SeqFrom: 1, SeqTo: 2, AgentRunID: "run-1"},
			{User: []string{"帮我查下天气"}, Assistant: "北京今天晴。", SeqFrom: 3, SeqTo: 3},
		},
	}}
	builder := NewConversationAIHostingRuntimeRequestBuilder(ConversationAIHostingRuntimeRequestBuilderConfig{
		DeepSeek:      config.DeepSeekConfig{Model: "deepseek-chat"},
		PrivConvStore: store,
	})

	trigger := AgentTrigger{
		RequestID:          "evt-9:agent:b5",
		TriggerType:        TriggerTypeUserPrivateMessage,
		AgentUserID:        "agent",
		RequestingUserID:   "peer",
		ConversationID:     "single:peer:agent",
		ConversationType:   ConversationTypeSingle,
		TriggerMessageID:   "m5",
		TriggerSeq:         5,
		PromptText:         "明天呢\n后天呢",
		PrivateConvContext: true,
	}

	req, err := builder.BuildRuntimeRequest(context.Background(), trigger)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// 历史 = 2 轮 × (user 合并 + assistant) = 4 条交互消息，严格 user/assistant 交互。
	if len(req.Conversation) != 4 {
		t.Fatalf("conversation len = %d, want 4", len(req.Conversation))
	}
	want := []struct {
		senderType string
		text       string
	}{
		{agentruntime.SenderTypeUser, "你好\n在吗"},
		{agentruntime.SenderTypeAgent, "在的，有什么可以帮你？"},
		{agentruntime.SenderTypeUser, "帮我查下天气"},
		{agentruntime.SenderTypeAgent, "北京今天晴。"},
	}
	for i, w := range want {
		got := req.Conversation[i]
		if got.SenderType != w.senderType || got.Text != w.text {
			t.Fatalf("conversation[%d] = (%s,%q), want (%s,%q)", i, got.SenderType, got.Text, w.senderType, w.text)
		}
	}
	// 当轮 user 批次由 PromptText 承载（合流后的多条追击消息）。
	if req.PromptText != "明天呢\n后天呢" {
		t.Fatalf("prompt_text = %q, want merged batch", req.PromptText)
	}
	if req.Metadata["private_conv_context"] != "true" {
		t.Fatalf("metadata missing private_conv_context marker: %+v", req.Metadata)
	}
	// 全程未触碰 msg-rpc（messageHistory 为 nil 也不 panic/报错）。
	if req.TriggerSeq != 5 || req.TriggerMessageID != "m5" {
		t.Fatalf("trigger passthrough mismatch: seq=%d msg=%q", req.TriggerSeq, req.TriggerMessageID)
	}
}
