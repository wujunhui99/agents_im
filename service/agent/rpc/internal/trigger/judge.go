// Package trigger implements the agent-side final judgment over
// agent.trigger.v1 events (00-decisions D15). msgtransfer produces the topic
// unconditionally and branch-free; ALL trigger decisions — recursion gate,
// agent-inbox detection, hosting lookup — live here, on the data owner's side.
package trigger

import (
	"context"
	"fmt"

	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/messaging"
)

// Kind classifies why an agent run is triggered. It is a consumer-internal
// concept by contract (D15): it never appears in the Kafka event schema.
type Kind string

const (
	// KindAgentInbox — a recipient's account id carries the agent type bits (D16).
	KindAgentInbox Kind = "agent_inbox"
	// KindHosting — the conversation is AI-hosted (conversation_ai_hosting).
	KindHosting Kind = "hosting"
)

// Trigger is one agent run to execute for an inbound message event.
type Trigger struct {
	Kind           Kind
	AgentAccountID string
	Event          messaging.MessageEvent
}

// HostingStore answers "is this conversation AI-hosted, and by which agent".
// The data owner is the agent domain (conversation_ai_hosting, D13 goctl model
// once real); the message main chain never performs this lookup (D15).
type HostingStore interface {
	HostingAgent(ctx context.Context, conversationID string) (agentAccountID string, hosted bool, err error)
}

// Judge runs the three-step final judgment (D15, 04-agent §4.2):
//  1. origin=ai without allow_recursive_trigger → drop (recursion gate,
//     zero-query: event fields only);
//  2. recipient / group member id type bits = agent (D16, zero-query) →
//     agent-inbox trigger;
//  3. conversation ∈ hosting store (agent-domain local data) → hosting trigger.
//
// No hit → drop. Steps are additive: one event can yield both an inbox and a
// hosting trigger, deduped per agent account.
type Judge struct {
	hosting HostingStore
}

func NewJudge(hosting HostingStore) (*Judge, error) {
	if hosting == nil {
		return nil, fmt.Errorf("trigger judge requires a hosting store")
	}
	return &Judge{hosting: hosting}, nil
}

// Evaluate returns the agent runs to execute for one accepted message event.
// An empty slice means the event is dropped.
func (j *Judge) Evaluate(ctx context.Context, event messaging.MessageEvent) ([]Trigger, error) {
	// Step 1 — recursion gate: AI-origin messages do not trigger further runs
	// unless explicitly marked. This is what terminates the write-back loop
	// (AI replies travel the same Kafka pipeline as human messages).
	if event.Payload.MessageOrigin == messaging.MessageOriginAI && !event.Payload.AllowRecursiveTrigger {
		return nil, nil
	}

	triggered := make([]Trigger, 0, 2)
	seen := make(map[string]struct{}, 2)

	// Step 2 — agent inbox: any recipient whose account id carries the agent
	// type bits (D16) gets a run. The sender never triggers on its own message.
	for _, id := range recipientCandidates(event) {
		if id == event.SenderID {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		if !idgen.IsAgentAccountID(id) {
			continue
		}
		seen[id] = struct{}{}
		triggered = append(triggered, Trigger{Kind: KindAgentInbox, AgentAccountID: id, Event: event})
	}

	// Step 3 — hosting: the hosting agent runs even when it is not an explicit
	// recipient (e.g. a human-human conversation hosted by an assistant).
	agentID, hosted, err := j.hosting.HostingAgent(ctx, event.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("hosting lookup conversation_id=%q: %w", event.ConversationID, err)
	}
	if hosted && agentID != "" && agentID != event.SenderID {
		if _, dup := seen[agentID]; !dup {
			triggered = append(triggered, Trigger{Kind: KindHosting, AgentAccountID: agentID, Event: event})
		}
	}
	return triggered, nil
}

// recipientCandidates lists every account that received the message: the
// single-chat receiver plus the push/visibility fanout (group members).
func recipientCandidates(event messaging.MessageEvent) []string {
	candidates := make([]string, 0, len(event.Payload.ReceiverIDs)+len(event.Payload.VisibleUserIDs)+1)
	if event.Payload.ReceiverID != "" {
		candidates = append(candidates, event.Payload.ReceiverID)
	}
	candidates = append(candidates, event.Payload.ReceiverIDs...)
	candidates = append(candidates, event.Payload.VisibleUserIDs...)
	return candidates
}
