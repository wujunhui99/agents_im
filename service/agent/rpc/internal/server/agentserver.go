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

func (s *AgentServer) CreateAgent(ctx context.Context, in *agent.CreateAgentRequest) (*agent.AgentResponse, error) {
	return logic.NewCreateAgentLogic(ctx, s.svcCtx).CreateAgent(in)
}

func (s *AgentServer) GetAgent(ctx context.Context, in *agent.GetAgentRequest) (*agent.AgentResponse, error) {
	return logic.NewGetAgentLogic(ctx, s.svcCtx).GetAgent(in)
}

func (s *AgentServer) ListAgents(ctx context.Context, in *agent.ListAgentsRequest) (*agent.ListAgentsResponse, error) {
	return logic.NewListAgentsLogic(ctx, s.svcCtx).ListAgents(in)
}

func (s *AgentServer) UpdateAgent(ctx context.Context, in *agent.UpdateAgentRequest) (*agent.AgentResponse, error) {
	return logic.NewUpdateAgentLogic(ctx, s.svcCtx).UpdateAgent(in)
}

func (s *AgentServer) UpdateAgentStatus(ctx context.Context, in *agent.UpdateAgentStatusRequest) (*agent.AgentResponse, error) {
	return logic.NewUpdateAgentStatusLogic(ctx, s.svcCtx).UpdateAgentStatus(in)
}

func (s *AgentServer) GetAgentDefinition(ctx context.Context, in *agent.GetAgentDefinitionRequest) (*agent.AgentDefinitionResponse, error) {
	return logic.NewGetAgentDefinitionLogic(ctx, s.svcCtx).GetAgentDefinition(in)
}

func (s *AgentServer) UpdateAgentDefinition(ctx context.Context, in *agent.UpdateAgentDefinitionRequest) (*agent.AgentDefinitionResponse, error) {
	return logic.NewUpdateAgentDefinitionLogic(ctx, s.svcCtx).UpdateAgentDefinition(in)
}

func (s *AgentServer) EnsureDefaultAssistant(ctx context.Context, in *agent.EnsureDefaultAssistantRequest) (*agent.EnsureDefaultAssistantResponse, error) {
	return logic.NewEnsureDefaultAssistantLogic(ctx, s.svcCtx).EnsureDefaultAssistant(in)
}
