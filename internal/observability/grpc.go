package observability

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type metadataCarrier metadata.MD

func (c metadataCarrier) Get(key string) string {
	values := metadata.MD(c).Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (c metadataCarrier) Set(key string, value string) {
	metadata.MD(c).Set(key, value)
}

func (c metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	return keys
}

func GRPCUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		method := safeGRPCMethod(info)
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			traceContext := ExtractTraceContext(ctx, metadataCarrier(md.Copy()))
			ctx = ContextWithTrace(ctx, traceContext)
		}
		ctx, span := StartSpan(
			ctx,
			"gRPC "+method,
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			oteltrace.WithAttributes(attribute.String("rpc.system", "grpc"), attribute.String("rpc.method", method)),
		)
		resp, err := handler(ctx, req)
		if err != nil {
			RecordSpanError(span, err)
		}
		span.End()
		return resp, err
	}
}

func GRPCUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req interface{}, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		method = strings.TrimSpace(method)
		ctx, span := StartSpan(
			ctx,
			"gRPC "+method,
			oteltrace.WithSpanKind(oteltrace.SpanKindClient),
			oteltrace.WithAttributes(attribute.String("rpc.system", "grpc"), attribute.String("rpc.method", method)),
		)
		md, _ := metadata.FromOutgoingContext(ctx)
		md = md.Copy()
		InjectTraceContext(ctx, metadataCarrier(md))
		ctx = metadata.NewOutgoingContext(ctx, md)
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			RecordSpanError(span, err)
		}
		span.End()
		return err
	}
}

func safeGRPCMethod(info *grpc.UnaryServerInfo) string {
	if info == nil {
		return "unknown"
	}
	if method := strings.TrimSpace(info.FullMethod); method != "" {
		return method
	}
	return "unknown"
}
