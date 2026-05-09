package svc

import (
	"log"

	"github.com/wujunhui99/agents_im/internal/mail"
	"github.com/wujunhui99/agents_im/internal/rpcgen/mail/internal/config"
)

type ServiceContext struct {
	Config            config.Config
	MailProvider      mail.TemplateEmailSender
	DefaultTemplateID uint64
}

func NewServiceContext(c config.Config) *ServiceContext {
	provider, err := mail.NewTencentSESProvider(c.TencentSES, nil)
	if err != nil {
		log.Fatalf("build mail provider: %v", err)
	}
	defaultTemplateID, err := c.TencentSES.DefaultTemplateIDValue()
	if err != nil {
		log.Fatalf("build mail provider: %v", err)
	}
	return &ServiceContext{
		Config:            c,
		MailProvider:      provider,
		DefaultTemplateID: defaultTemplateID,
	}
}
