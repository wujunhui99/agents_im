package logic

import (
	"context"
	"fmt"
	"testing"

	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/internal/mediavalidate"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
)

func TestMessageMediaValidation(t *testing.T) {
	ctx := context.Background()
	messageRepo := repository.NewMemoryMessageRepository()
	mediaRepo := repository.NewMemoryMediaRepository()
	mediaLogic := mediavalidate.NewMessageValidator(mediaRepo)
	messageLogic := NewMessageLogicWithMediaValidator(messageRepo, nil, nil, mediaLogic)

	image := createMediaForTest(t, mediaRepo, model.MediaObject{
		MediaID:     "med_image_ready",
		OwnerUserID: "usr_sender",
		Bucket:      "agents-im-media",
		ObjectKey:   "users/usr_sender/media/med_image_ready/cat.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   1024,
		Purpose:     model.MediaPurposeMessageImage,
		Status:      model.MediaStatusReady,
	})
	file := createMediaForTest(t, mediaRepo, model.MediaObject{
		MediaID:          "med_file_ready",
		OwnerUserID:      "usr_sender",
		Bucket:           "agents-im-media",
		ObjectKey:        "users/usr_sender/media/med_file_ready/report.pdf",
		ContentType:      "application/pdf",
		SizeBytes:        2048,
		OriginalFilename: "report.pdf",
		Purpose:          model.MediaPurposeMessageFile,
		Status:           model.MediaStatusReady,
	})
	pending := createMediaForTest(t, mediaRepo, model.MediaObject{
		MediaID:     "med_image_pending",
		OwnerUserID: "usr_sender",
		Bucket:      "agents-im-media",
		ObjectKey:   "users/usr_sender/media/med_image_pending/cat.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   1024,
		Purpose:     model.MediaPurposeMessageImage,
		Status:      model.MediaStatusPending,
	})
	notOwned := createMediaForTest(t, mediaRepo, model.MediaObject{
		MediaID:     "med_image_other",
		OwnerUserID: "usr_other",
		Bucket:      "agents-im-media",
		ObjectKey:   "users/usr_other/media/med_image_other/cat.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   1024,
		Purpose:     model.MediaPurposeMessageImage,
		Status:      model.MediaStatusReady,
	})

	if _, err := messageLogic.SendMessage(ctx, SendMessageRequest{
		SenderID:    "usr_sender",
		ReceiverID:  "usr_receiver",
		ChatType:    MessageChatTypeSingle,
		ClientMsgID: "client-image-ready",
		ContentType: MessageContentTypeImage,
		Content:     fmt.Sprintf(`{"mediaId":%q,"width":100,"height":100}`, image.MediaID),
	}); err != nil {
		t.Fatalf("send ready image message: %v", err)
	}

	if _, err := messageLogic.SendMessage(ctx, SendMessageRequest{
		SenderID:    "usr_sender",
		ReceiverID:  "usr_receiver",
		ChatType:    MessageChatTypeSingle,
		ClientMsgID: "client-file-ready",
		ContentType: MessageContentTypeFile,
		Content:     fmt.Sprintf(`{"mediaId":%q,"filename":"report.pdf","sizeBytes":2048,"contentType":"application/pdf"}`, file.MediaID),
	}); err != nil {
		t.Fatalf("send ready file message: %v", err)
	}

	cases := []struct {
		name        string
		contentType string
		content     string
		want        apperror.Code
	}{
		{name: "missing media id", contentType: MessageContentTypeImage, content: `{}`, want: apperror.CodeInvalidArgument},
		{name: "not ready", contentType: MessageContentTypeImage, content: fmt.Sprintf(`{"mediaId":%q}`, pending.MediaID), want: apperror.CodeInvalidArgument},
		{name: "not owned", contentType: MessageContentTypeImage, content: fmt.Sprintf(`{"mediaId":%q}`, notOwned.MediaID), want: apperror.CodeForbidden},
		{name: "file metadata mismatch", contentType: MessageContentTypeFile, content: fmt.Sprintf(`{"mediaId":%q,"filename":"report.pdf","sizeBytes":1,"contentType":"application/pdf"}`, file.MediaID), want: apperror.CodeInvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := messageLogic.SendMessage(ctx, SendMessageRequest{
				SenderID:    "usr_sender",
				ReceiverID:  "usr_receiver",
				ChatType:    MessageChatTypeSingle,
				ClientMsgID: "client-" + tc.name,
				ContentType: tc.contentType,
				Content:     tc.content,
			})
			assertLogicAppCode(t, err, tc.want)
		})
	}
}

