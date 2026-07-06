package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
)

const (
	defaultTavilyBaseURL       = "https://api.tavily.com"
	defaultWebSearchMaxResults = 5
	maxWebSearchMaxResults     = 10
	defaultWebSearchTimeout    = 15 * time.Second
	maxWebSearchResponseBytes  = 1 << 20 // 1 MiB 上限，防超大响应打爆内存。
)

// WebSearchConfig 是 Tavily 联网搜索适配器的配置。APIKey 缺失时适配器不可用（fail-closed）。
type WebSearchConfig struct {
	APIKey         string
	BaseURL        string
	Timeout        time.Duration
	DefaultResults int
	MaxResults     int
	// HTTPClient 便于测试注入；生产留空用带 Timeout 的默认 client。
	HTTPClient *http.Client
}

func (c WebSearchConfig) normalized() WebSearchConfig {
	c.APIKey = strings.TrimSpace(c.APIKey)
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if c.BaseURL == "" {
		c.BaseURL = defaultTavilyBaseURL
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultWebSearchTimeout
	}
	if c.DefaultResults <= 0 {
		c.DefaultResults = defaultWebSearchMaxResults
	}
	if c.MaxResults <= 0 {
		c.MaxResults = maxWebSearchMaxResults
	}
	if c.DefaultResults > c.MaxResults {
		c.DefaultResults = c.MaxResults
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: c.Timeout}
	}
	return c
}

// WebSearchAdapter 实现本地 web.search 工具：把 LLM 的查询转发给 Tavily Search API，
// 返回结构化搜索结果（含可选摘要 answer）。网络调用经属主 Tavily HTTP 端点，非本地进程。
type WebSearchAdapter struct {
	spec   ToolSpec
	config WebSearchConfig
}

// NewWebSearchAdapter 构造 web.search 适配器；spec 必须是本地 web.search 工具规格。
// APIKey 缺失时不在构造期报错——与 python.execute 一致，适配器总能解析出来，避免因单个工具
// 缺配置导致整个 agent 的工具解析（RequireAdapters）失败；改在 Invoke 时 fail-closed 报 Forbidden
// （可恢复错误，喂回模型让其无联网继续），而非打断整个 run。
func NewWebSearchAdapter(spec ToolSpec, config WebSearchConfig) (*WebSearchAdapter, error) {
	if !IsWebSearchToolSpec(spec) {
		return nil, apperror.InvalidArgument("web search adapter requires a local web.search tool spec")
	}
	return &WebSearchAdapter{spec: spec, config: config.normalized()}, nil
}

// IsWebSearchToolSpec 判断规格是否为本地 web.search 工具。
func IsWebSearchToolSpec(spec ToolSpec) bool {
	return spec.ToolType == model.AgentToolTypeLocal &&
		spec.Local != nil &&
		strings.TrimSpace(spec.Local.HandlerKey) == model.LocalToolHandlerWebSearch
}

func (a *WebSearchAdapter) Spec() ToolSpec {
	if a == nil {
		return ToolSpec{}
	}
	return a.spec
}

type webSearchInput struct {
	Query       string `json:"query"`
	MaxResults  int    `json:"max_results"`
	SearchDepth string `json:"search_depth"`
}

type tavilySearchRequest struct {
	Query         string `json:"query"`
	MaxResults    int    `json:"max_results"`
	SearchDepth   string `json:"search_depth"`
	IncludeAnswer bool   `json:"include_answer"`
}

type tavilySearchResponse struct {
	Query   string               `json:"query"`
	Answer  string               `json:"answer"`
	Results []tavilySearchResult `json:"results"`
}

type tavilySearchResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

type webSearchOutput struct {
	Query   string               `json:"query"`
	Answer  string               `json:"answer,omitempty"`
	Results []tavilySearchResult `json:"results"`
}

