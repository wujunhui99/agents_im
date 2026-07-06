package config

// Tavily 联网搜索工具（web.search）配置。APIKey 经 secret 注入的裸名 env TAVILY_API_KEY，
// 缺失时 web.search 适配器 fail-closed（工具不装配，默认助手回退为无联网能力），与 DeepSeek
// APIKey 同风格（#664：env 只 wire 裸名，默认值/env 覆盖走 struct tag）。
const DefaultTavilyBaseURL = "https://api.tavily.com"

type TavilyConfig struct {
	APIKey  string `json:",optional,env=TAVILY_API_KEY"`
	BaseURL string `json:",default=https://api.tavily.com,env=TAVILY_BASE_URL"`
	// TimeoutSeconds 单次搜索的 HTTP 超时（含连接+读取），默认 15s。
	TimeoutSeconds int `json:",default=15,env=TAVILY_TIMEOUT_SECONDS"`
	// DefaultMaxResults / MaxResults 分别是未指定 max_results 时的默认条数与上限。
	DefaultMaxResults int `json:",default=5,env=TAVILY_DEFAULT_MAX_RESULTS"`
	MaxResults        int `json:",default=10,env=TAVILY_MAX_RESULTS"`
}
