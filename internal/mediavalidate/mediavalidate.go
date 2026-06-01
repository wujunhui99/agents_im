// Package mediavalidate holds transitional in-process media validation used by
// callers that still read the shared media store directly: the message monolith
// (attachment validation on the send path) and user-rpc (avatar validation).
//
// The authoritative media logic lives in service/media/core (owned by media-rpc).
// This package is a deliberate, minimal shim so those callers do not depend on
// internal/logic; it is removed once the message monolith (Epic #394 Phase 6)
// and the user/media data layers move behind media-rpc.
package mediavalidate

import (
	"context"
	"encoding/json"
	"path"
	"strings"

	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
)

const (
	maxAvatarBytes = 5 * 1024 * 1024
	maxImageBytes  = 15 * 1024 * 1024
	maxFileBytes   = 20 * 1024 * 1024
)

// MessageValidator validates message attachment references against the media
// store. It implements logic.MessageMediaValidator.
type MessageValidator struct {
	repo repository.MediaRepository
}

func NewMessageValidator(repo repository.MediaRepository) *MessageValidator {
	return &MessageValidator{repo: repo}
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

func (v *MessageValidator) ValidateMessageMedia(ctx context.Context, ownerUserID string, contentType string, content string) error {
	switch normalizeContentType(contentType) {
	case repository.ContentTypeImage:
		var body messageImageContent
		if err := json.Unmarshal([]byte(content), &body); err != nil {
			return apperror.InvalidArgument("image content must be a JSON object")
		}
		media, err := mediaForOwner(ctx, v.repo, ownerUserID, body.MediaID)
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
		if media.SizeBytes > maxImageBytes {
			return apperror.InvalidArgument("image media exceeds size limit")
		}
		return nil
	case repository.ContentTypeFile:
		var body messageFileContent
		if err := json.Unmarshal([]byte(content), &body); err != nil {
			return apperror.InvalidArgument("file content must be a JSON object")
		}
		if _, err := normalizeOriginalFilename(body.Filename); err != nil {
			return err
		}
		body.ContentType = normalizeContentType(body.ContentType)
		if body.SizeBytes <= 0 {
			return apperror.InvalidArgument("file sizeBytes must be positive")
		}
		media, err := mediaForOwner(ctx, v.repo, ownerUserID, body.MediaID)
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
		if media.SizeBytes > maxFileBytes {
			return apperror.InvalidArgument("file media exceeds size limit")
		}
		return nil
	default:
		return apperror.InvalidArgument("content_type must be image or file")
	}
}

// AvatarValidator validates that a media object is a usable avatar for an owner.
type AvatarValidator struct {
	repo repository.MediaRepository
}

func NewAvatarValidator(repo repository.MediaRepository) *AvatarValidator {
	return &AvatarValidator{repo: repo}
}

func (v *AvatarValidator) ValidateAvatarMedia(ctx context.Context, ownerUserID string, mediaID string) error {
	media, err := mediaForOwner(ctx, v.repo, ownerUserID, mediaID)
	if err != nil {
		return err
	}
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

func mediaForOwner(ctx context.Context, repo repository.MediaRepository, ownerUserID string, mediaID string) (model.MediaObject, error) {
	ownerUserID, err := normalizeMediaIDComponent(ownerUserID, "owner_user_id")
	if err != nil {
		return model.MediaObject{}, err
	}
	mediaID, err = normalizeMediaIDComponent(mediaID, "media_id")
	if err != nil {
		return model.MediaObject{}, err
	}
	if repo == nil {
		return model.MediaObject{}, apperror.Internal("media repository is not configured")
	}
	media, err := repo.GetMediaObject(ctx, mediaID)
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
