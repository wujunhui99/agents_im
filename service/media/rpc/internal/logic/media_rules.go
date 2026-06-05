package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"path"
	"regexp"
	"strings"
	"time"

	sharedmodel "github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	mediapb "github.com/wujunhui99/agents_im/service/media/rpc/media"
)

// 媒体业务规则：purpose/status 整型(model/vars.go) <-> 字符串契约映射、输入校验（只 validate 不
// normalize，清洗由客户端负责）、对象存储 key 生成、内容类型白名单、下载鉴权。数据层走 svcCtx.MediaModel
// (goctl)；跨域下载鉴权（管理员/消息附件可见性）经 svcCtx.Accounts/AttachmentAccess 读 internal/repository，
// 是 keystone 阻塞的过渡，待 message-rpc 落地后 BFF 化（见 issue #433）。

const (
	purposeAvatar       = "avatar"
	purposeMessageImage = "message_image"
	purposeMessageFile  = "message_file"
	purposeAgentSkill   = "agent_skill"

	statusPending  = "pending"
	statusReady    = "ready"
	statusRejected = "rejected"
	statusDeleted  = "deleted"
)

const (
	maxAvatarBytes = 5 * 1024 * 1024
	maxImageBytes  = 15 * 1024 * 1024
	maxFileBytes   = 20 * 1024 * 1024

	uploadURLTTL         = 15 * time.Minute
	downloadURLTTL       = 10 * time.Minute
	avatarDownloadURLTTL = 24 * time.Hour
)

var sha256HexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// --- purpose / status 整型 <-> 字符串映射 ---

func purposeToDB(purpose string) (int64, bool) {
	switch purpose {
	case purposeAvatar:
		return model.MediaPurposeAvatar, true
	case purposeMessageImage:
		return model.MediaPurposeMessageImage, true
	case purposeMessageFile:
		return model.MediaPurposeMessageFile, true
	case purposeAgentSkill:
		return model.MediaPurposeAgentSkill, true
	default:
		return 0, false
	}
}

func purposeToString(purpose int64) string {
	switch purpose {
	case model.MediaPurposeAvatar:
		return purposeAvatar
	case model.MediaPurposeMessageImage:
		return purposeMessageImage
	case model.MediaPurposeAgentSkill:
		return purposeAgentSkill
	default:
		return purposeMessageFile
	}
}

func statusToString(status int64) string {
	switch status {
	case model.MediaStatusReady:
		return statusReady
	case model.MediaStatusRejected:
		return statusRejected
	case model.MediaStatusDeleted:
		return statusDeleted
	default:
		return statusPending
	}
}

// --- 输入校验（不做规范化）---

func validateMediaIDComponent(value string, field string) (string, error) {
	if value == "" {
		return "", apperror.InvalidArgument(field + " is required")
	}
	if len([]rune(value)) > 128 {
		return "", apperror.InvalidArgument(field + " must be 128 characters or fewer")
	}
	if strings.Contains(value, "\x00") || strings.Contains(value, ":") || strings.Contains(value, "/") {
		return "", apperror.InvalidArgument(field + " contains invalid characters")
	}
	return value, nil
}

func validateOriginalFilename(filename string) (string, error) {
	// path.Base 是为生成对象 key 服务的派生，不是对入参的规范化清洗。
	base := path.Base(strings.ReplaceAll(filename, "\\", "/"))
	if filename == "" || base == "." || base == "/" {
		return "", apperror.InvalidArgument("filename is required")
	}
	if len([]rune(base)) > 128 {
		return "", apperror.InvalidArgument("filename must be 128 characters or fewer")
	}
	return base, nil
}

type uploadIntentInput struct {
	purpose     string
	filename    string
	contentType string
	sha256      string
}

