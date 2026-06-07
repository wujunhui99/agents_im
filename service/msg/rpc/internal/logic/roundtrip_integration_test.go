//go:build integration

package logic_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/logic"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"
	"github.com/zeromicro/go-zero/core/stores/postgres"
)

// 单聊文本走全链：发送（seq 分配 + outbox）→ 幂等去重 → 拉历史 → seq 状态 → 已读。
// 单聊文本不触达跨域 Groups/Media，故 svcCtx 这两个依赖留 nil。
func TestSingleTextRoundtrip(t *testing.T) {
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
		Outbox:   model.NewMessageOutboxModel(conn),
	}

	ctx := context.Background()
	uniq := time.Now().UnixNano()
	sender := fmt.Sprintf("msgit-sender-%d", uniq)
	receiver := fmt.Sprintf("msgit-receiver-%d", uniq)
	clientMsgID := fmt.Sprintf("msgit-cmid-%d", uniq)
	convID := model.SingleConversationID(sender, receiver)

	send := func() (*msg.SendMessageResponse, error) {
		return logic.NewSendMessageLogic(ctx, svcCtx).SendMessage(&msg.SendMessageRequest{
			SenderId:    sender,
			ReceiverId:  receiver,
			ChatType:    "single",
			ClientMsgId: clientMsgID,
			ContentType: "text",
			Content:     "hello msg-rpc",
		})
	}

	// 1) 首发：seq=1、server_msg_id 生成、未去重
	resp, err := send()
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if resp.Deduplicated {
		t.Fatalf("first send should not be deduplicated")
	}
	if resp.Message.Seq != 1 {
		t.Fatalf("expected seq 1, got %d", resp.Message.Seq)
	}
	if resp.Message.ServerMsgId == "" || resp.Message.SendTime == 0 {
		t.Fatalf("expected server_msg_id + send_time, got %+v", resp.Message)
	}
	if resp.Message.Content != "hello msg-rpc" {
		t.Fatalf("content mismatch: %q", resp.Message.Content)
	}
	serverMsgID := resp.Message.ServerMsgId

	// outbox 事件已写入（喂 message-transfer）
	var outboxCount int
	if err := conn.QueryRowCtx(ctx, &outboxCount,
		"select count(*) from message_outbox where message_id = $1 and event_type = 1", serverMsgID); err != nil {
		t.Fatalf("query outbox: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("expected 1 outbox event, got %d", outboxCount)
	}

	// 2) 同 client_msg_id 重发：幂等去重，server_msg_id 不变
	resp2, err := send()
	if err != nil {
		t.Fatalf("idempotent resend: %v", err)
	}
	if !resp2.Deduplicated || resp2.Message.ServerMsgId != serverMsgID {
		t.Fatalf("expected dedup to same server_msg_id, got dedup=%v id=%s", resp2.Deduplicated, resp2.Message.ServerMsgId)
	}

	// 3) 发送方拉历史可见该消息
	pull, err := logic.NewPullMessagesLogic(ctx, svcCtx).PullMessages(&msg.PullMessagesRequest{
		UserId:         sender,
		ConversationId: convID,
	})
	if err != nil {
		t.Fatalf("PullMessages: %v", err)
	}
	if len(pull.Messages) != 1 || pull.Messages[0].ServerMsgId != serverMsgID {
		t.Fatalf("expected 1 pulled message %s, got %+v", serverMsgID, pull.Messages)
	}

	// 4) 接收方 seq 状态：max=1、未读=1
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

	// 5) 接收方已读到 seq=1 → 未读归零
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
