package objectstorage

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestS3StorePresignPutCanUseExternalHTTPSWithInternalHTTP(t *testing.T) {
	externalUseSSL := true
	store, err := NewS3Store(Config{
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
		t.Fatalf("new s3 store: %v", err)
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

// TestS3StorePresignPutWithChecksumSignsChecksumHeader 验证内容寻址直传 URL 把
// x-amz-checksum-sha256 烤进签名（SignedHeaders 含该头），客户端必须原样回放、OSS 据此校验。
func TestS3StorePresignPutWithChecksumSignsChecksumHeader(t *testing.T) {
	store, err := NewS3Store(Config{
		Endpoint:        "127.0.0.1:9000",
		Bucket:          "agents-im-media",
		Region:          "us-east-1",
		AccessKeyID:     "unit-test-access-key",
		SecretAccessKey: "unit-test-secret-key",
	})
	if err != nil {
		t.Fatalf("new s3 store: %v", err)
	}
	sha := strings.Repeat("a", 64)
	uploadURL, err := store.PresignPutWithChecksum(context.Background(), "tmp/123/"+sha, "image/jpeg", 42, sha, 5*time.Minute)
	if err != nil {
		t.Fatalf("presign put with checksum: %v", err)
	}
	parsed, err := url.Parse(uploadURL)
	if err != nil {
		t.Fatalf("parse upload URL: %v", err)
	}
	signed := parsed.Query().Get("X-Amz-SignedHeaders")
	if !strings.Contains(signed, "x-amz-checksum-sha256") {
		t.Fatalf("signed headers = %q, want it to include x-amz-checksum-sha256 so OSS enforces the digest", signed)
	}
}

func TestSHA256HexToBase64(t *testing.T) {
	sha := strings.Repeat("a", 64)
	got, err := sha256HexToBase64(sha)
	if err != nil {
		t.Fatalf("sha256HexToBase64: %v", err)
	}
	raw, _ := hex.DecodeString(sha)
	if want := base64.StdEncoding.EncodeToString(raw); got != want {
		t.Fatalf("base64 = %q, want %q", got, want)
	}
	if _, err := sha256HexToBase64("not-hex"); err == nil {
		t.Fatal("invalid hex should error")
	}
	if _, err := sha256HexToBase64(strings.Repeat("a", 10)); err == nil {
		t.Fatal("short digest should error")
	}
}
