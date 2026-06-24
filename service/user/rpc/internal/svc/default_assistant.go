package svc

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	agentpb "github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/wujunhui99/agents_im/service/agent/rpc/agentclient"
	friendsclient "github.com/wujunhui99/agents_im/service/friends/rpc/friendsclient"
)

// 默认助手开通是跨 user/agent/friends 三域的编排（#606，从 internal/logic.DefaultAssistantProvisioner
// 拆出，由 user-rpc 装配）：
//   - 账号（user 域）：本地 goctl assistantAccountRepo 幂等 ensure（agent_creator，含 legacy 改名 + 资料）；
//   - agent 行 + 提示词 + 工具绑定（agent 域）：经 agent-rpc.EnsureDefaultAssistant；
//   - 好友（friends 域）：经 friends-rpc.EnsureFriendship。
//
// 跨服务尽力而为非事务（user-rpc → agent-rpc / friends-rpc 均单向叶子调用，不成环）。

const (
	defaultAssistantIdentifier       = "agent_creator"
	defaultAssistantLegacyIdentifier = "agent_father"
	defaultAssistantDisplayName      = "AI 助手"
)

// assistantAccounts 是默认助手账号编排所需的账号端口（user 域，由 assistantAccountRepo 实现）。
type assistantAccounts interface {
	GetByIdentifier(ctx context.Context, identifier string) (model.User, error)
	GetByID(ctx context.Context, accountID string) (model.User, error)
	Create(ctx context.Context, account model.User) (model.User, error)
	ListByAccountType(ctx context.Context, accountType model.AccountType) ([]model.User, error)
	RenameIdentifier(ctx context.Context, fromIdentifier, toIdentifier string) (model.User, error)
	UpdateProfile(ctx context.Context, accountID string, patch repository.AccountProfilePatch) (model.User, error)
}

type defaultAssistantProvisioner struct {
	accounts assistantAccounts
	agentRPC agentclient.Agent
	friends  friendsclient.Friends
}

func newDefaultAssistantProvisioner(accounts assistantAccounts, agentRPC agentclient.Agent, friends friendsclient.Friends) *defaultAssistantProvisioner {
	return &defaultAssistantProvisioner{accounts: accounts, agentRPC: agentRPC, friends: friends}
}

type defaultAssistantBackfillResult struct {
	AssistantAccountID string
	AgentID            string
	HumanUsersScanned  int
}

// Backfill 在启动时确保默认助手（账号 + agent 域装配）就绪，并把所有人类用户与其互为好友。
func (p *defaultAssistantProvisioner) Backfill(ctx context.Context) (defaultAssistantBackfillResult, error) {
	assistant, agentID, err := p.ensureAssistant(ctx)
	if err != nil {
		return defaultAssistantBackfillResult{}, err
	}
	humans, err := p.accounts.ListByAccountType(ctx, model.AccountTypeUser)
	if err != nil {
		return defaultAssistantBackfillResult{}, err
	}
	for _, human := range humans {
		if human.AccountID == assistant.AccountID {
			continue
		}
		if err := p.ensureFriendship(ctx, human.AccountID, assistant.AccountID); err != nil {
			return defaultAssistantBackfillResult{}, err
		}
	}
	return defaultAssistantBackfillResult{
		AssistantAccountID: assistant.AccountID,
		AgentID:            agentID,
		HumanUsersScanned:  len(humans),
	}, nil
}

// EnsureForUser 在新用户创建时确保默认助手就绪并与该用户互为好友（user / test 账号）。
func (p *defaultAssistantProvisioner) EnsureForUser(ctx context.Context, accountID string) error {
	account, err := p.accounts.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	// 测试账户（admin 后台创建）与 user 一致开通默认助手，便于测试 AI 链路。
	if account.AccountType != model.AccountTypeUser && account.AccountType != model.AccountTypeTest {
		return nil
	}
	assistant, _, err := p.ensureAssistant(ctx)
	if err != nil {
		return err
	}
	if account.AccountID == assistant.AccountID {
		return nil
	}
	return p.ensureFriendship(ctx, account.AccountID, assistant.AccountID)
}

// ensureAssistant 确保助手账号（user 域）+ agent 域装配（经 agent-rpc），返回账号与 agent_id。
func (p *defaultAssistantProvisioner) ensureAssistant(ctx context.Context) (model.User, string, error) {
	account, err := p.ensureAssistantAccount(ctx)
	if err != nil {
		return model.User{}, "", err
	}
	resp, err := p.agentRPC.EnsureDefaultAssistant(ctx, &agentpb.EnsureDefaultAssistantRequest{AccountId: account.AccountID})
	if err != nil {
		return model.User{}, "", rpcerror.FromStatus(err)
	}
	return account, resp.GetAgentId(), nil
}

func (p *defaultAssistantProvisioner) ensureAssistantAccount(ctx context.Context) (model.User, error) {
	account, err := p.accounts.GetByIdentifier(ctx, defaultAssistantIdentifier)
	if err == nil {
		return p.ensureAssistantProfile(ctx, account)
	}
	if !isAppNotFound(err) {
		return model.User{}, err
	}

	legacy, legacyErr := p.accounts.GetByIdentifier(ctx, defaultAssistantLegacyIdentifier)
	if legacyErr == nil {
		renamed, renameErr := p.accounts.RenameIdentifier(ctx, legacy.Identifier, defaultAssistantIdentifier)
		if renameErr != nil {
			return model.User{}, renameErr
		}
		return p.ensureAssistantProfile(ctx, renamed)
	}
	if !isAppNotFound(legacyErr) {
		return model.User{}, legacyErr
	}

	return p.accounts.Create(ctx, model.User{
		Identifier:  defaultAssistantIdentifier,
		DisplayName: defaultAssistantDisplayName,
		Name:        defaultAssistantIdentifier,
		Gender:      "unknown",
		AccountType: model.AccountTypeAgent,
	})
}

func (p *defaultAssistantProvisioner) ensureAssistantProfile(ctx context.Context, account model.User) (model.User, error) {
	if account.AccountType != model.AccountTypeAgent {
		return model.User{}, apperror.InvalidArgument("agent_creator account_type must be agent")
	}
	if account.DisplayName == defaultAssistantDisplayName && account.Name == defaultAssistantIdentifier {
		return account, nil
	}
	displayName := defaultAssistantDisplayName
	name := defaultAssistantIdentifier
	return p.accounts.UpdateProfile(ctx, account.AccountID, repository.AccountProfilePatch{
		DisplayName: &displayName,
		Name:        &name,
	})
}

func (p *defaultAssistantProvisioner) ensureFriendship(ctx context.Context, userID, friendID string) error {
	if _, err := p.friends.EnsureFriendship(ctx, &friendsclient.EnsureFriendshipRequest{UserId: userID, FriendId: friendID}); err != nil {
		return rpcerror.FromStatus(err)
	}
	return nil
}

func isAppNotFound(err error) bool {
	return apperror.From(err).Code == apperror.CodeNotFound
}
