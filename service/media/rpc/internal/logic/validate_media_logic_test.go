package logic

import (
	"context"
	"fmt"
	"testing"

	"github.com/wujunhui99/agents_im/service/media/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/media"

	"google.golang.org/grpc/codes"
)

// 校验规则随 #533 从 internal/mediavalidate 迁入 media-rpc：这里覆盖头像与消息附件
// （image/file）的 purpose/ready/owned/类型/大小/元数据匹配规则。wantCode 复用 media_logic_test.go。

const validatorOwner = "323499294365372416"

func readyMediaRow(mediaID int64, owner string, purpose int64, contentType string, sizeBytes int64) *model.MediaObjects {
	return &model.MediaObjects{
		MediaId:          mediaID,
		UploaderId:       owner,
		Bucket:           "agents-im-media",
		ObjectKey:        fmt.Sprintf("agents_im/%064d", mediaID),
		OriginalFilename: "f",
		ContentType:      contentType,
		SizeBytes:        sizeBytes,
		Purpose:          purpose,
		Status:           model.MediaStatusReady,
		DigestAlgo:       model.MediaDigestAlgoSHA256,
	}
}

func avatarCtx(rows ...*model.MediaObjects) *ValidateAvatarMediaLogic {
	return NewValidateAvatarMediaLogic(context.Background(), &svc.ServiceContext{MediaModel: newFakeMediaModel(rows...)})
}

func messageCtx(rows ...*model.MediaObjects) *ValidateMessageMediaLogic {
	return NewValidateMessageMediaLogic(context.Background(), &svc.ServiceContext{MediaModel: newFakeMediaModel(rows...)})
}

func TestValidateAvatarMedia(t *testing.T) {
	const id = int64(60927948672204801)
	row := readyMediaRow(id, validatorOwner, model.MediaPurposeAvatar, "image/jpeg", 1024)
	if _, err := avatarCtx(row).ValidateAvatarMedia(&media.ValidateAvatarMediaRequest{
		OwnerUserId: validatorOwner, MediaId: formatMediaID(id),
	}); err != nil {
		t.Fatalf("ready avatar: %v", err)
	}

	_, err := avatarCtx(row).ValidateAvatarMedia(&media.ValidateAvatarMediaRequest{OwnerUserId: "999", MediaId: formatMediaID(id)})
	wantCode(t, err, codes.PermissionDenied)

	bad := readyMediaRow(id+1, validatorOwner, model.MediaPurposeMessageImage, "image/jpeg", 1024)
	_, err = avatarCtx(bad).ValidateAvatarMedia(&media.ValidateAvatarMediaRequest{OwnerUserId: validatorOwner, MediaId: formatMediaID(id + 1)})
	wantCode(t, err, codes.InvalidArgument)

	big := readyMediaRow(id+2, validatorOwner, model.MediaPurposeAvatar, "image/jpeg", 5*1024*1024+1)
	_, err = avatarCtx(big).ValidateAvatarMedia(&media.ValidateAvatarMediaRequest{OwnerUserId: validatorOwner, MediaId: formatMediaID(id + 2)})
	wantCode(t, err, codes.InvalidArgument)
}

func TestValidateMessageMediaImageAndFile(t *testing.T) {
	const imgID = int64(60927948672300001)
	const fileID = int64(60927948672300002)
	img := readyMediaRow(imgID, validatorOwner, model.MediaPurposeMessageImage, "image/jpeg", 1024)
	file := readyMediaRow(fileID, validatorOwner, model.MediaPurposeMessageFile, "application/pdf", 2048)

	okImage := fmt.Sprintf(`{"mediaId":%q,"width":10,"height":10}`, formatMediaID(imgID))
	if _, err := messageCtx(img).ValidateMessageMedia(&media.ValidateMessageMediaRequest{
		OwnerUserId: validatorOwner, ContentType: "image", Content: okImage,
	}); err != nil {
		t.Fatalf("ready image: %v", err)
	}

	okFile := fmt.Sprintf(`{"mediaId":%q,"filename":"report.pdf","sizeBytes":2048,"contentType":"application/pdf"}`, formatMediaID(fileID))
	if _, err := messageCtx(file).ValidateMessageMedia(&media.ValidateMessageMediaRequest{
		OwnerUserId: validatorOwner, ContentType: "file", Content: okFile,
	}); err != nil {
		t.Fatalf("ready file: %v", err)
	}

	badMeta := fmt.Sprintf(`{"mediaId":%q,"filename":"report.pdf","sizeBytes":1,"contentType":"application/pdf"}`, formatMediaID(fileID))
	_, err := messageCtx(file).ValidateMessageMedia(&media.ValidateMessageMediaRequest{OwnerUserId: validatorOwner, ContentType: "file", Content: badMeta})
	wantCode(t, err, codes.InvalidArgument)

	_, err = messageCtx().ValidateMessageMedia(&media.ValidateMessageMediaRequest{OwnerUserId: validatorOwner, ContentType: "video", Content: "{}"})
	wantCode(t, err, codes.InvalidArgument)
}

func TestValidateMessageMediaSizeLimits(t *testing.T) {
	const imgID = int64(60927948672400001)
	const fileID = int64(60927948672400002)
	overImg := readyMediaRow(imgID, validatorOwner, model.MediaPurposeMessageImage, "image/jpeg", 15*1024*1024+1)
	_, err := messageCtx(overImg).ValidateMessageMedia(&media.ValidateMessageMediaRequest{
		OwnerUserId: validatorOwner, ContentType: "image", Content: fmt.Sprintf(`{"mediaId":%q}`, formatMediaID(imgID)),
	})
	wantCode(t, err, codes.InvalidArgument)

	overFile := readyMediaRow(fileID, validatorOwner, model.MediaPurposeMessageFile, "application/pdf", 20*1024*1024+1)
	content := fmt.Sprintf(`{"mediaId":%q,"filename":"r.pdf","sizeBytes":%d,"contentType":"application/pdf"}`, formatMediaID(fileID), 20*1024*1024+1)
	_, err = messageCtx(overFile).ValidateMessageMedia(&media.ValidateMessageMediaRequest{OwnerUserId: validatorOwner, ContentType: "file", Content: content})
	wantCode(t, err, codes.InvalidArgument)
}