func TestMessageMediaValidationEnforcesMessageSizeLimits(t *testing.T) {
	cases := []struct {
		name        string
		media       model.MediaObject
		contentType string
		content     func(string) string
		wantErr     bool
	}{
		{
			name: "image accepts fifteen mib",
			media: model.MediaObject{
				MediaID:     "med_image_15_mib",
				OwnerUserID: "usr_sender",
				Bucket:      "agents-im-media",
				ObjectKey:   "users/usr_sender/media/med_image_15_mib/image.jpg",
				ContentType: "image/jpeg",
				SizeBytes:   15 * 1024 * 1024,
				Purpose:     model.MediaPurposeMessageImage,
				Status:      model.MediaStatusReady,
			},
			contentType: MessageContentTypeImage,
			content:     func(mediaID string) string { return fmt.Sprintf(`{"mediaId":%q,"width":100,"height":100}`, mediaID) },
		},
		{
			name: "image rejects over fifteen mib",
			media: model.MediaObject{
				MediaID:     "med_image_over_15_mib",
				OwnerUserID: "usr_sender",
				Bucket:      "agents-im-media",
				ObjectKey:   "users/usr_sender/media/med_image_over_15_mib/image.jpg",
				ContentType: "image/jpeg",
				SizeBytes:   15*1024*1024 + 1,
				Purpose:     model.MediaPurposeMessageImage,
				Status:      model.MediaStatusReady,
			},
			contentType: MessageContentTypeImage,
			content:     func(mediaID string) string { return fmt.Sprintf(`{"mediaId":%q,"width":100,"height":100}`, mediaID) },
			wantErr:     true,
		},
		{
			name: "file accepts twenty mib",
			media: model.MediaObject{
				MediaID:          "med_file_20_mib",
				OwnerUserID:      "usr_sender",
				Bucket:           "agents-im-media",
				ObjectKey:        "users/usr_sender/media/med_file_20_mib/report.pdf",
				ContentType:      "application/pdf",
				SizeBytes:        20 * 1024 * 1024,
				OriginalFilename: "report.pdf",
				Purpose:          model.MediaPurposeMessageFile,
				Status:           model.MediaStatusReady,
			},
			contentType: MessageContentTypeFile,
			content: func(mediaID string) string {
				return fmt.Sprintf(`{"mediaId":%q,"filename":"report.pdf","sizeBytes":20971520,"contentType":"application/pdf"}`, mediaID)
			},
		},
		{
			name: "file rejects over twenty mib",
			media: model.MediaObject{
				MediaID:          "med_file_over_20_mib",
				OwnerUserID:      "usr_sender",
				Bucket:           "agents-im-media",
				ObjectKey:        "users/usr_sender/media/med_file_over_20_mib/report.pdf",
				ContentType:      "application/pdf",
				SizeBytes:        20*1024*1024 + 1,
				OriginalFilename: "report.pdf",
				Purpose:          model.MediaPurposeMessageFile,
				Status:           model.MediaStatusReady,
			},
			contentType: MessageContentTypeFile,
			content: func(mediaID string) string {
				return fmt.Sprintf(`{"mediaId":%q,"filename":"report.pdf","sizeBytes":20971521,"contentType":"application/pdf"}`, mediaID)
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			messageRepo := repository.NewMemoryMessageRepository()
			mediaRepo := repository.NewMemoryMediaRepository()
			mediaLogic := mediavalidate.NewMessageValidator(mediaRepo)
			messageLogic := NewMessageLogicWithMediaValidator(messageRepo, nil, nil, mediaLogic)
			media := createMediaForTest(t, mediaRepo, tc.media)

			_, err := messageLogic.SendMessage(ctx, SendMessageRequest{
				SenderID:    "usr_sender",
				ReceiverID:  "usr_receiver",
				ChatType:    MessageChatTypeSingle,
				ClientMsgID: "client-" + tc.name,
				ContentType: tc.contentType,
				Content:     tc.content(media.MediaID),
			})
			if tc.wantErr {
				assertLogicAppCode(t, err, apperror.CodeInvalidArgument)
				return
			}
			if err != nil {
				t.Fatalf("send message: %v", err)
			}
		})
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

func createMediaForTest(t *testing.T, repo repository.MediaRepository, media model.MediaObject) model.MediaObject {
	t.Helper()
	created, err := repo.CreateMediaObject(context.Background(), media)
	if err != nil {
		t.Fatalf("create media fixture: %v", err)
	}
	return created
}
