package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	msgpb "github.com/wujunhui99/agents_im_protocol/msg/v1"
)

type fakeMediaURLSigner struct {
	asked []string
	urls  map[string]svc.MediaURL
	err   error
}

func (f *fakeMediaURLSigner) SignDownloadURLs(_ context.Context, mediaIDs []string) (map[string]svc.MediaURL, error) {
	f.asked = append(f.asked, mediaIDs...)
	return f.urls, f.err
}

func TestAttachMediaURLsSignsImageAndFileOnly(t *testing.T) {
	rows := []*model.Messages{
		{ContentType: model.ContentTypeTextValue, Content: `{"text":"hi"}`},
		{ContentType: model.ContentTypeImageValue, Content: `{"mediaId":"med_img_1"}`},
		{ContentType: model.ContentTypeFileValue, Content: `{"media_id":"med_doc_1"}`},
	}
	out := []*msgpb.Message{{}, {}, {}}
	signer := &fakeMediaURLSigner{urls: map[string]svc.MediaURL{
		"med_img_1": {URL: "https://oss/img", ExpiresAt: 1700000000000},
		"med_doc_1": {URL: "https://oss/doc", ExpiresAt: 1700000000000},
	}}

	attachMediaURLs(context.Background(), signer, rows, out)

	if len(signer.asked) != 2 || signer.asked[0] != "med_img_1" || signer.asked[1] != "med_doc_1" {
		t.Fatalf("signed ids = %v, want image+file only", signer.asked)
	}
	if out[0].GetMediaUrl() != "" {
		t.Fatalf("text message must not carry media_url, got %q", out[0].GetMediaUrl())
	}
	if out[1].GetMediaUrl() != "https://oss/img" || out[1].GetMediaUrlExpiresAt() != 1700000000000 {
		t.Fatalf("image media url = %q/%d", out[1].GetMediaUrl(), out[1].GetMediaUrlExpiresAt())
	}
	if out[2].GetMediaUrl() != "https://oss/doc" {
		t.Fatalf("file media url = %q (media_id alias must work)", out[2].GetMediaUrl())
	}
}

func TestAttachMediaURLsKeepsMessagesWhenSigningFails(t *testing.T) {
	rows := []*model.Messages{{ContentType: model.ContentTypeImageValue, Content: `{"mediaId":"med_1"}`}}
	out := []*msgpb.Message{{}}
	attachMediaURLs(context.Background(), &fakeMediaURLSigner{err: context.DeadlineExceeded}, rows, out)
	if out[0].GetMediaUrl() != "" {
		t.Fatalf("signing failure must leave media_url empty, got %q", out[0].GetMediaUrl())
	}
}
