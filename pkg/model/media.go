package model

import (
	"strings"
	"time"
)

type MediaPurpose string

const (
	MediaPurposeAvatar       MediaPurpose = "avatar"
	MediaPurposeMessageImage MediaPurpose = "message_image"
	MediaPurposeMessageFile  MediaPurpose = "message_file"
	MediaPurposeAgentSkill   MediaPurpose = "agent_skill"
)

type MediaStatus string

const (
	MediaStatusPending  MediaStatus = "pending"
	MediaStatusReady    MediaStatus = "ready"
	MediaStatusRejected MediaStatus = "rejected"
	MediaStatusDeleted  MediaStatus = "deleted"
)

type MediaObject struct {
	MediaID          string
	OwnerUserID      string
	Bucket           string
	ObjectKey        string
	SHA256           string
	ContentType      string
	SizeBytes        int64
	Width            int32
	Height           int32
	OriginalFilename string
	Purpose          MediaPurpose
	Status           MediaStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (m MediaObject) Clone() MediaObject {
	return m
}

func NormalizeMediaPurpose(value string) (MediaPurpose, bool) {
	purpose := MediaPurpose(strings.ToLower(strings.TrimSpace(value)))
	if purpose.IsValid() {
		return purpose, true
	}
	return "", false
}

func (p MediaPurpose) IsValid() bool {
	switch p {
	case MediaPurposeAvatar, MediaPurposeMessageImage, MediaPurposeMessageFile, MediaPurposeAgentSkill:
		return true
	default:
		return false
	}
}

func NormalizeMediaStatus(value string) (MediaStatus, bool) {
	status := MediaStatus(strings.ToLower(strings.TrimSpace(value)))
	if status.IsValid() {
		return status, true
	}
	return "", false
}

func (s MediaStatus) IsValid() bool {
	switch s {
	case MediaStatusPending, MediaStatusReady, MediaStatusRejected, MediaStatusDeleted:
		return true
	default:
		return false
	}
}
