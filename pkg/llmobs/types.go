package llmobs

import "time"

const (
	BackendNoop     = "noop"
	BackendMemory   = "memory"
	BackendTest     = "test"
	BackendLangfuse = "langfuse"

	EventTypeRun        = "run"
	EventTypeGeneration = "generation"
	EventTypeTool       = "tool"

	StatusStarted   = "started"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"

	RuntimeModeAIHostingAutoReply = "ai_hosting_auto_reply"
)

type Config struct {
	Enabled        bool
	Backend        string
	CaptureOutput  bool
	MaxOutputBytes int
	Langfuse       LangfuseConfig
}

type LangfuseConfig struct {
	Host      string
	PublicKey string
	SecretKey string
}

type Event struct {
	Type                 string
	TraceID              string
	RequestID            string
	AgentRunID           string
	ConversationID       string
	TriggerServerMsgID   string
	ResponseServerMsgID  string
	HostedOwnerAccountID string
	SenderAccountID      string
	AgentAccountID       string
	ModelProvider        string
	ModelName            string
	PromptVersion        string
	PromptHash           string
	RuntimeMode          string
	Status               string
	LatencyMs            int64
	ErrorClass           string
	ErrorMessage         string
	Generation           Generation
	StartedAt            time.Time
	FinishedAt           time.Time
	Metadata             map[string]string
}

type Generation struct {
	BoundedRecentMessageCount int
	TriggerInContext          bool
	FinishReason              string
	PromptTokens              int64
	CompletionTokens          int64
	ReasoningTokens           int64
	CachedTokens              int64
	TotalTokens               int64
	LatencyMs                 int64
	FinalOutput               string
	FinalOutputSummary        string
}

type ObserveResult struct {
	Backend        string
	Exported       bool
	DisabledReason string
}
