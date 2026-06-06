package ctxuser

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

// UserIDClaim is a custom (non-registered) claim. go-zero's jwt middleware strips
// the registered claims (sub/jti/iat/exp/...) from the request context, so the
// user id is mirrored here under a non-registered key to survive into handlers.
const UserIDClaim = "user_id"

func UserID(ctx context.Context) (string, error) {
	return stringClaim(ctx, UserIDClaim)
}

func stringClaim(ctx context.Context, claim string) (string, error) {
	if ctx == nil {
		return "", apperror.Unauthenticated("authenticated user is required")
	}

	value := ctx.Value(claim)
	var raw string
	switch v := value.(type) {
	case string:
		raw = v
	case json.Number:
		raw = v.String()
	case float64:
		if math.Trunc(v) == v {
			raw = strconv.FormatInt(int64(v), 10)
		}
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", apperror.Unauthenticated("authenticated user is required")
	}
	return raw, nil
}
