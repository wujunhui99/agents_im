package logic

import (
	"context"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/pkg/objectstorage"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestMediaUploadIntentValidationAndObjectKeyGeneration(t *testing.T) {
	ctx := context.Background()
	store := objectstorage.NewMemoryStore()
	mediaLogic := NewMediaLogic(repository.NewMemoryMediaRepository(), store, "agents-im-media")

	result, err := mediaLogic.CreateUploadIntent(ctx, CreateMediaUploadIntentRequest{
		OwnerUserID: "usr_media_owner",
		Purpose:     MediaPurposeMessageImage,
		Filename:    "../../client/chosen/cat photo.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   123456,
		SHA256:      strings.Repeat("a", 64),
		Width:       1080,
		Height:      720,
	})
	if err != nil {
		t.Fatalf("create upload intent: %v", err)
	}
	if !strings.HasPrefix(result.MediaID, "med_") {
		t.Fatalf("media id = %q, want med_ prefix", result.MediaID)
	}
	wantPrefix := "users/usr_media_owner/media/" + result.MediaID + "/"
	if !strings.HasPrefix(result.ObjectKey, wantPrefix) {
		t.Fatalf("object key = %q, want prefix %q", result.ObjectKey, wantPrefix)
	}
	if strings.Contains(result.ObjectKey, "..") || strings.Contains(result.ObjectKey, "client/chosen") {
		t.Fatalf("object key should not contain client path components: %q", result.ObjectKey)
	}
	if result.UploadURL == "" || result.ExpiresAt == 0 {
		t.Fatalf("missing presigned upload fields: %+v", result)
	}

	cases := []CreateMediaUploadIntentRequest{
		{OwnerUserID: "usr_media_owner", Purpose: "bad", Filename: "a.jpg", ContentType: "image/jpeg", SizeBytes: 1},
		{OwnerUserID: "usr_media_owner", Purpose: MediaPurposeAvatar, Filename: "a.svg", ContentType: "image/svg+xml", SizeBytes: 1},
		{OwnerUserID: "usr_media_owner", Purpose: MediaPurposeMessageImage, Filename: "a.jpg", ContentType: "image/jpeg", SizeBytes: MediaMaxImageBytes + 1},
		{OwnerUserID: "usr_media_owner", Purpose: MediaPurposeMessageFile, Filename: "a.html", ContentType: "text/html", SizeBytes: 1},
		{OwnerUserID: "usr_media_owner", Purpose: MediaPurposeMessageFile, Filename: "a.pdf", ContentType: "application/pdf", SizeBytes: 1, SHA256: strings.Repeat("A", 64)},
	}
	for _, req := range cases {
		if _, err := mediaLogic.CreateUploadIntent(ctx, req); err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
			t.Fatalf("CreateUploadIntent(%+v) error = %v, want INVALID_ARGUMENT", req, err)
		}
	}
}

func TestMediaUploadIntentMessageSizeLimits(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name    string
		req     CreateMediaUploadIntentRequest
		wantErr bool
	}{
		{
			name: "message image accepts fifteen mib",
			req: CreateMediaUploadIntentRequest{
				OwnerUserID: "usr_media_limits",
				Purpose:     MediaPurposeMessageImage,
				Filename:    "image.jpg",
				ContentType: "image/jpeg",
				SizeBytes:   15 * 1024 * 1024,
				Width:       1080,
				Height:      720,
			},
		},
		{
			name: "message image rejects over fifteen mib",
			req: CreateMediaUploadIntentRequest{
				OwnerUserID: "usr_media_limits",
				Purpose:     MediaPurposeMessageImage,
				Filename:    "image.jpg",
				ContentType: "image/jpeg",
				SizeBytes:   15*1024*1024 + 1,
				Width:       1080,
				Height:      720,
			},
			wantErr: true,
		},
		{
			name: "message file accepts twenty mib",
			req: CreateMediaUploadIntentRequest{
				OwnerUserID: "usr_media_limits",
				Purpose:     MediaPurposeMessageFile,
				Filename:    "report.pdf",
				ContentType: "application/pdf",
				SizeBytes:   20 * 1024 * 1024,
			},
		},
		{
			name: "message file rejects over twenty mib",
			req: CreateMediaUploadIntentRequest{
				OwnerUserID: "usr_media_limits",
				Purpose:     MediaPurposeMessageFile,
				Filename:    "report.pdf",
				ContentType: "application/pdf",
				SizeBytes:   20*1024*1024 + 1,
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mediaLogic := NewMediaLogic(repository.NewMemoryMediaRepository(), objectstorage.NewMemoryStore(), "agents-im-media")
			_, err := mediaLogic.CreateUploadIntent(ctx, tc.req)
			if tc.wantErr {
				assertLogicAppCode(t, err, apperror.CodeInvalidArgument)
				return
			}
			if err != nil {
				t.Fatalf("create upload intent: %v", err)
			}
		})
	}
}

