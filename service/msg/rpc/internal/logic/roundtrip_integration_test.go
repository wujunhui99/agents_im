//go:build integration

package logic_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/logic"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 读路径 roundtrip：写路径已 Kafka 化（03 §9 B2/B3b，落库归 msgtransfer，
// 链路覆盖见 service/msgtransfer/internal/chain 集成测试），这里按 msgtransfer
// persist 的落库形状直接用 goctl model 播种，再验证拉历史 → seq 状态 → 已读。
func TestSingleTextReadPathRoundtrip(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("AGENTS_IM_POSTGRES_DSN")
	}
	if dsn == "" {
		t.Skip("DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required for msg-rpc integration tests")
	}

	conn := postgres.New(dsn)
	svcCtx := &svc.ServiceContext{
		Messages: model.NewMessagesModel(conn),
		Threads:  model.NewConversationThreadsModel(conn),
		States:   model.NewUserConversationStatesModel(conn),
	}

	ctx := context.Background()
	uniq := time.Now().UnixNano()
	sender := fmt.Sprintf("msgit-sender-%d", uniq)
	receiver := fmt.Sprintf("msgit-receiver-%d", uniq)
	convID := model.SingleConversationID(sender, receiver)
	// message_id 自 #531 起为雪花 bigint；这里用 uniq(纳秒) 作合法 bigint，wire 仍十进制串。
	serverMsgIDInt := uniq
	serverMsgID := strconv.FormatInt(serverMsgIDInt, 10)

	// 播种一条 seq=1 的单聊消息（事务形状对齐 msgtransfer persist consumer）。
	err := svcCtx.Messages.Transact(ctx, func(ctx context.Context, session sqlx.Session) error {
		msgs := svcCtx.Messages.WithSession(session)
		threads := svcCtx.Threads.WithSession(session)
		states := svcCtx.States.WithSession(session)

		maxSeq, err := threads.UpsertAndLock(ctx, convID, model.UpsertConversationParams{
			ChatType:    model.ChatTypeSingle,
			SingleUserA: sender,
			SingleUserB: receiver,
		})
		if err != nil {
			return err
		}
		nextSeq := maxSeq + 1
		sendTime := time.Now().UTC()
		contentJSON, err := model.EncodeMessageContent("text", "hello msg-rpc read path")
		if err != nil {
			return err
		}
		inserted, err := msgs.InsertReturning(ctx, &model.Messages{
			MessageId:         serverMsgIDInt,
			ClientMsgId:       fmt.Sprintf("msgit-cmid-%d", uniq),
			SenderAccountId:   sender,
			ConversationId:    convID,
			Seq:               nextSeq,
			ConversationType:  1,
			ReceiverAccountId: receiver,
			ContentType:       1,
			Content:           contentJSON,
			MessageOrigin:     1,
			PayloadHash:       fmt.Sprintf("msgit-hash-%d", uniq),
			ClientSendTime:    sql.NullTime{Time: sendTime, Valid: true},
		})
		if err != nil {
			return err
		}
		if err := states.UpsertVisible(ctx, []string{sender, receiver}, convID, 0); err != nil {
			return err
		}
		if err := states.UpsertSenderRead(ctx, sender, convID, nextSeq); err != nil {
			return err
		}
		return threads.UpdateAfterMessage(ctx, convID, strconv.FormatInt(inserted.MessageId, 10), nextSeq, sendTime)
	})
	if err != nil {
		t.Fatalf("seed message: %v", err)
	}

	// 1) 接收方拉历史可见该消息
	pull, err := logic.NewPullMessagesLogic(ctx, svcCtx).PullMessages(&msg.PullMessagesRequest{
		UserId:         receiver,
		ConversationId: convID,
	})
	if err != nil {
		t.Fatalf("PullMessages: %v", err)
	}
	if len(pull.Messages) != 1 || pull.Messages[0].ServerMsgId != serverMsgID || pull.Messages[0].Seq != 1 {
		t.Fatalf("expected 1 pulled message %s seq=1, got %+v", serverMsgID, pull.Messages)
	}

	// 2) 接收方 seq 状态：max=1、未读=1
	seqState := func(user string) *msg.ConversationSeqState {
		out, err := logic.NewGetConversationsSeqStateLogic(ctx, svcCtx).GetConversationsSeqState(&msg.GetConversationsSeqStateRequest{
			UserId:          user,
			ConversationIds: []string{convID},
		})
		if err != nil {
			t.Fatalf("GetConversationsSeqState(%s): %v", user, err)
		}
		if len(out.States) != 1 {
			t.Fatalf("expected 1 state for %s, got %d", user, len(out.States))
		}
		return out.States[0]
	}
	st := seqState(receiver)
	if st.MaxSeq != 1 || st.UnreadCount != 1 {
		t.Fatalf("receiver before read: expected max=1 unread=1, got max=%d unread=%d", st.MaxSeq, st.UnreadCount)
	}

	// 3) 接收方已读到 seq=1 → 未读归零
	mark, err := logic.NewMarkConversationAsReadLogic(ctx, svcCtx).MarkConversationAsRead(&msg.MarkConversationAsReadRequest{
		UserId:         receiver,
		ConversationId: convID,
		HasReadSeq:     1,
	})
	if err != nil {
		t.Fatalf("MarkConversationAsRead: %v", err)
	}
	if !mark.Updated || mark.UnreadCount != 0 {
		t.Fatalf("expected updated=true unread=0, got updated=%v unread=%d", mark.Updated, mark.UnreadCount)
	}
	if st := seqState(receiver); st.UnreadCount != 0 || st.HasReadSeq != 1 {
		t.Fatalf("receiver after read: expected unread=0 has_read=1, got unread=%d has_read=%d", st.UnreadCount, st.HasReadSeq)
	}
}
