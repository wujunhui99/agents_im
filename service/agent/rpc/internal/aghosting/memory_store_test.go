package aghosting

import (
	"context"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

func TestMemoryAgentTriggerIdempotencyReclaimsStaleRunning(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	now := time.Date(2026, 5, 19, 8, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	input := testAgentTriggerStartInput("trigger_stale")
	input.RunningTTL = time.Minute
	started, err := store.TryStartAgentTrigger(ctx, input)
	if err != nil {
		t.Fatalf("start trigger: %v", err)
	}
	if !started {
		t.Fatal("first start = false, want true")
	}

	// Fresh running key → not reclaimed.
	retry := input
	retry.TriggerEventID = "evt_stale_retry_fresh"
	started, err = store.TryStartAgentTrigger(ctx, retry)
	if err != nil {
		t.Fatalf("fresh retry trigger: %v", err)
	}
	if started {
		t.Fatal("fresh running retry = true, want false")
	}

	// After TTL elapses the running key is stale → reclaimed.
	now = now.Add(time.Minute + time.Nanosecond)
	retry.TriggerEventID = "evt_stale_retry_reclaimed"
	started, err = store.TryStartAgentTrigger(ctx, retry)
	if err != nil {
		t.Fatalf("stale retry trigger: %v", err)
	}
	if !started {
		t.Fatal("stale running retry = false, want true")
	}

	store.mu.RLock()
	trigger := store.triggers[input.IdempotencyKey]
	store.mu.RUnlock()
	if trigger.status != AgentTriggerStatusRunning {
		t.Fatalf("status after reclaim = %q, want running", trigger.status)
	}
	if !trigger.createdAt.Equal(time.Date(2026, 5, 19, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("created_at changed after reclaim: %v", trigger.createdAt)
	}
	if !trigger.updatedAt.Equal(now) {
		t.Fatalf("updated_at after reclaim = %v, want %v", trigger.updatedAt, now)
	}
}

func TestMemoryAgentTriggerIdempotencyRetriesFailedButNotSucceeded(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	now := time.Date(2026, 5, 19, 8, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	failedInput := testAgentTriggerStartInput("trigger_failed")
	failedInput.RunningTTL = time.Hour
	if started, err := store.TryStartAgentTrigger(ctx, failedInput); err != nil || !started {
		t.Fatalf("first failed-key start = (%v, %v), want (true, nil)", started, err)
	}
	if err := store.FinishAgentTrigger(ctx, AgentTriggerFinishInput{
		IdempotencyKey: failedInput.IdempotencyKey,
		Status:         AgentTriggerStatusFailed,
		ErrorMessage:   "runtime failed",
	}); err != nil {
		t.Fatalf("finish failed trigger: %v", err)
	}
	retryFailed := failedInput
	retryFailed.TriggerEventID = "evt_failed_retry"
	if started, err := store.TryStartAgentTrigger(ctx, retryFailed); err != nil || !started {
		t.Fatalf("failed retry = (%v, %v), want (true, nil)", started, err)
	}

	succeededInput := testAgentTriggerStartInput("trigger_succeeded")
	succeededInput.RunningTTL = time.Millisecond
	if started, err := store.TryStartAgentTrigger(ctx, succeededInput); err != nil || !started {
		t.Fatalf("first succeeded-key start = (%v, %v), want (true, nil)", started, err)
	}
	if err := store.FinishAgentTrigger(ctx, AgentTriggerFinishInput{
		IdempotencyKey:      succeededInput.IdempotencyKey,
		Status:              AgentTriggerStatusSucceeded,
		ResponseServerMsgID: "msg_ai_response",
	}); err != nil {
		t.Fatalf("finish succeeded trigger: %v", err)
	}

	now = now.Add(time.Hour)
	if started, err := store.TryStartAgentTrigger(ctx, succeededInput); err != nil || started {
		t.Fatalf("succeeded retry = (%v, %v), want (false, nil)", started, err)
	}
	store.mu.RLock()
	trigger := store.triggers[succeededInput.IdempotencyKey]
	store.mu.RUnlock()
	if trigger.status != AgentTriggerStatusSucceeded || trigger.responseServerMsgID != "msg_ai_response" {
		t.Fatalf("succeeded trigger was overwritten: %+v", trigger)
	}
}

func TestMemoryFinishAgentTriggerTerminalSemantics(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	// Unknown key → NotFound.
	if err := store.FinishAgentTrigger(ctx, AgentTriggerFinishInput{
		IdempotencyKey:      "missing",
		Status:              AgentTriggerStatusSucceeded,
		ResponseServerMsgID: "msg",
	}); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("finish missing key error = %v, want not found", err)
	}

	input := testAgentTriggerStartInput("trigger_terminal")
	if _, err := store.TryStartAgentTrigger(ctx, input); err != nil {
		t.Fatalf("start trigger: %v", err)
	}
	if err := store.FinishAgentTrigger(ctx, AgentTriggerFinishInput{
		IdempotencyKey:      input.IdempotencyKey,
		Status:              AgentTriggerStatusSucceeded,
		ResponseServerMsgID: "msg_done",
	}); err != nil {
		t.Fatalf("finish running trigger: %v", err)
	}

	// Re-finishing a terminal trigger is rejected (terminal state is immutable).
	if err := store.FinishAgentTrigger(ctx, AgentTriggerFinishInput{
		IdempotencyKey: input.IdempotencyKey,
		Status:         AgentTriggerStatusFailed,
		ErrorMessage:   "late failure",
	}); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("re-finish terminal trigger error = %v, want not found", err)
	}

	// Invalid status is rejected.
	if err := store.FinishAgentTrigger(ctx, AgentTriggerFinishInput{
		IdempotencyKey: input.IdempotencyKey,
		Status:         "weird",
	}); apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("invalid status error = %v, want invalid argument", err)
	}
}

func TestMemoryUpsertAndGetAgentConversationHosting(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	if _, err := store.GetAgentConversationHosting(ctx, "single:usr_a:usr_b"); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing hosting error = %v, want not found", err)
	}

	saved, err := store.UpsertAgentConversationHosting(ctx, AgentConversationHosting{
		ConversationID:             "single:usr_a:usr_b",
		AgentAccountID:             "usr_agent",
		Enabled:                    true,
		AllowAgentMessageRecursion: true,
	})
	if err != nil {
		t.Fatalf("upsert hosting: %v", err)
	}
	if !saved.Enabled || saved.AgentAccountID != "usr_agent" || !saved.AllowAgentMessageRecursion {
		t.Fatalf("saved hosting mismatch: %+v", saved)
	}
	created := saved.CreatedAt

	// Re-upsert keeps created_at and applies new values.
	updated, err := store.UpsertAgentConversationHosting(ctx, AgentConversationHosting{
		ConversationID: "single:usr_a:usr_b",
		AgentAccountID: "usr_agent2",
		Enabled:        false,
	})
	if err != nil {
		t.Fatalf("re-upsert hosting: %v", err)
	}
	if !updated.CreatedAt.Equal(created) {
		t.Fatalf("created_at changed on re-upsert: %v != %v", updated.CreatedAt, created)
	}
	if updated.Enabled || updated.AgentAccountID != "usr_agent2" {
		t.Fatalf("updated hosting mismatch: %+v", updated)
	}

	got, err := store.GetAgentConversationHosting(ctx, "single:usr_a:usr_b")
	if err != nil {
		t.Fatalf("get hosting: %v", err)
	}
	if got.AgentAccountID != "usr_agent2" {
		t.Fatalf("get hosting mismatch: %+v", got)
	}

	// Validation: agent_account_id cannot contain ':'.
	if _, err := store.UpsertAgentConversationHosting(ctx, AgentConversationHosting{
		ConversationID: "single:x:y",
		AgentAccountID: "bad:id",
	}); apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("colon agent id error = %v, want invalid argument", err)
	}
}

func testAgentTriggerStartInput(idempotencyKey string) AgentTriggerStartInput {
	return AgentTriggerStartInput{
		IdempotencyKey:     idempotencyKey,
		ConversationID:     "single:usr_a:usr_b",
		AgentAccountID:     "usr_a",
		TriggerServerMsgID: "msg_human_trigger",
		TriggerEventID:     "evt_human_trigger",
	}
}
