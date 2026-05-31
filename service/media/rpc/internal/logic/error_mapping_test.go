package logic

import (
	"context"
	"testing"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/objectstorage"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/media"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestGetAvatarDisplayURLMissingMediaReturnsNotFoundStatus guards that media-rpc
// emits a typed gRPC NotFound (not an opaque Unknown) so media-api can map it to
// HTTP 404 instead of 500 (#390).
func TestGetAvatarDisplayURLMissingMediaReturnsNotFoundStatus(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		MediaLogic: business.NewMediaLogic(repository.NewMemoryMediaRepository(), objectstorage.NewMemoryStore(), "agents-im-media"),
	}

	_, err := NewGetAvatarDisplayURLLogic(context.Background(), svcCtx).
		GetAvatarDisplayURL(&media.GetAvatarDisplayURLRequest{MediaId: "med_missing"})
	if err == nil {
		t.Fatal("expected error for missing media id")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error is not a gRPC status: %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Fatalf("status code = %s, want NotFound; err = %v", st.Code(), err)
	}
}
