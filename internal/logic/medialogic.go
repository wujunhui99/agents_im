package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/objectstorage"
	"github.com/wujunhui99/agents_im/internal/repository"
)

const (
	MediaPurposeAvatar       = string(model.MediaPurposeAvatar)
	MediaPurposeMessageImage = string(model.MediaPurposeMessageImage)
	MediaPurposeMessageFile  = string(model.MediaPurposeMessageFile)

	MediaMaxAvatarBytes = 5 * 1024 * 1024
	MediaMaxImageBytes  = 15 * 1024 * 1024
	MediaMaxFileBytes   = 20 * 1024 * 1024

	MediaUploadURLTTL   = 15 * time.Minute
	MediaDownloadURLTTL = 10 * time.Minute
)

var sha256HexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type MediaLogic struct {
	repo   repository.MediaRepository
	store  objectstorage.ObjectStore
	bucket string
	now    func() time.Time
	newID  func() (string, error)
}

type CreateMediaUploadIntentRequest struct {
	OwnerUserID string `json:"ownerUserId"`
	Purpose     string `json:"purpose"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	SHA256      string `json:"sha256"`
	Width       int32  `json:"width"`
	Height      int32  `json:"height"`
}

type CreateMediaUploadIntentResponse struct {
	MediaID   string `json:"mediaId"`
	ObjectKey string `json:"objectKey"`
	UploadURL string `json:"uploadUrl"`
	ExpiresAt int64  `json:"expiresAt"`
}

type CompleteMediaUploadRequest struct {
	OwnerUserID string `json:"ownerUserId"`
	MediaID     string `json:"mediaId"`
}

type CompleteMediaUploadResponse struct {
	Media MediaObject `json:"media"`
}

type GetMediaDownloadURLRequest struct {
	OwnerUserID string `json:"ownerUserId"`
	MediaID     string `json:"mediaId"`
}

type GetMediaDownloadURLResponse struct {
	MediaID     string `json:"mediaId"`
	DownloadURL string `json:"downloadUrl"`
	ExpiresAt   int64  `json:"expiresAt"`
}

type MediaObject struct {
	MediaID          string `json:"mediaId"`
	OwnerUserID      string `json:"ownerUserId"`
	Bucket           string `json:"bucket"`
	ObjectKey        string `json:"objectKey"`
	SHA256           string `json:"sha256"`
	ContentType      string `json:"contentType"`
	SizeBytes        int64  `json:"sizeBytes"`
	Width            int32  `json:"width,omitempty"`
	Height           int32  `json:"height,omitempty"`
	OriginalFilename string `json:"originalFilename"`
	Purpose          string `json:"purpose"`
	Status           string `json:"status"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

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

func NewMediaLogic(repo repository.MediaRepository, store objectstorage.ObjectStore, bucket string) *MediaLogic {
	return &MediaLogic{
		repo:   repo,
		store:  store,
		bucket: strings.TrimSpace(bucket),
		now:    time.Now,
		newID:  newMediaID,
	}
}

func (l *MediaLogic) CreateUploadIntent(ctx context.Context, req CreateMediaUploadIntentRequest) (CreateMediaUploadIntentResponse, error) {
	if l.repo == nil {
		return CreateMediaUploadIntentResponse{}, apperror.Internal("media repository is not configured")
	}
	if l.store == nil {
		return CreateMediaUploadIntentResponse{}, apperror.Internal("object store is not configured")
	}
	ownerUserID, err := normalizeMediaIDComponent(req.OwnerUserID, "owner_user_id")
	if err != nil {
		return CreateMediaUploadIntentResponse{}, err
	}
	purpose, filename, contentType, sha, err := normalizeUploadIntent(req)
	if err != nil {
		return CreateMediaUploadIntentResponse{}, err
	}
	if err := validatePurposeContent(purpose, contentType, req.SizeBytes); err != nil {
		return CreateMediaUploadIntentResponse{}, err
	}
	if err := validateImageDimensions(purpose, req.Width, req.Height); err != nil {
		return CreateMediaUploadIntentResponse{}, err
	}

	mediaID, err := l.newID()
	if err != nil {
		return CreateMediaUploadIntentResponse{}, err
	}
	bucket := strings.TrimSpace(l.bucket)
	if bucket == "" {
		return CreateMediaUploadIntentResponse{}, apperror.Internal("object storage bucket is not configured")
	}
	objectKey := mediaObjectKey(ownerUserID, mediaID, filename)
	expiresAt := l.now().UTC().Add(MediaUploadURLTTL)
	uploadURL, err := l.store.PresignPut(ctx, objectKey, contentType, req.SizeBytes, MediaUploadURLTTL)
	if err != nil {
		return CreateMediaUploadIntentResponse{}, err
	}

	media, err := l.repo.CreateMediaObject(ctx, model.MediaObject{
		MediaID:          mediaID,
		OwnerUserID:      ownerUserID,
		Bucket:           bucket,
		ObjectKey:        objectKey,
		SHA256:           sha,
		ContentType:      contentType,
		SizeBytes:        req.SizeBytes,
		Width:            req.Width,
		Height:           req.Height,
		OriginalFilename: filename,
		Purpose:          purpose,
		Status:           model.MediaStatusPending,
	})
	if err != nil {
		return CreateMediaUploadIntentResponse{}, err
	}
	return CreateMediaUploadIntentResponse{
		MediaID:   media.MediaID,
		ObjectKey: media.ObjectKey,
		UploadURL: uploadURL,
		ExpiresAt: expiresAt.UnixMilli(),
	}, nil
}

func (l *MediaLogic) CompleteUpload(ctx context.Context, req CompleteMediaUploadRequest) (CompleteMediaUploadResponse, error) {
	media, err := l.mediaForOwner(ctx, req.OwnerUserID, req.MediaID)
	if err != nil {
		return CompleteMediaUploadResponse{}, err
	}
	if media.Status != model.MediaStatusPending {
		return CompleteMediaUploadResponse{}, apperror.InvalidArgument("media object is not pending")
	}
	if l.store == nil {
		return CompleteMediaUploadResponse{}, apperror.Internal("object store is not configured")
	}
	info, err := l.store.StatObject(ctx, media.ObjectKey)
	if err != nil {
		if errors.Is(err, objectstorage.ErrObjectNotFound) {
			return CompleteMediaUploadResponse{}, apperror.NotFound("uploaded object not found")
		}
		return CompleteMediaUploadResponse{}, err
	}
	if info.SizeBytes != media.SizeBytes {
		return CompleteMediaUploadResponse{}, apperror.InvalidArgument("uploaded object size does not match upload intent")
	}
	if normalizeContentType(info.ContentType) != media.ContentType {
		return CompleteMediaUploadResponse{}, apperror.InvalidArgument("uploaded object content_type does not match upload intent")
	}
	updated, err := l.repo.UpdateMediaStatus(ctx, media.MediaID, model.MediaStatusReady)
	if err != nil {
		return CompleteMediaUploadResponse{}, err
	}
	return CompleteMediaUploadResponse{Media: toMediaObject(updated)}, nil
}

func (l *MediaLogic) GetDownloadURL(ctx context.Context, req GetMediaDownloadURLRequest) (GetMediaDownloadURLResponse, error) {
	media, err := l.mediaForOwner(ctx, req.OwnerUserID, req.MediaID)
	if err != nil {
		return GetMediaDownloadURLResponse{}, err
	}
	if media.Status != model.MediaStatusReady {
		return GetMediaDownloadURLResponse{}, apperror.InvalidArgument("media object is not ready")
	}
	if l.store == nil {
		return GetMediaDownloadURLResponse{}, apperror.Internal("object store is not configured")
	}
	expiresAt := l.now().UTC().Add(MediaDownloadURLTTL)
	downloadURL, err := l.store.PresignGet(ctx, media.ObjectKey, MediaDownloadURLTTL)
	if err != nil {
		return GetMediaDownloadURLResponse{}, err
	}
	return GetMediaDownloadURLResponse{MediaID: media.MediaID, DownloadURL: downloadURL, ExpiresAt: expiresAt.UnixMilli()}, nil
}

func (l *MediaLogic) ValidateAvatarMedia(ctx context.Context, ownerUserID string, mediaID string) (MediaObject, error) {
	media, err := l.mediaForOwner(ctx, ownerUserID, mediaID)
	if err != nil {
		return MediaObject{}, err
	}
	if media.Purpose != model.MediaPurposeAvatar {
		return MediaObject{}, apperror.InvalidArgument("avatar media purpose is invalid")
	}
	if media.Status != model.MediaStatusReady {
		return MediaObject{}, apperror.InvalidArgument("avatar media is not ready")
	}
	if !isAllowedImageContentType(media.ContentType) {
		return MediaObject{}, apperror.InvalidArgument("avatar media content_type must be an allowed image type")
	}
	if media.SizeBytes > MediaMaxAvatarBytes {
		return MediaObject{}, apperror.InvalidArgument("avatar media exceeds size limit")
	}
	return toMediaObject(media), nil
}

func (l *MediaLogic) ValidateMessageMedia(ctx context.Context, ownerUserID string, contentType string, content string) error {
	switch normalizeContentType(contentType) {
	case MessageContentTypeImage:
		var body messageImageContent
		if err := json.Unmarshal([]byte(content), &body); err != nil {
			return apperror.InvalidArgument("image content must be a JSON object")
		}
		media, err := l.mediaForOwner(ctx, ownerUserID, body.MediaID)
		if err != nil {
			return err
		}
		if media.Purpose != model.MediaPurposeMessageImage {
			return apperror.InvalidArgument("image media purpose is invalid")
		}
		if media.Status != model.MediaStatusReady {
			return apperror.InvalidArgument("image media is not ready")
		}
		if !isAllowedImageContentType(media.ContentType) {
			return apperror.InvalidArgument("image media content_type must be an allowed image type")
		}
		if media.SizeBytes > MediaMaxImageBytes {
			return apperror.InvalidArgument("image media exceeds size limit")
		}
		return nil
	case MessageContentTypeFile:
		var body messageFileContent
		if err := json.Unmarshal([]byte(content), &body); err != nil {
			return apperror.InvalidArgument("file content must be a JSON object")
		}
		filename, err := normalizeOriginalFilename(body.Filename)
		if err != nil {
			return err
		}
		_ = filename
		body.ContentType = normalizeContentType(body.ContentType)
		if body.SizeBytes <= 0 {
			return apperror.InvalidArgument("file sizeBytes must be positive")
		}
		media, err := l.mediaForOwner(ctx, ownerUserID, body.MediaID)
		if err != nil {
			return err
		}
		if media.Purpose != model.MediaPurposeMessageFile {
			return apperror.InvalidArgument("file media purpose is invalid")
		}
		if media.Status != model.MediaStatusReady {
			return apperror.InvalidArgument("file media is not ready")
		}
		if !isAllowedFileContentType(media.ContentType) {
			return apperror.InvalidArgument("file media content_type is not allowed")
		}
		if body.ContentType != media.ContentType || body.SizeBytes != media.SizeBytes {
			return apperror.InvalidArgument("file content metadata does not match media object")
		}
		if media.SizeBytes > MediaMaxFileBytes {
			return apperror.InvalidArgument("file media exceeds size limit")
		}
		return nil
	default:
		return apperror.InvalidArgument("content_type must be image or file")
	}
}

func (l *MediaLogic) mediaForOwner(ctx context.Context, ownerUserID string, mediaID string) (model.MediaObject, error) {
	if l.repo == nil {
		return model.MediaObject{}, apperror.Internal("media repository is not configured")
	}
	ownerUserID, err := normalizeMediaIDComponent(ownerUserID, "owner_user_id")
	if err != nil {
		return model.MediaObject{}, err
	}
	mediaID, err = normalizeMediaIDComponent(mediaID, "media_id")
	if err != nil {
		return model.MediaObject{}, err
	}
	media, err := l.repo.GetMediaObject(ctx, mediaID)
	if err != nil {
		return model.MediaObject{}, err
	}
	if media.OwnerUserID != ownerUserID {
		return model.MediaObject{}, apperror.Forbidden("media object is not owned by requester")
	}
	if media.Status == model.MediaStatusDeleted {
		return model.MediaObject{}, apperror.NotFound("media object not found")
	}
	return media, nil
}

func normalizeUploadIntent(req CreateMediaUploadIntentRequest) (model.MediaPurpose, string, string, string, error) {
	purpose, ok := model.NormalizeMediaPurpose(req.Purpose)
	if !ok {
		return "", "", "", "", apperror.InvalidArgument("purpose must be avatar, message_image, or message_file")
	}
	if purpose == model.MediaPurposeAgentSkill {
		return "", "", "", "", apperror.InvalidArgument("agent_skill media uploads are not supported by this endpoint")
	}
	filename, err := normalizeOriginalFilename(req.Filename)
	if err != nil {
		return "", "", "", "", err
	}
	contentType := normalizeContentType(req.ContentType)
	if contentType == "" {
		return "", "", "", "", apperror.InvalidArgument("contentType is required")
	}
	if req.SizeBytes <= 0 {
		return "", "", "", "", apperror.InvalidArgument("sizeBytes must be positive")
	}
	sha := strings.TrimSpace(req.SHA256)
	if sha != "" && !sha256HexPattern.MatchString(sha) {
		return "", "", "", "", apperror.InvalidArgument("sha256 must be lowercase hex with 64 characters")
	}
	return purpose, filename, contentType, sha, nil
}

func validatePurposeContent(purpose model.MediaPurpose, contentType string, sizeBytes int64) error {
	switch purpose {
	case model.MediaPurposeAvatar:
		if !isAllowedImageContentType(contentType) {
			return apperror.InvalidArgument("avatar contentType must be an allowed image type")
		}
		if sizeBytes > MediaMaxAvatarBytes {
			return apperror.InvalidArgument("avatar sizeBytes must be 5 MiB or less")
		}
	case model.MediaPurposeMessageImage:
		if !isAllowedImageContentType(contentType) {
			return apperror.InvalidArgument("message_image contentType must be an allowed image type")
		}
		if sizeBytes > MediaMaxImageBytes {
			return apperror.InvalidArgument("message_image sizeBytes must be 15 MiB or less")
		}
	case model.MediaPurposeMessageFile:
		if !isAllowedFileContentType(contentType) {
			return apperror.InvalidArgument("message_file contentType is not allowed")
		}
		if sizeBytes > MediaMaxFileBytes {
			return apperror.InvalidArgument("message_file sizeBytes must be 20 MiB or less")
		}
	default:
		return apperror.InvalidArgument("purpose is invalid")
	}
	return nil
}

func validateImageDimensions(purpose model.MediaPurpose, width, height int32) error {
	if purpose != model.MediaPurposeAvatar && purpose != model.MediaPurposeMessageImage {
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

func normalizeOriginalFilename(filename string) (string, error) {
	filename = strings.TrimSpace(strings.ReplaceAll(filename, "\\", "/"))
	if filename == "" {
		return "", apperror.InvalidArgument("filename is required")
	}
	filename = path.Base(filename)
	if filename == "." || filename == "/" || filename == "" {
		return "", apperror.InvalidArgument("filename is required")
	}
	if len([]rune(filename)) > 128 {
		return "", apperror.InvalidArgument("filename must be 128 characters or fewer")
	}
	return filename, nil
}

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

func normalizeMediaIDComponent(value string, field string) (string, error) {
	value = strings.TrimSpace(value)
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

func newMediaID() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "med_" + hex.EncodeToString(raw[:]), nil
}

func toMediaObject(media model.MediaObject) MediaObject {
	return MediaObject{
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
		Purpose:          string(media.Purpose),
		Status:           string(media.Status),
		CreatedAt:        formatTime(media.CreatedAt),
		UpdatedAt:        formatTime(media.UpdatedAt),
	}
}