func TestMediaCompleteAndDownloadRequireOwnerAndObjectStat(t *testing.T) {
	ctx := context.Background()
	store := objectstorage.NewMemoryStore()
	mediaLogic := NewMediaLogic(repository.NewMemoryMediaRepository(), store, "agents-im-media")

	intent, err := mediaLogic.CreateUploadIntent(ctx, CreateMediaUploadIntentRequest{
		OwnerUserID: "usr_owner",
		Purpose:     MediaPurposeMessageFile,
		Filename:    "report.pdf",
		ContentType: "application/pdf",
		SizeBytes:   42,
	})
	if err != nil {
		t.Fatalf("create upload intent: %v", err)
	}

	_, err = mediaLogic.CompleteUpload(ctx, CompleteMediaUploadRequest{OwnerUserID: "usr_other", MediaID: intent.MediaID})
	assertLogicAppCode(t, err, apperror.CodeForbidden)

	_, err = mediaLogic.CompleteUpload(ctx, CompleteMediaUploadRequest{OwnerUserID: "usr_owner", MediaID: intent.MediaID})
	assertLogicAppCode(t, err, apperror.CodeNotFound)

	store.PutObjectInfo(objectstorage.ObjectInfo{ObjectKey: intent.ObjectKey, ContentType: "text/plain", SizeBytes: 42})
	_, err = mediaLogic.CompleteUpload(ctx, CompleteMediaUploadRequest{OwnerUserID: "usr_owner", MediaID: intent.MediaID})
	assertLogicAppCode(t, err, apperror.CodeInvalidArgument)

	store.PutObjectInfo(objectstorage.ObjectInfo{ObjectKey: intent.ObjectKey, ContentType: "application/pdf", SizeBytes: 42})
	completed, err := mediaLogic.CompleteUpload(ctx, CompleteMediaUploadRequest{OwnerUserID: "usr_owner", MediaID: intent.MediaID})
	if err != nil {
		t.Fatalf("complete upload: %v", err)
	}
	if completed.Media.Status != string(model.MediaStatusReady) {
		t.Fatalf("status = %q, want ready", completed.Media.Status)
	}

	_, err = mediaLogic.GetDownloadURL(ctx, GetMediaDownloadURLRequest{OwnerUserID: "usr_other", MediaID: intent.MediaID})
	assertLogicAppCode(t, err, apperror.CodeForbidden)

	download, err := mediaLogic.GetDownloadURL(ctx, GetMediaDownloadURLRequest{OwnerUserID: "usr_owner", MediaID: intent.MediaID})
	if err != nil {
		t.Fatalf("get download URL: %v", err)
	}
	if download.DownloadURL == "" || download.ExpiresAt == 0 {
		t.Fatalf("missing download URL fields: %+v", download)
	}
}

