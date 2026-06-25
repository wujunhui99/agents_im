package rpcerror

import (
	"errors"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// internalError 包装「被兜底成 Internal 的裸 error」：对客户端经 GRPCStatus() 只暴露
// 通用文案（不泄露内部细节），但保留原始 cause（Unwrap/Error），让服务端统一错误
// 拦截器能带 trace_id 把真因落盘——真因因此可按 trace_id 查询（#630；#624 教训）。
type internalError struct {
	cause error
}

func (e *internalError) Error() string {
	return "internal server error: " + e.cause.Error()
}

func (e *internalError) Unwrap() error {
	return e.cause
}

func (e *internalError) GRPCStatus() *status.Status {
	return status.New(codes.Internal, "internal server error")
}

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

	// 裸 error（非 apperror）兜底成 Internal：保留 cause（供服务端拦截器带
	// trace_id 落盘、按 trace_id 可查），对客户端只暴露通用文案（#628/#630）。
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		return &internalError{cause: err}
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
