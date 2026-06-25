package observability

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func runInterceptor(t *testing.T, retErr error) string {
	t.Helper()
	var buf bytes.Buffer
	logx.SetWriter(logx.NewWriter(&buf))
	defer logx.Reset()

	interceptor := ErrorLogUnaryServerInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/auth.v1.Auth/Register"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) { return nil, retErr }
	_, _ = interceptor(context.Background(), nil, info, handler)
	return buf.String()
}

func TestErrorLogInterceptorLogsServerFault(t *testing.T) {
	out := runInterceptor(t, status.Error(codes.Internal, "internal server error"))
	if !strings.Contains(out, "/auth.v1.Auth/Register") || !strings.Contains(out, "failed") {
		t.Fatalf("server-fault not logged with method; log = %q", out)
	}
}

// causeError 模拟 rpcerror.ToStatus 兜底裸 error 的返回：对客户端是 Internal，
// 但 Error() 保留 cause——验证拦截器把真因落盘（可按 trace_id 查）。
type causeError struct{ cause string }

func (e causeError) Error() string { return "internal server error: " + e.cause }
func (e causeError) GRPCStatus() *status.Status {
	return status.New(codes.Internal, "internal server error")
}

func TestErrorLogInterceptorLogsPreservedCause(t *testing.T) {
	out := runInterceptor(t, causeError{cause: "not matching destination to scan"})
	if !strings.Contains(out, "not matching destination to scan") {
		t.Fatalf("preserved cause not surfaced in log; log = %q", out)
	}
}

func TestErrorLogInterceptorSkipsClientFault(t *testing.T) {
	for _, c := range []codes.Code{codes.InvalidArgument, codes.NotFound, codes.AlreadyExists, codes.PermissionDenied} {
		if out := runInterceptor(t, status.Error(c, "biz")); out != "" {
			t.Fatalf("client-fault %v should not log; log = %q", c, out)
		}
	}
}

func TestErrorLogInterceptorSkipsNil(t *testing.T) {
	if out := runInterceptor(t, nil); out != "" {
		t.Fatalf("nil error should not log; log = %q", out)
	}
}
