package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"
)

// fakeMessagesModel 嵌入 MessagesModel 接口（nil），只覆写 GetMessageRef 用到的 FindOne；
// 其余方法存在但调用即 panic（断言 GetMessageRef 不应触达它们）。
type fakeMessagesModel struct {
	model.MessagesModel
	findOne func(ctx context.Context, id int64) (*model.Messages, error)
}

func (f fakeMessagesModel) FindOne(ctx context.Context, id int64) (*model.Messages, error) {
	return f.findOne(ctx, id)
}

func newGetMessageRefLogic(findOne func(ctx context.Context, id int64) (*model.Messages, error)) *GetMessageRefLogic {
	svcCtx := &svc.ServiceContext{Messages: fakeMessagesModel{findOne: findOne}}
	return NewGetMessageRefLogic(context.Background(), svcCtx)
}

func TestGetMessageRefSingleWithAttachment(t *testing.T) {
	row := &model.Messages{
		MessageId:         123,
		ConversationType:  model.ConversationTypeSingle,
		SenderAccountId:   "usr_alice",
		ReceiverAccountId: "usr_bob",
		ContentType:       model.ContentTypeImageValue,
		Content:           `{"mediaId":"998877","width":100,"height":200}`,
	}
	logic := newGetMessageRefLogic(func(_ context.Context, id int64) (*model.Messages, error) {
		if id != 123 {
			t.Fatalf("FindOne got id %d, want 123", id)
		}
		return row, nil
	})

	// requester = sender → peer = receiver
	resp, err := logic.GetMessageRef(&msg.GetMessageRefRequest{ServerMsgId: "123", RequesterAccountId: "usr_alice"})
	if err != nil {
		t.Fatalf("GetMessageRef: %v", err)
	}
	if resp.GetChatType() != model.ChatTypeSingle {
		t.Fatalf("chat_type = %q, want single", resp.GetChatType())
	}
	if resp.GetGroupId() != "" {
		t.Fatalf("single chat must not set group_id, got %q", resp.GetGroupId())
	}
	if resp.GetPeerAccountId() != "usr_bob" {
		t.Fatalf("peer (requester=sender) = %q, want usr_bob", resp.GetPeerAccountId())
	}
	if resp.GetMediaId() != "998877" {
		t.Fatalf("media_id = %q, want 998877", resp.GetMediaId())
	}

	// requester = receiver → peer = sender
	resp, err = logic.GetMessageRef(&msg.GetMessageRefRequest{ServerMsgId: "123", RequesterAccountId: "usr_bob"})
	if err != nil {
		t.Fatalf("GetMessageRef: %v", err)
	}
	if resp.GetPeerAccountId() != "usr_alice" {
		t.Fatalf("peer (requester=receiver) = %q, want usr_alice", resp.GetPeerAccountId())
	}
}

func TestGetMessageRefGroupWithoutAttachment(t *testing.T) {
	row := &model.Messages{
		MessageId:        456,
		ConversationType: model.ConversationTypeGroup,
		SenderAccountId:  "usr_alice",
		GroupId:          "grp_42",
		ContentType:      model.ContentTypeTextValue,
		Content:          `{"text":"hello"}`,
	}
	logic := newGetMessageRefLogic(func(_ context.Context, _ int64) (*model.Messages, error) {
		return row, nil
	})

	resp, err := logic.GetMessageRef(&msg.GetMessageRefRequest{ServerMsgId: "456"})
	if err != nil {
		t.Fatalf("GetMessageRef: %v", err)
	}
	if resp.GetChatType() != model.ChatTypeGroup {
		t.Fatalf("chat_type = %q, want group", resp.GetChatType())
	}
	if resp.GetGroupId() != "grp_42" {
		t.Fatalf("group_id = %q, want grp_42", resp.GetGroupId())
	}
	if resp.GetPeerAccountId() != "" {
		t.Fatalf("group chat must not set peer_account_id, got %q", resp.GetPeerAccountId())
	}
	if resp.GetMediaId() != "" {
		t.Fatalf("text message must have empty media_id, got %q", resp.GetMediaId())
	}
}

func TestGetMessageRefGroupWithAttachment(t *testing.T) {
	row := &model.Messages{
		MessageId:        789,
		ConversationType: model.ConversationTypeGroup,
		GroupId:          "grp_7",
		ContentType:      model.ContentTypeFileValue,
		Content:          `{"mediaId":"555","filename":"a.pdf","sizeBytes":10,"contentType":"application/pdf"}`,
	}
	logic := newGetMessageRefLogic(func(_ context.Context, _ int64) (*model.Messages, error) {
		return row, nil
	})

	resp, err := logic.GetMessageRef(&msg.GetMessageRefRequest{ServerMsgId: "789"})
	if err != nil {
		t.Fatalf("GetMessageRef: %v", err)
	}
	if resp.GetMediaId() != "555" {
		t.Fatalf("media_id = %q, want 555", resp.GetMediaId())
	}
	if resp.GetGroupId() != "grp_7" {
		t.Fatalf("group_id = %q, want grp_7", resp.GetGroupId())
	}
}

func TestGetMessageRefNotFound(t *testing.T) {
	logic := newGetMessageRefLogic(func(_ context.Context, _ int64) (*model.Messages, error) {
		return nil, model.ErrNotFound
	})
	if _, err := logic.GetMessageRef(&msg.GetMessageRefRequest{ServerMsgId: "404"}); err == nil {
		t.Fatal("expected error for nonexistent message")
	}
}

func TestGetMessageRefInvalidID(t *testing.T) {
	logic := newGetMessageRefLogic(func(_ context.Context, _ int64) (*model.Messages, error) {
		t.Fatal("FindOne must not be called for an invalid id")
		return nil, nil
	})
	for _, bad := range []string{"", "  ", "not-a-number", "0", "-5"} {
		if _, err := logic.GetMessageRef(&msg.GetMessageRefRequest{ServerMsgId: bad}); err == nil {
			t.Fatalf("expected error for invalid server_msg_id %q", bad)
		}
	}
}
