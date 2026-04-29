package agent

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateAgentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAgentLogic {
	return &CreateAgentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateAgentLogic) CreateAgent(req *types.CreateAgentReq) (*types.AgentResp, error) {
	createdBy, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	agent, err := l.svcCtx.AgentLogic.CreateAgent(l.ctx, business.CreateAgentRequest{
		IMUserID:    req.IMUserID,
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		CreatedBy:   createdBy,
	})
	if err != nil {
		return nil, err
	}
	return agentResp(agent), nil
}

type GetAgentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAgentLogic {
	return &GetAgentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAgentLogic) GetAgent(req *types.AgentPathReq) (*types.AgentResp, error) {
	agent, err := l.svcCtx.AgentLogic.GetAgent(l.ctx, business.AgentPathRequest{AgentID: req.AgentID})
	if err != nil {
		return nil, err
	}
	return agentResp(agent), nil
}

type ListAgentsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListAgentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAgentsLogic {
	return &ListAgentsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListAgentsLogic) ListAgents(req *types.ListAgentsReq) (*types.ListAgentsResp, error) {
	result, err := l.svcCtx.AgentLogic.ListAgents(l.ctx, business.ListAgentsRequest{
		Status:    req.Status,
		CreatedBy: req.CreatedBy,
		Limit:     int(req.Limit),
		Offset:    int(req.Offset),
	})
	if err != nil {
		return nil, err
	}
	agents := make([]types.Agent, 0, len(result.Agents))
	for _, item := range result.Agents {
		agents = append(agents, agentType(item))
	}
	return &types.ListAgentsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.ListAgentsData{
			Agents: agents,
		},
	}, nil
}

type UpdateAgentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAgentLogic {
	return &UpdateAgentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateAgentLogic) UpdateAgent(req *types.UpdateAgentReq) (*types.AgentResp, error) {
	agent, err := l.svcCtx.AgentLogic.UpdateAgent(l.ctx, business.UpdateAgentRequest{
		AgentID:     req.AgentID,
		Name:        optionalAgentString(req.Name),
		Description: optionalAgentString(req.Description),
	})
	if err != nil {
		return nil, err
	}
	return agentResp(agent), nil
}

type UpdateAgentStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAgentStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAgentStatusLogic {
	return &UpdateAgentStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateAgentStatusLogic) UpdateAgentStatus(req *types.UpdateAgentStatusReq) (*types.AgentResp, error) {
	agent, err := l.svcCtx.AgentLogic.UpdateAgentStatus(l.ctx, business.UpdateAgentStatusRequest{
		AgentID: req.AgentID,
		Status:  req.Status,
	})
	if err != nil {
		return nil, err
	}
	return agentResp(agent), nil
}

type DeleteAgentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAgentLogic {
	return &DeleteAgentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteAgentLogic) DeleteAgent(req *types.AgentPathReq) (*types.AgentResp, error) {
	agent, err := l.svcCtx.AgentLogic.ArchiveAgent(l.ctx, business.AgentPathRequest{AgentID: req.AgentID})
	if err != nil {
		return nil, err
	}
	return agentResp(agent), nil
}

func optionalAgentString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func agentResp(agent business.AgentInfo) *types.AgentResp {
	return &types.AgentResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    agentType(agent),
	}
}

func agentType(agent business.AgentInfo) types.Agent {
	return types.Agent{
		AgentID:     agent.AgentID,
		IMUserID:    agent.IMUserID,
		Name:        agent.Name,
		Description: agent.Description,
		Status:      agent.Status,
		CreatedBy:   agent.CreatedBy,
		CreatedAt:   agent.CreatedAt,
		UpdatedAt:   agent.UpdatedAt,
	}
}
