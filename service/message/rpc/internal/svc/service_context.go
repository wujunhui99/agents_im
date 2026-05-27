package svc

import (
	"log"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/service/message/rpc/internal/config"
)

type ServiceContext struct {
	Config       config.Config
	MessageLogic *business.MessageLogic
	MessageRepo  repository.MessageRepository
	OutboxRepo   repository.OutboxRepository
}

func NewServiceContext(c config.Config) *ServiceContext {
	messageRepo, err := repository.NewMessageRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build message repository: %v", err)
	}
	mediaRepo, err := repository.NewMediaRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build media repository: %v", err)
	}
	groupsRepo, err := repository.NewGroupsRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build groups repository: %v", err)
	}
	groupsLogic := business.NewGroupsLogic(groupsRepo, nil)
	return &ServiceContext{
		Config:       c,
		MessageLogic: business.NewMessageLogicWithMediaValidator(messageRepo, nil, groupsLogic, business.NewMediaLogic(mediaRepo, nil, "")),
		MessageRepo:  messageRepo,
		OutboxRepo:   outboxRepositoryFromMessageRepo(messageRepo),
	}
}

func outboxRepositoryFromMessageRepo(repo repository.MessageRepository) repository.OutboxRepository {
	outboxRepo, _ := repo.(repository.OutboxRepository)
	return outboxRepo
}
