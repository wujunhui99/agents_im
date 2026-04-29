package rpcerror

import (
	"github.com/wujunhui99/agents_im/internal/apperror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ToStatus(err error) error {
	if err == nil {
		return nil
	}

	appErr := apperror.From(err)
	switch appErr.Code {
	case apperror.CodeInvalidArgument:
		return status.Error(codes.InvalidArgument, appErr.Message)
	case apperror.CodeUnauthenticated:
		return status.Error(codes.Unauthenticated, appErr.Message)
	case apperror.CodeNotFound:
		return status.Error(codes.NotFound, appErr.Message)
	case apperror.CodeAlreadyExists:
		return status.Error(codes.AlreadyExists, appErr.Message)
	default:
		return status.Error(codes.Internal, appErr.Message)
	}
}
