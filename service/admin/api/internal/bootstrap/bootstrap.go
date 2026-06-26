package bootstrap

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	commonconfig "github.com/wujunhui99/agents_im/service/admin/api/internal/config"
	authpb "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"github.com/wujunhui99/agents_im/service/auth/rpc/authclient"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
)

// EnsureAdminAccount ensures the configured admin account exists and has an
// initial login credential. Existing admin credentials are never overwritten.
func EnsureAdminAccount(ctx context.Context, cfg commonconfig.AdminBootstrapConfig, users userclient.User, credentials authclient.Auth) (bool, error) {
	defaults := commonconfig.DefaultAdminBootstrapConfig()
	identifier := strings.TrimSpace(cfg.Identifier)
	if identifier == "" {
		identifier = defaults.Identifier
	}
	password := cfg.Password
	if identifier == "" || strings.TrimSpace(password) == "" {
		return false, nil
	}
	if users == nil {
		return false, apperror.Internal("admin bootstrap user rpc is not configured")
	}
	if credentials == nil {
		return false, apperror.Internal("admin bootstrap auth rpc is not configured")
	}

	displayName := strings.TrimSpace(cfg.DisplayName)
	if displayName == "" {
		displayName = defaults.DisplayName
	}
	if displayName == "" {
		displayName = identifier
	}

	user, created, err := ensureAdminUser(ctx, users, identifier, displayName)
	if err != nil {
		return false, err
	}
	credential, err := credentials.EnsureAdminCredential(ctx, &authpb.EnsureAdminCredentialRequest{
		UserId:     user.GetUserId(),
		Identifier: user.GetIdentifier(),
		Password:   password,
	})
	if err != nil {
		return false, rpcerror.FromStatus(err)
	}
	return created || credential.GetCreated(), nil
}

func ensureAdminUser(ctx context.Context, users userclient.User, identifier string, displayName string) (*userpb.UserEntity, bool, error) {
	found, err := users.GetUserByIdentifier(ctx, &userpb.GetUserByIdentifierRequest{Identifier: identifier})
	if err == nil {
		user := found.GetUser()
		if user == nil {
			return nil, false, apperror.Internal("admin bootstrap user lookup returned empty user")
		}
		if user.GetAccountType() != string(model.AccountTypeAdmin) {
			return nil, false, apperror.Forbidden("admin bootstrap identifier already exists as non-admin account")
		}
		return user, false, nil
	}
	mapped := rpcerror.FromStatus(err)
	if apperror.From(mapped).Code != apperror.CodeNotFound {
		return nil, false, mapped
	}

	created, err := users.CreateUser(ctx, &userpb.CreateUserRequest{
		Identifier:  identifier,
		DisplayName: displayName,
		Name:        displayName,
		AccountType: string(model.AccountTypeAdmin),
	})
	if err != nil {
		return nil, false, rpcerror.FromStatus(err)
	}
	user := created.GetUser()
	if user == nil {
		return nil, false, apperror.Internal("admin bootstrap user create returned empty user")
	}
	if user.GetAccountType() != string(model.AccountTypeAdmin) {
		return nil, false, apperror.Internal("admin bootstrap user create returned non-admin account")
	}
	return user, true, nil
}