// Invoke 解码查询、调用 Tavily Search API 并返回结构化结果。
func (a *WebSearchAdapter) Invoke(ctx context.Context, call ToolCall) (ToolResult, error) {
	if a == nil {
		return ToolResult{}, apperror.Internal("web search adapter is nil")
	}
	if ctx == nil {
		return ToolResult{}, apperror.InvalidArgument("context is required")
	}
	if strings.TrimSpace(call.ToolID) != a.spec.ToolID {
		return ToolResult{}, apperror.InvalidArgument("tool_id does not match web search adapter")
	}
	if strings.TrimSpace(a.config.APIKey) == "" {
		// fail-closed：未配置 TAVILY_API_KEY 时联网搜索不可用；Forbidden 属可恢复错误，
		// 会喂回模型让其在无联网结果的情况下继续，而不打断整个 agent run。
		return ToolResult{}, apperror.Forbidden("web search tool requires a Tavily API key (set TAVILY_API_KEY)")
	}

	input, err := decodeWebSearchInput(call.InputJSON)
	if err != nil {
		return ToolResult{}, err
	}
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return ToolResult{}, apperror.InvalidArgument("query is required")
	}
	maxResults := a.config.DefaultResults
	if input.MaxResults > 0 {
		maxResults = input.MaxResults
	}
	if maxResults > a.config.MaxResults {
		maxResults = a.config.MaxResults
	}
	searchDepth := strings.TrimSpace(input.SearchDepth)
	switch searchDepth {
	case "", "basic":
		searchDepth = "basic"
	case "advanced":
		// keep
	default:
		return ToolResult{}, apperror.InvalidArgument("search_depth must be basic or advanced")
	}

	resp, err := a.search(ctx, tavilySearchRequest{
		Query:         query,
		MaxResults:    maxResults,
		SearchDepth:   searchDepth,
		IncludeAnswer: true,
	})
	if err != nil {
		return ToolResult{}, err
	}
	output := webSearchOutput{
		Query:   query,
		Answer:  strings.TrimSpace(resp.Answer),
		Results: resp.Results,
	}
	if output.Results == nil {
		output.Results = []tavilySearchResult{}
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{OutputJSON: encoded}, nil
}

func (a *WebSearchAdapter) search(ctx context.Context, payload tavilySearchRequest) (tavilySearchResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return tavilySearchResponse{}, err
	}
	endpoint := a.config.BaseURL + "/search"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return tavilySearchResponse{}, apperror.Internal("build tavily request: " + err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.config.APIKey)

	httpResp, err := a.config.HTTPClient.Do(req)
	if err != nil {
		return tavilySearchResponse{}, apperror.ServiceUnavailable("tavily search request failed: " + err.Error())
	}
	defer httpResp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(httpResp.Body, maxWebSearchResponseBytes))
	if err != nil {
		return tavilySearchResponse{}, apperror.ServiceUnavailable("read tavily response: " + err.Error())
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return tavilySearchResponse{}, tavilyStatusError(httpResp.StatusCode, raw)
	}
	var decoded tavilySearchResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return tavilySearchResponse{}, apperror.ServiceUnavailable("decode tavily response: " + err.Error())
	}
	return decoded, nil
}

func tavilyStatusError(status int, raw []byte) error {
	message := strings.TrimSpace(string(raw))
	if len(message) > 512 {
		message = message[:512]
	}
	detail := fmt.Sprintf("tavily search returned status %d: %s", status, message)
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return apperror.Forbidden(detail)
	case status == http.StatusTooManyRequests:
		return apperror.ServiceUnavailable(detail)
	case status >= 400 && status < 500:
		return apperror.InvalidArgument(detail)
	default:
		return apperror.ServiceUnavailable(detail)
	}
}

func decodeWebSearchInput(raw json.RawMessage) (webSearchInput, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return webSearchInput{}, apperror.InvalidArgument("input_json is required")
	}
	var input webSearchInput
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return webSearchInput{}, apperror.InvalidArgument("web search input_json is invalid: " + err.Error())
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return webSearchInput{}, apperror.InvalidArgument("web search input_json must contain a single JSON object")
	}
	return input, nil
}
