package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
)

// 附件校验规则（purpose/ready/owned/类型/大小/元数据匹配）已迁到 media-rpc
// （service/media/rpc/internal/logic/validate_message_media_logic_test.go，#533）。
// 这里只验证 MessageLogic 与注入的 MessageMediaValidator 的集成契约：
// image/file 消息调用校验器并透传其错误，text 消息不调用。

type recordingMediaValidator struct {
	calls []recordedMediaCall
	err   error
}

type recordedMediaCall struct {
	ownerUserID string
	contentType string
	content     string
}

func (v *recordingMediaValidator) ValidateMessageMedia(_ context.Context, ownerUserID, contentType, content string) error {
	v.calls = append(v.calls, recordedMediaCall{ownerUserID: ownerUserID, contentType: contentType, content: content})
	return v.err
}

func TestMessageLogicInvokesMediaValidatorForAttachments(t *testing.T) {
	ctx := context.Background()
	validator := &recordingMediaValidator{}
	messageLogic := NewMessageLogicWithMediaValidator(repository.NewMemoryMessageRepository(), nil, nil, validator)

	imageContent := `{"mediaId":"60927948672204800","width":100,"height":100}`
	if _, err := messageLogic.SendMessage(ctx, SendMessageRequest{
		SenderID:    "usr_sender",
		ReceiverID:  "usr_receiver",
		ChatType:    MessageChatTypeSingle,
		ClientMsgID: "client-image",
		ContentType: MessageContentTypeImage,
		Content:     imageContent,
	}); err != nil {
		t.Fatalf("send image message: %v", err)
	}

	if len(validator.calls) != 1 {
		t.Fatalf("expected media validator called once for image, got %d", len(validator.calls))
	}
	if got := validator.calls[0]; got.ownerUserID != "usr_sender" || got.contentType != MessageContentTypeImage || got.content != imageContent {
		t.Fatalf("validator called with %+v, want sender/image/content forwarded verbatim", got)
	}
}

func TestMessageLogicPropagatesMediaValidatorError(t *testing.T) {
	validator := &recordingMediaValidator{err: apperror.InvalidArgument("image media is not ready")}
	messageLogic := NewMessageLogicWithMediaValidator(repository.NewMemoryMessageRepository(), nil, nil, validator)

	_, err := messageLogic.SendMessage(context.Background(), SendMessageRequest{
		SenderID:    "usr_sender",
		ReceiverID:  "usr_receiver",
		ChatType:    MessageChatTypeSingle,
		ClientMsgID: "client-bad-image",
		ContentType: MessageContentTypeImage,
		Content:     `{"mediaId":"60927948672204800"}`,
	})
	assertLogicAppCode(t, err, apperror.CodeInvalidArgument)
}

func TestMessageLogicSkipsMediaValidatorForText(t *testing.T) {
	validator := &recordingMediaValidator{err: apperror.InvalidArgument("should not be called")}
	messageLogic := NewMessageLogicWithMediaValidator(repository.NewMemoryMessageRepository(), nil, nil, validator)

	if _, err := messageLogic.SendMessage(context.Background(), logicTestSendRequest("usr_text_a", "usr_text_b", "client-text", "hello")); err != nil {
		t.Fatalf("text message should not be validated: %v", err)
	}
	if len(validator.calls) != 0 {
		t.Fatalf("expected media validator not called for text, got %d calls", len(validator.calls))
	}
}

func TestTextMessagesContinueWithoutMediaValidator(t *testing.T) {
	messageLogic := NewMessageLogic(repository.NewMemoryMessageRepository())
	if _, err := messageLogic.SendMessage(context.Background(), logicTestSendRequest("usr_text_a", "usr_text_b", "client-text-no-media", "hello")); err != nil {
		t.Fatalf("text message without media validator: %v", err)
	}

	_, err := messageLogic.SendMessage(context.Background(), SendMessageRequest{
		SenderID:    "usr_text_a",
		ReceiverID:  "usr_text_b",
		ChatType:    MessageChatTypeSingle,
		ClientMsgID: "client-image-no-media",
		ContentType: MessageContentTypeImage,
		Content:     `{"mediaId":"med_missing"}`,
	})
	assertLogicAppCode(t, err, apperror.CodeInternal)
}
