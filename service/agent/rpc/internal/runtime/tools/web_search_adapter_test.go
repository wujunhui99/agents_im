package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
)

func validWebSearchToolSpec() ToolSpec {
	return ToolSpec{
		ToolID:           "tool_web_search",
		Name:             model.LocalToolHandlerWebSearch,
		ToolType:         model.AgentToolTypeLocal,
		InputSchemaJSON:  `{"type":"object"}`,
		OutputSchemaJSON: `{"type":"object"}`,
		PermissionLevel:  "agent_bound",
		Local:            &LocalToolSpec{HandlerKey: model.LocalToolHandlerWebSearch},
	}
}

// roundTripFunc 让测试注入自定义 HTTP 响应，无需真实网络。
type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func newFakeHTTPClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func TestWebSearchAdapterForwardsQueryAndParsesResults(t *testing.T) {
	var captured tavilySearchRequest
	var authHeader string
	client := newFakeHTTPClient(func(req *http.Request) (*http.Response, error) {
		authHeader = req.Header.Get("Authorization")
		body, _ := io.ReadAll(req.Body)
		_ = json.Unmarshal(body, &captured)
		resp := `{"query":"eino","answer":"Eino is a Go LLM framework.","results":[{"title":"Eino","url":"https://example.com","content":"docs","score":0.9}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(resp)),
			Header:     make(http.Header),
		}, nil
	})

	spec := validWebSearchToolSpec()
	adapter, err := NewWebSearchAdapter(spec, WebSearchConfig{APIKey: "tvly-test", HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}

	result, err := adapter.Invoke(context.Background(), ToolCall{
		ToolID:    spec.ToolID,
		ToolName:  spec.Name,
		InputJSON: json.RawMessage(`{"query":"eino","max_results":3}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured.Query != "eino" {
		t.Fatalf("forwarded query = %q, want eino", captured.Query)
	}
	if captured.MaxResults != 3 {
		t.Fatalf("forwarded max_results = %d, want 3", captured.MaxResults)
	}
	if !captured.IncludeAnswer {
		t.Fatal("include_answer should be true")
	}
	if authHeader != "Bearer tvly-test" {
		t.Fatalf("authorization header = %q", authHeader)
	}

	var out webSearchOutput
	if err := json.Unmarshal(result.OutputJSON, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.Answer != "Eino is a Go LLM framework." {
		t.Fatalf("answer = %q", out.Answer)
	}
	if len(out.Results) != 1 || out.Results[0].URL != "https://example.com" {
		t.Fatalf("results = %+v", out.Results)
	}
}

func TestWebSearchAdapterClampsMaxResults(t *testing.T) {
	var captured tavilySearchRequest
	client := newFakeHTTPClient(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		_ = json.Unmarshal(body, &captured)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"results":[]}`)), Header: make(http.Header)}, nil
	})
	adapter, err := NewWebSearchAdapter(validWebSearchToolSpec(), WebSearchConfig{APIKey: "tvly-test", MaxResults: 10, HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Invoke(context.Background(), ToolCall{ToolID: "tool_web_search", InputJSON: json.RawMessage(`{"query":"go","max_results":999}`)}); err != nil {
		t.Fatal(err)
	}
	if captured.MaxResults != 10 {
		t.Fatalf("clamped max_results = %d, want 10", captured.MaxResults)
	}
}

func TestWebSearchAdapterRequiresQuery(t *testing.T) {
	adapter, err := NewWebSearchAdapter(validWebSearchToolSpec(), WebSearchConfig{APIKey: "tvly-test"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Invoke(context.Background(), ToolCall{ToolID: "tool_web_search", InputJSON: json.RawMessage(`{"query":"  "}`)})
	if apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

// 没有 APIKey 时适配器仍能构造（避免打断整个 agent 工具解析），但 Invoke 期 fail-closed。
func TestWebSearchAdapterFailsClosedWithoutAPIKey(t *testing.T) {
	adapter, err := NewWebSearchAdapter(validWebSearchToolSpec(), WebSearchConfig{})
	if err != nil {
		t.Fatalf("adapter construction should succeed without API key: %v", err)
	}
	_, err = adapter.Invoke(context.Background(), ToolCall{ToolID: "tool_web_search", InputJSON: json.RawMessage(`{"query":"go"}`)})
	if apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("expected forbidden without API key, got %v", err)
	}
}

func TestWebSearchAdapterMapsUpstreamStatusErrors(t *testing.T) {
	client := newFakeHTTPClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`{"detail":"bad key"}`)), Header: make(http.Header)}, nil
	})
	adapter, err := NewWebSearchAdapter(validWebSearchToolSpec(), WebSearchConfig{APIKey: "tvly-test", HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Invoke(context.Background(), ToolCall{ToolID: "tool_web_search", InputJSON: json.RawMessage(`{"query":"go"}`)})
	if apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("expected forbidden for 401 upstream, got %v", err)
	}
}
