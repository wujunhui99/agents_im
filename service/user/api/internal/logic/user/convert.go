package user

import (
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/user/api/internal/types"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// rfc3339FromUnixMilli 把 user-rpc 的 UnixMilli 时间戳渲染成对外 REST 契约的 RFC3339(UTC) 串；
// 0 → 空串（与旧 user-rpc formatTime 的零值行为一致，前端契约不变）。
func rfc3339FromUnixMilli(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

func userRespFromRPC(resp *userpb.UserResponse) (*types.UserResp, error) {
	if resp == nil || resp.GetUser() == nil {
		return nil, apperror.Internal("user rpc returned empty user")
	}
	user := resp.GetUser()
	return &types.UserResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.User{
			UserID:        user.GetUserId(),
			Identifier:    user.GetIdentifier(),
			Email:         user.GetEmail(),
			DisplayName:   user.GetDisplayName(),
			Name:          user.GetName(),
			Gender:        user.GetGender(),
			BirthDate:     user.GetBirthDate(),
			Region:        user.GetRegion(),
			AccountType:   user.GetAccountType(),
			AvatarMediaID: user.GetAvatarMediaId(),
			AvatarURL:     user.GetAvatarUrl(),
			CreatedAt:     rfc3339FromUnixMilli(user.GetCreatedAt()),
			UpdatedAt:     rfc3339FromUnixMilli(user.GetUpdatedAt()),
		},
	}, nil
}

func apiError(err error) error {
	if err == nil {
		return nil
	}
	if appErr := apperror.From(err); appErr.Code != apperror.CodeInternal || strings.HasPrefix(err.Error(), string(apperror.CodeInternal)+":") {
		return err
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.InvalidArgument:
		return apperror.InvalidArgument(st.Message())
	case codes.Unauthenticated:
		return apperror.Unauthenticated(st.Message())
	case codes.PermissionDenied:
		return apperror.Forbidden(st.Message())
	case codes.NotFound:
		return apperror.NotFound(st.Message())
	case codes.AlreadyExists:
		return apperror.AlreadyExists(st.Message())
	case codes.ResourceExhausted:
		return apperror.RateLimited(st.Message())
	case codes.Unavailable:
		return apperror.ServiceUnavailable(st.Message())
	default:
		return apperror.Internal("internal server error")
	}
}
