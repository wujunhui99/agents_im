package orchestrator

import "testing"

func TestMessageCreatedEventBuildsPrivateAgentTrigger(t *testing.T) {
	event := MessageCreatedEvent{
		EventID:          "evt_direct_1",
		OperationID:      "op_direct_1",
		TraceID:          "trace_direct_1",
		ConversationID:   "single:agent_1:user_1",
		ConversationType: ConversationTypeSingle,
		Message: MessageEnvelope{
			ServerMsgID: "msg_1",
			Seq:         7,
			SenderID:    "user_1",
			SenderType:  SenderTypeUser,
			ReceiverID:  "agent_1",
			ContentType: ContentTypeText,
			Text:        "hello",
		},
		TargetAgentUserIDs: []string{"agent_1"},
	}

	triggers, err := BuildMessageCreatedTriggers(event, TriggerPolicy{})
	if err != nil {
		t.Fatalf("build triggers: %v", err)
	}
	if len(triggers) != 1 {
		t.Fatalf("got %d triggers, want 1: %+v", len(triggers), triggers)
	}

	trigger := triggers[0]
	if trigger.TriggerType != TriggerTypeUserPrivateMessage {
		t.Fatalf("trigger type = %q, want %q", trigger.TriggerType, TriggerTypeUserPrivateMessage)
	}
	if trigger.AgentUserID != "agent_1" {
		t.Fatalf("agent user id = %q", trigger.AgentUserID)
	}
	if trigger.TriggerMessageID != "msg_1" || trigger.TriggerSeq != 7 {
		t.Fatalf("unexpected trigger message: %+v", trigger)
	}
	if trigger.RequestingUserID != "user_1" {
		t.Fatalf("requesting user id = %q", trigger.RequestingUserID)
	}
	if trigger.EventID != "evt_direct_1" || trigger.TraceID != "trace_direct_1" {
		t.Fatalf("trace fields were not preserved: %+v", trigger)
	}
}

func TestMessageCreatedEventBuildsGroupMentionTriggerOnlyForMentionedAgent(t *testing.T) {
	event := MessageCreatedEvent{
		EventID:          "evt_group_1",
		OperationID:      "op_group_1",
		TraceID:          "trace_group_1",
		ConversationID:   "group:grp_1",
		ConversationType: ConversationTypeGroup,
		Message: MessageEnvelope{
			ServerMsgID: "msg_2",
			Seq:         12,
			SenderID:    "user_1",
			SenderType:  SenderTypeUser,
			GroupID:     "grp_1",
			ContentType: ContentTypeText,
			Text:        "@agent_1 summarize",
			AtUserIDs:   []string{"agent_1"},
		},
		TargetAgentUserIDs: []string{"agent_1", "agent_2"},
	}

	triggers, err := BuildMessageCreatedTriggers(event, TriggerPolicy{})
	if err != nil {
		t.Fatalf("build triggers: %v", err)
	}
	if len(triggers) != 1 {
		t.Fatalf("got %d triggers, want 1: %+v", len(triggers), triggers)
	}
	if triggers[0].TriggerType != TriggerTypeGroupMention {
		t.Fatalf("trigger type = %q, want %q", triggers[0].TriggerType, TriggerTypeGroupMention)
	}
	if triggers[0].AgentUserID != "agent_1" {
		t.Fatalf("agent user id = %q", triggers[0].AgentUserID)
	}
}

func TestMessageCreatedEventSuppressesAgentMessagesByDefault(t *testing.T) {
	event := MessageCreatedEvent{
		EventID:          "evt_agent_1",
		OperationID:      "op_agent_1",
		TraceID:          "trace_agent_1",
		ConversationID:   "group:grp_1",
		ConversationType: ConversationTypeGroup,
		Message: MessageEnvelope{
			ServerMsgID: "msg_agent_1",
			Seq:         13,
			SenderID:    "agent_1",
			SenderType:  SenderTypeAgent,
			GroupID:     "grp_1",
			ContentType: ContentTypeText,
			Text:        "@agent_2 follow up",
			AtUserIDs:   []string{"agent_2"},
			AgentMetadata: AgentMessageMetadata{
				AgentRunID:       "run_1",
				TriggerMessageID: "msg_2",
			},
		},
		TargetAgentUserIDs: []string{"agent_2"},
	}

	triggers, err := BuildMessageCreatedTriggers(event, TriggerPolicy{})
	if err != nil {
		t.Fatalf("build triggers: %v", err)
	}
	if len(triggers) != 0 {
		t.Fatalf("got recursive triggers without opt-in: %+v", triggers)
	}
}

func TestMessageCreatedEventAllowsExplicitAgentRecursion(t *testing.T) {
	event := MessageCreatedEvent{
		EventID:          "evt_agent_recursive_1",
		OperationID:      "op_agent_recursive_1",
		TraceID:          "trace_agent_recursive_1",
		ConversationID:   "group:grp_1",
		ConversationType: ConversationTypeGroup,
		Message: MessageEnvelope{
			ServerMsgID: "msg_agent_2",
			Seq:         14,
			SenderID:    "agent_1",
			SenderType:  SenderTypeAgent,
			GroupID:     "grp_1",
			ContentType: ContentTypeText,
			Text:        "@agent_2 follow up",
			AtUserIDs:   []string{"agent_2"},
			AgentMetadata: AgentMessageMetadata{
				AgentRunID:            "run_1",
				TriggerMessageID:      "msg_2",
				AllowRecursiveTrigger: true,
			},
		},
		TargetAgentUserIDs: []string{"agent_2"},
	}

	triggers, err := BuildMessageCreatedTriggers(event, TriggerPolicy{AllowAgentMessageRecursion: true})
	if err != nil {
		t.Fatalf("build triggers: %v", err)
	}
	if len(triggers) != 1 {
		t.Fatalf("got %d triggers, want 1: %+v", len(triggers), triggers)
	}
	if triggers[0].TriggerType != TriggerTypeGroupMention {
		t.Fatalf("trigger type = %q, want %q", triggers[0].TriggerType, TriggerTypeGroupMention)
	}
}

func TestNewAdminManualRunTrigger(t *testing.T) {
	trigger, err := NewAdminManualRunTrigger(AdminManualRunRequest{
		RequestID:        "manual_req_1",
		OperationID:      "op_manual_1",
		TraceID:          "trace_manual_1",
		AdminUserID:      "admin_1",
		AgentUserID:      "agent_1",
		ConversationID:   "single:agent_1:user_1",
		ConversationType: ConversationTypeSingle,
		PromptText:       "rerun this conversation",
	})
	if err != nil {
		t.Fatalf("manual trigger: %v", err)
	}
	if trigger.TriggerType != TriggerTypeAdminManualRun {
		t.Fatalf("trigger type = %q, want %q", trigger.TriggerType, TriggerTypeAdminManualRun)
	}
	if trigger.RequestingUserID != "admin_1" {
		t.Fatalf("requesting user id = %q", trigger.RequestingUserID)
	}
	if trigger.TriggerMessageID != "" {
		t.Fatalf("manual run unexpectedly had trigger message id: %+v", trigger)
	}
}
