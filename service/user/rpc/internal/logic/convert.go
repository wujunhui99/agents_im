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
		CreatedAt: unixMilli(ap.ProfileCreatedAt),
		UpdatedAt: unixMilli(ap.ProfileUpdatedAt),
	}
}

// unixMilli 把时间戳编码成 UnixMilli（UTC）；零值 time → 0（与仓库其它 int64 时间字段一致）。
func unixMilli(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UTC().UnixMilli()
}
