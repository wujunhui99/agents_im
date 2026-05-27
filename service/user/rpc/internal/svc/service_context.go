package svc

import (
	"context"
	"log"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/config"
)

type ServiceContext struct {
	Config     config.Config
	UserLogic  *business.UserLogic
	MediaLogic *business.MediaLogic
	Repo       repository.Repository
}

func NewServiceContext(c config.Config) *ServiceContext {
	repo, err := repository.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build user repository: %v", err)
	}
	agentRepo, err := repository.NewAgentRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build agent repository: %v", err)
	}
	agentRegistryRepo, err := repository.NewAgentRegistryRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build agent registry repository: %v", err)
	}
	mediaRepo, err := repository.NewMediaRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build media repository: %v", err)
	}
	userLogic := business.NewUserLogic(repo)
	defaultAssistant := business.NewDefaultAssistantProvisioner(repo, agentRepo, agentRegistryRepo)
	userLogic.WithDefaultAssistantProvisioner(defaultAssistant)
	if _, err := defaultAssistant.Backfill(context.Background()); err != nil {
		log.Fatalf("backfill default assistant: %v", err)
	}
	return &ServiceContext{
		Config:     c,
		UserLogic:  userLogic,
		MediaLogic: business.NewMediaLogic(mediaRepo, nil, ""),
		Repo:       repo,
	}
}
