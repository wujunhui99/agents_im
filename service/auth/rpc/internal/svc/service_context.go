package svc

import (
	"context"
	"log"
	"time"

	"github.com/wujunhui99/agents_im/internal/adminbootstrap"
	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/internal/auth/mailadapter"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/common/share/auth/token"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
	userlogic "github.com/wujunhui99/agents_im/internal/logic"
	userrepo "github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/config"
)

type ServiceContext struct {
	Config    config.Config
	AuthLogic *business.AuthLogic
	AuthRepo  authrepo.CredentialRepository
	UserLogic *userlogic.UserLogic
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

	return &ServiceContext{
		Config: c,
		AuthLogic: business.NewAuthLogicWithOptions(authRepo, useradapter.NewLogicClient(userLogic), business.NewPasswordHasher(), token.NewHMACTokenManager(c.TokenAuth.AccessSecret, time.Duration(c.TokenAuth.AccessExpire)*time.Second), business.AuthOptions{
			VerificationRepo: authRepo,
			Mailer:           mailer,
		}),
		AuthRepo:  authRepo,
		UserLogic: userLogic,
	}
}
