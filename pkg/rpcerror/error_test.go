package rpcerror

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// captureLog 把 logx 输出重定向到 buffer，返回读取函数与还原函数。
func captureLog(t *testing.T) (read func() string, restore func()) {
	t.Helper()
	var buf bytes.Buffer
	logx.SetWriter(logx.NewWriter(&buf))
	return buf.String, func() { logx.Reset() }
}

func TestToStatusLogsRawErrorAndBucketsInternal(t *testing.T) {
	read, restore := captureLog(t)
	defer restore()

	raw := errors.New("not matching destination to scan")
	st, _ := status.FromError(ToStatus(raw))
	if st.Code() != codes.Internal {
		t.Fatalf("code = %v, want Internal", st.Code())
	}
	if st.Message() != "internal server error" {
		t.Fatalf("client message leaked internals: %q", st.Message())
	}
	if !strings.Contains(read(), "not matching destination to scan") {
		t.Fatalf("raw cause not logged; log = %q", read())
	}
}

func TestToStatusDoesNotLogAppErrors(t *testing.T) {
	read, restore := captureLog(t)
	defer restore()

	st, _ := status.FromError(ToStatus(apperror.InvalidArgument("bad email")))
	if st.Code() != codes.InvalidArgument || st.Message() != "bad email" {
		t.Fatalf("mapping wrong: code=%v msg=%q", st.Code(), st.Message())
	}
	// 显式 apperror.Internal 也不应被当作「未处理裸 error」打日志。
	_ = ToStatus(apperror.Internal("intentional internal"))
	if strings.Contains(read(), "unhandled error mapped to Internal") {
		t.Fatalf("apperror should not trigger unhandled-error log; log = %q", read())
	}
}

func TestToStatusNil(t *testing.T) {
	if ToStatus(nil) != nil {
		t.Fatal("ToStatus(nil) should be nil")
	}
}
