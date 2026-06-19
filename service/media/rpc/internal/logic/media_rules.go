package logic

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	mediapb "github.com/wujunhui99/agents_im/service/media/rpc/media"
)

// 媒体业务规则：purpose/status 整型(model/vars.go) <-> 字符串契约映射、输入校验（只 validate 不
// normalize，清洗由客户端负责）、对象存储 key 生成、内容类型白名单、下载授权编排。数据层走 svcCtx.MediaModel
// (goctl)；media_id 为雪花 bigint（EPIC #527 §1，wire 十进制字符串）。下载授权（EPIC #527 §4，issue
// #532）经属主 msg/friends/groups rpc 编排（链路校验 + 私聊单向好友 / 群成员校验），已脱
// internal/repository 反向依赖（旧 AttachmentAccess/管理员兜底退役）。

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

// validateMediaIDComponent 校验账号 id 形态的入参（owner_user_id / requester_user_id）。account 引用
// 仍以十进制字符串承载（D16），故按字符串校验，不解析成 int。
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

// parseMediaID 把 wire 上的十进制字符串 media_id 解析成雪花 bigint（EPIC #527 §1 / ADR #529）。
func parseMediaID(value string) (int64, error) {
	if value == "" {
		return 0, apperror.InvalidArgument("media_id is required")
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperror.InvalidArgument("media_id must be a positive integer")
	}
	return id, nil
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
	// sha256 是内容寻址的主键，必传（object_key=agents_im/{sha256}，EPIC #527 §3）。
	sha := in.GetSha256()
	if !sha256HexPattern.MatchString(sha) {
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

func mediaByID(ctx context.Context, svcCtx *svc.ServiceContext, mediaID int64) (*model.MediaObjects, error) {
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

func mediaForOwner(ctx context.Context, svcCtx *svc.ServiceContext, ownerUserID string, mediaID int64) (*model.MediaObjects, error) {
	ownerUserID, err := validateMediaIDComponent(ownerUserID, "owner_user_id")
	if err != nil {
		return nil, err
	}
	media, err := mediaByID(ctx, svcCtx, mediaID)
	if err != nil {
		return nil, err
	}
	if media.UploaderId != ownerUserID {
		return nil, apperror.Forbidden("media object is not owned by requester")
	}
	return media, nil
}

// chat_type 取值（msg-rpc GetMessageRef 返回，对齐 msg model.ChatTypeSingle/Group）。
const (
	chatTypeSingle = "single"
	chatTypeGroup  = "group"
)

// requesterCanAccessMedia 实现下载授权编排（EPIC #527 §4）：
//  1. uploader 本人 → 快速放行；
//  2. 否则经 msg-rpc GetMessageRef(msg_id) 取消息引用，做**链路校验**——消息真正引用的 media_id
//     必须等于入参 media_id（media_id/msg_id 由客户端各自传入，不校验就能用合法 msg_id 配任意
//     别人的 media_id 越权）；
//  3. 私聊 → friends 单向好友校验（只看 requester→peer 这条，对方删 requester 不影响）；
//     群聊 → groups 成员校验；失败拒绝。
//
// 语义均按**当前关系**：requester 删好友/退群后不可再下载历史文件、陌生人会话文件不可下载。
func requesterCanAccessMedia(ctx context.Context, svcCtx *svc.ServiceContext, requesterUserID, msgID string, media *model.MediaObjects) (bool, error) {
	if media.UploaderId == requesterUserID {
		return true, nil
	}

	msgID = strings.TrimSpace(msgID)
	if msgID == "" {
		return false, apperror.InvalidArgument("msg_id is required to download media you did not upload")
	}
	if svcCtx.MessageRef == nil {
		return false, apperror.Internal("message reference resolver is not configured")
	}
	chatType, groupID, peerAccountID, refMediaID, err := svcCtx.MessageRef.GetMessageRef(ctx, msgID, requesterUserID)
	if err != nil {
		return false, err
	}

	// 链路校验：入参 media 必须等于消息真正引用的 media（content ->> 'mediaId'）。
	if refMediaID == "" || refMediaID != formatMediaID(media.MediaId) {
		return false, nil
	}

	switch chatType {
	case chatTypeSingle:
		if peerAccountID == "" {
			return false, nil
		}
		if svcCtx.Friends == nil {
			return false, apperror.Internal("friendship checker is not configured")
		}
		return svcCtx.Friends.IsFriendOneWay(ctx, requesterUserID, peerAccountID)
	case chatTypeGroup:
		if groupID == "" {
			return false, nil
		}
		if svcCtx.Groups == nil {
			return false, apperror.Internal("group membership checker is not configured")
		}
		return svcCtx.Groups.IsMember(ctx, groupID, requesterUserID)
	default:
		return false, nil
	}
}

// --- 对象 key 生成（内容寻址）、checksum 比对、内容类型白名单、id/metadata 构造、PB 映射 ---

const (
	// finalKeyPrefix：确认后落地的内容寻址 key 前缀，object_key=agents_im/{整文件 sha256}。
	finalKeyPrefix = "agents_im/"
	// tmpKeyPrefix：直传 OSS 的暂存前缀，tmp/{upload_id}/{sha256}；确认后 copy 到 finalKey 并删除。
	tmpKeyPrefix = "tmp/"
)

// finalObjectKey 返回内容寻址的最终 key agents_im/{sha256}（同文件 → 同 key → 文件级去重/秒传）。
func finalObjectKey(sha256 string) string {
	return finalKeyPrefix + sha256
}

// tmpObjectKey 返回直传暂存 key tmp/{upload_id}/{sha256}；upload_id 用 media_id（雪花唯一）。
func tmpObjectKey(uploadID int64, sha256 string) string {
	return fmt.Sprintf("%s%d/%s", tmpKeyPrefix, uploadID, sha256)
}

// sha256FromTmpKey 从 tmp/{upload_id}/{sha256} 反解整文件 sha256（confirm 时据此算 finalKey、比对
// OSS checksum）。形态不符返回 false。
func sha256FromTmpKey(objectKey string) (string, bool) {
	if !strings.HasPrefix(objectKey, tmpKeyPrefix) {
		return "", false
	}
	parts := strings.Split(objectKey, "/")
	if len(parts) != 3 {
		return "", false
	}
	sha := parts[2]
	if !sha256HexPattern.MatchString(sha) {
		return "", false
	}
	return sha, true
}

// sha256ChecksumMatches 判定 OSS 返回的 base64 SHA-256 checksum 是否等于期望的 sha256(hex)。
// checksum 为空（OSS 未校验/未返回）一律视为不匹配——media 不回算兜底（EPIC #527 §3 职责划分）。
func sha256ChecksumMatches(checksumBase64, sha256Hex string) bool {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(checksumBase64))
	if err != nil || len(raw) != 32 {
		return false
	}
	return hex.EncodeToString(raw) == sha256Hex
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

// formatMediaID 把雪花 bigint media_id 转成 wire 十进制字符串（ADR #529）。
func formatMediaID(mediaID int64) string {
	return strconv.FormatInt(mediaID, 10)
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
		MediaId:          formatMediaID(media.MediaId),
		OwnerUserId:      media.UploaderId,
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