func TestAdminCanGetFeedbackAttachmentDownloadURL(t *testing.T) {
	ctx := context.Background()
	store := objectstorage.NewMemoryStore()
	mediaRepo := repository.NewMemoryMediaRepository()
	accountRepo := repository.NewMemoryRepository()
	if _, err := accountRepo.Create(ctx, model.User{UserID: "usr_admin", Identifier: "admin-feedback-media", AccountType: model.AccountTypeAdmin}); err != nil {
		t.Fatalf("create admin account: %v", err)
	}
	mediaLogic := NewMediaLogic(mediaRepo, store, "agents-im-media").WithAccountRepository(accountRepo)
	image := createMediaForTest(t, mediaRepo, model.MediaObject{
		MediaID:          "med_feedback_image_ready",
		OwnerUserID:      "usr_feedback_owner",
		Bucket:           "agents-im-media",
		ObjectKey:        "users/usr_feedback_owner/media/med_feedback_image_ready/screenshot.jpg",
		ContentType:      "image/jpeg",
		SizeBytes:        1024,
		OriginalFilename: "screenshot.jpg",
		Purpose:          model.MediaPurposeMessageImage,
		Status:           model.MediaStatusReady,
	})
	store.PutObjectInfo(objectstorage.ObjectInfo{ObjectKey: image.ObjectKey, ContentType: image.ContentType, SizeBytes: image.SizeBytes})

	download, err := mediaLogic.GetDownloadURL(ctx, GetMediaDownloadURLRequest{
		OwnerUserID:     "usr_feedback_owner",
		RequesterUserID: "usr_admin",
		MediaID:         image.MediaID,
	})
	if err != nil {
		t.Fatalf("admin get feedback attachment download URL: %v", err)
	}
	if download.DownloadURL == "" || download.ExpiresAt == 0 {
		t.Fatalf("missing admin download URL fields: %+v", download)
	}
}

