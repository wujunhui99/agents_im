// Package server is the agent-rpc gRPC surface (04-agent §3.2). Hand-written
// (not goctl-scaffolded) so it composes with the agent.trigger.v1 consumer that
// shares the same ServiceContext.
package server

import (
	"context"

	"github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/logic"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/svc"
)

type AgentServer struct {
	svcCtx *svc.ServiceContext
	agent.UnimplementedAgentServer
}

func NewAgentServer(svcCtx *svc.ServiceContext) *AgentServer {
	return &AgentServer{svcCtx: svcCtx}
}

func (s *AgentServer) GetConversationAIHosting(ctx context.Context, in *agent.GetConversationAIHostingRequest) (*agent.ConversationAIHostingState, error) {
	l := logic.NewGetConversationAIHostingLogic(ctx, s.svcCtx)
	return l.GetConversationAIHosting(in)
}

func (s *AgentServer) UpdateConversationAIHosting(ctx context.Context, in *agent.UpdateConversationAIHostingRequest) (*agent.ConversationAIHostingState, error) {
	l := logic.NewUpdateConversationAIHostingLogic(ctx, s.svcCtx)
	return l.UpdateConversationAIHosting(in)
}
