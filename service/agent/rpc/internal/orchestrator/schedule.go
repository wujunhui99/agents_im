package orchestrator

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

// ScheduleTrigger schedules one already-detected agent trigger: idempotency
// gate (agent_triggers ledger via TryStartAgentTrigger), group-membership
// authorization, read-marking and the async run+write-back. It is the entry
// point the agent-rpc trigger consumer uses after trigger.Judge has performed
// the D15 final judgment — detection lives in the Judge, scheduling/idempotency
// stays here on the proven CHS path (04-agent §4.2, #340).
func (s *ConversationHostingService) ScheduleTrigger(ctx context.Context, trigger AgentTrigger) (bool, error) {
	if s == nil || s.repo == nil {
		return false, apperror.Internal("agent conversation hosting repository is not configured")
	}
	if s.runner == nil {
		return false, apperror.Internal("agent trigger runner is not configured")
	}
	return s.acceptAndScheduleTrigger(ctx, trigger)
}
