package logic

import (
	"context"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/objectstorage"
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

func TestAvatarMediaValidationRequiresOwnerPurposeAndReady(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryMediaRepository()
	mediaLogic := NewMediaLogic(repo, nil, "agents-im-media")

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
	_, err := mediaLogic.ValidateAvatarMedia(ctx, "usr_avatar", notReady.MediaID)
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
}

func createMediaForTest(t *testing.T, repo repository.MediaRepository, media model.MediaObject) model.MediaObject {
	t.Helper()
	created, err := repo.CreateMediaObject(context.Background(), media)
	if err != nil {
		t.Fatalf("create media fixture: %v", err)
	}
	return created
}