func validateUploadIntent(in *mediapb.CreateUploadIntentRequest) (uploadIntentInput, error) {
	purpose := in.GetPurpose()
	if _, ok := purposeToDB(purpose); !ok {
		return uploadIntentInput{}, apperror.InvalidArgument("purpose must be avatar, message_image, message_file, or agent_skill")
	}
	if purpose == purposeAgentSkill {
		return uploadIntentInput{}, apperror.InvalidArgument("agent_skill media uploads are not supported by this endpoint")
	}
	filename, err := validateOriginalFilename(in.GetFilename())
	if err != nil {
		return uploadIntentInput{}, err
	}
	contentType := in.GetContentType()
	if contentType == "" {
		return uploadIntentInput{}, apperror.InvalidArgument("contentType is required")
	}
	if in.GetSizeBytes() <= 0 {
		return uploadIntentInput{}, apperror.InvalidArgument("sizeBytes must be positive")
	}
	sha := in.GetSha256()
	if sha != "" && !sha256HexPattern.MatchString(sha) {
		return uploadIntentInput{}, apperror.InvalidArgument("sha256 must be lowercase hex with 64 characters")
	}
	if err := validatePurposeContent(purpose, contentType, in.GetSizeBytes()); err != nil {
		return uploadIntentInput{}, err
	}
	if err := validateImageDimensions(purpose, in.GetWidth(), in.GetHeight()); err != nil {
		return uploadIntentInput{}, err
	}
	return uploadIntentInput{purpose: purpose, filename: filename, contentType: contentType, sha256: sha}, nil
}

func validatePurposeContent(purpose string, contentType string, sizeBytes int64) error {
	switch purpose {
	case purposeAvatar:
		if !isAllowedAvatarContentType(contentType) {
			return apperror.InvalidArgument("avatar contentType must be jpeg, png, or webp")
		}
		if sizeBytes > maxAvatarBytes {
			return apperror.InvalidArgument("avatar sizeBytes must be 5 MiB or less")
		}
	case purposeMessageImage:
		if !isAllowedImageContentType(contentType) {
			return apperror.InvalidArgument("message_image contentType must be an allowed image type")
		}
		if sizeBytes > maxImageBytes {
			return apperror.InvalidArgument("message_image sizeBytes must be 15 MiB or less")
		}
	case purposeMessageFile:
		if !isAllowedFileContentType(contentType) {
			return apperror.InvalidArgument("message_file contentType is not allowed")
		}
		if sizeBytes > maxFileBytes {
			return apperror.InvalidArgument("message_file sizeBytes must be 20 MiB or less")
		}
	default:
		return apperror.InvalidArgument("purpose is invalid")
	}
	return nil
}

func validateImageDimensions(purpose string, width, height int32) error {
	if purpose != purposeAvatar && purpose != purposeMessageImage {
		if width != 0 || height != 0 {
			return apperror.InvalidArgument("width and height are only allowed for image uploads")
		}
		return nil
	}
	if width < 0 || height < 0 {
		return apperror.InvalidArgument("width and height must be positive when provided")
	}
	return nil
}

func validateAvatarMediaObject(media *model.MediaObjects) error {
	if media.Purpose != model.MediaPurposeAvatar {
		return apperror.InvalidArgument("avatar media purpose is invalid")
	}
	if media.Status != model.MediaStatusReady {
		return apperror.InvalidArgument("avatar media is not ready")
	}
	if !isAllowedAvatarContentType(media.ContentType) {
		return apperror.InvalidArgument("avatar media content_type must be jpeg, png, or webp")
	}
	if media.SizeBytes > maxAvatarBytes {
		return apperror.InvalidArgument("avatar media exceeds size limit")
	}
	return nil
}

// --- 数据访问 + 跨域下载鉴权 ---

func mediaByID(ctx context.Context, svcCtx *svc.ServiceContext, mediaID string) (*model.MediaObjects, error) {
	media, err := svcCtx.MediaModel.FindOne(ctx, mediaID)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, apperror.NotFound("media object not found")
		}
		return nil, err
	}
	if media.Status == model.MediaStatusDeleted {
		return nil, apperror.NotFound("media object not found")
	}
	return media, nil
}

func mediaForOwner(ctx context.Context, svcCtx *svc.ServiceContext, ownerUserID string, mediaID string) (*model.MediaObjects, error) {
	ownerUserID, err := validateMediaIDComponent(ownerUserID, "owner_user_id")
	if err != nil {
		return nil, err
	}
	mediaID, err = validateMediaIDComponent(mediaID, "media_id")
	if err != nil {
		return nil, err
	}
	media, err := mediaByID(ctx, svcCtx, mediaID)
	if err != nil {
		return nil, err
	}
	if media.OwnerAccountId != ownerUserID {
		return nil, apperror.Forbidden("media object is not owned by requester")
	}
	return media, nil
}

