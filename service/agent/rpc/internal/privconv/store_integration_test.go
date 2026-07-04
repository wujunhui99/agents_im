//go:build integration

package privconv_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/privconv"
)

// TestPostgresPrivConvStore 验证私聊 conversation store（#686）的合流状态机与 16 轮裁剪，
// 对齐 model 层事务 SQL 语义。需已迁移 026 的 PG（DATABASE_URL / AGENTS_IM_POSTGRES_DSN）。
func TestPostgresPrivConvStore(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("AGENTS_IM_POSTGRES_DSN")
	}
	if dsn == "" {
		t.Skip("DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required for privconv integration tests")
	}

	ctx := context.Background()
	store := privconv.NewModelStore(dsn)
	uniq := time.Now().UnixNano()
	agentID := fmt.Sprintf("2%d", uniq)
	peerID := fmt.Sprintf("1%d", uniq)
	conversationID := fmt.Sprintf("single:%s:%s", peerID, agentID)

	appended := func(seq int64, text string) privconv.AppendUserMessageInput {
		return privconv.AppendUserMessageInput{
			ConversationID: conversationID,
			AgentAccountID: agentID,
			PeerAccountID:  peerID,
			Message:        privconv.PendingMessage{Seq: seq, Text: text, ServerMsgID: fmt.Sprintf("m%d", seq)},
			RunningTTL:     2 * time.Minute,
		}
	}

	// 首条：建行 + running，fired=true（本次驱动一轮）。
	if fired, err := store.AppendUserMessage(ctx, appended(1, "hi")); err != nil || !fired {
		t.Fatalf("first append = (%v, %v), want (true, nil)", fired, err)
	}
	// running 未过租约期间的追击：只累积 pending，fired=false（合流，不新起一轮）。
	if fired, err := store.AppendUserMessage(ctx, appended(2, "more")); err != nil || fired {
		t.Fatalf("append while running = (%v, %v), want (false, nil)", fired, err)
	}
	if fired, err := store.AppendUserMessage(ctx, appended(3, "and more")); err != nil || fired {
		t.Fatalf("append while running = (%v, %v), want (false, nil)", fired, err)
	}
	// 重复/回放 seq<=last_consumed_seq：冪等丢弃。
	if fired, err := store.AppendUserMessage(ctx, appended(3, "dup")); err != nil || fired {
		t.Fatalf("idempotent append = (%v, %v), want (false, nil)", fired, err)
	}

	snap, err := store.Load(ctx, conversationID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if snap.State != "running" || len(snap.Pending) != 3 {
		t.Fatalf("snapshot after appends = state=%q pending=%d, want running/3", snap.State, len(snap.Pending))
	}
	if snap.Pending[0].Text != "hi" || snap.Pending[2].Text != "and more" {
		t.Fatalf("pending order mismatch: %+v", snap.Pending)
	}

	// 收束一轮：消费 seq<=3，追加 round，pending 清空 → idle，morePending=false。
	more, err := store.CommitRound(ctx, privconv.CommitRoundInput{
		ConversationID:  conversationID,
		Round:           privconv.Round{User: []string{"hi", "more", "and more"}, Assistant: "hello there", SeqFrom: 1, SeqTo: 3, AgentRunID: "run-1"},
		ConsumedUpToSeq: 3,
		RunningTTL:      2 * time.Minute,
	})
	if err != nil || more {
		t.Fatalf("commit round = (%v, %v), want (false, nil)", more, err)
	}
	snap, _ = store.Load(ctx, conversationID)
	if snap.State != "idle" || len(snap.Pending) != 0 || len(snap.Rounds) != 1 {
		t.Fatalf("snapshot after commit = state=%q pending=%d rounds=%d, want idle/0/1", snap.State, len(snap.Pending), len(snap.Rounds))
	}
	if snap.Rounds[0].Assistant != "hello there" || len(snap.Rounds[0].User) != 3 {
		t.Fatalf("committed round mismatch: %+v", snap.Rounds[0])
	}

	// idle 后新消息再次 fired=true。
	if fired, err := store.AppendUserMessage(ctx, appended(4, "next")); err != nil || !fired {
		t.Fatalf("append after idle = (%v, %v), want (true, nil)", fired, err)
	}
	// 本轮在途时又来一条（seq 5）：累积 pending，不 fired。
	if fired, err := store.AppendUserMessage(ctx, appended(5, "during run")); err != nil || fired {
		t.Fatalf("append during run = (%v, %v), want (false, nil)", fired, err)
	}
	// 收束仅消费 seq<=4：seq 5 残留 → morePending=true，保持 running（run loop 再跑一轮）。
	more, err = store.CommitRound(ctx, privconv.CommitRoundInput{
		ConversationID:  conversationID,
		Round:           privconv.Round{User: []string{"next"}, Assistant: "ok", SeqFrom: 4, SeqTo: 4},
		ConsumedUpToSeq: 4,
		RunningTTL:      2 * time.Minute,
	})
	if err != nil || !more {
		t.Fatalf("commit with residual pending = (%v, %v), want (true, nil)", more, err)
	}
	snap, _ = store.Load(ctx, conversationID)
	if snap.State != "running" || len(snap.Pending) != 1 || snap.Pending[0].Seq != 5 {
		t.Fatalf("snapshot after residual commit = state=%q pending=%+v, want running/[seq5]", snap.State, snap.Pending)
	}
	if len(snap.Rounds) != 2 {
		t.Fatalf("rounds after second commit = %d, want 2", len(snap.Rounds))
	}

	// CommitRound 安全上限裁剪（#688：阈值改摘要后，DefaultMaxRounds 仅作防膨胀兜底）：
	// 收束到 >DefaultMaxRounds 轮，最旧被丢弃、只留最近 DefaultMaxRounds 轮。
	for i := 6; i <= 60; i++ {
		if _, err := store.AppendUserMessage(ctx, appended(int64(i), fmt.Sprintf("m%d", i))); err != nil {
			// running 时不 fired 也 OK；这里只是喂 pending 再收束。
			t.Fatalf("append seq %d: %v", i, err)
		}
		if _, err := store.CommitRound(ctx, privconv.CommitRoundInput{
			ConversationID:  conversationID,
			Round:           privconv.Round{User: []string{fmt.Sprintf("m%d", i)}, Assistant: fmt.Sprintf("a%d", i), SeqFrom: int64(i), SeqTo: int64(i)},
			ConsumedUpToSeq: int64(i),
			RunningTTL:      2 * time.Minute,
		}); err != nil {
			t.Fatalf("commit seq %d: %v", i, err)
		}
	}
	snap, _ = store.Load(ctx, conversationID)
	if len(snap.Rounds) != privconv.DefaultMaxRounds {
		t.Fatalf("rounds after 60 = %d, want %d (safety-trimmed)", len(snap.Rounds), privconv.DefaultMaxRounds)
	}
	if snap.Rounds[len(snap.Rounds)-1].Assistant != "a60" {
		t.Fatalf("newest kept round = %q, want a60", snap.Rounds[len(snap.Rounds)-1].Assistant)
	}

	// 缺失会话 → NotFound。
	if _, err := store.Load(ctx, conversationID+":missing"); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing load error = %v, want not found", err)
	}
	if _, err := store.CommitRound(ctx, privconv.CommitRoundInput{ConversationID: conversationID + ":missing", ConsumedUpToSeq: 1}); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing commit error = %v, want not found", err)
	}
}

