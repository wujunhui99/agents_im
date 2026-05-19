package repository

import (
	"context"
	"testing"
	"time"
)

func TestMemoryAgentTriggerIdempotencyReclaimsStaleRunning(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryAgentConversationHostingRepository()
	now := time.Date(2026, 5, 19, 8, 0, 0, 0, time.UTC)
	repo.now = func() time.Time {
		return now
	}

	input := testAgentTriggerStartInput("trigger_stale")
	input.RunningTTL = time.Minute
	started, err := repo.TryStartAgentTrigger(ctx, input)
	if err != nil {
		t.Fatalf("start trigger: %v", err)
	}
	if !started {
		t.Fatal("first start = false, want true")
	}

	retry := input
	retry.TriggerEventID = "evt_stale_retry_fresh"
	started, err = repo.TryStartAgentTrigger(ctx, retry)
	if err != nil {
		t.Fatalf("fresh retry trigger: %v", err)
	}
	if started {
		t.Fatal("fresh running retry = true, want false")
	}

	now = now.Add(time.Minute + time.Nanosecond)
	retry.TriggerEventID = "evt_stale_retry_reclaimed"
	started, err = repo.TryStartAgentTrigger(ctx, retry)
	if err != nil {
		t.Fatalf("stale retry trigger: %v", err)
	}
	if !started {
		t.Fatal("stale running retry = false, want true")
	}

	repo.mu.RLock()
	trigger := repo.triggers[input.IdempotencyKey]
	repo.mu.RUnlock()
	if trigger.status != AgentTriggerStatusRunning {
		t.Fatalf("status after reclaim = %q, want running", trigger.status)
	}
	if trigger.input.TriggerEventID != "evt_stale_retry_reclaimed" {
		t.Fatalf("trigger_event_id after reclaim = %q", trigger.input.TriggerEventID)
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
	repo := NewMemoryAgentConversationHostingRepository()
	now := time.Date(2026, 5, 19, 8, 0, 0, 0, time.UTC)
	repo.now = func() time.Time {
		return now
	}

	failedInput := testAgentTriggerStartInput("trigger_failed")
	failedInput.RunningTTL = time.Hour
	started, err := repo.TryStartAgentTrigger(ctx, failedInput)
	if err != nil {
		t.Fatalf("start failed trigger: %v", err)
	}
	if !started {
		t.Fatal("first failed-key start = false, want true")
	}
	if err := repo.FinishAgentTrigger(ctx, AgentTriggerFinishInput{
		IdempotencyKey: failedInput.IdempotencyKey,
		Status:         AgentTriggerStatusFailed,
		ErrorMessage:   "runtime failed",
	}); err != nil {
		t.Fatalf("finish failed trigger: %v", err)
	}
	retryFailed := failedInput
	retryFailed.TriggerEventID = "evt_failed_retry"
	started, err = repo.TryStartAgentTrigger(ctx, retryFailed)
	if err != nil {
		t.Fatalf("retry failed trigger: %v", err)
	}
	if !started {
		t.Fatal("failed retry = false, want true")
	}

	succeededInput := testAgentTriggerStartInput("trigger_succeeded")
	succeededInput.RunningTTL = time.Millisecond
	started, err = repo.TryStartAgentTrigger(ctx, succeededInput)
	if err != nil {
		t.Fatalf("start succeeded trigger: %v", err)
	}
	if !started {
		t.Fatal("first succeeded-key start = false, want true")
	}
	if err := repo.FinishAgentTrigger(ctx, AgentTriggerFinishInput{
		IdempotencyKey:      succeededInput.IdempotencyKey,
		Status:              AgentTriggerStatusSucceeded,
		ResponseServerMsgID: "msg_ai_response",
	}); err != nil {
		t.Fatalf("finish succeeded trigger: %v", err)
	}

	now = now.Add(time.Hour)
	started, err = repo.TryStartAgentTrigger(ctx, succeededInput)
	if err != nil {
		t.Fatalf("retry succeeded trigger: %v", err)
	}
	if started {
		t.Fatal("succeeded retry = true, want false")
	}
	repo.mu.RLock()
	trigger := repo.triggers[succeededInput.IdempotencyKey]
	repo.mu.RUnlock()
	if trigger.status != AgentTriggerStatusSucceeded || trigger.responseServerMsgID != "msg_ai_response" {
		t.Fatalf("succeeded trigger was overwritten: %+v", trigger)
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
