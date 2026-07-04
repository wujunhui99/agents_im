package orchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/agentaudit"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/privconv"
)

// fakePrivConvStore 是 privconv.Store 的内存实现，镜像真实合流状态机语义（idle→running 闸门、
// pending 累积、CommitRound 清消费+16轮裁剪、DiscardPending 清消费不落轮），供合流器白盒测试。
type fakePrivConvStore struct {
	mu           sync.Mutex
	rounds       []privconv.Round
	pending      []privconv.PendingMessage
	state        string
	lastSeq      int64
	commitCalls  int
	discardCalls int
	// onLoad 在每次 Load 后回调，用来模拟"run 在途时又来消息"。
	onLoad func(s *fakePrivConvStore)
}

func newFakePrivConvStore() *fakePrivConvStore {
	return &fakePrivConvStore{state: "idle"}
}

func (s *fakePrivConvStore) AppendUserMessage(_ context.Context, in privconv.AppendUserMessageInput) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if in.Message.Seq <= s.lastSeq {
		return false, nil
	}
	s.pending = append(s.pending, in.Message)
	s.lastSeq = in.Message.Seq
	if s.state == "idle" {
		s.state = "running"
		return true, nil
	}
	return false, nil
}

func (s *fakePrivConvStore) Load(_ context.Context, _ string) (privconv.Snapshot, error) {
	s.mu.Lock()
	snap := privconv.Snapshot{
		Rounds:  append([]privconv.Round(nil), s.rounds...),
		Pending: append([]privconv.PendingMessage(nil), s.pending...),
		State:   s.state,
	}
	cb := s.onLoad
	s.mu.Unlock()
	if cb != nil {
		cb(s)
	}
	return snap, nil
}

func (s *fakePrivConvStore) CommitRound(_ context.Context, in privconv.CommitRoundInput) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commitCalls++
	s.rounds = append(s.rounds, in.Round)
	if max := in.MaxRounds; max > 0 && len(s.rounds) > max {
		s.rounds = s.rounds[len(s.rounds)-max:]
	}
	s.dropConsumedLocked(in.ConsumedUpToSeq)
	return len(s.pending) > 0, nil
}

func (s *fakePrivConvStore) DiscardPending(_ context.Context, in privconv.DiscardPendingInput) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discardCalls++
	s.dropConsumedLocked(in.ConsumedUpToSeq)
	return len(s.pending) > 0, nil
}

func (s *fakePrivConvStore) dropConsumedLocked(consumedUpToSeq int64) {
	kept := s.pending[:0]
	for _, m := range s.pending {
		if m.Seq > consumedUpToSeq {
			kept = append(kept, m)
		}
	}
	s.pending = append([]privconv.PendingMessage(nil), kept...)
	if len(s.pending) > 0 {
		s.state = "running"
	} else {
		s.state = "idle"
	}
}

// recordingRunner 记录每轮 trigger 并按脚本返回回复文本。
type recordingRunner struct {
	mu       sync.Mutex
	triggers []AgentTrigger
	reply    func(t AgentTrigger) (string, error)
}

func (r *recordingRunner) Run(_ context.Context, trigger AgentTrigger) (AgentRunOrchestratorResult, error) {
	r.mu.Lock()
	r.triggers = append(r.triggers, trigger)
	reply := r.reply
	r.mu.Unlock()
	text, err := reply(trigger)
	if err != nil {
		return AgentRunOrchestratorResult{}, err
	}
	return AgentRunOrchestratorResult{
		AuditRun:  agentaudit.AgentRun{RunID: "run-" + trigger.TriggerMessageID},
		FinalText: text,
	}, nil
}

func baseTrigger() AgentTrigger {
	return AgentTrigger{
		RequestID:        "evt-1:agent",
		TriggerType:      TriggerTypeUserPrivateMessage,
		AgentUserID:      "agent",
		RequestingUserID: "peer",
		ConversationID:   "single:peer:agent",
		ConversationType: ConversationTypeSingle,
	}
}

func newCoalescer(t *testing.T, store privconv.Store, runner AgentTriggerRunner) *PrivateChatCoalescer {
	t.Helper()
	c, err := NewPrivateChatCoalescer(PrivateChatCoalescerConfig{Store: store, Runner: runner})
	if err != nil {
		t.Fatalf("new coalescer: %v", err)
	}
	return c
}

