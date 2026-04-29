package agentaudit

import (
	"strings"
	"testing"
)

func TestRedactSummaryRemovesSensitiveFields(t *testing.T) {
	summary, err := RedactSummary(Summary{
		"authorization": "Bearer should-not-be-stored",
		"api_token":     "token-value",
		"nested": map[string]any{
			"password": "pass-value",
			"note":     "safe text",
		},
		"output": "token=should-not-leak plain",
	})
	if err != nil {
		t.Fatalf("redact summary: %v", err)
	}

	if summary["authorization"] != RedactedValue {
		t.Fatalf("authorization was not redacted: %+v", summary)
	}
	if summary["api_token"] != RedactedValue {
		t.Fatalf("api_token was not redacted: %+v", summary)
	}
	nested, ok := summary["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested summary type mismatch: %#v", summary["nested"])
	}
	if nested["password"] != RedactedValue || nested["note"] != "safe text" {
		t.Fatalf("nested redaction mismatch: %+v", nested)
	}
	if strings.Contains(summary["output"].(string), "should-not-leak") {
		t.Fatalf("inline token value leaked: %q", summary["output"])
	}
}

func TestSummarizePythonCodeDoesNotStoreRawCode(t *testing.T) {
	code := "API_TOKEN = \"should-not-be-stored\"\nprint(API_TOKEN)\n"

	summary := SummarizePythonCode(code)

	if summary["sha256"] == "" || summary["size_bytes"] == nil {
		t.Fatalf("code summary missing hash or size: %+v", summary)
	}
	encoded := summary.String()
	if strings.Contains(encoded, "should-not-be-stored") || strings.Contains(encoded, "print(API_TOKEN)") {
		t.Fatalf("code summary stored raw code: %s", encoded)
	}
}
