package auth

import "context"

type clientIPKey struct{}

func clientIP(ctx context.Context) string {
	if v, ok := ctx.Value(clientIPKey{}).(string); ok {
		return v
	}
	return ""
}
