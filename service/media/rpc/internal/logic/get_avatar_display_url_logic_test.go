package logic

import (
	"context"
	"testing"

	business "github.com/wujunhui99/agents_im/service/media/core"
	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/objectstorage"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/media"
)

func TestGetAvatarDisplayURLLogicReturnsPresignedURL(t *testing.T) {
	mediaRepo := repository.NewMemoryMediaRepository()
	store := objectstorage.NewMemoryStore()
	const objectKey = "avatar/route-user/med_rpc_avatar.png"
	if _, err := mediaRepo.CreateMediaObject(context.Background(), model.MediaObject{
		MediaID:          "med_rpc_avatar",
		OwnerUserID:      "route-user",
		Bucket:           "agents-im-media",
		ObjectKey:        objectKey,
		ContentType:      "image/png",
		SizeBytes:        128,
		OriginalFilename: "avatar.png",
		Purpose:          model.MediaPurposeAvatar,
		Status:           model.MediaStatusReady,
	}); err != nil {
		t.Fatalf("seed avatar media: %v", err)
	}
	store.PutObjectInfo(objectstorage.ObjectInfo{ObjectKey: objectKey, ContentType: "image/png", SizeBytes: 128})

	svcCtx := &svc.ServiceContext{
		MediaLogic: business.NewMediaLogic(mediaRepo, store, "agents-im-media"),
	}
	resp, err := NewGetAvatarDisplayURLLogic(context.Background(), svcCtx).
		GetAvatarDisplayURL(&media.GetAvatarDisplayURLRequest{MediaId: "med_rpc_avatar"})
	if err != nil {
		t.Fatalf("GetAvatarDisplayURL: %v", err)
	}
	if resp.GetMediaId() != "med_rpc_avatar" || resp.GetDownloadUrl() == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
