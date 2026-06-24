package trigger

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/messaging"
)

func mintIDs(t *testing.T) (userID, agentID, secondAgentID string) {
	t.Helper()
	gen, err := idgen.NewAccountIDGenerator(1)
	if err != nil {
		t.Fatalf("new account id generator: %v", err)
	}
	userID, err = gen.NextString(idgen.FacetHuman)
	if err != nil {
		t.Fatalf("mint user id: %v", err)
	}
	agentID, err = gen.NextString(idgen.FacetAgent)
	if err != nil {
		t.Fatalf("mint agent id: %v", err)
	}
	secondAgentID, err = gen.NextString(idgen.FacetAgent)
	if err != nil {
		t.Fatalf("mint second agent id: %v", err)
	}
	return userID, agentID, secondAgentID
}

func acceptedEvent(sender, receiver, chatType string, visible []string) messaging.MessageEvent {
	return messaging.MessageEvent{
		EventID:        "evt-1",
		EventType:      messaging.EventTypeMessageAccepted,
		ConversationID: "conv-1",
		ServerMsgID:    "srv-1",
		Seq:            7,
		SenderID:       sender,
		ChatType:       chatType,
		CreatedAt:      1700000000000,
		Payload: messaging.MessageEventPayload{
			ClientMsgID:    "c1",
			ReceiverID:     receiver,
			ContentType:    "text",
			Content:        json.RawMessage(`{"text":"hi"}`),
			MessageOrigin:  messaging.MessageOriginHuman,
			VisibleUserIDs: visible,
		},
	}
}

type testHostingStore struct {
	hosted map[string]string
}

func (s testHostingStore) HostingAgent(_ context.Context, conversationID string) (string, bool, error) {
	agentID, ok := s.hosted[conversationID]
	return agentID, ok, nil
}

func TestEvaluateRecursionGateDropsAIOrigin(t *testing.T) {
	userID, agentID, secondAgentID := mintIDs(t)
	judge, err := NewJudge(testHostingStore{hosted: map[string]string{"conv-1": secondAgentID}})
	if err != nil {
		t.Fatalf("new judge: %v", err)
	}

	event := acceptedEvent(agentID, userID, messaging.ChatTypeSingle, []string{userID, agentID})
	event.Payload.MessageOrigin = messaging.MessageOriginAI

	got, err := judge.Evaluate(context.Background(), event)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ai-origin event must be dropped, got %d triggers", len(got))
	}

	// Explicit allow_recursive_trigger reopens the gate.
	event.Payload.AllowRecursiveTrigger = true
	got, err = judge.Evaluate(context.Background(), event)
	if err != nil {
		t.Fatalf("evaluate recursive: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("allow_recursive_trigger event must pass the gate")
	}
}

func TestEvaluateAgentInboxSingleChat(t *testing.T) {
	userID, agentID, _ := mintIDs(t)
	judge, _ := NewJudge(testHostingStore{})

	got, err := judge.Evaluate(context.Background(),
		acceptedEvent(userID, agentID, messaging.ChatTypeSingle, []string{userID, agentID}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(got))
	}
	if got[0].Kind != KindAgentInbox || got[0].AgentAccountID != agentID {
		t.Fatalf("unexpected trigger %+v", got[0])
	}
}

func TestEvaluateGroupMembersDedupAgents(t *testing.T) {
	userID, agentID, secondAgentID := mintIDs(t)
	judge, _ := NewJudge(testHostingStore{})

	event := acceptedEvent(userID, "", messaging.ChatTypeGroup,
		[]string{userID, agentID, secondAgentID, agentID})
	event.Payload.GroupID = "grp-1"
	event.Payload.ReceiverIDs = []string{userID, agentID, secondAgentID}

	got, err := judge.Evaluate(context.Background(), event)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 deduped agent triggers, got %d: %+v", len(got), got)
	}
	for _, trig := range got {
		if trig.Kind != KindAgentInbox {
			t.Fatalf("unexpected kind %q", trig.Kind)
		}
	}
}

func TestEvaluateHostingTrigger(t *testing.T) {
	userID, agentID, _ := mintIDs(t)
	otherUser := userID + "1" // numeric-string, still user-typed bits are irrelevant here

	judge, _ := NewJudge(testHostingStore{hosted: map[string]string{"conv-1": agentID}})

	// Human → human conversation hosted by an agent that is not a recipient.
	got, err := judge.Evaluate(context.Background(),
		acceptedEvent(userID, otherUser, messaging.ChatTypeSingle, []string{userID, otherUser}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(got) != 1 || got[0].Kind != KindHosting || got[0].AgentAccountID != agentID {
		t.Fatalf("expected hosting trigger for %s, got %+v", agentID, got)
	}
}

func TestEvaluateHostingDedupsAgainstInbox(t *testing.T) {
	userID, agentID, _ := mintIDs(t)
	judge, _ := NewJudge(testHostingStore{hosted: map[string]string{"conv-1": agentID}})

	got, err := judge.Evaluate(context.Background(),
		acceptedEvent(userID, agentID, messaging.ChatTypeSingle, []string{userID, agentID}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("agent hit via inbox AND hosting must dedup to 1 trigger, got %d", len(got))
	}
	if got[0].Kind != KindAgentInbox {
		t.Fatalf("inbox trigger wins the dedup, got %q", got[0].Kind)
	}
}

func TestEvaluateSenderNeverSelfTriggers(t *testing.T) {
	_, agentID, secondAgentID := mintIDs(t)
	judge, _ := NewJudge(testHostingStore{hosted: map[string]string{"conv-1": agentID}})

	// Agent-origin marked recursive: gate passes, but the sending agent itself
	// must not trigger — only the receiving agent does.
	event := acceptedEvent(agentID, secondAgentID, messaging.ChatTypeSingle, []string{agentID, secondAgentID})
	event.Payload.MessageOrigin = messaging.MessageOriginAI
	event.Payload.AllowRecursiveTrigger = true

	got, err := judge.Evaluate(context.Background(), event)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(got) != 1 || got[0].AgentAccountID != secondAgentID {
		t.Fatalf("expected only receiving agent %s, got %+v", secondAgentID, got)
	}
}

func TestEvaluateLegacyIDsDoNotTrigger(t *testing.T) {
	judge, _ := NewJudge(testHostingStore{})
	got, err := judge.Evaluate(context.Background(),
		acceptedEvent("user-a", "agent_creator", messaging.ChatTypeSingle, []string{"user-a", "agent_creator"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("legacy non-numeric ids must not trigger, got %+v", got)
	}
}
