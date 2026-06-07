package logic

import (
	"time"

	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"
)

func toUserResponse(ap *model.AccountProfile) *userpb.UserResponse {
	return &userpb.UserResponse{User: toUserEntity(ap)}
}

func toUserEntity(ap *model.AccountProfile) *userpb.UserEntity {
	return &userpb.UserEntity{
		UserId:        ap.AccountID,
		Identifier:    ap.Identifier,
		DisplayName:   ap.DisplayName,
		Name:          ap.Name,
		Gender:        genderFromDB(ap.Gender),
		BirthDate:     ap.BirthDate,
		Region:        ap.Region,
		AccountType:   accountTypeFromDB(ap.AccountType),
		AvatarMediaId: ap.AvatarMediaID,
		Email:         ap.EmailNormalized,
		AvatarUrl:     ap.AvatarURL,
		// transport 的 created_at/updated_at 取 profile 时间戳（与 monolith 行为一致）。
		CreatedAt: formatTime(ap.ProfileCreatedAt),
		UpdatedAt: formatTime(ap.ProfileUpdatedAt),
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
