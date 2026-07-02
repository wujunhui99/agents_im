package orchestrator

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

// waitForConversationCount 轮询等待某会话累计到 want 条消息（异步 AI 写回落库后）。
func waitForConversationCount(t *testing.T, im *fakeIM, conversationID string, want int) []Message {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		msgs := im.messages(conversationID)
		if len(msgs) == want {
			return msgs
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d messages in %s, have %d", want, conversationID, len(msgs))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// 本文件是 AI 托管 runtime 单测的进程内 fake，取代原先直接构造 internal MessageLogic +
// 内存 repository 的做法（#617：runtime message/groups 读写改走 owner gRPC，生产由
// msgrpc/groupsrpc 适配器承接，单测用下列 fake 替身）。

// singleConvID / groupConvID 复刻 message 域会话 id 约定（single:<lower>:<higher> / group:<id>）。
func singleConvID(a, b string) string {
	if a <= b {
		return "single:" + a + ":" + b
	}
	return "single:" + b + ":" + a
}

func groupConvID(groupID string) string { return "group:" + groupID }

// fakeIM 是消息服务在三个 runtime 读写接缝上的进程内替身：捕获 AI 写回（MessageSender）、
// 回放会话历史（MessageHistoryReader）、记录已读推进（ConversationReadAdvancer）。
type fakeIM struct {
	mu      sync.Mutex
	byConv  map[string][]Message
	seq     int64
	sendErr error
	reads   map[string]int64 // accountID|conversationID -> hasReadSeq
}

func newFakeIM() *fakeIM {
	return &fakeIM{byConv: map[string][]Message{}, reads: map[string]int64{}}
}

// appendHuman 播种一条人类消息（测试触发源），返回带 seq/server_msg_id 的消息。
func (f *fakeIM) appendHuman(msg Message) Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.appendLocked(msg)
}

func (f *fakeIM) appendLocked(msg Message) Message {
	f.seq++
	msg.Seq = f.seq
	if strings.TrimSpace(msg.ServerMsgID) == "" {
		msg.ServerMsgID = "srv_" + msg.ClientMsgID
	}
	f.byConv[msg.ConversationID] = append(f.byConv[msg.ConversationID], msg)
	return msg
}

// SendMessage 实现 orchestrator.MessageSender：把 AI 回复落进会话，回显请求字段（供响应校验）。
func (f *fakeIM) SendMessage(_ context.Context, req SendMessageRequest) (SendMessageResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sendErr != nil {
		return SendMessageResponse{}, f.sendErr
	}
	convID := singleConvID(req.SenderID, req.ReceiverID)
	if req.ChatType == MessageChatTypeGroup {
		convID = groupConvID(req.GroupID)
	}
	msg := f.appendLocked(Message{
		ServerMsgID:           "srv_" + req.ClientMsgID,
		ClientMsgID:           req.ClientMsgID,
		ConversationID:        convID,
		SenderID:              req.SenderID,
		ReceiverID:            req.ReceiverID,
		GroupID:               req.GroupID,
		ChatType:              req.ChatType,
		ContentType:           req.ContentType,
		Content:               req.Content,
		MessageOrigin:         req.MessageOrigin,
		AgentAccountID:        req.AgentAccountID,
		TriggerServerMsgID:    req.TriggerServerMsgID,
		AgentRunID:            req.AgentRunID,
		AllowRecursiveTrigger: req.AllowRecursiveTrigger,
	})
	return SendMessageResponse{Message: msg}, nil
}

// GetRecentMessages 实现 orchestrator.MessageHistoryReader：按 [from,to] 裁剪 + limit 取末 N 条（asc）。
func (f *fakeIM) GetRecentMessages(_ context.Context, req RecentMessagesRequest) ([]Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	all := f.byConv[req.ConversationID]
	out := make([]Message, 0, len(all))
	for _, m := range all {
		if req.FromSeq > 0 && m.Seq < req.FromSeq {
			continue
		}
		if req.ToSeq > 0 && m.Seq > req.ToSeq {
			continue
		}
		out = append(out, m)
	}
	if req.Limit > 0 && len(out) > req.Limit {
		out = out[len(out)-req.Limit:]
	}
	return out, nil
}

// MarkConversationRead 实现 orchestrator.ConversationReadAdvancer：单调推进已读 seq。
func (f *fakeIM) MarkConversationRead(_ context.Context, accountID, conversationID string, seq int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := accountID + "|" + conversationID
	if seq > f.reads[key] {
		f.reads[key] = seq
	}
	return nil
}

func (f *fakeIM) messages(conversationID string) []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Message(nil), f.byConv[conversationID]...)
}

func (f *fakeIM) readSeq(accountID, conversationID string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads[accountID+"|"+conversationID]
}

// stubGroupMembers 实现 orchestrator.GroupMemberLister（脱 internal GroupsLogic）：非成员 requester 拒绝。
type stubGroupMembers struct {
	groupID string
	members []GroupMemberInfo
}

func newStubGroupMembers(groupID string, activeMemberIDs []string) *stubGroupMembers {
	members := make([]GroupMemberInfo, 0, len(activeMemberIDs))
	for _, userID := range activeMemberIDs {
		members = append(members, GroupMemberInfo{
			GroupID: groupID,
			UserID:  userID,
			State:   "active",
			Role:    "member",
		})
	}
	return &stubGroupMembers{groupID: groupID, members: members}
}

func (l *stubGroupMembers) ListMembers(_ context.Context, req ListMembersRequest) (ListMembersResponse, error) {
	if strings.TrimSpace(req.GroupID) != l.groupID {
		return ListMembersResponse{}, nil
	}
	members := append([]GroupMemberInfo(nil), l.members...)
	if strings.TrimSpace(req.RequesterUserID) != "" {
		active := false
		for _, member := range members {
			if member.UserID == req.RequesterUserID && (member.State == "" || member.State == "active") {
				active = true
				break
			}
		}
		if !active {
			return ListMembersResponse{}, apperror.Forbidden("requester is not a group member")
		}
	}
	return ListMembersResponse{GroupID: req.GroupID, Members: members}, nil
}