func requesterCanAccessMedia(ctx context.Context, svcCtx *svc.ServiceContext, requesterUserID string, media *model.MediaObjects) (bool, error) {
	if media.OwnerAccountId == requesterUserID {
		return true, nil
	}
	allowed, err := requesterIsAdmin(ctx, svcCtx, requesterUserID)
	if err != nil || allowed {
		return allowed, err
	}
	return requesterCanAccessAttachment(ctx, svcCtx, requesterUserID, media)
}

func requesterIsAdmin(ctx context.Context, svcCtx *svc.ServiceContext, requesterUserID string) (bool, error) {
	if svcCtx.Accounts == nil {
		return false, nil
	}
	user, err := svcCtx.Accounts.GetByID(ctx, requesterUserID)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return false, nil
		}
		return false, err
	}
	return user.AccountType == sharedmodel.AccountTypeAdmin, nil
}

func requesterCanAccessAttachment(ctx context.Context, svcCtx *svc.ServiceContext, requesterUserID string, media *model.MediaObjects) (bool, error) {
	if media.Purpose != model.MediaPurposeMessageImage && media.Purpose != model.MediaPurposeMessageFile {
		return false, nil
	}
	if svcCtx.AttachmentAccess == nil {
		return false, nil
	}
	return svcCtx.AttachmentAccess.UserCanAccessMedia(ctx, requesterUserID, media.MediaId)
}

// --- 对象 key 生成、内容类型白名单、id/metadata 构造、PB 映射 ---

func mediaObjectKey(ownerUserID string, mediaID string, filename string) string {
	return "users/" + ownerUserID + "/media/" + mediaID + "/" + sanitizeObjectFilename(filename)
}

func sanitizeObjectFilename(filename string) string {
	filename = path.Base(strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/"))
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range filename {
		allowed := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '.' || r == '-' || r == '_'
		if allowed {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	sanitized := strings.Trim(builder.String(), "._-")
	if sanitized == "" {
		return "upload"
	}
	if len(sanitized) > 128 {
		sanitized = sanitized[:128]
	}
	return sanitized
}

func normalizeContentType(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isAllowedImageContentType(contentType string) bool {
	switch normalizeContentType(contentType) {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func isAllowedAvatarContentType(contentType string) bool {
	switch normalizeContentType(contentType) {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func isAllowedFileContentType(contentType string) bool {
	switch normalizeContentType(contentType) {
	case "application/pdf",
		"text/plain",
		"application/zip",
		"application/x-zip-compressed",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/msword",
		"application/vnd.ms-excel",
		"application/vnd.ms-powerpoint",
		"application/octet-stream":
		return true
	default:
		return false
	}
}

func newMediaID() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "med_" + hex.EncodeToString(raw[:]), nil
}

// uploadMetadataJSON 把 sha256/width/height 落进 media_objects.metadata（与旧 repository 行为一致；
// 这些字段当前只写不回读）。
func uploadMetadataJSON(sha256 string, width, height int32) (string, error) {
	payload := map[string]any{"sha256": sha256, "width": width, "height": height}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func toPBMediaObject(media *model.MediaObjects) *mediapb.MediaObject {
	// sha256/width/height 存在 metadata 里、当前不回读（与旧实现一致），故留空。
	return &mediapb.MediaObject{
		MediaId:          media.MediaId,
		OwnerUserId:      media.OwnerAccountId,
		Bucket:           media.Bucket,
		ObjectKey:        media.ObjectKey,
		ContentType:      media.ContentType,
		SizeBytes:        media.SizeBytes,
		OriginalFilename: media.OriginalFilename,
		Purpose:          purposeToString(media.Purpose),
		Status:           statusToString(media.Status),
		CreatedAt:        formatTime(media.CreatedAt),
		UpdatedAt:        formatTime(media.UpdatedAt),
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
