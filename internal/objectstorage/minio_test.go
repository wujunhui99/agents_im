package objectstorage

import (
	"context"
	"net/url"
	"testing"
	"time"
)

func TestMinIOStorePresignPutCanUseExternalHTTPSWithInternalHTTP(t *testing.T) {
	externalUseSSL := true
	store, err := NewMinIOStore(Config{
		Endpoint:         "127.0.0.1:9000",
		ExternalEndpoint: "storage.example.com",
		Bucket:           "agents-im-media",
		Region:           "us-east-1",
		UseSSL:           false,
		ExternalUseSSL:   &externalUseSSL,
		AccessKeyID:      "unit-test-access-key",
		SecretAccessKey:  "unit-test-secret-key",
	})
	if err != nil {
		t.Fatalf("new minio store: %v", err)
	}

	uploadURL, err := store.PresignPut(context.Background(), "users/usr_media/media/med_1/cat.jpg", "image/jpeg", 42, 5*time.Minute)
	if err != nil {
		t.Fatalf("presign put: %v", err)
	}
	parsed, err := url.Parse(uploadURL)
	if err != nil {
		t.Fatalf("parse upload URL: %v", err)
	}
	if parsed.Scheme != "https" {
		t.Fatalf("presigned upload URL scheme = %q, want https; URL=%s", parsed.Scheme, uploadURL)
	}
	if parsed.Host != "storage.example.com" {
		t.Fatalf("presigned upload URL host = %q, want storage.example.com", parsed.Host)
	}
	if got := parsed.Query().Get("X-Amz-SignedHeaders"); got != "host" {
		t.Fatalf("signed headers = %q, want host only so frontend Content-Type PUT header remains unsigned-compatible", got)
	}
}
