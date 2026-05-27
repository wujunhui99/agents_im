package auth

import (
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/service/auth/api/internal/types"
	authpb "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func authResp(result *authpb.AuthResponse) (*types.AuthResp, error) {
	if result == nil {
		return nil, apperror.Internal("auth rpc returned empty auth response")
	}
	return &types.AuthResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.AuthData{
			UserID:        result.GetUserId(),
			Identifier:    result.GetIdentifier(),
			Email:         result.GetEmail(),
			DisplayName:   result.GetDisplayName(),
			Name:          result.GetName(),
			Gender:        result.GetGender(),
			BirthDate:     result.GetBirthDate(),
			Region:        result.GetRegion(),
			AccountType:   result.GetAccountType(),
			AvatarMediaID: result.GetAvatarMediaId(),
			AvatarURL:     result.GetAvatarUrl(),
			Token:         result.GetToken(),
			ExpiresAt:     result.GetExpiresAt(),
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
