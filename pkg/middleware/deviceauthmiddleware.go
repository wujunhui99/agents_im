// Package middleware holds reusable go-zero rest middlewares (and their shared
// session store) used across services.
package middleware

import (
	"net/http"

	"github.com/wujunhui99/agents_im/pkg/apperror"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// Claim keys the middleware reads from the request context. These are NON-registered
// JWT claims on purpose: go-zero's jwt middleware (rest/handler/authhandler.go) strips
// the registered claims (sub/jti/iat/exp/...) before populating the context, so the
// token mirrors the values it needs under these custom keys.
const (
	claimUserID    = "user_id"
	claimSessionID = "session_id"
	claimDevice    = "device_type"
)

// DeviceAuthMiddleware rejects requests whose token jti is no longer the active
// session for its (user, device) pair. It runs AFTER go-zero's jwt middleware,
// reading the user_id/session_id/device_type claims it injected into the context.
type DeviceAuthMiddleware struct {
	store SessionStore
}

func NewDeviceAuthMiddleware(store SessionStore) *DeviceAuthMiddleware {
	return &DeviceAuthMiddleware{store: store}
}

func (m *DeviceAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID, _ := ctx.Value(claimUserID).(string)
		jti, _ := ctx.Value(claimSessionID).(string)
		device, _ := ctx.Value(claimDevice).(string)
		if userID == "" || jti == "" {
			httpx.ErrorCtx(ctx, w, apperror.Unauthenticated("token session is not active"))
			return
		}
		if err := m.store.Validate(ctx, userID, device, jti); err != nil {
			httpx.ErrorCtx(ctx, w, err)
			return
		}
		next(w, r)
	}
}
