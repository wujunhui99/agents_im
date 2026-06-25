package rpcerror

import (
	"errors"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FromStatus 是 ToStatus 的逆向：把 rpc 客户端返回的 gRPC status 映射回 apperror，
// 供 BFF/中间件统一渲染。非 gRPC status（如本地 apperror）原样返回。
func FromStatus(err error) error {
	if err == nil {
		return nil
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

func ToStatus(err error) error {
	if err == nil {
		return nil
	}

	// 裸 error（非 apperror）会被兜底成 Internal，原始真因既不回传也会丢——
	// 先打日志再吞，避免无信息 500（#628；#624 的 model scan 报错即被此吞掉）。
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		logx.Errorf("rpcerror: unhandled error mapped to Internal: %+v", err)
		appErr = apperror.Internal("internal server error")
	}
	switch appErr.Code {
	case apperror.CodeInvalidArgument:
		return status.Error(codes.InvalidArgument, appErr.Message)
	case apperror.CodeUnauthenticated:
		return status.Error(codes.Unauthenticated, appErr.Message)
	case apperror.CodeForbidden:
		return status.Error(codes.PermissionDenied, appErr.Message)
	case apperror.CodeNotFound:
		return status.Error(codes.NotFound, appErr.Message)
	case apperror.CodeAlreadyExists:
		return status.Error(codes.AlreadyExists, appErr.Message)
	case apperror.CodeRateLimited:
		return status.Error(codes.ResourceExhausted, appErr.Message)
	case apperror.CodeServiceUnavailable:
		return status.Error(codes.Unavailable, appErr.Message)
	default:
		return status.Error(codes.Internal, appErr.Message)
	}
}
