package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/internal/mediavalidate"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/objectstorage"
	"github.com/wujunhui99/agents_im/service/media/core"
)

// These integration tests exercise media-download access control (owned by
// service/media/core) together with message participation, which is seeded via
// MessageLogic. They live here (package logic) so they can reach both the
// message logic and the media core.

func TestMessageParticipantCanGetMediaDownloadURL(t *testing.T) {
	ctx := context.Background()
	store := objectstorage.NewMemoryStore()
	mediaRepo := repository.NewMemoryMediaRepository()
	messageRepo := repository.NewMemoryMessageRepository()
	mediaLogic := core.NewMediaLogic(mediaRepo, store, "agents-im-media").
		WithAttachmentAccessChecker(core.NewMessageMediaAccessChecker(messageRepo))
	messageLogic := NewMessageLogicWithMediaValidator(messageRepo, nil, nil, mediavalidate.NewMessageValidator(mediaRepo))

	image := createMediaForTest(t, mediaRepo, model.MediaObject{
		MediaID:          "med_chat_image_ready",
		OwnerUserID:      "usr_sender",
		Bucket:           "agents-im-media",
		ObjectKey:        "users/usr_sender/media/med_chat_image_ready/cat.jpg",
		ContentType:      "image/jpeg",
		SizeBytes:        1024,
		OriginalFilename: "cat.jpg",
		Purpose:          model.MediaPurposeMessageImage,
		Status:           model.MediaStatusReady,
	})

	if _, err := messageLogic.SendMessage(ctx, SendMessageRequest{
		SenderID:    "usr_sender",
		ReceiverID:  "usr_receiver",
		ChatType:    MessageChatTypeSingle,
		ClientMsgID: "client-image-access",
		ContentType: MessageContentTypeImage,
		Content:     `{"mediaId":"med_chat_image_ready","filename":"cat.jpg"}`,
	}); err != nil {
		t.Fatalf("send image message: %v", err)
	}

	ownerDownload, err := mediaLogic.GetDownloadURL(ctx, core.GetMediaDownloadURLRequest{
		OwnerUserID: "usr_sender",
		MediaID:     image.MediaID,
	})
	if err != nil {
		t.Fatalf("owner get download URL: %v", err)
	}
	if ownerDownload.DownloadURL == "" || ownerDownload.ExpiresAt == 0 {
		t.Fatalf("missing owner download URL fields: %+v", ownerDownload)
	}

	download, err := mediaLogic.GetDownloadURL(ctx, core.GetMediaDownloadURLRequest{
		OwnerUserID: "usr_receiver",
		MediaID:     image.MediaID,
	})
	if err != nil {
		t.Fatalf("receiver get download URL: %v", err)
	}
	if download.DownloadURL == "" || download.ExpiresAt == 0 {
		t.Fatalf("missing receiver download URL fields: %+v", download)
	}

	_, err = mediaLogic.GetDownloadURL(ctx, core.GetMediaDownloadURLRequest{
		OwnerUserID: "usr_outsider",
		MediaID:     image.MediaID,
	})
	assertLogicAppCode(t, err, apperror.CodeForbidden)
}

func TestGroupMemberCannotGetMediaDownloadURLBeforeJoinBoundary(t *testing.T) {
	ctx := context.Background()
	store := objectstorage.NewMemoryStore()
	mediaRepo := repository.NewMemoryMediaRepository()
	messageRepo := repository.NewMemoryMessageRepository()
	mediaLogic := core.NewMediaLogic(mediaRepo, store, "agents-im-media").
		WithAttachmentAccessChecker(core.NewMessageMediaAccessChecker(messageRepo))
	groups := &testGroupMemberLister{
		members: []GroupMemberInfo{
			{UserID: "usr_sender", State: "active"},
			{UserID: "usr_existing_member", State: "active"},
		},
	}
	messageLogic := NewMessageLogicWithMediaValidator(messageRepo, nil, groups, mediavalidate.NewMessageValidator(mediaRepo))

	beforeJoinImage := createMediaForTest(t, mediaRepo, model.MediaObject{
		MediaID:          "med_group_before_join",
		OwnerUserID:      "usr_sender",
		Bucket:           "agents-im-media",
		ObjectKey:        "users/usr_sender/media/med_group_before_join/before.jpg",
		ContentType:      "image/jpeg",
		SizeBytes:        1024,
		OriginalFilename: "before.jpg",
		Purpose:          model.MediaPurposeMessageImage,
		Status:           model.MediaStatusReady,
	})
	afterJoinImage := createMediaForTest(t, mediaRepo, model.MediaObject{
		MediaID:          "med_group_after_join",
		OwnerUserID:      "usr_sender",
		Bucket:           "agents-im-media",
		ObjectKey:        "users/usr_sender/media/med_group_after_join/after.jpg",
		ContentType:      "image/jpeg",
		SizeBytes:        1024,
		OriginalFilename: "after.jpg",
		Purpose:          model.MediaPurposeMessageImage,
		Status:           model.MediaStatusReady,
	})

	first, err := messageLogic.SendMessage(ctx, SendMessageRequest{
		SenderID:    "usr_sender",
		GroupID:     "grp_media_boundary",
		ChatType:    MessageChatTypeGroup,
		ClientMsgID: "client-group-before-join-image",
		ContentType: MessageContentTypeImage,
		Content:     `{"mediaId":"med_group_before_join","filename":"before.jpg"}`,
	})
	if err != nil {
		t.Fatalf("send pre-join image: %v", err)
	}

	groups.members = []GroupMemberInfo{
		{UserID: "usr_sender", State: "active"},
		{UserID: "usr_existing_member", State: "active"},
		{UserID: "usr_new_member", State: "active"},
	}
	second, err := messageLogic.SendMessage(ctx, SendMessageRequest{
		SenderID:    "usr_sender",
		GroupID:     "grp_media_boundary",
		ChatType:    MessageChatTypeGroup,
		ClientMsgID: "client-group-after-join-image",
		ContentType: MessageContentTypeImage,
		Content:     `{"mediaId":"med_group_after_join","filename":"after.jpg"}`,
	})
	if err != nil {
		t.Fatalf("send post-join image: %v", err)
	}
	if first.Message.Seq >= second.Message.Seq {
		t.Fatalf("test setup expected first seq before second: first=%d second=%d", first.Message.Seq, second.Message.Seq)
	}

	_, err = mediaLogic.GetDownloadURL(ctx, core.GetMediaDownloadURLRequest{
		OwnerUserID: "usr_new_member",
		MediaID:     beforeJoinImage.MediaID,
	})
	assertLogicAppCode(t, err, apperror.CodeForbidden)

	download, err := mediaLogic.GetDownloadURL(ctx, core.GetMediaDownloadURLRequest{
		OwnerUserID: "usr_new_member",
		MediaID:     afterJoinImage.MediaID,
	})
	if err != nil {
		t.Fatalf("new member get post-join media URL: %v", err)
	}
	if download.DownloadURL == "" || download.ExpiresAt == 0 {
		t.Fatalf("missing new member download URL fields: %+v", download)
	}
}
