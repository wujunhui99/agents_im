package svc

import (
	"github.com/wujunhui99/agents_im/service/resource/rpc/internal/mail/config"
	mailprovider "github.com/wujunhui99/agents_im/service/resource/rpc/internal/mail/provider"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config            config.Config
	MailProvider      mailprovider.TemplateEmailSender
	DefaultTemplateID uint64
}

func NewServiceContext(c config.Config) *ServiceContext {
	// mail 是 resource-rpc 的可选能力：未配置 SES 凭据时（本地开发、或未开启邮件通道的
	// 部署）降级为「不可用」而不是让整个进程退出——否则 media（对象存储）会被一起拖死。
	// SendTemplateEmail 在 provider 为 nil 时返回 FailedPrecondition。
	provider, err := mailprovider.NewTencentSESProvider(c.TencentSES, nil)
	if err != nil {
		logx.Errorf("resource-rpc: mail channel disabled (provider config incomplete): %v", err)
		return &ServiceContext{Config: c}
	}
	defaultTemplateID, err := c.TencentSES.DefaultTemplateIDValue()
	if err != nil {
		logx.Errorf("resource-rpc: mail channel disabled (template id): %v", err)
		return &ServiceContext{Config: c}
	}
	return &ServiceContext{
		Config:            c,
		MailProvider:      provider,
		DefaultTemplateID: defaultTemplateID,
	}
}
