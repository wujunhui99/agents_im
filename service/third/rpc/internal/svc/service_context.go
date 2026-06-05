package svc

import (
	"log"

	mailprovider "github.com/wujunhui99/agents_im/service/third/rpc/internal/provider"
	"github.com/wujunhui99/agents_im/service/third/rpc/internal/config"
)

type ServiceContext struct {
	Config            config.Config
	MailProvider      mailprovider.TemplateEmailSender
	DefaultTemplateID uint64
}

func NewServiceContext(c config.Config) *ServiceContext {
	provider, err := mailprovider.NewTencentSESProvider(c.TencentSES, nil)
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
