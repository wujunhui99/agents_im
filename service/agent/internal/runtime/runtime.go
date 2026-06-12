// Package runtime defines the agent run executor surface. The real
// implementation (Eino + LLM provider + tools, today living in
// internal/agentruntime) moves here with 04-agent.md §5; until then only the
// mock driver exists.
package runtime

import (
	"context"
	"encoding/json"
	"fmt"
)

// RunRequest is one agent run derived from a trigger (consumer-internal; the
// Kafka event schema stays untouched per D15).
type RunRequest struct {
	// TriggerEventID is the agent.trigger.v1 event id — the idempotency key for
	// run audit (a replayed event must not re-run).
	TriggerEventID string
	// TriggerKind is trigger.KindAgentInbox / trigger.KindHosting (string to
	// avoid an import cycle; values come from the trigger package).
	TriggerKind        string
	AgentAccountID     string
	ConversationID     string
	ChatType           string
	Seq                int64
	TriggerServerMsgID string
	SenderID           string
	ContentType        string
	Content            json.RawMessage
}

// RunResult is the agent's reply, ready for IM write-back.
type RunResult struct {
	RunID            string
	ReplyContentType string
	ReplyContent     json.RawMessage
}

// Runtime executes one agent run (LLM + tools).
type Runtime interface {
	Run(ctx context.Context, req RunRequest) (RunResult, error)
}

// Mock is the scaffold Runtime (issue #503): no LLM, deterministic canned
// reply echoing the trigger context. Selected only by the explicit
// Runtime.Driver=mock config.
type Mock struct{}

func NewMock() *Mock { return &Mock{} }

func (m *Mock) Run(_ context.Context, req RunRequest) (RunResult, error) {
	if req.AgentAccountID == "" {
		return RunResult{}, fmt.Errorf("mock runtime requires agent_account_id")
	}
	reply, err := json.Marshal(map[string]string{
		"text": fmt.Sprintf("[mock agent %s] reply to %s (kind=%s, seq=%d)",
			req.AgentAccountID, req.TriggerServerMsgID, req.TriggerKind, req.Seq),
	})
	if err != nil {
		return RunResult{}, err
	}
	return RunResult{
		RunID:            "mockrun-" + req.TriggerEventID + "-" + req.AgentAccountID,
		ReplyContentType: "text",
		ReplyContent:     reply,
	}, nil
}
