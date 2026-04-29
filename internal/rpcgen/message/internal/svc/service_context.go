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
}

func NewServiceContext(c config.Config) *ServiceContext {
	messageRepo := repository.NewMemoryMessageRepository()
	return &ServiceContext{
		Config:       c,
		MessageLogic: business.NewMessageLogic(messageRepo),
		MessageRepo:  messageRepo,
	}
}
