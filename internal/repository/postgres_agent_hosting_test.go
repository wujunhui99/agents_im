package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestPostgresAgentTriggerTryStartUsesStaleRunningReclaimPredicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := NewPostgresRepositoryFromConn(sqlx.NewSqlConnFromDB(db))
	input := testAgentTriggerStartInput("trigger_pg_stale")
	input.RunningTTL = 2 * time.Minute

	mock.ExpectBegin()
	mock.ExpectExec(`(?s)insert into agent_trigger_idempotency.+on conflict \(idempotency_key\) do nothing`).
		WithArgs(input.IdempotencyKey, input.ConversationID, input.AgentAccountID, input.TriggerServerMsgID, input.TriggerEventID, AgentTriggerStatusRunning).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`(?s)update agent_trigger_idempotency.+status = \$7.+status = \$8 and updated_at <= now\(\) - \(\$9 \* interval '1 millisecond'\)`).
		WithArgs(input.IdempotencyKey, input.ConversationID, input.AgentAccountID, input.TriggerServerMsgID, input.TriggerEventID,
			AgentTriggerStatusRunning, AgentTriggerStatusFailed, AgentTriggerStatusRunning, int64(120000)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	started, err := repo.TryStartAgentTrigger(context.Background(), input)
	if err != nil {
		t.Fatalf("try start trigger: %v", err)
	}
	if !started {
		t.Fatal("started = false, want true")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
