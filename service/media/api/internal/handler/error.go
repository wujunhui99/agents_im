package handler

import (
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// apiError translates a media-rpc gRPC status error into the shared apperror
// type so the HTTP error handler maps it to the right status code
// (NotFound→404, InvalidArgument→400, …) instead of a blanket 500.
// Non-gRPC errors (binding, ctxuser) are returned unchanged.
func apiError(err error) error {
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
