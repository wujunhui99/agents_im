package handler

import (
	"net/http"

	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/service/media/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// CompleteUploadHandler marks an uploaded object ready for the authenticated user.
func CompleteUploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CompleteMediaUploadReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		userID, err := ctxuser.UserID(r.Context())
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		resp, err := svcCtx.MediaRPC.CompleteUpload(r.Context(), &mediaclient.CompleteUploadRequest{
			OwnerUserId: userID,
			MediaId:     req.MediaID,
		})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, &types.CompleteMediaUploadResp{
			Code:    string(apperror.CodeOK),
			Message: "ok",
			Data:    types.CompleteMediaUploadData{Media: mediaObjectFromRPC(resp.GetMedia())},
		})
	}
}

func mediaObjectFromRPC(m *mediaclient.MediaObject) types.MediaObject {
	if m == nil {
		return types.MediaObject{}
	}
	return types.MediaObject{
		MediaID:          m.GetMediaId(),
		OwnerUserID:      m.GetOwnerUserId(),
		Bucket:           m.GetBucket(),
		ObjectKey:        m.GetObjectKey(),
		SHA256:           m.GetSha256(),
		ContentType:      m.GetContentType(),
		SizeBytes:        m.GetSizeBytes(),
		Width:            m.GetWidth(),
		Height:           m.GetHeight(),
		OriginalFilename: m.GetOriginalFilename(),
		Purpose:          m.GetPurpose(),
		Status:           m.GetStatus(),
		CreatedAt:        m.GetCreatedAt(),
		UpdatedAt:        m.GetUpdatedAt(),
	}
}
