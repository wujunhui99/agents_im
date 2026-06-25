package rpcerror

import (
	"errors"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToStatusRawErrorPreservesCauseHidesFromClient(t *testing.T) {
	raw := errors.New("not matching destination to scan")
	out := ToStatus(raw)

	// 客户端只见通用 Internal 文案（不泄露内部细节）。
	st, _ := status.FromError(out)
	if st.Code() != codes.Internal {
		t.Fatalf("code = %v, want Internal", st.Code())
	}
	if st.Message() != "internal server error" {
		t.Fatalf("client message leaked internals: %q", st.Message())
	}

	// 服务端可经 Unwrap 拿到原始 cause（供拦截器带 trace_id 落盘、按 trace_id 查）。
	if !errors.Is(out, raw) {
		t.Fatalf("cause not preserved for server-side logging: %v", out)
	}
	if got := out.Error(); got != "internal server error: not matching destination to scan" {
		t.Fatalf("server-side Error() = %q, want cause included", got)
	}
}

func TestToStatusAppErrorsMapDirectly(t *testing.T) {
	cases := []struct {
		err  error
		code codes.Code
		msg  string
	}{
		{apperror.InvalidArgument("bad email"), codes.InvalidArgument, "bad email"},
		{apperror.AlreadyExists("email already exists"), codes.AlreadyExists, "email already exists"},
		{apperror.Internal("intentional internal"), codes.Internal, "intentional internal"},
	}
	for _, c := range cases {
		st, _ := status.FromError(ToStatus(c.err))
		if st.Code() != c.code || st.Message() != c.msg {
			t.Fatalf("map %v -> code=%v msg=%q, want code=%v msg=%q", c.err, st.Code(), st.Message(), c.code, c.msg)
		}
	}
}

func TestToStatusNil(t *testing.T) {
	if ToStatus(nil) != nil {
		t.Fatal("ToStatus(nil) should be nil")
	}
}
