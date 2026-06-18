package handler

import (
	"net/http"

	"github.com/wujunhui99/agents_im/pkg/types"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/service/media/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// CreateUploadIntentHandler issues a presigned upload URL for the authenticated
// user. Owner is taken from the JWT, never the request body.
func CreateUploadIntentHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateMediaUploadReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		userID, err := ctxuser.UserID(r.Context())
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		resp, err := svcCtx.MediaRPC.CreateUploadIntent(r.Context(), &mediaclient.CreateUploadIntentRequest{
			OwnerUserId: userID,
			Purpose:     req.Purpose,
			Filename:    req.Filename,
			ContentType: req.ContentType,
			SizeBytes:   req.SizeBytes,
			Sha256:      req.SHA256,
			Width:       req.Width,
			Height:      req.Height,
		})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, apiError(err))
			return
		}
		httpx.OkJsonCtx(r.Context(), w, &types.CreateMediaUploadResp{
			Code:    string(apperror.CodeOK),
			Message: "ok",
			Data: types.CreateMediaUploadData{
				MediaID:         resp.GetMediaId(),
				ObjectKey:       resp.GetObjectKey(),
				UploadURL:       resp.GetUploadUrl(),
				ExpiresAt:       resp.GetExpiresAt(),
				AlreadyComplete: resp.GetAlreadyComplete(),
			},
		})
	}
}
