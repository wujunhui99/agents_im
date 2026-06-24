package model

import (
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
