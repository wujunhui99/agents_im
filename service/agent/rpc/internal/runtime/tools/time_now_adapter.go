package tools

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
)

// GetCurrentTimeAdapter 实现本地 time.now 工具：返回当前时间的年/月/日、UTC+8（Asia/Shanghai）
// 格式化字符串与 Unix 时间戳。纯本地、无网络、无副作用，供 LLM function call 获取实时时间锚点。
type GetCurrentTimeAdapter struct {
	spec ToolSpec
	// now 便于测试注入；生产用 time.Now。
	now func() time.Time
}

// timeNowOffsetEast8 是 UTC+8 固定偏移（无夏令时），避免依赖运行环境的 tzdata。
var timeNowOffsetEast8 = time.FixedZone("UTC+8", 8*60*60)

// GetCurrentTimeAdapterOption 配置适配器（当前仅测试注入时钟）。
type GetCurrentTimeAdapterOption func(*GetCurrentTimeAdapter)

// WithGetCurrentTimeClock 注入自定义时钟，仅供测试使用。
func WithGetCurrentTimeClock(now func() time.Time) GetCurrentTimeAdapterOption {
	return func(a *GetCurrentTimeAdapter) {
		if now != nil {
			a.now = now
		}
	}
}

// NewGetCurrentTimeAdapter 构造 time.now 适配器；spec 必须是本地 time.now 工具规格。
func NewGetCurrentTimeAdapter(spec ToolSpec, opts ...GetCurrentTimeAdapterOption) (*GetCurrentTimeAdapter, error) {
	if !IsGetCurrentTimeToolSpec(spec) {
		return nil, apperror.InvalidArgument("get current time adapter requires a local time.now tool spec")
	}
	adapter := &GetCurrentTimeAdapter{spec: spec, now: time.Now}
	for _, opt := range opts {
		if opt != nil {
			opt(adapter)
		}
	}
	if adapter.now == nil {
		adapter.now = time.Now
	}
	return adapter, nil
}

// IsGetCurrentTimeToolSpec 判断规格是否为本地 time.now 工具。
func IsGetCurrentTimeToolSpec(spec ToolSpec) bool {
	return spec.ToolType == model.AgentToolTypeLocal &&
		spec.Local != nil &&
		strings.TrimSpace(spec.Local.HandlerKey) == model.LocalToolHandlerGetCurrentTime
}

func (a *GetCurrentTimeAdapter) Spec() ToolSpec {
	if a == nil {
		return ToolSpec{}
	}
	return a.spec
}

type getCurrentTimeOutput struct {
	Year        int    `json:"year"`
	Month       int    `json:"month"`
	Day         int    `json:"day"`
	Timezone    string `json:"timezone"`
	Datetime    string `json:"datetime"`
	Date        string `json:"date"`
	Time        string `json:"time"`
	Weekday     string `json:"weekday"`
	UTCDatetime string `json:"utc_datetime"`
	UnixSeconds int64  `json:"unix_seconds"`
	UnixMillis  int64  `json:"unix_millis"`
	ISO8601     string `json:"iso8601"`
}

// Invoke 忽略输入参数（time.now 无入参），返回当前 UTC+8 时间快照。
func (a *GetCurrentTimeAdapter) Invoke(ctx context.Context, call ToolCall) (ToolResult, error) {
	if a == nil {
		return ToolResult{}, apperror.Internal("get current time adapter is nil")
	}
	if ctx == nil {
		return ToolResult{}, apperror.InvalidArgument("context is required")
	}
	if strings.TrimSpace(call.ToolID) != a.spec.ToolID {
		return ToolResult{}, apperror.InvalidArgument("tool_id does not match get current time adapter")
	}

	now := a.now()
	local := now.In(timeNowOffsetEast8)
	output := getCurrentTimeOutput{
		Year:        local.Year(),
		Month:       int(local.Month()),
		Day:         local.Day(),
		Timezone:    "UTC+8",
		Datetime:    local.Format("2006-01-02 15:04:05"),
		Date:        local.Format("2006-01-02"),
		Time:        local.Format("15:04:05"),
		Weekday:     local.Weekday().String(),
		UTCDatetime: now.UTC().Format("2006-01-02 15:04:05"),
		UnixSeconds: now.Unix(),
		UnixMillis:  now.UnixMilli(),
		ISO8601:     local.Format(time.RFC3339),
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{OutputJSON: encoded}, nil
}
