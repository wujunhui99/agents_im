package media

import (
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/types"
)

func toMediaObject(media business.MediaObject) types.MediaObject {
	return types.MediaObject{
		MediaID:          media.MediaID,
		OwnerUserID:      media.OwnerUserID,
		Bucket:           media.Bucket,
		ObjectKey:        media.ObjectKey,
		SHA256:           media.SHA256,
		ContentType:      media.ContentType,
		SizeBytes:        media.SizeBytes,
		Width:            media.Width,
		Height:           media.Height,
		OriginalFilename: media.OriginalFilename,
		Purpose:          media.Purpose,
		Status:           media.Status,
		CreatedAt:        media.CreatedAt,
		UpdatedAt:        media.UpdatedAt,
	}
}
