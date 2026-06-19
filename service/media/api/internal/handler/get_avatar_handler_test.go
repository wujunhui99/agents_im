package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wujunhui99/agents_im/service/media/api/internal/config"
	"github.com/wujunhui99/agents_im/service/media/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
	"google.golang.org/grpc"
)

// stubMediaClient is a no-network mediaclient.Media for handler tests.
type stubMediaClient struct {
	avatarURL string
}

func (s stubMediaClient) CreateUploadIntent(context.Context, *mediaclient.CreateUploadIntentRequest, ...grpc.CallOption) (*mediaclient.CreateUploadIntentResponse, error) {
	return &mediaclient.CreateUploadIntentResponse{}, nil
}

func (s stubMediaClient) CompleteUpload(context.Context, *mediaclient.CompleteUploadRequest, ...grpc.CallOption) (*mediaclient.CompleteUploadResponse, error) {
	return &mediaclient.CompleteUploadResponse{}, nil
}

func (s stubMediaClient) GetMedia(context.Context, *mediaclient.GetMediaRequest, ...grpc.CallOption) (*mediaclient.MediaObject, error) {
	return &mediaclient.MediaObject{}, nil
}

func (s stubMediaClient) GetDownloadURL(context.Context, *mediaclient.GetDownloadURLRequest, ...grpc.CallOption) (*mediaclient.GetDownloadURLResponse, error) {
	return &mediaclient.GetDownloadURLResponse{}, nil
}

func (s stubMediaClient) GetAvatarDisplayURL(_ context.Context, in *mediaclient.GetAvatarDisplayURLRequest, _ ...grpc.CallOption) (*mediaclient.GetDownloadURLResponse, error) {
	return &mediaclient.GetDownloadURLResponse{MediaId: in.GetMediaId(), DownloadUrl: s.avatarURL}, nil
}

func (s stubMediaClient) ValidateAvatarMedia(context.Context, *mediaclient.ValidateAvatarMediaRequest, ...grpc.CallOption) (*mediaclient.ValidateMediaResponse, error) {
	return &mediaclient.ValidateMediaResponse{}, nil
}

func (s stubMediaClient) ValidateMessageMedia(context.Context, *mediaclient.ValidateMessageMediaRequest, ...grpc.CallOption) (*mediaclient.ValidateMediaResponse, error) {
	return &mediaclient.ValidateMediaResponse{}, nil
}

func TestGetAvatarHandlerRedirects(t *testing.T) {
	const wantURL = "https://agenticim.xyz/agents-im-media/avatar/x/med_y.jpg?sig=abc"
	svcCtx := &svc.ServiceContext{
		Config: config.Config{Auth: struct {
			AccessSecret string
			AccessExpire int64
		}{AccessSecret: "test-secret"}},
		MediaRPC: stubMediaClient{avatarURL: wantURL},
	}

	server := rest.MustNewServer(rest.RestConf{
		ServiceConf: service.ServiceConf{Name: "media-route-test"},
		Host:        "127.0.0.1",
		Port:        8899,
	})
	t.Cleanup(server.Stop)
	RegisterHandlers(server, svcCtx)
	serverless, err := rest.NewServerless(server)
	if err != nil {
		t.Fatalf("build media route test router: %v", err)
	}
	router := http.HandlerFunc(serverless.Serve)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/media/avatars/med_y", nil)
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusTemporaryRedirect {
		t.Fatalf("avatar status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Location"); got != wantURL {
		t.Fatalf("redirect location = %q, want %q", got, wantURL)
	}
}
