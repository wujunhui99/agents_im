package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/model"
)

func validGetCurrentTimeToolSpec() ToolSpec {
	return ToolSpec{
		ToolID:           "tool_time_now",
		Name:             model.LocalToolHandlerGetCurrentTime,
		ToolType:         model.AgentToolTypeLocal,
		InputSchemaJSON:  `{"type":"object"}`,
		OutputSchemaJSON: `{"type":"object"}`,
		PermissionLevel:  "agent_bound",
		Local:            &LocalToolSpec{HandlerKey: model.LocalToolHandlerGetCurrentTime},
	}
}

func TestGetCurrentTimeAdapterReturnsUTC8Snapshot(t *testing.T) {
	// 2026-07-05 03:04:05 UTC == 2026-07-05 11:04:05 UTC+8.
	fixed := time.Date(2026, time.July, 5, 3, 4, 5, 0, time.UTC)
	spec := validGetCurrentTimeToolSpec()
	adapter, err := NewGetCurrentTimeAdapter(spec, WithGetCurrentTimeClock(func() time.Time { return fixed }))
	if err != nil {
		t.Fatal(err)
	}

	result, err := adapter.Invoke(context.Background(), ToolCall{ToolID: spec.ToolID, ToolName: spec.Name})
	if err != nil {
		t.Fatal(err)
	}

	var out struct {
		Year        int    `json:"year"`
		Month       int    `json:"month"`
		Day         int    `json:"day"`
		Timezone    string `json:"timezone"`
		Datetime    string `json:"datetime"`
		Date        string `json:"date"`
		Time        string `json:"time"`
		UTCDatetime string `json:"utc_datetime"`
		UnixSeconds int64  `json:"unix_seconds"`
		UnixMillis  int64  `json:"unix_millis"`
	}
	if err := json.Unmarshal(result.OutputJSON, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.Year != 2026 || out.Month != 7 || out.Day != 5 {
		t.Fatalf("date fields = %d-%d-%d, want 2026-7-5", out.Year, out.Month, out.Day)
	}
	if out.Timezone != "UTC+8" {
		t.Fatalf("timezone = %q, want UTC+8", out.Timezone)
	}
	if out.Datetime != "2026-07-05 11:04:05" {
		t.Fatalf("datetime = %q, want 2026-07-05 11:04:05", out.Datetime)
	}
	if out.Date != "2026-07-05" || out.Time != "11:04:05" {
		t.Fatalf("date/time = %q %q", out.Date, out.Time)
	}
	if out.UTCDatetime != "2026-07-05 03:04:05" {
		t.Fatalf("utc_datetime = %q, want 2026-07-05 03:04:05", out.UTCDatetime)
	}
	if out.UnixSeconds != fixed.Unix() {
		t.Fatalf("unix_seconds = %d, want %d", out.UnixSeconds, fixed.Unix())
	}
	if out.UnixMillis != fixed.UnixMilli() {
		t.Fatalf("unix_millis = %d, want %d", out.UnixMillis, fixed.UnixMilli())
	}
}

func TestGetCurrentTimeAdapterRejectsWrongToolID(t *testing.T) {
	spec := validGetCurrentTimeToolSpec()
	adapter, err := NewGetCurrentTimeAdapter(spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Invoke(context.Background(), ToolCall{ToolID: "mismatch"}); err == nil {
		t.Fatal("expected error for mismatched tool_id")
	}
}

func TestNewGetCurrentTimeAdapterRejectsWrongSpec(t *testing.T) {
	if _, err := NewGetCurrentTimeAdapter(validPythonExecuteToolSpec()); err == nil {
		t.Fatal("expected error for non time.now spec")
	}
}