// TestPostgresPrivConvApplySummary 验证 ApplySummary（#688）：写入 long_term_memory/user_profile，
// 移除已摘要轮（seq_to<=cutoff）、保留后续轮，并按字符数截断。需已迁移 027 的 PG。
func TestPostgresPrivConvApplySummary(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("AGENTS_IM_POSTGRES_DSN")
	}
	if dsn == "" {
		t.Skip("DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required for privconv integration tests")
	}
	ctx := context.Background()
	store := privconv.NewModelStore(dsn)
	uniq := time.Now().UnixNano()
	agentID := fmt.Sprintf("2%d", uniq)
	peerID := fmt.Sprintf("1%d", uniq)
	conversationID := fmt.Sprintf("single:%s:%s", peerID, agentID)

	// 攒够 20 轮：逐条 append + commit。
	for i := 1; i <= privconv.SummarizeThreshold; i++ {
		if _, err := store.AppendUserMessage(ctx, privconv.AppendUserMessageInput{
			ConversationID: conversationID, AgentAccountID: agentID, PeerAccountID: peerID,
			Message:    privconv.PendingMessage{Seq: int64(i), Text: fmt.Sprintf("u%d", i), ServerMsgID: fmt.Sprintf("m%d", i)},
			RunningTTL: 2 * time.Minute,
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
		if _, err := store.CommitRound(ctx, privconv.CommitRoundInput{
			ConversationID:  conversationID,
			Round:           privconv.Round{User: []string{fmt.Sprintf("u%d", i)}, Assistant: fmt.Sprintf("a%d", i), SeqFrom: int64(i), SeqTo: int64(i)},
			ConsumedUpToSeq: int64(i), RunningTTL: 2 * time.Minute,
		}); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
	}
	snap, _ := store.Load(ctx, conversationID)
	if len(snap.Rounds) != privconv.SummarizeThreshold {
		t.Fatalf("seeded rounds=%d, want %d", len(snap.Rounds), privconv.SummarizeThreshold)
	}

	// 摘要前 18 轮（cutoff=第18轮 seq_to=18），保留后 2 轮 + 写记忆/画像；记忆超长按字符截断。
	longMem := ""
	for i := 0; i < privconv.MaxMemoryChars+500; i++ {
		longMem += "记"
	}
	cutoff := snap.Rounds[len(snap.Rounds)-privconv.KeepRoundsAfterSummary-1].SeqTo
	if err := store.ApplySummary(ctx, privconv.ApplySummaryInput{
		ConversationID: conversationID, LongTermMemory: longMem, UserProfile: "用户是工程师", CutoffSeq: cutoff,
	}); err != nil {
		t.Fatalf("apply summary: %v", err)
	}
	snap, _ = store.Load(ctx, conversationID)
	if len(snap.Rounds) != privconv.KeepRoundsAfterSummary {
		t.Fatalf("rounds after summary=%d, want %d", len(snap.Rounds), privconv.KeepRoundsAfterSummary)
	}
	if snap.Rounds[len(snap.Rounds)-1].Assistant != fmt.Sprintf("a%d", privconv.SummarizeThreshold) {
		t.Fatalf("newest kept round=%q", snap.Rounds[len(snap.Rounds)-1].Assistant)
	}
	if got := len([]rune(snap.LongTermMemory)); got != privconv.MaxMemoryChars {
		t.Fatalf("memory not truncated to %d chars, got %d", privconv.MaxMemoryChars, got)
	}
	if snap.UserProfile != "用户是工程师" {
		t.Fatalf("user_profile mismatch: %q", snap.UserProfile)
	}
}