// 合流：两条消息在同一轮合并成一条 user 输入，只回复一次。
func TestPrivateCoalescerMergesBatchIntoOneRound(t *testing.T) {
	store := newFakePrivConvStore()
	runner := &recordingRunner{reply: func(tr AgentTrigger) (string, error) { return "reply:" + tr.PromptText, nil }}
	c := newCoalescer(t, store, runner)
	ctx := context.Background()

	if err := c.OnUserMessage(ctx, baseTrigger(), PrivateUserMessage{Seq: 1, ServerMsgID: "m1", Text: "帮我修复注册页 bug"}); err != nil {
		t.Fatalf("append m1: %v", err)
	}
	// 第二条在 running 期间到达，累积到 pending（fired=false）。
	if err := c.OnUserMessage(ctx, baseTrigger(), PrivateUserMessage{Seq: 2, ServerMsgID: "m2", Text: "补充：移动端按钮看不到"}); err != nil {
		t.Fatalf("append m2: %v", err)
	}

	if err := c.drive(baseTrigger()); err != nil {
		t.Fatalf("drive: %v", err)
	}

	if len(runner.triggers) != 1 {
		t.Fatalf("expected exactly 1 run (coalesced), got %d", len(runner.triggers))
	}
	got := runner.triggers[0]
	if !got.PrivateConvContext {
		t.Fatalf("run trigger must carry PrivateConvContext")
	}
	wantPrompt := "帮我修复注册页 bug\n补充：移动端按钮看不到"
	if got.PromptText != wantPrompt {
		t.Fatalf("merged prompt = %q, want %q", got.PromptText, wantPrompt)
	}
	if got.TriggerSeq != 2 || got.TriggerMessageID != "m2" {
		t.Fatalf("batch trigger must point at last message: seq=%d msg=%q", got.TriggerSeq, got.TriggerMessageID)
	}
	if len(store.rounds) != 1 {
		t.Fatalf("expected 1 committed round, got %d", len(store.rounds))
	}
	round := store.rounds[0]
	if len(round.User) != 2 || round.Assistant != "reply:"+wantPrompt {
		t.Fatalf("round mismatch: %+v", round)
	}
	if store.state != "idle" {
		t.Fatalf("state after drain = %q, want idle", store.state)
	}
}

// 实行后 pending 再检查：run 在途时新消息到达 → 收束后残留 → 再跑一轮。
func TestPrivateCoalescerRerunsOnPendingArrivedDuringRun(t *testing.T) {
	store := newFakePrivConvStore()
	// 首条落 pending 并占 running。
	if _, err := store.AppendUserMessage(context.Background(), privconv.AppendUserMessageInput{
		ConversationID: "single:peer:agent", AgentAccountID: "agent", PeerAccountID: "peer",
		Message: privconv.PendingMessage{Seq: 1, ServerMsgID: "m1", Text: "first"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// 第一轮 Load 后模拟"用户又发了 seq2"，仅注入一次。
	var once sync.Once
	store.onLoad = func(s *fakePrivConvStore) {
		once.Do(func() {
			s.mu.Lock()
			s.pending = append(s.pending, privconv.PendingMessage{Seq: 2, ServerMsgID: "m2", Text: "second"})
			s.lastSeq = 2
			s.mu.Unlock()
		})
	}
	runner := &recordingRunner{reply: func(tr AgentTrigger) (string, error) { return "ok:" + tr.PromptText, nil }}
	c := newCoalescer(t, store, runner)

	if err := c.drive(baseTrigger()); err != nil {
		t.Fatalf("drive: %v", err)
	}
	if len(runner.triggers) != 2 {
		t.Fatalf("expected 2 runs (post-run recheck), got %d", len(runner.triggers))
	}
	if runner.triggers[0].PromptText != "first" || runner.triggers[1].PromptText != "second" {
		t.Fatalf("unexpected run prompts: %q, %q", runner.triggers[0].PromptText, runner.triggers[1].PromptText)
	}
	if len(store.rounds) != 2 {
		t.Fatalf("expected 2 rounds, got %d", len(store.rounds))
	}
	if store.state != "idle" {
		t.Fatalf("final state = %q, want idle", store.state)
	}
}

// run 失败：消费该批（DiscardPending），不落 assistant 历史，避免对同一失败批无限重试。
func TestPrivateCoalescerDiscardsBatchOnRunFailure(t *testing.T) {
	store := newFakePrivConvStore()
	if _, err := store.AppendUserMessage(context.Background(), privconv.AppendUserMessageInput{
		ConversationID: "single:peer:agent", AgentAccountID: "agent", PeerAccountID: "peer",
		Message: privconv.PendingMessage{Seq: 1, ServerMsgID: "m1", Text: "boom"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	runner := &recordingRunner{reply: func(AgentTrigger) (string, error) { return "", errors.New("llm exploded") }}
	c := newCoalescer(t, store, runner)

	if err := c.drive(baseTrigger()); err != nil {
		t.Fatalf("drive should not surface run error (already reported to user): %v", err)
	}
	if store.commitCalls != 0 {
		t.Fatalf("failed run must not commit a round, commitCalls=%d", store.commitCalls)
	}
	if store.discardCalls != 1 {
		t.Fatalf("failed run must discard the batch once, discardCalls=%d", store.discardCalls)
	}
	if len(store.rounds) != 0 || len(store.pending) != 0 {
		t.Fatalf("after failure: rounds=%d pending=%d, want 0/0", len(store.rounds), len(store.pending))
	}
	if store.state != "idle" {
		t.Fatalf("state after failure = %q, want idle", store.state)
	}
}

// running 闸门：首条抢到 running（fired），running 期间的次条只累积、不再 fired。
func TestPrivateCoalescerFiredGate(t *testing.T) {
	store := newFakePrivConvStore()
	ctx := context.Background()
	f1, _ := store.AppendUserMessage(ctx, privconv.AppendUserMessageInput{ConversationID: "c", AgentAccountID: "a", PeerAccountID: "p", Message: privconv.PendingMessage{Seq: 1, Text: "a"}})
	f2, _ := store.AppendUserMessage(ctx, privconv.AppendUserMessageInput{ConversationID: "c", AgentAccountID: "a", PeerAccountID: "p", Message: privconv.PendingMessage{Seq: 2, Text: "b"}})
	if !f1 || f2 {
		t.Fatalf("fired gate wrong: f1=%v f2=%v, want true/false", f1, f2)
	}
}
