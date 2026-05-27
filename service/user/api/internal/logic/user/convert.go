package user

import (
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/proto/userpb"
	"github.com/wujunhui99/agents_im/service/user/api/internal/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
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
			CreatedAt:     user.GetCreatedAt(),
			UpdatedAt:     user.GetUpdatedAt(),
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