func TestMessageParticipantCanGetMediaDownloadURL(t *testing.T) {
	ctx := context.Background()
	store := objectstorage.NewMemoryStore()
	mediaRepo := repository.NewMemoryMediaRepository()
	messageRepo := repository.NewMemoryMessageRepository()
	mediaLogic := NewMediaLogic(mediaRepo, store, "agents-im-media").
		WithAttachmentAccessChecker(NewMessageMediaAccessChecker(messageRepo))
	messageLogic := NewMessageLogicWithMediaValidator(messageRepo, nil, nil, mediaLogic)

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

	ownerDownload, err := mediaLogic.GetDownloadURL(ctx, GetMediaDownloadURLRequest{
		OwnerUserID: "usr_sender",
		MediaID:     image.MediaID,
	})
	if err != nil {
		t.Fatalf("owner get download URL: %v", err)
	}
	if ownerDownload.DownloadURL == "" || ownerDownload.ExpiresAt == 0 {
		t.Fatalf("missing owner download URL fields: %+v", ownerDownload)
	}

	download, err := mediaLogic.GetDownloadURL(ctx, GetMediaDownloadURLRequest{
		OwnerUserID: "usr_receiver",
		MediaID:     image.MediaID,
	})
	if err != nil {
		t.Fatalf("receiver get download URL: %v", err)
	}
	if download.DownloadURL == "" || download.ExpiresAt == 0 {
		t.Fatalf("missing receiver download URL fields: %+v", download)
	}

	_, err = mediaLogic.GetDownloadURL(ctx, GetMediaDownloadURLRequest{
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
	mediaLogic := NewMediaLogic(mediaRepo, store, "agents-im-media").
		WithAttachmentAccessChecker(NewMessageMediaAccessChecker(messageRepo))
	groups := &testGroupMemberLister{
		members: []GroupMemberInfo{
			{UserID: "usr_sender", State: "active"},
			{UserID: "usr_existing_member", State: "active"},
		},
	}
	messageLogic := NewMessageLogicWithMediaValidator(messageRepo, nil, groups, mediaLogic)

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

	_, err = mediaLogic.GetDownloadURL(ctx, GetMediaDownloadURLRequest{
		OwnerUserID: "usr_new_member",
		MediaID:     beforeJoinImage.MediaID,
	})
	assertLogicAppCode(t, err, apperror.CodeForbidden)

	download, err := mediaLogic.GetDownloadURL(ctx, GetMediaDownloadURLRequest{
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

func TestAvatarMediaValidationRequiresOwnerPurposeAndReady(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryMediaRepository()
	store := objectstorage.NewMemoryStore()
	mediaLogic := NewMediaLogic(repo, store, "agents-im-media")

	avatar := createMediaForTest(t, repo, model.MediaObject{
		MediaID:     "med_avatar_ready",
		OwnerUserID: "usr_avatar",
		Bucket:      "agents-im-media",
		ObjectKey:   "users/usr_avatar/media/med_avatar_ready/avatar.png",
		ContentType: "image/png",
		SizeBytes:   1024,
		Purpose:     model.MediaPurposeAvatar,
		Status:      model.MediaStatusReady,
	})
	if _, err := mediaLogic.ValidateAvatarMedia(ctx, "usr_avatar", avatar.MediaID); err != nil {
		t.Fatalf("validate avatar media: %v", err)
	}
	display, err := mediaLogic.GetAvatarDisplayURL(ctx, avatar.MediaID)
	if err != nil {
		t.Fatalf("get avatar display URL: %v", err)
	}
	if display.MediaID != avatar.MediaID || display.DownloadURL == "" || display.ExpiresAt == 0 {
		t.Fatalf("avatar display URL missing fields: %+v", display)
	}

	notReady := createMediaForTest(t, repo, model.MediaObject{
		MediaID:     "med_avatar_pending",
		OwnerUserID: "usr_avatar",
		Bucket:      "agents-im-media",
		ObjectKey:   "users/usr_avatar/media/med_avatar_pending/avatar.png",
		ContentType: "image/png",
		SizeBytes:   1024,
		Purpose:     model.MediaPurposeAvatar,
		Status:      model.MediaStatusPending,
	})
	_, err = mediaLogic.ValidateAvatarMedia(ctx, "usr_avatar", notReady.MediaID)
	assertLogicAppCode(t, err, apperror.CodeInvalidArgument)

	wrongPurpose := createMediaForTest(t, repo, model.MediaObject{
		MediaID:     "med_avatar_wrong_purpose",
		OwnerUserID: "usr_avatar",
		Bucket:      "agents-im-media",
		ObjectKey:   "users/usr_avatar/media/med_avatar_wrong_purpose/avatar.png",
		ContentType: "image/png",
		SizeBytes:   1024,
		Purpose:     model.MediaPurposeMessageImage,
		Status:      model.MediaStatusReady,
	})
	_, err = mediaLogic.ValidateAvatarMedia(ctx, "usr_avatar", wrongPurpose.MediaID)
	assertLogicAppCode(t, err, apperror.CodeInvalidArgument)

	_, err = mediaLogic.ValidateAvatarMedia(ctx, "usr_other", avatar.MediaID)
	assertLogicAppCode(t, err, apperror.CodeForbidden)

	oversized := createMediaForTest(t, repo, model.MediaObject{
		MediaID:     "med_avatar_oversized",
		OwnerUserID: "usr_avatar",
		Bucket:      "agents-im-media",
		ObjectKey:   "users/usr_avatar/media/med_avatar_oversized/avatar.png",
		ContentType: "image/png",
		SizeBytes:   MediaMaxAvatarBytes + 1,
		Purpose:     model.MediaPurposeAvatar,
		Status:      model.MediaStatusReady,
	})
	_, err = mediaLogic.ValidateAvatarMedia(ctx, "usr_avatar", oversized.MediaID)
	assertLogicAppCode(t, err, apperror.CodeInvalidArgument)

	gifAvatar := createMediaForTest(t, repo, model.MediaObject{
		MediaID:     "med_avatar_gif",
		OwnerUserID: "usr_avatar",
		Bucket:      "agents-im-media",
		ObjectKey:   "users/usr_avatar/media/med_avatar_gif/avatar.gif",
		ContentType: "image/gif",
		SizeBytes:   1024,
		Purpose:     model.MediaPurposeAvatar,
		Status:      model.MediaStatusReady,
	})
	_, err = mediaLogic.ValidateAvatarMedia(ctx, "usr_avatar", gifAvatar.MediaID)
	assertLogicAppCode(t, err, apperror.CodeInvalidArgument)
	_, err = mediaLogic.GetAvatarDisplayURL(ctx, gifAvatar.MediaID)
	assertLogicAppCode(t, err, apperror.CodeInvalidArgument)
}

func createMediaForTest(t *testing.T, repo repository.MediaRepository, media model.MediaObject) model.MediaObject {
	t.Helper()
	created, err := repo.CreateMediaObject(context.Background(), media)
	if err != nil {
		t.Fatalf("create media fixture: %v", err)
	}
	return created
}
