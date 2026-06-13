package svc

import (
	"context"
	"log"
	"time"

	"github.com/wujunhui99/agents_im/common/middleware"
	"github.com/wujunhui99/agents_im/common/share/auth/token"
	userlogic "github.com/wujunhui99/agents_im/internal/logic"
	userrepo "github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/service/auth/core/adminbootstrap"
	business "github.com/wujunhui99/agents_im/service/auth/core/logic"
	"github.com/wujunhui99/agents_im/service/auth/core/mailadapter"
	authrepo "github.com/wujunhui99/agents_im/service/auth/core/repository"
	"github.com/wujunhui99/agents_im/service/auth/core/useradapter"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/postgres"
)

type ServiceContext struct {
	Config    config.Config
	AuthLogic *business.AuthLogic
	AuthRepo  authrepo.CredentialRepository
	UserLogic *userlogic.UserLogic

	// goctl 数据层（auth 域自有，不走 internal/）：EnsureTestCredential 用。
	// auth 域真相已搬到 service/auth/core（脱 internal/auth）；上面仍依赖的
	// internal/logic（UserLogic）+ internal/repository 属 user 域 monolith，
	// 待 user 域迁移后并入这里、auth 域再 goctl 化。
	Credentials model.AuthCredentialsModel
	// AccountsGuard 是跨域鉴权读 keystone 例外（详见 model/accounts_guard.go 注释）。
	AccountsGuard model.AccountsGuardModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	userRepo, err := userrepo.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build user repository: %v", err)
	}
	authRepo, err := authrepo.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build auth repository: %v", err)
	}
	agentRepo, err := userrepo.NewAgentRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build agent repository: %v", err)
	}
	agentRegistryRepo, err := userrepo.NewAgentRegistryRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build agent registry repository: %v", err)
	}
	mailer, err := mailadapter.NewRequiredRPCClient(c.MailRPC)
	if err != nil {
		log.Fatalf("build mail rpc client: %v", err)
	}
	userLogic := userlogic.NewUserLogic(userRepo)
	defaultAssistant := userlogic.NewDefaultAssistantProvisioner(userRepo, agentRepo, agentRegistryRepo)
	userLogic.WithDefaultAssistantProvisioner(defaultAssistant)
	if _, err := defaultAssistant.Backfill(context.Background()); err != nil {
		log.Fatalf("backfill default assistant: %v", err)
	}
	if created, err := adminbootstrap.EnsureAdminAccount(context.Background(), adminbootstrap.Config{
		Identifier:  c.AdminBootstrap.Identifier,
		Password:    c.AdminBootstrap.Password,
		DisplayName: c.AdminBootstrap.DisplayName,
	}, userLogic, authRepo); err != nil {
		log.Fatalf("bootstrap admin account: %v", err)
	} else if created {
		log.Printf("admin bootstrap account ensured for identifier %q", c.AdminBootstrap.Identifier)
	}

	conn := postgres.New(c.DataSource)

	return &ServiceContext{
		Config: c,
		AuthLogic: business.NewAuthLogicWithOptions(authRepo, useradapter.NewLogicClient(userLogic), business.NewPasswordHasher(), token.NewHMACTokenManager(c.TokenAuth.AccessSecret, time.Duration(c.TokenAuth.AccessExpire)*time.Second), business.AuthOptions{
			VerificationRepo: authRepo,
			Sessions:         middleware.NewRedisSessionStore(c.SessionRedis),
			Mailer:           mailer,
		}),
		AuthRepo:      authRepo,
		UserLogic:     userLogic,
		Credentials:   model.NewAuthCredentialsModel(conn),
		AccountsGuard: model.NewAccountsGuardModel(conn),
	}
}
