package observability

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorLogUnaryServerInterceptor 统一记录 rpc handler 返回的 server-fault 错误。
//
// go-zero zrpc 服务端默认不记 handler 返回的错误（stat 拦截器只打请求行+耗时，
// recover 只在 panic 时打）——server-fault 出错时服务端会完全沉默，只有调用方
// 的 client 拦截器记一笔通用文案，真因落不了盘（见 #624/#630）。
//
// 仅对 server-fault code（Internal/Unknown/DataLoss）打 error 日志，带 trace_id
// （logx.WithContext 从 ctx 取 trace）与方法名；client-fault（InvalidArgument/
// NotFound/AlreadyExists/...）属正常业务结果，不打以免噪声。
func ErrorLogUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil && isServerFault(status.Code(err)) {
			logx.WithContext(ctx).Errorf("rpc handler %s failed: %v", safeGRPCMethod(info), err)
		}
		return resp, err
	}
}

// isServerFault 区分服务端故障码与正常业务/客户端错误码。
func isServerFault(c codes.Code) bool {
	switch c {
	case codes.Internal, codes.Unknown, codes.DataLoss:
		return true
	default:
		return false
	}
}
