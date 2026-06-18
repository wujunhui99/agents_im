package logic

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/media"

	"github.com/zeromicro/go-zero/core/logx"
)

// 消息媒体引用的两种 content kind（消息域承载在 message.content_type 上）。
const (
	messageKindImage = "image"
	messageKindFile  = "file"
)

type messageImageContent struct {
	MediaID string `json:"mediaId"`
	Width   int32  `json:"width,omitempty"`
	Height  int32  `json:"height,omitempty"`
}

type messageFileContent struct {
	MediaID     string `json:"mediaId"`
	Filename    string `json:"filename"`
	SizeBytes   int64  `json:"sizeBytes"`
	ContentType string `json:"contentType"`
}

type ValidateMessageMediaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateMessageMediaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateMessageMediaLogic {
	return &ValidateMessageMediaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ValidateMessageMedia 校验消息附件引用：解析 content JSON（image/file），确认引用的 media 是 owner
// 拥有、ready、purpose 匹配且类型/大小合法的对象（file 还要 content 元数据与 media 一致）。
// #533，取代 internal/mediavalidate.MessageValidator。
func (l *ValidateMessageMediaLogic) ValidateMessageMedia(in *media.ValidateMessageMediaRequest) (*media.ValidateMediaResponse, error) {
	switch strings.ToLower(strings.TrimSpace(in.GetContentType())) {
	case messageKindImage:
		if err := l.validateImage(in.GetOwnerUserId(), in.GetContent()); err != nil {
			return nil, rpcerror.ToStatus(err)
		}
	case messageKindFile:
		if err := l.validateFile(in.GetOwnerUserId(), in.GetContent()); err != nil {
			return nil, rpcerror.ToStatus(err)
		}
	default:
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("content_type must be image or file"))
	}
	return &media.ValidateMediaResponse{}, nil
}

func (l *ValidateMessageMediaLogic) validateImage(ownerUserID, content string) error {
	var body messageImageContent
	if err := json.Unmarshal([]byte(content), &body); err != nil {
		return apperror.InvalidArgument("image content must be a JSON object")
	}
	obj, err := l.mediaRef(ownerUserID, body.MediaID)
	if err != nil {
		return err
	}
	if obj.Purpose != model.MediaPurposeMessageImage {
		return apperror.InvalidArgument("image media purpose is invalid")
	}
	if obj.Status != model.MediaStatusReady {
		return apperror.InvalidArgument("image media is not ready")
	}
	if !isAllowedImageContentType(obj.ContentType) {
		return apperror.InvalidArgument("image media content_type must be an allowed image type")
	}
	if obj.SizeBytes > maxImageBytes {
		return apperror.InvalidArgument("image media exceeds size limit")
	}
	return nil
}

func (l *ValidateMessageMediaLogic) validateFile(ownerUserID, content string) error {
	var body messageFileContent
	if err := json.Unmarshal([]byte(content), &body); err != nil {
		return apperror.InvalidArgument("file content must be a JSON object")
	}
	if _, err := validateOriginalFilename(body.Filename); err != nil {
		return err
	}
	body.ContentType = strings.ToLower(strings.TrimSpace(body.ContentType))
	if body.SizeBytes <= 0 {
		return apperror.InvalidArgument("file sizeBytes must be positive")
	}
	obj, err := l.mediaRef(ownerUserID, body.MediaID)
	if err != nil {
		return err
	}
	if obj.Purpose != model.MediaPurposeMessageFile {
		return apperror.InvalidArgument("file media purpose is invalid")
	}
	if obj.Status != model.MediaStatusReady {
		return apperror.InvalidArgument("file media is not ready")
	}
	if !isAllowedFileContentType(obj.ContentType) {
		return apperror.InvalidArgument("file media content_type is not allowed")
	}
	if body.ContentType != obj.ContentType || body.SizeBytes != obj.SizeBytes {
		return apperror.InvalidArgument("file content metadata does not match media object")
	}
	if obj.SizeBytes > maxFileBytes {
		return apperror.InvalidArgument("file media exceeds size limit")
	}
	return nil
}

func (l *ValidateMessageMediaLogic) mediaRef(ownerUserID, mediaID string) (*model.MediaObjects, error) {
	id, err := parseMediaID(mediaID)
	if err != nil {
		return nil, err
	}
	return mediaForOwner(l.ctx, l.svcCtx, ownerUserID, id)
}
