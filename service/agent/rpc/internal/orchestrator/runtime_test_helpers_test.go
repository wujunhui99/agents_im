package orchestrator

import (
	"context"

	agentruntime "github.com/wujunhui99/agents_im/service/agent/rpc/internal/runtime"
)

type runtimeFunc func(context.Context, agentruntime.RunRequest) (agentruntime.RunResult, error)

func (f runtimeFunc) Run(ctx context.Context, req agentruntime.RunRequest) (agentruntime.RunResult, error) {
	return f(ctx, req)
}

type runtimeRequestBuilderFunc func(context.Context, AgentTrigger) (agentruntime.RunRequest, error)

func (f runtimeRequestBuilderFunc) BuildRuntimeRequest(ctx context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error) {
	return f(ctx, trigger)
}
