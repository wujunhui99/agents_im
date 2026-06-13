package adminbootstrap

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/common/share/model"
	authlogic "github.com/wujunhui99/agents_im/service/auth/core/logic"
	authmodel "github.com/wujunhui99/agents_im/service/auth/core/model"
	authrepo "github.com/wujunhui99/agents_im/service/auth/core/repository"
	// 过渡依赖：bootstrap 建 admin user 暂调 user 域 monolith logic；待 user 域迁移后改走 user-rpc。
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/config"
)

type Config struct {
	Identifier  string
	Password    string
	DisplayName string
}

func FromAPIConfig(cfg config.APIConfig) Config {
	return Config{
		Identifier:  cfg.AdminBootstrap.Identifier,
		Password:    cfg.AdminBootstrap.Password,
		DisplayName: cfg.AdminBootstrap.DisplayName,
	}
}

func EnsureAdminAccount(ctx context.Context, cfg Config, users *logic.UserLogic, credentials authrepo.CredentialRepository) (bool, error) {
	identifier := strings.TrimSpace(cfg.Identifier)
	password := cfg.Password
	if identifier == "" || strings.TrimSpace(password) == "" {
		return false, nil
	}
	if users == nil {
		return false, apperror.Internal("admin bootstrap user logic is not configured")
	}
	if credentials == nil {
		return false, apperror.Internal("admin bootstrap credential repository is not configured")
	}

	profile, err := users.GetUserByIdentifier(ctx, logic.GetUserByIdentifierRequest{Identifier: identifier})
	created := false
	if err != nil {
		if apperror.From(err).Code != apperror.CodeNotFound {
			return false, err
		}
		displayName := strings.TrimSpace(cfg.DisplayName)
		if displayName == "" {
			displayName = identifier
		}
		profile, err = users.CreateUser(ctx, logic.CreateUserRequest{
			Identifier:  identifier,
			DisplayName: displayName,
			AccountType: string(model.AccountTypeAdmin),
		})
		if err != nil {
			return false, err
		}
		created = true
	} else if profile.AccountType != string(model.AccountTypeAdmin) {
		return false, apperror.Forbidden("admin bootstrap identifier already exists as non-admin account")
	}

	if _, err := credentials.GetByIdentifier(ctx, profile.Identifier); err == nil {
		return created, nil
	} else if apperror.From(err).Code != apperror.CodeNotFound {
		return false, err
	}

	hash, salt, version, err := authlogic.NewPasswordHasher().Hash(password)
	if err != nil {
		return false, apperror.Internal("admin bootstrap password hash failed")
	}
	if _, err := credentials.Create(ctx, authmodel.Credential{
		Identifier:   profile.Identifier,
		UserID:       profile.UserID,
		PasswordHash: hash,
		Salt:         salt,
		HashVersion:  version,
	}); err != nil {
		return false, err
	}
	return true, nil
}
