package svc

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
)

// 默认助手的「账号」部分归 user 域所有：user-rpc 只在启动时幂等确保 agent_creator
// 账号存在（本地 accounts/profiles 写入），不涉及 agent/friends 域。
//
// agent 域装配（agent 行 + 提示词 + 工具绑定）与好友关系由 agent-rpc 消费
// user.created.v1 后自行编排：user-rpc → agent-rpc / friends-rpc 的同步调用已删除，
// 这条反向边是历史上 user ↔ agent、user ↔ friends 两个调用环的来源。

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
	RenameIdentifier(ctx context.Context, fromIdentifier, toIdentifier string) (model.User, error)
	UpdateProfile(ctx context.Context, accountID string, patch AccountProfilePatch) (model.User, error)
}

type assistantAccountEnsurer struct {
	accounts assistantAccounts
}

func newAssistantAccountEnsurer(accounts assistantAccounts) *assistantAccountEnsurer {
	return &assistantAccountEnsurer{accounts: accounts}
}

// Ensure 幂等确保默认助手账号存在（legacy identifier 改名 + 资料回填），返回账号。
func (e *assistantAccountEnsurer) Ensure(ctx context.Context) (model.User, error) {
	account, err := e.accounts.GetByIdentifier(ctx, defaultAssistantIdentifier)
	if err == nil {
		return e.ensureProfile(ctx, account)
	}
	if !isAppNotFound(err) {
		return model.User{}, err
	}

	legacy, legacyErr := e.accounts.GetByIdentifier(ctx, defaultAssistantLegacyIdentifier)
	if legacyErr == nil {
		renamed, renameErr := e.accounts.RenameIdentifier(ctx, legacy.Identifier, defaultAssistantIdentifier)
		if renameErr != nil {
			return model.User{}, renameErr
		}
		return e.ensureProfile(ctx, renamed)
	}
	if !isAppNotFound(legacyErr) {
		return model.User{}, legacyErr
	}

	return e.accounts.Create(ctx, model.User{
		Identifier:  defaultAssistantIdentifier,
		DisplayName: defaultAssistantDisplayName,
		Name:        defaultAssistantIdentifier,
		Gender:      "unknown",
		AccountType: model.AccountTypeAgent,
	})
}

func (e *assistantAccountEnsurer) ensureProfile(ctx context.Context, account model.User) (model.User, error) {
	if account.AccountType != model.AccountTypeAgent {
		return model.User{}, apperror.InvalidArgument("agent_creator account_type must be agent")
	}
	if account.DisplayName == defaultAssistantDisplayName && account.Name == defaultAssistantIdentifier {
		return account, nil
	}
	displayName := defaultAssistantDisplayName
	name := defaultAssistantIdentifier
	return e.accounts.UpdateProfile(ctx, account.AccountID, AccountProfilePatch{
		DisplayName: &displayName,
		Name:        &name,
	})
}

func isAppNotFound(err error) bool {
	return apperror.From(err).Code == apperror.CodeNotFound
}
