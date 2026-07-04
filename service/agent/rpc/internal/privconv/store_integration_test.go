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

	// 16 轮滚动裁剪：继续收束到 >16 轮，最旧被丢弃。
	for i := 6; i <= 40; i++ {
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
		t.Fatalf("rounds after 40 = %d, want %d (trimmed)", len(snap.Rounds), privconv.DefaultMaxRounds)
	}
	if snap.Rounds[len(snap.Rounds)-1].Assistant != "a40" {
		t.Fatalf("newest kept round = %q, want a40", snap.Rounds[len(snap.Rounds)-1].Assistant)
	}

	// 缺失会话 → NotFound。
	if _, err := store.Load(ctx, conversationID+":missing"); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing load error = %v, want not found", err)
	}
	if _, err := store.CommitRound(ctx, privconv.CommitRoundInput{ConversationID: conversationID + ":missing", ConsumedUpToSeq: 1}); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing commit error = %v, want not found", err)
	}
}
