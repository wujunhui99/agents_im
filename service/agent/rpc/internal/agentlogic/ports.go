// Package agentlogic 是 agent 域纯业务逻辑（agent CRUD、注册表 CRUD/校验、定义装配、
// agent.create saga、默认助手装配），#606 从顶层 internal/logic 退役迁入属主 agent-rpc。
//
// 它依赖：
//   - agent 自有数据层（AgentStore over goctl AgentsModel + RegistryStore over registry.Store）；
//   - 跨域端口（AccountPort=user-rpc、FriendPort=friends-rpc），由调用方注入。
//
// 本包不 import svc/aihosting/registry 具体类型（只用接口），避免与 gRPC 装配成环。对外 string ID，
// 内部 string↔int64 转换沿用 #013 bigint keystone 约定。
package agentlogic

import (
	"context"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
)

const genderUnknown = "unknown"

// AccountReader 是 agent 业务所需的账号只读端口（属主 user 域，agent-rpc 经 user-rpc 注入）。
type AccountReader interface {
	GetByID(ctx context.Context, accountID string) (model.User, error)
	ExistsByIdentifier(ctx context.Context, identifier string) (bool, error)
}

// AccountPort 在只读基础上加账号创建（agent.create 工具建 agent 账号），经 user-rpc。
type AccountPort interface {
	AccountReader
	Create(ctx context.Context, account model.User) (model.User, error)
}

// FriendPort 是好友建立端口（属主 friends 域，经 friends-rpc EnsureFriendship）。
type FriendPort interface {
	EnsureFriendship(ctx context.Context, userID string, friendID string) error
}

// AgentStore 是 agents 表读写端口（属主 agent 域，goctl model 实现 PgAgentStore）。
// 对外 string agent_id；GetAgentByIMUserID 满足 orchestrator.AgentReader（im_user_id == account_id）。
type AgentStore interface {
	CreateAgent(ctx context.Context, agent model.Agent) (model.Agent, error)
	GetAgent(ctx context.Context, agentID string) (model.Agent, error)
	GetAgentByAccountID(ctx context.Context, accountID string) (model.Agent, error)
	GetAgentByIMUserID(ctx context.Context, imUserID string) (model.Agent, error)
	ListAgents(ctx context.Context, status, createdBy string, limit, offset int) ([]model.Agent, error)
	UpdateAgent(ctx context.Context, agentID string, name, description *string) (model.Agent, error)
	UpdateAgentStatus(ctx context.Context, agentID string, status string) (model.Agent, error)
}

// RegistryStore 是注册表读写端口，由 service/agent/rpc/internal/registry.Store 实现（goctl model）。
type RegistryStore interface {
	CreatePrompt(ctx context.Context, prompt model.AgentPrompt) (model.AgentPrompt, error)
	GetPrompt(ctx context.Context, promptID string) (model.AgentPrompt, error)
	GetPromptByNameVersion(ctx context.Context, name string, version string) (model.AgentPrompt, error)
	BindPrompt(ctx context.Context, binding model.AgentPromptBinding) (model.AgentPromptBinding, bool, error)
	ListPromptBindings(ctx context.Context, agentID string) ([]model.AgentPromptBinding, error)
	ReplacePromptBindings(ctx context.Context, agentID string, promptIDs []string, createdBy string) ([]model.AgentPromptBinding, error)

	CreateMCPServer(ctx context.Context, server model.AgentMCPServer) (model.AgentMCPServer, error)
	GetMCPServer(ctx context.Context, serverID string) (model.AgentMCPServer, error)

	RegisterTool(ctx context.Context, tool model.AgentTool) (model.AgentTool, error)
	UpsertToolByName(ctx context.Context, tool model.AgentTool) (model.AgentTool, error)
	GetTool(ctx context.Context, toolID string) (model.AgentTool, error)
	GetToolByName(ctx context.Context, name string) (model.AgentTool, error)
	ListActiveTools(ctx context.Context) ([]model.AgentTool, error)
	BindTool(ctx context.Context, binding model.AgentToolBinding) (model.AgentToolBinding, bool, error)
	GetToolBinding(ctx context.Context, agentID string, toolID string) (model.AgentToolBinding, error)
	ListToolBindings(ctx context.Context, agentID string) ([]model.AgentToolBinding, error)
	ReplaceToolBindings(ctx context.Context, agentID string, toolIDs []string, createdBy string) ([]model.AgentToolBinding, error)

	RegisterSkill(ctx context.Context, skill model.AgentSkill) (model.AgentSkill, error)
	GetSkill(ctx context.Context, skillID string) (model.AgentSkill, error)
	BindSkill(ctx context.Context, binding model.AgentSkillBinding) (model.AgentSkillBinding, bool, error)
}

// --- 共享校验 / 格式化（从 internal/logic 迁入；后端只守完整性，不做客户端负责的规范化）---

func normalizeRequiredID(value string, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument(field + " is required")
	}
	if len([]rune(value)) > 64 {
		return "", apperror.InvalidArgument(field + " must be 64 characters or fewer")
	}
	return value, nil
}

func normalizeIdentifier(identifier string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(identifier))
	if len(normalized) < 3 || len(normalized) > 32 {
		return "", apperror.InvalidArgument("identifier must be 3 to 32 characters")
	}
	for idx, r := range normalized {
		isLetter := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		isUnderscore := r == '_'
		if idx == 0 && !isLetter && !isDigit {
			return "", apperror.InvalidArgument("identifier must start with a letter or digit")
		}
		if !isLetter && !isDigit && !isUnderscore {
			return "", apperror.InvalidArgument("identifier can only contain letters, digits, and underscore")
		}
	}
	return normalized, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func isAppNotFound(err error) bool {
	return apperror.From(err).Code == apperror.CodeNotFound
}
