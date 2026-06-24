// agent_crud_logic.go 是 agent CRUD 的 gRPC 处理器（#606）：薄封装 agent 域业务逻辑
// （svcCtx.AgentLogic over agent 自有 goctl 数据层），做 proto↔业务类型转换 + apperror→status。
package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/agentlogic"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateAgentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAgentLogic {
	return &CreateAgentLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *CreateAgentLogic) CreateAgent(in *agent.CreateAgentRequest) (*agent.AgentResponse, error) {
	info, err := l.svcCtx.AgentLogic.CreateAgent(l.ctx, agentlogic.CreateAgentRequest{
		AccountID:   in.GetAccountId(),
		IMUserID:    in.GetImUserId(),
		Name:        in.GetName(),
		Description: in.GetDescription(),
		Status:      in.GetStatus(),
		CreatedBy:   in.GetCreatedBy(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &agent.AgentResponse{Agent: agentInfoToPB(info)}, nil
}

type GetAgentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAgentLogic {
	return &GetAgentLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetAgentLogic) GetAgent(in *agent.GetAgentRequest) (*agent.AgentResponse, error) {
	info, err := l.svcCtx.AgentLogic.GetAgent(l.ctx, in.GetAgentId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &agent.AgentResponse{Agent: agentInfoToPB(info)}, nil
}

type ListAgentsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAgentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAgentsLogic {
	return &ListAgentsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListAgentsLogic) ListAgents(in *agent.ListAgentsRequest) (*agent.ListAgentsResponse, error) {
	infos, err := l.svcCtx.AgentLogic.ListAgents(l.ctx, agentlogic.ListAgentsRequest{
		Status:    in.GetStatus(),
		CreatedBy: in.GetCreatedBy(),
		Limit:     int(in.GetLimit()),
		Offset:    int(in.GetOffset()),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	agents := make([]*agent.AgentEntity, 0, len(infos))
	for _, info := range infos {
		agents = append(agents, agentInfoToPB(info))
	}
	return &agent.ListAgentsResponse{Agents: agents}, nil
}

type UpdateAgentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAgentLogic {
	return &UpdateAgentLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateAgentLogic) UpdateAgent(in *agent.UpdateAgentRequest) (*agent.AgentResponse, error) {
	info, err := l.svcCtx.AgentLogic.UpdateAgent(l.ctx, agentlogic.UpdateAgentRequest{
		AgentID:     in.GetAgentId(),
		Name:        in.Name,
		Description: in.Description,
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &agent.AgentResponse{Agent: agentInfoToPB(info)}, nil
}

type UpdateAgentStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAgentStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAgentStatusLogic {
	return &UpdateAgentStatusLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateAgentStatusLogic) UpdateAgentStatus(in *agent.UpdateAgentStatusRequest) (*agent.AgentResponse, error) {
	info, err := l.svcCtx.AgentLogic.UpdateAgentStatus(l.ctx, agentlogic.UpdateAgentStatusRequest{
		AgentID: in.GetAgentId(),
		Status:  in.GetStatus(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &agent.AgentResponse{Agent: agentInfoToPB(info)}, nil
}

func agentInfoToPB(info agentlogic.AgentInfo) *agent.AgentEntity {
	return &agent.AgentEntity{
		AgentId:     info.AgentID,
		AccountId:   info.AccountID,
		ImUserId:    info.IMUserID,
		Name:        info.Name,
		Description: info.Description,
		Status:      info.Status,
		CreatedBy:   info.CreatedBy,
		CreatedAt:   info.CreatedAt,
		UpdatedAt:   info.UpdatedAt,
	}
}
