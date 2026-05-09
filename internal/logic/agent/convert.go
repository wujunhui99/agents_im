package agent

import (
	"github.com/wujunhui99/agents_im/internal/apperror"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/types"
)

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
