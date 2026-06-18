package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/response"
	"github.com/wujunhui99/agents_im/service/media/api/internal/config"
	"github.com/wujunhui99/agents_im/service/media/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// notFoundMediaClient returns a gRPC NotFound status from GetAvatarDisplayURL,
// mirroring media-rpc behaviour for a missing/non-avatar media id.
type notFoundMediaClient struct{}

func (notFoundMediaClient) CreateUploadIntent(context.Context, *mediaclient.CreateUploadIntentRequest, ...grpc.CallOption) (*mediaclient.CreateUploadIntentResponse, error) {
	return nil, status.Error(codes.NotFound, "media object not found")
}

func (notFoundMediaClient) CompleteUpload(context.Context, *mediaclient.CompleteUploadRequest, ...grpc.CallOption) (*mediaclient.CompleteUploadResponse, error) {
	return nil, status.Error(codes.NotFound, "media object not found")
}

func (notFoundMediaClient) GetDownloadURL(context.Context, *mediaclient.GetDownloadURLRequest, ...grpc.CallOption) (*mediaclient.GetDownloadURLResponse, error) {
	return nil, status.Error(codes.NotFound, "media object not found")
}

func (notFoundMediaClient) GetAvatarDisplayURL(context.Context, *mediaclient.GetAvatarDisplayURLRequest, ...grpc.CallOption) (*mediaclient.GetDownloadURLResponse, error) {
	return nil, status.Error(codes.NotFound, "media object not found")
}

func (notFoundMediaClient) ValidateAvatarMedia(context.Context, *mediaclient.ValidateAvatarMediaRequest, ...grpc.CallOption) (*mediaclient.ValidateMediaResponse, error) {
	return nil, status.Error(codes.NotFound, "media object not found")
}

func (notFoundMediaClient) ValidateMessageMedia(context.Context, *mediaclient.ValidateMessageMediaRequest, ...grpc.CallOption) (*mediaclient.ValidateMediaResponse, error) {
	return nil, status.Error(codes.NotFound, "media object not found")
}

// TestGetAvatarHandlerMapsRPCNotFoundTo404 guards the regression where a gRPC
// NotFound from media-rpc surfaced as HTTP 500 instead of 404 (#390).
func TestGetAvatarHandlerMapsRPCNotFoundTo404(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{Auth: struct {
			AccessSecret string
			AccessExpire int64
		}{AccessSecret: "test-secret"}},
		MediaRPC: notFoundMediaClient{},
	}

	// Production wires this in media.go; httpx error handler is process-global.
	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
	server := rest.MustNewServer(rest.RestConf{
		ServiceConf: service.ServiceConf{Name: "media-error-test"},
		Host:        "127.0.0.1",
		Port:        8898,
	})
	t.Cleanup(server.Stop)
	RegisterHandlers(server, svcCtx)
	serverless, err := rest.NewServerless(server)
	if err != nil {
		t.Fatalf("build media route test router: %v", err)
	}
	router := http.HandlerFunc(serverless.Serve)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/media/avatars/med_missing", nil)
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("avatar not-found status = %d, want 404; body = %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "NOT_FOUND") {
		t.Fatalf("avatar not-found body = %s, want NOT_FOUND code", resp.Body.String())
	}
}
