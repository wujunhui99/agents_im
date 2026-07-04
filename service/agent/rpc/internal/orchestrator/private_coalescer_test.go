package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/agentaudit"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/privconv"
)

// fakePrivConvStore 是 privconv.Store 的内存实现，镜像真实合流状态机语义（idle→running 闸门、
// pending 累积、CommitRound 清消费+16轮裁剪、DiscardPending 清消费不落轮），供合流器白盒测试。
type fakePrivConvStore struct {
	mu             sync.Mutex
	rounds         []privconv.Round
	pending        []privconv.PendingMessage
	state          string
	lastSeq        int64
	commitCalls    int
	discardCalls   int
	summaryCalls   int
	longTermMemory string
	userProfile    string
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
		Rounds:         append([]privconv.Round(nil), s.rounds...),
		Pending:        append([]privconv.PendingMessage(nil), s.pending...),
		State:          s.state,
		LongTermMemory: s.longTermMemory,
		UserProfile:    s.userProfile,
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

func (s *fakePrivConvStore) ApplySummary(_ context.Context, in privconv.ApplySummaryInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summaryCalls++
	s.longTermMemory = in.LongTermMemory
	s.userProfile = in.UserProfile
	if in.CutoffSeq > 0 {
		kept := make([]privconv.Round, 0, len(s.rounds))
		for _, r := range s.rounds {
			if r.SeqTo > in.CutoffSeq {
				kept = append(kept, r)
			}
		}
		s.rounds = kept
	}
	return nil
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

// fakeSummarizer 记录调用并返回脚本化的记忆/画像（或错误）。
type fakeSummarizer struct {
	calls      int
	lastRounds int
	memory     string
	profile    string
	err        error
}

func (f *fakeSummarizer) Summarize(_ context.Context, in SummarizeInput) (SummarizeResult, error) {
	f.calls++
	f.lastRounds = len(in.Rounds)
	if f.err != nil {
		return SummarizeResult{}, f.err
	}
	return SummarizeResult{Memory: f.memory, Profile: f.profile}, nil
}

func seedRounds(store *fakePrivConvStore, n int) {
	for i := 1; i <= n; i++ {
		store.rounds = append(store.rounds, privconv.Round{
			User: []string{fmt.Sprintf("u%d", i)}, Assistant: fmt.Sprintf("a%d", i),
			SeqFrom: int64(i), SeqTo: int64(i),
		})
	}
	store.state = "running"
	store.lastSeq = int64(n)
}

func newSummarizingCoalescer(t *testing.T, store *fakePrivConvStore, sum ConversationSummarizer) *PrivateChatCoalescer {
	t.Helper()
	c, err := NewPrivateChatCoalescer(PrivateChatCoalescerConfig{
		Store:      store,
		Runner:     &recordingRunner{reply: func(AgentTrigger) (string, error) { return "x", nil }},
		Summarizer: sum,
	})
	if err != nil {
		t.Fatalf("new coalescer: %v", err)
	}
	return c
}

// 达阈值（20 轮）触发摘要：折叠前 18 轮进长期记忆/用户画像，保留后 2 轮。
func TestPrivateCoalescerSummarizesAtThreshold(t *testing.T) {
	store := newFakePrivConvStore()
	seedRounds(store, privconv.SummarizeThreshold) // 20 轮
	sum := &fakeSummarizer{memory: "压缩后的长期记忆", profile: "用户画像X"}
	c := newSummarizingCoalescer(t, store, sum)

	c.maybeSummarize("single:peer:agent")

	if sum.calls != 1 {
		t.Fatalf("summarizer calls=%d, want 1", sum.calls)
	}
	if sum.lastRounds != privconv.SummarizeThreshold-privconv.KeepRoundsAfterSummary {
		t.Fatalf("folded rounds=%d, want %d", sum.lastRounds, privconv.SummarizeThreshold-privconv.KeepRoundsAfterSummary)
	}
	if store.summaryCalls != 1 {
		t.Fatalf("ApplySummary calls=%d, want 1", store.summaryCalls)
	}
	if len(store.rounds) != privconv.KeepRoundsAfterSummary {
		t.Fatalf("rounds after summary=%d, want %d", len(store.rounds), privconv.KeepRoundsAfterSummary)
	}
	if store.rounds[0].Assistant != "a19" || store.rounds[1].Assistant != "a20" {
		t.Fatalf("kept rounds not the last 2: %+v", store.rounds)
	}
	if store.longTermMemory != "压缩后的长期记忆" || store.userProfile != "用户画像X" {
		t.Fatalf("memory/profile not applied: mem=%q prof=%q", store.longTermMemory, store.userProfile)
	}
}

// 未达阈值不摘要。
func TestPrivateCoalescerNoSummaryBelowThreshold(t *testing.T) {
	store := newFakePrivConvStore()
	seedRounds(store, privconv.SummarizeThreshold-1) // 19 轮
	sum := &fakeSummarizer{memory: "m"}
	c := newSummarizingCoalescer(t, store, sum)
	c.maybeSummarize("single:peer:agent")
	if sum.calls != 0 || store.summaryCalls != 0 {
		t.Fatalf("should not summarize below threshold: sum=%d apply=%d", sum.calls, store.summaryCalls)
	}
	if len(store.rounds) != privconv.SummarizeThreshold-1 {
		t.Fatalf("rounds changed unexpectedly: %d", len(store.rounds))
	}
}

// 摘要失败降级：不 ApplySummary、不丢轮，留待下次触发。
func TestPrivateCoalescerSummaryFailureIsGraceful(t *testing.T) {
	store := newFakePrivConvStore()
	seedRounds(store, privconv.SummarizeThreshold)
	sum := &fakeSummarizer{err: errors.New("llm down")}
	c := newSummarizingCoalescer(t, store, sum)
	c.maybeSummarize("single:peer:agent")
	if sum.calls != 1 {
		t.Fatalf("summarizer should be called once, got %d", sum.calls)
	}
	if store.summaryCalls != 0 {
		t.Fatalf("failed summary must not ApplySummary, got %d", store.summaryCalls)
	}
	if len(store.rounds) != privconv.SummarizeThreshold {
		t.Fatalf("rounds must stay intact on failure, got %d", len(store.rounds))
	}
}

// summarizer 为 nil 时不摘要（退回 CommitRound 安全裁剪）。
func TestPrivateCoalescerNoSummarizerConfigured(t *testing.T) {
	store := newFakePrivConvStore()
	seedRounds(store, privconv.SummarizeThreshold+5)
	c := newCoalescer(t, store, &recordingRunner{reply: func(AgentTrigger) (string, error) { return "x", nil }})
	c.maybeSummarize("single:peer:agent")
	if store.summaryCalls != 0 {
		t.Fatalf("nil summarizer must not ApplySummary, got %d", store.summaryCalls)
	}
}
