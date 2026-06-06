package auth

import "context"

type clientIPKey struct{}

// WithClientIP stashes the server-derived client IP on the context so login/register
// logic can forward it to auth-rpc as login_ip (never trust a client-supplied value).
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey{}, ip)
}

func clientIP(ctx context.Context) string {
	if v, ok := ctx.Value(clientIPKey{}).(string); ok {
		return v
	}
	return ""
}
