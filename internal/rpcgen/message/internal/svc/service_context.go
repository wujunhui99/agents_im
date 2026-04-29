package svc

import (
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/rpcgen/message/internal/config"
)

type ServiceContext struct {
	Config       config.Config
	MessageLogic *business.MessageLogic
	MessageRepo  repository.MessageRepository
	OutboxRepo   repository.OutboxRepository
}

func NewServiceContext(c config.Config) *ServiceContext {
	messageRepo := repository.MustMessageRepositoryForStorage(c.StorageDriver, c.DataSource)
	return &ServiceContext{
		Config:       c,
		MessageLogic: business.NewMessageLogic(messageRepo),
		MessageRepo:  messageRepo,
		OutboxRepo:   outboxRepositoryFromMessageRepo(messageRepo),
	}
}

func outboxRepositoryFromMessageRepo(repo repository.MessageRepository) repository.OutboxRepository {
	outboxRepo, _ := repo.(repository.OutboxRepository)
	return outboxRepo
}
