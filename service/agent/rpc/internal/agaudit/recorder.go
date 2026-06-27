package agaudit

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/agentaudit"
	"github.com/wujunhui99/agents_im/pkg/apperror"
)

// RunRecorder 把 Store 适配成 orchestrator.AgentRunOrchestrator 期望的 run 审计写入器
// （结构化匹配 orchestrator.AgentRunAuditRecorder，无需互相 import）：RecordAgentRun →
// Store.CreateAgentRun。取代旧 internal/logic.AgentAuditLogic 的 run 写路径。
type RunRecorder struct {
	store Store
}

// NewRunRecorder 包装一个 Store 为 run 审计写入器。
func NewRunRecorder(store Store) RunRecorder {
	return RunRecorder{store: store}
}

// RecordAgentRun 记录一条 agent run 审计行。
func (r RunRecorder) RecordAgentRun(ctx context.Context, input agentaudit.CreateRunInput) (agentaudit.AgentRun, error) {
	if r.store == nil {
		return agentaudit.AgentRun{}, apperror.Internal("agent audit store is not configured")
	}
	return r.store.CreateAgentRun(ctx, input)
}
