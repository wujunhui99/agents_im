package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/response"

	"github.com/zeromicro/go-zero/rest/httpx"
)

type testSessionStore struct {
	active map[string]string
}

func newTestSessionStore() *testSessionStore {
	return &testSessionStore{active: map[string]string{}}
}

func (s *testSessionStore) SetActive(_ context.Context, userID, device, jti string, _ time.Duration) error {
	s.active[userID+"\x00"+device] = jti
	return nil
}

func (s *testSessionStore) Validate(_ context.Context, userID, device, jti string) error {
	if s.active[userID+"\x00"+device] != jti {
		return apperror.Unauthenticated("token session is not active")
	}
	return nil
}

func TestDeviceAuthMiddleware(t *testing.T) {
	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)

	store := newTestSessionStore()
	if err := store.SetActive(context.Background(), "usr_1", "web", "jti-active", time.Hour); err != nil {
		t.Fatalf("set active: %v", err)
	}

	handler := NewDeviceAuthMiddleware(store).Handle(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// ctxWithClaims mimics what go-zero's jwt middleware injects (non-registered claims only).
	ctxWithClaims := func(userID, jti, device string) context.Context {
		ctx := context.Background()
		ctx = context.WithValue(ctx, claimUserID, userID)
		ctx = context.WithValue(ctx, claimSessionID, jti)
		ctx = context.WithValue(ctx, claimDevice, device)
		return ctx
	}

	cases := []struct {
		name string
		ctx  context.Context
		want int
	}{
		{"active jti passes", ctxWithClaims("usr_1", "jti-active", "web"), http.StatusOK},
		{"stale jti rejected", ctxWithClaims("usr_1", "jti-stale", "web"), http.StatusUnauthorized},
		{"wrong device rejected", ctxWithClaims("usr_1", "jti-active", "ios"), http.StatusUnauthorized},
		{"missing claims rejected", context.Background(), http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/me", nil).WithContext(tc.ctx)
			rec := httptest.NewRecorder()
			handler(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
