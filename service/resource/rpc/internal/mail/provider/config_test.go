package mailprovider

import "testing"

func TestTencentSESConfigValidateFailsClosedOnMissingCredentials(t *testing.T) {
	cfg := TencentSESConfig{
		Region:            "ap-hongkong",
		Endpoint:          "https://ses.tencentcloudapi.com",
		FromEmailAddress:  "noreply@agenticim.xyz",
		DefaultTemplateID: "177952",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected missing credentials to fail validation")
	}
	if got := err.Error(); got == "" || got == "ok" {
		t.Fatalf("expected explicit validation error, got %q", got)
	}
}
